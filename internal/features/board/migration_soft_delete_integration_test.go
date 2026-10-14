//go:build integration

package board_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	"github.com/stretchr/testify/suite"

	"github.com/kitavrus/kanban-tree/pkg/testutil"
)

// BoardSoftDeleteMigrationSuite — миграции 0018 (soft-delete-core §4.1, заказы) и 0020
// (soft-delete-board-columns §4.1, колонки доски), TS-012: три колонки удаления (deleted_at,
// deleted_by FK users RESTRICT, delete_reason), CHECK «три поля вместе» с причиной 1–500
// символов после обрезки, частичный индекс списка удалённого. Откат на данных с удалёнными
// записями: колонки уходят, записи остаются и снова действующие; повторный накат проходит.
type BoardSoftDeleteMigrationSuite struct {
	testutil.BaseSuite
}

func TestBoardSoftDeleteMigrationSuite(t *testing.T) {
	suite.Run(t, new(BoardSoftDeleteMigrationSuite))
}

const (
	bsdAdminID  = "aaaaaaaa-0018-0018-0018-000000000001"
	bsdSecondID = "aaaaaaaa-0018-0018-0018-000000000002"
)

func (s *BoardSoftDeleteMigrationSuite) scalar(q string, args ...any) string {
	var v string
	s.Require().NoError(s.Pool.QueryRow(context.Background(), q, args...).Scan(&v), q)
	return v
}

func (s *BoardSoftDeleteMigrationSuite) exec(q string, args ...any) {
	_, err := s.Pool.Exec(context.Background(), q, args...)
	s.Require().NoError(err, q)
}

// fails — запрос обязан упасть на ограничении constraint.
func (s *BoardSoftDeleteMigrationSuite) fails(constraint, name, q string, args ...any) {
	_, err := s.Pool.Exec(context.Background(), q, args...)
	if s.Error(err, name) {
		s.Contains(err.Error(), constraint, name)
	}
}

// columns — «имя:тип:nullable» колонок удаления таблицы, по имени.
func (s *BoardSoftDeleteMigrationSuite) columns(table string) string {
	return s.scalar(`
		SELECT coalesce(string_agg(column_name || ':' || data_type || ':' || is_nullable, ',' ORDER BY column_name), '')
		FROM information_schema.columns
		WHERE table_name = $1 AND column_name IN ('deleted_at', 'deleted_by', 'delete_reason')`, table)
}

func (s *BoardSoftDeleteMigrationSuite) exists(catalog, name string) bool {
	q := `SELECT count(*)::text FROM pg_constraint WHERE conname = $1`
	if catalog == "index" {
		q = `SELECT count(*)::text FROM pg_indexes WHERE indexname = $1`
	}
	return s.scalar(q, name) == "1"
}

// assertUp — накат добавил колонки, CHECK и частичный индекс; все записи действующие.
func (s *BoardSoftDeleteMigrationSuite) assertUp(table string) {
	s.Equal("delete_reason:text:YES,deleted_at:timestamp with time zone:YES,deleted_by:uuid:YES", s.columns(table))
	s.True(s.exists("constraint", table+"_deleted_chk"))
	s.True(s.exists("index", table+"_deleted_at_idx"))
	s.Contains(s.scalar(`SELECT indexdef FROM pg_indexes WHERE indexname = $1`, table+"_deleted_at_idx"), "WHERE (deleted_at IS NOT NULL)")
	s.Equal("0", s.scalar(`SELECT count(*)::text FROM `+table+` WHERE deleted_at IS NOT NULL OR deleted_by IS NOT NULL OR delete_reason IS NOT NULL`),
		"все существующие записи %s — действующие", table)
}

// assertCheck — «три поля вместе», причина 1–500 символов, FK deleted_by.
func (s *BoardSoftDeleteMigrationSuite) assertCheck(table string, id any) {
	chk := table + "_deleted_chk"
	s.fails(chk, "только deleted_at", `UPDATE `+table+` SET deleted_at = now() WHERE id = $1`, id)
	s.fails(chk, "без deleted_by", `UPDATE `+table+` SET deleted_at = now(), delete_reason = 'x' WHERE id = $1`, id)
	s.fails(chk, "без причины", `UPDATE `+table+` SET deleted_at = now(), deleted_by = $2 WHERE id = $1`, id, bsdAdminID)
	s.fails(chk, "только причина", `UPDATE `+table+` SET delete_reason = 'x' WHERE id = $1`, id)
	s.fails(chk, "причина из пробелов",
		`UPDATE `+table+` SET deleted_at = now(), deleted_by = $2, delete_reason = '   ' WHERE id = $1`, id, bsdAdminID)
	s.fails(chk, "причина 501 символ",
		`UPDATE `+table+` SET deleted_at = now(), deleted_by = $2, delete_reason = $3 WHERE id = $1`, id, bsdAdminID, strings.Repeat("я", 501))
	s.fails(table+"_deleted_by_fkey", "несуществующий удаливший",
		`UPDATE `+table+` SET deleted_at = now(), deleted_by = 'aaaaaaaa-0018-0018-0018-00000000dead', delete_reason = 'x' WHERE id = $1`, id)
}

func (s *BoardSoftDeleteMigrationSuite) assertDown(table string) {
	s.Empty(s.columns(table))
	s.False(s.exists("constraint", table+"_deleted_chk"))
	s.False(s.exists("index", table+"_deleted_at_idx"))
}

// TS-012, миграция 0018: накат → удалённые и действующие заказы (в т. ч. одноимённые, оплаченный
// удалённый) → откат → накат. Откат выполним: колонки уходят, удалённые заказы остаются в своей
// колонке на своей позиции и снова действующие; автора удаления после отката можно стереть.
func (s *BoardSoftDeleteMigrationSuite) TestTS012_Migration0018_UpRollbackUp() {
	ctx := context.Background()
	mig, err := migrate.New(migrateRoot(), s.DSN)
	s.Require().NoError(err)
	defer func() { _, _ = mig.Close() }()

	s.Require().NoError(mig.Migrate(17))
	defer func() {
		if err := mig.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			s.Require().NoError(err)
		}
	}()
	s.Empty(s.columns("tasks"), "0017: колонок удаления на tasks нет")

	// ---- данные «до» (0017) ----
	s.exec(`INSERT INTO users (id, login, password_hash, name, role) VALUES ($1, 'bsd-admin', 'x', 'Admin', 'admin') ON CONFLICT (id) DO NOTHING`, bsdAdminID)
	column := s.scalar(`SELECT id::text FROM columns ORDER BY position, id LIMIT 1`)
	var clientID string
	s.Require().NoError(s.Pool.QueryRow(ctx, `INSERT INTO clients (company_name) VALUES ('Клиент миграции 0018') RETURNING id`).Scan(&clientID))
	defer s.exec(`DELETE FROM clients WHERE id = $1`, clientID)
	task := func(title string, position int, paid bool) int64 {
		var id int64
		s.Require().NoError(s.Pool.QueryRow(ctx, `
			INSERT INTO tasks (column_id, client_id, title, position, created_by, is_paid, paid_at)
			VALUES ($1, $2, $3, $4, $5, $6, CASE WHEN $6 THEN now() END) RETURNING id`,
			column, clientID, title, position, bsdAdminID, paid).Scan(&id))
		return id
	}
	before := task("Заказ до 0018", 9001, false)
	defer s.exec(`DELETE FROM tasks WHERE client_id = $1`, clientID)

	// ---- up → 0018 ----
	s.Require().NoError(mig.Migrate(18))
	s.assertUp("tasks")
	s.assertCheck("tasks", before)

	// удалённые: одноимённый с действующим, оплаченный, с причиной 500 символов; автор — второй.
	s.exec(`INSERT INTO users (id, login, password_hash, name, role) VALUES ($1, 'bsd-second', 'x', 'Second', 'admin') ON CONFLICT (id) DO NOTHING`, bsdSecondID)
	namesake, paid := task("Заказ до 0018", 9002, false), task("Оплаченный удалённый 0018", 9003, true)
	s.exec(`UPDATE tasks SET deleted_at = now() - interval '1 day', deleted_by = $2, delete_reason = 'Дубль' WHERE id = $1`, namesake, bsdSecondID)
	s.exec(`UPDATE tasks SET deleted_at = now(), deleted_by = $2, delete_reason = $3 WHERE id = $1`, paid, bsdAdminID, strings.Repeat("я", 500))
	_, err = s.Pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, bsdSecondID)
	s.Error(err, "удаление пользователя, удалившего заказ (RESTRICT)")

	// ---- down → 0017: колонки ушли, заказы на месте ----
	s.Require().NoError(mig.Migrate(17))
	s.assertDown("tasks")
	s.Equal("9001,9002,9003", s.scalar(`
		SELECT string_agg(position::text, ',' ORDER BY position) FROM tasks WHERE id = ANY($1) AND column_id = $2`,
		[]int64{before, namesake, paid}, column), "заказы остались в своей колонке на своих позициях")
	s.Equal("true", s.scalar(`SELECT (is_paid AND paid_at IS NOT NULL)::text FROM tasks WHERE id = $1`, paid))
	s.exec(`DELETE FROM users WHERE id = $1`, bsdSecondID) // FK ушёл вместе с колонкой

	// ---- повторный up: бывшие удалённые — действующие, удаление снова работает ----
	s.Require().NoError(mig.Migrate(18))
	s.assertUp("tasks")
	s.exec(`UPDATE tasks SET deleted_at = now(), deleted_by = $2, delete_reason = 'Снова' WHERE id = $1`, namesake, bsdAdminID)
	s.Equal("1", s.scalar(`SELECT count(*)::text FROM tasks WHERE client_id = $1 AND deleted_at IS NOT NULL`, clientID))
}

// TS-012, миграция 0020: накат → удалённые колонки (одноимённая с действующей, финальная, с
// прежней позицией внутри доски и с одинаковыми позициями) → откат → накат. Откат выполним:
// действующие колонки сохраняют порядок и идут 1..N, ожившие встают в конец доски по
// (прежняя позиция, id); позиции не повторяются.
func (s *BoardSoftDeleteMigrationSuite) TestTS012_Migration0020_UpRollbackUp() {
	ctx := context.Background()
	mig, err := migrate.New(migrateRoot(), s.DSN)
	s.Require().NoError(err)
	defer func() { _, _ = mig.Close() }()

	s.Require().NoError(mig.Migrate(19))
	defer func() {
		if err := mig.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			s.Require().NoError(err)
		}
	}()
	s.Empty(s.columns("columns"), "0019: колонок удаления на columns нет")
	order := func() string {
		return s.scalar(`SELECT string_agg(name || '=' || position, ', ' ORDER BY position, id) FROM columns`)
	}
	seed := order()
	s.Equal("Новые=1, Согласование=2, В работе=3, Ждёт запчасти=4, Готово=5", seed)
	defer func() {
		s.exec(`DELETE FROM columns WHERE name LIKE '% 0020'  OR name = 'В работе' AND position > 5`)
		s.Equal(seed, order(), "доска после теста — как до него")
	}()

	s.exec(`INSERT INTO users (id, login, password_hash, name, role) VALUES ($1, 'bsd-admin', 'x', 'Admin', 'admin') ON CONFLICT (id) DO NOTHING`, bsdAdminID)
	col := func(name string, position int, final bool) string {
		var id string
		s.Require().NoError(s.Pool.QueryRow(ctx, `
			INSERT INTO columns (name, color, position, is_final, consumes_stock) VALUES ($1, '#112233', $2, $3, $3) RETURNING id`,
			name, position, final).Scan(&id))
		return id
	}
	active := col("Действующая 0020", 6, false)

	// ---- up → 0020 ----
	s.Require().NoError(mig.Migrate(20))
	s.assertUp("columns")
	s.assertCheck("columns", active)

	// удалённые колонки хранят прежнюю позицию: внутри доски, с повтором и за её пределами.
	del := func(id, ago string) {
		s.exec(`UPDATE columns SET deleted_at = now() - $2::interval, deleted_by = $3, delete_reason = 'Не нужна' WHERE id = $1`, id, ago, bsdAdminID)
	}
	namesake, final, far := col("В работе", 6, false), col("Финальная 0020", 3, true), col("Дальняя 0020", 40, false)
	twin := col("Близнец 0020", 3, false)
	del(namesake, "3 day")
	del(final, "2 day")
	del(far, "1 day")
	del(twin, "1 hour")

	// ---- down → 0019 ----
	s.Require().NoError(mig.Migrate(19))
	s.assertDown("columns")
	first, second := "Финальная 0020", "Близнец 0020" // одинаковая прежняя позиция 3 — порядок по id
	if twin < final {
		first, second = second, first
	}
	s.Equal(seed+", Действующая 0020=6, "+first+"=7, "+second+"=8, В работе=9, Дальняя 0020=10", order(),
		"действующие 1..N в прежнем порядке, ожившие — в конце по (прежняя позиция, id)")
	s.Equal("true", s.scalar(`SELECT is_final::text FROM columns WHERE id = $1`, final), "признаки колонки не меняются")

	// ---- повторный up: все колонки действующие, позиции отката сохраняются ----
	s.Require().NoError(mig.Migrate(20))
	s.assertUp("columns")
	s.Equal("10", s.scalar(`SELECT count(DISTINCT position)::text FROM columns`))
}
