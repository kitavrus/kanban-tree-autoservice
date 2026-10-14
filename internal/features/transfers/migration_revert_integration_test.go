//go:build integration

package transfers_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	"github.com/stretchr/testify/suite"

	"github.com/kitavrus/kanban-tree/pkg/testutil"
)

// TransfersRevertMigrationSuite — миграция 0017 (transfers-revert §4.1, DS-044 / DS-053):
// сведения о последней отмене проведения на документе — unreceived_by (FK users RESTRICT),
// unreceived_at, unreceive_reason (NOT NULL, ” у документов без отката, CHECK ≤ 500 символов);
// down убирает колонки, документы и движения transfer_unreceive остаются.
type TransfersRevertMigrationSuite struct {
	testutil.BaseSuite
}

func TestTransfersRevertMigrationSuite(t *testing.T) {
	suite.Run(t, new(TransfersRevertMigrationSuite))
}

const trmAdminID = "aaaaaaaa-0017-0017-0017-000000000001"

func (s *TransfersRevertMigrationSuite) scalar(q string, args ...any) string {
	var v string
	s.Require().NoError(s.Pool.QueryRow(context.Background(), q, args...).Scan(&v), q)
	return v
}

func (s *TransfersRevertMigrationSuite) exec(q string, args ...any) {
	_, err := s.Pool.Exec(context.Background(), q, args...)
	s.Require().NoError(err, q)
}

func (s *TransfersRevertMigrationSuite) fails(name, q string, args ...any) {
	_, err := s.Pool.Exec(context.Background(), q, args...)
	s.Error(err, name)
}

// columns — «имя:тип:nullable» колонок отката таблицы transfers, по имени.
func (s *TransfersRevertMigrationSuite) columns() string {
	return s.scalar(`
		SELECT coalesce(string_agg(column_name || ':' || data_type || ':' || is_nullable, ',' ORDER BY column_name), '')
		FROM information_schema.columns
		WHERE table_name = 'transfers' AND column_name IN ('unreceived_by', 'unreceived_at', 'unreceive_reason')`)
}

func (s *TransfersRevertMigrationSuite) constraint(name string) bool {
	return s.scalar(`SELECT count(*)::text FROM pg_constraint WHERE conname = $1`, name) == "1"
}

func (s *TransfersRevertMigrationSuite) TestMigration0017_UpDown() {
	ctx := context.Background()
	mig, err := migrate.New(migrationsURL(), s.DSN)
	s.Require().NoError(err)
	defer func() { _, _ = mig.Close() }()

	s.Require().NoError(mig.Migrate(16))
	defer func() {
		if err := mig.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			s.Require().NoError(err)
		}
	}()
	s.Empty(s.columns(), "0016: колонок отката нет")

	// ---- данные «до» (0016): полученный приход-подобный документ склад → склад ----
	s.exec(`INSERT INTO users (id, login, password_hash, name, role) VALUES ($1, 'trm-admin', 'x', 'Admin', 'admin') ON CONFLICT (id) DO NOTHING`, trmAdminID)
	almaty := s.scalar(`SELECT id::text FROM warehouses WHERE name = 'Алматы'`)
	moscow := s.scalar(`SELECT id::text FROM warehouses WHERE name = 'Москва'`)
	catID := s.scalar(`INSERT INTO catalog_categories (kind, name) VALUES ('product', 'TRM ' || gen_random_uuid()) RETURNING id::text`)
	productID := s.scalar(`INSERT INTO products (category_id, name) VALUES ($1, 'TRM товар') RETURNING id::text`, catID)
	var doc int64
	s.Require().NoError(s.Pool.QueryRow(ctx, `
		INSERT INTO transfers (direction, from_warehouse_id, to_warehouse_id, status, created_by, sent_by, sent_at, received_by, received_at)
		VALUES ('warehouse_to_warehouse', $1, $2, 'received', $3, $3, now(), $3, now()) RETURNING id`, almaty, moscow, trmAdminID).Scan(&doc))
	defer s.exec(`DELETE FROM transfers WHERE id = $1`, doc)
	s.exec(`INSERT INTO transfer_items (transfer_id, kind, product_id, position, qty_sent, qty_received) VALUES ($1, 'product', $2, 1, 4, 4)`, doc, productID)

	// ---- up → 0017 ----
	s.Require().NoError(mig.Migrate(17))
	s.Equal("unreceive_reason:text:NO,unreceived_at:timestamp with time zone:YES,unreceived_by:uuid:YES", s.columns())
	s.True(s.constraint("transfers_unreceive_reason_check"))
	s.Equal("||", s.scalar(`
		SELECT coalesce(unreceived_by::text, '') || '|' || coalesce(unreceived_at::text, '') || '|' || unreceive_reason
		FROM transfers WHERE id = $1`, doc), "существующий документ: отката не было — NULL, NULL, ''")

	// причина — до 500 символов (символов, не байт); 501 — отказ CHECK.
	s.exec(`UPDATE transfers SET unreceived_by = $2, unreceived_at = now(), unreceive_reason = $3 WHERE id = $1`,
		doc, trmAdminID, strings.Repeat("я", 500))
	s.Equal("500", s.scalar(`SELECT char_length(unreceive_reason)::text FROM transfers WHERE id = $1`, doc))
	s.fails("причина 501 символ", `UPDATE transfers SET unreceive_reason = $2 WHERE id = $1`, doc, strings.Repeat("я", 501))
	s.fails("причина NULL", `UPDATE transfers SET unreceive_reason = NULL WHERE id = $1`, doc)
	// FK unreceived_by → users: несуществующий пользователь и удаление автора отката запрещены.
	s.fails("unreceived_by — несуществующий пользователь",
		`UPDATE transfers SET unreceived_by = 'aaaaaaaa-0017-0017-0017-00000000dead' WHERE id = $1`, doc)
	s.exec(`INSERT INTO users (id, login, password_hash, name, role) VALUES ('aaaaaaaa-0017-0017-0017-000000000002', 'trm-second', 'x', 'Second', 'admin') ON CONFLICT (id) DO NOTHING`)
	s.exec(`UPDATE transfers SET unreceived_by = 'aaaaaaaa-0017-0017-0017-000000000002' WHERE id = $1`, doc)
	s.fails("удаление пользователя, отменившего проведение (RESTRICT)",
		`DELETE FROM users WHERE id = 'aaaaaaaa-0017-0017-0017-000000000002'`)
	// компенсирующее движение отката допустимо CHECK'ом причин (значение добавлено в 0015).
	s.exec(`INSERT INTO stock_movements (product_id, warehouse_id, qty_delta, reason, user_id, transfer_id)
		VALUES ($1, $2, -4, 'transfer_unreceive', $3, $4)`, productID, moscow, trmAdminID, doc)

	// ---- down → 0016 ----
	s.Require().NoError(mig.Migrate(16))
	s.Empty(s.columns())
	s.False(s.constraint("transfers_unreceive_reason_check"))
	s.Equal("received", s.scalar(`SELECT status FROM transfers WHERE id = $1`, doc), "документ остался")
	s.Equal("-4.000", s.scalar(`SELECT qty_delta::text FROM stock_movements WHERE transfer_id = $1 AND reason = 'transfer_unreceive'`, doc),
		"движение отката осталось")
	s.exec(`DELETE FROM users WHERE id = 'aaaaaaaa-0017-0017-0017-000000000002'`) // FK ушёл вместе с колонкой

	// повторный up поверх отката проходит (идемпотентность пары up/down).
	s.Require().NoError(mig.Migrate(17))
	s.Equal("||", s.scalar(`
		SELECT coalesce(unreceived_by::text, '') || '|' || coalesce(unreceived_at::text, '') || '|' || unreceive_reason
		FROM transfers WHERE id = $1`, doc))
}
