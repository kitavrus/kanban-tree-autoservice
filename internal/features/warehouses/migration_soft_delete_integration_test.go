//go:build integration

package warehouses_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/stretchr/testify/suite"

	"github.com/kitavrus/kanban-tree/pkg/testutil"
)

// WarehousesSoftDeleteMigrationSuite — миграция 0023 (soft-delete-warehouses-suppliers §4.1,
// §4.2, TS-012): три колонки удаления (deleted_at, deleted_by FK users RESTRICT, delete_reason) на
// warehouses и suppliers; CHECK «три поля вместе» с причиной 1–500 символов после обрезки;
// частичные индексы списков удалённого; уникальные индексы названий становятся частичными
// (DS-017); функция default_warehouse_id() перестаёт выбирать удалённые склады (DS-021).
// Существующие записи после up — действующие, признак архива не меняется (DS-022); down убирает
// колонки, записи остаются.
type WarehousesSoftDeleteMigrationSuite struct {
	testutil.BaseSuite
}

func TestWarehousesSoftDeleteMigrationSuite(t *testing.T) {
	suite.Run(t, new(WarehousesSoftDeleteMigrationSuite))
}

const (
	whsdAdminID  = "aaaaaaaa-0023-0023-0023-000000000001"
	whsdSecondID = "aaaaaaaa-0023-0023-0023-000000000002"
)

// whsdTables — таблицы части 6.
var whsdTables = []string{"warehouses", "suppliers"}

func (s *WarehousesSoftDeleteMigrationSuite) scalar(q string, args ...any) string {
	var v string
	s.Require().NoError(s.Pool.QueryRow(context.Background(), q, args...).Scan(&v), q)
	return v
}

func (s *WarehousesSoftDeleteMigrationSuite) exec(q string, args ...any) {
	_, err := s.Pool.Exec(context.Background(), q, args...)
	s.Require().NoError(err, q)
}

// fails — запрос обязан упасть на ограничении constraint.
func (s *WarehousesSoftDeleteMigrationSuite) fails(constraint, name, q string, args ...any) {
	_, err := s.Pool.Exec(context.Background(), q, args...)
	if s.Error(err, name) {
		s.Contains(err.Error(), constraint, name)
	}
}

// columns — «имя:тип:nullable» колонок удаления таблицы, по имени.
func (s *WarehousesSoftDeleteMigrationSuite) columns(table string) string {
	return s.scalar(`
		SELECT coalesce(string_agg(column_name || ':' || data_type || ':' || is_nullable, ',' ORDER BY column_name), '')
		FROM information_schema.columns
		WHERE table_name = $1 AND column_name IN ('deleted_at', 'deleted_by', 'delete_reason')`, table)
}

func (s *WarehousesSoftDeleteMigrationSuite) exists(catalog, name string) bool {
	q := `SELECT count(*)::text FROM pg_constraint WHERE conname = $1`
	if catalog == "index" {
		q = `SELECT count(*)::text FROM pg_indexes WHERE indexname = $1`
	}
	return s.scalar(q, name) == "1"
}

func (s *WarehousesSoftDeleteMigrationSuite) indexDef(name string) string {
	return s.scalar(`SELECT indexdef FROM pg_indexes WHERE indexname = $1`, name)
}

// defaultWarehouse — название склада по умолчанию (” — функция вернула NULL).
func (s *WarehousesSoftDeleteMigrationSuite) defaultWarehouse() string {
	return s.scalar(`SELECT coalesce((SELECT name FROM warehouses WHERE id = default_warehouse_id()), '')`)
}

// TS-012 (DS-022, DS-030, DS-017, DS-021): up — существующие склады и поставщики действующие,
// архивные остаются архивными; CHECK отклоняет частичное заполнение, пустую и слишком длинную
// причину; название удалённого склада и поставщика свободно, среди действующих — по-прежнему
// уникально; склад по умолчанию — первый активный НЕудалённый по position; down / up. Негатив:
// только deleted_at без deleted_by → ошибка CHECK.
func (s *WarehousesSoftDeleteMigrationSuite) TestTS012_Migration0023_UpDown() {
	ctx := context.Background()
	mig, err := migrate.New(migrationsURL(), s.DSN)
	s.Require().NoError(err)
	defer func() { _, _ = mig.Close() }()

	s.Require().NoError(mig.Migrate(22))
	defer func() {
		if err := mig.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			s.Require().NoError(err)
		}
	}()
	for _, t := range whsdTables {
		s.Empty(s.columns(t), "0022: колонок удаления на %s нет", t)
	}

	// ---- данные «до» (0022): архивный склад с остатком, действующий и архивный поставщики ----
	s.exec(`INSERT INTO users (id, login, password_hash, name, role) VALUES ($1, 'whsd-admin', 'x', 'Admin', 'admin') ON CONFLICT (id) DO NOTHING`, whsdAdminID)
	ids := map[string]string{}
	one := func(key, q string, args ...any) {
		var id string
		s.Require().NoError(s.Pool.QueryRow(ctx, q, args...).Scan(&id), q)
		ids[key] = id
	}
	one("warehouses", `INSERT INTO warehouses (name, position, is_active) VALUES ('Склад до 0023', 50, false) RETURNING id`)
	one("suppliers", `INSERT INTO suppliers (name, contact_name) VALUES ('Поставщик до 0023', 'Контакт') RETURNING id`)
	one("archived_supplier", `INSERT INTO suppliers (name, is_active) VALUES ('Архивный до 0023', false) RETURNING id`)
	one("category", `INSERT INTO catalog_categories (kind, name) VALUES ('product', 'Категория 0023') RETURNING id`)
	one("product", `INSERT INTO products (category_id, name) VALUES ($1, 'Товар до 0023') RETURNING id`, ids["category"])
	s.exec(`INSERT INTO product_stock (product_id, warehouse_id, stock) VALUES ($1, $2, 7)`, ids["product"], ids["warehouses"])
	defer func() {
		// порядок — от ссылающихся к справочникам (FK RESTRICT).
		s.exec(`DELETE FROM products WHERE id = $1`, ids["product"])
		s.exec(`DELETE FROM catalog_categories WHERE id = $1`, ids["category"])
		s.exec(`DELETE FROM warehouses WHERE name IN ('Склад до 0023', 'Склад 0023 второй', 'Склад 0023 первый по позиции')`)
		s.exec(`DELETE FROM suppliers WHERE name IN ('Поставщик до 0023', 'Архивный до 0023', 'Поставщик 0023 второй')`)
	}()
	s.Equal("Алматы", s.defaultWarehouse(), "0022: склад по умолчанию — первый активный")

	// ---- up → 0023 ----
	s.Require().NoError(mig.Migrate(23))
	for _, t := range whsdTables {
		s.Equal("delete_reason:text:YES,deleted_at:timestamp with time zone:YES,deleted_by:uuid:YES", s.columns(t), t)
		s.True(s.exists("constraint", t+"_deleted_chk"), t)
		s.True(s.exists("index", t+"_deleted_at_idx"), t)
		s.Contains(s.indexDef(t+"_deleted_at_idx"), "WHERE (deleted_at IS NOT NULL)", t)
		s.Equal("0", s.scalar(`SELECT count(*)::text FROM `+t+` WHERE deleted_at IS NOT NULL OR deleted_by IS NOT NULL OR delete_reason IS NOT NULL`),
			"все существующие записи %s — действующие", t)
		def := s.indexDef(t + "_name_lower_uq")
		s.Contains(def, "UNIQUE", t)
		s.Contains(def, "WHERE (deleted_at IS NULL)", "%s: индекс названия — частичный (DS-017)", t)
	}
	s.Equal("false", s.scalar(`SELECT is_active::text FROM warehouses WHERE id = $1`, ids["warehouses"]), "признак архива склада не изменился")
	s.Equal("false", s.scalar(`SELECT is_active::text FROM suppliers WHERE id = $1`, ids["archived_supplier"]), "признак архива поставщика не изменился")
	s.Equal("true", s.scalar(`SELECT is_active::text FROM suppliers WHERE id = $1`, ids["suppliers"]))
	s.Equal("7.000", s.scalar(`SELECT stock::text FROM product_stock WHERE warehouse_id = $1`, ids["warehouses"]), "остаток на складе на месте")

	// негатив: «три поля вместе» и причина 1–500 символов — одинаково для обеих таблиц.
	for _, table := range whsdTables {
		id, chk := ids[table], table+"_deleted_chk"
		s.fails(chk, table+": только deleted_at", `UPDATE `+table+` SET deleted_at = now() WHERE id = $1`, id)
		s.fails(chk, table+": без deleted_by", `UPDATE `+table+` SET deleted_at = now(), delete_reason = 'x' WHERE id = $1`, id)
		s.fails(chk, table+": без причины", `UPDATE `+table+` SET deleted_at = now(), deleted_by = $2 WHERE id = $1`, id, whsdAdminID)
		s.fails(chk, table+": только причина", `UPDATE `+table+` SET delete_reason = 'x' WHERE id = $1`, id)
		s.fails(chk, table+": только deleted_by", `UPDATE `+table+` SET deleted_by = $2 WHERE id = $1`, id, whsdAdminID)
		s.fails(chk, table+": причина из пробелов",
			`UPDATE `+table+` SET deleted_at = now(), deleted_by = $2, delete_reason = '   ' WHERE id = $1`, id, whsdAdminID)
		s.fails(chk, table+": причина 501 символ",
			`UPDATE `+table+` SET deleted_at = now(), deleted_by = $2, delete_reason = $3 WHERE id = $1`, id, whsdAdminID, strings.Repeat("я", 501))
		s.fails(table+"_deleted_by_fkey", table+": несуществующий удаливший",
			`UPDATE `+table+` SET deleted_at = now(), deleted_by = 'aaaaaaaa-0023-0023-0023-00000000dead', delete_reason = 'x' WHERE id = $1`, id)
	}

	// позитив: полное заполнение проходит; причина 500 символов (символов, не байт) допустима.
	for _, table := range whsdTables {
		s.exec(`UPDATE `+table+` SET deleted_at = now(), deleted_by = $2, delete_reason = $3 WHERE id = $1`, ids[table], whsdAdminID, strings.Repeat("я", 500))
	}
	// мягкое удаление склада строки остатков не трогает (FK RESTRICT не срабатывает на UPDATE).
	s.Equal("1", s.scalar(`SELECT count(*)::text FROM product_stock WHERE warehouse_id = $1`, ids["warehouses"]))

	// DS-017: название удалённой записи свободно — и только среди действующих оно уникально (без
	// учёта регистра и крайних пробелов); восстановление при занятом названии ловит индекс.
	s.exec(`INSERT INTO warehouses (name, position) VALUES ('  СКЛАД ДО 0023 ', 51)`)
	s.fails("warehouses_name_lower_uq", "два действующих склада", `INSERT INTO warehouses (name, position) VALUES ('склад до 0023', 52)`)
	s.fails("warehouses_name_lower_uq", "восстановление склада при занятом названии",
		`UPDATE warehouses SET deleted_at = NULL, deleted_by = NULL, delete_reason = NULL WHERE id = $1`, ids["warehouses"])
	s.exec(`UPDATE warehouses SET name = 'Склад 0023 второй' WHERE deleted_at IS NULL AND lower(btrim(name)) = 'склад до 0023'`)
	s.exec(`INSERT INTO suppliers (name) VALUES ('поставщик ДО 0023')`)
	s.fails("suppliers_name_lower_uq", "два действующих поставщика", `INSERT INTO suppliers (name) VALUES ('Поставщик до 0023')`)
	s.fails("suppliers_name_lower_uq", "восстановление поставщика при занятом названии",
		`UPDATE suppliers SET deleted_at = NULL, deleted_by = NULL, delete_reason = NULL WHERE id = $1`, ids["suppliers"])
	s.exec(`UPDATE suppliers SET name = 'Поставщик 0023 второй' WHERE deleted_at IS NULL AND lower(btrim(name)) = 'поставщик до 0023'`)

	// DS-021: склад по умолчанию — первый АКТИВНЫЙ НЕУДАЛЁННЫЙ по position. Активный склад с
	// наименьшей позицией становится складом по умолчанию; удалённый (is_active остаётся true —
	// прежнее условие «только is_active» выбрало бы его) — перестаёт им быть.
	s.exec(`UPDATE warehouses SET position = position + 1`)
	one("first", `INSERT INTO warehouses (name, position) VALUES ('Склад 0023 первый по позиции', 1) RETURNING id`)
	s.Equal("Склад 0023 первый по позиции", s.defaultWarehouse())
	s.exec(`UPDATE warehouses SET deleted_at = now(), deleted_by = $2, delete_reason = 'Закрыт' WHERE id = $1`, ids["first"], whsdAdminID)
	s.Equal("true", s.scalar(`SELECT is_active::text FROM warehouses WHERE id = $1`, ids["first"]), "удаление признак архива не меняет")
	s.Equal("Алматы", s.defaultWarehouse(), "удалённый склад по умолчанию не выбирается")
	s.exec(`UPDATE warehouses SET deleted_at = NULL, deleted_by = NULL, delete_reason = NULL WHERE id = $1`, ids["first"])
	s.exec(`DELETE FROM warehouses WHERE id = $1`, ids["first"])
	s.exec(`UPDATE warehouses SET position = position - 1`)

	// FK deleted_by → users RESTRICT: автора удаления стереть нельзя.
	s.exec(`INSERT INTO users (id, login, password_hash, name, role) VALUES ($1, 'whsd-second', 'x', 'Second', 'admin') ON CONFLICT (id) DO NOTHING`, whsdSecondID)
	s.exec(`UPDATE suppliers SET deleted_by = $2 WHERE id = $1`, ids["suppliers"], whsdSecondID)
	_, err = s.Pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, whsdSecondID)
	s.Error(err, "удаление пользователя, удалившего поставщика (RESTRICT)")

	// ---- down → 0022: колонки и ограничения ушли, записи остались, индексы названий — полные ----
	s.Require().NoError(mig.Migrate(22))
	for _, t := range whsdTables {
		s.Empty(s.columns(t), t)
		s.False(s.exists("constraint", t+"_deleted_chk"), t)
		s.False(s.exists("index", t+"_deleted_at_idx"), t)
		s.Equal("1", s.scalar(`SELECT count(*)::text FROM `+t+` WHERE id = $1`, ids[t]), t)
		s.NotContains(s.indexDef(t+"_name_lower_uq"), "WHERE", t)
	}
	s.Equal("Алматы", s.defaultWarehouse(), "функция склада по умолчанию работает и после отката")
	s.exec(`DELETE FROM users WHERE id = $1`, whsdSecondID) // FK ушёл вместе с колонкой

	// повторный up поверх отката проходит; записи снова действующие.
	s.Require().NoError(mig.Migrate(23))
	for _, t := range whsdTables {
		s.Equal("0", s.scalar(`SELECT count(*)::text FROM `+t+` WHERE id = $1 AND deleted_at IS NOT NULL`, ids[t]), t)
	}
}
