//go:build integration

package board_test

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/stretchr/testify/suite"

	"github.com/kitavrus/kanban-tree/pkg/testutil"
)

// SubtaskProductsPositionSuite — IT-020 (design-sql.md §Subtask Logs Migration, ADR-103):
// down до 0005 (столбца position ещё нет) → 3 строки subtask_products одной услуги без
// position → up (0006) → backfill row_number() по (subtask_id ORDER BY product_id, id) —
// position 1..3, без нулей/дублей.
type SubtaskProductsPositionSuite struct {
	testutil.BaseSuite
}

func TestSubtaskProductsPositionSuite(t *testing.T) {
	suite.Run(t, new(SubtaskProductsPositionSuite))
}

const (
	sppAdminID  = "aaaaaaaa-1111-1111-1111-111111111111"
	sppClientID = "bbbbbbbb-1111-1111-1111-111111111111"
	sppColumnID = "cccccccc-1111-1111-1111-111111111111"
)

func (s *SubtaskProductsPositionSuite) SetupSuite() {
	s.BaseSuite.SetupSuite()
	ctx := context.Background()
	_, err := s.Pool.Exec(ctx, `INSERT INTO users (id, login, password_hash, name, role) VALUES
		($1, 'spp-admin', 'x', 'Admin', 'admin') ON CONFLICT (id) DO NOTHING`, sppAdminID)
	s.Require().NoError(err)
	_, err = s.Pool.Exec(ctx, `INSERT INTO clients (id, company_name) VALUES ($1, 'SPP Co')
		ON CONFLICT (id) DO NOTHING`, sppClientID)
	s.Require().NoError(err)
	_, err = s.Pool.Exec(ctx, `INSERT INTO columns (id, name, color, is_final, consumes_stock, position) VALUES
		($1, 'Новые', '#94A3B8', false, false, 1) ON CONFLICT (id) DO NOTHING`, sppColumnID)
	s.Require().NoError(err)
}

// migrateRoot — каталог migrations/ (репозиторный корень, три уровня вверх от board/).
func migrateRoot() string {
	_, f, _, _ := runtime.Caller(0)
	return "file://" + filepath.Join(filepath.Dir(f), "..", "..", "..", "migrations")
}

func (s *SubtaskProductsPositionSuite) TestSubtaskProductsPosition_BackfillOnMigrate() {
	ctx := context.Background()

	mig, err := migrate.New(migrateRoot(), s.DSN)
	s.Require().NoError(err)
	defer func() { _, _ = mig.Close() }()

	// down до 0005 — 0006 (и всё новее, напр. 0007 вложений) откатывается: subtask_logs
	// дропается, subtask_products.position исчезает (IT-020, CM-1 MVP down). Явная версия, а
	// не Steps(-1): тест не должен зависеть от того, какая миграция последняя.
	s.Require().NoError(mig.Migrate(5))
	defer func() { s.Require().NoError(mig.Up()) }() // вернуть схему к последней версии

	var categoryID string
	s.Require().NoError(s.Pool.QueryRow(ctx,
		`INSERT INTO catalog_categories (kind, name) VALUES ('service', 'SPP Services ' || gen_random_uuid()) RETURNING id`,
	).Scan(&categoryID))
	var serviceID string
	s.Require().NoError(s.Pool.QueryRow(ctx,
		`INSERT INTO services (category_id, name) VALUES ($1, 'SPP Service') RETURNING id`, categoryID,
	).Scan(&serviceID))
	var prodCategoryID string
	s.Require().NoError(s.Pool.QueryRow(ctx,
		`INSERT INTO catalog_categories (kind, name) VALUES ('product', 'SPP Products ' || gen_random_uuid()) RETURNING id`,
	).Scan(&prodCategoryID))

	var taskID int64
	s.Require().NoError(s.Pool.QueryRow(ctx,
		`INSERT INTO tasks (column_id, client_id, title, position, created_by)
		 VALUES ($1, $2, 'SPP Task', 1024, $3) RETURNING id`, sppColumnID, sppClientID, sppAdminID,
	).Scan(&taskID))
	var subtaskID string
	s.Require().NoError(s.Pool.QueryRow(ctx,
		`INSERT INTO subtasks (task_id, service_id) VALUES ($1, $2) RETURNING id`, taskID, serviceID,
	).Scan(&subtaskID))

	// 3 товара одной услуги, вставлены до появления колонки position (её ещё нет на 0005).
	productIDs := make([]string, 3)
	for i := range productIDs {
		var productID string
		s.Require().NoError(s.Pool.QueryRow(ctx,
			`INSERT INTO products (category_id, name) VALUES ($1, $2) RETURNING id`,
			prodCategoryID, fmt.Sprintf("SPP Product %d", i),
		).Scan(&productID))
		productIDs[i] = productID
		_, err := s.Pool.Exec(ctx,
			`INSERT INTO subtask_products (subtask_id, product_id, qty) VALUES ($1, $2, 1)`,
			subtaskID, productID)
		s.Require().NoError(err)
	}

	// up — снова 0006: ADD COLUMN position + backfill row_number().
	s.Require().NoError(mig.Migrate(6))

	rows, err := s.Pool.Query(ctx,
		`SELECT position FROM subtask_products WHERE subtask_id = $1 ORDER BY position`, subtaskID)
	s.Require().NoError(err)
	defer rows.Close()

	var positions []int
	for rows.Next() {
		var p int
		s.Require().NoError(rows.Scan(&p))
		positions = append(positions, p)
	}
	s.Require().NoError(rows.Err())

	s.Equal([]int{1, 2, 3}, positions, "backfill row_number() — 1..N без нулей/дублей (ADR-103)")
}
