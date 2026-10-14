package clients

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kitavrus/kanban-tree/pkg/sqlutil"
)

// Client — карточка клиента (repo-модель). Version — оптимистичная блокировка (D28),
// только у clients (у equipment_types её нет).
type Client struct {
	ID              string
	CompanyName     string
	Address         string
	ContactName     string
	ContactPosition string
	Phones          []PhoneDTO
	EquipmentCount  int
	Version         int64
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type Repo struct {
	pool *pgxpool.Pool
}

func NewRepo(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

var (
	ErrNotFound        = pgx.ErrNoRows
	errVersionConflict = errors.New("clients: version conflict")
	// errClientGone — 23503 на equipment.client_id (клиент удалён параллельно с созданием
	// единицы оборудования, ADR из design-sql.md `## Create Equipment Query`).
	errClientGone = errors.New("clients: client gone")
	// errTypeGone — 23503 на equipment.type_id (тип удалён параллельно между TypeActive-
	// проверкой в сервисе и вставкой/обновлением строки, code review LOW#1): сервис маппит
	// в поле `type_id: not_found`, а не в 404/500 клиента.
	errTypeGone = errors.New("clients: type gone")
	// errNotDeleted — восстановление действующей записи (409 NOT_DELETED, DS-024).
	errNotDeleted = errors.New("clients: not deleted")
	// errClientDeleted — восстановление единицы, чей клиент в удалённых (409 CLIENT_DELETED,
	// DS-015).
	errClientDeleted = errors.New("clients: client deleted")
	// errInUse — служебная физическая уборка: на запись ссылаются заказы либо строки
	// документов (409 CONFLICT, DS-027).
	errInUse = errors.New("clients: record in use")
)

// mapEquipmentFKViolation — 23503 на equipment различает нарушенный constraint по имени:
// client_id (клиент удалён параллельно) → errClientGone, type_id (тип удалён параллельно)
// → errTypeGone. Раньше любой 23503 в CreateEquipment трактовался как errClientGone, а в
// UpdateEquipment не обрабатывался вовсе — оба случая уходили в 500/неверный 404
// (code review LOW#1).
func mapEquipmentFKViolation(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23503" {
		return err
	}
	switch pgErr.ConstraintName {
	case "equipment_client_id_fkey":
		return errClientGone
	case "equipment_type_id_fkey":
		return errTypeGone
	default:
		return err
	}
}

// Equipment — единица оборудования клиента (repo-модель, JOIN с equipment_types).
type Equipment struct {
	ID           string
	ClientID     string
	TypeID       string
	TypeName     string
	TypeIsActive bool
	// TypeIsDeleted — тип единицы мягко удалён: единица остаётся действующей, тип показывается
	// с бейджем «удалён» (soft-delete-catalog §6.9, DS-014).
	TypeIsDeleted bool
	PlateNumber   string
	Brand         string
	Model         string
	SKU           string
	Serial        string
	Specs         []SpecDTO
	Note          string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// TypeOption — опция <select> формы единицы (ADR-102: только SQL-чтение equipment_types,
// без Go-импорта пакета equipmenttypes).
type TypeOption struct {
	ID       string
	Name     string
	SpecKeys []string
	IsActive bool
	// IsDeleted — текущий тип единицы мягко удалён (в выборе для других единиц его нет).
	IsDeleted bool
}

const equipmentColumns = `e.id, e.client_id, e.type_id, t.name, t.is_active, e.plate_number, e.brand, e.model, e.sku, e.serial,
	e.specs, e.note, e.created_at, e.updated_at, t.deleted_at IS NOT NULL`

func scanEquipment(row pgx.Row) (*Equipment, error) {
	var e Equipment
	var specsRaw []byte
	if err := row.Scan(&e.ID, &e.ClientID, &e.TypeID, &e.TypeName, &e.TypeIsActive,
		&e.PlateNumber, &e.Brand, &e.Model, &e.SKU, &e.Serial, &specsRaw, &e.Note, &e.CreatedAt, &e.UpdatedAt, &e.TypeIsDeleted); err != nil {
		return nil, err
	}
	if len(specsRaw) > 0 {
		if err := json.Unmarshal(specsRaw, &e.Specs); err != nil {
			return nil, fmt.Errorf("clients: unmarshal specs: %w", err)
		}
	}
	return &e, nil
}

// ListParams — фильтры списка. Digits/DigitsAlt — цифры телефона для сравнения (ADR-003,
// searchDigits в service.go); "" значит соответствующее сравнение не применяется.
type ListParams struct {
	Q         string
	Digits    string
	DigitsAlt string
	// PlateLike — LIKE-шаблон госномера (sqlutil.PlatePattern); "" — не применяется.
	PlateLike     string
	Limit, Offset int
}

// phoneDigitsExpr — цифры телефона как есть (без модификаций).
const phoneDigitsExpr = `regexp_replace(p.phone, '\D', '', 'g')`

// phoneAltDigitsExpr — та же строка БЕЗ ведущей 7/8 (если она есть): у номера в БД
// отбрасывается СИММЕТРИЧНО тому же правилу, что применяется к запросу (`searchDigits`),
// НЕЗАВИСИМО от длины строки — раньше (итерация 1) БД-сторона стриплась только при полных
// ≥11 цифрах, а запрос — тоже только при ≥11, из-за чего частичный ввод с кодом страны
// («+7 999 555», «8 (999) 555») переставал находить совпадения (regression, code review
// итерация 2). Сравнивается ОБА варианта через OR ($2 — прямое совпадение цифр, $3 — без
// ведущей 7/8 с обеих сторон), поэтому "8999123" находит "+7 (999) 123-…", а полный
// "+79991234567"/"89991234567" находят друг друга при любой длине частичного ввода.
const phoneAltDigitsExpr = `regexp_replace(` + phoneDigitsExpr + `, '^[78]', '')`

// ListClients — список действующих клиентов (мягко удалённые не входят, soft-delete-clients
// DS-014; equipment_count — только действующие единицы) с equipment_count (без N+1: телефоны — один batch-запрос
// на страницу, design-sql.md `## Client Phones Batch Query`).
func (r *Repo) ListClients(ctx context.Context, p ListParams) ([]Client, int, error) {
	var args []any
	where := ""
	if p.Q != "" {
		likeQ := "%" + sqlutil.EscapeLike(p.Q) + "%"
		digitsLike := ""
		if p.Digits != "" {
			digitsLike = "%" + p.Digits + "%"
		}
		digitsAltLike := ""
		if p.DigitsAlt != "" {
			digitsAltLike = "%" + p.DigitsAlt + "%"
		}
		args = []any{likeQ, digitsLike, digitsAltLike, p.PlateLike}
		where = ` AND (c.company_name ILIKE $1 OR c.contact_name ILIKE $1
			OR ($4 <> '' AND EXISTS (SELECT 1 FROM equipment e
			           WHERE e.client_id = c.id AND e.deleted_at IS NULL
			             AND ` + sqlutil.PlateCompactSQL("e.plate_number") + ` LIKE $4))
			OR EXISTS (SELECT 1 FROM client_phones p
			           WHERE p.client_id = c.id
			             AND (p.phone ILIKE $1
			                  OR ($2 <> '' AND ` + phoneDigitsExpr + ` LIKE $2)
			                  OR ($3 <> '' AND ` + phoneAltDigitsExpr + ` LIKE $3))))`
	}

	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM clients c WHERE c.deleted_at IS NULL`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// company_name регистронезависимо, id вторым ключом — стабильный порядок при равенстве.
	q := `SELECT c.id, c.company_name, c.address, c.contact_name, c.contact_position,
	       c.version, c.created_at, c.updated_at,
	       (SELECT count(*) FROM equipment e WHERE e.client_id = c.id AND e.deleted_at IS NULL) AS equipment_count
	FROM clients c WHERE c.deleted_at IS NULL` + where +
		fmt.Sprintf(" ORDER BY lower(c.company_name), c.id LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2)
	rows, err := r.pool.Query(ctx, q, append(args, p.Limit, p.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := []Client{}
	ids := []string{}
	for rows.Next() {
		var c Client
		if err := rows.Scan(&c.ID, &c.CompanyName, &c.Address, &c.ContactName, &c.ContactPosition,
			&c.Version, &c.CreatedAt, &c.UpdatedAt, &c.EquipmentCount); err != nil {
			return nil, 0, err
		}
		out = append(out, c)
		ids = append(ids, c.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	phones, err := r.phonesBatch(ctx, ids)
	if err != nil {
		return nil, 0, err
	}
	for i := range out {
		out[i].Phones = phones[out[i].ID]
	}
	return out, total, nil
}

// phonesBatch — телефоны нескольких клиентов одним запросом (без N+1); отсутствующие
// в результате id — без телефонов (nil-срез, приводится к [] в toResponse).
func (r *Repo) phonesBatch(ctx context.Context, clientIDs []string) (map[string][]PhoneDTO, error) {
	res := make(map[string][]PhoneDTO, len(clientIDs))
	if len(clientIDs) == 0 {
		return res, nil
	}
	rows, err := r.pool.Query(ctx,
		`SELECT client_id, phone, note FROM client_phones
		 WHERE client_id = ANY($1::uuid[]) ORDER BY client_id, position`, clientIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var clientID string
		var p PhoneDTO
		if err := rows.Scan(&clientID, &p.Phone, &p.Note); err != nil {
			return nil, err
		}
		res[clientID] = append(res[clientID], p)
	}
	return res, rows.Err()
}

// GetClient — id уже провалидирован как uuid сервисом. Мягко удалённый клиент — ErrNotFound
// (DS-026).
func (r *Repo) GetClient(ctx context.Context, id string) (*Client, error) {
	var c Client
	err := r.pool.QueryRow(ctx,
		`SELECT c.id, c.company_name, c.address, c.contact_name, c.contact_position,
		        c.version, c.created_at, c.updated_at,
		        (SELECT count(*) FROM equipment e WHERE e.client_id = c.id AND e.deleted_at IS NULL)
		 FROM clients c WHERE c.id = $1::uuid AND c.deleted_at IS NULL`, id).
		Scan(&c.ID, &c.CompanyName, &c.Address, &c.ContactName, &c.ContactPosition,
			&c.Version, &c.CreatedAt, &c.UpdatedAt, &c.EquipmentCount)
	if err != nil {
		return nil, err
	}
	phones, err := r.phonesBatch(ctx, []string{id})
	if err != nil {
		return nil, err
	}
	c.Phones = phones[id]
	return &c, nil
}

// CreateClient — клиент + телефоны в одной транзакции.
func (r *Repo) CreateClient(ctx context.Context, in ClientInput) (*Client, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	c, err := insertClient(ctx, tx, in)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return c, nil
}

// insertClient — строка клиента + телефоны в транзакции вызывающего (общая для POST /clients
// и атомарного создания заказа с новым клиентом, CreateClientTx).
func insertClient(ctx context.Context, tx pgx.Tx, in ClientInput) (*Client, error) {
	var c Client
	err := tx.QueryRow(ctx,
		`INSERT INTO clients (company_name, address, contact_name, contact_position)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, company_name, address, contact_name, contact_position, version, created_at, updated_at`,
		in.CompanyName, in.Address, in.ContactName, in.ContactPosition).
		Scan(&c.ID, &c.CompanyName, &c.Address, &c.ContactName, &c.ContactPosition, &c.Version, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if err := insertPhones(ctx, tx, c.ID, in.Phones); err != nil {
		return nil, err
	}
	c.Phones = in.Phones
	c.EquipmentCount = 0
	return &c, nil
}

// UpdateClient — полная замена карточки + набора телефонов в транзакции (ADR-100/ADR-101).
// 0 строк на UPDATE (version не совпал ИЛИ записи нет) → SELECT EXISTS различает случаи:
// запись есть → errVersionConflict, нет → ErrNotFound.
func (r *Repo) UpdateClient(ctx context.Context, id string, version int64, in ClientInput) (*Client, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var c Client
	err = tx.QueryRow(ctx,
		`UPDATE clients SET company_name = $2, address = $3, contact_name = $4, contact_position = $5,
		        version = version + 1, updated_at = now()
		 WHERE id = $1::uuid AND version = $6 AND deleted_at IS NULL
		 RETURNING id, company_name, address, contact_name, contact_position, version, created_at, updated_at`,
		id, in.CompanyName, in.Address, in.ContactName, in.ContactPosition, version).
		Scan(&c.ID, &c.CompanyName, &c.Address, &c.ContactName, &c.ContactPosition, &c.Version, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		var exists bool
		if scanErr := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM clients WHERE id = $1::uuid AND deleted_at IS NULL)`, id).Scan(&exists); scanErr != nil {
			return nil, scanErr
		}
		if exists {
			return nil, errVersionConflict
		}
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	if _, err := tx.Exec(ctx, `DELETE FROM client_phones WHERE client_id = $1::uuid`, id); err != nil {
		return nil, err
	}
	if err := insertPhones(ctx, tx, id, in.Phones); err != nil {
		return nil, err
	}
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM equipment WHERE client_id = $1::uuid AND deleted_at IS NULL`, id).Scan(&c.EquipmentCount); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	c.Phones = in.Phones
	return &c, nil
}

// ClientExists — действующий клиент существует (мягко удалённый — false, DS-026); используется Phase 7 (создание/список оборудования проверяет клиента
// до запроса); заведена в этой фазе, т.к. часть Repo-интерфейса design-go-layers.md.
func (r *Repo) ClientExists(ctx context.Context, id string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM clients WHERE id = $1::uuid AND deleted_at IS NULL)`, id).Scan(&exists)
	return exists, err
}

// insertPhones — вставка телефонов unnest(...) WITH ORDINALITY (ADR-101): позиция —
// порядок ввода (0-based). Пустой список — no-op.
func insertPhones(ctx context.Context, tx pgx.Tx, clientID string, phones []PhoneDTO) error {
	if len(phones) == 0 {
		return nil
	}
	texts := make([]string, len(phones))
	notes := make([]string, len(phones))
	for i, p := range phones {
		texts[i] = p.Phone
		notes[i] = p.Note
	}
	_, err := tx.Exec(ctx,
		`INSERT INTO client_phones (client_id, position, phone, note)
		 SELECT $1::uuid, t.ord - 1, t.phone, t.note
		 FROM unnest($2::text[], $3::text[]) WITH ORDINALITY AS t(phone, note, ord)`,
		clientID, texts, notes)
	return err
}

// ListEquipment — оборудование клиента (JOIN с equipment_types для type_name/type_is_active,
// ADR-102). Сервис проверяет существование клиента до вызова (design-sql.md).
func (r *Repo) ListEquipment(ctx context.Context, clientID string, limit, offset int) ([]Equipment, int, error) {
	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM equipment WHERE client_id = $1::uuid AND deleted_at IS NULL`, clientID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx,
		`SELECT `+equipmentColumns+`
		 FROM equipment e JOIN equipment_types t ON t.id = e.type_id
		 WHERE e.client_id = $1::uuid AND e.deleted_at IS NULL
		 ORDER BY e.created_at, e.id
		 LIMIT $2 OFFSET $3`, clientID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []Equipment{}
	for rows.Next() {
		e, err := scanEquipment(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *e)
	}
	return out, total, rows.Err()
}

// GetEquipment — id уже провалидирован как uuid сервисом. ErrNoRows → ErrNotFound; мягко
// удалённая единица — тоже (DS-026).
func (r *Repo) GetEquipment(ctx context.Context, id string) (*Equipment, error) {
	return scanEquipment(r.pool.QueryRow(ctx,
		`SELECT `+equipmentColumns+`
		 FROM equipment e JOIN equipment_types t ON t.id = e.type_id
		 WHERE e.id = $1::uuid AND e.deleted_at IS NULL`, id))
}

// TypeActive — ADR-004: ErrNoRows → тип не существует либо мягко удалён (сервис маппит в поле
// type_id not_found, soft-delete-catalog DS-014), false → тип в архиве (поле type_id archived).
func (r *Repo) TypeActive(ctx context.Context, typeID string) (bool, error) {
	var active bool
	err := r.pool.QueryRow(ctx, `SELECT is_active FROM equipment_types WHERE id = $1::uuid AND deleted_at IS NULL`, typeID).Scan(&active)
	return active, err
}

// lockLiveType — выбранный тип под FOR SHARE с условием «не удалён» до конца транзакции записи
// единицы: удаление типа (UPDATE строки) либо уже видно (errTypeGone → type_id: not_found),
// либо ждёт — новая ссылка на тип не появится «между проверкой и записью» (DS-014).
func lockLiveType(ctx context.Context, tx pgx.Tx, typeID string) error {
	var one int
	err := tx.QueryRow(ctx, `SELECT 1 FROM equipment_types WHERE id = $1::uuid AND deleted_at IS NULL FOR SHARE`, typeID).Scan(&one)
	if errors.Is(err, pgx.ErrNoRows) {
		return errTypeGone
	}
	return err
}

// CreateEquipment — вставка + повторное чтение (Get Equipment Query) ради type_name/
// type_is_active. 23503 различается по constraint (mapEquipmentFKViolation): client_id
// (клиент удалён параллельно) → errClientGone, type_id (тип удалён параллельно между
// TypeActive-проверкой сервиса и вставкой) → errTypeGone.
func (r *Repo) CreateEquipment(ctx context.Context, clientID string, in EquipmentInput) (*Equipment, error) {
	specsJSON, err := json.Marshal(in.Specs)
	if err != nil {
		return nil, fmt.Errorf("clients: marshal specs: %w", err)
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	id, err := insertEquipment(ctx, tx, clientID, in, specsJSON)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return r.GetEquipment(ctx, id)
}

// insertEquipment — блокировки «клиент → тип» и INSERT единицы в транзакции вызывающего
// (общая для POST /clients/:id/equipment и атомарного создания заказа, CreateEquipmentTx).
func insertEquipment(ctx context.Context, tx pgx.Tx, clientID string, in EquipmentInput, specsJSON []byte) (string, error) {
	// soft-delete-clients DS-015: клиент читается FOR KEY SHARE с условием «не удалён» и
	// держится до коммита — удаление клиента (FOR UPDATE) либо уже видно (errClientGone → 404),
	// либо ждёт вставку и забирает новую единицу в удалённые вместе с клиентом.
	var one int
	err := tx.QueryRow(ctx, `SELECT 1 FROM clients WHERE id = $1::uuid AND deleted_at IS NULL FOR KEY SHARE`, clientID).Scan(&one)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", errClientGone
	}
	if err != nil {
		return "", err
	}
	// порядок блокировок: клиент → тип.
	if err := lockLiveType(ctx, tx, in.TypeID); err != nil {
		return "", err
	}
	var id string
	err = tx.QueryRow(ctx,
		`INSERT INTO equipment (client_id, type_id, brand, model, sku, serial, specs, note, plate_number)
		 VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7::jsonb, $8, $9)
		 RETURNING id`,
		clientID, in.TypeID, in.Brand, in.Model, in.SKU, in.Serial, specsJSON, in.Note, in.PlateNumber).Scan(&id)
	if err != nil {
		return "", mapEquipmentFKViolation(err)
	}
	return id, nil
}

// UpdateEquipment — полная замена (LWW, без version). check — валидация архивного типа
// (ADR-004): архивный допустим, только если type_id не менялся; вызывается ВНУТРИ tx с
// текущим type_id единицы, чтобы избежать TOCTOU-гонки со сменой типа. ErrNoRows → ErrNotFound.
func (r *Repo) UpdateEquipment(ctx context.Context, id string, in EquipmentInput, check func(curTypeID string) error) (*Equipment, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var curTypeID string
	if err := tx.QueryRow(ctx, `SELECT type_id FROM equipment WHERE id = $1::uuid AND deleted_at IS NULL`, id).Scan(&curTypeID); err != nil {
		return nil, err
	}
	if err := check(curTypeID); err != nil {
		return nil, err
	}
	// Смена типа — новая ссылка: тип-цель под FOR SHARE и не удалён. Сохранение единицы без
	// смены уже удалённого типа разрешено (soft-delete-catalog §4.4).
	if curTypeID != in.TypeID {
		if err := lockLiveType(ctx, tx, in.TypeID); err != nil {
			return nil, err
		}
	}

	specsJSON, err := json.Marshal(in.Specs)
	if err != nil {
		return nil, fmt.Errorf("clients: marshal specs: %w", err)
	}
	var updatedID string
	err = tx.QueryRow(ctx,
		`UPDATE equipment SET type_id = $2::uuid, brand = $3, model = $4, sku = $5, serial = $6,
		        specs = $7::jsonb, note = $8, plate_number = $9, updated_at = now()
		 WHERE id = $1::uuid AND deleted_at IS NULL RETURNING id`,
		id, in.TypeID, in.Brand, in.Model, in.SKU, in.Serial, specsJSON, in.Note, in.PlateNumber).Scan(&updatedID)
	if err != nil {
		// 23503 на type_id (тип удалён параллельно между check(curTypeID) выше и этим
		// UPDATE) — раньше уходило в 500 (code review LOW#1), теперь errTypeGone → сервис
		// мапит в поле type_id: not_found, как при создании.
		return nil, mapEquipmentFKViolation(err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return r.GetEquipment(ctx, updatedID)
}

// TypeOptions — опции формы единицы (ADR-102): активные + текущий тип единицы (может быть
// архивным либо мягко удалённым — правка без смены такого типа допустима; удалённый отдаётся с
// признаком IsDeleted и в выборе других единиц отсутствует). currentTypeID == "" — новая единица.
func (r *Repo) TypeOptions(ctx context.Context, currentTypeID string) ([]TypeOption, error) {
	var curArg any
	if currentTypeID != "" {
		curArg = currentTypeID
	}
	rows, err := r.pool.Query(ctx,
		`SELECT id, name, spec_keys, is_active, deleted_at IS NOT NULL FROM equipment_types
		 WHERE (is_active AND deleted_at IS NULL) OR id = $1::uuid
		 ORDER BY lower(name), id`, curArg)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []TypeOption{}
	for rows.Next() {
		var t TypeOption
		if err := rows.Scan(&t.ID, &t.Name, &t.SpecKeys, &t.IsActive, &t.IsDeleted); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// LookupEquipmentByPlate — действующие единицы действующих клиентов с ТОЧНЫМ совпадением
// госномера без учёта пробелов/дефисов (compact — sqlutil.PlateCompact, верхний регистр;
// в БД госномер хранится нормализованным). Свежие — первыми.
func (r *Repo) LookupEquipmentByPlate(ctx context.Context, compact string, limit, offset int) ([]EquipmentLookupItem, int, error) {
	const where = ` FROM equipment e
		 JOIN clients c ON c.id = e.client_id AND c.deleted_at IS NULL
		 JOIN equipment_types t ON t.id = e.type_id
		 WHERE e.deleted_at IS NULL AND e.plate_number <> '' AND `
	cond := sqlutil.PlateCompactSQL("e.plate_number") + ` = $1`
	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*)`+where+cond, compact).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx,
		`SELECT e.id, e.client_id, c.company_name, e.type_id, t.name, e.plate_number, e.brand, e.model, e.serial`+
			where+cond+` ORDER BY e.created_at DESC, e.id LIMIT $2 OFFSET $3`, compact, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []EquipmentLookupItem{}
	for rows.Next() {
		var it EquipmentLookupItem
		if err := rows.Scan(&it.ID, &it.ClientID, &it.ClientName, &it.TypeID, &it.TypeName,
			&it.PlateNumber, &it.Brand, &it.Model, &it.Serial); err != nil {
			return nil, 0, err
		}
		out = append(out, it)
	}
	return out, total, rows.Err()
}
