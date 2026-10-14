package softdelete

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Источники вкладок «Товары», «Услуги», «Категории», «Типы оборудования» раздела «Удалённые»
// (часть 5 soft-delete-catalog §6.9, §8.2). Таблицы читаются напрямую: softdelete — reader,
// отметку удаления пишут пакеты catalog и equipmenttypes. Строка у всех четырёх видов одной
// формы: title — название, subtitle — по виду, is_archived = true у записи, удалённой из архива
// (DS-009: признак архива удалением не меняется).

// CatalogSource — удалённые записи одной таблицы каталога.
type CatalogSource struct {
	pool *pgxpool.Pool
	kind Kind
	// table — таблица вида (алиас r); subtitleSQL — выражение подзаголовка; joinSQL — соединения
	// для него (без условия «не удалён»: категория записи сама может быть в удалённых).
	table, subtitleSQL, joinSQL string
}

// CategoryKindSubtitle — подзаголовок строки удалённой категории: «товары» либо «услуги».
func CategoryKindSubtitle(kind string) string {
	if kind == "product" {
		return "товары"
	}
	return "услуги"
}

// NewProductsSource — вкладка «Товары»: subtitle — название категории товара.
func NewProductsSource(pool *pgxpool.Pool) *CatalogSource {
	return &CatalogSource{pool: pool, kind: KindProducts, table: "products",
		subtitleSQL: "cc.name", joinSQL: "JOIN catalog_categories cc ON cc.id = r.category_id"}
}

// NewServicesSource — вкладка «Услуги»: subtitle — название категории услуги.
func NewServicesSource(pool *pgxpool.Pool) *CatalogSource {
	return &CatalogSource{pool: pool, kind: KindServices, table: "services",
		subtitleSQL: "cc.name", joinSQL: "JOIN catalog_categories cc ON cc.id = r.category_id"}
}

// NewCatalogCategoriesSource — вкладка «Категории»: subtitle — «товары» либо «услуги».
func NewCatalogCategoriesSource(pool *pgxpool.Pool) *CatalogSource {
	return &CatalogSource{pool: pool, kind: KindCatalogCategories, table: "catalog_categories",
		subtitleSQL: "CASE r.kind WHEN 'product' THEN '" + CategoryKindSubtitle("product") + "' ELSE '" + CategoryKindSubtitle("service") + "' END"}
}

// NewEquipmentTypesSource — вкладка «Типы оборудования»: subtitle — пустая строка.
func NewEquipmentTypesSource(pool *pgxpool.Pool) *CatalogSource {
	return &CatalogSource{pool: pool, kind: KindEquipmentTypes, table: "equipment_types", subtitleSQL: "''"}
}

// NewWarehousesSource — вкладка «Склады» (часть 6 soft-delete-warehouses-suppliers §6.5, §8.2):
// subtitle — пустая строка. Строка той же формы, что у каталога: название, признак архива.
func NewWarehousesSource(pool *pgxpool.Pool) *CatalogSource {
	return &CatalogSource{pool: pool, kind: KindWarehouses, table: "warehouses", subtitleSQL: "''"}
}

// NewSuppliersSource — вкладка «Поставщики» (часть 6): subtitle — контактное лицо.
func NewSuppliersSource(pool *pgxpool.Pool) *CatalogSource {
	return &CatalogSource{pool: pool, kind: KindSuppliers, table: "suppliers", subtitleSQL: "r.contact_name"}
}

// whereSQL — фильтр вкладки: $1 — строка поиска (название, причина; уже экранирована), $2/$3 —
// период [с; по), $4 — кто удалил. Номера у записей каталога нет. Частичный индекс
// {table}_deleted_at_idx покрывает условие и сортировку.
func (s *CatalogSource) whereSQL() string {
	return `
FROM ` + s.table + ` r
` + s.joinSQL + `
JOIN users u ON u.id = r.deleted_by
WHERE r.deleted_at IS NOT NULL
  AND ($1::text IS NULL OR r.name ILIKE '%' || $1 || '%' ESCAPE '\'
       OR r.delete_reason ILIKE '%' || $1 || '%' ESCAPE '\')
  AND ($2::timestamptz IS NULL OR r.deleted_at >= $2)
  AND ($3::timestamptz IS NULL OR r.deleted_at < $3)
  AND ($4::uuid IS NULL OR r.deleted_by = $4)`
}

func (s *CatalogSource) List(ctx context.Context, f Filter) ([]Item, int, error) {
	args := filterArgs(f)
	where := s.whereSQL()

	var total int
	if err := s.pool.QueryRow(ctx, `SELECT count(*)`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := s.pool.Query(ctx, `
SELECT r.id::text, r.name, `+s.subtitleSQL+`, NOT r.is_active, r.deleted_at, u.id::text,
       coalesce(nullif(u.name, ''), u.login), r.delete_reason`+where+`
ORDER BY r.deleted_at DESC, r.id
LIMIT $5 OFFSET $6`, append(args, f.Limit, f.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := []Item{}
	for rows.Next() {
		it := Item{Kind: string(s.kind)}
		if err := rows.Scan(&it.ID, &it.Title, &it.Subtitle, &it.IsArchived, &it.DeletedAt, &it.DeletedBy.ID, &it.DeletedBy.Name, &it.DeleteReason); err != nil {
			return nil, 0, err
		}
		it.DeletedAt = it.DeletedAt.UTC()
		out = append(out, it)
	}
	return out, total, rows.Err()
}

func (s *CatalogSource) Count(ctx context.Context) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM `+s.table+` WHERE deleted_at IS NOT NULL`).Scan(&n)
	return n, err
}
