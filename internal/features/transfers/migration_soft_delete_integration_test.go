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

// TransfersSoftDeleteMigrationSuite — миграция 0019 (soft-delete-transfers §4.1, TS-012):
// три колонки удаления на transfers (deleted_at, deleted_by FK users RESTRICT, delete_reason),
// CHECK transfers_deleted_chk («три поля вместе», причина 1–500 символов после обрезки, удалён
// может быть только документ в requested / cancelled) и частичный индекс
// transfers_deleted_at_idx. Существующие документы после up — действующие (DS-022); down
// убирает колонки, документы остаются.
type TransfersSoftDeleteMigrationSuite struct {
	testutil.BaseSuite
}

func TestTransfersSoftDeleteMigrationSuite(t *testing.T) {
	suite.Run(t, new(TransfersSoftDeleteMigrationSuite))
}

const (
	tsdAdminID  = "aaaaaaaa-0019-0019-0019-000000000001"
	tsdSecondID = "aaaaaaaa-0019-0019-0019-000000000002"
)

func (s *TransfersSoftDeleteMigrationSuite) scalar(q string, args ...any) string {
	var v string
	s.Require().NoError(s.Pool.QueryRow(context.Background(), q, args...).Scan(&v), q)
	return v
}

func (s *TransfersSoftDeleteMigrationSuite) exec(q string, args ...any) {
	_, err := s.Pool.Exec(context.Background(), q, args...)
	s.Require().NoError(err, q)
}

// failsCheck — запрос обязан упасть на CHECK transfers_deleted_chk.
func (s *TransfersSoftDeleteMigrationSuite) failsCheck(name, q string, args ...any) {
	_, err := s.Pool.Exec(context.Background(), q, args...)
	if s.Error(err, name) {
		s.Contains(err.Error(), "transfers_deleted_chk", name)
	}
}

// columns — «имя:тип:nullable» колонок удаления таблицы transfers, по имени.
func (s *TransfersSoftDeleteMigrationSuite) columns() string {
	return s.scalar(`
		SELECT coalesce(string_agg(column_name || ':' || data_type || ':' || is_nullable, ',' ORDER BY column_name), '')
		FROM information_schema.columns
		WHERE table_name = 'transfers' AND column_name IN ('deleted_at', 'deleted_by', 'delete_reason')`)
}

func (s *TransfersSoftDeleteMigrationSuite) exists(catalog, name string) bool {
	q := `SELECT count(*)::text FROM pg_constraint WHERE conname = $1`
	if catalog == "index" {
		q = `SELECT count(*)::text FROM pg_indexes WHERE indexname = $1`
	}
	return s.scalar(q, name) == "1"
}

// TS-012 (DS-022, DS-030): up — существующие документы действующие; CHECK отклоняет частичное
// заполнение, пустую и слишком длинную причину, удаление проведённого документа; down / up.
func (s *TransfersSoftDeleteMigrationSuite) TestTS012_Migration0019_UpDown() {
	ctx := context.Background()
	mig, err := migrate.New(migrationsURL(), s.DSN)
	s.Require().NoError(err)
	defer func() { _, _ = mig.Close() }()

	s.Require().NoError(mig.Migrate(18))
	defer func() {
		if err := mig.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			s.Require().NoError(err)
		}
	}()
	s.Empty(s.columns(), "0018: колонок удаления на transfers нет")

	// ---- данные «до» (0018): документы во всех четырёх статусах ----
	s.exec(`INSERT INTO users (id, login, password_hash, name, role) VALUES ($1, 'tsd-admin', 'x', 'Admin', 'admin') ON CONFLICT (id) DO NOTHING`, tsdAdminID)
	almaty := s.scalar(`SELECT id::text FROM warehouses WHERE name = 'Алматы'`)
	moscow := s.scalar(`SELECT id::text FROM warehouses WHERE name = 'Москва'`)
	docs := map[string]int64{}
	for _, status := range []string{"requested", "in_transit", "received", "cancelled"} {
		var id int64
		s.Require().NoError(s.Pool.QueryRow(ctx, `
			INSERT INTO transfers (direction, from_warehouse_id, to_warehouse_id, status, created_by)
			VALUES ('warehouse_to_warehouse', $1, $2, $3, $4) RETURNING id`, almaty, moscow, status, tsdAdminID).Scan(&id))
		docs[status] = id
		defer s.exec(`DELETE FROM transfers WHERE id = $1`, id)
	}

	// ---- up → 0019 ----
	s.Require().NoError(mig.Migrate(19))
	s.Equal("delete_reason:text:YES,deleted_at:timestamp with time zone:YES,deleted_by:uuid:YES", s.columns())
	s.True(s.exists("constraint", "transfers_deleted_chk"))
	s.True(s.exists("index", "transfers_deleted_at_idx"))
	s.Equal("0", s.scalar(`SELECT count(*)::text FROM transfers WHERE deleted_at IS NOT NULL OR deleted_by IS NOT NULL OR delete_reason IS NOT NULL`),
		"все существующие документы — действующие")

	requested, cancelled := docs["requested"], docs["cancelled"]
	// негатив: «три поля вместе» — частичное заполнение отклоняется.
	s.failsCheck("только deleted_at", `UPDATE transfers SET deleted_at = now() WHERE id = $1`, requested)
	s.failsCheck("без deleted_by", `UPDATE transfers SET deleted_at = now(), delete_reason = 'x' WHERE id = $1`, requested)
	s.failsCheck("без причины", `UPDATE transfers SET deleted_at = now(), deleted_by = $2 WHERE id = $1`, requested, tsdAdminID)
	s.failsCheck("только причина", `UPDATE transfers SET delete_reason = 'x' WHERE id = $1`, requested)
	// причина — 1–500 символов после обрезки пробелов (символов, не байт).
	s.failsCheck("причина из пробелов",
		`UPDATE transfers SET deleted_at = now(), deleted_by = $2, delete_reason = '   ' WHERE id = $1`, requested, tsdAdminID)
	s.failsCheck("причина 501 символ",
		`UPDATE transfers SET deleted_at = now(), deleted_by = $2, delete_reason = $3 WHERE id = $1`, requested, tsdAdminID, strings.Repeat("я", 501))
	// удалён может быть только документ, не влияющий на учёт (DS-018).
	for _, status := range []string{"in_transit", "received"} {
		s.failsCheck("удаление документа "+status,
			`UPDATE transfers SET deleted_at = now(), deleted_by = $2, delete_reason = 'x' WHERE id = $1`, docs[status], tsdAdminID)
	}
	// позитив: заявка и отменённый удаляются; причина 500 символов допустима.
	s.exec(`UPDATE transfers SET deleted_at = now(), deleted_by = $2, delete_reason = $3 WHERE id = $1`, requested, tsdAdminID, strings.Repeat("я", 500))
	s.exec(`UPDATE transfers SET deleted_at = now(), deleted_by = $2, delete_reason = 'Создан по ошибке' WHERE id = $1`, cancelled, tsdAdminID)
	// удалённый документ нельзя перевести в проведённый статус в обход восстановления.
	s.failsCheck("отправка удалённой заявки", `UPDATE transfers SET status = 'in_transit' WHERE id = $1`, requested)

	// FK deleted_by → users: несуществующий пользователь и удаление автора удаления запрещены.
	_, err = s.Pool.Exec(ctx, `UPDATE transfers SET deleted_by = 'aaaaaaaa-0019-0019-0019-00000000dead' WHERE id = $1`, cancelled)
	s.Error(err, "deleted_by — несуществующий пользователь")
	s.exec(`INSERT INTO users (id, login, password_hash, name, role) VALUES ($1, 'tsd-second', 'x', 'Second', 'admin') ON CONFLICT (id) DO NOTHING`, tsdSecondID)
	s.exec(`UPDATE transfers SET deleted_by = $2 WHERE id = $1`, cancelled, tsdSecondID)
	_, err = s.Pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, tsdSecondID)
	s.Error(err, "удаление пользователя, удалившего документ (RESTRICT)")

	// ---- down → 0018: колонки и ограничения ушли, документы остались ----
	s.Require().NoError(mig.Migrate(18))
	s.Empty(s.columns())
	s.False(s.exists("constraint", "transfers_deleted_chk"))
	s.False(s.exists("index", "transfers_deleted_at_idx"))
	s.Equal("4", s.scalar(`SELECT count(*)::text FROM transfers WHERE id = ANY($1)`, []int64{docs["requested"], docs["in_transit"], docs["received"], docs["cancelled"]}))
	s.exec(`DELETE FROM users WHERE id = $1`, tsdSecondID) // FK ушёл вместе с колонкой

	// повторный up поверх отката проходит; документы снова действующие.
	s.Require().NoError(mig.Migrate(19))
	s.Equal("0", s.scalar(`SELECT count(*)::text FROM transfers WHERE id = ANY($1) AND deleted_at IS NOT NULL`,
		[]int64{requested, cancelled}))
}
