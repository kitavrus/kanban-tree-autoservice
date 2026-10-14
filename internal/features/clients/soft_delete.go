package clients

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/kitavrus/kanban-tree/internal/features/softdelete"
	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
)

// Мягкое удаление клиентов и единиц оборудования (docs/features/soft-delete-clients/draft-spec.md,
// часть 4 мастер-спеки soft-delete): удаление с причиной, восстановление, служебная уборка.
//
// Порядок блокировок (общий со всеми путями, пишущими в equipment): клиент → equipment (по
// возрастанию id); доска берёт заказ → клиента (FOR KEY SHARE) либо заказ → equipment (FOR SHARE) и клиента с единицами вместе не
// держит.

// Actor — пользователь запроса: удаляет и восстанавливает только администратор (DS-003).
type Actor struct {
	ID      string
	IsAdmin bool
}

var (
	// ErrClientDeleted — единицу удалённого клиента отдельно восстановить нельзя (DS-015).
	ErrClientDeleted = errorspkg.New("CLIENT_DELETED", http.StatusConflict, "Сначала восстановите клиента")
)

// ClientDeleteResponse — ответ DELETE /api/v1/clients/:id (§6.1). DeletedAt — UTC.
type ClientDeleteResponse struct {
	ID               string    `json:"id"`
	DeletedAt        time.Time `json:"deleted_at"`
	EquipmentDeleted int       `json:"equipment_deleted"`
}

// ClientRestoreResponse — ответ POST /api/v1/clients/:id/restore (§6.2).
type ClientRestoreResponse struct {
	ID                string `json:"id"`
	EquipmentRestored int    `json:"equipment_restored"`
}

// EquipmentDeleteResponse — ответ DELETE /api/v1/equipment/:id (§6.3). DeletedAt — UTC.
type EquipmentDeleteResponse struct {
	ID        string    `json:"id"`
	DeletedAt time.Time `json:"deleted_at"`
}

// EquipmentRestoreResponse — ответ POST /api/v1/equipment/:id/restore (§6.4).
type EquipmentRestoreResponse struct {
	ID string `json:"id"`
}

// ---- repo ----

// SoftDeleteClient — мягкое удаление клиента вместе с его действующими единицами (§3, DS-015).
// Одна транзакция:
//   - клиент под FOR UPDATE с условием «не удалён»: нет либо уже удалён → ErrNotFound (DS-024);
//   - его действующие единицы под FOR NO KEY UPDATE по возрастанию id;
//   - клиент и все его действующие единицы получают одни и те же «кто, когда, почему»; у единиц
//     — deleted_with_client = true.
//
// Заказы и телефоны не трогаются.
func (r *Repo) SoftDeleteClient(ctx context.Context, id, userID, reason string) (time.Time, int, error) {
	var at time.Time
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return at, 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var one int
	err = tx.QueryRow(ctx, `SELECT 1 FROM clients WHERE id = $1::uuid AND deleted_at IS NULL FOR UPDATE`, id).Scan(&one)
	if err != nil {
		return at, 0, err // pgx.ErrNoRows == ErrNotFound
	}

	if _, err := tx.Exec(ctx, `
SELECT 1 FROM equipment WHERE client_id = $1::uuid AND deleted_at IS NULL ORDER BY id FOR NO KEY UPDATE`, id); err != nil {
		return at, 0, err
	}

	err = tx.QueryRow(ctx, `
UPDATE clients SET deleted_at = now(), deleted_by = $2::uuid, delete_reason = $3,
       version = version + 1, updated_at = now()
WHERE id = $1::uuid AND deleted_at IS NULL
RETURNING deleted_at`, id, userID, reason).Scan(&at)
	if err != nil {
		return at, 0, err
	}
	tag, err := tx.Exec(ctx, `
UPDATE equipment SET deleted_at = $2, deleted_by = $3::uuid, delete_reason = $4,
       deleted_with_client = true, updated_at = now()
WHERE client_id = $1::uuid AND deleted_at IS NULL`, id, at, userID, reason)
	if err != nil {
		return at, 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return at, 0, err
	}
	return at.UTC(), int(tag.RowsAffected()), nil
}

// RestoreClient — восстановление клиента и единиц, ушедших вместе с ним (§3, DS-015). Одна
// транзакция: клиент под FOR UPDATE (нет → ErrNotFound; не удалён → errNotDeleted), отметка
// стирается; у единиц с deleted_with_client = true стираются отметка и флаг. Единицы,
// удалённые отдельно, не трогаются.
func (r *Repo) RestoreClient(ctx context.Context, id string) (int, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var deleted bool
	if err := tx.QueryRow(ctx, `SELECT deleted_at IS NOT NULL FROM clients WHERE id = $1::uuid FOR UPDATE`, id).Scan(&deleted); err != nil {
		return 0, err
	}
	if !deleted {
		return 0, errNotDeleted
	}
	tag, err := tx.Exec(ctx, `
UPDATE clients SET deleted_at = NULL, deleted_by = NULL, delete_reason = NULL,
       version = version + 1, updated_at = now()
WHERE id = $1::uuid AND deleted_at IS NOT NULL`, id)
	if err != nil {
		return 0, err
	}
	if tag.RowsAffected() == 0 {
		return 0, errNotDeleted
	}
	// ORDER BY id + FOR NO KEY UPDATE — тот же порядок блокировок единиц, что при удалении.
	if _, err := tx.Exec(ctx, `
SELECT 1 FROM equipment WHERE client_id = $1::uuid AND deleted_with_client ORDER BY id FOR NO KEY UPDATE`, id); err != nil {
		return 0, err
	}
	tag, err = tx.Exec(ctx, `
UPDATE equipment SET deleted_at = NULL, deleted_by = NULL, delete_reason = NULL,
       deleted_with_client = false, updated_at = now()
WHERE client_id = $1::uuid AND deleted_with_client`, id)
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

// SoftDeleteEquipment — мягкое удаление отдельной единицы (§3, DS-014): один UPDATE с условием
// «не удалена» (строка equipment блокируется самим UPDATE): нет либо уже удалена → ErrNotFound.
// Заказы и услуги не читаются — единицу они не удерживают (DS-014). Возвращает клиента единицы.
func (r *Repo) SoftDeleteEquipment(ctx context.Context, id, userID, reason string) (string, time.Time, error) {
	var at time.Time
	var clientID string
	err := r.pool.QueryRow(ctx, `
UPDATE equipment SET deleted_at = now(), deleted_by = $2::uuid, delete_reason = $3,
       deleted_with_client = false, updated_at = now()
WHERE id = $1::uuid AND deleted_at IS NULL
RETURNING client_id::text, deleted_at`, id, userID, reason).Scan(&clientID, &at)
	if err != nil {
		return "", at, err // pgx.ErrNoRows == ErrNotFound
	}
	return clientID, at.UTC(), nil
}

// RestoreEquipment — восстановление отдельной единицы (§3, DS-015). Клиент единицы читается
// FOR KEY SHARE (удаление клиента, FOR UPDATE, либо уже видно, либо ждёт), затем строка единицы
// под FOR NO KEY UPDATE — порядок «клиент → equipment». Единицы нет → ErrNotFound; не удалена →
// errNotDeleted; клиент удалён → errClientDeleted (и для ушедшей с клиентом, и для удалённой
// отдельно).
func (r *Repo) RestoreEquipment(ctx context.Context, id string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var clientID string
	// client_id единицы не меняется — читается без блокировки, чтобы клиента взять первым.
	if err := tx.QueryRow(ctx, `SELECT client_id::text FROM equipment WHERE id = $1::uuid`, id).Scan(&clientID); err != nil {
		return err
	}
	var clientDeleted bool
	if err := tx.QueryRow(ctx, `SELECT deleted_at IS NOT NULL FROM clients WHERE id = $1::uuid FOR KEY SHARE`, clientID).Scan(&clientDeleted); err != nil {
		return err
	}
	var deleted bool
	if err := tx.QueryRow(ctx, `SELECT deleted_at IS NOT NULL FROM equipment WHERE id = $1::uuid FOR NO KEY UPDATE`, id).Scan(&deleted); err != nil {
		return err
	}
	if !deleted {
		return errNotDeleted
	}
	if clientDeleted {
		return errClientDeleted
	}
	tag, err := tx.Exec(ctx, `
UPDATE equipment SET deleted_at = NULL, deleted_by = NULL, delete_reason = NULL,
       deleted_with_client = false, updated_at = now()
WHERE id = $1::uuid AND deleted_at IS NOT NULL`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errNotDeleted
	}
	return tx.Commit(ctx)
}

// isRestrictViolation — нарушение ON DELETE RESTRICT (PG отдаёт то 23503, то 23001).
func isRestrictViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && (pgErr.Code == "23503" || pgErr.Code == "23001")
}

// PurgeClient — служебная ФИЗИЧЕСКАЯ уборка клиента автотестами (§6.5, DS-027): действующий
// либо мягко удалённый клиент по правилам прежнего безвозвратного удаления — на клиента либо
// его единицы ссылаются заказы → errInUse. Телефоны и единицы уходят
// каскадом FK. Клиента нет → ErrNotFound.
func (r *Repo) PurgeClient(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM clients WHERE id = $1::uuid`, id)
	if err != nil {
		if isRestrictViolation(err) {
			return errInUse
		}
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// PurgeEquipment — служебная ФИЗИЧЕСКАЯ уборка единицы (§6.5, DS-027): действующая либо мягко
// удалённая; на единицу ссылаются заказы или услуги → errInUse. Единицы
// нет → ErrNotFound.
func (r *Repo) PurgeEquipment(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM equipment WHERE id = $1::uuid`, id)
	if err != nil {
		if isRestrictViolation(err) {
			return errInUse
		}
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ---- service ----

// mapDeleteErr — ошибки repo удаления/восстановления → wire-ошибки; notFound — 404 вида.
func mapDeleteErr(err error, notFound *errorspkg.AppError) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ErrNotFound):
		return notFound
	case errors.Is(err, errNotDeleted):
		return softdelete.ErrNotDeleted
	case errors.Is(err, errClientDeleted):
		return ErrClientDeleted
	}
	return err
}

// Delete — МЯГКОЕ удаление клиента (§6.1): только admin — роль проверяется первой, до поиска
// записи (DS-003); причина обязательна (DS-020). Клиент и его действующие единицы уходят в
// раздел «Удалённые»; version клиента не передаётся и не проверяется.
func (s *Service) Delete(ctx context.Context, a Actor, id string, in softdelete.DeleteRequest) (*ClientDeleteResponse, error) {
	if !a.IsAdmin {
		return nil, errorspkg.Forbidden("Нет доступа")
	}
	if uuid.Validate(id) != nil {
		return nil, ErrClientNotFound
	}
	reason, fields := softdelete.ValidateReason(in.Reason)
	if len(fields) > 0 {
		return nil, errorspkg.Validation(fields...)
	}
	at, n, err := s.repo.SoftDeleteClient(ctx, id, a.ID, reason)
	if err != nil {
		return nil, mapDeleteErr(err, ErrClientNotFound)
	}
	// Аудит (DS-030): факт удаления и число затронутых единиц; причина и данные клиента в
	// журнал не пишутся.
	slog.InfoContext(ctx, "soft delete", "kind", "clients", "id", id, "user_id", a.ID, "action", "delete", "equipment", n)
	return &ClientDeleteResponse{ID: id, DeletedAt: at, EquipmentDeleted: n}, nil
}

// Restore — восстановление клиента вместе с единицами, ушедшими с ним (§6.2): только admin.
func (s *Service) Restore(ctx context.Context, a Actor, id string) (*ClientRestoreResponse, error) {
	if !a.IsAdmin {
		return nil, errorspkg.Forbidden("Нет доступа")
	}
	if uuid.Validate(id) != nil {
		return nil, ErrClientNotFound
	}
	n, err := s.repo.RestoreClient(ctx, id)
	if err != nil {
		return nil, mapDeleteErr(err, ErrClientNotFound)
	}
	slog.InfoContext(ctx, "soft delete", "kind", "clients", "id", id, "user_id", a.ID, "action", "restore", "equipment", n)
	return &ClientRestoreResponse{ID: id, EquipmentRestored: n}, nil
}

// DeleteEquipment — МЯГКОЕ удаление отдельной единицы (§6.3): только admin, роль — первой;
// причина обязательна. Второй результат — клиент единицы (PRG-редирект SSR).
func (s *Service) DeleteEquipment(ctx context.Context, a Actor, id string, in softdelete.DeleteRequest) (*EquipmentDeleteResponse, string, error) {
	if !a.IsAdmin {
		return nil, "", errorspkg.Forbidden("Нет доступа")
	}
	if uuid.Validate(id) != nil {
		return nil, "", ErrEquipmentNotFound
	}
	reason, fields := softdelete.ValidateReason(in.Reason)
	if len(fields) > 0 {
		return nil, "", errorspkg.Validation(fields...)
	}
	clientID, at, err := s.repo.SoftDeleteEquipment(ctx, id, a.ID, reason)
	if err != nil {
		return nil, "", mapDeleteErr(err, ErrEquipmentNotFound)
	}
	slog.InfoContext(ctx, "soft delete", "kind", "equipment", "id", id, "user_id", a.ID, "action", "delete")
	return &EquipmentDeleteResponse{ID: id, DeletedAt: at}, clientID, nil
}

// RestoreEquipment — восстановление отдельной единицы (§6.4): только admin; клиент единицы в
// удалённых → 409 CLIENT_DELETED.
func (s *Service) RestoreEquipment(ctx context.Context, a Actor, id string) (*EquipmentRestoreResponse, error) {
	if !a.IsAdmin {
		return nil, errorspkg.Forbidden("Нет доступа")
	}
	if uuid.Validate(id) != nil {
		return nil, ErrEquipmentNotFound
	}
	if err := s.repo.RestoreEquipment(ctx, id); err != nil {
		return nil, mapDeleteErr(err, ErrEquipmentNotFound)
	}
	slog.InfoContext(ctx, "soft delete", "kind", "equipment", "id", id, "user_id", a.ID, "action", "restore")
	return &EquipmentRestoreResponse{ID: id}, nil
}

// mapPurgeErr — уборка не раскрывает вид записи: 404 NOT_FOUND; запись используется → 409
// CONFLICT (§6.5).
func mapPurgeErr(err error) error {
	switch {
	case errors.Is(err, ErrNotFound):
		return errorspkg.NotFound("Не найдено")
	case errors.Is(err, errInUse):
		return errorspkg.Conflict("Запись используется")
	}
	return err
}

// PurgeClient — вид clients служебного адреса DELETE /api/v1/test/purge/:kind/:id (роут и гард
// admin — в пакете board, существует только при TEST_PURGE_ENABLED=true, DS-027).
func (s *Service) PurgeClient(ctx context.Context, id string) error {
	if uuid.Validate(id) != nil {
		return errorspkg.NotFound("Не найдено")
	}
	return mapPurgeErr(s.repo.PurgeClient(ctx, id))
}

// PurgeEquipment — вид equipment того же служебного адреса.
func (s *Service) PurgeEquipment(ctx context.Context, id string) error {
	if uuid.Validate(id) != nil {
		return errorspkg.NotFound("Не найдено")
	}
	return mapPurgeErr(s.repo.PurgeEquipment(ctx, id))
}
