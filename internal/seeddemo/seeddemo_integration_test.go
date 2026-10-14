//go:build integration

package seeddemo_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"

	"github.com/kitavrus/kanban-tree/internal/filestore"
	"github.com/kitavrus/kanban-tree/internal/seeddemo"
	"github.com/kitavrus/kanban-tree/pkg/testutil"
)

// SeedDemoSuite (§TD-8/BaseSuite) — AC-019: make seed-demo заливает демо-данные автосервиса
// одной командой в пустую БД; повторный вызов — no-op; без активного admin — ошибка.
// Колонки (5 дефолтных, Сид D32) сеет миграция 0005; SetupTest пересоздаёт их перед каждым
// тестом: TRUNCATE users … CASCADE очищает и columns (columns.deleted_by → users, миграция
// 0012 soft-delete-board-columns) и suppliers (suppliers.deleted_by → users, миграция 0015
// soft-delete-suppliers). Склад один — остаток живёт в products.stock.
type SeedDemoSuite struct {
	testutil.BaseSuite
	files *filestore.LocalFileStore
}

func TestSeedDemoSuite(t *testing.T) {
	suite.Run(t, new(SeedDemoSuite))
}

func (s *SeedDemoSuite) SetupTest() {
	ctx := context.Background()
	// CASCADE тянет transitively services (FK на catalog_categories),
	// tasks/subtasks/subtask_products/task_stock/stock_movements (FK на users/clients/
	// products/services) и columns (FK deleted_by → users) — колонки по умолчанию
	// восстанавливаются ниже тем же набором, что сеет миграция 0005 (Сид D32).
	_, err := s.Pool.Exec(ctx,
		`TRUNCATE TABLE users, clients, catalog_categories, products RESTART IDENTITY CASCADE`)
	s.Require().NoError(err)
	_, err = s.Pool.Exec(ctx, `
		INSERT INTO columns (name, color, is_final, consumes_stock, position) VALUES
		    ('Приёмка',         '#94A3B8', false, false, 1),
		    ('Диагностика',     '#0891B2', false, false, 2),
		    ('Согласование',    '#7C3AED', false, false, 3),
		    ('Ждёт запчасти',   '#D97706', false, true,  4),
		    ('В ремонте',       '#2563EB', false, true,  5),
		    ('Готов к выдаче',  '#16A34A', false, true,  6),
		    ('Выдан',           '#475569', true,  true,  7)`)
	s.Require().NoError(err)
	s.files, err = filestore.NewLocal(s.T().TempDir())
	s.Require().NoError(err)
}

func (s *SeedDemoSuite) createAdmin() string {
	ctx := context.Background()
	var id string
	s.Require().NoError(s.Pool.QueryRow(ctx,
		`INSERT INTO users (login, password_hash, name, role) VALUES ('admin', 'x', 'Администратор', 'admin') RETURNING id`).
		Scan(&id))
	return id
}

// TestSeedDemo_Run_EmptyDB_CreatesFullDemoSet — AC-019: пустая БД + активный admin →
// клиенты/оборудование/каталог/заказы во всех 7 колонках с услугами и товарами, инвариант
// склада выполняется.
func (s *SeedDemoSuite) TestSeedDemo_Run_EmptyDB_CreatesFullDemoSet() {
	ctx := context.Background()
	adminID := s.createAdmin()

	res, err := seeddemo.Run(ctx, s.Pool, seeddemo.WithFiles(s.files))
	s.Require().NoError(err)
	s.False(res.Skipped)
	s.NotEmpty(res.Message)
	s.Equal(4, res.Executors)
	s.Equal(10, res.Clients)
	s.Equal(18, res.Equipment)
	s.Equal(46, res.Products)
	s.Equal(29, res.Services)
	s.Equal(15, res.Tasks)

	var tasksTotal int
	s.Require().NoError(s.Pool.QueryRow(ctx, `SELECT count(*) FROM tasks WHERE deleted_at IS NULL`).Scan(&tasksTotal))
	s.Equal(15, tasksTotal)

	s.assertPlates()
	s.assertAttachments(res)

	// soft-delete-core TS-F5: ровно один мягко удалённый заказ с причиной и автором удаления —
	// раздел «Удалённые» на демо-стенде непустой; товара он не держит.
	var deletedTotal, deletedHoldingStock int
	var deleteReason, deletedBy string
	s.Require().NoError(s.Pool.QueryRow(ctx, `
		SELECT count(*), coalesce(min(delete_reason), ''), coalesce(min(deleted_by::text), ''),
		       count(*) FILTER (WHERE EXISTS (SELECT 1 FROM task_stock ts WHERE ts.task_id = tasks.id))
		FROM tasks WHERE deleted_at IS NOT NULL`).Scan(&deletedTotal, &deleteReason, &deletedBy, &deletedHoldingStock))
	s.Equal(1, deletedTotal)
	s.NotEmpty(deleteReason)
	s.Equal(adminID, deletedBy)
	s.Zero(deletedHoldingStock)

	// soft-delete-board-columns: ровно одна мягко удалённая колонка с причиной и автором
	// удаления — вкладка «Колонки» раздела «Удалённые» на демо-стенде непустая; заказов в ней
	// нет, действующие колонки идут подряд 1..N.
	var deletedCols, deletedColTasks, activeCols, activeMaxPos int
	var colReason, colBy string
	s.Require().NoError(s.Pool.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE deleted_at IS NOT NULL),
		       coalesce(min(delete_reason) FILTER (WHERE deleted_at IS NOT NULL), ''),
		       coalesce(min(deleted_by::text) FILTER (WHERE deleted_at IS NOT NULL), ''),
		       (SELECT count(*) FROM tasks t JOIN columns d ON d.id = t.column_id WHERE d.deleted_at IS NOT NULL),
		       count(*) FILTER (WHERE deleted_at IS NULL),
		       coalesce(max(position) FILTER (WHERE deleted_at IS NULL), 0)
		FROM columns`).Scan(&deletedCols, &colReason, &colBy, &deletedColTasks, &activeCols, &activeMaxPos))
	s.Equal(1, deletedCols)
	s.NotEmpty(colReason)
	s.Equal(adminID, colBy)
	s.Zero(deletedColTasks, "в удалённой колонке нет заказов")
	s.Equal(activeCols, activeMaxPos, "позиции действующих колонок — подряд 1..N")

	// soft-delete-clients: ровно один мягко удалённый клиент и две удалённые единицы — одна ушла
	// вместе с ним (те же «кто, когда, почему»), вторая удалена отдельно у действующего клиента;
	// действующих клиентов и единиц — столько же, сколько в Result (DS-041).
	var liveClients, deletedClients, liveUnits, withClient, separately, inherited int
	var clientBy string
	s.Require().NoError(s.Pool.QueryRow(ctx, `
		SELECT (SELECT count(*) FROM clients WHERE deleted_at IS NULL),
		       (SELECT count(*) FROM clients WHERE deleted_at IS NOT NULL),
		       (SELECT coalesce(min(deleted_by::text), '') FROM clients WHERE deleted_at IS NOT NULL),
		       (SELECT count(*) FROM equipment WHERE deleted_at IS NULL),
		       (SELECT count(*) FROM equipment WHERE deleted_with_client),
		       (SELECT count(*) FROM equipment e JOIN clients c ON c.id = e.client_id
		         WHERE e.deleted_at IS NOT NULL AND NOT e.deleted_with_client AND c.deleted_at IS NULL),
		       (SELECT count(*) FROM equipment e JOIN clients c ON c.id = e.client_id
		         WHERE e.deleted_with_client AND e.deleted_at = c.deleted_at
		           AND e.deleted_by = c.deleted_by AND e.delete_reason = c.delete_reason)`).
		Scan(&liveClients, &deletedClients, &clientBy, &liveUnits, &withClient, &separately, &inherited))
	s.Equal(res.Clients, liveClients)
	s.Equal(1, deletedClients)
	s.Equal(adminID, clientBy)
	s.Equal(res.Equipment, liveUnits)
	s.Equal(1, withClient, "одна единица ушла вместе с клиентом")
	s.Equal(1, separately, "одна единица действующего клиента удалена отдельно")
	s.Equal(1, inherited, "единица, ушедшая с клиентом, наследует его «кто, когда, почему»")

	s.assertDeletedCatalog(adminID, res)
	s.assertSuppliers(adminID)

	var distinctColumns int
	s.Require().NoError(s.Pool.QueryRow(ctx,
		`SELECT count(DISTINCT column_id) FROM tasks`).Scan(&distinctColumns))
	s.Equal(7, distinctColumns, "заказы должны быть во всех 7 колонках (D32)")

	var subtasksTotal int
	s.Require().NoError(s.Pool.QueryRow(ctx, `SELECT count(*) FROM subtasks`).Scan(&subtasksTotal))
	s.Positive(subtasksTotal, "услуги заказов должны быть созданы")

	var distinctStatuses int
	s.Require().NoError(s.Pool.QueryRow(ctx,
		`SELECT count(DISTINCT status) FROM subtasks`).Scan(&distinctStatuses))
	s.GreaterOrEqual(distinctStatuses, 2, "услуги должны быть в разных статусах")

	var subtaskProductsTotal int
	s.Require().NoError(s.Pool.QueryRow(ctx, `SELECT count(*) FROM subtask_products`).Scan(&subtaskProductsTotal))
	s.Positive(subtaskProductsTotal, "товары заказов должны быть заведены")

	var logsTotal, tasksWithEquipment int
	s.Require().NoError(s.Pool.QueryRow(ctx, `SELECT count(*) FROM subtask_logs`).Scan(&logsTotal))
	s.Positive(logsTotal, "журнал работ по услугам должен быть заполнен")
	s.Require().NoError(s.Pool.QueryRow(ctx, `SELECT count(DISTINCT task_id) FROM task_equipment`).Scan(&tasksWithEquipment))
	s.Equal(15, tasksWithEquipment, "у каждого заказа есть автомобиль клиента")

	var movementsTotal int
	s.Require().NoError(s.Pool.QueryRow(ctx, `SELECT count(*) FROM stock_movements`).Scan(&movementsTotal))
	s.Positive(movementsTotal, "должны быть проведённые списания/начальные остатки")

	// Существующий admin не изменяется (созданный этим тестом — единственный).
	var adminCount int
	s.Require().NoError(s.Pool.QueryRow(ctx, `SELECT count(*) FROM users WHERE role = 'admin'`).Scan(&adminCount))
	s.Equal(1, adminCount)
	var stillActive bool
	s.Require().NoError(s.Pool.QueryRow(ctx, `SELECT is_active FROM users WHERE id = $1`, adminID).Scan(&stillActive))
	s.True(stillActive)

	// TS-F5 (user-timezone, DS-033): админ — Asia/Almaty (умолчание, не тронут), ровно один
	// исполнитель — Europe/Moscow, остальные — умолчание.
	var adminTZ string
	s.Require().NoError(s.Pool.QueryRow(ctx, `SELECT timezone FROM users WHERE id = $1`, adminID).Scan(&adminTZ))
	s.Equal("Asia/Almaty", adminTZ)
	var moscow, almaty int
	s.Require().NoError(s.Pool.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE timezone = 'Europe/Moscow'), count(*) FILTER (WHERE timezone = 'Asia/Almaty')
		FROM users WHERE role = 'executor'`).Scan(&moscow, &almaty))
	s.Equal(1, moscow)
	s.Equal(3, almaty)

	assertStockInvariant(s.T(), s.Pool)
	s.assertExecutorPayouts()
}

// assertPlates — автомобиль = госномер (equipment.plate_number): у каждого действующего
// авто — российский номер «А123ВС 777» (кириллица), в том числе у машины снятого двигателя
// («Р126ОР 750»).
func (s *SeedDemoSuite) assertPlates() {
	ctx := context.Background()
	rows, err := s.Pool.Query(ctx, `SELECT plate_number FROM equipment WHERE deleted_at IS NULL`)
	s.Require().NoError(err)
	defer rows.Close()
	cars := 0
	plates := map[string]bool{}
	for rows.Next() {
		var plate string
		s.Require().NoError(rows.Scan(&plate))
		cars++
		plates[plate] = true
		s.Regexp(seeddemo.PlateRe, plate)
	}
	s.Require().NoError(rows.Err())
	s.Equal(18, cars)
	s.True(plates["Р126ОР 750"], "у автомобиля снятого двигателя тоже есть госномер")
}

// assertAttachments — у каждого заказа есть фото приёмки; есть видео; файл каждой строки
// task_attachments лежит в FileStore под storage_key, размер совпадает, тип по сигнатуре —
// тот же, что записан (как при загрузке через UI).
func (s *SeedDemoSuite) assertAttachments(res seeddemo.Result) {
	ctx := context.Background()
	var total, videos, tasksWithPhoto int
	s.Require().NoError(s.Pool.QueryRow(ctx, `
		SELECT count(*), count(*) FILTER (WHERE kind = 'video'),
		       (SELECT count(DISTINCT task_id) FROM task_attachments WHERE kind = 'photo')
		FROM task_attachments`).Scan(&total, &videos, &tasksWithPhoto))
	s.Equal(res.Attachments, total)
	s.Equal(res.Videos, videos)
	s.Equal(3, videos)
	s.Equal(res.Tasks, tasksWithPhoto, "у каждого заказа есть фото")

	rows, err := s.Pool.Query(ctx, `SELECT storage_key, size, content_type FROM task_attachments`)
	s.Require().NoError(err)
	defer rows.Close()
	for rows.Next() {
		var key, ct string
		var size int64
		s.Require().NoError(rows.Scan(&key, &size, &ct))
		f, info, err := s.files.Open(ctx, key)
		s.Require().NoError(err)
		head := make([]byte, 512)
		n, _ := io.ReadFull(f, head)
		_ = f.Close()
		s.Equal(size, info.Size)
		s.Equal(ct, http.DetectContentType(head[:n]))
	}
	s.Require().NoError(rows.Err())
	entries, err := os.ReadDir(s.files.Dir())
	s.Require().NoError(err)
	s.Len(entries, total, "в хранилище ровно файлы вложений")
}

// assertSuppliers — soft-delete-suppliers: в справочнике два демо-поставщика (действующий и
// архивный) и ровно один мягко удалённый («ТехИмпорт») с причиной и автором удаления.
func (s *SeedDemoSuite) assertSuppliers(adminID string) {
	ctx := context.Background()
	var active, archived, deleted int
	var reason, by string
	s.Require().NoError(s.Pool.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE deleted_at IS NULL AND is_active),
		       count(*) FILTER (WHERE deleted_at IS NULL AND NOT is_active),
		       count(*) FILTER (WHERE deleted_at IS NOT NULL),
		       coalesce(min(delete_reason) FILTER (WHERE deleted_at IS NOT NULL), ''),
		       coalesce(min(deleted_by::text) FILTER (WHERE deleted_at IS NOT NULL), '')
		FROM suppliers`).Scan(&active, &archived, &deleted, &reason, &by))
	s.Equal(1, active, "действующий поставщик")
	s.Equal(1, archived, "архивный поставщик")
	s.Equal(1, deleted, "suppliers: одна удалённая запись")
	s.NotEmpty(reason, "suppliers: причина удаления")
	s.Equal(adminID, by, "suppliers: кто удалил")
}

// assertDeletedCatalog — soft-delete-catalog: ровно по одной мягко удалённой записи каждого
// вида каталога с причиной и автором удаления — вкладки «Товары», «Услуги», «Категории»
// раздела «Удалённые» на демо-стенде непустые. Действующих товаров и услуг — столько же,
// сколько в Result; удалённый товар — из архива и без остатка (DS-021); на удалённую
// категорию никто не ссылается; категории — только услуг.
func (s *SeedDemoSuite) assertDeletedCatalog(adminID string, res seeddemo.Result) {
	ctx := context.Background()
	for _, table := range []string{"products", "services", "catalog_categories"} {
		var deleted int
		var reason, by string
		s.Require().NoError(s.Pool.QueryRow(ctx, `
			SELECT count(*), coalesce(min(delete_reason), ''), coalesce(min(deleted_by::text), '')
			FROM `+table+` WHERE deleted_at IS NOT NULL`).Scan(&deleted, &reason, &by))
		s.Equal(1, deleted, "%s: одна удалённая запись", table)
		s.NotEmpty(reason, "%s: причина удаления", table)
		s.Equal(adminID, by, "%s: кто удалил", table)
	}
	var liveProducts, liveServices, deletedProductStock, refsToDeletedCategory, productCategories int
	var deletedProductActive bool
	s.Require().NoError(s.Pool.QueryRow(ctx, `
		SELECT (SELECT count(*) FROM products WHERE deleted_at IS NULL),
		       (SELECT count(*) FROM services WHERE deleted_at IS NULL),
		       (SELECT is_active FROM products WHERE deleted_at IS NOT NULL),
		       (SELECT count(*) FROM products p WHERE p.deleted_at IS NOT NULL AND p.stock <> 0),
		       (SELECT count(*) FROM catalog_categories c WHERE c.deleted_at IS NOT NULL AND
		              EXISTS (SELECT 1 FROM services sv WHERE sv.category_id = c.id)),
		       (SELECT count(*) FROM catalog_categories WHERE kind <> 'service')`).
		Scan(&liveProducts, &liveServices, &deletedProductActive, &deletedProductStock, &refsToDeletedCategory, &productCategories))
	s.Equal(res.Products, liveProducts)
	s.Equal(res.Services, liveServices)
	s.False(deletedProductActive, "удалённый товар — из архива (бейдж «в архиве»)")
	s.Zero(deletedProductStock, "у удалённого товара нет остатка")
	s.Zero(refsToDeletedCategory, "на удалённую категорию никто не ссылается")
	s.Zero(productCategories, "категорий товаров нет — категории только у услуг")
}

// assertExecutorPayouts — TS-019 (executor-payouts, D71): часть выполненных услуг выплачена
// с суммой и датой, часть — нет; отметка стоит только у услуг с исполнителем, дата — ровно
// у отмеченных.
func (s *SeedDemoSuite) assertExecutorPayouts() {
	ctx := context.Background()
	var paidDone, unpaidDone, unpaidWithPayout, paidWithoutPayout, inconsistent int
	s.Require().NoError(s.Pool.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE status = 'done' AND is_executor_paid),
		       count(*) FILTER (WHERE status = 'done' AND assignee_id IS NOT NULL AND NOT is_executor_paid),
		       count(*) FILTER (WHERE NOT is_executor_paid AND executor_payout IS NOT NULL),
		       count(*) FILTER (WHERE is_executor_paid AND (executor_payout IS NULL OR executor_payout <= 0)),
		       count(*) FILTER (WHERE (is_executor_paid AND (assignee_id IS NULL OR executor_paid_at IS NULL))
		                           OR (NOT is_executor_paid AND executor_paid_at IS NOT NULL))
		FROM subtasks`).Scan(&paidDone, &unpaidDone, &unpaidWithPayout, &paidWithoutPayout, &inconsistent))
	s.Positive(paidDone, "часть выполненных услуг выплачена")
	s.Positive(unpaidDone, "часть выполненных услуг с исполнителем не выплачена")
	s.Positive(unpaidWithPayout, "есть невыплаченные услуги с заданной суммой выплаты")
	s.Zero(paidWithoutPayout, "выплаченные услуги — с суммой")
	s.Zero(inconsistent, "отметка только при исполнителе, дата — ровно у отмеченных")
}

// TestSeedDemo_Run_TasksExist_NoOpWithMessage — SD-003: в БД уже есть заказ → seeddemo
// печатает сообщение и не создаёт демо-данные (exit 0 в cmd, здесь — Skipped=true, nil error).
func (s *SeedDemoSuite) TestSeedDemo_Run_TasksExist_NoOpWithMessage() {
	ctx := context.Background()
	adminID := s.createAdmin()

	var clientID string
	s.Require().NoError(s.Pool.QueryRow(ctx,
		`INSERT INTO clients (company_name) VALUES ('Существующий клиент') RETURNING id`).Scan(&clientID))
	var columnID string
	s.Require().NoError(s.Pool.QueryRow(ctx, `SELECT id FROM columns ORDER BY position LIMIT 1`).Scan(&columnID))
	_, err := s.Pool.Exec(ctx,
		`INSERT INTO tasks (column_id, client_id, title, position, created_by) VALUES ($1, $2, 'Существующий заказ', 1024, $3)`,
		columnID, clientID, adminID)
	s.Require().NoError(err)

	res, err := seeddemo.Run(ctx, s.Pool)
	s.Require().NoError(err)
	s.True(res.Skipped)
	s.NotEmpty(res.Message)

	var clientsTotal int
	s.Require().NoError(s.Pool.QueryRow(ctx, `SELECT count(*) FROM clients`).Scan(&clientsTotal))
	s.Equal(1, clientsTotal, "no-op — новые демо-клиенты не создаются")

	var tasksTotal int
	s.Require().NoError(s.Pool.QueryRow(ctx, `SELECT count(*) FROM tasks`).Scan(&tasksTotal))
	s.Equal(1, tasksTotal, "no-op — новые демо-заказы не создаются")
}

// TestSeedDemo_Run_NoAdmin_ReturnsError — AC-019: пустая БД без активного admin → ошибка
// (cmd/seed-demo подсказывает make local-up-seed, exit 1), никаких данных не создаётся.
func (s *SeedDemoSuite) TestSeedDemo_Run_NoAdmin_ReturnsError() {
	ctx := context.Background()

	_, err := seeddemo.Run(ctx, s.Pool, seeddemo.WithFiles(s.files))
	s.Require().Error(err)
	s.True(errors.Is(err, seeddemo.ErrNoAdmin))

	var clientsTotal int
	s.Require().NoError(s.Pool.QueryRow(ctx, `SELECT count(*) FROM clients`).Scan(&clientsTotal))
	s.Zero(clientsTotal)
	entries, err := os.ReadDir(s.files.Dir())
	s.Require().NoError(err)
	s.Empty(entries, "без admin файлы вложений не пишутся")
}

// assertStockInvariant — единый склад (остаток в products.stock):
//  1. для каждого товара products.stock = Σ stock_movements.qty_delta (manual-старт +
//     task_consume/task_return);
//  2. по каждому заказу Σ движений = −Σ task_stock.qty (design-sql.md §Reconcile Query,
//     эталон board.assertStockInvariant);
//  3. task_stock действующего заказа = спрос его услуг (Σ subtask_products.qty по товару),
//     если колонка consumes_stock, иначе пусто;
//  4. в журнале только причины task_consume/task_return/manual. Отрицательный остаток
//     допустим — дефицит в демо намеренный (заказы в «Ждёт запчасти», seeddemo.go).
func assertStockInvariant(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	checks := []struct{ name, sql string }{
		{"products.stock ≠ Σ stock_movements", `
			SELECT count(*) FROM products p
			WHERE p.stock <> coalesce((SELECT sum(m.qty_delta) FROM stock_movements m WHERE m.product_id = p.id), 0)`},
		{"task_stock ≠ −Σ движений заказа", `
			SELECT count(*) FROM (
				SELECT t.id,
				       coalesce((SELECT sum(ts.qty) FROM task_stock ts WHERE ts.task_id = t.id), 0) AS stock_sum,
				       coalesce((SELECT sum(m.qty_delta) FROM stock_movements m WHERE m.task_id = t.id), 0) AS moved_sum
				FROM tasks t
			) x
			WHERE x.stock_sum <> -x.moved_sum`},
		{"task_stock ≠ спрос заказов в consumes_stock-колонках", `
			WITH demand AS (
			    SELECT s.task_id, sp.product_id, sum(sp.qty) AS qty
			    FROM subtask_products sp
			    JOIN subtasks s ON s.id = sp.subtask_id
			    JOIN tasks t ON t.id = s.task_id AND t.deleted_at IS NULL
			    JOIN columns c ON c.id = t.column_id AND c.consumes_stock
			    GROUP BY s.task_id, sp.product_id)
			SELECT count(*) FROM demand d
			FULL JOIN task_stock ts ON ts.task_id = d.task_id AND ts.product_id = d.product_id
			WHERE coalesce(d.qty, 0) <> coalesce(ts.qty, 0)`},
		{"посторонние причины движений", `
			SELECT count(*) FROM stock_movements WHERE reason NOT IN ('task_consume', 'task_return', 'manual')`},
	}
	for _, c := range checks {
		var violations int
		if err := pool.QueryRow(ctx, c.sql).Scan(&violations); err != nil {
			t.Fatalf("assertStockInvariant (%s): query failed: %v", c.name, err)
		}
		if violations != 0 {
			t.Fatalf("assertStockInvariant: %s — %d нарушений", c.name, violations)
		}
	}
	var manual, consumed int
	if err := pool.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE reason = 'manual'), count(*) FILTER (WHERE reason = 'task_consume')
		FROM stock_movements`).Scan(&manual, &consumed); err != nil {
		t.Fatalf("assertStockInvariant: movements query failed: %v", err)
	}
	if manual == 0 || consumed == 0 {
		t.Fatalf("assertStockInvariant: ожидались движения manual (%d) и task_consume (%d)", manual, consumed)
	}
}
