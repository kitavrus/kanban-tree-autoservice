//go:build integration

package warehouses_test

import (
	"context"
	"errors"
	"strings"

	"github.com/golang-migrate/migrate/v4"
)

// TS-012, откат 0023 на данных с одноимёнными записями (DS-017 разрешил занять название
// удалённой записи): накат → удалённые и действующие склады и поставщики с одинаковыми
// названиями → откат → накат. Откат обязан пройти: удалённая запись, чьё название занято,
// получает суффикс « (удал. N)» (наименьший свободный N, обход по (deleted_at, id)); действующие
// не переименовываются; из одноимённых удалённых название сохраняет удалённая последней;
// название обрезается только под лимит поля; все записи остаются и снова действующие.
func (s *WarehousesSoftDeleteMigrationSuite) TestTS012_Migration0023_RollbackWithNamesakes() {
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
	s.Require().NoError(mig.Migrate(23))

	s.exec(`INSERT INTO users (id, login, password_hash, name, role) VALUES ($1, 'whsd-admin', 'x', 'Admin', 'admin') ON CONFLICT (id) DO NOTHING`, whsdAdminID)
	created := map[string][]string{}
	defer func() {
		for _, t := range whsdTables {
			s.exec(`DELETE FROM `+t+` WHERE id = ANY($1::uuid[])`, created[t])
		}
	}()
	// add — запись table с названием name; ago != "" — мягко удалена столько времени назад.
	add := func(table, name, ago string) string {
		q := `INSERT INTO suppliers (name) VALUES ($1) RETURNING id`
		if table == "warehouses" {
			q = `INSERT INTO warehouses (name, position) VALUES ($1, 60) RETURNING id`
		}
		var id string
		s.Require().NoError(s.Pool.QueryRow(ctx, q, name).Scan(&id), name)
		created[table] = append(created[table], id)
		if ago != "" {
			s.exec(`UPDATE `+table+` SET deleted_at = now() - $2::interval, deleted_by = $3, delete_reason = 'Закрыт' WHERE id = $1`, id, ago, whsdAdminID)
		}
		return id
	}
	name := func(table, id string) string { return s.scalar(`SELECT name FROM `+table+` WHERE id = $1`, id) }

	for table, limit := range map[string]int{"warehouses": 100, "suppliers": 200} {
		long := strings.Repeat("я", limit)
		// удаляем по очереди: запись с занятым названием создаётся только после удаления прежней.
		first := add(table, "Дубль 0023", "3 day")
		second := add(table, "дубль 0023 ", "2 day")
		active := add(table, " ДУБЛЬ 0023", "")
		taken := add(table, "Дубль 0023 (удал. 1)", "") // суффикс уже занят действующей записью
		pairOld := add(table, "Пара 0023", "2 day")
		pairNew := add(table, "пара 0023", "1 day") // одноимённые удалённые без действующей
		longDeleted := add(table, long, "1 day")
		longActive := add(table, strings.Repeat("Я", limit), "")
		alone := add(table, "Одиночка 0023", "1 day") // название свободно — не переименовывается

		// ---- down → 0022: проходит, несмотря на одноимённые записи ----
		s.Require().NoError(mig.Migrate(22), table)

		s.Equal(" ДУБЛЬ 0023", name(table, active), "%s: действующая запись не переименована", table)
		s.Equal("Дубль 0023 (удал. 1)", name(table, taken), table)
		s.Equal("Дубль 0023 (удал. 2)", name(table, first), "%s: «удал. 1» занято — берётся следующий номер", table)
		s.Equal("дубль 0023 (удал. 3)", name(table, second), "%s: крайние пробелы убраны, регистр сохранён", table)
		s.Equal("Пара 0023 (удал. 1)", name(table, pairOld), table)
		s.Equal("пара 0023", name(table, pairNew), "%s: название сохраняет удалённая последней", table)
		s.Equal("Одиночка 0023", name(table, alone), table)
		s.Equal(strings.Repeat("Я", limit), name(table, longActive), table)
		s.Equal(strings.Repeat("я", limit-len([]rune(" (удал. 1)")))+" (удал. 1)", name(table, longDeleted),
			"%s: название обрезано ровно под лимит %d", table, limit)
		s.Equal(limit, len([]rune(name(table, longDeleted))), table)

		s.Empty(s.columns(table), table)
		s.False(s.exists("constraint", table+"_deleted_chk"), table)
		s.False(s.exists("index", table+"_deleted_at_idx"), table)
		def := s.indexDef(table + "_name_lower_uq")
		s.Contains(def, "UNIQUE", table)
		s.NotContains(def, "WHERE", "%s: индекс названия — полный", table)
		s.Equal("9", s.scalar(`SELECT count(*)::text FROM `+table+` WHERE id = ANY($1::uuid[])`, created[table]), "%s: записи не потеряны", table)
		s.fails(table+"_name_lower_uq", table+": полный индекс работает",
			`UPDATE `+table+` SET name = 'пара 0023' WHERE id = $1`, pairOld)

		// ---- повторный up: бывшие удалённые — действующие, названия отката сохраняются ----
		s.Require().NoError(mig.Migrate(23), table)
		s.Equal("0", s.scalar(`SELECT count(*)::text FROM `+table+` WHERE deleted_at IS NOT NULL OR deleted_by IS NOT NULL OR delete_reason IS NOT NULL`), table)
		s.Equal("Дубль 0023 (удал. 2)", name(table, first), table)
		s.Contains(s.indexDef(table+"_name_lower_uq"), "WHERE (deleted_at IS NULL)", table)

		// второй откат на уже разведённых названиях ничего не переименовывает (идемпотентность).
		s.exec(`UPDATE `+table+` SET deleted_at = now(), deleted_by = $2, delete_reason = 'Снова' WHERE id = $1`, first, whsdAdminID)
		s.Require().NoError(mig.Migrate(22), table)
		s.Equal("Дубль 0023 (удал. 2)", name(table, first), table)
		s.Require().NoError(mig.Migrate(23), table)

		s.exec(`DELETE FROM `+table+` WHERE id = ANY($1::uuid[])`, created[table])
		created[table] = nil
	}
	s.Equal("Алматы", s.defaultWarehouse())
}
