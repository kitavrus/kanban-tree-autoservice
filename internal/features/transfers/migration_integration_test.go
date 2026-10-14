//go:build integration

package transfers_test

import (
	"context"
	"errors"
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

// TransfersMigration0009Suite — миграция 0009 (DS-006/DS-007): таблицы перемещений,
// CHECK from ≠ to, FK stock_movements.transfer_id ON DELETE SET NULL; down удаляет таблицы,
// движения transfer_* сохраняются без ссылки.
type TransfersMigration0009Suite struct {
	testutil.BaseSuite
}

func TestTransfersMigration0009Suite(t *testing.T) {
	suite.Run(t, new(TransfersMigration0009Suite))
}

const stmAdminID = "aaaaaaaa-0009-0009-0009-000000000001"

func migrationsURL() string {
	_, f, _, _ := runtime.Caller(0)
	return "file://" + filepath.Join(filepath.Dir(f), "..", "..", "..", "migrations")
}

func (s *TransfersMigration0009Suite) tableExists(name string) bool {
	var ok bool
	s.Require().NoError(s.Pool.QueryRow(context.Background(), `SELECT to_regclass($1) IS NOT NULL`, name).Scan(&ok))
	return ok
}

func (s *TransfersMigration0009Suite) TestUpDown() {
	ctx := context.Background()
	mig, err := migrate.New(migrationsURL(), s.DSN)
	s.Require().NoError(err)
	defer func() { _, _ = mig.Close() }()

	s.Require().NoError(mig.Migrate(8))
	defer func() { s.Require().NoError(mig.Up()) }()
	s.False(s.tableExists("stock_transfers"))

	s.Require().NoError(mig.Migrate(9))
	s.True(s.tableExists("stock_transfers"))
	s.True(s.tableExists("stock_transfer_items"))

	_, err = s.Pool.Exec(ctx, `INSERT INTO users (id, login, password_hash, name, role) VALUES ($1, 'stm-admin', 'x', 'Admin', 'admin')`, stmAdminID)
	s.Require().NoError(err)
	var almaty, moscow, catID, productID string
	s.Require().NoError(s.Pool.QueryRow(ctx, `SELECT id FROM warehouses WHERE name = 'Алматы'`).Scan(&almaty))
	s.Require().NoError(s.Pool.QueryRow(ctx, `SELECT id FROM warehouses WHERE name = 'Москва'`).Scan(&moscow))
	s.Require().NoError(s.Pool.QueryRow(ctx, `INSERT INTO catalog_categories (kind, name) VALUES ('product', 'STM') RETURNING id`).Scan(&catID))
	s.Require().NoError(s.Pool.QueryRow(ctx, `INSERT INTO products (category_id, name) VALUES ($1, 'STM товар') RETURNING id`, catID).Scan(&productID))

	// from = to запрещено CHECK'ом.
	_, err = s.Pool.Exec(ctx, `INSERT INTO stock_transfers (from_warehouse_id, to_warehouse_id, created_by) VALUES ($1, $1, $2)`, almaty, stmAdminID)
	s.Error(err)

	var transferID int64
	s.Require().NoError(s.Pool.QueryRow(ctx,
		`INSERT INTO stock_transfers (from_warehouse_id, to_warehouse_id, created_by) VALUES ($1, $2, $3) RETURNING id`,
		almaty, moscow, stmAdminID).Scan(&transferID))
	_, err = s.Pool.Exec(ctx, `INSERT INTO stock_transfer_items (transfer_id, product_id, position, qty_sent) VALUES ($1, $2, 1, 0)`, transferID, productID)
	s.Error(err, "qty_sent > 0")
	_, err = s.Pool.Exec(ctx, `INSERT INTO stock_transfer_items (transfer_id, product_id, position, qty_sent, qty_received) VALUES ($1, $2, 1, 2, 3)`, transferID, productID)
	s.Error(err, "qty_received ≤ qty_sent")
	_, err = s.Pool.Exec(ctx, `INSERT INTO stock_movements (product_id, warehouse_id, qty_delta, reason, user_id, transfer_id) VALUES ($1, $2, -1, 'transfer_out', $3, $4)`,
		productID, almaty, stmAdminID, transferID)
	s.Require().NoError(err)
	_, err = s.Pool.Exec(ctx, `INSERT INTO stock_movements (product_id, warehouse_id, qty_delta, reason, user_id, transfer_id) VALUES ($1, $2, -1, 'transfer_out', $3, 999999)`,
		productID, almaty, stmAdminID)
	s.Error(err, "FK transfer_id")

	// ON DELETE SET NULL.
	_, err = s.Pool.Exec(ctx, `DELETE FROM stock_transfers WHERE id = $1`, transferID)
	s.Require().NoError(err)
	var nullRef int
	s.Require().NoError(s.Pool.QueryRow(ctx, `SELECT count(*) FROM stock_movements WHERE product_id = $1 AND transfer_id IS NULL`, productID).Scan(&nullRef))
	s.Equal(1, nullRef)

	// down → 0008: таблиц нет, движение осталось.
	s.Require().NoError(mig.Migrate(8))
	s.False(s.tableExists("stock_transfers"))
	s.False(s.tableExists("stock_transfer_items"))
	var cnt int
	s.Require().NoError(s.Pool.QueryRow(ctx, `SELECT count(*) FROM stock_movements WHERE product_id = $1`, productID).Scan(&cnt))
	s.Equal(1, cnt)
}

// TransfersCoreMigrationSuite — миграция 0014 (transfers-core TS-026, DS-029/DS-030/DS-054):
// переименование stock_transfers → transfers (+ kind = product у существующих строк), перенос
// местоположения со строк заказа на единицу оборудования по правилу «завершённый заказ =
// заказ в последней колонке доски», удаление журнала task_equipment_movements, новые CHECK'и.
type TransfersCoreMigrationSuite struct {
	testutil.BaseSuite
}

func TestTransfersCoreMigrationSuite(t *testing.T) {
	suite.Run(t, new(TransfersCoreMigrationSuite))
}

const tcmAdminID = "aaaaaaaa-0014-0014-0014-000000000001"

// upToLatest — вернуть схему на последнюю версию (тест может закончиться уже на ней).
func (s *TransfersCoreMigrationSuite) upToLatest(mig *migrate.Migrate) {
	if err := mig.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		s.Require().NoError(err)
	}
}

func (s *TransfersCoreMigrationSuite) exists(q string, args ...any) bool {
	var ok bool
	s.Require().NoError(s.Pool.QueryRow(context.Background(), q, args...).Scan(&ok))
	return ok
}

func (s *TransfersCoreMigrationSuite) tableExists(name string) bool {
	return s.exists(`SELECT to_regclass($1) IS NOT NULL`, name)
}

func (s *TransfersCoreMigrationSuite) columnExists(table, column string) bool {
	return s.exists(`SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = $1 AND column_name = $2)`, table, column)
}

func (s *TransfersCoreMigrationSuite) scalar(q string, args ...any) string {
	var v string
	s.Require().NoError(s.Pool.QueryRow(context.Background(), q, args...).Scan(&v), q)
	return v
}

func (s *TransfersCoreMigrationSuite) exec(q string, args ...any) {
	_, err := s.Pool.Exec(context.Background(), q, args...)
	s.Require().NoError(err, q)
}

// tcmFixture — данные «до» (версия 0013): склады, клиент, тип, админ.
type tcmFixture struct {
	almaty, moscow, clientID, typeID string
}

func (s *TransfersCoreMigrationSuite) fixture() tcmFixture {
	var f tcmFixture
	s.exec(`INSERT INTO users (id, login, password_hash, name, role) VALUES ($1, 'tcm-admin', 'x', 'Admin', 'admin') ON CONFLICT (id) DO NOTHING`, tcmAdminID)
	f.almaty = s.scalar(`SELECT id::text FROM warehouses WHERE name = 'Алматы'`)
	f.moscow = s.scalar(`SELECT id::text FROM warehouses WHERE name = 'Москва'`)
	f.clientID = s.scalar(`INSERT INTO clients (company_name) VALUES ('TCM ' || gen_random_uuid()) RETURNING id::text`)
	f.typeID = s.scalar(`INSERT INTO equipment_types (name) VALUES ('TCM ' || gen_random_uuid()) RETURNING id::text`)
	return f
}

func (s *TransfersCoreMigrationSuite) newEquipment(f tcmFixture, model string) string {
	return s.scalar(`INSERT INTO equipment (client_id, type_id, model) VALUES ($1, $2, $3) RETURNING id::text`, f.clientID, f.typeID, model)
}

func (s *TransfersCoreMigrationSuite) newTask(f tcmFixture, columnID, title string) int64 {
	var id int64
	s.Require().NoError(s.Pool.QueryRow(context.Background(),
		`INSERT INTO tasks (column_id, client_id, title, position, created_by) VALUES ($1, $2, $3, 1, $4) RETURNING id`,
		columnID, f.clientID, title, tcmAdminID).Scan(&id))
	return id
}

// atWarehouse / inTransit — строка task_equipment модели «до» (0010).
func (s *TransfersCoreMigrationSuite) atWarehouse(taskID int64, equipmentID, warehouseID string) {
	s.exec(`INSERT INTO task_equipment (task_id, equipment_id, warehouse_id) VALUES ($1, $2, $3)`, taskID, equipmentID, warehouseID)
}

func (s *TransfersCoreMigrationSuite) inTransit(taskID int64, equipmentID, from, to string) {
	s.exec(`INSERT INTO task_equipment (task_id, equipment_id, warehouse_id, status, transit_from_warehouse_id, transit_to_warehouse_id)
		VALUES ($1, $2, NULL, 'in_transit', $3, $4)`, taskID, equipmentID, from, to)
}

// location — «статус|склад» единицы после миграции (склад — имя либо пусто).
func (s *TransfersCoreMigrationSuite) location(equipmentID string) string {
	return s.scalar(`
		SELECT e.location_status || '|' || coalesce(w.name, '') || '|' || coalesce(e.active_transfer_id::text, '')
		FROM equipment e LEFT JOIN warehouses w ON w.id = e.warehouse_id WHERE e.id = $1`, equipmentID)
}

// TS-026: данные «до» по каждому случаю DS-054 (а)–(е) + негативы; переименование таблиц,
// kind = product, удаление журнала, CHECK'и; down возвращает модель 0013.
func (s *TransfersCoreMigrationSuite) TestTS026_MigrationLocations() {
	ctx := context.Background()
	mig, err := migrate.New(migrationsURL(), s.DSN)
	s.Require().NoError(err)
	defer func() { _, _ = mig.Close() }()

	s.Require().NoError(mig.Migrate(13))
	defer s.upToLatest(mig)
	s.True(s.tableExists("stock_transfers"))
	s.False(s.tableExists("transfers"))

	f := s.fixture()
	lastCol := s.scalar(`SELECT id::text FROM columns ORDER BY position DESC, id DESC LIMIT 1`)
	openCol := s.scalar(`SELECT id::text FROM columns ORDER BY position, id LIMIT 1`)
	// негатив: колонка с is_final = true, но НЕ последняя по position, — незавершённая.
	finalNotLast := s.scalar(`UPDATE columns SET is_final = true WHERE name = 'В работе' RETURNING id::text`)
	s.NotEqual(lastCol, finalNotLast)

	done := s.newTask(f, lastCol, "TCM done")
	open1 := s.newTask(f, openCol, "TCM open 1")
	open2 := s.newTask(f, openCol, "TCM open 2")
	flagged := s.newTask(f, finalNotLast, "TCM is_final not last")
	s.Greater(open2, open1)

	eqA := s.newEquipment(f, "A only completed")
	eqB := s.newEquipment(f, "B open at warehouse")
	eqC := s.newEquipment(f, "C open in transit")
	eqD := s.newEquipment(f, "D completed + open")
	eqE := s.newEquipment(f, "E two open")
	eqF := s.newEquipment(f, "F no tasks")
	eqG := s.newEquipment(f, "G is_final not last")

	s.atWarehouse(done, eqA, f.almaty)          // (а)
	s.atWarehouse(open1, eqB, f.moscow)         // (б)
	s.inTransit(open1, eqC, f.almaty, f.moscow) // (в)
	s.atWarehouse(done, eqD, f.almaty)          // (г)
	s.atWarehouse(open1, eqD, f.moscow)         // (г)
	s.atWarehouse(open1, eqE, f.almaty)         // (д)
	s.atWarehouse(open2, eqE, f.moscow)         // (д) — больший id заказа
	s.atWarehouse(flagged, eqG, f.moscow)       // негатив is_final
	s.exec(`INSERT INTO task_equipment_movements (task_id, equipment_id, action, to_warehouse_id, user_id) VALUES ($1, $2, 'received', $3, $4)`,
		open1, eqB, f.moscow, tcmAdminID)

	// перемещение товаров «до» + движение со ссылкой на него.
	catID := s.scalar(`INSERT INTO catalog_categories (kind, name) VALUES ('product', 'TCM ' || gen_random_uuid()) RETURNING id::text`)
	productID := s.scalar(`INSERT INTO products (category_id, name) VALUES ($1, 'TCM товар') RETURNING id::text`, catID)
	var oldTransfer int64
	s.Require().NoError(s.Pool.QueryRow(ctx,
		`INSERT INTO stock_transfers (from_warehouse_id, to_warehouse_id, status, created_by, sent_by, sent_at)
		 VALUES ($1, $2, 'in_transit', $3, $3, now()) RETURNING id`, f.almaty, f.moscow, tcmAdminID).Scan(&oldTransfer))
	s.exec(`INSERT INTO stock_transfer_items (transfer_id, product_id, position, qty_sent) VALUES ($1, $2, 1, 4)`, oldTransfer, productID)
	s.exec(`INSERT INTO stock_movements (product_id, warehouse_id, qty_delta, reason, user_id, transfer_id) VALUES ($1, $2, -4, 'transfer_out', $3, $4)`,
		productID, f.almaty, tcmAdminID, oldTransfer)

	// ---- up → 0014 ----
	s.Require().NoError(mig.Migrate(14))

	// DS-030: таблицы, индексы, ограничения переименованы; строки целы, kind = product.
	s.False(s.tableExists("stock_transfers"))
	s.False(s.tableExists("stock_transfer_items"))
	s.True(s.tableExists("transfers"))
	s.True(s.tableExists("transfer_items"))
	s.Equal("in_transit", s.scalar(`SELECT status FROM transfers WHERE id = $1`, oldTransfer))
	s.Equal("product|4.000", s.scalar(`SELECT kind || '|' || qty_sent::text FROM transfer_items WHERE transfer_id = $1`, oldTransfer))
	s.Equal("1", s.scalar(`SELECT count(*)::text FROM stock_movements WHERE transfer_id = $1`, oldTransfer))
	for _, idx := range []string{"transfers_status_idx", "transfers_from_warehouse_id_idx", "transfers_to_warehouse_id_idx",
		"transfer_items_product_id_idx", "transfer_items_equipment_id_idx", "equipment_warehouse_id_idx", "equipment_active_transfer_id_idx"} {
		s.True(s.exists(`SELECT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = $1)`, idx), idx)
	}
	for _, con := range []string{"transfers_distinct_warehouses", "transfers_pkey", "transfer_items_pkey",
		"transfer_items_kind_check", "transfer_items_transfer_id_product_id_key", "transfer_items_transfer_id_equipment_id_key",
		"equipment_location_check"} {
		s.True(s.exists(`SELECT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = $1)`, con), con)
	}
	s.False(s.exists(`SELECT EXISTS (SELECT 1 FROM pg_constraint WHERE conname LIKE 'stock\_transfer%')`), "старых имён ограничений нет")
	s.False(s.exists(`SELECT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname LIKE 'stock\_transfer%')`), "старых имён индексов нет")
	// новый номер документа продолжает прежнюю последовательность.
	var nextTransfer int64
	s.Require().NoError(s.Pool.QueryRow(ctx,
		`INSERT INTO transfers (from_warehouse_id, to_warehouse_id, created_by) VALUES ($1, $2, $3) RETURNING id`,
		f.almaty, f.moscow, tcmAdminID).Scan(&nextTransfer))
	s.Greater(nextTransfer, oldTransfer)

	// DS-029: журнал и складские колонки строки заказа удалены.
	s.False(s.tableExists("task_equipment_movements"))
	for _, col := range []string{"warehouse_id", "status", "transit_from_warehouse_id", "transit_to_warehouse_id"} {
		s.False(s.columnExists("task_equipment", col), col)
	}
	s.Equal("8", s.scalar(`SELECT count(*)::text FROM task_equipment WHERE task_id = ANY($1)`, []int64{done, open1, open2, flagged}), "строки заказов целы")

	// DS-054: местоположение единиц.
	s.Equal("at_client||", s.location(eqA), "(а) только в завершённом заказе")
	s.Equal("at_warehouse|Москва|", s.location(eqB), "(б) незавершённый, на складе")
	s.Equal("at_warehouse|Алматы|", s.location(eqC), "(в) незавершённый, в пути → склад отправления")
	s.Equal("at_warehouse|Москва|", s.location(eqD), "(г) завершённый + незавершённый → склад незавершённого")
	s.Equal("at_warehouse|Москва|", s.location(eqE), "(д) два незавершённых → заказ с большим id")
	s.Equal("at_client||", s.location(eqF), "(е) без заказов")
	s.Equal("at_warehouse|Москва|", s.location(eqG), "is_final, но не последняя колонка — незавершённый")

	// CHECK transfer_items_kind_check + UNIQUE единицы в документе.
	for _, tc := range []struct {
		name string
		q    string
		args []any
	}{
		{"product без qty_sent", `INSERT INTO transfer_items (transfer_id, kind, product_id, position) VALUES ($1, 'product', $2, 9)`,
			[]any{nextTransfer, productID}},
		{"product с equipment_id", `INSERT INTO transfer_items (transfer_id, kind, product_id, equipment_id, position, qty_sent) VALUES ($1, 'product', $2, $3, 9, 1)`,
			[]any{nextTransfer, productID, eqB}},
		{"equipment с qty_sent", `INSERT INTO transfer_items (transfer_id, kind, equipment_id, position, qty_sent) VALUES ($1, 'equipment', $2, 9, 1)`,
			[]any{nextTransfer, eqB}},
		{"equipment с product_id", `INSERT INTO transfer_items (transfer_id, kind, equipment_id, product_id, position) VALUES ($1, 'equipment', $2, $3, 9)`,
			[]any{nextTransfer, eqB, productID}},
		{"equipment без equipment_id", `INSERT INTO transfer_items (transfer_id, kind, position) VALUES ($1, 'equipment', 9)`,
			[]any{nextTransfer}},
		{"неизвестный kind", `INSERT INTO transfer_items (transfer_id, kind, equipment_id, position) VALUES ($1, 'service', $2, 9)`,
			[]any{nextTransfer, eqB}},
		{"equipment с qty_received", `INSERT INTO transfer_items (transfer_id, kind, equipment_id, position, qty_received) VALUES ($1, 'equipment', $2, 9, 0)`,
			[]any{nextTransfer, eqB}},
	} {
		_, err := s.Pool.Exec(ctx, tc.q, tc.args...)
		s.Error(err, tc.name)
	}
	s.exec(`INSERT INTO transfer_items (transfer_id, kind, equipment_id, position) VALUES ($1, 'equipment', $2, 1)`, nextTransfer, eqB)
	_, err = s.Pool.Exec(ctx, `INSERT INTO transfer_items (transfer_id, kind, equipment_id, position) VALUES ($1, 'equipment', $2, 2)`, nextTransfer, eqB)
	s.Error(err, "дубль единицы в документе")
	_, err = s.Pool.Exec(ctx, `DELETE FROM equipment WHERE id = $1`, eqB)
	s.Error(err, "RESTRICT: единица со строкой документа / заказа не удаляется")

	// CHECK equipment_location_check (+ допустимые значения статуса).
	for name, q := range map[string]string{
		"in_transit без active_transfer_id": `UPDATE equipment SET location_status = 'in_transit', warehouse_id = NULL WHERE id = $1`,
		"at_warehouse без склада":           `UPDATE equipment SET location_status = 'at_warehouse', warehouse_id = NULL WHERE id = $1`,
		"at_client со складом":              `UPDATE equipment SET location_status = 'at_client' WHERE id = $1`,
		"неизвестный статус":                `UPDATE equipment SET location_status = 'lost', warehouse_id = NULL WHERE id = $1`,
	} {
		_, err := s.Pool.Exec(ctx, q, eqB)
		s.Error(err, name)
	}
	s.exec(`UPDATE equipment SET location_status = 'in_transit', warehouse_id = NULL, active_transfer_id = $2 WHERE id = $1`, eqB, nextTransfer)
	s.Equal(fmt.Sprintf("in_transit||%d", nextTransfer), s.location(eqB), "корректный «в пути»")

	// ---- down → 0013: прежняя модель ----
	s.Require().NoError(mig.Migrate(13))
	s.True(s.tableExists("stock_transfers"))
	s.True(s.tableExists("stock_transfer_items"))
	s.False(s.tableExists("transfers"))
	s.True(s.tableExists("task_equipment_movements"))
	s.False(s.columnExists("equipment", "location_status"))
	s.False(s.columnExists("stock_transfer_items", "kind"))
	s.Equal("1", s.scalar(`SELECT count(*)::text FROM stock_transfer_items WHERE transfer_id = $1`, oldTransfer), "товарная строка цела")
	s.Equal("0", s.scalar(`SELECT count(*)::text FROM stock_transfer_items WHERE transfer_id = $1`, nextTransfer), "строка-единица удалена")
	s.Equal("at_warehouse|Москва", s.scalar(`
		SELECT te.status || '|' || w.name FROM task_equipment te JOIN warehouses w ON w.id = te.warehouse_id
		WHERE te.task_id = $1 AND te.equipment_id = $2`, flagged, eqG), "склад строки заказа — со склада единицы")
	s.Equal("at_warehouse|Алматы", s.scalar(`
		SELECT te.status || '|' || w.name FROM task_equipment te JOIN warehouses w ON w.id = te.warehouse_id
		WHERE te.task_id = $1 AND te.equipment_id = $2`, done, eqA), "единица «у клиента» → склад по умолчанию")
	s.True(s.exists(`SELECT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'stock_transfers_distinct_warehouses')`))

	s.exec(`DELETE FROM tasks WHERE id = ANY($1)`, []int64{done, open1, open2, flagged})
}

// TS-026 (негатив): на доске одна колонка — она же последняя, все заказы завершённые, все
// единицы → at_client. Имя метода сортируется после TestTS026_MigrationLocations (тот
// опирается на колонки по умолчанию).
func (s *TransfersCoreMigrationSuite) TestTS026_SingleColumnAllAtClient() {
	mig, err := migrate.New(migrationsURL(), s.DSN)
	s.Require().NoError(err)
	defer func() { _, _ = mig.Close() }()

	s.Require().NoError(mig.Migrate(13))
	defer s.upToLatest(mig)

	f := s.fixture()
	s.exec(`DELETE FROM tasks`)
	only := s.scalar(`SELECT id::text FROM columns ORDER BY position, id LIMIT 1`)
	s.exec(`DELETE FROM columns WHERE id <> $1`, only)
	task := s.newTask(f, only, "TCM single column")
	atWh := s.newEquipment(f, "single at warehouse")
	moving := s.newEquipment(f, "single in transit")
	s.atWarehouse(task, atWh, f.moscow)
	s.inTransit(task, moving, f.almaty, f.moscow)

	s.Require().NoError(mig.Migrate(14))
	s.Equal("at_client||", s.location(atWh))
	s.Equal("at_client||", s.location(moving))
	s.Equal("0", s.scalar(`SELECT count(*)::text FROM equipment WHERE location_status <> 'at_client'`), "все единицы — у клиента")
}

// TransfersPartnersMigrationSuite — миграция 0015 (transfers-partners §4, DS-002/DS-012/
// DS-015/DS-040): направление и точки маршрута документа (CHECK transfers_route_check вместо
// transfers_distinct_warehouses), необязательные склады, FK заказа / клиента / поставщика
// RESTRICT, снятие «факт ≤ отправленного», три новые причины движений; down возвращает модель
// 0014 — документы новых направлений удаляются, их единицы освобождаются.
type TransfersPartnersMigrationSuite struct {
	testutil.BaseSuite
}

func TestTransfersPartnersMigrationSuite(t *testing.T) {
	suite.Run(t, new(TransfersPartnersMigrationSuite))
}

const tpmAdminID = "aaaaaaaa-0015-0015-0015-000000000001"

func (s *TransfersPartnersMigrationSuite) exists(q string, args ...any) bool {
	var ok bool
	s.Require().NoError(s.Pool.QueryRow(context.Background(), q, args...).Scan(&ok), q)
	return ok
}

func (s *TransfersPartnersMigrationSuite) scalar(q string, args ...any) string {
	var v string
	s.Require().NoError(s.Pool.QueryRow(context.Background(), q, args...).Scan(&v), q)
	return v
}

func (s *TransfersPartnersMigrationSuite) exec(q string, args ...any) {
	_, err := s.Pool.Exec(context.Background(), q, args...)
	s.Require().NoError(err, q)
}

func (s *TransfersPartnersMigrationSuite) fails(name, q string, args ...any) {
	_, err := s.Pool.Exec(context.Background(), q, args...)
	s.Error(err, name)
}

func (s *TransfersPartnersMigrationSuite) constraint(name string) bool {
	return s.exists(`SELECT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = $1)`, name)
}

func (s *TransfersPartnersMigrationSuite) column(table, column string) bool {
	return s.exists(`SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = $1 AND column_name = $2)`, table, column)
}

func (s *TransfersPartnersMigrationSuite) TestMigration0015_UpDown() {
	ctx := context.Background()
	mig, err := migrate.New(migrationsURL(), s.DSN)
	s.Require().NoError(err)
	defer func() { _, _ = mig.Close() }()

	s.Require().NoError(mig.Migrate(14))
	defer func() {
		if err := mig.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			s.Require().NoError(err)
		}
	}()

	// ---- данные «до» (0014): два документа склад → склад и движение ----
	s.exec(`INSERT INTO users (id, login, password_hash, name, role) VALUES ($1, 'tpm-admin', 'x', 'Admin', 'admin') ON CONFLICT (id) DO NOTHING`, tpmAdminID)
	almaty := s.scalar(`SELECT id::text FROM warehouses WHERE name = 'Алматы'`)
	moscow := s.scalar(`SELECT id::text FROM warehouses WHERE name = 'Москва'`)
	clientID := s.scalar(`INSERT INTO clients (company_name) VALUES ('TPM ' || gen_random_uuid()) RETURNING id::text`)
	typeID := s.scalar(`INSERT INTO equipment_types (name) VALUES ('TPM ' || gen_random_uuid()) RETURNING id::text`)
	catID := s.scalar(`INSERT INTO catalog_categories (kind, name) VALUES ('product', 'TPM ' || gen_random_uuid()) RETURNING id::text`)
	productID := s.scalar(`INSERT INTO products (category_id, name) VALUES ($1, 'TPM товар') RETURNING id::text`, catID)
	supplierID := s.scalar(`INSERT INTO suppliers (name) VALUES ('TPM ' || gen_random_uuid()) RETURNING id::text`)
	var taskID, oldTransit, oldReceived int64
	s.Require().NoError(s.Pool.QueryRow(ctx, `
		INSERT INTO tasks (column_id, client_id, title, position, created_by)
		VALUES ((SELECT id FROM columns ORDER BY position, id LIMIT 1), $1, 'TPM заказ', 1, $2) RETURNING id`, clientID, tpmAdminID).Scan(&taskID))
	defer s.exec(`DELETE FROM tasks WHERE id = $1`, taskID)
	unit := s.scalar(`INSERT INTO equipment (client_id, type_id, model) VALUES ($1, $2, 'TPM unit') RETURNING id::text`, clientID, typeID)
	s.exec(`INSERT INTO task_equipment (task_id, equipment_id) VALUES ($1, $2)`, taskID, unit)

	s.Require().NoError(s.Pool.QueryRow(ctx, `
		INSERT INTO transfers (from_warehouse_id, to_warehouse_id, status, created_by, sent_by, sent_at)
		VALUES ($1, $2, 'in_transit', $3, $3, now()) RETURNING id`, almaty, moscow, tpmAdminID).Scan(&oldTransit))
	s.exec(`INSERT INTO transfer_items (transfer_id, kind, product_id, position, qty_sent) VALUES ($1, 'product', $2, 1, 4)`, oldTransit, productID)
	s.Require().NoError(s.Pool.QueryRow(ctx, `
		INSERT INTO transfers (from_warehouse_id, to_warehouse_id, status, created_by) VALUES ($1, $2, 'received', $3) RETURNING id`,
		almaty, moscow, tpmAdminID).Scan(&oldReceived))
	s.exec(`INSERT INTO transfer_items (transfer_id, kind, product_id, position, qty_sent, qty_received) VALUES ($1, 'product', $2, 1, 5, 5)`, oldReceived, productID)
	s.True(s.constraint("transfers_distinct_warehouses"))
	s.False(s.column("transfers", "direction"))
	s.fails("0014: факт ≤ отправленного", `UPDATE transfer_items SET qty_received = 6 WHERE transfer_id = $1`, oldReceived)
	s.fails("0014: причина supplier_in неизвестна",
		`INSERT INTO stock_movements (product_id, warehouse_id, qty_delta, reason, user_id) VALUES ($1, $2, 1, 'supplier_in', $3)`, productID, almaty, tpmAdminID)

	// ---- up → 0015 ----
	s.Require().NoError(mig.Migrate(15))

	for _, col := range []string{"direction", "task_id", "client_id", "supplier_id"} {
		s.True(s.column("transfers", col), col)
	}
	s.Equal("2", s.scalar(`SELECT count(*)::text FROM transfers WHERE id = ANY($1) AND direction = 'warehouse_to_warehouse'
		AND task_id IS NULL AND client_id IS NULL AND supplier_id IS NULL`, []int64{oldTransit, oldReceived}),
		"существующие документы → склад → склад")
	s.Equal("YES|YES|NO", s.scalar(`
		SELECT string_agg(is_nullable, '|' ORDER BY column_name DESC) FROM information_schema.columns
		WHERE table_name = 'transfers' AND column_name IN ('to_warehouse_id', 'from_warehouse_id', 'direction')`),
		"склады необязательны, направление обязательно")
	s.False(s.exists(`SELECT column_default IS NOT NULL FROM information_schema.columns WHERE table_name = 'transfers' AND column_name = 'direction'`),
		"у direction нет DEFAULT")
	for _, idx := range []string{"transfers_direction_idx", "transfers_task_id_idx", "transfers_client_id_idx", "transfers_supplier_id_idx",
		"transfers_created_at_idx", "transfers_status_idx", "transfers_from_warehouse_id_idx", "transfers_to_warehouse_id_idx"} {
		s.True(s.exists(`SELECT EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = $1)`, idx), idx)
	}
	s.True(s.constraint("transfers_route_check"))
	s.False(s.constraint("transfers_distinct_warehouses"), "заменён transfers_route_check")
	for _, fk := range []string{"transfers_task_id_fkey", "transfers_client_id_fkey", "transfers_supplier_id_fkey"} {
		s.Equal("r", s.scalar(`SELECT confdeltype::text FROM pg_constraint WHERE conname = $1`, fk), fk+" — RESTRICT")
	}

	// CHECK маршрута: допустимый набор точек у каждого направления.
	insert := `INSERT INTO transfers (direction, from_warehouse_id, to_warehouse_id, task_id, client_id, supplier_id, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`
	type route struct {
		name                         string
		direction                    string
		from, to, task, client, supp any
	}
	for _, r := range []route{
		{"без direction", "", almaty, moscow, nil, nil, nil},
		{"неизвестное направление", "warehouse_to_supplier", almaty, moscow, nil, nil, nil},
		{"w2w: один склад", "warehouse_to_warehouse", almaty, almaty, nil, nil, nil},
		{"w2w: без склада назначения", "warehouse_to_warehouse", almaty, nil, nil, nil, nil},
		{"w2w: с заказом", "warehouse_to_warehouse", almaty, moscow, taskID, clientID, nil},
		{"w2w: с поставщиком", "warehouse_to_warehouse", almaty, moscow, nil, nil, supplierID},
		{"c2w: без заказа", "client_to_warehouse", nil, almaty, nil, clientID, nil},
		{"c2w: без клиента", "client_to_warehouse", nil, almaty, taskID, nil, nil},
		{"c2w: со складом отправления", "client_to_warehouse", moscow, almaty, taskID, clientID, nil},
		{"c2w: без склада назначения", "client_to_warehouse", nil, nil, taskID, clientID, nil},
		{"c2w: с поставщиком", "client_to_warehouse", nil, almaty, taskID, clientID, supplierID},
		{"w2c: со складом назначения", "warehouse_to_client", almaty, moscow, taskID, clientID, nil},
		{"w2c: без склада отправления", "warehouse_to_client", nil, nil, taskID, clientID, nil},
		{"w2c: без заказа", "warehouse_to_client", almaty, nil, nil, nil, nil},
		{"s2w: без поставщика", "supplier_to_warehouse", nil, almaty, nil, nil, nil},
		{"s2w: с заказом", "supplier_to_warehouse", nil, almaty, taskID, clientID, supplierID},
		{"s2w: со складом отправления", "supplier_to_warehouse", moscow, almaty, nil, nil, supplierID},
		{"s2w: без склада назначения", "supplier_to_warehouse", nil, nil, nil, nil, supplierID},
	} {
		var direction any = r.direction
		if r.direction == "" {
			direction = nil
		}
		s.fails(r.name, insert, direction, r.from, r.to, r.task, r.client, r.supp, tpmAdminID)
	}
	newDoc := func(direction string, from, to, task, client, supp any) int64 {
		var id int64
		s.Require().NoError(s.Pool.QueryRow(ctx, insert+` RETURNING id`, direction, from, to, task, client, supp, tpmAdminID).Scan(&id), direction)
		return id
	}
	w2w := newDoc("warehouse_to_warehouse", almaty, moscow, nil, nil, nil)
	intake := newDoc("client_to_warehouse", nil, almaty, taskID, clientID, nil)
	back := newDoc("warehouse_to_client", almaty, nil, taskID, clientID, nil)
	supply := newDoc("supplier_to_warehouse", nil, almaty, nil, nil, supplierID)

	// FK RESTRICT: заказ, клиент и поставщик с документом не удаляются.
	s.fails("заказ с документом", `DELETE FROM tasks WHERE id = $1`, taskID)
	s.fails("поставщик с документом", `DELETE FROM suppliers WHERE id = $1`, supplierID)
	s.True(s.exists(`SELECT EXISTS (SELECT 1 FROM tasks WHERE id = $1)`, taskID))

	// DS-015: факт может превышать количество; отрицательный — по-прежнему нельзя.
	s.exec(`INSERT INTO transfer_items (transfer_id, kind, product_id, position, qty_sent, qty_received) VALUES ($1, 'product', $2, 1, 10, 12)`, supply, productID)
	s.fails("отрицательный факт", `UPDATE transfer_items SET qty_received = -1 WHERE transfer_id = $1`, supply)
	s.exec(`UPDATE transfer_items SET qty_received = 7 WHERE transfer_id = $1`, oldReceived) // для проверки отката
	s.True(s.constraint("transfer_items_qty_received_check"))

	// DS-040: три новые причины движений — сразу; чужая причина по-прежнему отклоняется.
	for _, reason := range []string{"supplier_in", "client_out", "transfer_unreceive"} {
		s.exec(`INSERT INTO stock_movements (product_id, warehouse_id, qty_delta, reason, user_id, transfer_id) VALUES ($1, $2, 12, $3, $4, $5)`,
			productID, almaty, reason, tpmAdminID, supply)
	}
	for _, reason := range []string{"task_consume", "manual", "transfer_out", "transfer_in", "transfer_loss", "transfer_return"} {
		s.exec(`INSERT INTO stock_movements (product_id, warehouse_id, qty_delta, reason, user_id) VALUES ($1, $2, 1, $3, $4)`,
			productID, almaty, reason, tpmAdminID)
	}
	s.fails("неизвестная причина",
		`INSERT INTO stock_movements (product_id, warehouse_id, qty_delta, reason, user_id) VALUES ($1, $2, 1, 'supplier_out', $3)`, productID, almaty, tpmAdminID)

	// единица в пути по документу с клиентом — для проверки отката.
	s.exec(`INSERT INTO transfer_items (transfer_id, kind, equipment_id, position) VALUES ($1, 'equipment', $2, 1)`, intake, unit)
	s.exec(`UPDATE transfers SET status = 'in_transit' WHERE id = $1`, intake)
	s.exec(`UPDATE equipment SET location_status = 'in_transit', active_transfer_id = $2 WHERE id = $1`, unit, intake)

	// ---- down → 0014 ----
	s.Require().NoError(mig.Migrate(14))
	for _, col := range []string{"direction", "task_id", "client_id", "supplier_id"} {
		s.False(s.column("transfers", col), col)
	}
	s.True(s.constraint("transfers_distinct_warehouses"))
	s.False(s.constraint("transfers_route_check"))
	s.Equal("NO|NO", s.scalar(`
		SELECT string_agg(is_nullable, '|') FROM information_schema.columns
		WHERE table_name = 'transfers' AND column_name IN ('to_warehouse_id', 'from_warehouse_id')`), "склады снова обязательны")
	s.Equal("3", s.scalar(`SELECT count(*)::text FROM transfers WHERE id = ANY($1)`, []int64{oldTransit, oldReceived, w2w}), "склад → склад сохранены")
	s.Equal("0", s.scalar(`SELECT count(*)::text FROM transfers WHERE id = ANY($1)`, []int64{intake, back, supply}), "документы новых направлений удалены")
	s.Equal("at_client|", s.scalar(`SELECT location_status || '|' || coalesce(active_transfer_id::text, '') FROM equipment WHERE id = $1`, unit),
		"единица удалённого документа освобождена и «у клиента»")
	s.Equal("5.000", s.scalar(`SELECT qty_received::text FROM transfer_items WHERE transfer_id = $1`, oldReceived), "факт снова ≤ отправленного")
	s.fails("0014: факт ≤ отправленного", `UPDATE transfer_items SET qty_received = 6 WHERE transfer_id = $1`, oldReceived)
	s.Equal("0", s.scalar(`SELECT count(*)::text FROM stock_movements WHERE reason IN ('supplier_in', 'client_out', 'transfer_unreceive')`))
	s.Equal("3", s.scalar(`SELECT count(*)::text FROM stock_movements WHERE product_id = $1 AND qty_delta = 12 AND reason = 'manual' AND transfer_id IS NULL`, productID),
		"движения новых причин сохранены как manual, ссылка на удалённый документ снята")
	s.fails("0014: причина supplier_in неизвестна",
		`INSERT INTO stock_movements (product_id, warehouse_id, qty_delta, reason, user_id) VALUES ($1, $2, 1, 'supplier_in', $3)`, productID, almaty, tpmAdminID)

	// повторный up поверх отката проходит (идемпотентность пары up/down).
	s.Require().NoError(mig.Migrate(15))
	s.Equal("3", s.scalar(`SELECT count(*)::text FROM transfers WHERE id = ANY($1) AND direction = 'warehouse_to_warehouse'`, []int64{oldTransit, oldReceived, w2w}))
}
