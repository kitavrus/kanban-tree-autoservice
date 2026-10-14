package catalog

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/kitavrus/kanban-tree/internal/features/softdelete"
	"github.com/kitavrus/kanban-tree/internal/server/middleware"
	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
)

// Мягкое удаление товаров, услуг и категорий каталога (docs/features/soft-delete-catalog/
// draft-spec.md, часть 5 мастер-спеки soft-delete): удаление с причиной, восстановление,
// служебная уборка. Архив (is_active) и удаление независимы (DS-009): удаление и восстановление
// is_active не трогают.
//
// Порядок блокировок товара (общий со всеми путями, которые берут строку products явно): товар
// → категория. Остаток живёт в самой строке товара: удаление берёт её FOR NO KEY UPDATE и под
// этой блокировкой читает остаток — правка остатка (форма товара, ручная корректировка,
// списание и возврат по заказам) пишет ту же строку и потому либо уже видна, либо ждёт.
// Пути, добавляющие НОВУЮ ссылку на товар (строка услуги заказа, состав услуги каталога), берут
// товар с условием «не удалён» под блокировкой строки.

// Actor — пользователь запроса: удаляет и восстанавливает только администратор (DS-003).
type Actor struct {
	ID      string
	IsAdmin bool
}

// DeleteResponse — ответ DELETE /api/v1/{products|services|catalog-categories}/:id (§6.1, §6.3,
// §6.5). DeletedAt — UTC.
type DeleteResponse struct {
	ID        string    `json:"id"`
	DeletedAt time.Time `json:"deleted_at"`
}

// RestoreResponse — ответ POST /api/v1/{…}/:id/restore (§6.2, §6.4, §6.6).
type RestoreResponse struct {
	ID string `json:"id"`
}

var (
	errNotDeleted      = errors.New("catalog: not deleted")
	errProductHasStock = errors.New("catalog: product has stock")
	errNameTaken       = errors.New("catalog: name taken")
)

// ---- repo ----

// SoftDelete — мягкое удаление товара (§3, DS-021, DS-024). Одна транзакция:
//   - строка товара под FOR NO KEY UPDATE с условием «не удалён»: нет либо уже удалён →
//     ErrProductNotFound (второй одновременный запрос ждёт и получает 404);
//   - остаток товара (под той же блокировкой) не равен нулю → errProductHasStock, отметка не
//     ставится;
//   - UPDATE трёх полей удаления; is_active не меняется (DS-009).
//
// Ссылки из услуг, заказов и журнала склада не читаются — удалению не мешают
// (DS-014).
func (r *ProductRepo) SoftDelete(ctx context.Context, id, userID, reason string) (time.Time, error) {
	var at time.Time
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return at, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var hasStock bool
	if err := tx.QueryRow(ctx, `SELECT stock <> 0 FROM products WHERE id = $1::uuid AND deleted_at IS NULL FOR NO KEY UPDATE`, id).Scan(&hasStock); err != nil {
		return at, err // pgx.ErrNoRows == ErrProductNotFound
	}
	if hasStock {
		return at, errProductHasStock
	}
	err = tx.QueryRow(ctx, `
UPDATE products SET deleted_at = now(), deleted_by = $2::uuid, delete_reason = $3, updated_at = now()
WHERE id = $1::uuid AND deleted_at IS NULL
RETURNING deleted_at`, id, userID, reason).Scan(&at)
	if err != nil {
		return at, err
	}
	return at.UTC(), tx.Commit(ctx)
}

// softDeleteRow — мягкое удаление записи без собственных запретов (услуга, категория): один
// UPDATE с условием «ещё не удалена»; 0 строк → pgx.ErrNoRows (404 вида; второй одновременный
// запрос — тоже 404, DS-024). is_active не меняется (DS-009).
func softDeleteRow(ctx context.Context, q interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}, table, id, userID, reason string) (time.Time, error) {
	var at time.Time
	err := q.QueryRow(ctx, `
UPDATE `+table+` SET deleted_at = now(), deleted_by = $2::uuid, delete_reason = $3, updated_at = now()
WHERE id = $1::uuid AND deleted_at IS NULL
RETURNING deleted_at`, id, userID, reason).Scan(&at)
	return at.UTC(), err
}

// restoreRow — восстановление записи: один UPDATE с условием «удалена» стирает три поля
// удаления, is_active остаётся прежним (DS-009). 0 строк: записи нет → pgx.ErrNoRows, запись
// действует → errNotDeleted (повтор восстановления, DS-024). Нарушение частичного уникального
// индекса названия (категория) → errNameTaken (DS-017): уникальность держит индекс, поэтому
// одновременные «создать/переименовать» и «восстановить» с одним названием дают ровно один
// успех.
func restoreRow(ctx context.Context, q interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}, table, id string) error {
	tag, err := q.Exec(ctx, `
UPDATE `+table+` SET deleted_at = NULL, deleted_by = NULL, delete_reason = NULL, updated_at = now()
WHERE id = $1::uuid AND deleted_at IS NOT NULL`, id)
	if err != nil {
		return mapUniqueViolation(err, errNameTaken)
	}
	if tag.RowsAffected() > 0 {
		return nil
	}
	var one int
	if err := q.QueryRow(ctx, `SELECT 1 FROM `+table+` WHERE id = $1::uuid`, id).Scan(&one); err != nil {
		return err // pgx.ErrNoRows — записи нет
	}
	return errNotDeleted
}

// Restore — восстановление товара (§6.2). За время удаления остаток мог появиться (возврат из
// заказа — DS-042), категория — уйти в удалённые (DS-014): ни то, ни другое
// восстановлению не мешает и перепроверки не требует.
func (r *ProductRepo) Restore(ctx context.Context, id string) error {
	return restoreRow(ctx, r.pool, "products", id)
}

// SoftDelete — мягкое удаление услуги каталога (§6.3): заказы и состав не мешают (DS-014).
func (r *ServiceRepo) SoftDelete(ctx context.Context, id, userID, reason string) (time.Time, error) {
	return softDeleteRow(ctx, r.pool, "services", id, userID, reason)
}

// Restore — восстановление услуги каталога (§6.4); товары её состава и категория могли уйти в
// удалённые — услуга возвращается с бейджами (DS-014).
func (r *ServiceRepo) Restore(ctx context.Context, id string) error {
	return restoreRow(ctx, r.pool, "services", id)
}

// SoftDelete — мягкое удаление категории (§6.5): товары и услуги категории остаются
// действующими (DS-014); название освобождается частичным уникальным индексом (DS-017).
func (r *CategoryRepo) SoftDelete(ctx context.Context, id, userID, reason string) (time.Time, error) {
	return softDeleteRow(ctx, r.pool, "catalog_categories", id, userID, reason)
}

// Restore — восстановление категории (§6.6): название занято действующей категорией того же
// вида → errNameTaken (частичный уникальный индекс).
func (r *CategoryRepo) Restore(ctx context.Context, id string) error {
	return restoreRow(ctx, r.pool, "catalog_categories", id)
}

// ---- service ----

func forbidden() error { return errorspkg.Forbidden("Нет доступа") }

// softDelete — общий каркас удаления записи каталога: роль проверяется первой, до поиска
// записи (DS-003); причина обязательна (DS-020); факт — в журнал, причина не пишется (DS-030).
func softDelete(ctx context.Context, a Actor, kind softdelete.Kind, id string, in softdelete.DeleteRequest,
	notFound *errorspkg.AppError, do func(reason string) (time.Time, error)) (*DeleteResponse, error) {
	if !a.IsAdmin {
		return nil, forbidden()
	}
	if uuid.Validate(id) != nil {
		return nil, notFound
	}
	reason, fields := softdelete.ValidateReason(in.Reason)
	if len(fields) > 0 {
		return nil, errorspkg.Validation(fields...)
	}
	at, err := do(reason)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil, notFound
	case errors.Is(err, errProductHasStock):
		return nil, ErrProductHasStock
	case err != nil:
		return nil, err
	}
	slog.InfoContext(ctx, "soft delete", "kind", string(kind), "id", id, "user_id", a.ID, "action", "delete")
	return &DeleteResponse{ID: id, DeletedAt: at}, nil
}

// restore — общий каркас восстановления: только admin; нет записи → 404 вида; не удалена →
// 409 NOT_DELETED; название занято → 409 NAME_TAKEN.
func restore(ctx context.Context, a Actor, kind softdelete.Kind, id string, notFound *errorspkg.AppError, do func() error) (*RestoreResponse, error) {
	if !a.IsAdmin {
		return nil, forbidden()
	}
	if uuid.Validate(id) != nil {
		return nil, notFound
	}
	err := do()
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil, notFound
	case errors.Is(err, errNotDeleted):
		return nil, softdelete.ErrNotDeleted
	case errors.Is(err, errNameTaken):
		return nil, softdelete.ErrNameTaken
	case err != nil:
		return nil, err
	}
	slog.InfoContext(ctx, "soft delete", "kind", string(kind), "id", id, "user_id", a.ID, "action", "restore")
	return &RestoreResponse{ID: id}, nil
}

// purge — служебная уборка: вид записи не раскрывается (404 NOT_FOUND); запись используется →
// прежний код отказа своего вида (as-is коды остаются только здесь, §6.9).
func purge(id string, inUseErr error, inUse *errorspkg.AppError, do func() error) error {
	if uuid.Validate(id) != nil {
		return errorspkg.NotFound("Не найдено")
	}
	err := do()
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return errorspkg.NotFound("Не найдено")
	case errors.Is(err, inUseErr):
		return inUse
	}
	return err
}

// Delete — МЯГКОЕ удаление товара (§6.1): только admin; остаток не равен нулю → 409
// PRODUCT_HAS_STOCK; ссылки не мешают (отказ PRODUCT_IN_USE снят, DS-014).
func (s *ProductService) Delete(ctx context.Context, a Actor, id string, in softdelete.DeleteRequest) (*DeleteResponse, error) {
	return softDelete(ctx, a, softdelete.KindProducts, id, in, ErrProductNotFoundErr, func(reason string) (time.Time, error) {
		return s.repo.SoftDelete(ctx, id, a.ID, reason)
	})
}

// Restore — восстановление товара (§6.2): признак архива прежний (DS-009).
func (s *ProductService) Restore(ctx context.Context, a Actor, id string) (*RestoreResponse, error) {
	return restore(ctx, a, softdelete.KindProducts, id, ErrProductNotFoundErr, func() error { return s.repo.Restore(ctx, id) })
}

// Purge — вид products служебного адреса DELETE /api/v1/test/purge/:kind/:id (роут и гард
// admin — в пакете board, существует только при TEST_PURGE_ENABLED=true, DS-027).
func (s *ProductService) Purge(ctx context.Context, id string) error {
	return purge(id, errProductInUse, ErrProductInUse, func() error { return s.repo.Purge(ctx, id) })
}

// Delete — МЯГКОЕ удаление услуги каталога (§6.3): заказы не мешают (отказ SERVICE_IN_USE снят).
func (s *ServiceService) Delete(ctx context.Context, a Actor, id string, in softdelete.DeleteRequest) (*DeleteResponse, error) {
	return softDelete(ctx, a, softdelete.KindServices, id, in, ErrServiceNotFoundErr, func(reason string) (time.Time, error) {
		return s.repo.SoftDelete(ctx, id, a.ID, reason)
	})
}

// Restore — восстановление услуги каталога (§6.4).
func (s *ServiceService) Restore(ctx context.Context, a Actor, id string) (*RestoreResponse, error) {
	return restore(ctx, a, softdelete.KindServices, id, ErrServiceNotFoundErr, func() error { return s.repo.Restore(ctx, id) })
}

// Purge — вид services служебного адреса уборки.
func (s *ServiceService) Purge(ctx context.Context, id string) error {
	return purge(id, errServiceInUse, ErrServiceInUse, func() error { return s.repo.Purge(ctx, id) })
}

// Delete — МЯГКОЕ удаление категории (§6.5): товары и услуги не мешают (отказ CATEGORY_IN_USE
// снят) и остаются действующими с пометкой «категория удалена».
func (s *CategoryService) Delete(ctx context.Context, a Actor, id string, in softdelete.DeleteRequest) (*DeleteResponse, error) {
	return softDelete(ctx, a, softdelete.KindCatalogCategories, id, in, ErrCategoryNotFoundErr, func(reason string) (time.Time, error) {
		return s.repo.SoftDelete(ctx, id, a.ID, reason)
	})
}

// Restore — восстановление категории (§6.6): название занято действующей категорией того же
// вида → 409 NAME_TAKEN (DS-017).
func (s *CategoryService) Restore(ctx context.Context, a Actor, id string) (*RestoreResponse, error) {
	return restore(ctx, a, softdelete.KindCatalogCategories, id, ErrCategoryNotFoundErr, func() error { return s.repo.Restore(ctx, id) })
}

// Purge — вид catalog-categories служебного адреса уборки.
func (s *CategoryService) Purge(ctx context.Context, id string) error {
	return purge(id, errCategoryInUse, ErrCategoryInUse, func() error { return s.repo.Purge(ctx, id) })
}

// ---- handlers ----

// currentActor — Actor из middleware.CurrentPrincipal (роут закрыт RequireAPI/RequireSSR —
// principal всегда не nil).
func currentActor(c fiber.Ctx) Actor {
	p := middleware.CurrentPrincipal(c)
	return Actor{ID: p.ID, IsAdmin: p.IsAdmin()}
}

func jsonOr(c fiber.Ctx, resp any, err error) error {
	if err != nil {
		return err
	}
	return c.JSON(resp)
}

// Delete — DELETE /api/v1/products/:id (admin), тело {reason}: 200 + id, deleted_at (§6.1).
func (h *ProductHandler) Delete(c fiber.Ctx) error {
	resp, err := h.svc.Delete(c.Context(), currentActor(c), c.Params("id"), softdelete.ParseDeleteRequest(c.Body()))
	return jsonOr(c, resp, err)
}

// Restore — POST /api/v1/products/:id/restore (admin), тела нет (§6.2).
func (h *ProductHandler) Restore(c fiber.Ctx) error {
	resp, err := h.svc.Restore(c.Context(), currentActor(c), c.Params("id"))
	return jsonOr(c, resp, err)
}

// Delete — DELETE /api/v1/services/:id (admin), тело {reason} (§6.3).
func (h *ServiceHandler) Delete(c fiber.Ctx) error {
	resp, err := h.svc.Delete(c.Context(), currentActor(c), c.Params("id"), softdelete.ParseDeleteRequest(c.Body()))
	return jsonOr(c, resp, err)
}

// Restore — POST /api/v1/services/:id/restore (admin), тела нет (§6.4).
func (h *ServiceHandler) Restore(c fiber.Ctx) error {
	resp, err := h.svc.Restore(c.Context(), currentActor(c), c.Params("id"))
	return jsonOr(c, resp, err)
}

// Delete — DELETE /api/v1/catalog-categories/:id (admin), тело {reason} (§6.5).
func (h *CategoryHandler) Delete(c fiber.Ctx) error {
	resp, err := h.svc.Delete(c.Context(), currentActor(c), c.Params("id"), softdelete.ParseDeleteRequest(c.Body()))
	return jsonOr(c, resp, err)
}

// Restore — POST /api/v1/catalog-categories/:id/restore (admin), тела нет (§6.6).
func (h *CategoryHandler) Restore(c fiber.Ctx) error {
	resp, err := h.svc.Restore(c.Context(), currentActor(c), c.Params("id"))
	return jsonOr(c, resp, err)
}
