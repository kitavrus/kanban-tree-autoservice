//go:build integration

package repository_test

import (
	"context"
	"errors"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/stretchr/testify/suite"

	"github.com/kitavrus/kanban-tree/pkg/testutil"
)

// PurchaseOrdersMigrationSuite — миграция 0019 (docs/features/purchase-orders/draft-spec.md §4,
// TS-008): up/down на чистой БД; после up — роль supplier ⇔ supplier_id (users_supplier_chk),
// Базар не бывает sent/ready, номер — из sequence; down убирает таблицы и роль.
type PurchaseOrdersMigrationSuite struct {
	testutil.BaseSuite
}

func TestPurchaseOrdersMigrationSuite(t *testing.T) {
	suite.Run(t, new(PurchaseOrdersMigrationSuite))
}

func migrationsURL() string {
	_, f, _, _ := runtime.Caller(0)
	return "file://" + filepath.Join(filepath.Dir(f), "..", "..", "..", "..", "migrations")
}

func (s *PurchaseOrdersMigrationSuite) tableExists(name string) bool {
	var ok bool
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = $1)`, name).Scan(&ok))
	return ok
}

func (s *PurchaseOrdersMigrationSuite) TestTS008_Migration0019_UpDown() {
	ctx := context.Background()
	mig, err := migrate.New(migrationsURL(), s.DSN)
	s.Require().NoError(err)
	defer func() { _, _ = mig.Close() }()

	s.Require().True(s.tableExists("purchase_orders"))
	var supplierID, adminID string
	s.Require().NoError(s.Pool.QueryRow(ctx, `INSERT INTO suppliers (name) VALUES ('Миграция') RETURNING id::text`).Scan(&supplierID))
	s.Require().NoError(s.Pool.QueryRow(ctx,
		`INSERT INTO users (login, password_hash, name, role) VALUES ('po-mig-admin', 'x', 'А', 'admin') RETURNING id::text`).Scan(&adminID))

	// CHECK users_supplier_chk (DS-007).
	_, err = s.Pool.Exec(ctx, `INSERT INTO users (login, password_hash, name, role) VALUES ('po-mig-1', 'x', 'x', 'supplier')`)
	s.Error(err, "supplier без supplier_id")
	_, err = s.Pool.Exec(ctx, `INSERT INTO users (login, password_hash, name, role, supplier_id) VALUES ('po-mig-2', 'x', 'x', 'executor', $1::uuid)`, supplierID)
	s.Error(err, "executor с supplier_id")
	_, err = s.Pool.Exec(ctx, `INSERT INTO users (login, password_hash, name, role, supplier_id) VALUES ('po-mig-3', 'x', 'x', 'supplier', $1::uuid)`, supplierID)
	s.NoError(err)
	_, err = s.Pool.Exec(ctx, `INSERT INTO users (login, password_hash, name, role) VALUES ('po-mig-4', 'x', 'x', 'root')`)
	s.Error(err, "роль вне списка")

	// CHECK purchase_orders_bazaar_chk, cancel_chk; sequence.
	_, err = s.Pool.Exec(ctx, `INSERT INTO purchase_orders (created_by, status) VALUES ($1::uuid, 'ready')`, adminID)
	s.Error(err, "Базар в ready")
	_, err = s.Pool.Exec(ctx, `INSERT INTO purchase_orders (created_by, status) VALUES ($1::uuid, 'cancelled')`, adminID)
	s.Error(err, "cancelled без причины")
	var n1, n2 int64
	s.Require().NoError(s.Pool.QueryRow(ctx, `INSERT INTO purchase_orders (created_by) VALUES ($1::uuid) RETURNING number`, adminID).Scan(&n1))
	s.Require().NoError(s.Pool.QueryRow(ctx, `INSERT INTO purchase_orders (created_by) VALUES ($1::uuid) RETURNING number`, adminID).Scan(&n2))
	s.Equal(n1+1, n2)
	_, err = s.Pool.Exec(ctx, `UPDATE purchase_orders SET deleted_at = now(), deleted_by = $1::uuid, delete_reason = 'x', status = 'cancelled',
		cancelled_at = now(), cancelled_by = $1::uuid, cancel_reason = 'y' WHERE number = $2`, adminID, n1)
	s.Error(err, "удалённым может быть только черновик")

	// down → таблиц и роли нет; повторный up — чистый.
	s.Require().NoError(mig.Migrate(18))
	s.False(s.tableExists("purchase_orders"))
	s.False(s.tableExists("purchase_order_items"))
	_, err = s.Pool.Exec(ctx, `INSERT INTO users (login, password_hash, name, role) VALUES ('po-mig-5', 'x', 'x', 'supplier')`)
	s.Error(err, "после down роли supplier нет")
	if err := mig.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		s.Require().NoError(err)
	}
	s.True(s.tableExists("purchase_orders"))
	var n3 int64
	s.Require().NoError(s.Pool.QueryRow(ctx, `INSERT INTO purchase_orders (created_by) VALUES ($1::uuid) RETURNING number`, adminID).Scan(&n3))
	s.EqualValues(1, n3, "sequence создана заново")
}
