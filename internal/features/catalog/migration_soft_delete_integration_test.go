//go:build integration

package catalog_test

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

// CatalogSoftDeleteMigrationSuite — миграция 0014 (soft-delete-catalog §4.1–§4.4, TS-012): три
// колонки удаления (deleted_at, deleted_by FK users RESTRICT, delete_reason) на products,
// services, catalog_categories, equipment_types; CHECK «три поля вместе» с причиной 1–500
// символов после обрезки; частичные индексы списков удалённого; уникальные индексы названий
// категорий и типов становятся частичными (DS-017). Существующие записи после up —
// действующие, признак архива не меняется (DS-022); down убирает колонки, записи остаются.
type CatalogSoftDeleteMigrationSuite struct {
	testutil.BaseSuite
}

func TestCatalogSoftDeleteMigrationSuite(t *testing.T) {
	suite.Run(t, new(CatalogSoftDeleteMigrationSuite))
}

const (
	catAdminID  = "aaaaaaaa-0014-0014-0014-000000000001"
	catSecondID = "aaaaaaaa-0014-0014-0014-000000000002"
)

// catTables — таблицы части 5.
var catTables = []string{"products", "services", "catalog_categories", "equipment_types"}

func migrationsURL() string {
	_, f, _, _ := runtime.Caller(0)
	return "file://" + filepath.Join(filepath.Dir(f), "..", "..", "..", "migrations")
}

func (s *CatalogSoftDeleteMigrationSuite) scalar(q string, args ...any) string {
	var v string
	s.Require().NoError(s.Pool.QueryRow(context.Background(), q, args...).Scan(&v), q)
	return v
}

func (s *CatalogSoftDeleteMigrationSuite) exec(q string, args ...any) {
	_, err := s.Pool.Exec(context.Background(), q, args...)
	s.Require().NoError(err, q)
}

// fails — запрос обязан упасть на ограничении constraint.
func (s *CatalogSoftDeleteMigrationSuite) fails(constraint, name, q string, args ...any) {
	_, err := s.Pool.Exec(context.Background(), q, args...)
	if s.Error(err, name) {
		s.Contains(err.Error(), constraint, name)
	}
}

// columns — «имя:тип:nullable» колонок удаления таблицы, по имени.
func (s *CatalogSoftDeleteMigrationSuite) columns(table string) string {
	return s.scalar(`
		SELECT coalesce(string_agg(column_name || ':' || data_type || ':' || is_nullable, ',' ORDER BY column_name), '')
		FROM information_schema.columns
		WHERE table_name = $1 AND column_name IN ('deleted_at', 'deleted_by', 'delete_reason')`, table)
}

func (s *CatalogSoftDeleteMigrationSuite) exists(catalog, name string) bool {
	q := `SELECT count(*)::text FROM pg_constraint WHERE conname = $1`
	if catalog == "index" {
		q = `SELECT count(*)::text FROM pg_indexes WHERE indexname = $1`
	}
	return s.scalar(q, name) == "1"
}

// indexDef — определение индекса (pg_indexes.indexdef).
func (s *CatalogSoftDeleteMigrationSuite) indexDef(name string) string {
	return s.scalar(`SELECT indexdef FROM pg_indexes WHERE indexname = $1`, name)
}

// TS-012 (DS-022, DS-030, DS-017): up — существующие записи четырёх таблиц действующие, архивная
// остаётся архивной; CHECK отклоняет частичное заполнение, пустую и слишком длинную причину;
// название удалённой категории (в пределах вида) и удалённого типа свободно, среди действующих
// — по-прежнему уникально; down / up. Негатив: только deleted_at без deleted_by → ошибка CHECK.
func (s *CatalogSoftDeleteMigrationSuite) TestTS012_Migration0014_UpDown() {
	ctx := context.Background()
	mig, err := migrate.New(migrationsURL(), s.DSN)
	s.Require().NoError(err)
	defer func() { _, _ = mig.Close() }()

	s.Require().NoError(mig.Migrate(13))
	defer func() {
		if err := mig.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			s.Require().NoError(err)
		}
	}()
	for _, t := range catTables {
		s.Empty(s.columns(t), "0013: колонок удаления на %s нет", t)
	}

	// ---- данные «до» (0013): категории обоих видов, товар с остатком, архивная услуга, тип ----
	s.exec(`INSERT INTO users (id, login, password_hash, name, role) VALUES ($1, 'cat-mig-admin', 'x', 'Admin', 'admin') ON CONFLICT (id) DO NOTHING`, catAdminID)
	ids := map[string]string{}
	one := func(key, q string, args ...any) {
		var id string
		s.Require().NoError(s.Pool.QueryRow(ctx, q, args...).Scan(&id), q)
		ids[key] = id
	}
	one("catalog_categories", `INSERT INTO catalog_categories (kind, name) VALUES ('product', 'Категория до 0014') RETURNING id`)
	one("svc_category", `INSERT INTO catalog_categories (kind, name) VALUES ('service', 'Категория до 0014') RETURNING id`)
	one("products", `INSERT INTO products (category_id, name) VALUES ($1, 'Товар до 0014') RETURNING id`, ids["catalog_categories"])
	one("services", `INSERT INTO services (category_id, name, is_active) VALUES ($1, 'Услуга до 0014', false) RETURNING id`, ids["svc_category"])
	one("equipment_types", `INSERT INTO equipment_types (name) VALUES ('Тип до 0014') RETURNING id`)
	s.exec(`UPDATE products SET stock = 7 WHERE id = $1`, ids["products"])
	defer func() {
		// порядок — от ссылающихся к справочникам (FK RESTRICT).
		s.exec(`DELETE FROM products WHERE category_id = ANY($1::uuid[])`, []string{ids["catalog_categories"]})
		s.exec(`DELETE FROM services WHERE category_id = ANY($1::uuid[])`, []string{ids["svc_category"]})
		s.exec(`DELETE FROM catalog_categories WHERE name IN ('Категория до 0014', 'Категория 0014 новая')`)
		s.exec(`DELETE FROM equipment_types WHERE name IN ('Тип до 0014', 'Тип 0014 второй')`)
	}()

	// ---- up → 0014 ----
	s.Require().NoError(mig.Migrate(14))
	for _, t := range catTables {
		s.Equal("delete_reason:text:YES,deleted_at:timestamp with time zone:YES,deleted_by:uuid:YES", s.columns(t), t)
		s.True(s.exists("constraint", t+"_deleted_chk"), t)
		s.True(s.exists("index", t+"_deleted_at_idx"), t)
		s.Contains(s.indexDef(t+"_deleted_at_idx"), "WHERE (deleted_at IS NOT NULL)", t)
		s.Equal("0", s.scalar(`SELECT count(*)::text FROM `+t+` WHERE deleted_at IS NOT NULL OR deleted_by IS NOT NULL OR delete_reason IS NOT NULL`),
			"все существующие записи %s — действующие", t)
	}
	s.Equal("false", s.scalar(`SELECT is_active::text FROM services WHERE id = $1`, ids["services"]), "признак архива не изменился")
	s.Equal("7.000", s.scalar(`SELECT stock::text FROM products WHERE id = $1`, ids["products"]), "остаток на месте")
	// уникальные индексы названий — частичные (DS-017).
	for _, i := range []string{"catalog_categories_kind_name_lower_uq", "equipment_types_name_lower_uq"} {
		def := s.indexDef(i)
		s.Contains(def, "UNIQUE", i)
		s.Contains(def, "WHERE (deleted_at IS NULL)", i)
	}

	// негатив: «три поля вместе» и причина 1–500 символов — одинаково для четырёх таблиц.
	for _, table := range catTables {
		id, chk := ids[table], table+"_deleted_chk"
		s.fails(chk, table+": только deleted_at", `UPDATE `+table+` SET deleted_at = now() WHERE id = $1`, id)
		s.fails(chk, table+": без deleted_by", `UPDATE `+table+` SET deleted_at = now(), delete_reason = 'x' WHERE id = $1`, id)
		s.fails(chk, table+": без причины", `UPDATE `+table+` SET deleted_at = now(), deleted_by = $2 WHERE id = $1`, id, catAdminID)
		s.fails(chk, table+": только причина", `UPDATE `+table+` SET delete_reason = 'x' WHERE id = $1`, id)
		s.fails(chk, table+": только deleted_by", `UPDATE `+table+` SET deleted_by = $2 WHERE id = $1`, id, catAdminID)
		s.fails(chk, table+": причина из пробелов",
			`UPDATE `+table+` SET deleted_at = now(), deleted_by = $2, delete_reason = '   ' WHERE id = $1`, id, catAdminID)
		s.fails(chk, table+": причина 501 символ",
			`UPDATE `+table+` SET deleted_at = now(), deleted_by = $2, delete_reason = $3 WHERE id = $1`, id, catAdminID, strings.Repeat("я", 501))
		s.fails(table+"_deleted_by_fkey", table+": несуществующий удаливший",
			`UPDATE `+table+` SET deleted_at = now(), deleted_by = 'aaaaaaaa-0014-0014-0014-00000000dead', delete_reason = 'x' WHERE id = $1`, id)
	}

	// позитив: полное заполнение проходит; причина 500 символов (символов, не байт) допустима.
	for _, table := range catTables {
		s.exec(`UPDATE `+table+` SET deleted_at = now(), deleted_by = $2, delete_reason = $3 WHERE id = $1`, ids[table], catAdminID, strings.Repeat("я", 500))
	}
	// мягкое удаление товара остаток не трогает.
	s.Equal("7.000", s.scalar(`SELECT stock::text FROM products WHERE id = $1`, ids["products"]))

	// DS-017: название удалённой категории свободно в её виде — и только среди действующих оно
	// уникально (без учёта регистра и крайних пробелов).
	s.exec(`INSERT INTO catalog_categories (kind, name) VALUES ('product', '  КАТЕГОРИЯ ДО 0014 ')`)
	s.fails("catalog_categories_kind_name_lower_uq", "две действующие категории одного вида",
		`INSERT INTO catalog_categories (kind, name) VALUES ('product', 'категория до 0014')`)
	s.fails("catalog_categories_kind_name_lower_uq", "восстановление при занятом названии",
		`UPDATE catalog_categories SET deleted_at = NULL, deleted_by = NULL, delete_reason = NULL WHERE id = $1`, ids["catalog_categories"])
	s.fails("catalog_categories_kind_name_lower_uq", "тот же вид service — категория действует",
		`INSERT INTO catalog_categories (kind, name) VALUES ('service', 'Категория до 0014')`)
	s.exec(`UPDATE catalog_categories SET name = 'Категория 0014 новая' WHERE kind = 'product' AND deleted_at IS NULL AND lower(btrim(name)) = 'категория до 0014'`)
	s.exec(`UPDATE catalog_categories SET deleted_at = NULL, deleted_by = NULL, delete_reason = NULL WHERE id = $1`, ids["catalog_categories"])
	// то же для типов.
	s.exec(`INSERT INTO equipment_types (name) VALUES ('тип ДО 0014')`)
	s.fails("equipment_types_name_lower_uq", "два действующих типа",
		`INSERT INTO equipment_types (name) VALUES ('Тип до 0014')`)
	s.fails("equipment_types_name_lower_uq", "восстановление типа при занятом названии",
		`UPDATE equipment_types SET deleted_at = NULL, deleted_by = NULL, delete_reason = NULL WHERE id = $1`, ids["equipment_types"])
	s.exec(`UPDATE equipment_types SET name = 'Тип 0014 второй' WHERE deleted_at IS NULL AND lower(btrim(name)) = 'тип до 0014'`)
	s.exec(`UPDATE equipment_types SET deleted_at = NULL, deleted_by = NULL, delete_reason = NULL WHERE id = $1`, ids["equipment_types"])

	// FK deleted_by → users RESTRICT: автора удаления стереть нельзя.
	s.exec(`INSERT INTO users (id, login, password_hash, name, role) VALUES ($1, 'cat-mig-second', 'x', 'Second', 'admin') ON CONFLICT (id) DO NOTHING`, catSecondID)
	s.exec(`UPDATE services SET deleted_by = $2 WHERE id = $1`, ids["services"], catSecondID)
	_, err = s.Pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, catSecondID)
	s.Error(err, "удаление пользователя, удалившего услугу (RESTRICT)")

	// ---- down → 0013: колонки и ограничения ушли, записи остались, индексы названий — полные ----
	s.Require().NoError(mig.Migrate(13))
	for _, t := range catTables {
		s.Empty(s.columns(t), t)
		s.False(s.exists("constraint", t+"_deleted_chk"), t)
		s.False(s.exists("index", t+"_deleted_at_idx"), t)
		s.Equal("1", s.scalar(`SELECT count(*)::text FROM `+t+` WHERE id = $1`, ids[t]), t)
	}
	for _, i := range []string{"catalog_categories_kind_name_lower_uq", "equipment_types_name_lower_uq"} {
		s.NotContains(s.indexDef(i), "WHERE", i)
	}
	s.exec(`DELETE FROM users WHERE id = $1`, catSecondID) // FK ушёл вместе с колонкой

	// повторный up поверх отката проходит; записи снова действующие.
	s.Require().NoError(mig.Migrate(14))
	for _, t := range catTables {
		s.Equal("0", s.scalar(`SELECT count(*)::text FROM `+t+` WHERE id = $1 AND deleted_at IS NOT NULL`, ids[t]), t)
	}
}
