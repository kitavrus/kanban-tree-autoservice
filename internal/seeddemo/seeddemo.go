// Package seeddemo — «make seed-demo»: заполняет пустую БД демо-данными автосервиса с
// упором на ремонт двигателей (мастера: моторист, диагност, автоэлектрик, мастер-приёмщик;
// частные клиенты и компании с автопарком; типы авто и автомобили с госномером и VIN;
// каталог работ и запчастей с остатками; заказы во всех колонках доски с услугами в разных
// статусах, журналом мастеров, выплатами и списаниями через StockEngine.Reconcile; фото и
// видео приёмки/дефектовки во вложениях заказов) одной командой (AC-019, ADR-105).
//
// Guard — count(tasks)=0, иначе no-op с сообщением (SD-003); нужен активный администратор
// (role='admin' AND is_active) — иначе ErrNoAdmin. Существующий admin не изменяется,
// демо-мастера заводятся ON CONFLICT (login) DO NOTHING.
//
// Автомобиль клиента хранится госномером — equipment.plate_number («А123ВС 777»); заказ —
// ровно одна машина клиента (tasks.car_id, one-car-order), её марка/модель/госномер — в
// названии заказа. Марка/модель/VIN/характеристики демо-данных используются лишь для
// генерации медиа и текстов заказов.
//
// План работ заказов (order-start-duration) — от «сегодня» по Алматы (pkg/timezone.Default,
// пояс календаря загрузки у seed-админа): start_date/duration_days, выданным — finished_at не
// раньше начала; created_at, журнал и вложения — внутри окна «приёмка … выдача».
//
// Вложения (task_attachments) создаются, только если передан FileStore (WithFiles): файлы
// пишутся в него так же, как при загрузке через UI (ключ — uuid, в БД — метаданные). Фото —
// JPEG, видео — MP4/H.264, оба генерируются программно (media.go, mp4.go). Если транзакция
// не зафиксирована, записанные файлы удаляются.
//
// Склад один — остаток живёт в товаре (products.stock), стартовый остаток пишется движением
// manual; task_stock заполняет StockEngine.Reconcile, а не seed напрямую. Поставщики —
// справочник: два — действующий и архивный.
//
// Выплаты мастерам (executor-payouts, D71): часть выполненных услуг отмечена выплаченной с
// суммой выплаты, часть — с суммой, но без отметки, часть — без суммы.
package seeddemo

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/kitavrus/kanban-tree/internal/features/board"
	"github.com/kitavrus/kanban-tree/internal/filestore"
	"github.com/kitavrus/kanban-tree/pkg/timezone"
)

// ErrNoAdmin — в БД нет ни одного активного администратора; демо-данным нужен его id
// (created_by заказов, user_id движений склада). Типовое лечение — make local-up-seed
// (создаёт seed-админа) ДО make seed-demo.
var ErrNoAdmin = errors.New("нет активного администратора — сначала выполните make local-up-seed")

// demoPassword — пароль демо-мастеров (аналог cmd/seed: секрет не для прод, только
// локальный/демо-стенд).
const demoPassword = "demo12345"

// Result — сводка по демо-данным (AC-019). Skipped=true — в БД уже были заказы, ничего
// не создано (идемпотентность повторного запуска, SD-003).
type Result struct {
	Skipped     bool
	Message     string
	Executors   int
	Clients     int
	Equipment   int
	Products    int
	Services    int
	Tasks       int
	Attachments int
	Videos      int
}

// Option — настройка прогона.
type Option func(*options)

type options struct{ files filestore.FileStore }

// WithFiles — хранилище вложений (UPLOADS_DIR сервера): заказы получают фото/видео
// приёмки. Без него вложения не создаются.
func WithFiles(fs filestore.FileStore) Option { return func(o *options) { o.files = fs } }

// seedDemoLockKey — произвольный bigint для pg_advisory_xact_lock (LOW-3, code review):
// сериализует конкурентные «make seed-demo», чтобы guard count(tasks)=0 выполнялся в ТОЙ
// ЖЕ транзакции, что и запись демо-данных, а не отдельным запросом до BEGIN (TOCTOU).
const seedDemoLockKey = 8_721_394_651

// Run — точка входа cmd/seed-demo. Guard-порядок — design-sql.md §Seed Demo Guard Query:
// сперва count(tasks), затем активный admin; оба — под advisory-локом внутри одной
// транзакции записи (Lock Order columns→tasks→products соблюдает StockEngine.Reconcile,
// ADR-104).
func Run(ctx context.Context, pool *pgxpool.Pool, opts ...Option) (res Result, err error) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	if err := validateDemoData(); err != nil {
		return Result{}, fmt.Errorf("seeddemo: %w", err)
	}

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

	sd := &seeder{ctx: ctx, tx: tx, adminID: adminID, engine: board.NewStockEngine(), files: o.files}
	// Файлы пишутся до COMMIT: при любой ошибке они удаляются (иначе остались бы сиротами
	// до прохода сборщика UPLOADS_GC).
	defer func() {
		if err != nil {
			sd.removeWritten()
		}
	}()
	res, err = sd.run()
	if err != nil {
		return Result{}, err
	}
	if err = tx.Commit(ctx); err != nil {
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
	files   filestore.FileStore
	written []string // ключи файлов, записанных в FileStore этим прогоном
}

func (s *seeder) removeWritten() {
	ctx := context.WithoutCancel(s.ctx)
	for _, k := range s.written {
		_ = s.files.Delete(ctx, k)
	}
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

	equip, err := s.seedEquipment(clients)
	if err != nil {
		return Result{}, fmt.Errorf("seeddemo: equipment: %w", err)
	}
	res.Equipment = len(equip)

	categoryIDs, err := s.seedCategories()
	if err != nil {
		return Result{}, fmt.Errorf("seeddemo: categories: %w", err)
	}

	products, err := s.seedProducts()
	if err != nil {
		return Result{}, fmt.Errorf("seeddemo: products: %w", err)
	}
	res.Products = len(products)

	if err := s.seedSuppliers(); err != nil {
		return Result{}, fmt.Errorf("seeddemo: suppliers: %w", err)
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

	if err := s.seedTasks(&res, columns, clients, executors, equip, services, products); err != nil {
		return Result{}, fmt.Errorf("seeddemo: tasks: %w", err)
	}

	if err := s.seedDeletedTask(clients, equip["vesta"]); err != nil {
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
	if err := s.seedDeletedSupplier(); err != nil {
		return Result{}, fmt.Errorf("seeddemo: deleted supplier: %w", err)
	}
	return res, nil
}

// ---------------------------------------------------------------------------------------
// Проверка демо-данных до записи: госномера (только буквы, допустимые на номерах РФ), VIN
// (17 символов, без I/O/Q), ссылки заказов на авто/услуги/товары.
// ---------------------------------------------------------------------------------------

var (
	// PlateRe — российский госномер «А123ВС 777»: буквы АВЕКМНОРСТУХ (кириллица), регион
	// 2–3 цифры через пробел.
	PlateRe = regexp.MustCompile(`^[АВЕКМНОРСТУХ]\d{3}[АВЕКМНОРСТУХ]{2} \d{2,3}$`)
	vinRe   = regexp.MustCompile(`^[A-HJ-NPR-Z0-9]{17}$`)
)

func validateDemoData() error {
	keys := map[string]demoEquipmentItem{}
	for _, e := range demoEquipment {
		if !PlateRe.MatchString(e.plate) {
			return fmt.Errorf("авто %s: неверный госномер %q", e.key, e.plate)
		}
		if e.typeIdx != typeEngine && !vinRe.MatchString(e.serial) {
			return fmt.Errorf("авто %s: неверный VIN %q", e.key, e.serial)
		}
		keys[e.key] = e
	}
	skus := map[string]bool{}
	for _, p := range demoProducts {
		skus[p.sku] = true
	}
	svc := map[string]bool{}
	for _, sv := range demoServices {
		svc[sv.code] = true
		for _, gd := range sv.goods {
			if !skus[gd.sku] {
				return fmt.Errorf("услуга %s: нет товара %q", sv.code, gd.sku)
			}
		}
	}
	cars := map[string]bool{}
	for _, t := range demoTasks {
		e, ok := keys[t.car]
		if !ok || e.clientIdx != t.clientIdx {
			return fmt.Errorf("заказ %q: авто %q не найдено у клиента", t.title, t.car)
		}
		if cars[t.car] {
			return fmt.Errorf("заказ %q: у авто %q уже есть действующий заказ", t.title, t.car)
		}
		cars[t.car] = true
		if !strings.Contains(t.title, e.plate) || !strings.Contains(t.title, strings.Fields(e.model)[0]) {
			return fmt.Errorf("заказ %q: в названии нет модели/госномера авто %s %s", t.title, e.model, e.plate)
		}
		if err := validatePlan(t); err != nil {
			return fmt.Errorf("заказ %q: %w", t.title, err)
		}
		for _, st := range t.subtasks {
			if !svc[st.service] {
				return fmt.Errorf("заказ %q: нет услуги %q", t.title, st.service)
			}
			for _, gd := range st.goodsOverride {
				if !skus[gd.sku] {
					return fmt.Errorf("заказ %q: нет товара %q", t.title, gd.sku)
				}
			}
		}
	}
	return nil
}

// demoFinalColumn — финальная колонка демо-доски («Выдан», миграция 0005).
const demoFinalColumn = "Выдан"

// validatePlan — план работ заказа и время его событий (order-start-duration): дни 1..14;
// выданный — день выдачи в прошлом и не раньше начала; срок услуги — внутри плана; журнал,
// вложения и выплата — не раньше приёмки (acceptedH) и, у выданного, не позже выдачи
// (finished_at — день выдачи 17:30 по Алматы: запас +7 ч на время суток прогона).
func validatePlan(t demoTaskItem) error {
	if t.days < 1 || t.days > 14 {
		return fmt.Errorf("дней на работу %d вне 1..14", t.days)
	}
	if t.acceptedH < 1 {
		return fmt.Errorf("нет момента приёмки")
	}
	final := t.column == demoFinalColumn
	switch {
	case final && (t.finishedOffset > -1 || t.finishedOffset < t.startOffset):
		return fmt.Errorf("день выдачи %d: должен быть в прошлом и не раньше начала %d", t.finishedOffset, t.startOffset)
	case !final && t.finishedOffset != 0:
		return fmt.Errorf("день выдачи у невыданного заказа")
	}
	minAgo := 0
	if final {
		minAgo = -t.finishedOffset*24 + 7
	}
	check := func(what string, hoursAgo int) error {
		if hoursAgo > t.acceptedH || hoursAgo < minAgo {
			return fmt.Errorf("%s: %d ч назад — вне окна приёмки (%d ч) … выдачи", what, hoursAgo, t.acceptedH)
		}
		return nil
	}
	for _, m := range t.media {
		if err := check("вложение "+m.name, m.hoursAgo); err != nil {
			return err
		}
	}
	for _, st := range t.subtasks {
		if st.dueOffset != nil && (*st.dueOffset < t.startOffset || *st.dueOffset > t.startOffset+t.days-1) {
			return fmt.Errorf("услуга %s: срок %d вне плана %d..%d", st.service, *st.dueOffset, t.startOffset, t.startOffset+t.days-1)
		}
		if st.payoutPaid && t.acceptedH < 24 {
			return fmt.Errorf("услуга %s: выплата (вчера) раньше приёмки", st.service)
		}
		for _, l := range st.logs {
			if err := check("журнал "+st.service, l.hoursAgo); err != nil {
				return err
			}
		}
	}
	return nil
}

// ---------------------------------------------------------------------------------------
// Люди и автомобили.
// ---------------------------------------------------------------------------------------

type executorRow struct{ ID string }
type clientRow struct{ ID string }

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

func (s *seeder) seedClients() ([]clientRow, error) {
	out := make([]clientRow, 0, len(demoClients))
	for _, c := range demoClients {
		var id string
		if err := s.tx.QueryRow(s.ctx,
			`INSERT INTO clients (company_name) VALUES ($1) RETURNING id`,
			c.company).Scan(&id); err != nil {
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

// seedEquipment — автомобили; возвращает id по ключу demoEquipmentItem.key.
func (s *seeder) seedEquipment(clients []clientRow) (map[string]string, error) {
	out := make(map[string]string, len(demoEquipment))
	for _, e := range demoEquipment {
		var id string
		if err := s.tx.QueryRow(s.ctx,
			`INSERT INTO equipment (client_id, plate_number) VALUES ($1, $2) RETURNING id`,
			clients[e.clientIdx].ID, e.plate).Scan(&id); err != nil {
			return nil, err
		}
		out[e.key] = id
	}
	return out, nil
}

// ---------------------------------------------------------------------------------------
// Каталог.
// ---------------------------------------------------------------------------------------

type productRow struct{ ID, Price string }
type serviceRow struct {
	ID    string
	Price string      // "" — NULL («по факту»)
	Goods []protoGood // комплект по умолчанию
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

// seedProducts — товар (price/sku/unit/min_stock/box/place сразу) и стартовый остаток
// движением manual (Stock Adjust Query, D44) — как ручная правка на «Остатках» (нулевой
// остаток — без движения: stock_movements.qty_delta CHECK <> 0). Ключ — артикул.
func (s *seeder) seedProducts() (map[string]productRow, error) {
	out := make(map[string]productRow, len(demoProducts))
	for _, p := range demoProducts {
		var id string
		if err := s.tx.QueryRow(s.ctx,
			`INSERT INTO products (name, sku, unit, price, stock, min_stock, box, place)
			 VALUES ($1, $2, $3, $4::numeric, $5::numeric, $6::numeric, $7, $8) RETURNING id`,
			p.name, p.sku, p.unit, p.price, p.stock, p.minStock, p.box, p.place).Scan(&id); err != nil {
			return nil, err
		}
		if p.stock != "0" {
			if _, err := s.tx.Exec(s.ctx,
				`INSERT INTO stock_movements (product_id, qty_delta, reason, user_id) VALUES ($1::uuid, $2::numeric, 'manual', $3::uuid)`,
				id, p.stock, s.adminID); err != nil {
				return nil, err
			}
		}
		out[p.sku] = productRow{ID: id, Price: p.price}
	}
	return out, nil
}

// seedSuppliers — два поставщика (один в архиве); повторный прогон по тому же названию
// (suppliers_name_lower_uq — частичный, только неудалённые) строку не дублирует.
func (s *seeder) seedSuppliers() error {
	for _, sp := range demoSuppliers {
		if _, err := s.tx.Exec(s.ctx, `
			INSERT INTO suppliers (name, contact_name, phone, address, note, is_active)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (lower(btrim(name))) WHERE deleted_at IS NULL DO NOTHING`,
			sp.name, sp.contact, sp.phone, sp.address, sp.note, sp.active); err != nil {
			return err
		}
	}
	return nil
}

// seedServices — услуги + service_products (комплект по умолчанию); ключ — code.
func (s *seeder) seedServices(categoryIDs []string, products map[string]productRow) (map[string]serviceRow, error) {
	out := make(map[string]serviceRow, len(demoServices))
	for _, sv := range demoServices {
		var price any
		if sv.price != "" {
			price = sv.price
		}
		var id string
		if err := s.tx.QueryRow(s.ctx,
			`INSERT INTO services (category_id, name, price) VALUES ($1, $2, $3::numeric) RETURNING id`,
			categoryIDs[sv.categoryIdx], sv.name, price).Scan(&id); err != nil {
			return nil, err
		}
		for pos, gd := range sv.goods {
			if _, err := s.tx.Exec(s.ctx,
				`INSERT INTO service_products (service_id, product_id, qty, position) VALUES ($1, $2, $3::numeric, $4)`,
				id, products[gd.sku].ID, gd.qty, pos); err != nil {
				return nil, err
			}
		}
		out[sv.code] = serviceRow{ID: id, Price: sv.price, Goods: sv.goods}
	}
	return out, nil
}

// ---------------------------------------------------------------------------------------
// Доска.
// ---------------------------------------------------------------------------------------

type columnRow struct {
	ID            string
	IsFinal       bool
	ConsumesStock bool
}

// demoColumnNames — колонки по умолчанию засеяны миграцией 0005 (Сид D32); seeddemo их не
// переименовывает, только читает FOR UPDATE (Lock Order columns→tasks→products, ADR-104).
var demoColumnNames = []string{"Приёмка", "Диагностика", "Согласование", "Ждёт запчасти", "В ремонте", "Готов к выдаче", "Выдан"}

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
	for _, name := range demoColumnNames {
		if _, ok := out[name]; !ok {
			return nil, fmt.Errorf("seeddemo: колонка %q не найдена (миграция 0005 изменена?)", name)
		}
	}
	return out, nil
}

// dueDate — YYYY-MM-DD от сегодня (overdue/soon относительно текущей даты, не хардкод).
func dueDate(n *int) *string {
	if n == nil {
		return nil
	}
	d := dayFromToday(*n)
	return &d
}

// dayFromToday — YYYY-MM-DD через n дней от сегодня по Алматы (pkg/timezone.Default — пояс
// seed-админа: начало работ заказа, день выдачи, срок услуги).
func dayFromToday(n int) string {
	return time.Now().In(timezone.Load(timezone.Default)).AddDate(0, 0, n).Format(time.DateOnly)
}

// finishedDay — день выдачи заказа финальной колонки (nil — не выдан).
func finishedDay(t demoTaskItem, final bool) *string {
	if !final {
		return nil
	}
	d := dayFromToday(t.finishedOffset)
	return &d
}

func userID(executors []executorRow, idx int, adminID string) string {
	if idx >= 0 {
		return executors[idx].ID
	}
	return adminID
}

func (s *seeder) seedTasks(res *Result, columns map[string]columnRow, clients []clientRow, executors []executorRow,
	equip map[string]string, services map[string]serviceRow, products map[string]productRow) error {
	equipByKey := make(map[string]demoEquipmentItem, len(demoEquipment))
	for _, e := range demoEquipment {
		equipByKey[e.key] = e
	}
	for i, t := range demoTasks {
		col := columns[t.column]
		var assigneeID *string
		if t.assignee >= 0 {
			assigneeID = &executors[t.assignee].ID
		}
		// Заказ = ровно один автомобиль клиента (tasks.car_id, one-car-order). Выданный —
		// finished_at явно (день выдачи 17:30 по Алматы; триггер tasks_set_finished_at его не
		// перетирает), оплата — в момент выдачи; created_at — момент приёмки/записи.
		var taskID int64
		if err := s.tx.QueryRow(s.ctx, `
			WITH v AS (SELECT ($12::date + time '17:30') AT TIME ZONE 'Asia/Almaty' AS finished_at)
			INSERT INTO tasks (column_id, client_id, car_id, title, description_html, assignee_id, start_date, duration_days,
			                   is_paid, paid_at, finished_at, position, created_by, created_at, updated_at)
			SELECT $1, $2, $3, $4, $5, $6, $7::date, $11,
			       $8, CASE WHEN $8 THEN COALESCE(v.finished_at, now()) END, v.finished_at, $9::numeric, $10,
			       now() - make_interval(hours => $13), COALESCE(v.finished_at, now())
			FROM v RETURNING id`,
			col.ID, clients[t.clientIdx].ID, equip[t.car], t.title, t.descHTML, assigneeID, dayFromToday(t.startOffset), t.paid,
			fmt.Sprintf("%d", (i+1)*1024), s.adminID, t.days, finishedDay(t, col.IsFinal), t.acceptedH).
			Scan(&taskID); err != nil {
			return err
		}

		for pos, st := range t.subtasks {
			if err := s.seedSubtask(taskID, pos, st, executors, services, products); err != nil {
				return fmt.Errorf("заказ %q, услуга %s: %w", t.title, st.service, err)
			}
		}

		// StockEngine.Reconcile — единственный путь списания/резерва (как TaskService.Move/
		// Create, ADR-104). Для не-consumes колонок — no-op.
		if _, err := s.engine.Reconcile(s.ctx, s.tx, taskID, col.ConsumesStock, s.adminID); err != nil {
			return fmt.Errorf("reconcile task %d: %w", taskID, err)
		}

		if s.files != nil {
			car := equipByKey[t.car]
			for _, m := range t.media {
				if err := s.seedAttachment(taskID, car, m, executors); err != nil {
					return fmt.Errorf("заказ %q, вложение %q: %w", t.title, m.name, err)
				}
				res.Attachments++
				if m.video {
					res.Videos++
				}
			}
		}
		res.Tasks++
	}
	return nil
}

func (s *seeder) seedSubtask(taskID int64, pos int, st demoSubtaskItem, executors []executorRow,
	services map[string]serviceRow, products map[string]productRow) error {
	svc := services[st.service]
	var assigneeID *string
	if st.assignee >= 0 {
		assigneeID = &executors[st.assignee].ID
	}
	var price any
	if p := st.priceOverride; p != "" {
		price = p
	} else if svc.Price != "" {
		price = svc.Price
	}
	var payout *string
	if st.payout != "" {
		payout = &st.payout
	}
	payoutPaid := st.payoutPaid && assigneeID != nil && payout != nil

	var subtaskID string
	if err := s.tx.QueryRow(s.ctx,
		`INSERT INTO subtasks (task_id, service_id, status, assignee_id, due_date, price, position,
		                       executor_payout, is_executor_paid, executor_paid_at)
		 VALUES ($1, $2, $3, $4, $5::date, $6::numeric, $7,
		         $8::numeric, $9, CASE WHEN $9 THEN now() - interval '1 day' END) RETURNING id`,
		taskID, svc.ID, st.status, assigneeID, dueDate(st.dueOffset), price, pos,
		payout, payoutPaid).Scan(&subtaskID); err != nil {
		return err
	}

	for _, l := range st.logs {
		if _, err := s.tx.Exec(s.ctx,
			`INSERT INTO subtask_logs (subtask_id, user_id, text, created_at, updated_at)
			 VALUES ($1, $2, $3, now() - make_interval(hours => $4), now() - make_interval(hours => $4))`,
			subtaskID, userID(executors, l.user, s.adminID), l.text, l.hoursAgo); err != nil {
			return err
		}
	}

	goods := st.goodsOverride
	if goods == nil {
		goods = svc.Goods
	}
	for gPos, gd := range goods {
		gPrice := gd.price
		if gPrice == "" {
			gPrice = products[gd.sku].Price
		}
		if _, err := s.tx.Exec(s.ctx,
			`INSERT INTO subtask_products (subtask_id, product_id, qty, price, position) VALUES ($1, $2, $3::numeric, $4::numeric, $5)`,
			subtaskID, products[gd.sku].ID, gd.qty, gPrice, gPos+1); err != nil {
			return err
		}
	}
	return nil
}

// sceneFor — сцена вложения; для снятого агрегата «авто» — блок на стенде.
func sceneFor(m demoMedia, car demoEquipmentItem) sceneFn {
	switch m.scene {
	case mediaCar:
		if car.typeIdx == typeEngine {
			return sceneBlock(0)
		}
		return sceneCar(car.plate, car.specValue("Цвет"), car.typeIdx == typeCommercial)
	case mediaDash:
		mileage := strings.ToUpper(car.specValue("Пробег"))
		return sceneDash(m.codes, mileage)
	case mediaEngine:
		return sceneEngine(m.markX, m.markY, m.shake, m.steam)
	case mediaEndoscope:
		return sceneEndoscope("ЦИЛ. 3")
	case mediaDipstick:
		return sceneDipstick()
	case mediaGasket:
		return sceneGasket(2)
	case mediaTank:
		return sceneTank()
	case mediaBlock:
		return sceneBlock(1)
	}
	return sceneEngine(-1, 0, false, false)
}

// seedAttachment — файл в FileStore (ключ uuid, как при загрузке через UI) + строка
// task_attachments с метаданными.
func (s *seeder) seedAttachment(taskID int64, car demoEquipmentItem, m demoMedia, executors []executorRow) error {
	ident := car.plate
	if ident == "" {
		ident = "ДВС " + car.specValue("Код двигателя")
	}
	header := fmt.Sprintf("ЗАКАЗ #%d · %s", taskID, ident)
	sc := sceneFor(m, car)

	var data []byte
	kind, ctype, ext, limit := board.AttachmentKindPhoto, "image/jpeg", ".jpg", int64(board.AttachmentMaxPhoto)
	if m.video {
		kind, ctype, ext, limit = board.AttachmentKindVideo, "video/mp4", ".mp4", int64(board.AttachmentMaxVideo)
		data = renderVideo(header, m.caption, sc)
	} else {
		var err error
		if data, err = renderPhoto(header, m.caption, sc); err != nil {
			return err
		}
	}

	key := filestore.NewKey()
	n, err := s.files.Put(s.ctx, key, bytes.NewReader(data), limit)
	if err != nil {
		return err
	}
	s.written = append(s.written, key)
	name := m.name
	if car.plate != "" {
		name += " " + car.plate
	}
	_, err = s.tx.Exec(s.ctx,
		`INSERT INTO task_attachments (task_id, kind, original_name, content_type, size, storage_key, user_id, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, now() - make_interval(hours => $8))`,
		taskID, kind, name+ext, ctype, n, key, userID(executors, m.user, s.adminID), m.hoursAgo)
	return err
}

// ---------------------------------------------------------------------------------------
// Мягко удалённые записи — разделы «Удалённые» на демо-стенде непустые.
// ---------------------------------------------------------------------------------------

// seedDeletedTask — один мягко удалённый заказ с причиной (soft-delete-core TS-F5, DS-011);
// без услуг и товаров — возвращать на склад нечего; в Result.Tasks не входит. Автомобиль —
// та же Lada Vesta клиента (carID): дубль заказа, заведённый в день её приёмки (план — тот же
// день, 1 день) и удалённый позже.
func (s *seeder) seedDeletedTask(clients []clientRow, carID string) error {
	_, err := s.tx.Exec(s.ctx, `
		INSERT INTO tasks (column_id, client_id, car_id, title, description_html, position, created_by,
		                   start_date, duration_days, created_at, updated_at, deleted_at, deleted_by, delete_reason)
		SELECT c.id, $1::uuid, $3::uuid, 'Lada Vesta Е245КХ 797: диагностика (дубль)', '', 999424, $2::uuid,
		       $4::date, 1, now() - interval '119 hours', now() - interval '2 days',
		       now() - interval '2 days', $2::uuid, 'Создан по ошибке — дубль заказа'
		FROM columns c WHERE NOT c.is_final AND c.deleted_at IS NULL ORDER BY c.position, c.id LIMIT 1`,
		clients[0].ID, s.adminID, carID, dayFromToday(-5))
	return err
}

// demoDeletedColumnName — название мягко удалённой демо-колонки.
const demoDeletedColumnName = "Контроль качества"

// seedDeletedColumn — одна мягко удалённая пустая нефинальная колонка (soft-delete-board-
// columns, DS-011); позиция — за последней действующей. Повторный запуск вторую не создаёт.
func (s *seeder) seedDeletedColumn() error {
	_, err := s.tx.Exec(s.ctx, `
		INSERT INTO columns (name, color, is_final, consumes_stock, position, deleted_at, deleted_by, delete_reason)
		SELECT $1, '#6c757d', false, false,
		       (SELECT coalesce(max(position), 0) + 1 FROM columns WHERE deleted_at IS NULL),
		       now() - interval '1 day', $2::uuid, 'Этап объединён с «Готов к выдаче» — проверку делает приёмщик при выдаче'
		WHERE NOT EXISTS (SELECT 1 FROM columns WHERE name = $1 AND deleted_at IS NOT NULL)`,
		demoDeletedColumnName, s.adminID)
	return err
}

// demoDeletedClientName — мягко удалённый демо-клиент.
const demoDeletedClientName = "Тарасов Игорь Викторович (продал авто)"

// seedDeletedClient — клиент удалён вместе со своим авто (deleted_with_client = true, те же
// «кто, когда, почему»); ещё одно авто действующего клиента удалено отдельно (DS-015).
func (s *seeder) seedDeletedClient(clients []clientRow) error {
	const reason = "Клиент продал автомобиль, больше не обслуживается"
	var clientID string
	var at time.Time
	if err := s.tx.QueryRow(s.ctx, `
		INSERT INTO clients (company_name, deleted_at, deleted_by, delete_reason)
		VALUES ($1, now() - interval '3 days', $2::uuid, $3)
		RETURNING id, deleted_at`, demoDeletedClientName, s.adminID, reason).Scan(&clientID, &at); err != nil {
		return err
	}
	if _, err := s.tx.Exec(s.ctx,
		`INSERT INTO client_phones (client_id, position, phone, note) VALUES ($1, 0, '+7 916 200-10-20', 'мобильный')`, clientID); err != nil {
		return err
	}
	if _, err := s.tx.Exec(s.ctx, `
		INSERT INTO equipment (client_id, plate_number, deleted_at, deleted_by, delete_reason, deleted_with_client)
		VALUES ($1::uuid, 'О404ВК 799', $2, $3::uuid, $4, true)`,
		clientID, at, s.adminID, reason); err != nil {
		return err
	}
	_, err := s.tx.Exec(s.ctx, `
		INSERT INTO equipment (client_id, plate_number, deleted_at, deleted_by, delete_reason)
		VALUES ($1::uuid, 'Х112АМ 97',
		        now() - interval '5 days', $2::uuid, 'Автомобиль продан')`, clients[0].ID, s.adminID)
	return err
}

// Названия мягко удалённых записей каталога.
const (
	demoDeletedCategoryName = "Выездные услуги"
	demoDeletedProductName  = "Масло моторное 10W-40 минеральное, 4 л (снято с продажи)"
	demoDeletedServiceName  = "Выезд мастера на дом"
)

// seedDeletedCatalog — по одной мягко удалённой записи каждого вида каталога (soft-delete-
// catalog, DS-011, DS-009). Товар удалён из архива (is_active = false) и без остатка
// (DS-021); на удалённую категорию никто не ссылается.
func (s *seeder) seedDeletedCatalog() error {
	if _, err := s.tx.Exec(s.ctx, `
		INSERT INTO catalog_categories (kind, name, deleted_at, deleted_by, delete_reason)
		VALUES ('service', $1, now() - interval '6 days', $2::uuid, 'Выездные услуги больше не оказываем')`,
		demoDeletedCategoryName, s.adminID); err != nil {
		return err
	}
	if _, err := s.tx.Exec(s.ctx, `
		INSERT INTO products (name, sku, unit, price, is_active, deleted_at, deleted_by, delete_reason)
		VALUES ($1, 'MIN-10W40-4', 'шт', 1100, false, now() - interval '2 days', $2::uuid, 'Минеральные масла больше не продаём')`,
		demoDeletedProductName, s.adminID); err != nil {
		return err
	}
	_, err := s.tx.Exec(s.ctx, `
		INSERT INTO services (category_id, name, price, deleted_at, deleted_by, delete_reason)
		SELECT c.id, $1, 2500, now() - interval '4 days', $2::uuid, 'Услугу больше не оказываем'
		FROM catalog_categories c WHERE c.kind = 'service' AND c.deleted_at IS NULL ORDER BY lower(c.name), c.id LIMIT 1`,
		demoDeletedServiceName, s.adminID)
	return err
}

// demoDeletedSupplierName — мягко удалённый демо-поставщик.
const demoDeletedSupplierName = "ТехИмпорт"

// seedDeletedSupplier — одна мягко удалённая запись поставщика (soft-delete-suppliers,
// DS-011). Повторный запуск вторую удалённую запись не создаёт.
func (s *seeder) seedDeletedSupplier() error {
	_, err := s.tx.Exec(s.ctx, `
		INSERT INTO suppliers (name, contact_name, phone, note, deleted_at, deleted_by, delete_reason)
		SELECT $1, 'Ким Олег', '+7 495 250-40-40', 'Разовая закупка',
		       now() - interval '3 days', $2::uuid, 'Заведён дублем — работаем через «АвтоДеталь-Опт»'
		WHERE NOT EXISTS (SELECT 1 FROM suppliers WHERE name = $1 AND deleted_at IS NOT NULL)`,
		demoDeletedSupplierName, s.adminID)
	return err
}
