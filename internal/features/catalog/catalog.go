// Package catalog — фаза 4: категории и товары каталога (услуги/комплект — подфаза B).
package catalog

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Root — шаблоны экрана «Каталог» (товары/категории/заглушка услуг) относительно корня модуля.
const Root = "web/ssr/catalog/templates"

// TabCounts — счётчики вкладок каталога (Услуги/Товары): считаются по активным записям
// (архивные не в счёт).
type TabCounts struct {
	Services int
	Products int
}

// LoadTabCounts — счётчики вкладок одним запросом.
func LoadTabCounts(ctx context.Context, pool *pgxpool.Pool) (TabCounts, error) {
	var t TabCounts
	err := pool.QueryRow(ctx, `SELECT
		(SELECT count(*) FROM services WHERE is_active AND deleted_at IS NULL),
		(SELECT count(*) FROM products WHERE is_active AND deleted_at IS NULL)`).
		Scan(&t.Services, &t.Products)
	return t, err
}
