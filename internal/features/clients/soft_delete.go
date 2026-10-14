package clients

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strings"
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
// возрастанию id). Документы перемещения берут клиента FOR KEY SHARE вставкой transfers
// (FK client_id) и затем единицы FOR NO KEY UPDATE — тот же порядок; доска берёт заказ →
// клиента (FOR KEY SHARE) либо заказ → equipment (FOR SHARE) и клиента с единицами вместе не
// держит.

// Actor — пользователь запроса: удаляет и восстанавливает только администратор (DS-003).
type Actor struct {
	ID      string
	IsAdmin bool
}

// equipmentLocationAtClient — единица у клиента: только такую можно удалить (DS-041).
const equipmentLocationAtClient = "at_client"

var (
	// ErrClientEquipmentAtWarehouse — действующая единица клиента на нашем складе или в пути
	// (DS-041); details.equipment_ids — такие единицы.
	ErrClientEquipmentAtWarehouse = errorspkg.New("CLIENT_EQUIPMENT_AT_WAREHOUSE", http.StatusConflict,
		"Оборудование клиента на складе или в пути — сначала верните его клиенту")
	// ErrClientEquipmentInTransfer — действующая единица клиента стоит в незавершённом документе
	// (DS-041); текст с номерами документов и details.transfer_ids собирает
	// clientEquipmentInTransfer (копия, sentinel не мутируется).
	ErrClientEquipmentInTransfer = errorspkg.New("CLIENT_EQUIPMENT_IN_TRANSFER", http.StatusConflict,
		"Оборудование клиента в перемещениях — сначала отмените их")
	// ErrEquipmentInTransfer — единица стоит в незавершённом документе; текст с номером и
	// details.transfer_id собирает equipmentInTransfer.
	ErrEquipmentInTransfer = errorspkg.New("EQUIPMENT_IN_TRANSFER", http.StatusConflict,
		"Оборудование в перемещении — сначала отмените его")
	// ErrEquipmentAtWarehouse — единица на нашем складе или в пути (DS-041).
	ErrEquipmentAtWarehouse = errorspkg.New("EQUIPMENT_AT_WAREHOUSE", http.StatusConflict,
		"Оборудование на складе или в пути — сначала верните его клиенту")
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

// equipmentAwayError — удаление клиента: единицы ids числятся не у клиента.
type equipmentAwayError struct{ ids []string }

func (e *equipmentAwayError) Error() string { return "clients: equipment not at client" }

// equipmentBusyError — удаление клиента либо единицы: единицы стоят в незавершённых документах
// transferIDs (по возрастанию, без повторов).
type equipmentBusyError struct{ transferIDs []int64 }

func (e *equipmentBusyError) Error() string { return "clients: equipment in transfer" }

func clientEquipmentInTransfer(ids []int64) *errorspkg.AppError {
	nums := make([]string, len(ids))
	for i, id := range ids {
		nums[i] = fmt.Sprintf("#%d", id)
	}
	e := ErrClientEquipmentInTransfer.WithDetail("transfer_ids", ids)
	e.Message = "Оборудование клиента в перемещениях: " + strings.Join(nums, ", ") + " — сначала отмените их"
	return e
}

func equipmentInTransfer(transferID int64) *errorspkg.AppError {
	e := ErrEquipmentInTransfer.WithDetail("transfer_id", transferID)
	e.Message = fmt.Sprintf("Оборудование в перемещении #%d — сначала отмените его", transferID)
	return e
}

// ---- repo ----

// SoftDeleteClient — мягкое удаление клиента вместе с его действующими единицами (§3, DS-015,
// DS-041). Одна транзакция:
//   - клиент под FOR UPDATE с условием «не удалён»: нет либо уже удалён → ErrNotFound (DS-024);
//   - его действующие единицы под FOR NO KEY UPDATE по возрастанию id (порядок блокировок
//     документов перемещения);
//   - единица не у клиента → equipmentAwayError; единица в незавершённом документе →
//     equipmentBusyError — ни клиент, ни единицы не помечаются;
//   - клиент и все его действующие единицы получают одни и те же «кто, когда, почему»; у единиц
//     — deleted_with_client = true.
//
// Заказы, документы и телефоны не трогаются; местоположение единиц не меняется (DS-008).
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

	rows, err := tx.Query(ctx, `
SELECT id::text, location_status, active_transfer_id FROM equipment
WHERE client_id = $1::uuid AND deleted_at IS NULL ORDER BY id FOR NO KEY UPDATE`, id)
	if err != nil {
		return at, 0, err
	}
	var away []string
	busy := map[int64]bool{}
	for rows.Next() {
		var eqID, location string
		var transferID *int64
		if err := rows.Scan(&eqID, &location, &transferID); err != nil {
			rows.Close()
			return at, 0, err
		}
		if location != equipmentLocationAtClient {
			away = append(away, eqID)
		}
		if transferID != nil {
			busy[*transferID] = true
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return at, 0, err
	}
	if len(away) > 0 {
		return at, 0, &equipmentAwayError{ids: away}
	}
	if len(busy) > 0 {
		ids := make([]int64, 0, len(busy))
		for t := range busy {
			ids = append(ids, t)
		}
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		return at, 0, &equipmentBusyError{transferIDs: ids}
	}

	err = tx.QueryRow(ctx, `
UPDATE clients SET deleted_at = now(), deleted_by = $2::uuid, delete_reason = $3,
       version = version + 1, updated_at = now()
WHERE id = $1::uuid AND deleted_at IS NULL
RETURNING deleted_at`, id, userID, reason).Scan(&at)
	if err != nil {
		return at, 0, err
	}
	// условия «у клиента и свободна» повторены в UPDATE: под блокировкой строк они уже
	// проверены, guard — страховка (DS-041).
	tag, err := tx.Exec(ctx, `
UPDATE equipment SET deleted_at = $2, deleted_by = $3::uuid, delete_reason = $4,
       deleted_with_client = true, updated_at = now()
WHERE client_id = $1::uuid AND deleted_at IS NULL
  AND location_status = 'at_client' AND active_transfer_id IS NULL`, id, at, userID, reason)
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
// удалённые отдельно, не трогаются. Местоположение и занятие удалённой единицы за время
// удаления измениться не могли: все пути документов, которые занимают единицу либо двигают её,
// отказывают по удалённой — создание, правка и восстановление заявки (transfers.claimEquipment),
// отправка (doSend) и отмена проведения полученного документа (unsettleEquipment) проверяют
// deleted_at под блокировкой строки equipment (DS-041, DS-045).
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

// SoftDeleteEquipment — мягкое удаление отдельной единицы (§3, DS-014, DS-041). Строка единицы
// под FOR NO KEY UPDATE (тот же режим, что у занятия единицы документом — гонка «удалить ↔
// занять» даёт ровно один успех): нет либо уже удалена → ErrNotFound; в незавершённом
// документе → equipmentBusyError; не у клиента → equipmentAwayError. Условия повторены в
// UPDATE. Строки transfer_items (в том числе мягко удалённых заявок), заказы и услуги не
// читаются — единицу они не удерживают (DS-014, DS-018). Возвращает клиента единицы.
func (r *Repo) SoftDeleteEquipment(ctx context.Context, id, userID, reason string) (string, time.Time, error) {
	var at time.Time
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return "", at, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var location string
	var transferID *int64
	err = tx.QueryRow(ctx, `
SELECT location_status, active_transfer_id FROM equipment
WHERE id = $1::uuid AND deleted_at IS NULL FOR NO KEY UPDATE`, id).Scan(&location, &transferID)
	if err != nil {
		return "", at, err
	}
	if transferID != nil {
		return "", at, &equipmentBusyError{transferIDs: []int64{*transferID}}
	}
	if location != equipmentLocationAtClient {
		return "", at, &equipmentAwayError{ids: []string{id}}
	}
	var clientID string
	err = tx.QueryRow(ctx, `
UPDATE equipment SET deleted_at = now(), deleted_by = $2::uuid, delete_reason = $3,
       deleted_with_client = false, updated_at = now()
WHERE id = $1::uuid AND deleted_at IS NULL
  AND active_transfer_id IS NULL AND location_status = 'at_client'
RETURNING client_id::text, deleted_at`, id, userID, reason).Scan(&clientID, &at)
	if err != nil {
		return "", at, err
	}
	return clientID, at.UTC(), tx.Commit(ctx)
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
// его единицы ссылаются заказы или строки документов → errInUse. Телефоны и единицы уходят
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
// удалённая; на единицу ссылаются заказы, услуги или строки документов → errInUse. Единицы
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
func mapDeleteErr(err error, notFound *errorspkg.AppError, client bool) error {
	var away *equipmentAwayError
	var busy *equipmentBusyError
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ErrNotFound):
		return notFound
	case errors.Is(err, errNotDeleted):
		return softdelete.ErrNotDeleted
	case errors.Is(err, errClientDeleted):
		return ErrClientDeleted
	case errors.As(err, &away) && client:
		return ErrClientEquipmentAtWarehouse.WithDetail("equipment_ids", away.ids)
	case errors.As(err, &away):
		return ErrEquipmentAtWarehouse
	case errors.As(err, &busy) && client:
		return clientEquipmentInTransfer(busy.transferIDs)
	case errors.As(err, &busy):
		return equipmentInTransfer(busy.transferIDs[0])
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
		return nil, mapDeleteErr(err, ErrClientNotFound, true)
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
		return nil, mapDeleteErr(err, ErrClientNotFound, true)
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
		return nil, "", mapDeleteErr(err, ErrEquipmentNotFound, false)
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
		return nil, mapDeleteErr(err, ErrEquipmentNotFound, false)
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
