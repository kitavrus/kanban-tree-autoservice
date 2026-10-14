package equipmenttypes

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/kitavrus/kanban-tree/internal/features/softdelete"
	"github.com/kitavrus/kanban-tree/internal/server/middleware"
	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
)

// Мягкое удаление типов оборудования (docs/features/soft-delete-catalog/draft-spec.md §4.4,
// §6.7, §6.8, часть 5 мастер-спеки soft-delete): удаление с причиной, восстановление, служебная
// уборка. Архив (is_active) и удаление независимы (DS-009).

// Actor — пользователь запроса: удаляет и восстанавливает только администратор (DS-003).
type Actor struct {
	ID      string
	IsAdmin bool
}

// DeleteResponse — ответ DELETE /api/v1/equipment-types/:id (§6.7). DeletedAt — UTC.
type DeleteResponse struct {
	ID        string    `json:"id"`
	DeletedAt time.Time `json:"deleted_at"`
}

// RestoreResponse — ответ POST /api/v1/equipment-types/:id/restore (§6.8).
type RestoreResponse struct {
	ID string `json:"id"`
}

var (
	errNotDeleted       = errors.New("equipmenttypes: not deleted")
	errRestoreNameTaken = errors.New("equipmenttypes: restore name taken")
)

// SoftDelete — мягкое удаление типа: один UPDATE с условием «ещё не удалён»; 0 строк →
// ErrNotFound (второй одновременный запрос — тоже, DS-024). Единицы оборудования этого типа не
// читаются и не мешают — в том числе когда единственная единица сама в удалённых: UPDATE вместо
// DELETE, внешний ключ equipment.type_id не срабатывает (DS-014, TS-061). is_active не меняется.
func (r *Repo) SoftDelete(ctx context.Context, id, userID, reason string) (time.Time, error) {
	var at time.Time
	err := r.pool.QueryRow(ctx, `
UPDATE equipment_types SET deleted_at = now(), deleted_by = $2::uuid, delete_reason = $3, updated_at = now()
WHERE id = $1::uuid AND deleted_at IS NULL
RETURNING deleted_at`, id, userID, reason).Scan(&at)
	return at.UTC(), err
}

// Restore — восстановление типа: один UPDATE с условием «удалён» стирает три поля удаления,
// is_active прежний (DS-009). Название за время удаления мог занять действующий тип (активный
// либо архивный) — нарушение частичного уникального индекса → errRestoreNameTaken (DS-017);
// индекс же решает гонку «создать/переименовать ↔ восстановить» — ровно один успех. 0 строк:
// типа нет → ErrNotFound, тип действует → errNotDeleted.
func (r *Repo) Restore(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `
UPDATE equipment_types SET deleted_at = NULL, deleted_by = NULL, delete_reason = NULL, updated_at = now()
WHERE id = $1::uuid AND deleted_at IS NOT NULL`, id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return errRestoreNameTaken
		}
		return err
	}
	if tag.RowsAffected() > 0 {
		return nil
	}
	var one int
	if err := r.pool.QueryRow(ctx, `SELECT 1 FROM equipment_types WHERE id = $1::uuid`, id).Scan(&one); err != nil {
		return err // pgx.ErrNoRows == ErrNotFound
	}
	return errNotDeleted
}

// Delete — МЯГКОЕ удаление типа (§6.7): только admin — роль проверяется первой, до поиска
// записи (DS-003); причина обязательна (DS-020); единицы не мешают (отказ
// EQUIPMENT_TYPE_IN_USE снят, DS-014).
func (s *Service) Delete(ctx context.Context, a Actor, id string, in softdelete.DeleteRequest) (*DeleteResponse, error) {
	if !a.IsAdmin {
		return nil, errorspkg.Forbidden("Нет доступа")
	}
	if uuid.Validate(id) != nil {
		return nil, ErrTypeNotFound
	}
	reason, fields := softdelete.ValidateReason(in.Reason)
	if len(fields) > 0 {
		return nil, errorspkg.Validation(fields...)
	}
	at, err := s.repo.SoftDelete(ctx, id, a.ID, reason)
	if errors.Is(err, ErrNotFound) {
		return nil, ErrTypeNotFound
	}
	if err != nil {
		return nil, err
	}
	// Аудит (DS-030): факт удаления; причина в журнал не пишется.
	slog.InfoContext(ctx, "soft delete", "kind", string(softdelete.KindEquipmentTypes), "id", id, "user_id", a.ID, "action", "delete")
	return &DeleteResponse{ID: id, DeletedAt: at}, nil
}

// Restore — восстановление типа (§6.8): только admin; название занято действующим типом → 409
// NAME_TAKEN; не удалён → 409 NOT_DELETED.
func (s *Service) Restore(ctx context.Context, a Actor, id string) (*RestoreResponse, error) {
	if !a.IsAdmin {
		return nil, errorspkg.Forbidden("Нет доступа")
	}
	if uuid.Validate(id) != nil {
		return nil, ErrTypeNotFound
	}
	err := s.repo.Restore(ctx, id)
	switch {
	case errors.Is(err, ErrNotFound):
		return nil, ErrTypeNotFound
	case errors.Is(err, errNotDeleted):
		return nil, softdelete.ErrNotDeleted
	case errors.Is(err, errRestoreNameTaken):
		return nil, softdelete.ErrNameTaken
	case err != nil:
		return nil, err
	}
	slog.InfoContext(ctx, "soft delete", "kind", string(softdelete.KindEquipmentTypes), "id", id, "user_id", a.ID, "action", "restore")
	return &RestoreResponse{ID: id}, nil
}

// Purge — вид equipment-types служебного адреса DELETE /api/v1/test/purge/:kind/:id (роут и
// гард admin — в пакете board, только при TEST_PURGE_ENABLED=true, DS-027): вид записи не
// раскрывается (404 NOT_FOUND); на тип ссылаются единицы → прежний 409 EQUIPMENT_TYPE_IN_USE.
func (s *Service) Purge(ctx context.Context, id string) error {
	if uuid.Validate(id) != nil {
		return errorspkg.NotFound("Не найдено")
	}
	err := s.repo.Purge(ctx, id)
	switch {
	case errors.Is(err, ErrNotFound):
		return errorspkg.NotFound("Не найдено")
	case errors.Is(err, errInUse):
		return ErrTypeInUse
	}
	return err
}

// currentActor — Actor из middleware.CurrentPrincipal (роут закрыт RequireAPI/RequireSSR —
// principal всегда не nil).
func currentActor(c fiber.Ctx) Actor {
	p := middleware.CurrentPrincipal(c)
	return Actor{ID: p.ID, IsAdmin: p.IsAdmin()}
}

// Delete — DELETE /api/v1/equipment-types/:id (admin), тело {reason}: 200 + id, deleted_at.
func (h *Handler) Delete(c fiber.Ctx) error {
	resp, err := h.svc.Delete(c.Context(), currentActor(c), c.Params("id"), softdelete.ParseDeleteRequest(c.Body()))
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

// Restore — POST /api/v1/equipment-types/:id/restore (admin), тела нет.
func (h *Handler) Restore(c fiber.Ctx) error {
	resp, err := h.svc.Restore(c.Context(), currentActor(c), c.Params("id"))
	if err != nil {
		return err
	}
	return c.JSON(resp)
}
