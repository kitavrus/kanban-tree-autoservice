//go:build integration

package transfers_test

import (
	"context"
	"errors"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	"github.com/stretchr/testify/suite"

	"github.com/kitavrus/kanban-tree/pkg/testutil"
)

// TransfersSalesMigrationSuite — миграция 0016 (transfers-sales §4.1, DS-011): цена продажи
// transfer_items.price (существующие строки → NULL), CHECK «у строки-единицы цены нет» и
// «цена ≥ 0»; down возвращает модель 0015 — товарные строки накладных клиенту удаляются.
type TransfersSalesMigrationSuite struct {
	testutil.BaseSuite
}

func TestTransfersSalesMigrationSuite(t *testing.T) {
	suite.Run(t, new(TransfersSalesMigrationSuite))
}

const tsmAdminID = "aaaaaaaa-0016-0016-0016-000000000001"

func (s *TransfersSalesMigrationSuite) scalar(q string, args ...any) string {
	var v string
	s.Require().NoError(s.Pool.QueryRow(context.Background(), q, args...).Scan(&v), q)
	return v
}

func (s *TransfersSalesMigrationSuite) exec(q string, args ...any) {
	_, err := s.Pool.Exec(context.Background(), q, args...)
	s.Require().NoError(err, q)
}

func (s *TransfersSalesMigrationSuite) fails(name, q string, args ...any) {
	_, err := s.Pool.Exec(context.Background(), q, args...)
	s.Error(err, name)
}

func (s *TransfersSalesMigrationSuite) priceColumn() bool {
	return s.scalar(`SELECT count(*)::text FROM information_schema.columns
		WHERE table_name = 'transfer_items' AND column_name = 'price'`) == "1"
}

func (s *TransfersSalesMigrationSuite) constraint(name string) bool {
	return s.scalar(`SELECT count(*)::text FROM pg_constraint WHERE conname = $1`, name) == "1"
}

func (s *TransfersSalesMigrationSuite) TestMigration0016_UpDown() {
	ctx := context.Background()
	mig, err := migrate.New(migrationsURL(), s.DSN)
	s.Require().NoError(err)
	defer func() { _, _ = mig.Close() }()

	s.Require().NoError(mig.Migrate(15))
	defer func() {
		if err := mig.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			s.Require().NoError(err)
		}
	}()
	s.False(s.priceColumn(), "0015: колонки price нет")

	// ---- данные «до» (0015): склад → склад с товаром, возврат клиенту с единицей ----
	s.exec(`INSERT INTO users (id, login, password_hash, name, role) VALUES ($1, 'tsm-admin', 'x', 'Admin', 'admin') ON CONFLICT (id) DO NOTHING`, tsmAdminID)
	almaty := s.scalar(`SELECT id::text FROM warehouses WHERE name = 'Алматы'`)
	moscow := s.scalar(`SELECT id::text FROM warehouses WHERE name = 'Москва'`)
	clientID := s.scalar(`INSERT INTO clients (company_name) VALUES ('TSM ' || gen_random_uuid()) RETURNING id::text`)
	typeID := s.scalar(`INSERT INTO equipment_types (name) VALUES ('TSM ' || gen_random_uuid()) RETURNING id::text`)
	catID := s.scalar(`INSERT INTO catalog_categories (kind, name) VALUES ('product', 'TSM ' || gen_random_uuid()) RETURNING id::text`)
	productID := s.scalar(`INSERT INTO products (category_id, name, price) VALUES ($1, 'TSM товар', 150) RETURNING id::text`, catID)
	var taskID, w2w, back int64
	s.Require().NoError(s.Pool.QueryRow(ctx, `
		INSERT INTO tasks (column_id, client_id, title, position, created_by)
		VALUES ((SELECT id FROM columns ORDER BY position, id LIMIT 1), $1, 'TSM заказ', 1, $2) RETURNING id`, clientID, tsmAdminID).Scan(&taskID))
	// документы заказа держат его FK RESTRICT — убираются перед заказом.
	defer s.exec(`DELETE FROM tasks WHERE id = $1`, taskID)
	defer s.exec(`DELETE FROM transfers WHERE task_id = $1`, taskID)
	unit := s.scalar(`INSERT INTO equipment (client_id, type_id, model) VALUES ($1, $2, 'TSM unit') RETURNING id::text`, clientID, typeID)

	s.Require().NoError(s.Pool.QueryRow(ctx, `
		INSERT INTO transfers (direction, from_warehouse_id, to_warehouse_id, created_by)
		VALUES ('warehouse_to_warehouse', $1, $2, $3) RETURNING id`, almaty, moscow, tsmAdminID).Scan(&w2w))
	s.exec(`INSERT INTO transfer_items (transfer_id, kind, product_id, position, qty_sent) VALUES ($1, 'product', $2, 1, 4)`, w2w, productID)
	s.Require().NoError(s.Pool.QueryRow(ctx, `
		INSERT INTO transfers (direction, from_warehouse_id, task_id, client_id, created_by)
		VALUES ('warehouse_to_client', $1, $2, $3, $4) RETURNING id`, almaty, taskID, clientID, tsmAdminID).Scan(&back))
	s.exec(`INSERT INTO transfer_items (transfer_id, kind, equipment_id, position) VALUES ($1, 'equipment', $2, 1)`, back, unit)

	// ---- up → 0016 ----
	s.Require().NoError(mig.Migrate(16))
	s.True(s.priceColumn())
	s.Equal("numeric|12|2|YES", s.scalar(`
		SELECT data_type || '|' || numeric_precision || '|' || numeric_scale || '|' || is_nullable
		FROM information_schema.columns WHERE table_name = 'transfer_items' AND column_name = 'price'`))
	s.Equal("2", s.scalar(`SELECT count(*)::text FROM transfer_items WHERE transfer_id = ANY($1) AND price IS NULL`, []int64{w2w, back}),
		"существующие строки → price NULL")
	s.True(s.constraint("transfer_items_kind_check"))
	s.True(s.constraint("transfer_items_price_check"))

	// цена — у товарной строки, ≥ 0, два знака; у строки-единицы — запрещена.
	s.exec(`INSERT INTO transfer_items (transfer_id, kind, product_id, position, qty_sent, price) VALUES ($1, 'product', $2, 2, 3, 18500.50)`, back, productID)
	s.Equal("18500.50", s.scalar(`SELECT price::text FROM transfer_items WHERE transfer_id = $1 AND kind = 'product'`, back))
	s.exec(`UPDATE transfer_items SET price = 0 WHERE transfer_id = $1 AND kind = 'product'`, back)
	s.fails("отрицательная цена", `UPDATE transfer_items SET price = -0.01 WHERE transfer_id = $1 AND kind = 'product'`, back)
	s.fails("цена у строки-единицы", `UPDATE transfer_items SET price = 1 WHERE transfer_id = $1 AND kind = 'equipment'`, back)
	s.fails("прежние правила kind_check: товар без количества",
		`INSERT INTO transfer_items (transfer_id, kind, product_id, position, price) VALUES ($1, 'product', $2, 3, 1)`, back, productID)
	s.fails("прежние правила kind_check: единица с количеством",
		`UPDATE transfer_items SET qty_sent = 1 WHERE transfer_id = $1 AND kind = 'equipment'`, back)

	// ---- down → 0015 ----
	s.Require().NoError(mig.Migrate(15))
	s.False(s.priceColumn())
	s.False(s.constraint("transfer_items_price_check"))
	s.True(s.constraint("transfer_items_kind_check"))
	s.Equal("equipment", s.scalar(`SELECT string_agg(kind, ',') FROM transfer_items WHERE transfer_id = $1`, back),
		"товарная строка накладной клиенту удалена, единица осталась")
	s.Equal("4.000", s.scalar(`SELECT qty_sent::text FROM transfer_items WHERE transfer_id = $1`, w2w), "склад → склад не тронут")
	s.fails("0015: единица с количеством", `UPDATE transfer_items SET qty_sent = 1 WHERE transfer_id = $1 AND kind = 'equipment'`, back)

	// повторный up поверх отката проходит (идемпотентность пары up/down).
	s.Require().NoError(mig.Migrate(16))
	s.True(s.priceColumn())
}
