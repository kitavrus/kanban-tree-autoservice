//go:build integration

package seeddemo_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"

	"github.com/kitavrus/kanban-tree/internal/seeddemo"
	"github.com/kitavrus/kanban-tree/pkg/testutil"
)

// SeedDemoSuite (§TD-8/BaseSuite) — AC-019: make seed-demo заливает демо-данные лазерного сервиса
// одной командой в пустую БД; повторный вызов — no-op; без активного admin — ошибка.
// Колонки (5 дефолтных, Сид D32) сеет миграция 0005; SetupTest пересоздаёт их перед каждым
// тестом: TRUNCATE users … CASCADE очищает и columns (columns.deleted_by → users, миграция
// 0020 soft-delete-board-columns) и warehouses (warehouses.deleted_by → users, миграция 0023
// soft-delete-warehouses-suppliers) — склады «Алматы» и «Москва» (сид миграции 0008)
// пересоздаются так же.
type SeedDemoSuite struct {
	testutil.BaseSuite
}

func TestSeedDemoSuite(t *testing.T) {
	suite.Run(t, new(SeedDemoSuite))
}

func (s *SeedDemoSuite) SetupTest() {
	ctx := context.Background()
	// CASCADE тянет transitively products/services (FK на catalog_categories),
	// tasks/subtasks/subtask_products/task_stock/stock_movements (FK на users/clients/
	// products/services) и columns (FK deleted_by → users) — колонки по умолчанию
	// восстанавливаются ниже тем же набором, что сеет миграция 0005 (Сид D32).
	_, err := s.Pool.Exec(ctx,
		`TRUNCATE TABLE users, clients, equipment_types, catalog_categories RESTART IDENTITY CASCADE`)
	s.Require().NoError(err)
	_, err = s.Pool.Exec(ctx, `
		INSERT INTO columns (name, color, is_final, consumes_stock, position) VALUES
		    ('Новые',          '#94A3B8', false, false, 1),
		    ('Согласование',   '#7C3AED', false, false, 2),
		    ('В работе',       '#2563EB', false, true,  3),
		    ('Ждёт запчасти',  '#D97706', false, true,  4),
		    ('Готово',         '#059669', true,  true,  5)`)
	s.Require().NoError(err)
	_, err = s.Pool.Exec(ctx, `INSERT INTO warehouses (name, position) VALUES ('Алматы', 1), ('Москва', 2)`)
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
// клиенты/оборудование/каталог/заказы во всех 5 колонках с услугами и товарами, инвариант
// склада выполняется.
func (s *SeedDemoSuite) TestSeedDemo_Run_EmptyDB_CreatesFullDemoSet() {
	ctx := context.Background()
	adminID := s.createAdmin()

	res, err := seeddemo.Run(ctx, s.Pool)
	s.Require().NoError(err)
	s.False(res.Skipped)
	s.NotEmpty(res.Message)
	s.Equal(4, res.Executors)
	s.Equal(10, res.Clients)
	s.Equal(28, res.Equipment)
	s.Equal(22, res.Products)
	s.Equal(17, res.Services)
	s.Equal(12, res.Tasks)

	var tasksTotal int
	s.Require().NoError(s.Pool.QueryRow(ctx, `SELECT count(*) FROM tasks WHERE deleted_at IS NULL`).Scan(&tasksTotal))
	s.Equal(12, tasksTotal)

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
	// действующих клиентов и единиц — столько же, сколько в Result. Удалённые единицы у клиента
	// и не стоят в документах (DS-041).
	var liveClients, deletedClients, liveUnits, withClient, separately, inherited, away int
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
		           AND e.deleted_by = c.deleted_by AND e.delete_reason = c.delete_reason),
		       (SELECT count(*) FROM equipment WHERE deleted_at IS NOT NULL
		           AND (location_status <> 'at_client' OR active_transfer_id IS NOT NULL))`).
		Scan(&liveClients, &deletedClients, &clientBy, &liveUnits, &withClient, &separately, &inherited, &away))
	s.Equal(res.Clients, liveClients)
	s.Equal(1, deletedClients)
	s.Equal(adminID, clientBy)
	s.Equal(res.Equipment, liveUnits)
	s.Equal(1, withClient, "одна единица ушла вместе с клиентом")
	s.Equal(1, separately, "одна единица действующего клиента удалена отдельно")
	s.Equal(1, inherited, "единица, ушедшая с клиентом, наследует его «кто, когда, почему»")
	s.Zero(away, "удалённые единицы у клиента и свободны")

	s.assertDeletedCatalog(adminID, res)
	s.assertDeletedWarehouseSupplier(adminID)

	var distinctColumns int
	s.Require().NoError(s.Pool.QueryRow(ctx,
		`SELECT count(DISTINCT column_id) FROM tasks`).Scan(&distinctColumns))
	s.Equal(5, distinctColumns, "заказы должны быть во всех 5 колонках (D32)")

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
	s.Equal(12, tasksWithEquipment, "у каждого заказа есть оборудование клиента")

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
	s.assertWarehouses()
	s.assertExecutorPayouts()
}

// assertDeletedWarehouseSupplier — soft-delete-warehouses-suppliers: ровно по одной мягко
// удалённой записи видов «склад» и «поставщик», причина и автор заполнены; удалённый склад пуст
// (DS-021: ни остатка, ни оборудования, ни документов) и удалён из архива; действующих складов
// по-прежнему два, склад по умолчанию — действующий активный; на удалённого поставщика документы
// не ссылаются.
func (s *SeedDemoSuite) assertDeletedWarehouseSupplier(adminID string) {
	ctx := context.Background()
	for _, table := range []string{"warehouses", "suppliers"} {
		var deleted int
		var reason, by string
		s.Require().NoError(s.Pool.QueryRow(ctx, `
			SELECT count(*), coalesce(min(delete_reason), ''), coalesce(min(deleted_by::text), '')
			FROM `+table+` WHERE deleted_at IS NOT NULL`).Scan(&deleted, &reason, &by))
		s.Equal(1, deleted, "%s: одна удалённая запись", table)
		s.NotEmpty(reason, "%s: причина удаления", table)
		s.Equal(adminID, by, "%s: кто удалил", table)
	}
	var liveWarehouses, whStock, whEquipment, whDocs, supplierDocs int
	var whActive, defaultLive bool
	s.Require().NoError(s.Pool.QueryRow(ctx, `
		SELECT (SELECT count(*) FROM warehouses WHERE deleted_at IS NULL),
		       (SELECT is_active FROM warehouses WHERE deleted_at IS NOT NULL),
		       (SELECT count(*) FROM product_stock ps JOIN warehouses w ON w.id = ps.warehouse_id
		         WHERE w.deleted_at IS NOT NULL AND ps.stock <> 0),
		       (SELECT count(*) FROM equipment e JOIN warehouses w ON w.id = e.warehouse_id WHERE w.deleted_at IS NOT NULL),
		       (SELECT count(*) FROM transfers t JOIN warehouses w ON w.id IN (t.from_warehouse_id, t.to_warehouse_id)
		         WHERE w.deleted_at IS NOT NULL),
		       (SELECT count(*) FROM transfers t JOIN suppliers sp ON sp.id = t.supplier_id WHERE sp.deleted_at IS NOT NULL),
		       (SELECT EXISTS (SELECT 1 FROM warehouses WHERE id = default_warehouse_id() AND is_active AND deleted_at IS NULL))`).
		Scan(&liveWarehouses, &whActive, &whStock, &whEquipment, &whDocs, &supplierDocs, &defaultLive))
	s.Equal(2, liveWarehouses, "действующие склады — «Алматы» и «Москва»")
	s.False(whActive, "удалённый склад — из архива (бейдж «в архиве»)")
	s.Zero(whStock, "на удалённом складе нет остатка")
	s.Zero(whEquipment, "на удалённом складе нет оборудования")
	s.Zero(whDocs, "документов с удалённым складом нет")
	s.Zero(supplierDocs, "документов удалённого поставщика нет")
	s.True(defaultLive, "склад по умолчанию — действующий активный")
}

// assertDeletedCatalog — soft-delete-catalog: ровно по одной мягко удалённой записи каждого
// вида каталога с причиной и автором удаления — вкладки «Товары», «Услуги», «Категории», «Типы
// оборудования» раздела «Удалённые» на демо-стенде непустые. Действующих товаров и услуг —
// столько же, сколько в Result; удалённый товар — из архива и без остатка (DS-021); на
// удалённые категорию и тип никто не ссылается.
func (s *SeedDemoSuite) assertDeletedCatalog(adminID string, res seeddemo.Result) {
	ctx := context.Background()
	for _, table := range []string{"products", "services", "catalog_categories", "equipment_types"} {
		var deleted int
		var reason, by string
		s.Require().NoError(s.Pool.QueryRow(ctx, `
			SELECT count(*), coalesce(min(delete_reason), ''), coalesce(min(deleted_by::text), '')
			FROM `+table+` WHERE deleted_at IS NOT NULL`).Scan(&deleted, &reason, &by))
		s.Equal(1, deleted, "%s: одна удалённая запись", table)
		s.NotEmpty(reason, "%s: причина удаления", table)
		s.Equal(adminID, by, "%s: кто удалил", table)
	}
	var liveProducts, liveServices, deletedProductStock, refsToDeletedCategory, refsToDeletedType int
	var deletedProductActive bool
	s.Require().NoError(s.Pool.QueryRow(ctx, `
		SELECT (SELECT count(*) FROM products WHERE deleted_at IS NULL),
		       (SELECT count(*) FROM services WHERE deleted_at IS NULL),
		       (SELECT is_active FROM products WHERE deleted_at IS NOT NULL),
		       (SELECT count(*) FROM product_stock ps JOIN products p ON p.id = ps.product_id
		         WHERE p.deleted_at IS NOT NULL AND ps.stock <> 0),
		       (SELECT count(*) FROM catalog_categories c WHERE c.deleted_at IS NOT NULL AND (
		              EXISTS (SELECT 1 FROM products p WHERE p.category_id = c.id)
		           OR EXISTS (SELECT 1 FROM services sv WHERE sv.category_id = c.id))),
		       (SELECT count(*) FROM equipment e JOIN equipment_types t ON t.id = e.type_id WHERE t.deleted_at IS NOT NULL)`).
		Scan(&liveProducts, &liveServices, &deletedProductActive, &deletedProductStock, &refsToDeletedCategory, &refsToDeletedType))
	s.Equal(res.Products, liveProducts)
	s.Equal(res.Services, liveServices)
	s.False(deletedProductActive, "удалённый товар — из архива (бейдж «в архиве»)")
	s.Zero(deletedProductStock, "у удалённого товара нет остатка")
	s.Zero(refsToDeletedCategory, "на удалённую категорию никто не ссылается")
	s.Zero(refsToDeletedType, "на удалённый тип никто не ссылается")
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

// assertWarehouses — TS-F1 (transfers-core DS-029/DS-054; warehouses DS-012;
// transfers-partners): остатки на обоих складах; документы «Перемещение» всех четырёх
// направлений с консистентными движениями, в документе «в пути» между складами —
// строка-единица; местоположение — на единице: оборудование заказов последней колонки
// доски — «у клиента», незавершённых — «на складе» (часть — в Москве) либо «в пути» (одна —
// между складами, одна — к клиенту); журнала task_equipment_movements нет.
func (s *SeedDemoSuite) assertWarehouses() {
	ctx := context.Background()
	var whWithStock int
	s.Require().NoError(s.Pool.QueryRow(ctx,
		`SELECT count(DISTINCT warehouse_id) FROM product_stock WHERE stock > 0`).Scan(&whWithStock))
	s.Equal(2, whWithStock, "остатки раскиданы по Алматы и Москве")

	// transfers-partners TS-F1: документы по направлению и статусу. Склад → склад и приход от
	// поставщика — по одному received и in_transit; документы с клиентом: приём — только
	// проведённые (по одному на заказ с единицами «у клиента»), возврат — проведённые (заказы
	// последней колонки) и ровно один «в пути».
	statuses := map[string]int{}
	rows, err := s.Pool.Query(ctx, `SELECT direction || ':' || status, count(*) FROM transfers GROUP BY 1`)
	s.Require().NoError(err)
	for rows.Next() {
		var st string
		var n int
		s.Require().NoError(rows.Scan(&st, &n))
		statuses[st] = n
	}
	rows.Close()
	s.Equal(1, statuses["warehouse_to_warehouse:received"])
	s.Equal(1, statuses["warehouse_to_warehouse:in_transit"])
	s.Equal(1, statuses["supplier_to_warehouse:received"])
	s.Equal(1, statuses["supplier_to_warehouse:in_transit"])
	s.Positive(statuses["client_to_warehouse:received"], "приём от клиента проведён")
	s.Positive(statuses["warehouse_to_client:received"], "возврат клиенту проведён (заказы последней колонки)")
	s.Equal(1, statuses["warehouse_to_client:in_transit"], "один возврат клиенту в пути")
	// transfers-revert TS-F1: один отменённый документ у заказа (удаляется вместе с заказом).
	s.Equal(1, statuses["warehouse_to_client:cancelled"], "одна отменённая заявка на возврат")
	// soft-delete-transfers TS-F: ровно один мягко удалённый документ — заявка «склад → склад»
	// с причиной и автором удаления: вкладка «Перемещения» раздела «Удалённые» на демо-стенде
	// непустая; единиц оборудования он не занимает и остатки не двигал.
	s.Equal(1, statuses["warehouse_to_warehouse:requested"], "одна (мягко удалённая) заявка склад → склад")
	s.Len(statuses, 9, "других сочетаний направление × статус нет")
	var deletedDocs, deletedDocMoves, deletedDocUnits, deletedDocItems int
	var docState, docReason, docBy string
	s.Require().NoError(s.Pool.QueryRow(ctx, `
		SELECT count(*), coalesce(min(t.direction || ':' || t.status), ''), coalesce(min(t.delete_reason), ''),
		       coalesce(min(u.role), ''),
		       (SELECT count(*) FROM stock_movements m JOIN transfers d ON d.id = m.transfer_id WHERE d.deleted_at IS NOT NULL),
		       (SELECT count(*) FROM equipment e JOIN transfers d ON d.id = e.active_transfer_id WHERE d.deleted_at IS NOT NULL),
		       (SELECT count(*) FROM transfer_items i JOIN transfers d ON d.id = i.transfer_id WHERE d.deleted_at IS NOT NULL)
		FROM transfers t LEFT JOIN users u ON u.id = t.deleted_by
		WHERE t.deleted_at IS NOT NULL`).Scan(&deletedDocs, &docState, &docReason, &docBy, &deletedDocMoves, &deletedDocUnits, &deletedDocItems))
	s.Equal(1, deletedDocs)
	s.Equal("warehouse_to_warehouse:requested", docState)
	s.NotEmpty(docReason)
	s.Equal("admin", docBy, "удалил администратор")
	s.Zero(deletedDocMoves, "заявка остатки не двигала")
	s.Zero(deletedDocUnits, "единицы удалённой заявки свободны")
	s.Equal(1, deletedDocItems, "строки удалённого документа на месте")
	s.assertPartners()

	// журнал перемещений = qty_sent (out) и qty_received (in) по строкам отправленных документов
	// (заявка — мягко удалённая — остатки не двигает).
	var outSum, sentSum, inSum, receivedSum string
	s.Require().NoError(s.Pool.QueryRow(ctx, `
		SELECT (SELECT coalesce(-sum(qty_delta), 0)::text FROM stock_movements WHERE reason = 'transfer_out'),
		       (SELECT coalesce(sum(i.qty_sent), 0)::text FROM transfer_items i JOIN transfers t ON t.id = i.transfer_id
		         WHERE i.kind = 'product' AND t.direction = 'warehouse_to_warehouse' AND t.status <> 'requested'),
		       (SELECT coalesce(sum(qty_delta), 0)::text FROM stock_movements WHERE reason = 'transfer_in'),
		       (SELECT coalesce(sum(i.qty_received), 0)::text FROM transfer_items i JOIN transfers t ON t.id = i.transfer_id
		         WHERE i.kind = 'product' AND t.direction = 'warehouse_to_warehouse')`).Scan(&outSum, &sentSum, &inSum, &receivedSum))
	s.Equal(sentSum, outSum)
	s.Equal(receivedSum, inSum)

	// остаток пары = Σ журнала пары (сид пишет каждое изменение движением).
	var drift int
	s.Require().NoError(s.Pool.QueryRow(ctx, `
		SELECT count(*) FROM product_stock ps
		WHERE ps.stock <> coalesce((SELECT sum(m.qty_delta) FROM stock_movements m
		                            WHERE m.product_id = ps.product_id AND m.warehouse_id = ps.warehouse_id), 0)`).Scan(&drift))
	s.Zero(drift, "product_stock = Σ stock_movements по паре")

	var atMoscow, atAlmaty, inTransit int
	s.Require().NoError(s.Pool.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE e.location_status = 'at_warehouse' AND w.name = 'Москва'),
		       count(*) FILTER (WHERE e.location_status = 'at_warehouse' AND w.name = 'Алматы'),
		       count(*) FILTER (WHERE e.location_status = 'in_transit')
		FROM equipment e LEFT JOIN warehouses w ON w.id = e.warehouse_id`).Scan(&atMoscow, &atAlmaty, &inTransit))
	s.Positive(atMoscow, "часть оборудования демо-заказов — в Москве")
	s.Positive(atAlmaty, "часть оборудования демо-заказов — в Алматы")
	s.Equal(2, inTransit, "две единицы в пути: между складами и к клиенту")

	// единица «в пути» стоит строкой в документе in_transit и занята им (DS-019/DS-041).
	var transitLines, busyElsewhere int
	s.Require().NoError(s.Pool.QueryRow(ctx, `
		SELECT count(*) FROM transfer_items i
		JOIN transfers t ON t.id = i.transfer_id AND t.status = 'in_transit'
		JOIN equipment e ON e.id = i.equipment_id AND e.active_transfer_id = t.id AND e.location_status = 'in_transit'
		WHERE i.kind = 'equipment'`).Scan(&transitLines))
	s.Equal(2, transitLines, "строки-единицы в документах «в пути» (склад → склад и склад → клиент)")
	s.Require().NoError(s.Pool.QueryRow(ctx, `
		SELECT count(*) FROM equipment e
		WHERE e.active_transfer_id IS NOT NULL AND NOT EXISTS (
		    SELECT 1 FROM transfers t WHERE t.id = e.active_transfer_id AND t.status IN ('requested', 'in_transit'))`).Scan(&busyElsewhere))
	s.Zero(busyElsewhere, "active_transfer_id — только незавершённый документ")

	// DS-054: единицы, стоящие только в заказах последней колонки доски (и без заказов), —
	// «у клиента»; единицы незавершённых заказов — не «у клиента».
	var completedNotAtClient, openAtClient int
	s.Require().NoError(s.Pool.QueryRow(ctx, `
		WITH last_col AS (SELECT id FROM columns WHERE deleted_at IS NULL ORDER BY position DESC, id DESC LIMIT 1),
		open_eq AS (
		    SELECT DISTINCT te.equipment_id FROM task_equipment te JOIN tasks t ON t.id = te.task_id
		    WHERE t.column_id <> (SELECT id FROM last_col))
		SELECT count(*) FILTER (WHERE e.id NOT IN (SELECT equipment_id FROM open_eq) AND e.location_status <> 'at_client'),
		       count(*) FILTER (WHERE e.id IN (SELECT equipment_id FROM open_eq) AND e.location_status = 'at_client')
		FROM equipment e`).Scan(&completedNotAtClient, &openAtClient))
	s.Zero(completedNotAtClient, "оборудование завершённых заказов и без заказов — у клиента")
	s.Zero(openAtClient, "оборудование незавершённых заказов — на складе либо в пути")

	var journal bool
	s.Require().NoError(s.Pool.QueryRow(ctx, `SELECT to_regclass('task_equipment_movements') IS NOT NULL`).Scan(&journal))
	s.False(journal, "журнал task_equipment_movements удалён (DS-029)")
}

// assertPartners — transfers-partners TS-F1: два поставщика (один в архиве); приход от
// поставщика: полученный — с расхождением и движением supplier_in на факт, «в пути» — без
// движений (DS-040); документы с клиентом — по заказу, клиент = клиент заказа, единицы — из
// оборудования заказа; товар — только в накладных «склад → клиент» (transfers-sales TS-F1,
// assertInvoices); единицы заказов последней колонки —
// «у клиента» через проведённую пару «приём + возврат».
func (s *SeedDemoSuite) assertPartners() {
	ctx := context.Background()
	var active, archived int
	s.Require().NoError(s.Pool.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE is_active), count(*) FILTER (WHERE NOT is_active) FROM suppliers
		WHERE deleted_at IS NULL`).Scan(&active, &archived))
	s.Equal(1, active, "действующий поставщик")
	s.Equal(1, archived, "архивный поставщик")

	var ordered, fact, supplierIn string
	var transitMovements, archivedInDocs int
	s.Require().NoError(s.Pool.QueryRow(ctx, `
		SELECT (SELECT sum(i.qty_sent)::text FROM transfer_items i JOIN transfers t ON t.id = i.transfer_id
		         WHERE t.direction = 'supplier_to_warehouse' AND t.status = 'received'),
		       (SELECT sum(i.qty_received)::text FROM transfer_items i JOIN transfers t ON t.id = i.transfer_id
		         WHERE t.direction = 'supplier_to_warehouse' AND t.status = 'received'),
		       (SELECT coalesce(sum(qty_delta), 0)::text FROM stock_movements WHERE reason = 'supplier_in'),
		       (SELECT count(*) FROM stock_movements m JOIN transfers t ON t.id = m.transfer_id
		         WHERE t.direction = 'supplier_to_warehouse' AND t.status = 'in_transit'),
		       (SELECT count(*) FROM transfers t JOIN suppliers sp ON sp.id = t.supplier_id WHERE NOT sp.is_active)`).
		Scan(&ordered, &fact, &supplierIn, &transitMovements, &archivedInDocs))
	s.NotEqual(ordered, fact, "полученный приход — с расхождением")
	// transfers-revert TS-F1: полученный приход откатан и принят заново — ошибочная приёмка
	// (supplier_in на заказанное) погашена компенсирующим движением transfer_unreceive, итог
	// журнала = факт повторной приёмки; на документе — кто, когда и почему отменил проведение.
	var unreceive, net, reason string
	var unreceivedSet bool
	s.Require().NoError(s.Pool.QueryRow(ctx, `
		SELECT (SELECT coalesce(sum(qty_delta), 0)::text FROM stock_movements WHERE reason = 'transfer_unreceive'),
		       (SELECT coalesce(sum(qty_delta), 0)::text FROM stock_movements WHERE reason IN ('supplier_in', 'transfer_unreceive')),
		       t.unreceive_reason, t.unreceived_by IS NOT NULL AND t.unreceived_at < t.received_at
		FROM transfers t WHERE t.direction = 'supplier_to_warehouse' AND t.status = 'received'`).
		Scan(&unreceive, &net, &reason, &unreceivedSet))
	s.Equal("-"+ordered, unreceive, "transfer_unreceive = −ошибочный факт")
	s.Equal(fact, net, "supplier_in − откат = факт повторной приёмки")
	s.NotEqual(fact, supplierIn, "в журнале остались обе приёмки")
	s.NotEmpty(reason, "причина отката")
	s.True(unreceivedSet, "откат раньше повторной приёмки")
	s.Zero(transitMovements, "приход «в пути» движений не создаёт")
	s.Zero(archivedInDocs, "документы — на действующего поставщика")

	var foreignClient, notInTask, productLines, clientMovements int
	s.Require().NoError(s.Pool.QueryRow(ctx, `
		SELECT (SELECT count(*) FROM transfers t JOIN tasks k ON k.id = t.task_id
		         WHERE t.direction IN ('client_to_warehouse', 'warehouse_to_client') AND t.client_id <> k.client_id),
		       (SELECT count(*) FROM transfer_items i JOIN transfers t ON t.id = i.transfer_id
		         WHERE t.task_id IS NOT NULL AND i.kind = 'equipment' AND NOT EXISTS (
		             SELECT 1 FROM task_equipment te WHERE te.task_id = t.task_id AND te.equipment_id = i.equipment_id)),
		       (SELECT count(*) FROM transfer_items i JOIN transfers t ON t.id = i.transfer_id
		         WHERE t.direction = 'client_to_warehouse' AND i.kind <> 'equipment'),
		       (SELECT count(*) FROM stock_movements m JOIN transfers t ON t.id = m.transfer_id
		         WHERE t.direction = 'client_to_warehouse')`).
		Scan(&foreignClient, &notInTask, &productLines, &clientMovements))
	s.Zero(foreignClient, "клиент документа = клиент заказа (DS-012)")
	s.Zero(notInTask, "единицы документа — из оборудования заказа (DS-035)")
	s.Zero(productLines, "в приёме от клиента — только оборудование (DS-004)")
	s.Zero(clientMovements, "приём от клиента остаток не двигает (DS-040)")
	s.assertInvoices()

	// единица «у клиента» из заказа последней колонки прошла пару «приём + возврат».
	var withoutPair int
	s.Require().NoError(s.Pool.QueryRow(ctx, `
		WITH last_col AS (SELECT id FROM columns WHERE deleted_at IS NULL ORDER BY position DESC, id DESC LIMIT 1)
		SELECT count(*) FROM task_equipment te
		JOIN tasks k ON k.id = te.task_id AND k.column_id = (SELECT id FROM last_col)
		JOIN equipment e ON e.id = te.equipment_id AND e.location_status = 'at_client'
		WHERE (SELECT count(DISTINCT t.direction) FROM transfer_items i JOIN transfers t ON t.id = i.transfer_id
		       WHERE i.equipment_id = te.equipment_id AND t.task_id = te.task_id AND t.status = 'received') <> 2`).Scan(&withoutPair))
	s.Zero(withoutPair, "единицы завершённых заказов вернулись клиенту парой документов")

	// «в пути → клиент»: единица занята документом «склад → клиент» в пути.
	var toClient int
	s.Require().NoError(s.Pool.QueryRow(ctx, `
		SELECT count(*) FROM equipment e JOIN transfers t ON t.id = e.active_transfer_id
		WHERE e.location_status = 'in_transit' AND t.direction = 'warehouse_to_client' AND t.status = 'in_transit'`).Scan(&toClient))
	s.Equal(1, toClient)
}

// assertInvoices — transfers-sales TS-F1 (DS-011, DS-014): две накладные клиенту с товаром и
// ценой — полученная в оплаченном заказе и «в пути» в неоплаченном; товар списан движением
// client_out на количество строки со склада отправления; цена есть только у товарных строк
// накладных; сумма каждой накладной больше нуля.
func (s *SeedDemoSuite) assertInvoices() {
	ctx := context.Background()
	var receivedPaid, transitUnpaid, unpriced, pricedElsewhere, movementDrift, totalDrift int
	s.Require().NoError(s.Pool.QueryRow(ctx, `
		WITH inv AS (
		    SELECT t.id, t.status, t.task_id, t.from_warehouse_id, k.is_paid, i.product_id, i.qty_sent, i.price
		    FROM transfer_items i JOIN transfers t ON t.id = i.transfer_id JOIN tasks k ON k.id = t.task_id
		    WHERE t.direction = 'warehouse_to_client' AND i.kind = 'product')
		SELECT (SELECT count(*) FROM inv WHERE status = 'received' AND is_paid),
		       (SELECT count(*) FROM inv WHERE status = 'in_transit' AND NOT is_paid),
		       (SELECT count(*) FROM inv WHERE price IS NULL OR price <= 0),
		       (SELECT count(*) FROM transfer_items i JOIN transfers t ON t.id = i.transfer_id
		         WHERE i.price IS NOT NULL AND (t.direction <> 'warehouse_to_client' OR i.kind <> 'product')),
		       (SELECT count(*) FROM inv WHERE NOT EXISTS (
		             SELECT 1 FROM stock_movements m
		             WHERE m.transfer_id = inv.id AND m.reason = 'client_out' AND m.product_id = inv.product_id
		               AND m.warehouse_id = inv.from_warehouse_id AND m.qty_delta = -inv.qty_sent AND m.task_id IS NULL)),
		       (SELECT count(*) FROM (
		             SELECT inv.task_id, sum(round(inv.price * inv.qty_sent, 2)) AS sold FROM inv GROUP BY inv.task_id) x
		         WHERE x.sold <= 0)`).
		Scan(&receivedPaid, &transitUnpaid, &unpriced, &pricedElsewhere, &movementDrift, &totalDrift))
	s.Equal(1, receivedPaid, "полученная накладная с товаром в оплаченном заказе")
	s.Equal(1, transitUnpaid, "накладная «в пути» с товаром в неоплаченном заказе")
	s.Zero(unpriced, "у товарной строки накладной есть цена")
	s.Zero(pricedElsewhere, "цена — только у товара накладной клиенту (DS-011)")
	s.Zero(movementDrift, "товар накладной списан client_out со склада отправления")
	s.Zero(totalDrift, "сумма накладной больше нуля")
	// демо-остатки проданных товаров не уходят в минус ни на одном складе.
	var negative int
	s.Require().NoError(s.Pool.QueryRow(ctx, `
		SELECT count(*) FROM product_stock ps
		WHERE ps.stock < 0 AND ps.product_id IN (
		    SELECT i.product_id FROM transfer_items i JOIN transfers t ON t.id = i.transfer_id
		    WHERE t.direction = 'warehouse_to_client' AND i.kind = 'product')`).Scan(&negative))
	s.Zero(negative, "остаток товара накладной на складе ≥ 0")
	var clientOut int
	s.Require().NoError(s.Pool.QueryRow(ctx, `SELECT count(*) FROM stock_movements WHERE reason = 'client_out'`).Scan(&clientOut))
	s.Equal(2, clientOut, "client_out — по одному на товарную строку накладной")
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

	_, err := seeddemo.Run(ctx, s.Pool)
	s.Require().Error(err)
	s.True(errors.Is(err, seeddemo.ErrNoAdmin))

	var clientsTotal int
	s.Require().NoError(s.Pool.QueryRow(ctx, `SELECT count(*) FROM clients`).Scan(&clientsTotal))
	s.Zero(clientsTotal)
}

// assertStockInvariant — design-sql.md §Reconcile Query: «Σ stock_movements.qty_delta по
// заказу = −Σ task_stock.qty» (эталон board.assertStockInvariant, stock_engine_integration_test.go).
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
