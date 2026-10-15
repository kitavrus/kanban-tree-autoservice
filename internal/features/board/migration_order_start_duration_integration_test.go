//go:build integration

package board_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/stretchr/testify/suite"

	"github.com/kitavrus/kanban-tree/pkg/testutil"
	"github.com/kitavrus/kanban-tree/pkg/timezone"
)

// OrderStartDurationMigrationSuite — миграция 0018 (order-start-duration): бэкфилл
// tasks.start_date (день приёмки в поясе Asia/Almaty) и duration_days (до прежнего срока, ≤ 60,
// иначе 1), due_date — вычисляемое плановое окончание; down возвращает due_date обычной колонкой
// со значением плана.
type OrderStartDurationMigrationSuite struct {
	testutil.BaseSuite
}

func TestOrderStartDurationMigrationSuite(t *testing.T) {
	suite.Run(t, new(OrderStartDurationMigrationSuite))
}

const osdAdminID = "aaaaaaaa-0018-0018-0018-000000000001"

func (s *OrderStartDurationMigrationSuite) scalar(q string, args ...any) string {
	var v string
	s.Require().NoError(s.Pool.QueryRow(context.Background(), q, args...).Scan(&v), q)
	return v
}

func (s *OrderStartDurationMigrationSuite) exec(q string, args ...any) {
	_, err := s.Pool.Exec(context.Background(), q, args...)
	s.Require().NoError(err, q)
}

// TestMigration0018_Backfill_UpDown — случаи бэкфилла (решения пользователя, fix-loop 1):
//
//	(a) срок не раньше приёмки → дни = срок − начало + 1, плановое окончание = прежний срок;
//	(b) открытый, срок раньше дня приёмки → как без срока: до дня миграции (окончание — сегодня);
//	(c) открытый без срока → до дня миграции включительно (окончание — сегодня, «Горит», не
//	    «просрочено»);
//	(c2) открытый без срока, принят > 60 дней назад → 60 (кламп CHECK);
//	(d) срок дальше 60 дней от приёмки → 60;
//	(e) приёмка вечером по UTC — уже следующий день в Алматы → начало = дата по Алматы;
//	(f) срок в день приёмки → 1 день;
//	(g) выданный (финальная колонка) со сроком → до срока, как открытый;
//	(h) выданный без срока / со сроком раньше приёмки → 1.
//
// После up — start_date/duration_days NOT NULL с DEFAULT, CHECK 1..60, due_date GENERATED; down →
// due_date обычная колонка со значением плана, повторный up даёт те же начало и дни.
func (s *OrderStartDurationMigrationSuite) TestMigration0018_Backfill_UpDown() {
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
	s.Equal("0", s.scalar(`SELECT count(*)::text FROM information_schema.columns WHERE table_name = 'tasks' AND column_name = 'start_date'`))

	// «сегодня» и дни приёмки — по Алматы (пояс бэкфилла); моменты приёмки — относительно now.
	almaty := timezone.Load(timezone.Default)
	now := time.Now()
	day := func(t time.Time) string { return t.In(almaty).Format(time.DateOnly) }
	ago := func(days int) time.Time { return now.Add(-time.Duration(days) * 24 * time.Hour) }
	at := func(t time.Time) string { return t.UTC().Format(time.RFC3339) }
	today := day(now)
	planOf := func(start string, days int, end string) string { return fmt.Sprintf("%s|%d|%s", start, days, end) }

	// ---- данные «до» (0017) ----
	s.exec(`INSERT INTO users (id, login, password_hash, name, role) VALUES ($1, 'osd-admin', 'x', 'Admin', 'admin') ON CONFLICT (id) DO NOTHING`, osdAdminID)
	open := s.scalar(`SELECT id::text FROM columns WHERE deleted_at IS NULL AND NOT is_final ORDER BY position, id LIMIT 1`)
	final := s.scalar(`SELECT id::text FROM columns WHERE deleted_at IS NULL AND is_final ORDER BY position, id LIMIT 1`)
	client := s.scalar(`INSERT INTO clients (company_name) VALUES ('OSD клиент') RETURNING id::text`)
	car := s.scalar(`INSERT INTO equipment (client_id, plate_number) VALUES ($1, 'ОСД 018') RETURNING id::text`, client)
	task := func(column, title, createdAt string, due any) string {
		return s.scalar(`
			INSERT INTO tasks (column_id, client_id, car_id, title, position, created_by, created_at, due_date)
			VALUES ($1, $2, $3, $4, 1, $5, $6::timestamptz, $7::date) RETURNING id::text`,
			column, client, car, title, osdAdminID, createdAt, due)
	}
	tA := task(open, "OSD (a)", "2026-10-10T06:00:00Z", "2026-10-14")
	tB := task(open, "OSD (b)", at(ago(2)), day(ago(4)))
	tC := task(open, "OSD (c)", at(ago(3)), nil)
	tC2 := task(open, "OSD (c2)", at(ago(90)), nil)
	tD := task(open, "OSD (d)", "2026-10-10T06:00:00Z", "2027-01-30")
	tE := task(open, "OSD (e)", "2026-10-14T20:30:00Z", "2026-10-16") // 01:30 15.10 по Алматы (UTC+5)
	tF := task(open, "OSD (f)", "2026-10-10T06:00:00Z", "2026-10-10")
	tG := task(final, "OSD (g)", "2026-10-10T06:00:00Z", "2026-10-12")
	tH := task(final, "OSD (h)", at(ago(3)), nil)
	tH2 := task(final, "OSD (h2)", "2026-10-10T06:00:00Z", "2026-10-08")
	defer func() {
		_, _ = s.Pool.Exec(ctx, `DELETE FROM tasks WHERE client_id = $1`, client)
		_, _ = s.Pool.Exec(ctx, `DELETE FROM clients WHERE id = $1`, client) // машина — каскадом
		_, _ = s.Pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, osdAdminID)
	}()

	plan := func(id string) string {
		return s.scalar(`SELECT start_date::text || '|' || duration_days::text || '|' || due_date::text FROM tasks WHERE id = $1::bigint`, id)
	}

	// ---- up → 0018 ----
	s.Require().NoError(mig.Migrate(18))
	for _, col := range []string{"start_date", "duration_days"} {
		s.Equal("NO", s.scalar(`SELECT is_nullable FROM information_schema.columns WHERE table_name = 'tasks' AND column_name = $1`, col), "%s NOT NULL", col)
	}
	s.Equal("ALWAYS", s.scalar(`SELECT is_generated FROM information_schema.columns WHERE table_name = 'tasks' AND column_name = 'due_date'`), "due_date вычисляемая")
	s.Equal("1", s.scalar(`SELECT count(*)::text FROM pg_constraint WHERE conname = 'tasks_duration_days_chk'`))

	s.Equal("2026-10-10|5|2026-10-14", plan(tA), "(a) дни до прежнего срока")
	s.Equal(planOf(day(ago(2)), 3, today), plan(tB), "(b) открытый, срок раньше приёмки → до сегодня")
	s.Equal(planOf(day(ago(3)), 4, today), plan(tC), "(c) открытый без срока → окончание сегодня")
	s.Equal(planOf(day(ago(90)), 60, day(ago(31))), plan(tC2), "(c2) открытый без срока, принят > 60 дней назад → 60")
	s.Equal("2026-10-10|60|2026-12-08", plan(tD), "(d) кламп до 60 дней")
	s.Equal("2026-10-15|2|2026-10-16", plan(tE), "(e) начало — дата приёмки по Алматы")
	s.Equal("2026-10-10|1|2026-10-10", plan(tF), "(f) срок в день приёмки → 1")
	s.Equal("2026-10-10|3|2026-10-12", plan(tG), "(g) выданный со сроком — до срока")
	s.Equal(planOf(day(ago(3)), 1, day(ago(3))), plan(tH), "(h) выданный без срока → 1")
	s.Equal("2026-10-10|1|2026-10-10", plan(tH2), "(h) выданный, срок раньше приёмки → 1")

	// плановое окончание пересчитывается при правке, напрямую не пишется; CHECK 1..60.
	s.exec(`UPDATE tasks SET start_date = '2026-11-01', duration_days = 3 WHERE id = $1::bigint`, tC)
	s.Equal("2026-11-01|3|2026-11-03", plan(tC))
	_, err = s.Pool.Exec(ctx, `UPDATE tasks SET due_date = '2026-12-01' WHERE id = $1::bigint`, tC)
	s.Error(err, "due_date вычисляемая — запись запрещена")
	for _, days := range []int{0, 61} {
		_, err = s.Pool.Exec(ctx, `UPDATE tasks SET duration_days = $2 WHERE id = $1::bigint`, tC, days)
		s.Error(err, "CHECK duration_days: %d", days)
	}
	// прямой INSERT без новых полей — DEFAULT: сегодня по Алматы, 1 день.
	tI := s.scalar(`INSERT INTO tasks (column_id, client_id, car_id, title, position, created_by) VALUES ($1, $2, $3, 'OSD (i)', 1, $4) RETURNING id::text`,
		open, client, car, osdAdminID)
	s.Equal(planOf(today, 1, today), plan(tI), "DEFAULT: начало — сегодня по Алматы")

	// ---- down → 0017: due_date — обычная колонка со значением плана ----
	s.Require().NoError(mig.Migrate(17))
	s.Equal("0", s.scalar(`SELECT count(*)::text FROM information_schema.columns WHERE table_name = 'tasks' AND column_name IN ('start_date', 'duration_days')`))
	s.Equal("NEVER|YES", s.scalar(`SELECT is_generated || '|' || is_nullable FROM information_schema.columns WHERE table_name = 'tasks' AND column_name = 'due_date'`))
	due := func(id string) string {
		return s.scalar(`SELECT coalesce(due_date::text, '-') FROM tasks WHERE id = $1::bigint`, id)
	}
	s.Equal("2026-10-14", due(tA))
	s.Equal("2026-11-03", due(tC), "план после правки")
	s.Equal("2026-12-08", due(tD))
	s.exec(`UPDATE tasks SET due_date = NULL WHERE id = $1::bigint`, tB) // срок снова вводится вручную

	// ---- повторный up поверх отката ----
	s.Require().NoError(mig.Migrate(18))
	s.Equal("2026-10-10|5|2026-10-14", plan(tA))
	s.Equal(planOf(day(ago(2)), 3, today), plan(tB), "открытый без срока после отката → снова до сегодня")
	s.Equal("2026-10-15|2|2026-10-16", plan(tE))
	s.Equal(planOf(day(ago(3)), 1, day(ago(3))), plan(tH))
}
