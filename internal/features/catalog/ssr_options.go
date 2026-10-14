package catalog

import (
	"context"
	"errors"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/kitavrus/kanban-tree/internal/features/softdelete"
)

// Опции выбора в формах каталога (soft-delete-catalog §8.1): в выборе нет мягко удалённых
// категорий и товаров; уже выбранная в записи удалённая категория (товар строки состава)
// показывается текущим значением с бейджем «удалена» («удалён») и повторно не предлагается.

// CategoryOptionVM — опция <select> категории формы товара/услуги.
type CategoryOptionVM struct {
	ID        string
	Name      string
	IsActive  bool
	IsDeleted bool
}

// CatDeleteVM — отказ удаления категории по причине: окно «Удаление» строки категории ID
// открывается заново с введённым текстом Reason и ошибкой Error под полем (§8.1).
type CatDeleteVM struct {
	ID     string
	Reason string
	Error  string
}

// catDeleteLocal — ключ c.Locals: categoryRemove передаёт отказ по причине экрану списка.
const catDeleteLocal = "catalog.cat_delete"

func catDeleteFrom(c fiber.Ctx) CatDeleteVM {
	vm, _ := c.Locals(catDeleteLocal).(CatDeleteVM)
	return vm
}

// toastOf — flash мягкого удаления → тост после PRG-перехода (§8.1); прочие flash — alert.
func toastOf(flash string) string {
	switch flash {
	case "deleted", "service_deleted", "category_deleted":
		return softdelete.ToastDeleted
	}
	return ""
}

// AnyByID — категория по id независимо от отметки удаления (текущее значение формы записи,
// чья категория уже удалена). Нет строки → pgx.ErrNoRows.
func (r *CategoryRepo) AnyByID(ctx context.Context, id, kind string) (CategoryOptionVM, error) {
	o := CategoryOptionVM{ID: id}
	err := r.pool.QueryRow(ctx, `SELECT name, is_active, deleted_at IS NOT NULL FROM catalog_categories WHERE id = $1::uuid AND kind = $2`,
		id, kind).Scan(&o.Name, &o.IsActive, &o.IsDeleted)
	return o, err
}

// categoryOptions — активные действующие категории вида kind + текущая категория записи
// currentID, даже если она в архиве; удалённая текущая добавляется только при keepDeleted
// (форма существующей записи) — в форме новой записи удалённая категория не предлагается.
//
// pagination DS-006: TruncVM — активных категорий больше defaultLimit → пометка под select.
func (h *SSR) categoryOptions(c fiber.Ctx, kind, currentID string, keepDeleted bool) ([]CategoryOptionVM, TruncVM, error) {
	resp, err := h.catSvc.List(c.Context(), CategoryListQuery{Kind: kind, IncludeArchived: false, Limit: defaultLimit}, true)
	if err != nil {
		return nil, TruncVM{}, err
	}
	opts := make([]CategoryOptionVM, 0, len(resp.Items)+1)
	found := currentID == ""
	for _, o := range resp.Items {
		opts = append(opts, CategoryOptionVM{ID: o.ID, Name: o.Name, IsActive: o.IsActive})
		found = found || o.ID == currentID
	}
	trunc := TruncVM{Shown: len(resp.Items), Total: resp.Total}
	if found || !isUUID(currentID) {
		return opts, trunc, nil
	}
	cur, err := h.catSvc.repo.AnyByID(c.Context(), currentID, kind)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && cur.IsDeleted && !keepDeleted) {
		return opts, trunc, nil
	}
	if err != nil {
		return nil, TruncVM{}, err
	}
	return append([]CategoryOptionVM{cur}, opts...), trunc, nil
}

// currentCategoryDeleted — выбранная в форме категория — удалённая (бейдж «удалена» у поля).
func currentCategoryDeleted(opts []CategoryOptionVM, currentID string) bool {
	for _, o := range opts {
		if o.ID == currentID {
			return o.IsDeleted
		}
	}
	return false
}

// OptionsByIDs — товары по id независимо от архива и отметки удаления (уже выбранные строки
// состава услуги): название и подпись остаются видны, удалённый — с признаком.
func (r *ProductRepo) OptionsByIDs(ctx context.Context, ids []string) ([]ProductOptionVM, error) {
	out := []ProductOptionVM{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := r.pool.Query(ctx, `SELECT id::text, name, sku, unit, price::text, is_active, deleted_at IS NOT NULL
		FROM products WHERE id = ANY($1::uuid[]) ORDER BY lower(name), id`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var o ProductOptionVM
		var sku, unit, price string
		if err := rows.Scan(&o.ID, &o.Name, &sku, &unit, &price, &o.IsActive, &o.IsDeleted); err != nil {
			return nil, err
		}
		o.Hint = productHint(sku, unit, price)
		out = append(out, o)
	}
	return out, rows.Err()
}

func isUUID(s string) bool { return uuid.Validate(s) == nil }

// productHint — подпись опции товара: «артикул · цена ₽/ед.».
func productHint(sku, unit, price string) string {
	hint := price + " ₽"
	if unit != "" {
		hint += "/" + unit
	}
	if sku != "" {
		hint = sku + " · " + hint
	}
	return hint
}
