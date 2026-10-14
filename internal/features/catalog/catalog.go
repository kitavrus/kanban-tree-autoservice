// Package catalog — фаза 4: категории и товары каталога (услуги/комплект — подфаза B).
package catalog

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Root — шаблоны экрана «Каталог» (товары/категории/заглушка услуг) относительно корня модуля.
const Root = "web/ssr/catalog/templates"

// TabCounts — счётчики трёх вкладок каталога (Услуги/Товары/Типы оборудования): считаются
// по активным записям (архивные не в счёт, как в панели категорий equipmenttypes).
type TabCounts struct {
	Services int
	Products int
	Types    int
}

// LoadTabCounts — общая полоса вкладок нужна и экрану типов оборудования (фаза 3),
// поэтому счётчики считаются одним запросом без зависимости от пакета equipmenttypes
// (нет обратного импорта catalog → equipmenttypes).
func LoadTabCounts(ctx context.Context, pool *pgxpool.Pool) (TabCounts, error) {
	var t TabCounts
	err := pool.QueryRow(ctx, `SELECT
		(SELECT count(*) FROM services WHERE is_active AND deleted_at IS NULL),
		(SELECT count(*) FROM products WHERE is_active AND deleted_at IS NULL),
		(SELECT count(*) FROM equipment_types WHERE is_active AND deleted_at IS NULL)`).
		Scan(&t.Services, &t.Products, &t.Types)
	return t, err
}
