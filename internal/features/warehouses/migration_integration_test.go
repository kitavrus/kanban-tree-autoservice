//go:build integration

package warehouses_test

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/stretchr/testify/suite"

	"github.com/kitavrus/kanban-tree/pkg/testutil"
)

// WarehousesMigrationSuite — TS-001: миграция 0008 (docs/features/warehouses/draft-spec.md
// DS-002/DS-005/DS-007). На 0007 заводится товар с остатком, строка товара услуги и
// task_stock/stock_movements; up → всё лежит на «Алматы», в products нет stock-колонок;
// down → products.stock/min_stock/box/place восстановлены.
type WarehousesMigrationSuite struct {
	testutil.BaseSuite
}

func TestWarehousesMigrationSuite(t *testing.T) {
	suite.Run(t, new(WarehousesMigrationSuite))
}

const (
	whmAdminID  = "aaaaaaaa-0008-0008-0008-000000000001"
	whmClientID = "bbbbbbbb-0008-0008-0008-000000000001"
	whmColumnID = "cccccccc-0008-0008-0008-000000000001"
)

func migrationsURL() string {
	_, f, _, _ := runtime.Caller(0)
	return "file://" + filepath.Join(filepath.Dir(f), "..", "..", "..", "migrations")
}

func (s *WarehousesMigrationSuite) columnExists(table, column string) bool {
	var ok bool
	s.Require().NoError(s.Pool.QueryRow(context.Background(), `
		SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = $1 AND column_name = $2)`,
		table, column).Scan(&ok))
	return ok
}

func (s *WarehousesMigrationSuite) TestTS001_UpMovesStockToAlmaty_DownRestores() {
	ctx := context.Background()
	mig, err := migrate.New(migrationsURL(), s.DSN)
	s.Require().NoError(err)
	defer func() { _, _ = mig.Close() }()

	s.Require().NoError(mig.Migrate(7))
	defer func() { s.Require().NoError(mig.Up()) }()

	// --- данные «до складов» (схема 0007) ---
	_, err = s.Pool.Exec(ctx, `INSERT INTO users (id, login, password_hash, name, role) VALUES ($1, 'whm-admin', 'x', 'Admin', 'admin')`, whmAdminID)
	s.Require().NoError(err)
	_, err = s.Pool.Exec(ctx, `INSERT INTO clients (id, company_name) VALUES ($1, 'WHM Co')`, whmClientID)
	s.Require().NoError(err)
	_, err = s.Pool.Exec(ctx, `INSERT INTO columns (id, name, color, is_final, consumes_stock, position) VALUES ($1, 'WHM', '#94A3B8', false, true, 99)`, whmColumnID)
	s.Require().NoError(err)
	var prodCat, svcCat, productID, serviceID, subtaskID string
	var taskID int64
	s.Require().NoError(s.Pool.QueryRow(ctx, `INSERT INTO catalog_categories (kind, name) VALUES ('product', 'WHM P') RETURNING id`).Scan(&prodCat))
	s.Require().NoError(s.Pool.QueryRow(ctx, `INSERT INTO catalog_categories (kind, name) VALUES ('service', 'WHM S') RETURNING id`).Scan(&svcCat))
	s.Require().NoError(s.Pool.QueryRow(ctx,
		`INSERT INTO products (category_id, name, stock, min_stock, box, place) VALUES ($1, 'WHM товар', 8.5, 2, 'К-1', 'Полка 3') RETURNING id`,
		prodCat).Scan(&productID))
	s.Require().NoError(s.Pool.QueryRow(ctx, `INSERT INTO services (category_id, name) VALUES ($1, 'WHM услуга') RETURNING id`, svcCat).Scan(&serviceID))
	s.Require().NoError(s.Pool.QueryRow(ctx,
		`INSERT INTO tasks (column_id, client_id, title, position, created_by) VALUES ($1, $2, 'WHM', 1024, $3) RETURNING id`,
		whmColumnID, whmClientID, whmAdminID).Scan(&taskID))
	s.Require().NoError(s.Pool.QueryRow(ctx, `INSERT INTO subtasks (task_id, service_id) VALUES ($1, $2) RETURNING id`, taskID, serviceID).Scan(&subtaskID))
	_, err = s.Pool.Exec(ctx, `INSERT INTO subtask_products (subtask_id, product_id, qty) VALUES ($1, $2, 1.5)`, subtaskID, productID)
	s.Require().NoError(err)
	_, err = s.Pool.Exec(ctx, `INSERT INTO task_stock (task_id, product_id, qty) VALUES ($1, $2, 1.5)`, taskID, productID)
	s.Require().NoError(err)
	_, err = s.Pool.Exec(ctx, `INSERT INTO stock_movements (product_id, task_id, qty_delta, reason, user_id) VALUES ($1, $2, -1.5, 'task_consume', $3)`,
		productID, taskID, whmAdminID)
	s.Require().NoError(err)

	// --- up 0008 ---
	s.Require().NoError(mig.Migrate(8))

	var almaty string
	s.Require().NoError(s.Pool.QueryRow(ctx, `SELECT id FROM warehouses WHERE name = 'Алматы' AND position = 1`).Scan(&almaty))
	var moscowPos int
	s.Require().NoError(s.Pool.QueryRow(ctx, `SELECT position FROM warehouses WHERE name = 'Москва'`).Scan(&moscowPos))
	s.Equal(2, moscowPos)

	var stock, minStock, box, place string
	s.Require().NoError(s.Pool.QueryRow(ctx,
		`SELECT stock::text, min_stock::text, box, place FROM product_stock WHERE product_id = $1 AND warehouse_id = $2`,
		productID, almaty).Scan(&stock, &minStock, &box, &place))
	s.Equal("8.500", stock)
	s.Equal("2.000", minStock)
	s.Equal("К-1", box)
	s.Equal("Полка 3", place)
	var rows int
	s.Require().NoError(s.Pool.QueryRow(ctx, `SELECT count(*) FROM product_stock WHERE product_id = $1`, productID).Scan(&rows))
	s.Equal(1, rows, "только строка «Алматы»")
	for _, c := range []string{"stock", "min_stock", "box", "place"} {
		s.False(s.columnExists("products", c), "products.%s удалена", c)
	}

	var spWh, tsWh, smWh string
	s.Require().NoError(s.Pool.QueryRow(ctx, `SELECT warehouse_id FROM subtask_products WHERE subtask_id = $1`, subtaskID).Scan(&spWh))
	s.Require().NoError(s.Pool.QueryRow(ctx, `SELECT warehouse_id FROM task_stock WHERE task_id = $1`, taskID).Scan(&tsWh))
	s.Require().NoError(s.Pool.QueryRow(ctx, `SELECT warehouse_id FROM stock_movements WHERE task_id = $1`, taskID).Scan(&smWh))
	s.Equal(almaty, spWh)
	s.Equal(almaty, tsWh)
	s.Equal(almaty, smWh)

	// новые reason-значения приняты CHECK'ом.
	_, err = s.Pool.Exec(ctx, `INSERT INTO stock_movements (product_id, warehouse_id, qty_delta, reason, user_id) VALUES ($1, $2, 1, 'transfer_in', $3)`,
		productID, almaty, whmAdminID)
	s.Require().NoError(err)
	// вставка без склада — DEFAULT default_warehouse_id() (D49).
	_, err = s.Pool.Exec(ctx, `INSERT INTO subtask_products (subtask_id, product_id, qty) VALUES ($1, $2, 1)`, subtaskID, productID)
	s.Require().NoError(err)

	// --- down → 0007 ---
	s.Require().NoError(mig.Migrate(7))
	for _, c := range []string{"stock", "min_stock", "box", "place"} {
		s.True(s.columnExists("products", c), "products.%s восстановлена", c)
	}
	s.Require().NoError(s.Pool.QueryRow(ctx, `SELECT stock::text, min_stock::text, box, place FROM products WHERE id = $1`, productID).
		Scan(&stock, &minStock, &box, &place))
	s.Equal("8.500", stock)
	s.Equal("2.000", minStock)
	s.Equal("К-1", box)
	s.Equal("Полка 3", place)
	var tsQty string
	s.Require().NoError(s.Pool.QueryRow(ctx, `SELECT qty::text FROM task_stock WHERE task_id = $1 AND product_id = $2`, taskID, productID).Scan(&tsQty))
	s.Equal("1.500", tsQty)
	s.False(s.columnExists("stock_movements", "warehouse_id"))
	s.False(s.columnExists("subtask_products", "warehouse_id"))
	var cnt int
	s.Require().NoError(s.Pool.QueryRow(ctx, `SELECT count(*) FROM stock_movements WHERE product_id = $1`, productID).Scan(&cnt))
	s.Equal(1, cnt, "transfer_* движения удалены откатом, task_consume сохранён")
	var whTable bool
	s.Require().NoError(s.Pool.QueryRow(ctx, `SELECT to_regclass('warehouses') IS NOT NULL`).Scan(&whTable))
	s.False(whTable)
}
