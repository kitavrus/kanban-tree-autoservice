package suppliers

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

// Мягкое удаление поставщиков (docs/features/soft-delete-suppliers/draft-spec.md
// §4.2, §6.3, §6.4, часть 6 мастер-спеки soft-delete): удаление с причиной, восстановление,
// служебная уборка. Архив (is_active) и удаление независимы (DS-009).

// Actor — пользователь запроса: удаляет и восстанавливает только администратор (DS-003).
type Actor struct {
	ID      string
	IsAdmin bool
}

// DeleteResponse — ответ DELETE /api/v1/suppliers/:id (§6.3). DeletedAt — UTC.
type DeleteResponse struct {
	ID        string    `json:"id"`
	DeletedAt time.Time `json:"deleted_at"`
}

// RestoreResponse — ответ POST /api/v1/suppliers/:id/restore (§6.4).
type RestoreResponse struct {
	ID string `json:"id"`
}

var (
	errNotDeleted       = errors.New("suppliers: not deleted")
	errRestoreNameTaken = errors.New("suppliers: restore name taken")
)

// SoftDelete — мягкое удаление поставщика: один UPDATE с условием «ещё не удалён»; 0 строк →
// ErrNotFound (второй одновременный запрос — тоже, DS-024). is_active не меняется.
func (r *Repo) SoftDelete(ctx context.Context, id, userID, reason string) (time.Time, error) {
	var at time.Time
	err := r.pool.QueryRow(ctx, `
UPDATE suppliers SET deleted_at = now(), deleted_by = $2::uuid, delete_reason = $3, updated_at = now()
WHERE id = $1::uuid AND deleted_at IS NULL
RETURNING deleted_at`, id, userID, reason).Scan(&at)
	return at.UTC(), err
}

// Restore — восстановление поставщика: один UPDATE с условием «удалён» стирает три поля удаления,
// is_active прежний (DS-009). Название за время удаления мог занять действующий поставщик —
// нарушение частичного уникального индекса → errRestoreNameTaken (DS-017); индекс же решает
// гонку «создать/переименовать ↔ восстановить». 0 строк: поставщика нет → ErrNotFound,
// поставщик действует → errNotDeleted.
func (r *Repo) Restore(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `
UPDATE suppliers SET deleted_at = NULL, deleted_by = NULL, delete_reason = NULL, updated_at = now()
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
	if err := r.pool.QueryRow(ctx, `SELECT 1 FROM suppliers WHERE id = $1::uuid`, id).Scan(&one); err != nil {
		return err // pgx.ErrNoRows == ErrNotFound
	}
	return errNotDeleted
}

// Purge — физическое удаление поставщика (служебная уборка автотестов, DS-027): действующего
// либо удалённого.
func (r *Repo) Purge(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM suppliers WHERE id = $1::uuid`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete — МЯГКОЕ удаление поставщика (§6.3): только admin — роль проверяется первой, до поиска
// записи (DS-003); причина обязательна (DS-020).
func (s *Service) Delete(ctx context.Context, a Actor, id string, in softdelete.DeleteRequest) (*DeleteResponse, error) {
	if !a.IsAdmin {
		return nil, errorspkg.Forbidden("Нет доступа")
	}
	if uuid.Validate(id) != nil {
		return nil, ErrSupplierNotFound
	}
	reason, fields := softdelete.ValidateReason(in.Reason)
	if len(fields) > 0 {
		return nil, errorspkg.Validation(fields...)
	}
	at, err := s.repo.SoftDelete(ctx, id, a.ID, reason)
	if errors.Is(err, ErrNotFound) {
		return nil, ErrSupplierNotFound
	}
	if err != nil {
		return nil, err
	}
	// Аудит (DS-030): факт удаления; причина и контакты в журнал не пишутся.
	slog.InfoContext(ctx, "soft delete", "kind", string(softdelete.KindSuppliers), "id", id, "user_id", a.ID, "action", "delete")
	return &DeleteResponse{ID: id, DeletedAt: at}, nil
}

// Restore — восстановление поставщика (§6.4): только admin; название занято действующим
// поставщиком → 409 NAME_TAKEN; не удалён → 409 NOT_DELETED.
func (s *Service) Restore(ctx context.Context, a Actor, id string) (*RestoreResponse, error) {
	if !a.IsAdmin {
		return nil, errorspkg.Forbidden("Нет доступа")
	}
	if uuid.Validate(id) != nil {
		return nil, ErrSupplierNotFound
	}
	err := s.repo.Restore(ctx, id)
	switch {
	case errors.Is(err, ErrNotFound):
		return nil, ErrSupplierNotFound
	case errors.Is(err, errNotDeleted):
		return nil, softdelete.ErrNotDeleted
	case errors.Is(err, errRestoreNameTaken):
		return nil, softdelete.ErrNameTaken
	case err != nil:
		return nil, err
	}
	slog.InfoContext(ctx, "soft delete", "kind", string(softdelete.KindSuppliers), "id", id, "user_id", a.ID, "action", "restore")
	return &RestoreResponse{ID: id}, nil
}

// Purge — вид suppliers служебного адреса DELETE /api/v1/test/purge/:kind/:id (роут и гард
// admin — в пакете board, только при TEST_PURGE_ENABLED=true, DS-027): вид записи не
// раскрывается (404 NOT_FOUND).
func (s *Service) Purge(ctx context.Context, id string) error {
	if uuid.Validate(id) != nil {
		return errorspkg.NotFound("Не найдено")
	}
	err := s.repo.Purge(ctx, id)
	if errors.Is(err, ErrNotFound) {
		return errorspkg.NotFound("Не найдено")
	}
	return err
}

// currentActor — Actor из middleware.CurrentPrincipal (роут закрыт RequireAPI — principal всегда
// не nil).
func currentActor(c fiber.Ctx) Actor {
	p := middleware.CurrentPrincipal(c)
	return Actor{ID: p.ID, IsAdmin: p.IsAdmin()}
}

// Delete — DELETE /api/v1/suppliers/:id (admin), тело {reason}: 200 + id, deleted_at.
func (h *Handler) Delete(c fiber.Ctx) error {
	resp, err := h.svc.Delete(c.Context(), currentActor(c), c.Params("id"), softdelete.ParseDeleteRequest(c.Body()))
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

// Restore — POST /api/v1/suppliers/:id/restore (admin), тела нет.
func (h *Handler) Restore(c fiber.Ctx) error {
	resp, err := h.svc.Restore(c.Context(), currentActor(c), c.Params("id"))
	if err != nil {
		return err
	}
	return c.JSON(resp)
}
