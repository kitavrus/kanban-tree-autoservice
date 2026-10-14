// Package seeddemo — «make seed-demo»: заполняет пустую БД демо-данными сервисного
// центра лазерного оборудования (D36 — пользователи-исполнители, клиенты из РФ и СНГ,
// типы оборудования и оборудование для лазерной резки/сварки/очистки/маркировки, каталог
// категорий/товаров/услуг с остатками, заказы во всех 5 колонках доски с услугами в
// разных статусах и списаниями через StockEngine.Reconcile) одной командой (AC-019,
// ADR-105). Guard — count(tasks)=0, иначе no-op с сообщением (SD-003); нужен активный
// администратор (role='admin' AND is_active) — иначе ErrNoAdmin. Существующий admin не
// изменяется, демо-исполнители заводятся ON CONFLICT (login) DO NOTHING.
//
// Бренды — популярные в России и СНГ: станки Bodor, HSG, Han's Laser, Senfeng, Golden
// Laser, LXShow; источники IPG Photonics, Raycus, MAX Photonics, JPT; головки Raytools,
// Precitec, WSX, Qilin; сварка/очистка Qilin, Relfar; чиллеры S&A (Teyu); ЧПУ Friendess
// (CypCut/FSCUT). Компании-клиенты, контакты, серийные номера и цены — вымышленные.
//
// Заказы получают оборудование клиента (task_equipment, опора услуг — subtasks.
// equipment_id) и журнал работ по услугам (subtask_logs). Не заполняются вложения заказа;
// task_stock заполняет StockEngine.Reconcile, а не seed напрямую.
//
// Склады (warehouses DS-012, D60): остатки товаров с остатком ≥20 раскиданы по «Алматы» и
// «Москве» (¼ — в Москве); два документа «Перемещение» Алматы → Москва (один получен, один
// в пути — в нём, кроме товаров, строка-единица оборудования клиента). Местоположение — на
// самой единице (transfers-core DS-008/DS-054) и меняется только документами
// (transfers-partners DS-013, DS-045): оборудование незавершённых заказов принято на склад
// документом «клиент → склад» (у трёх заказов — в Москве); оборудование заказов последней
// колонки доски — снова «у клиента» через проведённую пару «приём + возврат»; одна единица —
// «в пути» между складами, одна — «в пути → клиент» (возврат отправлен, не принят). Журнал
// task_equipment_movements не ведётся (DS-029).
//
// Поставщики (transfers-partners TS-F1): два — действующий и архивный; два документа «приход
// от поставщика» на «Алматы»: полученный с расхождением (движение supplier_in на факт) и в
// пути (движений нет, DS-040).
//
// Накладные клиенту (transfers-sales TS-F1, DS-011/DS-014): в двух документах «склад → клиент»
// — товар с ценой продажи: полученная накладная в оплаченном заказе и накладная «в пути» в
// неоплаченном; товар списан со склада отправления движением client_out, сумма накладной
// входит в итог заказа.
//
// Выплаты исполнителям (executor-payouts, D71): часть выполненных услуг отмечена
// выплаченной с суммой выплаты, часть — с суммой, но без отметки, часть — без суммы.
package seeddemo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/kitavrus/kanban-tree/internal/features/board"
)

// ErrNoAdmin — в БД нет ни одного активного администратора; демо-данным нужен его id
// (created_by заказов, user_id движений склада). Типовое лечение — make local-up-seed
// (создаёт seed-админа) ДО make seed-demo.
var ErrNoAdmin = errors.New("нет активного администратора — сначала выполните make local-up-seed")

// demoPassword — пароль демо-исполнителей (аналог cmd/seed: секрет не для прод, только
// локальный/демо-стенд).
const demoPassword = "demo12345"

// Result — сводка по демо-данным (AC-019). Skipped=true — в БД уже были заказы, ничего
// не создано (идемпотентность повторного запуска, SD-003).
type Result struct {
	Skipped   bool
	Message   string
	Executors int
	Clients   int
	Equipment int
	Products  int
	Services  int
	Tasks     int
}

// seedDemoLockKey — произвольный bigint для pg_advisory_xact_lock (LOW-3, code review):
// сериализует конкурентные «make seed-demo», чтобы guard count(tasks)=0 выполнялся в ТОЙ
// ЖЕ транзакции, что и запись демо-данных, а не отдельным запросом до BEGIN (TOCTOU).
const seedDemoLockKey = 8_721_394_651

// Run — точка входа cmd/seed-demo. Guard-порядок — design-sql.md §Seed Demo Guard Query:
// сперва count(tasks), затем активный admin; оба — под advisory-локом внутри одной
// транзакции записи (Lock Order columns→tasks→products соблюдает StockEngine.Reconcile,
// ADR-104).
func Run(ctx context.Context, pool *pgxpool.Pool) (Result, error) {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Result{}, fmt.Errorf("seeddemo: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, int64(seedDemoLockKey)); err != nil {
		return Result{}, fmt.Errorf("seeddemo: advisory lock: %w", err)
	}

	var tasksCount int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM tasks`).Scan(&tasksCount); err != nil {
		return Result{}, fmt.Errorf("seeddemo: count tasks: %w", err)
	}
	if tasksCount > 0 {
		return Result{Skipped: true, Message: "заказы уже есть — демо-данные не создаются"}, nil
	}

	var adminID string
	err = tx.QueryRow(ctx,
		`SELECT id FROM users WHERE role = 'admin' AND is_active ORDER BY created_at LIMIT 1`).
		Scan(&adminID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Result{}, ErrNoAdmin
	}
	if err != nil {
		return Result{}, fmt.Errorf("seeddemo: lookup admin: %w", err)
	}

	sd := &seeder{ctx: ctx, tx: tx, adminID: adminID, engine: board.NewStockEngine()}
	res, err := sd.run()
	if err != nil {
		return Result{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Result{}, fmt.Errorf("seeddemo: commit: %w", err)
	}
	res.Message = "демо-данные созданы"
	return res, nil
}

// seeder — состояние одного прогона внутри транзакции.
type seeder struct {
	ctx     context.Context
	tx      pgx.Tx
	adminID string
	engine  *board.StockEngine
	// almatyID/moscowID — склады миграционного сида 0008 (DS-012).
	almatyID, moscowID string
	// transitTransferID — документ «Перемещение» в пути Алматы → Москва (seedTransfers);
	// в него встаёт строкой единица оборудования заказа demoTransitTask.
	transitTransferID int64
	// supplierID — действующий демо-поставщик (seedSuppliers) документов «поставщик → склад».
	supplierID string
	// lastColumnID — последняя колонка доски (наибольший position): заказы в ней —
	// завершённые, их оборудование остаётся «у клиента» (transfers-core DS-054).
	lastColumnID string
}

func (s *seeder) run() (Result, error) {
	var res Result

	executors, err := s.seedExecutors()
	if err != nil {
		return Result{}, fmt.Errorf("seeddemo: executors: %w", err)
	}
	res.Executors = len(executors)

	clients, err := s.seedClients()
	if err != nil {
		return Result{}, fmt.Errorf("seeddemo: clients: %w", err)
	}
	res.Clients = len(clients)

	equipIDs, err := s.seedEquipment(clients)
	if err != nil {
		return Result{}, fmt.Errorf("seeddemo: equipment: %w", err)
	}
	res.Equipment = len(equipIDs)

	categoryIDs, err := s.seedCategories()
	if err != nil {
		return Result{}, fmt.Errorf("seeddemo: categories: %w", err)
	}

	products, err := s.seedProducts(categoryIDs)
	if err != nil {
		return Result{}, fmt.Errorf("seeddemo: products: %w", err)
	}
	res.Products = len(products)

	if err := s.seedSuppliers(); err != nil {
		return Result{}, fmt.Errorf("seeddemo: suppliers: %w", err)
	}

	if err := s.seedTransfers(products); err != nil {
		return Result{}, fmt.Errorf("seeddemo: transfers: %w", err)
	}

	services, err := s.seedServices(categoryIDs, products)
	if err != nil {
		return Result{}, fmt.Errorf("seeddemo: services: %w", err)
	}
	res.Services = len(services)

	columns, err := s.loadColumns()
	if err != nil {
		return Result{}, fmt.Errorf("seeddemo: load columns: %w", err)
	}

	tasksCreated, err := s.seedTasks(columns, clients, executors, equipIDs, services, products)
	if err != nil {
		return Result{}, fmt.Errorf("seeddemo: tasks: %w", err)
	}
	res.Tasks = tasksCreated

	if err := s.seedInvoices(products); err != nil {
		return Result{}, fmt.Errorf("seeddemo: invoices: %w", err)
	}

	if err := s.seedDeletedTask(clients); err != nil {
		return Result{}, fmt.Errorf("seeddemo: deleted task: %w", err)
	}

	if err := s.seedDeletedColumn(); err != nil {
		return Result{}, fmt.Errorf("seeddemo: deleted column: %w", err)
	}

	if err := s.seedDeletedClient(clients); err != nil {
		return Result{}, fmt.Errorf("seeddemo: deleted client: %w", err)
	}

	if err := s.seedDeletedCatalog(); err != nil {
		return Result{}, fmt.Errorf("seeddemo: deleted catalog: %w", err)
	}

	if err := s.seedDeletedWarehouseSupplier(); err != nil {
		return Result{}, fmt.Errorf("seeddemo: deleted warehouse and supplier: %w", err)
	}

	return res, nil
}

// seedDeletedTask — один мягко удалённый заказ с причиной (soft-delete-core TS-F5, DS-011):
// раздел «Удалённые» на демо-стенде непустой. Заказ без услуг и товаров — возвращать на склад
// нечего (инвариант «удалённый заказ не держит товар» выполняется); в Result.Tasks не входит —
// там действующие заказы доски.
func (s *seeder) seedDeletedTask(clients []clientRow) error {
	_, err := s.tx.Exec(s.ctx, `
		INSERT INTO tasks (column_id, client_id, title, description_html, position, created_by,
		                   deleted_at, deleted_by, delete_reason)
		SELECT c.id, $1::uuid, 'Диагностика кофемашины (дубль)', '', 999424, $2::uuid,
		       now() - interval '2 days', $2::uuid, 'Создан по ошибке — дубль заказа'
		FROM columns c WHERE NOT c.is_final AND c.deleted_at IS NULL ORDER BY c.position, c.id LIMIT 1`,
		clients[0].ID, s.adminID)
	return err
}

// demoDeletedColumnName — название мягко удалённой демо-колонки.
const demoDeletedColumnName = "На проверке"

// seedDeletedColumn — одна мягко удалённая колонка доски с причиной (soft-delete-board-columns,
// DS-011): вкладка «Колонки» раздела «Удалённые» на демо-стенде непустая. Колонка пустая и
// нефинальная; «кто, когда, почему» заполнены вместе (CHECK columns_deleted_chk); позиция —
// за последней действующей (у удалённой колонки не используется). Колонки демо-сидом не
// очищаются (их сеет миграция 0005) — повторный запуск вторую удалённую колонку не создаёт.
func (s *seeder) seedDeletedColumn() error {
	_, err := s.tx.Exec(s.ctx, `
		INSERT INTO columns (name, color, is_final, consumes_stock, position, deleted_at, deleted_by, delete_reason)
		SELECT $1, '#6c757d', false, false,
		       (SELECT coalesce(max(position), 0) + 1 FROM columns WHERE deleted_at IS NULL),
		       now() - interval '1 day', $2::uuid, 'Этап объединён с «Согласованием»'
		WHERE NOT EXISTS (SELECT 1 FROM columns WHERE name = $1 AND deleted_at IS NOT NULL)`,
		demoDeletedColumnName, s.adminID)
	return err
}

// demoDeletedClientName — название мягко удалённого демо-клиента.
const demoDeletedClientName = "ИП Сергеев А. В. (закрыт)"

// seedDeletedClient — мягко удалённые клиент и единицы оборудования (soft-delete-clients,
// DS-011, DS-015): вкладки «Клиенты» и «Оборудование» раздела «Удалённые» на демо-стенде
// непустые. Клиент удалён вместе со своей единицей (те же «кто, когда, почему»,
// deleted_with_client = true); ещё одна единица действующего клиента удалена отдельно. Обе
// единицы у клиента и не стоят в документах (DS-041); заказов и документов у них нет. В
// Result.Clients / Result.Equipment не входят — там действующие записи.
func (s *seeder) seedDeletedClient(clients []clientRow) error {
	const reason = "Клиент закрыл бизнес"
	var clientID string
	var at time.Time
	if err := s.tx.QueryRow(s.ctx, `
		INSERT INTO clients (company_name, address, contact_name, contact_position, deleted_at, deleted_by, delete_reason)
		VALUES ($1, 'Россия, г. Казань, ул. Баумана, 7', 'Сергеев Андрей', 'Владелец',
		        now() - interval '3 days', $2::uuid, $3)
		RETURNING id, deleted_at`, demoDeletedClientName, s.adminID, reason).Scan(&clientID, &at); err != nil {
		return err
	}
	if _, err := s.tx.Exec(s.ctx,
		`INSERT INTO client_phones (client_id, position, phone, note) VALUES ($1, 0, '+7 843 200-10-20', 'офис')`, clientID); err != nil {
		return err
	}
	if _, err := s.tx.Exec(s.ctx, `
		INSERT INTO equipment (client_id, type_id, brand, model, sku, serial, deleted_at, deleted_by, delete_reason, deleted_with_client)
		SELECT $1::uuid, t.id, 'Raycus', 'RFL-C1000', 'RFL-C1000', 'RC-22-1K-0044', $2, $3::uuid, $4, true
		FROM equipment_types t ORDER BY t.name, t.id LIMIT 1`, clientID, at, s.adminID, reason); err != nil {
		return err
	}
	_, err := s.tx.Exec(s.ctx, `
		INSERT INTO equipment (client_id, type_id, brand, model, sku, serial, deleted_at, deleted_by, delete_reason)
		SELECT $1::uuid, t.id, 'S&A', 'CW-5200', 'CW-5200', 'SA-21-5200-0311',
		       now() - interval '5 days', $2::uuid, 'Списан клиентом'
		FROM equipment_types t ORDER BY t.name, t.id LIMIT 1`, clients[0].ID, s.adminID)
	return err
}

// Названия мягко удалённых демо-записей каталога и типа оборудования.
const (
	demoDeletedCategoryName = "Снятые с производства"
	demoDeletedProductName  = "Линза фокусирующая D20 F100 (снята с продажи)"
	demoDeletedServiceName  = "Выезд в другой город"
	demoDeletedTypeName     = "Плоттер режущий"
)

// seedDeletedCatalog — по одной мягко удалённой записи каждого вида каталога (soft-delete-catalog,
// DS-011, DS-009): вкладки «Товары», «Услуги», «Категории», «Типы оборудования» раздела
// «Удалённые» на демо-стенде непустые. Товар удалён из архива (is_active = false — бейдж «в
// архиве») и остатка не имеет (строк product_stock нет, DS-021); услуга и категория — активные;
// на удалённые категорию и тип никто не ссылается. «Кто, когда, почему» заполнены вместе
// (CHECK {table}_deleted_chk). В Result.Products / Result.Services не входят — там
// действующие записи.
func (s *seeder) seedDeletedCatalog() error {
	if _, err := s.tx.Exec(s.ctx, `
		INSERT INTO catalog_categories (kind, name, deleted_at, deleted_by, delete_reason)
		VALUES ('product', $1, now() - interval '6 days', $2::uuid, 'Товары категории больше не закупаем')`,
		demoDeletedCategoryName, s.adminID); err != nil {
		return err
	}
	if _, err := s.tx.Exec(s.ctx, `
		INSERT INTO products (category_id, name, sku, unit, price, is_active, deleted_at, deleted_by, delete_reason)
		SELECT c.id, $1, 'LNS-D20-F100', 'шт', 4200, false, now() - interval '2 days', $2::uuid, 'Снята с продажи поставщиком'
		FROM catalog_categories c WHERE c.kind = 'product' AND c.deleted_at IS NULL ORDER BY lower(c.name), c.id LIMIT 1`,
		demoDeletedProductName, s.adminID); err != nil {
		return err
	}
	if _, err := s.tx.Exec(s.ctx, `
		INSERT INTO services (category_id, name, price, deleted_at, deleted_by, delete_reason)
		SELECT c.id, $1, 15000, now() - interval '4 days', $2::uuid, 'Услугу больше не оказываем'
		FROM catalog_categories c WHERE c.kind = 'service' AND c.deleted_at IS NULL ORDER BY lower(c.name), c.id LIMIT 1`,
		demoDeletedServiceName, s.adminID); err != nil {
		return err
	}
	_, err := s.tx.Exec(s.ctx, `
		INSERT INTO equipment_types (name, spec_keys, deleted_at, deleted_by, delete_reason)
		VALUES ($1, '{"Ширина реза"}', now() - interval '1 day', $2::uuid, 'Тип заведён по ошибке')`,
		demoDeletedTypeName, s.adminID)
	return err
}

// Названия мягко удалённых демо-склада и демо-поставщика.
const (
	demoDeletedWarehouseName = "Астана (закрыт)"
	demoDeletedSupplierName  = "ТехИмпорт"
)

// seedDeletedWarehouseSupplier — по одной мягко удалённой записи видов «склад» и «поставщик»
// (soft-delete-warehouses-suppliers, DS-011): вкладки «Склады» и «Поставщики» раздела
// «Удалённые» на демо-стенде непустые. Склад удалён пустым (DS-021: без остатков, оборудования и
// документов) из архива (is_active = false — бейдж «в архиве»), позиция — за последним складом;
// поставщик удалён активным, документов у него нет. «Кто, когда, почему» заполнены вместе (CHECK
// {table}_deleted_chk). Склады и поставщики демо-сидом не очищаются — повторный запуск вторую
// удалённую запись не создаёт.
func (s *seeder) seedDeletedWarehouseSupplier() error {
	if _, err := s.tx.Exec(s.ctx, `
		INSERT INTO warehouses (name, position, is_active, deleted_at, deleted_by, delete_reason)
		SELECT $1, (SELECT coalesce(max(position), 0) + 1 FROM warehouses), false,
		       now() - interval '5 days', $2::uuid, 'Склад закрыт, остатки вывезены'
		WHERE NOT EXISTS (SELECT 1 FROM warehouses WHERE name = $1 AND deleted_at IS NOT NULL)`,
		demoDeletedWarehouseName, s.adminID); err != nil {
		return err
	}
	_, err := s.tx.Exec(s.ctx, `
		INSERT INTO suppliers (name, contact_name, phone, note, deleted_at, deleted_by, delete_reason)
		SELECT $1, 'Ким Олег', '+7 717 250-40-40', 'Разовая закупка',
		       now() - interval '3 days', $2::uuid, 'Заведён дублем — работаем через «ЛазерКомплект»'
		WHERE NOT EXISTS (SELECT 1 FROM suppliers WHERE name = $1 AND deleted_at IS NOT NULL)`,
		demoDeletedSupplierName, s.adminID)
	return err
}

// clientRow/productRow/serviceRow/columnRow — минимум полей, нужных для построения
// демо-заказов ниже.
type clientRow struct{ ID string }
type productRow struct{ ID string }
type serviceRow struct {
	ID    string
	Price string      // "" — NULL («по факту»); эталон demoServiceItem.price
	Goods []protoGood // комплект товаров услуги по умолчанию (для subtask_products)
}
type columnRow struct {
	ID            string
	IsFinal       bool
	ConsumesStock bool
}

// ---------------------------------------------------------------------------------------
// Демо-данные: сервисный центр лазерного оборудования (D36). Связи между массивами —
// через 0-based индексы (clientIdx, typeIdx, categoryIdx, productIdx, serviceIdx,
// assigneeIdx); индекс указан в комментарии строки.
// ---------------------------------------------------------------------------------------

// demoExecutor — исполнители-инженеры (admin не создаётся: существующий не трогается —
// его пояс остаётся умолчанием Asia/Almaty). timezone пуст — пояс по умолчанию; один
// исполнитель — в Москве (user-timezone, DS-033: демо показывает оба пояса).
type demoExecutor struct{ login, name, phone, timezone string }

var demoExecutors = []demoExecutor{
	{login: "sidorov", name: "Сидоров Олег", phone: "+7 916 400-00-01"},                           // 0 — источники, электроника
	{login: "volkov", name: "Волков Павел", phone: "+7 916 400-00-02", timezone: "Europe/Moscow"}, // 1 — ТО, чиллеры, очистка
	{login: "egorova", name: "Егорова Ирина", phone: "+7 916 400-00-03"},                          // 2 — оптика, головки
	{login: "kim", name: "Ким Артём", phone: "+7 916 400-00-04"},                                  // 3 — сварка, технолог резки
}

type executorRow struct{ ID string }

// seedExecutors — ON CONFLICT (login) DO NOTHING (users_login_lower_uq): существующий
// admin/исполнитель не трогается, повторный прогон безопасен.
func (s *seeder) seedExecutors() ([]executorRow, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(demoPassword), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("bcrypt: %w", err)
	}

	out := make([]executorRow, 0, len(demoExecutors))
	for _, e := range demoExecutors {
		if _, err := s.tx.Exec(s.ctx,
			`INSERT INTO users (login, password_hash, name, role, phone, timezone)
			 VALUES ($1, $2, $3, 'executor', $4, COALESCE(NULLIF($5, ''), 'Asia/Almaty'))
			 ON CONFLICT (login) DO NOTHING`,
			e.login, string(hash), e.name, e.phone, e.timezone); err != nil {
			return nil, err
		}
		var id string
		if err := s.tx.QueryRow(s.ctx, `SELECT id FROM users WHERE login = $1`, e.login).Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, executorRow{ID: id})
	}
	return out, nil
}

// demoPhone — телефон клиента.
type demoPhone struct{ phone, note string }

// demoClient — клиенты: металлообработка, реклама, реставрация — РФ, Казахстан,
// Беларусь, Узбекистан (названия вымышленные).
type demoClient struct {
	company, address, contact, position string
	phones                              []demoPhone
}

var demoClients = []demoClient{
	{company: "ООО «МеталлКрафт»", address: "г. Москва, Электродная ул., 2, стр. 12", contact: "Гусев Андрей Викторович", position: "Главный механик",
		phones: []demoPhone{{"+7 916 123-45-67", "мобильный"}, {"+7 495 111-22-33", "офис"}}}, // 0
	{company: "ООО «Невский лазер»", address: "г. Санкт-Петербург, ул. Салова, 57", contact: "Лебедева Мария Андреевна", position: "Технический директор",
		phones: []demoPhone{{"+7 812 300-40-50", "офис"}}}, // 1
	{company: "АО «УралСтальКонструкция»", address: "г. Екатеринбург, ул. Монтёров, 3", contact: "Никитин Сергей Павлович", position: "Начальник участка резки",
		phones: []demoPhone{{"+7 343 222-10-15", "цех"}, {"+7 912 600-70-80", "WhatsApp"}}}, // 2
	{company: "ИП Гаврилов Р. А. (рекламная мастерская)", address: "г. Казань, ул. Техническая, 25", contact: "Гаврилов Руслан", position: "Владелец",
		phones: []demoPhone{{"+7 987 222-33-44", "WhatsApp"}}}, // 3
	{company: "ООО «Реставрация-Юг»", address: "г. Краснодар, ул. Уральская, 99", contact: "Фёдоров Алексей", position: "Директор",
		phones: []demoPhone{{"+7 918 123-00-00", ""}}}, // 4
	{company: "ООО «ВолгаСварка»", address: "г. Нижний Новгород, ул. Ларина, 22", contact: "Орлов Игорь", position: "Мастер сварочного участка",
		phones: []demoPhone{{"+7 910 777-88-99", ""}}}, // 5
	{company: "ООО «СибМеталлПрофиль»", address: "г. Новосибирск, ул. Станционная, 60", contact: "Попова Елена", position: "Инженер по ТО",
		phones: []demoPhone{{"+7 383 255-60-70", "после 10:00 мск+4"}}}, // 6
	{company: "ТОО «Astana Metal Group»", address: "Казахстан, г. Астана, ул. Жангельдина, 29/1", contact: "Ахметов Нурлан", position: "Главный инженер",
		phones: []demoPhone{{"+7 701 555-66-77", "WhatsApp"}, {"+7 7172 50-60-70", "офис"}}}, // 7
	{company: "ООО «БелЛазерТех»", address: "Беларусь, г. Минск, ул. Тимирязева, 65", contact: "Ковальчук Дмитрий", position: "Сервисный инженер",
		phones: []demoPhone{{"+375 29 612-34-56", "Viber"}}}, // 8
	{company: "ООО «Узмет Сервис»", address: "Узбекистан, г. Ташкент, ул. Бунёдкор, 12", contact: "Каримов Бахтиёр", position: "Директор",
		phones: []demoPhone{{"+998 90 123-45-67", "Telegram"}}}, // 9
}

func (s *seeder) seedClients() ([]clientRow, error) {
	out := make([]clientRow, 0, len(demoClients))
	for _, c := range demoClients {
		var id string
		if err := s.tx.QueryRow(s.ctx,
			`INSERT INTO clients (company_name, address, contact_name, contact_position)
			 VALUES ($1, $2, $3, $4) RETURNING id`,
			c.company, c.address, c.contact, c.position).Scan(&id); err != nil {
			return nil, err
		}
		for i, p := range c.phones {
			if _, err := s.tx.Exec(s.ctx,
				`INSERT INTO client_phones (client_id, position, phone, note) VALUES ($1, $2, $3, $4)`,
				id, i+1, p.phone, p.note); err != nil {
				return nil, err
			}
		}
		out = append(out, clientRow{ID: id})
	}
	return out, nil
}

// demoEquipmentType — типы оборудования (keys → spec_keys).
type demoEquipmentType struct {
	name string
	keys []string
}

var demoEquipmentTypes = []demoEquipmentType{
	{name: "Станок лазерной резки", keys: []string{"Рабочее поле", "Мощность источника", "Лазерный источник", "Режущая головка", "ЧПУ", "Год выпуска"}}, // 0
	{name: "Лазерный источник", keys: []string{"Мощность", "Длина волны", "Режим", "Длина кабеля", "Разъём"}},                                           // 1
	{name: "Аппарат ручной лазерной сварки", keys: []string{"Мощность", "Лазерный источник", "Сварочная головка", "Подача проволоки", "Напряжение"}},    // 2
	{name: "Аппарат лазерной очистки", keys: []string{"Мощность", "Режим", "Лазерный источник", "Ширина сканирования"}},                                 // 3
	{name: "Режущая / сварочная головка", keys: []string{"Разъём", "Коллиматор", "Фокус", "Защитное стекло"}},                                           // 4
	{name: "Чиллер", keys: []string{"Для лазера", "Контуры", "Хладагент", "Напряжение"}},                                                                // 5
	{name: "ЧПУ / контроллер", keys: []string{"Система", "Версия ПО", "Датчик высоты"}},                                                                 // 6
	{name: "Лазерный маркер", keys: []string{"Мощность", "Поле маркировки", "Лазерный источник"}},                                                       // 7
	{name: "Кабель / оптоволокно", keys: []string{"Длина", "Разъём", "Диаметр жилы"}},                                                                   // 8
	{name: "Другое", keys: []string{}}, // 9
}

// specJSON — jsonb-элемент specs {"k":...,"v":...} (эталон clients.SpecDTO: json-теги k/v).
type specJSON struct {
	K string `json:"k"`
	V string `json:"v"`
}

// demoEquipmentSpec — prototype equipment[].specs[] {k,v}.
type demoEquipmentSpec = specJSON

// demoEquipmentItem — оборудование клиентов. clientIdx/typeIdx — индексы
// demoClients/demoEquipmentTypes. Серийные номера вымышленные.
type demoEquipmentItem struct {
	clientIdx, typeIdx              int
	brand, model, sku, serial, note string
	specs                           []demoEquipmentSpec
}

type sp = demoEquipmentSpec

var demoEquipment = []demoEquipmentItem{
	// 0 — ООО «МеталлКрафт»
	{clientIdx: 0, typeIdx: 0, brand: "Bodor", model: "A3", sku: "A3-1530", serial: "BD21-04417",
		specs: []sp{{"Рабочее поле", "1500×3000 мм"}, {"Мощность источника", "3 кВт"}, {"Лазерный источник", "Raycus RFL-C3000S"}, {"Режущая головка", "Raytools BM111"}, {"ЧПУ", "Friendess FSCUT2000"}, {"Год выпуска", "2021"}}},
	{clientIdx: 0, typeIdx: 1, brand: "Raycus", model: "RFL-C3000S", sku: "RFL-C3000S", serial: "RC2103A118", note: "Стоит в станке Bodor A3",
		specs: []sp{{"Мощность", "3000 Вт"}, {"Длина волны", "1080 нм"}, {"Режим", "непрерывный"}, {"Длина кабеля", "20 м"}, {"Разъём", "QBH"}}},
	{clientIdx: 0, typeIdx: 4, brand: "Raytools", model: "BM111", sku: "BM111", serial: "RT-BM111-20931",
		specs: []sp{{"Разъём", "QBH"}, {"Коллиматор", "F100"}, {"Фокус", "F125, автофокус"}, {"Защитное стекло", "27,9×4,1 мм"}}},
	{clientIdx: 0, typeIdx: 5, brand: "S&A (Teyu)", model: "CWFL-3000", sku: "CWFL-3000ANP", serial: "CW3K-21-0773",
		specs: []sp{{"Для лазера", "3 кВт"}, {"Контуры", "2 (источник + оптика)"}, {"Хладагент", "R-410A"}, {"Напряжение", "220 В"}}},
	// 1 — ООО «Невский лазер»
	{clientIdx: 1, typeIdx: 0, brand: "HSG", model: "G3015C", sku: "G3015C", serial: "HSG-22-G3015C-118",
		specs: []sp{{"Рабочее поле", "1500×3000 мм"}, {"Мощность источника", "6 кВт"}, {"Лазерный источник", "IPG YLS-6000"}, {"Режущая головка", "Precitec ProCutter 2.0"}, {"ЧПУ", "Friendess FSCUT4000"}, {"Год выпуска", "2022"}}},
	{clientIdx: 1, typeIdx: 1, brand: "IPG Photonics", model: "YLS-6000", sku: "YLS-6000-CUT", serial: "PLMR21030877", note: "Стоит в станке HSG G3015C",
		specs: []sp{{"Мощность", "6000 Вт"}, {"Длина волны", "1070 нм"}, {"Режим", "непрерывный"}, {"Длина кабеля", "20 м"}, {"Разъём", "LCA"}}},
	{clientIdx: 1, typeIdx: 4, brand: "Precitec", model: "ProCutter 2.0", sku: "PC2.0", serial: "PC2-0034561",
		specs: []sp{{"Разъём", "LCA"}, {"Коллиматор", "F100"}, {"Фокус", "F200, автофокус"}, {"Защитное стекло", "30×1,5 мм"}}},
	{clientIdx: 1, typeIdx: 7, brand: "JPT", model: "Маркер MOPA M7 30 Вт", sku: "YDFLP-30-M7", serial: "JPT-M7-30-5521",
		specs: []sp{{"Мощность", "30 Вт"}, {"Поле маркировки", "110×110 мм"}, {"Лазерный источник", "JPT YDFLP-30-M7-M-R"}}},
	// 2 — АО «УралСтальКонструкция»
	{clientIdx: 2, typeIdx: 0, brand: "Han's Laser", model: "G3015HF", sku: "G3015HF", serial: "HL-20-3015-0461",
		specs: []sp{{"Рабочее поле", "1500×3000 мм"}, {"Мощность источника", "6 кВт"}, {"Лазерный источник", "MAX Photonics MFSC-6000"}, {"Режущая головка", "Raytools BM115"}, {"ЧПУ", "Friendess FSCUT4000"}, {"Год выпуска", "2020"}}},
	{clientIdx: 2, typeIdx: 1, brand: "MAX Photonics", model: "MFSC-6000", sku: "MFSC-6000", serial: "MX20-6K-0412",
		specs: []sp{{"Мощность", "6000 Вт"}, {"Длина волны", "1080 нм"}, {"Режим", "непрерывный"}, {"Длина кабеля", "20 м"}, {"Разъём", "QBH"}}},
	{clientIdx: 2, typeIdx: 5, brand: "S&A (Teyu)", model: "CWFL-6000", sku: "CWFL-6000ENP", serial: "CW6K-20-0198",
		specs: []sp{{"Для лазера", "6 кВт"}, {"Контуры", "2 (источник + оптика)"}, {"Хладагент", "R-410A"}, {"Напряжение", "380 В"}}},
	// 3 — ИП Гаврилов
	{clientIdx: 3, typeIdx: 0, brand: "Senfeng", model: "SF3015G", sku: "SF3015G", serial: "SF19-3015-2280", note: "Режет в основном нержавейку 0,8–2 мм под вывески",
		specs: []sp{{"Рабочее поле", "1500×3000 мм"}, {"Мощность источника", "1,5 кВт"}, {"Лазерный источник", "Raycus RFL-C1500"}, {"Режущая головка", "Raytools BT240S"}, {"ЧПУ", "Friendess FSCUT1000"}, {"Год выпуска", "2019"}}},
	{clientIdx: 3, typeIdx: 7, brand: "Raycus", model: "Маркер 20 Вт", sku: "RFL-P20QB", serial: "RC-P20-19-0077",
		specs: []sp{{"Мощность", "20 Вт"}, {"Поле маркировки", "110×110 мм"}, {"Лазерный источник", "Raycus RFL-P20QB"}}},
	// 4 — ООО «Реставрация-Юг»
	{clientIdx: 4, typeIdx: 3, brand: "Qilin", model: "Очистка 1500 Вт (непрерывная)", sku: "QL-C1500", serial: "QL-23-C1500-044", note: "Удаление ржавчины и краски с металлоконструкций",
		specs: []sp{{"Мощность", "1500 Вт"}, {"Режим", "непрерывный"}, {"Лазерный источник", "Raycus RFL-C1500"}, {"Ширина сканирования", "до 150 мм"}}},
	{clientIdx: 4, typeIdx: 3, brand: "Relfar", model: "Очистка 200 Вт (импульсная)", sku: "RF-CL200P", serial: "RF-22-0915", note: "Реставрация камня и дерева",
		specs: []sp{{"Мощность", "200 Вт"}, {"Режим", "импульсный"}, {"Лазерный источник", "MAX Photonics MFP-200"}, {"Ширина сканирования", "до 100 мм"}}},
	// 5 — ООО «ВолгаСварка»
	{clientIdx: 5, typeIdx: 2, brand: "Qilin", model: "QL-W1500", sku: "QLW-1500-4", serial: "QL-22-0931",
		specs: []sp{{"Мощность", "1500 Вт"}, {"Лазерный источник", "Raycus RFL-C1500"}, {"Сварочная головка", "Qilin QHW-3"}, {"Подача проволоки", "да, Ø0,8–1,2 мм"}, {"Напряжение", "220 В"}}},
	{clientIdx: 5, typeIdx: 4, brand: "Qilin", model: "QHW-3", sku: "QHW-3", serial: "QH-7781",
		specs: []sp{{"Разъём", "QBH"}, {"Коллиматор", "D20 F60"}, {"Фокус", "F150"}, {"Защитное стекло", "20×3 мм"}}},
	{clientIdx: 5, typeIdx: 2, brand: "Relfar", model: "3-в-1 2000 Вт (сварка/резка/очистка)", sku: "RF-3IN1-2000", serial: "RF-23-2K-0310",
		specs: []sp{{"Мощность", "2000 Вт"}, {"Лазерный источник", "MAX Photonics MFSC-2000"}, {"Сварочная головка", "Relfar, штатная"}, {"Подача проволоки", "да, Ø0,8–1,6 мм"}, {"Напряжение", "220 В"}}},
	{clientIdx: 5, typeIdx: 8, brand: "Raycus", model: "Выходной кабель QBH", sku: "QBH-10", note: "Привезли только кабель — перебит у разъёма",
		specs: []sp{{"Длина", "10 м"}, {"Разъём", "QBH"}, {"Диаметр жилы", "50 мкм"}}},
	// 6 — ООО «СибМеталлПрофиль»
	{clientIdx: 6, typeIdx: 0, brand: "Golden Laser", model: "GF-1530", sku: "GF-1530", serial: "GL-18-1530-0716",
		specs: []sp{{"Рабочее поле", "1500×3000 мм"}, {"Мощность источника", "2 кВт"}, {"Лазерный источник", "IPG YLR-2000"}, {"Режущая головка", "Precitec LightCutter"}, {"ЧПУ", "Friendess FSCUT2000"}, {"Год выпуска", "2018"}}},
	{clientIdx: 6, typeIdx: 1, brand: "IPG Photonics", model: "YLR-2000", sku: "YLR-2000", serial: "PLMR18010233",
		specs: []sp{{"Мощность", "2000 Вт"}, {"Длина волны", "1070 нм"}, {"Режим", "непрерывный"}, {"Длина кабеля", "15 м"}, {"Разъём", "QD"}}},
	{clientIdx: 6, typeIdx: 6, brand: "Friendess", model: "FSCUT2000", sku: "FSCUT2000C", serial: "FR-2000-18-5530",
		specs: []sp{{"Система", "CypCut"}, {"Версия ПО", "6.3"}, {"Датчик высоты", "BCS100"}}},
	// 7 — ТОО «Astana Metal Group»
	{clientIdx: 7, typeIdx: 0, brand: "Bodor", model: "P3", sku: "P3-1530", serial: "BD23-12K-0088",
		specs: []sp{{"Рабочее поле", "1500×3000 мм"}, {"Мощность источника", "12 кВт"}, {"Лазерный источник", "Raycus RFL-C12000"}, {"Режущая головка", "Raytools BM115"}, {"ЧПУ", "Bodor BodorThinker"}, {"Год выпуска", "2023"}}},
	{clientIdx: 7, typeIdx: 5, brand: "S&A (Teyu)", model: "CWFL-12000", sku: "CWFL-12000", serial: "CW12K-23-0031",
		specs: []sp{{"Для лазера", "12 кВт"}, {"Контуры", "2 (источник + оптика)"}, {"Хладагент", "R-410A"}, {"Напряжение", "380 В"}}},
	// 8 — ООО «БелЛазерТех»
	{clientIdx: 8, typeIdx: 0, brand: "Senfeng", model: "SF3015H", sku: "SF3015H", serial: "SF22-3015-4417",
		specs: []sp{{"Рабочее поле", "1500×3000 мм"}, {"Мощность источника", "3 кВт"}, {"Лазерный источник", "MAX Photonics MFSC-3000"}, {"Режущая головка", "WSX NC30"}, {"ЧПУ", "Friendess FSCUT2000"}, {"Год выпуска", "2022"}}},
	{clientIdx: 8, typeIdx: 4, brand: "WSX", model: "NC30", sku: "NC30", serial: "WSX-NC30-22-1207",
		specs: []sp{{"Разъём", "QBH"}, {"Коллиматор", "F100"}, {"Фокус", "F150, автофокус"}, {"Защитное стекло", "27,9×4,1 мм"}}},
	// 9 — ООО «Узмет Сервис»
	{clientIdx: 9, typeIdx: 0, brand: "LXShow", model: "LX3015FC", sku: "LX3015FC", serial: "LX26-3015-0102", note: "Новый, пришёл в контейнере — нужна пусконаладка",
		specs: []sp{{"Рабочее поле", "1500×3000 мм"}, {"Мощность источника", "2 кВт"}, {"Лазерный источник", "Raycus RFL-C2000S"}, {"Режущая головка", "Raytools BM111"}, {"ЧПУ", "Friendess FSCUT2000"}, {"Год выпуска", "2026"}}},
	{clientIdx: 9, typeIdx: 2, brand: "Qilin", model: "QL-W2000", sku: "QLW-2000", serial: "QL-25-2K-0617",
		specs: []sp{{"Мощность", "2000 Вт"}, {"Лазерный источник", "Raycus RFL-C2000S"}, {"Сварочная головка", "Qilin QHW-3"}, {"Подача проволоки", "да, Ø0,8–1,2 мм"}, {"Напряжение", "220 В"}}},
}

func (s *seeder) seedEquipment(clients []clientRow) ([]string, error) {
	typeIDs := make([]string, 0, len(demoEquipmentTypes))
	for _, t := range demoEquipmentTypes {
		var id string
		if err := s.tx.QueryRow(s.ctx,
			`INSERT INTO equipment_types (name, spec_keys) VALUES ($1, $2::text[]) RETURNING id`,
			t.name, t.keys).Scan(&id); err != nil {
			return nil, err
		}
		typeIDs = append(typeIDs, id)
	}

	equipIDs := make([]string, 0, len(demoEquipment))
	for _, e := range demoEquipment {
		specsJSON, err := json.Marshal(e.specs)
		if err != nil {
			return nil, fmt.Errorf("marshal specs: %w", err)
		}
		var id string
		if err := s.tx.QueryRow(s.ctx,
			`INSERT INTO equipment (client_id, type_id, brand, model, sku, serial, specs, note)
			 VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, $8) RETURNING id`,
			clients[e.clientIdx].ID, typeIDs[e.typeIdx], e.brand, e.model, e.sku, e.serial, specsJSON, e.note).Scan(&id); err != nil {
			return nil, err
		}
		equipIDs = append(equipIDs, id)
	}
	return equipIDs, nil
}

// ---------------------------------------------------------------------------------------
// Каталог: единый список категорий, combined-индекс 0..10 (0-4 — услуги, 5-10 — товары),
// затем products/services.
// ---------------------------------------------------------------------------------------

type demoCategory struct{ kind, name string }

var demoCategories = []demoCategory{
	{kind: "service", name: "Диагностика"},           // 0
	{kind: "service", name: "Ремонт"},                // 1
	{kind: "service", name: "Настройка и юстировка"}, // 2
	{kind: "service", name: "Техобслуживание"},       // 3
	{kind: "service", name: "Выезд и пусконаладка"},  // 4
	{kind: "product", name: "Оптика"},                // 5
	{kind: "product", name: "Сопла и керамика"},      // 6
	{kind: "product", name: "Электроника и датчики"}, // 7
	{kind: "product", name: "Охлаждение"},            // 8
	{kind: "product", name: "Сварочные расходники"},  // 9
	{kind: "product", name: "Кабели"},                // 10
}

func (s *seeder) seedCategories() ([]string, error) {
	out := make([]string, 0, len(demoCategories))
	for _, c := range demoCategories {
		var id string
		if err := s.tx.QueryRow(s.ctx,
			`INSERT INTO catalog_categories (kind, name) VALUES ($1, $2) RETURNING id`,
			c.kind, c.name).Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, nil
}

// demoProductItem — запчасти и расходники. categoryIdx — combined-индекс demoCategories
// (5-10, товарные категории). stock="0" и stock<min_stock — намеренно (дефицит на
// «Остатках», заказы в «Ждёт запчасти»).
type demoProductItem struct {
	categoryIdx            int
	name, sku, unit        string
	price, stock, minStock string
	box, place             string
}

var demoProducts = []demoProductItem{
	{categoryIdx: 5, name: "Защитное стекло Raytools 27,9×4,1 мм", sku: "RT-PW-27941", unit: "шт", price: "450", stock: "60", minStock: "20", box: "О-01", place: "Шкаф оптики, полка 1"},        // 0
	{categoryIdx: 5, name: "Защитное стекло Precitec 30×1,5 мм", sku: "PR-PW-3015", unit: "шт", price: "900", stock: "25", minStock: "10", box: "О-02", place: "Шкаф оптики, полка 1"},           // 1
	{categoryIdx: 5, name: "Защитное стекло сварочной головки 20×3 мм", sku: "QL-PW-2003", unit: "шт", price: "180", stock: "100", minStock: "30", box: "О-03", place: "Шкаф оптики, полка 1"},   // 2
	{categoryIdx: 5, name: "Линза фокусирующая D28 F125 (Raytools BM111)", sku: "RT-FL-28125", unit: "шт", price: "6500", stock: "3", minStock: "2", box: "О-04", place: "Шкаф оптики, полка 2"}, // 3
	{categoryIdx: 5, name: "Линза коллиматорная D30 F100 (Precitec)", sku: "PR-CL-30100", unit: "шт", price: "9800", stock: "1", minStock: "1", box: "О-04", place: "Шкаф оптики, полка 2"},      // 4
	{categoryIdx: 6, name: "Сопло одинарное Raytools D1.5 (Ø28)", sku: "RT-NZS-15", unit: "шт", price: "220", stock: "80", minStock: "30", box: "С-01", place: "Стеллаж A, полка 2"},             // 5
	{categoryIdx: 6, name: "Сопло двойное Raytools D2.0 (Ø28)", sku: "RT-NZD-20", unit: "шт", price: "240", stock: "60", minStock: "30", box: "С-01", place: "Стеллаж A, полка 2"},               // 6
	{categoryIdx: 6, name: "Сопло Precitec D1.2", sku: "PR-NZ-12", unit: "шт", price: "480", stock: "20", minStock: "10", box: "С-02", place: "Стеллаж A, полка 2"},                              // 7
	{categoryIdx: 6, name: "Керамическое кольцо Raytools Ø28 (BT240S/BM111)", sku: "RT-CR-28", unit: "шт", price: "1900", stock: "4", minStock: "3", box: "С-03", place: "Стеллаж A, полка 3"},   // 8
	{categoryIdx: 6, name: "Керамика Precitec KT B2", sku: "PR-CR-KTB2", unit: "шт", price: "3600", stock: "0", minStock: "2", box: "С-03", place: "Стеллаж A, полка 3"},                         // 9
	{categoryIdx: 7, name: "Кабель ёмкостного датчика Raytools", sku: "RT-SC-01", unit: "шт", price: "4500", stock: "2", minStock: "1", box: "Э-01", place: "Стеллаж B, полка 1"},                // 10
	{categoryIdx: 7, name: "Усилитель датчика высоты Friendess BCS100", sku: "FR-BCS100", unit: "шт", price: "28000", stock: "1", minStock: "1", box: "Э-02", place: "Стеллаж B, полка 1"},       // 11
	{categoryIdx: 7, name: "Модуль накачки Raycus (RFL-C1500)", sku: "RC-PM-1500", unit: "шт", price: "65000", stock: "0", minStock: "1", box: "", place: "Склад, паллет 1"},                     // 12
	{categoryIdx: 8, name: "Фильтр PP для чиллера S&A 10\"", sku: "SA-FLT-PP10", unit: "шт", price: "900", stock: "10", minStock: "4", box: "Х-01", place: "Стеллаж C, полка 1"},                 // 13
	{categoryIdx: 8, name: "Фильтр деионизирующий S&A", sku: "SA-FLT-DI", unit: "шт", price: "2500", stock: "5", minStock: "2", box: "Х-01", place: "Стеллаж C, полка 1"},                        // 14
	{categoryIdx: 8, name: "Вода дистиллированная 5 л", sku: "H2O-5", unit: "кан", price: "250", stock: "20", minStock: "6", box: "", place: "Стеллаж C, пол"},                                   // 15
	{categoryIdx: 8, name: "Незамерзающая жидкость для чиллера 5 л", sku: "AF-5", unit: "кан", price: "3200", stock: "3", minStock: "2", box: "", place: "Стеллаж C, пол"},                       // 16
	{categoryIdx: 9, name: "Проволока ER308L Ø1,0 мм (нерж.)", sku: "WR-308L-10", unit: "кг", price: "1100", stock: "30", minStock: "10", box: "", place: "Стеллаж D, полка 1"},                  // 17
	{categoryIdx: 9, name: "Проволока ER5356 Ø1,0 мм (алюминий)", sku: "WR-5356-10", unit: "кг", price: "1600", stock: "8", minStock: "5", box: "", place: "Стеллаж D, полка 1"},                 // 18
	{categoryIdx: 9, name: "Медные насадки сварочной головки (набор)", sku: "QL-TIP-SET", unit: "компл", price: "1200", stock: "6", minStock: "2", box: "Св-01", place: "Стеллаж D, полка 2"},    // 19
	{categoryIdx: 8, name: "Шланг чиллера Ø8 мм", sku: "SA-HOSE-8", unit: "м", price: "180", stock: "40", minStock: "10", box: "", place: "Стеллаж C, полка 2"},                                  // 20
	{categoryIdx: 10, name: "Кабель ПВС 3×1,5", sku: "PVS-315", unit: "м", price: "120", stock: "150", minStock: "30", box: "", place: "Стеллаж B, полка 3"},                                     // 21
}

// moscowShareMin — товар с остатком ≥ этого числа делит его со складом «Москва» (¼).
const moscowShareMin = 20

// splitStock — стартовый остаток по складам (DS-012): ≥moscowShareMin → ¼ (вниз) в Москву,
// остальное — Алматы; иначе (или нецелое) всё на «Алматы».
func splitStock(stock string) (almaty, moscow string) {
	n, err := strconv.Atoi(stock)
	if err != nil || n < moscowShareMin {
		return stock, "0"
	}
	m := n / 4
	return strconv.Itoa(n - m), strconv.Itoa(m)
}

// loadWarehouses — id «Алматы»/«Москва» (сид миграции 0008).
func (s *seeder) loadWarehouses() error {
	if err := s.tx.QueryRow(s.ctx, `SELECT id FROM warehouses WHERE name = 'Алматы' AND deleted_at IS NULL`).Scan(&s.almatyID); err != nil {
		return err
	}
	return s.tx.QueryRow(s.ctx, `SELECT id FROM warehouses WHERE name = 'Москва' AND deleted_at IS NULL`).Scan(&s.moscowID)
}

// seedProducts — вставляет товар (price/sku/unit сразу), строки складов (product_stock:
// box/place/min_stock) и стартовый остаток движением manual по каждому складу (Stock
// Adjust Query, D44) — так же, как ручная правка на «Остатках». Раскладка по складам —
// splitStock (DS-012).
func (s *seeder) seedProducts(categoryIDs []string) ([]productRow, error) {
	if err := s.loadWarehouses(); err != nil {
		return nil, err
	}
	out := make([]productRow, 0, len(demoProducts))
	for _, p := range demoProducts {
		var id string
		if err := s.tx.QueryRow(s.ctx,
			`INSERT INTO products (category_id, name, sku, unit, price)
			 VALUES ($1, $2, $3, $4, $5::numeric) RETURNING id`,
			categoryIDs[p.categoryIdx], p.name, p.sku, p.unit, p.price).Scan(&id); err != nil {
			return nil, err
		}
		almaty, moscow := splitStock(p.stock)
		if err := s.initialStock(id, s.almatyID, almaty, p.minStock, p.box, p.place); err != nil {
			return nil, err
		}
		if moscow != "0" {
			if err := s.initialStock(id, s.moscowID, moscow, "0", "", "Москва, стеллаж M"); err != nil {
				return nil, err
			}
		}
		out = append(out, productRow{ID: id})
	}
	return out, nil
}

// initialStock — строка product_stock + стартовое движение manual (нулевой остаток — без
// движения: stock_movements.qty_delta CHECK <> 0).
func (s *seeder) initialStock(productID, warehouseID, stock, minStock, box, place string) error {
	if _, err := s.tx.Exec(s.ctx,
		`INSERT INTO product_stock (product_id, warehouse_id, stock, min_stock, box, place)
		 VALUES ($1::uuid, $2::uuid, $3::numeric, $4::numeric, $5, $6)`,
		productID, warehouseID, stock, minStock, box, place); err != nil {
		return err
	}
	if stock == "0" {
		return nil
	}
	_, err := s.tx.Exec(s.ctx,
		`INSERT INTO stock_movements (product_id, warehouse_id, qty_delta, reason, user_id) VALUES ($1::uuid, $2::uuid, $3::numeric, 'manual', $4::uuid)`,
		productID, warehouseID, stock, s.adminID)
	return err
}

// demoTransferLine — строка демо-перемещения: productIdx — индекс demoProducts.
type demoTransferLine struct {
	productIdx int
	qty        string
}

// demoSupplier — поставщики (справочник части suppliers). Архивный — в новом документе не
// выбирается (TS-020), но виден в фильтре списка с пометкой.
type demoSupplier struct {
	name, contact, phone, address, note string
	active                              bool
}

var demoSuppliers = []demoSupplier{
	{name: "ЛазерКомплект", contact: "Ахметов Данияр", phone: "+7 727 300-10-10", address: "Алматы, ул. Сатпаева, 30", note: "Сопла, линзы, защитные стёкла", active: true},
	{name: "ОптТехСнаб", contact: "Петров Игорь", phone: "+7 495 700-20-20", address: "Москва, Варшавское ш., 125", note: "Больше не работаем — сорвали поставку", active: false},
}

// seedSuppliers — два поставщика (один в архиве); повторный прогон по тому же названию
// (suppliers_name_lower_uq — частичный, только неудалённые, миграция 0023) строку не дублирует. Первый действующий — s.supplierID.
func (s *seeder) seedSuppliers() error {
	for _, sp := range demoSuppliers {
		var id string
		if err := s.tx.QueryRow(s.ctx, `
			INSERT INTO suppliers (name, contact_name, phone, address, note, is_active)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (lower(btrim(name))) WHERE deleted_at IS NULL DO UPDATE SET updated_at = suppliers.updated_at
			RETURNING id::text`, sp.name, sp.contact, sp.phone, sp.address, sp.note, sp.active).Scan(&id); err != nil {
			return err
		}
		if sp.active && s.supplierID == "" {
			s.supplierID = id
		}
	}
	return nil
}

// seedTransfers — документы «Перемещение» без клиента (transfers-core / transfers-partners
// TS-F1). Склад → склад, Алматы → Москва: полученный (transfer_out на источнике + transfer_in
// на получателе) и в пути (только transfer_out); в документ «в пути» позже добавляется
// строка-единица оборудования (seedEquipmentLocation, заказ demoTransitTask) — его id
// запоминается в s.transitTransferID. Поставщик → склад «Алматы»: полученный с расхождением
// (заказано 20, получено 18 — supplier_in на факт) и в пути (движений нет, DS-040). Остатки
// product_stock правятся теми же дельтами, что и журнал (как transfers.Service).
func (s *seeder) seedTransfers(products []productRow) error {
	received := []demoTransferLine{{productIdx: 2, qty: "10"}}
	inTransit := []demoTransferLine{{productIdx: 5, qty: "10"}, {productIdx: 20, qty: "5"}}

	var receivedID int64
	if err := s.tx.QueryRow(s.ctx, `
		INSERT INTO transfers (direction, from_warehouse_id, to_warehouse_id, status, note, created_by,
		                       sent_by, sent_at, received_by, received_at, created_at, updated_at)
		VALUES ('warehouse_to_warehouse', $1, $2, 'received', 'Пополнение склада в Москве', $3, $3, now() - interval '3 days',
		        $3, now() - interval '2 days', now() - interval '3 days', now() - interval '2 days')
		RETURNING id`, s.almatyID, s.moscowID, s.adminID).Scan(&receivedID); err != nil {
		return err
	}
	for pos, l := range received {
		if err := s.transferLine(receivedID, pos+1, products[l.productIdx].ID, l.qty, true); err != nil {
			return err
		}
	}

	var transitID int64
	if err := s.tx.QueryRow(s.ctx, `
		INSERT INTO transfers (direction, from_warehouse_id, to_warehouse_id, status, note, created_by, sent_by, sent_at)
		VALUES ('warehouse_to_warehouse', $1, $2, 'in_transit', 'Сопла, шланг и оборудование клиента для выездной бригады', $3, $3, now() - interval '5 hours')
		RETURNING id`, s.almatyID, s.moscowID, s.adminID).Scan(&transitID); err != nil {
		return err
	}
	for pos, l := range inTransit {
		if err := s.transferLine(transitID, pos+1, products[l.productIdx].ID, l.qty, false); err != nil {
			return err
		}
	}
	s.transitTransferID = transitID
	if err := s.seedDeletedTransfer(products); err != nil {
		return err
	}
	return s.seedSupplierTransfers(products)
}

// seedDeletedTransfer — одна мягко удалённая заявка «склад → склад» (soft-delete-transfers
// TS-F: вкладка «Перемещения» раздела «Удалённые» на демо-стенде непустая): статус requested,
// «кто, когда, почему» заполнены вместе (CHECK transfers_deleted_chk), version = 2 (создание +
// удаление). Строка — товарная: заявка остатки не двигает и единиц оборудования не занимает,
// так что документ восстанавливается без условий.
func (s *seeder) seedDeletedTransfer(products []productRow) error {
	var id int64
	if err := s.tx.QueryRow(s.ctx, `
		INSERT INTO transfers (direction, from_warehouse_id, to_warehouse_id, status, note, created_by,
		                       created_at, updated_at, version, deleted_at, deleted_by, delete_reason)
		VALUES ('warehouse_to_warehouse', $1, $2, 'requested', 'Заявка на перемещение расходников', $3,
		        now() - interval '1 day 2 hours', now() - interval '1 day', 2,
		        now() - interval '1 day', $3, 'Заявка создана по ошибке: товар уже отправлен другим документом')
		RETURNING id`, s.almatyID, s.moscowID, s.adminID).Scan(&id); err != nil {
		return err
	}
	_, err := s.tx.Exec(s.ctx, `
		INSERT INTO transfer_items (transfer_id, kind, product_id, position, qty_sent) VALUES ($1, 'product', $2::uuid, 1, 3)`,
		id, products[5].ID)
	return err
}

// seedSupplierTransfers — два прихода от поставщика на «Алматы»: полученный с недопоставкой
// (остаток растёт на факт, движение supplier_in) и «в пути» (учтён на «Остатках» в колонке
// «в пути», DS-047; движений нет). Полученный — откатанный и принятый заново (transfers-revert
// TS-F1): сначала по ошибке принят на всё заказанное (supplier_in +20), проведение отменено с
// причиной (transfer_unreceive −20), затем принят с верным фактом (supplier_in +18).
func (s *seeder) seedSupplierTransfers(products []productRow) error {
	const ordered, fact = "20", "18"
	var receivedID int64
	if err := s.tx.QueryRow(s.ctx, `
		INSERT INTO transfers (direction, to_warehouse_id, supplier_id, status, note, created_by,
		                       sent_by, sent_at, received_by, received_at, created_at, updated_at,
		                       unreceived_by, unreceived_at, unreceive_reason, version)
		VALUES ('supplier_to_warehouse', $1, $2, 'received', 'Плановая закупка расходников', $3, $3, now() - interval '6 days',
		        $3, now() - interval '4 days', now() - interval '6 days', now() - interval '4 days',
		        $3, now() - interval '4 days 2 hours', 'Приняли без пересчёта — по факту пришло меньше', 5)
		RETURNING id`, s.almatyID, s.supplierID, s.adminID).Scan(&receivedID); err != nil {
		return err
	}
	// первая (ошибочная) приёмка на всё заказанное и её откат — компенсирующее движение.
	if err := s.moveStock(products[7].ID, s.almatyID, ordered, "supplier_in", receivedID); err != nil {
		return err
	}
	if err := s.moveStock(products[7].ID, s.almatyID, "-"+ordered, "transfer_unreceive", receivedID); err != nil {
		return err
	}
	if _, err := s.tx.Exec(s.ctx, `
		INSERT INTO transfer_items (transfer_id, kind, product_id, position, qty_sent, qty_received)
		VALUES ($1, 'product', $2, 1, $3::numeric, $4::numeric)`, receivedID, products[7].ID, ordered, fact); err != nil {
		return err
	}
	if err := s.moveStock(products[7].ID, s.almatyID, fact, "supplier_in", receivedID); err != nil {
		return err
	}

	var transitID int64
	if err := s.tx.QueryRow(s.ctx, `
		INSERT INTO transfers (direction, to_warehouse_id, supplier_id, status, note, created_by, sent_by, sent_at)
		VALUES ('supplier_to_warehouse', $1, $2, 'in_transit', 'Довоз по прошлой закупке', $3, $3, now() - interval '1 day')
		RETURNING id`, s.almatyID, s.supplierID, s.adminID).Scan(&transitID); err != nil {
		return err
	}
	_, err := s.tx.Exec(s.ctx, `
		INSERT INTO transfer_items (transfer_id, kind, product_id, position, qty_sent)
		VALUES ($1, 'product', $2, 1, 30)`, transitID, products[9].ID)
	return err
}

// transferLine — строка перемещения «склад → склад» + списание с «Алматы» (transfer_out) и, если получено,
// приход на «Москву» (transfer_in, без недостачи).
func (s *seeder) transferLine(transferID int64, pos int, productID, qty string, received bool) error {
	var qtyReceived any
	if received {
		qtyReceived = qty
	}
	if _, err := s.tx.Exec(s.ctx, `
		INSERT INTO transfer_items (transfer_id, kind, product_id, position, qty_sent, qty_received)
		VALUES ($1, 'product', $2, $3, $4::numeric, $5::numeric)`, transferID, productID, pos, qty, qtyReceived); err != nil {
		return err
	}
	if err := s.moveStock(productID, s.almatyID, "-"+qty, "transfer_out", transferID); err != nil {
		return err
	}
	if !received {
		return nil
	}
	return s.moveStock(productID, s.moscowID, qty, "transfer_in", transferID)
}

// moveStock — дельта остатка пары товар×склад (строка создаётся при отсутствии) + движение.
func (s *seeder) moveStock(productID, warehouseID, delta, reason string, transferID int64) error {
	if _, err := s.tx.Exec(s.ctx, `
		INSERT INTO product_stock (product_id, warehouse_id, stock) VALUES ($1, $2, $3::numeric)
		ON CONFLICT (product_id, warehouse_id) DO UPDATE SET stock = product_stock.stock + EXCLUDED.stock`,
		productID, warehouseID, delta); err != nil {
		return err
	}
	_, err := s.tx.Exec(s.ctx, `
		INSERT INTO stock_movements (product_id, warehouse_id, qty_delta, reason, user_id, transfer_id)
		VALUES ($1, $2, $3::numeric, $4, $5, $6)`, productID, warehouseID, delta, reason, s.adminID, transferID)
	return err
}

// protoGood — комплект товара услуги/подзадачи (prototype goods[] {productId,qty[,price]}).
// productIdx — индекс demoProducts; Price="" → берём цену товара.
type protoGood struct {
	productIdx int
	qty        string
	price      string // "" — цена товара (как newSubtask() в прототипе)
}

// demoServiceItem — услуги сервиса. categoryIdx — combined-индекс demoCategories (0-4,
// категории услуг). price="" → NULL («по факту»: ремонт источника/очистки — цена после
// вскрытия).
type demoServiceItem struct {
	categoryIdx int
	name        string
	price       string
	goods       []protoGood
}

var demoServices = []demoServiceItem{
	{categoryIdx: 0, name: "Диагностика лазерного источника", price: "5000", goods: []protoGood{}},                                                            // 0
	{categoryIdx: 0, name: "Диагностика станка лазерной резки", price: "4000", goods: []protoGood{}},                                                          // 1
	{categoryIdx: 3, name: "Замена защитного стекла и чистка оптики головки", price: "1500", goods: []protoGood{{0, "1", ""}}},                                // 2
	{categoryIdx: 2, name: "Центровка сопла и юстировка луча", price: "3000", goods: []protoGood{{5, "1", ""}}},                                               // 3
	{categoryIdx: 2, name: "Калибровка ёмкостного датчика высоты", price: "2500", goods: []protoGood{}},                                                       // 4
	{categoryIdx: 1, name: "Замена керамики и сопла (Raytools)", price: "1500", goods: []protoGood{{8, "1", ""}, {5, "1", ""}}},                               // 5
	{categoryIdx: 1, name: "Замена фокусирующей линзы", price: "3500", goods: []protoGood{{3, "1", ""}, {0, "1", ""}}},                                        // 6
	{categoryIdx: 1, name: "Ремонт лазерного источника (замена модуля накачки)", price: "", goods: []protoGood{{12, "1", ""}}},                                // 7
	{categoryIdx: 3, name: "ТО чиллера (фильтры, замена воды)", price: "3500", goods: []protoGood{{13, "1", ""}, {14, "1", ""}, {15, "4", ""}}},               // 8
	{categoryIdx: 3, name: "Плановое ТО станка лазерной резки", price: "12000", goods: []protoGood{{0, "2", ""}, {5, "2", ""}, {13, "1", ""}, {15, "4", ""}}}, // 9
	{categoryIdx: 2, name: "Подбор режимов резки (технолог)", price: "6000", goods: []protoGood{}},                                                            // 10
	{categoryIdx: 1, name: "Ремонт сварочной головки", price: "7000", goods: []protoGood{{2, "2", ""}, {19, "1", ""}}},                                        // 11
	{categoryIdx: 2, name: "Настройка аппарата лазерной сварки", price: "4000", goods: []protoGood{{2, "1", ""}}},                                             // 12
	{categoryIdx: 1, name: "Ремонт аппарата лазерной очистки", price: "", goods: []protoGood{}},                                                               // 13
	{categoryIdx: 4, name: "Пусконаладка станка лазерной резки", price: "45000", goods: []protoGood{}},                                                        // 14
	{categoryIdx: 4, name: "Выезд инженера (Москва и МО)", price: "3500", goods: []protoGood{}},                                                               // 15
	{categoryIdx: 1, name: "Замена керамики и сопла (Precitec)", price: "2000", goods: []protoGood{{9, "1", ""}, {7, "1", ""}}},                               // 16
}

// seedServices — услуги + service_products (комплект по умолчанию, position — порядок
// goods); price NULL для услуг «по факту».
func (s *seeder) seedServices(categoryIDs []string, products []productRow) ([]serviceRow, error) {
	out := make([]serviceRow, 0, len(demoServices))
	for _, sv := range demoServices {
		var id string
		var err error
		if sv.price == "" {
			err = s.tx.QueryRow(s.ctx,
				`INSERT INTO services (category_id, name, price) VALUES ($1, $2, NULL) RETURNING id`,
				categoryIDs[sv.categoryIdx], sv.name).Scan(&id)
		} else {
			err = s.tx.QueryRow(s.ctx,
				`INSERT INTO services (category_id, name, price) VALUES ($1, $2, $3::numeric) RETURNING id`,
				categoryIDs[sv.categoryIdx], sv.name, sv.price).Scan(&id)
		}
		if err != nil {
			return nil, err
		}
		for pos, g := range sv.goods {
			if _, err := s.tx.Exec(s.ctx,
				`INSERT INTO service_products (service_id, product_id, qty, position) VALUES ($1, $2, $3::numeric, $4)`,
				id, products[g.productIdx].ID, g.qty, pos); err != nil {
				return nil, err
			}
		}
		out = append(out, serviceRow{ID: id, Price: sv.price, Goods: sv.goods})
	}
	return out, nil
}

// loadColumns — колонки по умолчанию уже засеяны миграцией 0005 (design-sql.md §Columns
// Table, Сид D32) — seeddemo только читает их FOR UPDATE (Lock Order columns→tasks→products,
// ADR-104), по имени (id — gen_random_uuid(), фиксированных значений нет).
var demoColumnNames = []string{"Новые", "Согласование", "В работе", "Ждёт запчасти", "Готово"}

func (s *seeder) loadColumns() (map[string]columnRow, error) {
	rows, err := s.tx.Query(s.ctx,
		`SELECT id, name, is_final, consumes_stock FROM columns WHERE name = ANY($1) AND deleted_at IS NULL ORDER BY id FOR UPDATE`,
		demoColumnNames)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]columnRow, len(demoColumnNames))
	for rows.Next() {
		var name string
		var c columnRow
		if err := rows.Scan(&c.ID, &name, &c.IsFinal, &c.ConsumesStock); err != nil {
			return nil, err
		}
		out[name] = c
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// DS-054: «завершённый заказ» = заказ в последней колонке доски (наибольший position).
	if err := s.tx.QueryRow(s.ctx, `SELECT id FROM columns WHERE deleted_at IS NULL ORDER BY position DESC, id DESC LIMIT 1`).Scan(&s.lastColumnID); err != nil {
		return nil, err
	}
	for _, name := range demoColumnNames {
		if _, ok := out[name]; !ok {
			return nil, fmt.Errorf("seeddemo: колонка %q не найдена (миграция 0005 изменена?)", name)
		}
	}
	return out, nil
}

// ---------------------------------------------------------------------------------------
// Заказы: по 2-3 в каждой из 5 колонок доски (AC-019, D32), подзадачи в разных статусах,
// часть — просрочена/оплачена/без исполнителя.
// ---------------------------------------------------------------------------------------

// demoSubtaskItem — prototype newSubtask(serviceId, extra): serviceIdx — индекс
// demoServices; assigneeIdx -1 — без исполнителя; dueOffset —
// дни от текущей даты (nil — без срока, эталон dayOffset() прототипа); goodsOverride/
// priceOverride — explicit-поля extra (nil/"" — берём из услуги, как newSubtask()).
type demoSubtaskItem struct {
	serviceIdx    int
	status        string
	assigneeIdx   int
	dueOffset     *int
	priceOverride string // "" — из услуги
	goodsOverride []protoGood
	logs          []demoLog
	// Выплата исполнителю (executor-payouts, D71): payout — сумма выплаты ("" — не задана);
	// payoutPaid — отметка «Выплачено исполнителю» (executor_paid_at — сутки назад). Только
	// для услуг с исполнителем.
	payout     string
	payoutPaid bool
}

// demoLog — запись журнала работ услуги (subtask_logs). userIdx — индекс demoExecutors,
// -1 — администратор; hoursAgo — сдвиг created_at назад от текущего момента.
type demoLog struct {
	userIdx  int
	hoursAgo int
	text     string
}

// demoTaskEquipment — оборудование заказа (task_equipment): equipmentIdx — индекс
// demoEquipment (того же клиента, что и заказ); комплектность/состояние — для актов.
type demoTaskEquipment struct {
	equipmentIdx            int
	completeness, condition string
}

// demoTaskItem — заказ доски (без вложений — см. комментарий пакета).
type demoTaskItem struct {
	column      string
	clientIdx   int
	title       string
	assigneeIdx int // -1 — без исполнителя
	dueOffset   *int
	paid        bool
	descHTML    string
	equipment   []demoTaskEquipment
	subtasks    []demoSubtaskItem
}

func offset(n int) *int { return &n }

var demoTasks = []demoTaskItem{
	// --- Новые ---
	{
		column: "Новые", clientIdx: 0, title: "Bodor A3: не прорезает 8 мм сталь, рваный рез", assigneeIdx: 0, dueOffset: offset(1),
		descHTML:  "<p>Со слов клиента: после замены защитного стекла станок перестал прорезать 8 мм Ст3, на кромке грат.</p><ul><li>Bodor A3, Raycus RFL-C3000S, головка Raytools BM111</li><li>Проверить линзу, центровку сопла, мощность источника</li></ul>",
		equipment: []demoTaskEquipment{{equipmentIdx: 0}, {equipmentIdx: 2, condition: "Защитное стекло мутное, на корпусе следы брызг"}},
		subtasks: []demoSubtaskItem{
			{serviceIdx: 1, status: board.SubtaskStatusNew, assigneeIdx: 0, logs: []demoLog{
				{userIdx: -1, hoursAgo: 26, text: "Клиент прислал фото реза: грат по нижней кромке, 8 мм не прорезает, 5 мм — с трудом."},
			}},
			{serviceIdx: 6, status: board.SubtaskStatusInProgress, assigneeIdx: 0, dueOffset: offset(3), logs: []demoLog{
				{userIdx: 0, hoursAgo: 5, text: "Сняли BM111: на фокусирующей линзе прожог в центре, защитное стекло в налёте. Линзу меняем."},
			}},
		},
	},
	{
		column: "Новые", clientIdx: 3, title: "Senfeng 1,5 кВт: плановое ТО станка и чиллера", assigneeIdx: 1, dueOffset: offset(6), paid: true,
		equipment: []demoTaskEquipment{{equipmentIdx: 11}},
		subtasks: []demoSubtaskItem{
			{serviceIdx: 9, status: board.SubtaskStatusNew, assigneeIdx: 1, logs: []demoLog{
				{userIdx: 1, hoursAgo: 20, text: "Согласовали выезд, клиент подготовит доступ к станку и чиллеру, резку остановит на день."},
			}},
			{serviceIdx: 8, status: board.SubtaskStatusNew, assigneeIdx: 1},
		},
	},
	{
		column: "Новые", clientIdx: 9, title: "Узмет: пусконаладка LXShow LX3015FC после доставки", assigneeIdx: -1, dueOffset: offset(14),
		descHTML:  "<p>Выезд в Ташкент. Согласовать даты, подготовить станок к подключению (380 В, азот, воздух 8 бар).</p>",
		equipment: []demoTaskEquipment{{equipmentIdx: 26, completeness: "Станок, чиллер, компрессор — в заводской упаковке", condition: "Упаковка без повреждений"}},
		subtasks: []demoSubtaskItem{{serviceIdx: 14, status: board.SubtaskStatusNew, assigneeIdx: -1, logs: []demoLog{
			{userIdx: -1, hoursAgo: 48, text: "Ждём от клиента подтверждение готовности электрики 380 В и фото площадки под станок."},
		}}},
	},
	// --- Согласование ---
	{
		column: "Согласование", clientIdx: 1, title: "IPG YLS-6000: падение мощности, ошибка на источнике", assigneeIdx: 2,
		descHTML:  "<p>Источник выдаёт ~4,2 кВт вместо 6. Смета на ремонт у клиента на согласовании.</p>",
		equipment: []demoTaskEquipment{{equipmentIdx: 5, condition: "Ошибка на панели источника, мощность ~70%"}},
		subtasks: []demoSubtaskItem{
			{serviceIdx: 0, status: board.SubtaskStatusDone, assigneeIdx: 0, logs: []demoLog{
				{userIdx: 0, hoursAgo: 52, text: "Замер мощности на выходе: 4,2 кВт при 100% задания. По логам источника — деградация одного из модулей."},
			}},
			// модуль Raycus к IPG не подходит — комплект пустой, цена по смете
			{serviceIdx: 7, status: board.SubtaskStatusNew, assigneeIdx: 0, priceOverride: "180000", goodsOverride: []protoGood{}, logs: []demoLog{
				{userIdx: -1, hoursAgo: 30, text: "Смета отправлена клиенту, ждём ответа. Срок поставки модуля IPG — 3–4 недели."},
			}},
		},
	},
	{
		column: "Согласование", clientIdx: 7, title: "Смета: годовое ТО Bodor P3 12 кВт", assigneeIdx: -1,
		equipment: []demoTaskEquipment{{equipmentIdx: 22}, {equipmentIdx: 23}},
		subtasks: []demoSubtaskItem{
			{serviceIdx: 9, status: board.SubtaskStatusNew, assigneeIdx: -1, logs: []demoLog{
				{userIdx: -1, hoursAgo: 12, text: "Клиент просит смету с выездом в Астану, 2 дня работ, проживание за счёт клиента."},
			}},
			{serviceIdx: 8, status: board.SubtaskStatusNew, assigneeIdx: -1},
		},
	},
	// --- В работе ---
	{
		column: "В работе", clientIdx: 5, title: "Qilin QL-W1500: нестабильный шов, греется головка", assigneeIdx: 3, dueOffset: offset(-1),
		descHTML: "<p>Сварка нержавейки 2 мм: шов «плывёт», головка QHW-3 перегревается через 10 минут.</p>",
		equipment: []demoTaskEquipment{
			{equipmentIdx: 15, completeness: "Аппарат, кабель 10 м, подающий механизм", condition: "Корпус без повреждений"},
			{equipmentIdx: 16, completeness: "Головка QHW-3, набор насадок", condition: "Головка в нагаре, насадка оплавлена"},
		},
		subtasks: []demoSubtaskItem{
			{serviceIdx: 11, status: board.SubtaskStatusDone, assigneeIdx: 3, logs: []demoLog{
				{userIdx: 3, hoursAgo: 28, text: "Разобрали головку: защитное стекло треснуло, медная насадка оплавлена. Заменили оба."},
			}},
			{serviceIdx: 12, status: board.SubtaskStatusInProgress, assigneeIdx: 3, dueOffset: offset(-1), logs: []demoLog{
				{userIdx: 3, hoursAgo: 6, text: "Шов по нержавейке 2 мм стабилен при 1100 Вт, воблинг 3 мм. Проверяем нагрев на длинном прогоне."},
				{userIdx: 3, hoursAgo: 2, text: "После 30 минут сварки головка тёплая, но в норме. Проверим проток воды в чиллере."},
			}},
		},
	},
	{
		column: "В работе", clientIdx: 2, title: "Han's 6 кВт: сбой датчика высоты, удары головки о лист", assigneeIdx: 0, dueOffset: offset(2),
		equipment: []demoTaskEquipment{{equipmentIdx: 8}},
		subtasks: []demoSubtaskItem{
			{serviceIdx: 1, status: board.SubtaskStatusDone, assigneeIdx: 0, payout: "10000", logs: []demoLog{
				{userIdx: 0, hoursAgo: 40, text: "Кабель датчика перетёрт у головки — заменили. Ошибки BCS100 ушли не полностью."},
			}},
			{serviceIdx: 5, status: board.SubtaskStatusDone, assigneeIdx: 2, logs: []demoLog{
				{userIdx: 2, hoursAgo: 36, text: "Керамика треснула после удара — заменена вместе с соплом."},
			}},
			{serviceIdx: 4, status: board.SubtaskStatusInProgress, assigneeIdx: 0, logs: []demoLog{
				{userIdx: 0, hoursAgo: 4, text: "На листе 10 мм высота плавает ±0,3 мм. Проверяем усилитель BCS100 и заземление."},
			}},
		},
	},
	// --- Ждёт запчасти ---
	{
		column: "Ждёт запчасти", clientIdx: 4, title: "Очистка Qilin 1500 Вт: нет генерации — замена модуля накачки Raycus", assigneeIdx: 1, dueOffset: offset(10),
		descHTML:  "<p>Модуль накачки RFL-C1500 заказан у поставщика, срок поставки ~7 дней.</p>",
		equipment: []demoTaskEquipment{{equipmentIdx: 13, completeness: "Аппарат очистки, ручная головка", condition: "Не включается генерация, ошибка источника"}},
		subtasks: []demoSubtaskItem{
			{serviceIdx: 0, status: board.SubtaskStatusDone, assigneeIdx: 0, logs: []demoLog{
				{userIdx: 0, hoursAgo: 70, text: "Один из модулей накачки Raycus не выдаёт ток. Нужен модуль RC-PM-1500."},
			}},
			{serviceIdx: 7, status: board.SubtaskStatusNew, assigneeIdx: 0, dueOffset: offset(10), priceOverride: "85000", logs: []demoLog{
				{userIdx: -1, hoursAgo: 50, text: "Модуль заказан у поставщика, счёт оплачен, поставка ~7 дней."},
			}},
		},
	},
	{
		column: "Ждёт запчасти", clientIdx: 1, title: "Precitec ProCutter: замена керамики KT B2", assigneeIdx: 2, dueOffset: offset(5),
		descHTML:  "<p>Керамики KT B2 нет на складе — ждём поставку.</p>",
		equipment: []demoTaskEquipment{{equipmentIdx: 6}},
		subtasks: []demoSubtaskItem{{serviceIdx: 16, status: board.SubtaskStatusNew, assigneeIdx: 2, logs: []demoLog{
			{userIdx: 2, hoursAgo: 22, text: "Керамика KT B2 с трещиной, на складе нет — ждём поставку."},
		}}},
	},
	// --- Готово ---
	{
		column: "Готово", clientIdx: 6, title: "Golden Laser 2 кВт: ТО чиллера и чистка оптики Precitec", assigneeIdx: 1, dueOffset: offset(-5), paid: true,
		equipment: []demoTaskEquipment{{equipmentIdx: 19}},
		subtasks: []demoSubtaskItem{
			{serviceIdx: 8, status: board.SubtaskStatusDone, assigneeIdx: 1, payout: "12000", payoutPaid: true, logs: []demoLog{
				{userIdx: 1, hoursAgo: 150, text: "Заменили PP и DI фильтры, залили 20 л дистиллята. Проводимость в норме."},
			}},
			// головка Precitec — стекло 30×1,5 вместо Raytools из комплекта услуги
			{serviceIdx: 2, status: board.SubtaskStatusDone, assigneeIdx: 2, payout: "8000", payoutPaid: true, goodsOverride: []protoGood{{1, "1", ""}}, logs: []demoLog{
				{userIdx: 2, hoursAgo: 146, text: "Защитное стекло Precitec заменено, коллиматор чистый."},
			}},
		},
	},
	{
		column: "Готово", clientIdx: 8, title: "Senfeng 3 кВт: подбор режимов резки нержавейки 4 мм (азот)", assigneeIdx: 3, dueOffset: offset(-2), paid: true,
		equipment: []demoTaskEquipment{{equipmentIdx: 24}, {equipmentIdx: 25}},
		subtasks: []demoSubtaskItem{
			{serviceIdx: 10, status: board.SubtaskStatusDone, assigneeIdx: 3, payout: "15000", payoutPaid: true, logs: []demoLog{
				{userIdx: 3, hoursAgo: 60, text: "Нерж. 4 мм, азот 16 бар: 2,2 кВт, 3,2 м/мин, фокус −2 мм. Рез без окалины, таблица режимов передана клиенту."},
			}},
			{serviceIdx: 3, status: board.SubtaskStatusDone, assigneeIdx: 3, payout: "6000"},
		},
	},
	{
		column: "Готово", clientIdx: 0, title: "Выезд: замена защитного стекла Raytools BM111", assigneeIdx: 0, dueOffset: offset(-10),
		equipment: []demoTaskEquipment{{equipmentIdx: 2}},
		subtasks: []demoSubtaskItem{
			{serviceIdx: 15, status: board.SubtaskStatusDone, assigneeIdx: 0, payout: "5000"},
			{serviceIdx: 2, status: board.SubtaskStatusDone, assigneeIdx: 0, goodsOverride: []protoGood{{0, "2", ""}}, logs: []demoLog{
				{userIdx: 0, hoursAgo: 245, text: "Заменено защитное стекло, второе оставили клиенту в резерв."},
			}},
		},
	},
}

// dueDate — YYYY-MM-DD от сегодня (эталон dayOffset() прототипа: overdue/soon вычисляются
// относительно текущей даты, не хардкодятся).
func dueDate(n *int) *string {
	if n == nil {
		return nil
	}
	d := time.Now().AddDate(0, 0, *n).Format("2006-01-02")
	return &d
}

// demoMoscowTasks — индексы demoTasks, чьё оборудование принято на склад «Москва» (DS-012;
// товары их услуг — тоже с «Москвы», как склад по умолчанию DS-005). demoTransitTask —
// заказ, первая единица которого в пути Алматы → Москва. demoReturnTask — заказ, последняя
// единица которого отправлена клиенту документом «склад → клиент» и ещё в пути.
var demoMoscowTasks = map[int]bool{2: true, 4: true, 10: true}

const (
	demoTransitTask = 3
	demoReturnTask  = 5
)

func (s *seeder) seedTasks(columns map[string]columnRow, clients []clientRow, executors []executorRow, equipIDs []string, services []serviceRow, products []productRow) (int, error) {
	created := 0
	for i, t := range demoTasks {
		taskWarehouse := s.almatyID
		if demoMoscowTasks[i] {
			taskWarehouse = s.moscowID
		}
		col := columns[t.column]
		client := clients[t.clientIdx]

		var assigneeID *string
		if t.assigneeIdx >= 0 {
			assigneeID = &executors[t.assigneeIdx].ID
		}

		var taskID int64
		if err := s.tx.QueryRow(s.ctx,
			`INSERT INTO tasks (column_id, client_id, title, description_html, assignee_id, due_date, is_paid, paid_at, position, created_by)
			 VALUES ($1, $2, $3, $4, $5, $6::date, $7, CASE WHEN $7 THEN now() END, $8::numeric, $9) RETURNING id`,
			col.ID, client.ID, t.title, t.descHTML, assigneeID, dueDate(t.dueOffset), t.paid, fmt.Sprintf("%d", (i+1)*1024), s.adminID).
			Scan(&taskID); err != nil {
			return created, err
		}

		// Оборудование заказа; первое — опора услуг заказа (subtasks.equipment_id).
		var mainEquipID *string
		unitIDs := make([]string, 0, len(t.equipment))
		for _, te := range t.equipment {
			if demoEquipment[te.equipmentIdx].clientIdx != t.clientIdx {
				return created, fmt.Errorf("заказ %q: оборудование %d чужого клиента", t.title, te.equipmentIdx)
			}
			if err := s.seedTaskEquipment(taskID, equipIDs[te.equipmentIdx], te); err != nil {
				return created, err
			}
			unitIDs = append(unitIDs, equipIDs[te.equipmentIdx])
			if mainEquipID == nil {
				mainEquipID = &equipIDs[te.equipmentIdx]
			}
		}
		if err := s.seedEquipmentLocation(i, taskID, client.ID, unitIDs, taskWarehouse, col.ID == s.lastColumnID); err != nil {
			return created, fmt.Errorf("перемещения заказа %q: %w", t.title, err)
		}

		for pos, st := range t.subtasks {
			var stAssigneeID *string
			if st.assigneeIdx >= 0 {
				stAssigneeID = &executors[st.assigneeIdx].ID
			}
			price := services[st.serviceIdx].defaultPrice(st.priceOverride)

			var subtaskID string
			var payout *string
			if st.payout != "" {
				payout = &st.payout
			}
			payoutPaid := st.payoutPaid && stAssigneeID != nil
			if err := s.tx.QueryRow(s.ctx,
				`INSERT INTO subtasks (task_id, service_id, status, assignee_id, due_date, price, equipment_id, position,
				                       executor_payout, is_executor_paid, executor_paid_at)
				 VALUES ($1, $2, $3, $4, $5::date, $6::numeric, $7, $8,
				         $9::numeric, $10, CASE WHEN $10 THEN now() - interval '1 day' END) RETURNING id`,
				taskID, services[st.serviceIdx].ID, st.status, stAssigneeID, dueDate(st.dueOffset), price, mainEquipID, pos,
				payout, payoutPaid).Scan(&subtaskID); err != nil {
				return created, err
			}

			for _, l := range st.logs {
				userID := s.adminID
				if l.userIdx >= 0 {
					userID = executors[l.userIdx].ID
				}
				if _, err := s.tx.Exec(s.ctx,
					`INSERT INTO subtask_logs (subtask_id, user_id, text, created_at, updated_at)
					 VALUES ($1, $2, $3, now() - make_interval(hours => $4), now() - make_interval(hours => $4))`,
					subtaskID, userID, l.text, l.hoursAgo); err != nil {
					return created, err
				}
			}

			goods := st.goodsOverride
			if goods == nil {
				goods = services[st.serviceIdx].Goods
			}
			for gPos, g := range goods {
				gPrice := g.price
				if gPrice == "" {
					gPrice = demoProducts[g.productIdx].price
				}
				if _, err := s.tx.Exec(s.ctx,
					`INSERT INTO subtask_products (subtask_id, product_id, qty, price, position, warehouse_id) VALUES ($1, $2, $3::numeric, $4::numeric, $5, $6)`,
					subtaskID, products[g.productIdx].ID, g.qty, gPrice, gPos+1, taskWarehouse); err != nil {
					return created, err
				}
			}
		}

		// StockEngine.Reconcile — единственный путь списания/резерва (не дублируется прямым
		// UPDATE products): консистентно с design-sql.md §Reconcile Query и рабочим кодом
		// TaskService.Move/Create (ADR-104). Для не-consumes колонок — no-op (нет дельт).
		if _, err := s.engine.Reconcile(s.ctx, s.tx, taskID, col.ConsumesStock, s.adminID); err != nil {
			return created, fmt.Errorf("reconcile task %d: %w", taskID, err)
		}

		created++
	}
	return created, nil
}

// seedTaskEquipment — единица в заказе. Местоположение единицы добавление в заказ не меняет
// (transfers-partners DS-045) — его ведут документы (seedEquipmentLocation).
func (s *seeder) seedTaskEquipment(taskID int64, equipmentID string, te demoTaskEquipment) error {
	_, err := s.tx.Exec(s.ctx,
		`INSERT INTO task_equipment (task_id, equipment_id, completeness, condition) VALUES ($1, $2, $3, $4)`,
		taskID, equipmentID, te.completeness, te.condition)
	return err
}

// seedEquipmentLocation — документы «Перемещение» заказа и итоговое местоположение его
// оборудования (transfers-partners DS-013, правило DS-054):
//   - единицы, ещё числящиеся «у клиента», принимаются на склад warehouseID проведённым
//     документом «клиент → склад»;
//   - заказ в последней колонке доски (completed) — те же единицы возвращены клиенту
//     проведённым документом «склад → клиент»: снова «у клиента»;
//   - demoTransitTask — первая единица со склада «Алматы» встаёт строкой в документ «в пути»
//     Алматы → Москва (s.transitTransferID);
//   - demoReturnTask — последняя единица отправлена клиенту документом «склад → клиент»,
//     который ещё не принят: «в пути → клиент».
func (s *seeder) seedEquipmentLocation(taskIdx int, taskID int64, clientID string, unitIDs []string, warehouseID string, completed bool) error {
	if len(unitIDs) == 0 {
		return nil
	}
	// единица может стоять и в более раннем незавершённом заказе — тогда она уже на складе.
	rows, err := s.tx.Query(s.ctx, `
		SELECT id::text FROM equipment
		WHERE id = ANY($1::uuid[]) AND location_status = 'at_client' AND active_transfer_id IS NULL ORDER BY id`, unitIDs)
	if err != nil {
		return err
	}
	var intake []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		intake = append(intake, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	if len(intake) > 0 {
		if _, err := s.clientTransfer("client_to_warehouse", taskID, clientID, warehouseID, "received", "6 days", intake); err != nil {
			return err
		}
		if _, err := s.tx.Exec(s.ctx, `
			UPDATE equipment SET location_status = 'at_warehouse', warehouse_id = $2 WHERE id = ANY($1::uuid[])`,
			intake, warehouseID); err != nil {
			return err
		}
		if completed {
			if _, err := s.clientTransfer("warehouse_to_client", taskID, clientID, warehouseID, "received", "1 day", intake); err != nil {
				return err
			}
			if _, err := s.tx.Exec(s.ctx, `
				UPDATE equipment SET location_status = 'at_client', warehouse_id = NULL WHERE id = ANY($1::uuid[])`, intake); err != nil {
				return err
			}
		}
	}

	if taskIdx == demoTransitTask {
		tag, err := s.tx.Exec(s.ctx, `
			UPDATE equipment SET location_status = 'in_transit', warehouse_id = NULL, active_transfer_id = $2
			WHERE id = $1 AND location_status = 'at_warehouse' AND warehouse_id = $3 AND active_transfer_id IS NULL`,
			unitIDs[0], s.transitTransferID, s.almatyID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 1 { // единица не на «Алматы» — в документ не встаёт
			if _, err := s.tx.Exec(s.ctx, `
				INSERT INTO transfer_items (transfer_id, kind, equipment_id, position)
				VALUES ($1, 'equipment', $2, (SELECT coalesce(max(position), 0) + 1 FROM transfer_items WHERE transfer_id = $1))`,
				s.transitTransferID, unitIDs[0]); err != nil {
				return err
			}
		}
	}

	if taskIdx == demoReturnTask {
		// отменённая заявка на возврат (transfers-revert TS-F1): на учёт не влияет, удаляется
		// вместе с заказом; местоположение единицы не меняет.
		if err := s.cancelledReturn(taskID, clientID, warehouseID, unitIDs[0]); err != nil {
			return err
		}
	}

	if taskIdx == demoReturnTask && !completed {
		unit := unitIDs[len(unitIDs)-1]
		var onStock bool
		if err := s.tx.QueryRow(s.ctx, `
			SELECT EXISTS (SELECT 1 FROM equipment WHERE id = $1 AND location_status = 'at_warehouse'
			               AND warehouse_id = $2 AND active_transfer_id IS NULL)`, unit, warehouseID).Scan(&onStock); err != nil {
			return err
		}
		if onStock {
			docID, err := s.clientTransfer("warehouse_to_client", taskID, clientID, warehouseID, "in_transit", "3 hours", []string{unit})
			if err != nil {
				return err
			}
			if _, err := s.tx.Exec(s.ctx, `
				UPDATE equipment SET location_status = 'in_transit', warehouse_id = NULL, active_transfer_id = $2 WHERE id = $1`,
				unit, docID); err != nil {
				return err
			}
		}
	}
	return nil
}

// cancelledReturn — отменённая (не отправленная) заявка «склад → клиент» заказа с одной единицей.
func (s *seeder) cancelledReturn(taskID int64, clientID, warehouseID, unit string) error {
	var id int64
	if err := s.tx.QueryRow(s.ctx, `
		INSERT INTO transfers (direction, from_warehouse_id, task_id, client_id, status, note, created_by,
		                       cancelled_by, cancelled_at, created_at, updated_at, version)
		VALUES ('warehouse_to_client', $1, $2, $3, 'cancelled', 'Заявка на возврат создана по ошибке', $4,
		        $4, now() - interval '2 days', now() - interval '2 days 1 hour', now() - interval '2 days', 2)
		RETURNING id`, warehouseID, taskID, clientID, s.adminID).Scan(&id); err != nil {
		return err
	}
	_, err := s.tx.Exec(s.ctx, `
		INSERT INTO transfer_items (transfer_id, kind, equipment_id, position) VALUES ($1, 'equipment', $2, 1)`, id, unit)
	return err
}

// clientTransfer — документ с клиентом по заказу со строками-единицами: direction —
// client_to_warehouse (warehouseID — склад назначения) либо warehouse_to_client (склад
// отправления); status — received либо in_transit; ago — давность создания (interval).
// Клиент документа — клиент заказа (DS-012). Местоположение единиц правит вызывающий.
func (s *seeder) clientTransfer(direction string, taskID int64, clientID, warehouseID, status, ago string, unitIDs []string) (int64, error) {
	var from, to *string
	note := "Возврат оборудования клиенту после ремонта"
	if direction == "client_to_warehouse" {
		to = &warehouseID
		note = "Приём оборудования клиента в ремонт"
	} else {
		from = &warehouseID
	}
	var id int64
	if err := s.tx.QueryRow(s.ctx, `
		INSERT INTO transfers (direction, from_warehouse_id, to_warehouse_id, task_id, client_id, status, note, created_by,
		                       sent_by, sent_at, received_by, received_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8, now() - $9::interval,
		        CASE WHEN $6 = 'received' THEN $8::uuid END, CASE WHEN $6 = 'received' THEN now() - $9::interval END,
		        now() - $9::interval, now() - $9::interval)
		RETURNING id`, direction, from, to, taskID, clientID, status, note, s.adminID, ago).Scan(&id); err != nil {
		return 0, err
	}
	for pos, unit := range unitIDs {
		if _, err := s.tx.Exec(s.ctx, `
			INSERT INTO transfer_items (transfer_id, kind, equipment_id, position) VALUES ($1, 'equipment', $2, $3)`,
			id, unit, pos+1); err != nil {
			return 0, err
		}
	}
	return id, nil
}

// demoInvoices — накладные клиенту (transfers-sales TS-F1): товар с ценой продажи в документе
// «склад → клиент» заказа. status / paid — какой документ дополняется: полученный в оплаченном
// заказе и «в пути» в неоплаченном; price "" — цена каталога (DS-039), иначе правленая в
// документе.
var demoInvoices = []struct {
	status     string
	paid       bool
	productIdx int
	qty, price string
}{
	// товары с большим остатком на обоих складах, уже затронутые демо-перемещениями — остаток
	// не уходит в минус, на каком бы складе ни лежало оборудование заказа.
	{status: "received", paid: true, productIdx: 2, qty: "5"},
	{status: "in_transit", paid: false, productIdx: 5, qty: "2", price: "250.00"},
}

// seedInvoices — товарная строка с ценой в существующих документах «склад → клиент»
// (seedEquipmentLocation): обе накладные отправлены, поэтому товар списан со склада
// отправления (client_out; остаток правится той же дельтой, что и журнал — как
// transfers.Service) и их сумма входит в итог заказа (DS-014).
func (s *seeder) seedInvoices(products []productRow) error {
	for _, inv := range demoInvoices {
		var docID int64
		var warehouseID string
		err := s.tx.QueryRow(s.ctx, `
			SELECT t.id, t.from_warehouse_id::text FROM transfers t JOIN tasks k ON k.id = t.task_id
			WHERE t.direction = 'warehouse_to_client' AND t.status = $1 AND k.is_paid = $2
			ORDER BY t.id LIMIT 1`, inv.status, inv.paid).Scan(&docID, &warehouseID)
		if err != nil {
			return fmt.Errorf("invoice %s (paid=%v): %w", inv.status, inv.paid, err)
		}
		productID := products[inv.productIdx].ID
		var price *string
		if inv.price != "" {
			price = &inv.price
		}
		if _, err := s.tx.Exec(s.ctx, `
			INSERT INTO transfer_items (transfer_id, kind, product_id, position, qty_sent, price)
			VALUES ($1, 'product', $2::uuid,
			        (SELECT coalesce(max(position), 0) + 1 FROM transfer_items WHERE transfer_id = $1),
			        $3::numeric, coalesce($4::numeric, (SELECT p.price FROM products p WHERE p.id = $2::uuid)))`,
			docID, productID, inv.qty, price); err != nil {
			return err
		}
		if _, err := s.tx.Exec(s.ctx, `UPDATE transfers SET note = 'Возврат после ремонта + расходные материалы' WHERE id = $1`, docID); err != nil {
			return err
		}
		if err := s.moveStock(productID, warehouseID, "-"+inv.qty, "client_out", docID); err != nil {
			return err
		}
	}
	return nil
}

// defaultPrice — цена подзадачи (prototype newSubtask: `price: svc.price`, extra.price
// переопределяет): priceOverride, если задан, иначе price услуги ("" → NULL, «по факту»).
func (r serviceRow) defaultPrice(override string) any {
	price := override
	if price == "" {
		price = r.Price
	}
	if price == "" {
		return nil
	}
	return price
}
