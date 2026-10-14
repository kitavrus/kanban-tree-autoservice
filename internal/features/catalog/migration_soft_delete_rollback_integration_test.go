//go:build integration

package catalog_test

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/golang-migrate/migrate/v4"
)

// TS-012, откат 0014 на данных с одноимёнными записями (DS-017 разрешил занять название
// удалённой категории в пределах вида и удалённого типа оборудования): накат → удалённые и
// действующие записи с одинаковыми названиями → откат → накат. Откат обязан пройти: удалённая
// запись, чьё название занято, получает суффикс « (удал. N)» (наименьший свободный N, обход по
// (deleted_at, id)); действующие не переименовываются; категории разных видов друг другу не
// мешают; товары и услуги (названия не уникальны) не переименовываются; все записи остаются.
func (s *CatalogSoftDeleteMigrationSuite) TestTS012_Migration0014_RollbackWithNamesakes() {
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
	s.Require().NoError(mig.Migrate(14))

	s.exec(`INSERT INTO users (id, login, password_hash, name, role) VALUES ($1, 'cat-mig-admin', 'x', 'Admin', 'admin') ON CONFLICT (id) DO NOTHING`, catAdminID)
	created := map[string][]string{}
	defer func() {
		// порядок — от ссылающихся к справочникам (FK RESTRICT).
		for _, t := range []string{"products", "services", "catalog_categories", "equipment_types"} {
			s.exec(`DELETE FROM `+t+` WHERE id = ANY($1::uuid[])`, created[t])
		}
	}()
	// add — запись table (insert — её INSERT … RETURNING id); ago != "" — мягко удалена.
	add := func(table, ago, insert string, args ...any) string {
		var id string
		s.Require().NoError(s.Pool.QueryRow(ctx, insert, args...).Scan(&id), insert)
		created[table] = append(created[table], id)
		if ago != "" {
			s.exec(`UPDATE `+table+` SET deleted_at = now() - $2::interval, deleted_by = $3, delete_reason = 'Не нужна' WHERE id = $1`, id, ago, catAdminID)
		}
		return id
	}
	category := func(kind, name, ago string) string {
		return add("catalog_categories", ago, `INSERT INTO catalog_categories (kind, name) VALUES ($1, $2) RETURNING id`, kind, name)
	}
	eqType := func(name, ago string) string {
		return add("equipment_types", ago, `INSERT INTO equipment_types (name) VALUES ($1) RETURNING id`, name)
	}
	name := func(table, id string) string { return s.scalar(`SELECT name FROM `+table+` WHERE id = $1`, id) }
	long := strings.Repeat("я", 100)

	// категории: удаляем по очереди — запись с занятым названием создаётся после удаления прежней.
	catFirst := category("product", "Дубль 0014", "3 day")
	catSecond := category("product", "дубль 0014 ", "2 day")
	catActive := category("product", " ДУБЛЬ 0014", "")
	catTaken := category("product", "Дубль 0014 (удал. 1)", "") // суффикс занят действующей записью
	catOtherKind := category("service", "Дубль 0014", "1 day")  // другой вид — название свободно
	catPairOld := category("service", "Пара 0014", "2 day")
	catPairNew := category("service", "пара 0014", "1 day") // одноимённые удалённые без действующей
	catLongDeleted := category("product", long, "1 day")
	catLongActive := category("product", strings.Repeat("Я", 100), "")
	// типы оборудования.
	typeDeleted := eqType("Тип-дубль 0014", "2 day")
	typeActive := eqType("ТИП-ДУБЛЬ 0014", "")
	typeAlone := eqType("Тип-одиночка 0014", "1 day")
	// товары и услуги: названия не уникальны — удалённый и действующий тёзки сосуществуют.
	prodDeleted := add("products", "1 day", `INSERT INTO products (category_id, name) VALUES ($1, 'Товар-дубль 0014') RETURNING id`, catActive)
	prodActive := add("products", "", `INSERT INTO products (category_id, name) VALUES ($1, 'Товар-дубль 0014') RETURNING id`, catActive)
	svcDeleted := add("services", "1 day", `INSERT INTO services (category_id, name, is_active) VALUES ($1, 'Услуга-дубль 0014', false) RETURNING id`, catPairNew)
	svcActive := add("services", "", `INSERT INTO services (category_id, name) VALUES ($1, 'Услуга-дубль 0014') RETURNING id`, catPairNew)

	// ---- down → 0013: проходит, несмотря на одноимённые записи ----
	s.Require().NoError(mig.Migrate(13))

	const cc, et = "catalog_categories", "equipment_types"
	s.Equal(" ДУБЛЬ 0014", name(cc, catActive), "действующая категория не переименована")
	s.Equal("Дубль 0014 (удал. 1)", name(cc, catTaken))
	s.Equal("Дубль 0014 (удал. 2)", name(cc, catFirst), "«удал. 1» занято — берётся следующий номер")
	s.Equal("дубль 0014 (удал. 3)", name(cc, catSecond), "крайние пробелы убраны, регистр сохранён")
	s.Equal("Дубль 0014", name(cc, catOtherKind), "уникальность — в пределах вида")
	s.Equal("Пара 0014 (удал. 1)", name(cc, catPairOld))
	s.Equal("пара 0014", name(cc, catPairNew), "название сохраняет удалённая последней")
	s.Equal(strings.Repeat("Я", 100), name(cc, catLongActive))
	s.Equal(strings.Repeat("я", 100-len([]rune(" (удал. 1)")))+" (удал. 1)", name(cc, catLongDeleted), "название обрезано под лимит формы 100")
	s.Equal("Тип-дубль 0014 (удал. 1)", name(et, typeDeleted))
	s.Equal("ТИП-ДУБЛЬ 0014", name(et, typeActive))
	s.Equal("Тип-одиночка 0014", name(et, typeAlone), "свободное название не меняется")
	for table, ids := range map[string][]string{"products": {prodDeleted, prodActive}, "services": {svcDeleted, svcActive}} {
		s.Equal("2", s.scalar(`SELECT count(*)::text FROM `+table+` WHERE id = ANY($1::uuid[]) AND name LIKE '%-дубль 0014'`, ids),
			"%s: названия не тронуты", table)
	}
	s.Equal("false", s.scalar(`SELECT is_active::text FROM services WHERE id = $1`, svcDeleted), "признак архива не меняется")

	for _, t := range catTables {
		s.Empty(s.columns(t), t)
		s.False(s.exists("constraint", t+"_deleted_chk"), t)
		s.False(s.exists("index", t+"_deleted_at_idx"), t)
		s.Equal(strconv.Itoa(len(created[t])), s.scalar(`SELECT count(*)::text FROM `+t+` WHERE id = ANY($1::uuid[])`, created[t]),
			"%s: записи не потеряны", t)
	}
	for _, idx := range []string{"catalog_categories_kind_name_lower_uq", "equipment_types_name_lower_uq"} {
		def := s.indexDef(idx)
		s.Contains(def, "UNIQUE", idx)
		s.NotContains(def, "WHERE", "%s — полный", idx)
	}
	s.fails("catalog_categories_kind_name_lower_uq", "полный индекс категорий работает",
		`UPDATE catalog_categories SET name = 'ПАРА 0014' WHERE id = $1`, catPairOld)
	s.fails("equipment_types_name_lower_uq", "полный индекс типов работает",
		`UPDATE equipment_types SET name = 'тип-дубль 0014' WHERE id = $1`, typeDeleted)

	// ---- повторный up: бывшие удалённые — действующие, названия отката сохраняются ----
	s.Require().NoError(mig.Migrate(14))
	for _, t := range catTables {
		s.Equal("0", s.scalar(`SELECT count(*)::text FROM `+t+` WHERE deleted_at IS NOT NULL OR deleted_by IS NOT NULL OR delete_reason IS NOT NULL`), t)
	}
	s.Equal("Дубль 0014 (удал. 2)", name(cc, catFirst))
	s.Contains(s.indexDef("catalog_categories_kind_name_lower_uq"), "WHERE (deleted_at IS NULL)")
	s.Contains(s.indexDef("equipment_types_name_lower_uq"), "WHERE (deleted_at IS NULL)")
}
