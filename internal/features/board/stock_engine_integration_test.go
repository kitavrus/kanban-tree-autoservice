//go:build integration

package board_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"

	"github.com/kitavrus/kanban-tree/internal/features/board"
	"github.com/kitavrus/kanban-tree/internal/features/softdelete"
	"github.com/kitavrus/kanban-tree/pkg/testutil"
)

// StockEngineSuite (§TD-8/BaseSuite): StockEngine.Reconcile — списание/возврат товаров,
// переключение consumes_stock, идемпотентность. Заказы/подзадачи/услуги — прямой SQL
// insert (TaskService появится в фазе 03).
type StockEngineSuite struct {
	testutil.BaseSuite
	engine *board.StockEngine
}

func TestStockEngineSuite(t *testing.T) {
	suite.Run(t, new(StockEngineSuite))
}

const (
	seAdminID  = "99999999-9999-9999-9999-999999999999"
	seClientID = "88888888-8888-8888-8888-888888888888"
	seColTodo  = "11111111-1111-1111-1111-111111111111" // не списывает
	seColWIP   = "22222222-2222-2222-2222-222222222222" // списывает
	seColDone  = "33333333-3333-3333-3333-333333333333" // финальная, списывает
)

func (s *StockEngineSuite) SetupSuite() {
	s.BaseSuite.SetupSuite()
	s.engine = board.NewStockEngine()

	ctx := context.Background()
	_, err := s.Pool.Exec(ctx, `INSERT INTO users (id, login, password_hash, name, role) VALUES
		($1, 'stock-engine-admin', 'x', 'Admin', 'admin') ON CONFLICT (id) DO NOTHING`, seAdminID)
	s.Require().NoError(err)
	_, err = s.Pool.Exec(ctx, `INSERT INTO clients (id, company_name) VALUES ($1, 'Test Co')
		ON CONFLICT (id) DO NOTHING`, seClientID)
	s.Require().NoError(err)
}

func (s *StockEngineSuite) SetupTest() {
	ctx := context.Background()
	// CASCADE тянет за собой tasks/subtasks/subtask_products/task_stock/task_equipment/
	// stock_movements — колонки/каталог/услуги для каждого теста создаются заново.
	_, err := s.Pool.Exec(ctx, `TRUNCATE TABLE catalog_categories, products, services, columns RESTART IDENTITY CASCADE`)
	s.Require().NoError(err)
	_, err = s.Pool.Exec(ctx, `INSERT INTO columns (id, name, color, is_final, consumes_stock, position) VALUES
		($1, 'Приёмка', '#94A3B8', false, false, 1),
		($2, 'В ремонте', '#2563EB', false, true, 2),
		($3, 'Выдан', '#059669', true, true, 3)`, seColTodo, seColWIP, seColDone)
	s.Require().NoError(err)
}

// createProduct — товар с заданным остатком/минимумом в свежей категории (уникальное имя —
// unique index (kind, lower(btrim(name))) допускает архивные дубли, но не активные).
func (s *StockEngineSuite) createProduct(stock, minStock string) string {
	ctx := context.Background()
	var categoryID string
	s.Require().NoError(s.Pool.QueryRow(ctx,
		`INSERT INTO catalog_categories (kind, name) VALUES ('product', 'Products ' || gen_random_uuid()) RETURNING id`,
	).Scan(&categoryID))
	var productID string
	s.Require().NoError(s.Pool.QueryRow(ctx,
		`INSERT INTO products (category_id, name, stock, min_stock) VALUES ($1, 'Product', $2::numeric, $3::numeric) RETURNING id`,
		categoryID, stock, minStock,
	).Scan(&productID))
	return productID
}

// createTaskWithDemand — заказ в columnID с одной подзадачей-услугой, несущей комплект
// товаров productQty (product_id → qty); подзадача НЕ done (Reconcile от статуса не зависит
// в этой фазе — только от column.consumes_stock, D4/AC-004).
func (s *StockEngineSuite) createTaskWithDemand(columnID string, productQty map[string]string) int64 {
	ctx := context.Background()
	var categoryID string
	s.Require().NoError(s.Pool.QueryRow(ctx,
		`INSERT INTO catalog_categories (kind, name) VALUES ('service', 'Services ' || gen_random_uuid()) RETURNING id`,
	).Scan(&categoryID))
	var serviceID string
	s.Require().NoError(s.Pool.QueryRow(ctx,
		`INSERT INTO services (category_id, name) VALUES ($1, 'Service') RETURNING id`, categoryID,
	).Scan(&serviceID))

	var taskID int64
	s.Require().NoError(s.Pool.QueryRow(ctx,
		`INSERT INTO tasks (column_id, client_id, title, position, created_by)
		 VALUES ($1, $2, 'Task', 1024, $3) RETURNING id`, columnID, seClientID, seAdminID,
	).Scan(&taskID))

	var subtaskID string
	s.Require().NoError(s.Pool.QueryRow(ctx,
		`INSERT INTO subtasks (task_id, service_id) VALUES ($1, $2) RETURNING id`, taskID, serviceID,
	).Scan(&subtaskID))

	for productID, qty := range productQty {
		_, err := s.Pool.Exec(ctx,
			`INSERT INTO subtask_products (subtask_id, product_id, qty) VALUES ($1, $2, $3::numeric)`,
			subtaskID, productID, qty)
		s.Require().NoError(err)
	}
	return taskID
}

func (s *StockEngineSuite) productStock(productID string) string {
	var stock string
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`SELECT stock::text FROM products WHERE id = $1`, productID).Scan(&stock))
	return stock
}

func (s *StockEngineSuite) taskStockQty(taskID int64, productID string) (string, bool) {
	var qty string
	err := s.Pool.QueryRow(context.Background(),
		`SELECT qty::text FROM task_stock WHERE task_id = $1 AND product_id = $2`, taskID, productID).Scan(&qty)
	if err != nil {
		return "", false
	}
	return qty, true
}

func (s *StockEngineSuite) movementsCount(taskID int64) int {
	var n int
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`SELECT count(*) FROM stock_movements WHERE task_id = $1`, taskID).Scan(&n))
	return n
}

func (s *StockEngineSuite) reconcile(taskID int64, targetConsumes bool) board.ReconcileResult {
	ctx := context.Background()
	tx, err := s.Pool.Begin(ctx)
	s.Require().NoError(err)
	defer tx.Rollback(ctx) //nolint:errcheck

	res, err := s.engine.Reconcile(ctx, tx, taskID, targetConsumes, seAdminID)
	s.Require().NoError(err)
	s.Require().NoError(tx.Commit(ctx))
	return res
}

// TestStockEngine_Reconcile_ConsumeThenReturn — AC-004/AC-005: заказ переезжает в
// consumes-колонку → товар списывается и резервируется в task_stock; переезд обратно →
// полный возврат, task_stock-строка удаляется (design-sql.md §Reconcile Query: «target=0 →
// строка удаляется»).
func (s *StockEngineSuite) TestStockEngine_Reconcile_ConsumeThenReturn() {
	productID := s.createProduct("10", "2")
	taskID := s.createTaskWithDemand(seColTodo, map[string]string{productID: "3"})

	res := s.reconcile(taskID, true)
	s.Equal(1, res.Movements)
	s.Equal("7.000", s.productStock(productID))
	qty, ok := s.taskStockQty(taskID, productID)
	s.True(ok)
	s.Equal("3.000", qty)
	assertStockInvariant(s.T(), s.Pool)

	res = s.reconcile(taskID, false)
	s.Equal(1, res.Movements)
	s.Equal("10.000", s.productStock(productID))
	_, ok = s.taskStockQty(taskID, productID)
	s.False(ok, "task_stock qty=0 — строка должна быть удалена")
	s.Equal(2, s.movementsCount(taskID))
	assertStockInvariant(s.T(), s.Pool)
}

// TestStockEngine_Reconcile_ToggleColumnConsumes — AC-007: несколько переключений
// consumes_stock подряд (эталон ColumnService.Update-цикла по TaskIDsForUpdate) —
// каждый раз склад приходит к согласованному состоянию.
func (s *StockEngineSuite) TestStockEngine_Reconcile_ToggleColumnConsumes() {
	productID := s.createProduct("5", "10") // min_stock=10 > stock — заведомо below_min/negative
	taskID := s.createTaskWithDemand(seColWIP, map[string]string{productID: "2"})

	res := s.reconcile(taskID, true)
	s.Equal("3.000", s.productStock(productID))
	s.Require().Len(res.Warnings, 1)
	s.Equal(board.WarningKindBelowMin, res.Warnings[0].Kind)
	assertStockInvariant(s.T(), s.Pool)

	res = s.reconcile(taskID, false)
	s.Equal("5.000", s.productStock(productID))
	// min_stock=10 задан заведомо выше stock — товар остаётся в состоянии below_min даже
	// после возврата (Reconcile предупреждает по товарам, ЗАТРОНУТЫМ этим вызовом, вне
	// зависимости от направления движения).
	s.Require().Len(res.Warnings, 1)
	s.Equal(board.WarningKindBelowMin, res.Warnings[0].Kind)
	assertStockInvariant(s.T(), s.Pool)

	res = s.reconcile(taskID, true)
	s.Equal("3.000", s.productStock(productID))
	qty, ok := s.taskStockQty(taskID, productID)
	s.True(ok)
	s.Equal("2.000", qty)
	assertStockInvariant(s.T(), s.Pool)
}

// TestStockEngine_Reconcile_Idempotent_NoDoubleConsume — повторный Reconcile с тем же
// targetConsumes и неизменным комплектом — 0 дельт, склад/журнал не трогаются (защита от
// двойного списания, AC-004).
func (s *StockEngineSuite) TestStockEngine_Reconcile_Idempotent_NoDoubleConsume() {
	productID := s.createProduct("10", "2")
	taskID := s.createTaskWithDemand(seColWIP, map[string]string{productID: "4"})

	first := s.reconcile(taskID, true)
	s.Equal(1, first.Movements)
	s.Equal("6.000", s.productStock(productID))

	second := s.reconcile(taskID, true)
	s.Equal(0, second.Movements)
	s.Empty(second.Warnings)
	s.Equal("6.000", s.productStock(productID), "повторный Reconcile не должен списывать повторно")
	s.Equal(1, s.movementsCount(taskID), "0 дельт — новых строк в журнале нет")
	assertStockInvariant(s.T(), s.Pool)
}

// ---- IT-010 (фаза 03): полная последовательность через реальные TaskRepo/TaskService
// (не прямой SQL, как в тестах фазы 02 выше) — consume → return → повторный consume →
// toggle колонки → delete заказа. Manual adjust — заготовка без реализации (появится в
// фазе 04, StockRepo.Adjust); проверка инварианта после каждого шага. ----

func (s *StockEngineSuite) TestIT010_FullSequence_ViaTaskServiceAndColumnService() {
	ctx := context.Background()
	taskRepo := board.NewTaskRepo(s.Pool)
	taskSvc := board.NewTaskService(s.Pool, taskRepo, s.engine, board.NewTaskEquipmentRepo())
	colRepo := board.NewColumnRepo(s.Pool)
	colSvc := board.NewColumnService(s.Pool, colRepo, s.engine)
	actor := board.Actor{ID: seAdminID, IsAdmin: true}

	productID := s.createProduct("20", "2")

	// подзадача-услуга с потребностью в товаре — напрямую SQL (subtask_products не входит
	// в DTO заказа фазы 03, появится в фазе 6), заказ и его перенос — через сервис.
	create, err := taskSvc.Create(ctx, actor, board.TaskCreateRequest{TaskHeader: board.TaskHeader{
		Title: "IT-010 заказ", ClientID: seClientID, ColumnID: seColTodo,
	}})
	s.Require().NoError(err)
	taskID := create.Task.ID

	var categoryID, serviceID, subtaskID string
	s.Require().NoError(s.Pool.QueryRow(ctx,
		`INSERT INTO catalog_categories (kind, name) VALUES ('service', 'IT010 ' || gen_random_uuid()) RETURNING id`).Scan(&categoryID))
	s.Require().NoError(s.Pool.QueryRow(ctx,
		`INSERT INTO services (category_id, name) VALUES ($1, 'IT010 Service') RETURNING id`, categoryID).Scan(&serviceID))
	s.Require().NoError(s.Pool.QueryRow(ctx,
		`INSERT INTO subtasks (task_id, service_id) VALUES ($1, $2) RETURNING id`, taskID, serviceID).Scan(&subtaskID))
	_, err = s.Pool.Exec(ctx,
		`INSERT INTO subtask_products (subtask_id, product_id, qty) VALUES ($1, $2, '4')`, subtaskID, productID)
	s.Require().NoError(err)
	assertStockInvariant(s.T(), s.Pool)

	// шаг 1: consume — move в consumes-колонку.
	_, err = taskSvc.Move(ctx, actor, s.itoa(taskID), board.TaskMoveRequest{ColumnID: seColWIP})
	s.Require().NoError(err)
	s.Equal("16.000", s.productStock(productID))
	assertStockInvariant(s.T(), s.Pool)

	// шаг 2: return — move обратно в не-consumes-колонку.
	_, err = taskSvc.Move(ctx, actor, s.itoa(taskID), board.TaskMoveRequest{ColumnID: seColTodo})
	s.Require().NoError(err)
	s.Equal("20.000", s.productStock(productID))
	assertStockInvariant(s.T(), s.Pool)

	// шаг 3: повторный consume.
	_, err = taskSvc.Move(ctx, actor, s.itoa(taskID), board.TaskMoveRequest{ColumnID: seColWIP})
	s.Require().NoError(err)
	s.Equal("16.000", s.productStock(productID))
	assertStockInvariant(s.T(), s.Pool)

	// шаг 4: toggle колонки — выключить consumes_stock у seColWIP пересчитывает все её
	// заказы (AC-007), включая наш — возврат товара без переноса заказа.
	_, err = colSvc.Update(ctx, seColWIP, seAdminID, board.ColumnInput{
		Name: "В ремонте", Color: "#2563EB", IsFinal: false, ConsumesStock: false,
	})
	s.Require().NoError(err)
	s.Equal("20.000", s.productStock(productID))
	assertStockInvariant(s.T(), s.Pool)

	// вернуть consumes_stock=true — заказ снова списывается для следующего шага (delete).
	_, err = colSvc.Update(ctx, seColWIP, seAdminID, board.ColumnInput{
		Name: "В ремонте", Color: "#2563EB", IsFinal: false, ConsumesStock: true,
	})
	s.Require().NoError(err)
	s.Equal("16.000", s.productStock(productID))
	assertStockInvariant(s.T(), s.Pool)

	// шаг 5: мягкое удаление заказа (soft-delete-core DS-004) — Reconcile(false) возвращает
	// весь склад перед отметкой удаления; строка заказа остаётся.
	reason := "Создан по ошибке"
	_, err = taskSvc.Delete(ctx, actor, s.itoa(taskID), softdelete.DeleteRequest{Reason: &reason})
	s.Require().NoError(err)
	s.Equal("20.000", s.productStock(productID))
	_, ok := s.taskStockQty(taskID, productID)
	s.False(ok)
	assertStockInvariant(s.T(), s.Pool)

	// шаг 5а (TS-052, DS-004): правка колонки, пока заказ удалён, его не пересчитывает —
	// выключение и включение consumes_stock товар удалённого заказа не списывает.
	for _, consumes := range []bool{false, true} {
		_, err = colSvc.Update(ctx, seColWIP, seAdminID, board.ColumnInput{
			Name: "В ремонте", Color: "#2563EB", IsFinal: false, ConsumesStock: consumes,
		})
		s.Require().NoError(err)
		s.Equal("20.000", s.productStock(productID), "удалённый заказ не списывает товар при правке колонки")
		_, ok = s.taskStockQty(taskID, productID)
		s.False(ok)
	}

	// шаг 5б (DS-005): восстановление — товар списывается снова по правилу колонки заказа.
	_, err = taskSvc.Restore(ctx, actor, s.itoa(taskID))
	s.Require().NoError(err)
	s.Equal("16.000", s.productStock(productID))
	assertStockInvariant(s.T(), s.Pool)

	// шаг 6 (manual adjust) — заготовка: StockRepo.Adjust появляется в фазе 04
	// (design-go-layers.md §Package Layout, stock_repo.go); здесь не покрывается.
}

func (s *StockEngineSuite) itoa(id int64) string { return fmt.Sprintf("%d", id) }

// assertStockInvariant — design-tests.md §Stock Invariant / design-sql.md §Reconcile Query:
// «Σ stock_movements.qty_delta по заказу = −Σ task_stock.qty». Проверка выполняется в SQL —
// без разбора десятичных строк в Go.
func assertStockInvariant(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	var violations int
	err := pool.QueryRow(ctx, `
		SELECT count(*) FROM (
			SELECT ts.task_id,
			       sum(ts.qty) AS stock_sum,
			       coalesce((SELECT sum(m.qty_delta) FROM stock_movements m WHERE m.task_id = ts.task_id), 0) AS moved_sum
			FROM task_stock ts
			GROUP BY ts.task_id
		) x
		WHERE x.stock_sum <> -x.moved_sum`).Scan(&violations)
	if err != nil {
		t.Fatalf("assertStockInvariant: query failed: %v", err)
	}
	if violations != 0 {
		t.Fatalf("assertStockInvariant: %d заказов нарушают инвариант task_stock/stock_movements", violations)
	}
}
