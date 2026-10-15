//go:build integration

package board_test

import (
	"context"
	"errors"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/stretchr/testify/suite"

	"github.com/kitavrus/kanban-tree/pkg/testutil"
)

// OneCarOrderMigrationSuite — миграция 0017 (one-car-order): бэкфилл tasks.car_id из
// task_equipment / первой машины клиента / новой машины из названия заказа, затем NOT NULL,
// индекс и DROP task_equipment, subtasks.equipment_id; down восстанавливает обе из car_id.
type OneCarOrderMigrationSuite struct {
	testutil.BaseSuite
}

func TestOneCarOrderMigrationSuite(t *testing.T) {
	suite.Run(t, new(OneCarOrderMigrationSuite))
}

const ocoAdminID = "aaaaaaaa-0017-0017-0017-000000000001"

func (s *OneCarOrderMigrationSuite) scalar(q string, args ...any) string {
	var v string
	s.Require().NoError(s.Pool.QueryRow(context.Background(), q, args...).Scan(&v), q)
	return v
}

func (s *OneCarOrderMigrationSuite) exec(q string, args ...any) {
	_, err := s.Pool.Exec(context.Background(), q, args...)
	s.Require().NoError(err, q)
}

func (s *OneCarOrderMigrationSuite) id(q string, args ...any) string { return s.scalar(q, args...) }

// TestMigration0017_Backfill_UpDown — случаи бэкфилла (код-ревью MEDIUM-1, LOW-2):
//
//	(a) в task_equipment две свои машины и чужая → первая СВОЯ по госномеру (чужая пропущена);
//	(e) несколько своих машин, с одной чаще работали в услугах → она, вопреки порядку госномеров;
//	(b) task_equipment нет / только чужая машина → первая машина клиента, действующая раньше
//	    удалённой;
//	(c) у клиента машин нет → машина из названия: допустимые символы, верхний регистр, ≤ 20
//	    символов; одинаковое название двух заказов — одна машина; название из спецсимволов —
//	    «БЕЗ НОМЕРА»;
//	(d) мягко удалённый клиент без машин → новая машина удалена «вместе с клиентом» с его
//	    «кто, когда, почему».
//
// После up — car_id NOT NULL, FK, индекс, владелец машины = клиент заказа, task_equipment и
// subtasks.equipment_id нет. Down → task_equipment и subtasks.equipment_id восстановлены из car_id.
func (s *OneCarOrderMigrationSuite) TestMigration0017_Backfill_UpDown() {
	mig, err := migrate.New(migrateRoot(), s.DSN)
	s.Require().NoError(err)
	defer func() { _, _ = mig.Close() }()

	s.Require().NoError(mig.Migrate(16))
	defer func() {
		if err := mig.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			s.Require().NoError(err)
		}
	}()
	s.Equal("1", s.scalar(`SELECT count(*)::text FROM information_schema.tables WHERE table_name = 'task_equipment'`), "0016: task_equipment есть")
	s.Equal("0", s.scalar(`SELECT count(*)::text FROM information_schema.columns WHERE table_name = 'tasks' AND column_name = 'car_id'`))

	// ---- данные «до» (0016) ----
	s.exec(`INSERT INTO users (id, login, password_hash, name, role) VALUES ($1, 'oco-admin', 'x', 'Admin', 'admin') ON CONFLICT (id) DO NOTHING`, ocoAdminID)
	column := s.id(`SELECT id::text FROM columns WHERE deleted_at IS NULL ORDER BY position, id LIMIT 1`)
	client := func(name string) string {
		return s.id(`INSERT INTO clients (company_name) VALUES ($1) RETURNING id::text`, name)
	}
	car := func(clientID, plate string) string {
		return s.id(`INSERT INTO equipment (client_id, plate_number) VALUES ($1, $2) RETURNING id::text`, clientID, plate)
	}
	task := func(clientID, title string) string {
		return s.id(`INSERT INTO tasks (column_id, client_id, title, position, created_by) VALUES ($1, $2, $3, 1, $4) RETURNING id::text`,
			column, clientID, title, ocoAdminID)
	}
	attach := func(taskID, carID string) {
		s.exec(`INSERT INTO task_equipment (task_id, equipment_id) VALUES ($1::bigint, $2)`, taskID, carID)
	}
	catID := s.id(`INSERT INTO catalog_categories (kind, name) VALUES ('service', 'OCO услуги ' || gen_random_uuid()) RETURNING id::text`)
	svcID := s.id(`INSERT INTO services (category_id, name) VALUES ($1, 'OCO услуга') RETURNING id::text`, catID)
	subtask := func(taskID string, carID any) string {
		return s.id(`INSERT INTO subtasks (task_id, service_id, equipment_id) VALUES ($1::bigint, $2, $3) RETURNING id::text`, taskID, svcID, carID)
	}

	cA, cF, cB, cC, cD := client("OCO клиент A"), client("OCO чужой F"), client("OCO клиент B"), client("OCO клиент C"), client("OCO клиент D")
	var tasks []string
	defer func() {
		ctx := context.Background()
		for _, t := range tasks {
			_, _ = s.Pool.Exec(ctx, `DELETE FROM tasks WHERE id = $1::bigint`, t)
		}
		for _, c := range []string{cA, cF, cB, cC, cD} {
			_, _ = s.Pool.Exec(ctx, `DELETE FROM clients WHERE id = $1`, c) // машины — каскадом
		}
		_, _ = s.Pool.Exec(ctx, `DELETE FROM services WHERE id = $1`, svcID)
		_, _ = s.Pool.Exec(ctx, `DELETE FROM catalog_categories WHERE id = $1`, catID)
		_, _ = s.Pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, ocoAdminID)
	}()

	// (a): свои Б222/А111 и чужая А000 (по алфавиту первая, но не своя) → А111.
	a1, a2, f1 := car(cA, "Б222 ОК"), car(cA, "А111 ОК"), car(cF, "А000 ЧУЖ")
	tA := task(cA, "OCO заказ A")
	attach(tA, a1)
	attach(tA, a2)
	attach(tA, f1)
	// (e): обе свои, в услугах чаще Б222 (2 против 1) → Б222, хотя А111 раньше по алфавиту.
	tE := task(cA, "OCO заказ E")
	attach(tE, a1)
	attach(tE, a2)
	subE := []string{subtask(tE, a1), subtask(tE, a1), subtask(tE, a2), subtask(tE, nil)}
	// (b): у B удалённая А000 и действующая Я999; заказ без task_equipment и заказ только с чужой.
	bGone, bLive := car(cB, "А000 УД"), car(cB, "Я999 ЖИВ")
	s.exec(`UPDATE equipment SET deleted_at = now(), deleted_by = $2, delete_reason = 'Продан' WHERE id = $1`, bGone, ocoAdminID)
	tB := task(cB, "OCO заказ B")
	tB2 := task(cB, "OCO заказ B чужая")
	attach(tB2, f1)
	// (c): у C машин нет.
	tC1 := task(cC, "Ремонт: двигатель!!! и коробка передач, срочно")
	tC2 := task(cC, "Ремонт: двигатель!!! и коробка передач, срочно")
	tC3 := task(cC, "!!! ??? ***")
	// (d): мягко удалённый клиент без машин.
	s.exec(`UPDATE clients SET deleted_at = '2026-10-01T10:00:00Z', deleted_by = $2, delete_reason = 'Закрылся' WHERE id = $1`, cD, ocoAdminID)
	tD := task(cD, "т555тт 55")
	tasks = append(tasks, tA, tE, tB, tB2, tC1, tC2, tC3, tD)

	// ---- up → 0017 ----
	s.Require().NoError(mig.Migrate(17))
	s.Equal("NO", s.scalar(`SELECT is_nullable FROM information_schema.columns WHERE table_name = 'tasks' AND column_name = 'car_id'`), "car_id NOT NULL")
	s.Equal("1", s.scalar(`SELECT count(*)::text FROM pg_indexes WHERE indexname = 'tasks_car_id_idx'`))
	s.Equal("1", s.scalar(`SELECT count(*)::text FROM pg_constraint WHERE conname = 'tasks_car_id_fkey'`))
	s.Equal("0", s.scalar(`SELECT count(*)::text FROM information_schema.tables WHERE table_name = 'task_equipment'`), "task_equipment удалена")
	s.Equal("0", s.scalar(`SELECT count(*)::text FROM information_schema.columns WHERE table_name = 'subtasks' AND column_name = 'equipment_id'`))
	s.Equal("0", s.scalar(`SELECT count(*)::text FROM tasks WHERE car_id IS NULL`))

	carOf := func(taskID string) string {
		return s.scalar(`SELECT car_id::text FROM tasks WHERE id = $1::bigint`, taskID)
	}
	s.Equal(a2, carOf(tA), "(a) первая своя по госномеру, чужая пропущена")
	s.Equal(a1, carOf(tE), "(e) машина, с которой чаще работали в услугах")
	s.Equal(bLive, carOf(tB), "(b) действующая раньше удалённой")
	s.Equal(bLive, carOf(tB2), "(b) только чужая в task_equipment → первая машина клиента")

	s.Equal(carOf(tC1), carOf(tC2), "(c) одинаковое название — одна машина")
	s.NotEqual(carOf(tC1), carOf(tC3))
	s.Equal("РЕМОНТ ДВИГАТЕЛЬ И К", s.scalar(`SELECT plate_number FROM equipment WHERE id = $1`, carOf(tC1)), "(c) госномер из названия ≤ 20")
	s.Equal("БЕЗ НОМЕРА", s.scalar(`SELECT plate_number FROM equipment WHERE id = $1`, carOf(tC3)), "(c) название из спецсимволов")
	s.Equal("2", s.scalar(`SELECT count(*)::text FROM equipment WHERE client_id = $1 AND deleted_at IS NULL`, cC), "(c) клиенту заведены две машины")

	s.Equal("Т555ТТ 55|true|true|Закрылся|true", s.scalar(`
		SELECT e.plate_number || '|' || (e.deleted_at = c.deleted_at)::text || '|' || (e.deleted_by = c.deleted_by)::text || '|' ||
		       e.delete_reason || '|' || e.deleted_with_client::text
		FROM tasks t JOIN equipment e ON e.id = t.car_id JOIN clients c ON c.id = t.client_id WHERE t.id = $1::bigint`, tD),
		"(d) машина удалённого клиента — удалена вместе с ним")

	s.Equal("0", s.scalar(`
		SELECT count(*)::text FROM tasks t JOIN equipment e ON e.id = t.car_id
		WHERE t.id::text = ANY($1) AND e.client_id <> t.client_id`, tasks), "владелец машины = клиент заказа")

	// ---- down → 0016: task_equipment и subtasks.equipment_id — из car_id ----
	s.Require().NoError(mig.Migrate(16))
	s.Equal("0", s.scalar(`SELECT count(*)::text FROM information_schema.columns WHERE table_name = 'tasks' AND column_name = 'car_id'`))
	for _, tc := range []struct{ task, car string }{{tA, a2}, {tE, a1}, {tB, bLive}, {tB2, bLive}} {
		s.Equal(tc.car+"|1", s.scalar(`
			SELECT string_agg(equipment_id::text, ',') || '|' || count(*)::text || ''
			FROM task_equipment WHERE task_id = $1::bigint`, tc.task), "task_equipment заказа %s", tc.task)
	}
	s.Equal("|", s.scalar(`SELECT coalesce(completeness, '-') || '|' || coalesce(condition, '-') FROM task_equipment WHERE task_id = $1::bigint`, tA),
		"комплектность/состояние после отката пустые")
	for _, sub := range subE {
		s.Equal(a1, s.scalar(`SELECT equipment_id::text FROM subtasks WHERE id = $1`, sub), "услуга заказа — машина заказа")
	}

	// повторный up поверх отката проходит, выбор машин прежний.
	s.Require().NoError(mig.Migrate(17))
	s.Equal(a2, carOf(tA))
	s.Equal(a1, carOf(tE))
}
