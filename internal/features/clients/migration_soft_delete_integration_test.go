//go:build integration

package clients_test

import (
	"context"
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/stretchr/testify/suite"

	"github.com/kitavrus/kanban-tree/pkg/testutil"
)

// ClientsSoftDeleteMigrationSuite — миграция 0013 (soft-delete-clients §4.1, §4.2, TS-012):
// три колонки удаления на clients и equipment (deleted_at, deleted_by FK users RESTRICT,
// delete_reason), у equipment — ещё deleted_with_client; CHECK «три поля вместе» с причиной
// 1–500 символов после обрезки, CHECK «вместе с клиентом — только у удалённой единицы» и
// частичные индексы. Существующие клиенты и единицы после up — действующие (DS-022); down
// убирает колонки, записи остаются.
type ClientsSoftDeleteMigrationSuite struct {
	testutil.BaseSuite
}

func TestClientsSoftDeleteMigrationSuite(t *testing.T) {
	suite.Run(t, new(ClientsSoftDeleteMigrationSuite))
}

const (
	csdAdminID  = "aaaaaaaa-0013-0013-0013-000000000001"
	csdSecondID = "aaaaaaaa-0013-0013-0013-000000000002"
)

func migrationsURL() string {
	_, f, _, _ := runtime.Caller(0)
	return "file://" + filepath.Join(filepath.Dir(f), "..", "..", "..", "migrations")
}

func (s *ClientsSoftDeleteMigrationSuite) scalar(q string, args ...any) string {
	var v string
	s.Require().NoError(s.Pool.QueryRow(context.Background(), q, args...).Scan(&v), q)
	return v
}

func (s *ClientsSoftDeleteMigrationSuite) exec(q string, args ...any) {
	_, err := s.Pool.Exec(context.Background(), q, args...)
	s.Require().NoError(err, q)
}

// fails — запрос обязан упасть на ограничении constraint.
func (s *ClientsSoftDeleteMigrationSuite) fails(constraint, name, q string, args ...any) {
	_, err := s.Pool.Exec(context.Background(), q, args...)
	if s.Error(err, name) {
		s.Contains(err.Error(), constraint, name)
	}
}

// columns — «имя:тип:nullable» колонок удаления таблицы, по имени.
func (s *ClientsSoftDeleteMigrationSuite) columns(table string) string {
	return s.scalar(`
		SELECT coalesce(string_agg(column_name || ':' || data_type || ':' || is_nullable, ',' ORDER BY column_name), '')
		FROM information_schema.columns
		WHERE table_name = $1 AND column_name IN ('deleted_at', 'deleted_by', 'delete_reason', 'deleted_with_client')`, table)
}

func (s *ClientsSoftDeleteMigrationSuite) exists(catalog, name string) bool {
	q := `SELECT count(*)::text FROM pg_constraint WHERE conname = $1`
	if catalog == "index" {
		q = `SELECT count(*)::text FROM pg_indexes WHERE indexname = $1`
	}
	return s.scalar(q, name) == "1"
}

// TS-012 (DS-022, DS-030, DS-015): up — существующие клиент и единица действующие; CHECK
// отклоняет частичное заполнение, пустую и слишком длинную причину, флаг «вместе с клиентом» у
// действующей единицы; down / up. Негатив: только deleted_at без deleted_by → ошибка CHECK.
func (s *ClientsSoftDeleteMigrationSuite) TestTS012_Migration0021_UpDown() {
	ctx := context.Background()
	mig, err := migrate.New(migrationsURL(), s.DSN)
	s.Require().NoError(err)
	defer func() { _, _ = mig.Close() }()

	s.Require().NoError(mig.Migrate(12))
	defer func() {
		if err := mig.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			s.Require().NoError(err)
		}
	}()
	s.Empty(s.columns("clients"), "0012: колонок удаления на clients нет")
	s.Empty(s.columns("equipment"), "0012: колонок удаления на equipment нет")

	// ---- данные «до» (0012): клиент с телефоном и единицей ----
	s.exec(`INSERT INTO users (id, login, password_hash, name, role) VALUES ($1, 'csd-admin', 'x', 'Admin', 'admin') ON CONFLICT (id) DO NOTHING`, csdAdminID)
	var clientID, typeID, unitID string
	s.Require().NoError(s.Pool.QueryRow(ctx, `INSERT INTO clients (company_name) VALUES ('Клиент до миграции 0013') RETURNING id`).Scan(&clientID))
	defer s.exec(`DELETE FROM clients WHERE id = $1`, clientID)
	s.exec(`INSERT INTO client_phones (client_id, position, phone) VALUES ($1, 0, '111')`, clientID)
	s.Require().NoError(s.Pool.QueryRow(ctx, `INSERT INTO equipment_types (name) VALUES ('Тип до миграции 0013') RETURNING id`).Scan(&typeID))
	defer s.exec(`DELETE FROM equipment_types WHERE id = $1`, typeID)
	s.Require().NoError(s.Pool.QueryRow(ctx, `INSERT INTO equipment (client_id, type_id, model) VALUES ($1, $2, 'M-0013') RETURNING id`, clientID, typeID).Scan(&unitID))
	defer s.exec(`DELETE FROM equipment WHERE id = $1`, unitID)

	// ---- up → 0013 ----
	s.Require().NoError(mig.Migrate(13))
	s.Equal("delete_reason:text:YES,deleted_at:timestamp with time zone:YES,deleted_by:uuid:YES", s.columns("clients"))
	s.Equal("delete_reason:text:YES,deleted_at:timestamp with time zone:YES,deleted_by:uuid:YES,deleted_with_client:boolean:NO", s.columns("equipment"))
	for _, c := range []string{"clients_deleted_chk", "equipment_deleted_chk", "equipment_deleted_with_client_chk"} {
		s.True(s.exists("constraint", c), c)
	}
	for _, i := range []string{"clients_deleted_at_idx", "equipment_deleted_at_idx"} {
		s.True(s.exists("index", i), i)
	}
	s.Equal("0", s.scalar(`SELECT count(*)::text FROM clients WHERE deleted_at IS NOT NULL OR deleted_by IS NOT NULL OR delete_reason IS NOT NULL`),
		"все существующие клиенты — действующие")
	s.Equal("0", s.scalar(`SELECT count(*)::text FROM equipment
		WHERE deleted_at IS NOT NULL OR deleted_by IS NOT NULL OR delete_reason IS NOT NULL OR deleted_with_client`),
		"все существующие единицы — действующие, флаг «вместе с клиентом» снят")

	// негатив: «три поля вместе» и причина 1–500 символов — одинаково для обеих таблиц.
	for table, id := range map[string]string{"clients": clientID, "equipment": unitID} {
		chk := table + "_deleted_chk"
		s.fails(chk, table+": только deleted_at", `UPDATE `+table+` SET deleted_at = now() WHERE id = $1`, id)
		s.fails(chk, table+": без deleted_by", `UPDATE `+table+` SET deleted_at = now(), delete_reason = 'x' WHERE id = $1`, id)
		s.fails(chk, table+": без причины", `UPDATE `+table+` SET deleted_at = now(), deleted_by = $2 WHERE id = $1`, id, csdAdminID)
		s.fails(chk, table+": только причина", `UPDATE `+table+` SET delete_reason = 'x' WHERE id = $1`, id)
		s.fails(chk, table+": только deleted_by", `UPDATE `+table+` SET deleted_by = $2 WHERE id = $1`, id, csdAdminID)
		s.fails(chk, table+": причина из пробелов",
			`UPDATE `+table+` SET deleted_at = now(), deleted_by = $2, delete_reason = '   ' WHERE id = $1`, id, csdAdminID)
		s.fails(chk, table+": причина 501 символ",
			`UPDATE `+table+` SET deleted_at = now(), deleted_by = $2, delete_reason = $3 WHERE id = $1`, id, csdAdminID, strings.Repeat("я", 501))
		s.fails(table+"_deleted_by_fkey", table+": несуществующий удаливший",
			`UPDATE `+table+` SET deleted_at = now(), deleted_by = 'aaaaaaaa-0013-0013-0013-00000000dead', delete_reason = 'x' WHERE id = $1`, id)
	}
	// DS-015: «вместе с клиентом» — только у удалённой единицы.
	s.fails("equipment_deleted_with_client_chk", "флаг у действующей единицы",
		`UPDATE equipment SET deleted_with_client = true WHERE id = $1`, unitID)

	// позитив: полное заполнение проходит; причина 500 символов (символов, не байт) допустима.
	s.exec(`UPDATE clients SET deleted_at = now(), deleted_by = $2, delete_reason = $3 WHERE id = $1`, clientID, csdAdminID, strings.Repeat("я", 500))
	s.exec(`UPDATE equipment SET deleted_at = now(), deleted_by = $2, delete_reason = 'Создан по ошибке', deleted_with_client = true WHERE id = $1`, unitID, csdAdminID)
	// снять отметку, оставив флаг, нельзя.
	s.fails("equipment_deleted_with_client_chk", "восстановление без сброса флага",
		`UPDATE equipment SET deleted_at = NULL, deleted_by = NULL, delete_reason = NULL WHERE id = $1`, unitID)
	// мягкое удаление клиента телефоны не трогает (каскад FK не срабатывает).
	s.Equal("1", s.scalar(`SELECT count(*)::text FROM client_phones WHERE client_id = $1`, clientID))

	// FK deleted_by → users RESTRICT: автора удаления стереть нельзя.
	s.exec(`INSERT INTO users (id, login, password_hash, name, role) VALUES ($1, 'csd-second', 'x', 'Second', 'admin') ON CONFLICT (id) DO NOTHING`, csdSecondID)
	s.exec(`UPDATE equipment SET deleted_by = $2 WHERE id = $1`, unitID, csdSecondID)
	_, err = s.Pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, csdSecondID)
	s.Error(err, "удаление пользователя, удалившего единицу (RESTRICT)")

	// ---- down → 0012: колонки и ограничения ушли, записи остались ----
	s.Require().NoError(mig.Migrate(12))
	s.Empty(s.columns("clients"))
	s.Empty(s.columns("equipment"))
	for _, c := range []string{"clients_deleted_chk", "equipment_deleted_chk", "equipment_deleted_with_client_chk"} {
		s.False(s.exists("constraint", c), c)
	}
	for _, i := range []string{"clients_deleted_at_idx", "equipment_deleted_at_idx"} {
		s.False(s.exists("index", i), i)
	}
	s.Equal("1", s.scalar(`SELECT count(*)::text FROM clients WHERE id = $1`, clientID))
	s.Equal("1", s.scalar(`SELECT count(*)::text FROM equipment WHERE id = $1`, unitID))
	s.exec(`DELETE FROM users WHERE id = $1`, csdSecondID) // FK ушёл вместе с колонкой

	// повторный up поверх отката проходит; записи снова действующие.
	s.Require().NoError(mig.Migrate(13))
	s.Equal("0", s.scalar(`SELECT count(*)::text FROM clients WHERE id = $1 AND deleted_at IS NOT NULL`, clientID))
	s.Equal("0", s.scalar(`SELECT count(*)::text FROM equipment WHERE id = $1 AND (deleted_at IS NOT NULL OR deleted_with_client)`, unitID))
}
