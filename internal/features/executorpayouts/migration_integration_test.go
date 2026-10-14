//go:build integration

package executorpayouts_test

import (
	"context"
	"errors"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/stretchr/testify/suite"

	"github.com/kitavrus/kanban-tree/pkg/testutil"
)

// ExecutorPayoutsMigrationSuite — TS-018: миграция 0011 (docs/plan.md D63). На 0010
// заводится услуга заказа; up → у неё is_executor_paid=false, дата и сумма NULL, CHECK
// суммы ≥ 0 и частичный индекс на месте; down → колонки и индекс сняты, услуга цела.
type ExecutorPayoutsMigrationSuite struct {
	testutil.BaseSuite
}

func TestExecutorPayoutsMigrationSuite(t *testing.T) {
	suite.Run(t, new(ExecutorPayoutsMigrationSuite))
}

const (
	epmAdminID  = "aaaaaaaa-0011-4011-8011-000000000001"
	epmClientID = "bbbbbbbb-0011-4011-8011-000000000001"
)

var epmColumns = []string{"is_executor_paid", "executor_paid_at", "executor_payout"}

func migrationsURL() string {
	_, f, _, _ := runtime.Caller(0)
	return "file://" + filepath.Join(filepath.Dir(f), "..", "..", "..", "migrations")
}

func (s *ExecutorPayoutsMigrationSuite) columnExists(column string) bool {
	var ok bool
	s.Require().NoError(s.Pool.QueryRow(context.Background(), `
		SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'subtasks' AND column_name = $1)`,
		column).Scan(&ok))
	return ok
}

func (s *ExecutorPayoutsMigrationSuite) indexExists(name string) bool {
	var ok bool
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`SELECT EXISTS (SELECT 1 FROM pg_indexes WHERE tablename = 'subtasks' AND indexname = $1)`, name).Scan(&ok))
	return ok
}

func (s *ExecutorPayoutsMigrationSuite) TestTS018_UpAddsPayoutColumns_DownDropsThem() {
	ctx := context.Background()
	mig, err := migrate.New(migrationsURL(), s.DSN)
	s.Require().NoError(err)
	defer func() { _, _ = mig.Close() }()

	s.Require().NoError(mig.Migrate(10))
	defer func() {
		// вернуть схему на последнюю версию при любом исходе (на ней уже — ErrNoChange).
		if err := mig.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			s.Require().NoError(err)
		}
	}()
	for _, c := range epmColumns {
		s.False(s.columnExists(c), "на 0010 колонки subtasks.%s ещё нет", c)
	}

	// --- данные «до выплат» (схема 0010) ---
	_, err = s.Pool.Exec(ctx, `INSERT INTO users (id, login, password_hash, name, role) VALUES ($1, 'epm-admin', 'x', 'Admin', 'admin')`, epmAdminID)
	s.Require().NoError(err)
	_, err = s.Pool.Exec(ctx, `INSERT INTO clients (id, company_name) VALUES ($1, 'EPM Co')`, epmClientID)
	s.Require().NoError(err)
	var columnID, svcCat, serviceID, subtaskID string
	var taskID int64
	s.Require().NoError(s.Pool.QueryRow(ctx, `SELECT id::text FROM columns ORDER BY position, id LIMIT 1`).Scan(&columnID))
	s.Require().NoError(s.Pool.QueryRow(ctx, `INSERT INTO catalog_categories (kind, name) VALUES ('service', 'EPM S') RETURNING id`).Scan(&svcCat))
	s.Require().NoError(s.Pool.QueryRow(ctx, `INSERT INTO services (category_id, name) VALUES ($1, 'EPM услуга') RETURNING id`, svcCat).Scan(&serviceID))
	s.Require().NoError(s.Pool.QueryRow(ctx,
		`INSERT INTO tasks (column_id, client_id, title, position, created_by) VALUES ($1, $2, 'EPM', 1024, $3) RETURNING id`,
		columnID, epmClientID, epmAdminID).Scan(&taskID))
	s.Require().NoError(s.Pool.QueryRow(ctx,
		`INSERT INTO subtasks (task_id, service_id, status, assignee_id, price) VALUES ($1, $2, 'done', $3, 1500) RETURNING id`,
		taskID, serviceID, epmAdminID).Scan(&subtaskID))

	// --- up 0011 ---
	s.Require().NoError(mig.Migrate(11))
	for _, c := range epmColumns {
		s.True(s.columnExists(c), "subtasks.%s добавлена", c)
	}
	s.True(s.indexExists("subtasks_executor_unpaid_idx"))

	var paid bool
	var paidAt, payout *string
	var price string
	s.Require().NoError(s.Pool.QueryRow(ctx,
		`SELECT is_executor_paid, executor_paid_at::text, executor_payout::text, price::text FROM subtasks WHERE id = $1`, subtaskID).
		Scan(&paid, &paidAt, &payout, &price))
	s.False(paid, "существующая услуга — не выплачена (DEFAULT false)")
	s.Nil(paidAt)
	s.Nil(payout)
	s.Equal("1500.00", price, "цена для клиента не тронута")

	// NOT NULL флага; сумма — NUMERIC(12,2) ≥ 0, NULL допустим.
	_, err = s.Pool.Exec(ctx, `UPDATE subtasks SET is_executor_paid = NULL WHERE id = $1`, subtaskID)
	s.Error(err, "is_executor_paid NOT NULL")
	_, err = s.Pool.Exec(ctx, `UPDATE subtasks SET executor_payout = -0.01 WHERE id = $1`, subtaskID)
	s.Error(err, "CHECK executor_payout >= 0")
	_, err = s.Pool.Exec(ctx, `UPDATE subtasks SET executor_payout = 10000000000 WHERE id = $1`, subtaskID)
	s.Error(err, "NUMERIC(12,2): 10 цифр целой части")
	_, err = s.Pool.Exec(ctx,
		`UPDATE subtasks SET is_executor_paid = true, executor_paid_at = now(), executor_payout = 1234.567 WHERE id = $1`, subtaskID)
	s.Require().NoError(err)
	s.Require().NoError(s.Pool.QueryRow(ctx, `SELECT executor_payout::text FROM subtasks WHERE id = $1`, subtaskID).Scan(&payout))
	s.Equal("1234.57", *payout, "масштаб 2 знака")
	_, err = s.Pool.Exec(ctx, `UPDATE subtasks SET executor_payout = 0 WHERE id = $1`, subtaskID)
	s.Require().NoError(err, "0 — допустимая сумма")

	// удаление сотрудника не упирается в выплаченную услугу (assignee_id ON DELETE SET NULL, D63).
	var tmpUser string
	s.Require().NoError(s.Pool.QueryRow(ctx,
		`INSERT INTO users (login, password_hash, name, role) VALUES ('epm-exec', 'x', 'Exec', 'executor') RETURNING id`).Scan(&tmpUser))
	_, err = s.Pool.Exec(ctx, `UPDATE subtasks SET assignee_id = $2 WHERE id = $1`, subtaskID, tmpUser)
	s.Require().NoError(err)
	_, err = s.Pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, tmpUser)
	s.Require().NoError(err)
	var assignee *string
	s.Require().NoError(s.Pool.QueryRow(ctx, `SELECT assignee_id::text, is_executor_paid FROM subtasks WHERE id = $1`, subtaskID).Scan(&assignee, &paid))
	s.Nil(assignee)
	s.True(paid)

	// --- down → 0010 ---
	s.Require().NoError(mig.Migrate(10))
	for _, c := range epmColumns {
		s.False(s.columnExists(c), "subtasks.%s снята", c)
	}
	s.False(s.indexExists("subtasks_executor_unpaid_idx"))
	var status string
	s.Require().NoError(s.Pool.QueryRow(ctx, `SELECT status, price::text FROM subtasks WHERE id = $1`, subtaskID).Scan(&status, &price))
	s.Equal("done", status, "услуга пережила откат")
	s.Equal("1500.00", price)

	// --- повторный up: миграция накатывается заново поверх данных ---
	s.Require().NoError(mig.Migrate(11))
	s.Require().NoError(s.Pool.QueryRow(ctx, `SELECT is_executor_paid FROM subtasks WHERE id = $1`, subtaskID).Scan(&paid))
	s.False(paid, "после down/up отметки сброшены")
}
