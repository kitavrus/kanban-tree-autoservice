package warehouses

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/kitavrus/kanban-tree/internal/features/softdelete"
	"github.com/kitavrus/kanban-tree/internal/server/middleware"
	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
)

// Мягкое удаление складов (docs/features/soft-delete-warehouses-suppliers/draft-spec.md §4.1,
// §6.1, §6.2, часть 6 мастер-спеки soft-delete): удаление с причиной — только пустого склада,
// восстановление, служебная уборка. Архив (is_active) и удаление независимы (DS-009).

// Actor — пользователь запроса: удаляет и восстанавливает только администратор (DS-003).
type Actor struct {
	ID      string
	IsAdmin bool
}

// DeleteResponse — ответ DELETE /api/v1/warehouses/:id (§6.1). DeletedAt — UTC.
type DeleteResponse struct {
	ID        string    `json:"id"`
	DeletedAt time.Time `json:"deleted_at"`
}

// RestoreResponse — ответ POST /api/v1/warehouses/:id/restore (§6.2).
type RestoreResponse struct {
	ID string `json:"id"`
}

// Отказы удаления (§6.1): код WAREHOUSE_IN_USE один, сообщение называет, что мешает (DS-021).
var (
	ErrDeleteHasStock = errorspkg.New("WAREHOUSE_IN_USE", http.StatusConflict,
		"На складе есть товар — сначала переместите или спишите остаток")
	ErrDeleteHasEquipment = errorspkg.New("WAREHOUSE_IN_USE", http.StatusConflict,
		"На складе есть оборудование — сначала переместите его")
	ErrDeleteHasTransfers = errorspkg.New("WAREHOUSE_IN_USE", http.StatusConflict,
		"На складе есть незавершённые перемещения")
	ErrDeleteLastActive = errorspkg.New("WAREHOUSE_LAST_ACTIVE", http.StatusConflict,
		"Нельзя удалить последний активный склад")
	// ErrPurgeInUse — служебная уборка: на склад ссылаются другие строки (§6.5).
	ErrPurgeInUse = errorspkg.Conflict("Запись используется")
)

var (
	errNotDeleted       = errors.New("warehouses: not deleted")
	errRestoreNameTaken = errors.New("warehouses: restore name taken")
	errPurgeInUse       = errors.New("warehouses: purge in use")
)

// deleteBlockers — что мешает удалить склад (DS-021). ОДИН запрос = один снимок данных: приём
// либо отмена документа «В пути» одновременно с удалением виден либо незавершённым документом,
// либо уже остатком/оборудованием на складе — раздельные запросы пропустили бы его между собой.
// Остаток считается и у товаров, находящихся в удалённых; единицы — все, без условия по удалению
// (страховка: удалённых единиц на складе по DS-041 не бывает); документы — только действующие
// (мягко удалённая заявка не мешает, DS-044).
func (r *Repo) deleteBlockers(ctx context.Context, q dbtx, id string) (stock, equipment, transfers bool, err error) {
	err = q.QueryRow(ctx, `
SELECT EXISTS (SELECT 1 FROM product_stock WHERE warehouse_id = $1::uuid AND stock <> 0),
       EXISTS (SELECT 1 FROM equipment WHERE warehouse_id = $1::uuid),
       EXISTS (SELECT 1 FROM transfers
               WHERE status IN ('requested', 'in_transit') AND deleted_at IS NULL
                 AND (from_warehouse_id = $1::uuid OR to_warehouse_id = $1::uuid))`, id).
		Scan(&stock, &equipment, &transfers)
	return stock, equipment, transfers, err
}

// softDelete — мягкое удаление склада одной транзакцией (§3 «Конкурентность», DS-021, DS-024).
// Блокировка FOR UPDATE по возрастанию id: действующие активные склады (тот же набор и порядок,
// что у архивирования — withLastActiveGuard) плюс сам удаляемый, если он архивный. Под ней —
// проверки и UPDATE с условием «ещё не удалён». Записи, создающие ссылку на склад (документ,
// строка товара заказа, корректировка и правка остатка), берут строку склада FOR SHARE с
// условием «не удалён» — они ждут эту транзакцию либо она ждёт их и затем видит их результат.
func (s *Service) softDelete(ctx context.Context, id, userID, reason string) (time.Time, error) {
	var at time.Time
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return at, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rows, err := tx.Query(ctx, `
SELECT id::text, is_active FROM warehouses
WHERE deleted_at IS NULL AND (is_active OR id = $1::uuid)
ORDER BY id FOR UPDATE`, id)
	if err != nil {
		return at, err
	}
	found, isActive, activeCount := false, false, 0
	for rows.Next() {
		var wid string
		var active bool
		if err := rows.Scan(&wid, &active); err != nil {
			rows.Close()
			return at, err
		}
		if active {
			activeCount++
		}
		if strings.EqualFold(wid, id) {
			found, isActive = true, active
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return at, err
	}
	if !found {
		return at, ErrNotFound
	}

	stock, equipment, transfers, err := s.repo.deleteBlockers(ctx, tx, id)
	switch {
	case err != nil:
		return at, err
	case stock:
		return at, ErrDeleteHasStock
	case equipment:
		return at, ErrDeleteHasEquipment
	case transfers:
		return at, ErrDeleteHasTransfers
	case isActive && activeCount == 1:
		return at, ErrDeleteLastActive
	}

	err = tx.QueryRow(ctx, `
UPDATE warehouses SET deleted_at = now(), deleted_by = $2::uuid, delete_reason = $3, updated_at = now()
WHERE id = $1::uuid AND deleted_at IS NULL
RETURNING deleted_at`, id, userID, reason).Scan(&at)
	if err != nil {
		return at, err // pgx.ErrNoRows == ErrNotFound
	}
	return at.UTC(), tx.Commit(ctx)
}

// Restore — восстановление склада: один UPDATE с условием «удалён» стирает три поля удаления;
// is_active и position прежние (DS-009). Название за время удаления мог занять действующий
// склад (активный либо архивный) — нарушение частичного уникального индекса →
// errRestoreNameTaken (DS-017); индекс же решает гонку «создать/переименовать ↔ восстановить».
// 0 строк: склада нет → ErrNotFound, склад действует → errNotDeleted.
func (r *Repo) Restore(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `
UPDATE warehouses SET deleted_at = NULL, deleted_by = NULL, delete_reason = NULL, updated_at = now()
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
	if err := r.pool.QueryRow(ctx, `SELECT 1 FROM warehouses WHERE id = $1::uuid`, id).Scan(&one); err != nil {
		return err // pgx.ErrNoRows == ErrNotFound
	}
	return errNotDeleted
}

// Purge — физическое удаление склада (служебная уборка автотестов, DS-027): действующего либо
// удалённого. Нулевые строки остатков склада и его журнал (stock_movements) ссылкой не считаются
// и убираются вместе с ним — журнал производный: заказ и документ, чьи движения в нём записаны,
// держат склад своими строками. Ненулевой остаток, оборудование, документы, строки заказов —
// внешние ключи RESTRICT → errPurgeInUse, журнал при отказе остаётся (одна транзакция).
// Последний действующий активный склад не убирается — системе нужен склад по
// умолчанию (тот же инвариант, что у удаления и архивирования).
func (r *Repo) Purge(ctx context.Context, id string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rows, err := tx.Query(ctx, `
SELECT id::text, is_active AND deleted_at IS NULL FROM warehouses
WHERE (is_active AND deleted_at IS NULL) OR id = $1::uuid
ORDER BY id FOR UPDATE`, id)
	if err != nil {
		return err
	}
	found, live, liveCount := false, false, 0
	for rows.Next() {
		var wid string
		var l bool
		if err := rows.Scan(&wid, &l); err != nil {
			rows.Close()
			return err
		}
		if l {
			liveCount++
		}
		if strings.EqualFold(wid, id) {
			found, live = true, l
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	if !found {
		return ErrNotFound
	}
	if live && liveCount == 1 {
		return errPurgeInUse
	}
	if _, err := tx.Exec(ctx, `DELETE FROM product_stock WHERE warehouse_id = $1::uuid AND stock = 0`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM stock_movements WHERE warehouse_id = $1::uuid`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM warehouses WHERE id = $1::uuid`, id); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && (pgErr.Code == "23503" || pgErr.Code == "23001") {
			return errPurgeInUse
		}
		return err
	}
	return tx.Commit(ctx)
}

// Delete — МЯГКОЕ удаление склада (§6.1): только admin — роль проверяется первой, до поиска
// записи (DS-003); причина обязательна (DS-020); удаляется только пустой склад (DS-021).
func (s *Service) Delete(ctx context.Context, a Actor, id string, in softdelete.DeleteRequest) (*DeleteResponse, error) {
	if !a.IsAdmin {
		return nil, errorspkg.Forbidden("Нет доступа")
	}
	if uuid.Validate(id) != nil {
		return nil, ErrWarehouseNotFound
	}
	reason, fields := softdelete.ValidateReason(in.Reason)
	if len(fields) > 0 {
		return nil, errorspkg.Validation(fields...)
	}
	at, err := s.softDelete(ctx, id, a.ID, reason)
	if errors.Is(err, ErrNotFound) {
		return nil, ErrWarehouseNotFound
	}
	if err != nil {
		return nil, err
	}
	// Аудит (DS-030): факт удаления; причина в журнал не пишется.
	slog.InfoContext(ctx, "soft delete", "kind", string(softdelete.KindWarehouses), "id", id, "user_id", a.ID, "action", "delete")
	return &DeleteResponse{ID: id, DeletedAt: at}, nil
}

// Restore — восстановление склада (§6.2): только admin; название занято действующим складом →
// 409 NAME_TAKEN; не удалён → 409 NOT_DELETED. Остаток, оборудование и документы перепроверять
// не нужно: восстановление ничего не запрещает — склад с остатком, появившимся за время удаления
// (DS-042), просто снова действует.
func (s *Service) Restore(ctx context.Context, a Actor, id string) (*RestoreResponse, error) {
	if !a.IsAdmin {
		return nil, errorspkg.Forbidden("Нет доступа")
	}
	if uuid.Validate(id) != nil {
		return nil, ErrWarehouseNotFound
	}
	err := s.repo.Restore(ctx, id)
	switch {
	case errors.Is(err, ErrNotFound):
		return nil, ErrWarehouseNotFound
	case errors.Is(err, errNotDeleted):
		return nil, softdelete.ErrNotDeleted
	case errors.Is(err, errRestoreNameTaken):
		return nil, softdelete.ErrNameTaken
	case err != nil:
		return nil, err
	}
	slog.InfoContext(ctx, "soft delete", "kind", string(softdelete.KindWarehouses), "id", id, "user_id", a.ID, "action", "restore")
	return &RestoreResponse{ID: id}, nil
}

// Purge — вид warehouses служебного адреса DELETE /api/v1/test/purge/:kind/:id (роут и гард
// admin — в пакете board, только при TEST_PURGE_ENABLED=true, DS-027): вид записи не
// раскрывается (404 NOT_FOUND); на склад ссылаются другие строки → 409 CONFLICT.
func (s *Service) Purge(ctx context.Context, id string) error {
	if uuid.Validate(id) != nil {
		return errorspkg.NotFound("Не найдено")
	}
	err := s.repo.Purge(ctx, id)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return errorspkg.NotFound("Не найдено")
	case errors.Is(err, errPurgeInUse):
		return ErrPurgeInUse
	}
	return err
}

// currentActor — Actor из middleware.CurrentPrincipal (роут закрыт RequireAPI/RequireSSR —
// principal всегда не nil).
func currentActor(c fiber.Ctx) Actor {
	p := middleware.CurrentPrincipal(c)
	return Actor{ID: p.ID, IsAdmin: p.IsAdmin()}
}

// Delete — DELETE /api/v1/warehouses/:id (admin), тело {reason}: 200 + id, deleted_at.
func (h *Handler) Delete(c fiber.Ctx) error {
	resp, err := h.svc.Delete(c.Context(), currentActor(c), c.Params("id"), softdelete.ParseDeleteRequest(c.Body()))
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

// Restore — POST /api/v1/warehouses/:id/restore (admin), тела нет. Не путать с SSR
// POST /warehouses/:id/restore — это возврат из архива.
func (h *Handler) Restore(c fiber.Ctx) error {
	resp, err := h.svc.Restore(c.Context(), currentActor(c), c.Params("id"))
	if err != nil {
		return err
	}
	return c.JSON(resp)
}
