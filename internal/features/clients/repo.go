package clients

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kitavrus/kanban-tree/pkg/sqlutil"
)

// Client — карточка клиента (repo-модель). Version — оптимистичная блокировка (D28).
type Client struct {
	ID             string
	CompanyName    string
	Phones         []PhoneDTO
	EquipmentCount int
	Version        int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
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
	// errNotDeleted — восстановление действующей записи (409 NOT_DELETED, DS-024).
	errNotDeleted = errors.New("clients: not deleted")
	// errClientDeleted — восстановление единицы, чей клиент в удалённых (409 CLIENT_DELETED,
	// DS-015).
	errClientDeleted = errors.New("clients: client deleted")
	// errInUse — служебная физическая уборка: на запись ссылаются заказы либо строки
	// документов (409 CONFLICT, DS-027).
	errInUse = errors.New("clients: record in use")
)

// mapEquipmentFKViolation — 23503 на equipment.client_id (клиент удалён параллельно) →
// errClientGone (code review LOW#1).
func mapEquipmentFKViolation(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23503" {
		return err
	}
	switch pgErr.ConstraintName {
	case "equipment_client_id_fkey":
		return errClientGone
	default:
		return err
	}
}

// Equipment — автомобиль клиента (repo-модель): только госномер.
type Equipment struct {
	ID          string
	ClientID    string
	PlateNumber string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

const equipmentColumns = `e.id, e.client_id, e.plate_number, e.created_at, e.updated_at`

func scanEquipment(row pgx.Row) (*Equipment, error) {
	var e Equipment
	if err := row.Scan(&e.ID, &e.ClientID, &e.PlateNumber, &e.CreatedAt, &e.UpdatedAt); err != nil {
		return nil, err
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
		where = ` AND (c.company_name ILIKE $1
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
	q := `SELECT c.id, c.company_name,
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
		if err := rows.Scan(&c.ID, &c.CompanyName,
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
		`SELECT c.id, c.company_name,
		        c.version, c.created_at, c.updated_at,
		        (SELECT count(*) FROM equipment e WHERE e.client_id = c.id AND e.deleted_at IS NULL)
		 FROM clients c WHERE c.id = $1::uuid AND c.deleted_at IS NULL`, id).
		Scan(&c.ID, &c.CompanyName,
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
		`INSERT INTO clients (company_name)
		 VALUES ($1)
		 RETURNING id, company_name, version, created_at, updated_at`,
		in.CompanyName).
		Scan(&c.ID, &c.CompanyName, &c.Version, &c.CreatedAt, &c.UpdatedAt)
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
		`UPDATE clients SET company_name = $2, version = version + 1, updated_at = now()
		 WHERE id = $1::uuid AND version = $3 AND deleted_at IS NULL
		 RETURNING id, company_name, version, created_at, updated_at`,
		id, in.CompanyName, version).
		Scan(&c.ID, &c.CompanyName, &c.Version, &c.CreatedAt, &c.UpdatedAt)
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

// ListEquipment — оборудование клиента. Сервис проверяет существование клиента до вызова (design-sql.md).
func (r *Repo) ListEquipment(ctx context.Context, clientID string, limit, offset int) ([]Equipment, int, error) {
	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM equipment WHERE client_id = $1::uuid AND deleted_at IS NULL`, clientID).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.pool.Query(ctx,
		`SELECT `+equipmentColumns+`
		 FROM equipment e
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
		 FROM equipment e
		 WHERE e.id = $1::uuid AND e.deleted_at IS NULL`, id))
}

// CreateEquipment — вставка + повторное чтение (Get Equipment Query). 23503 на client_id
// (клиент удалён параллельно) → errClientGone.
func (r *Repo) CreateEquipment(ctx context.Context, clientID string, in EquipmentInput) (*Equipment, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	id, err := insertEquipment(ctx, tx, clientID, in)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return r.GetEquipment(ctx, id)
}

// insertEquipment — блокировка клиента и INSERT единицы в транзакции вызывающего
// (общая для POST /clients/:id/equipment и атомарного создания заказа, CreateEquipmentTx).
func insertEquipment(ctx context.Context, tx pgx.Tx, clientID string, in EquipmentInput) (string, error) {
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
	var id string
	err = tx.QueryRow(ctx,
		`INSERT INTO equipment (client_id, plate_number) VALUES ($1::uuid, $2) RETURNING id`,
		clientID, in.PlateNumber).Scan(&id)
	if err != nil {
		return "", mapEquipmentFKViolation(err)
	}
	return id, nil
}

// UpdateEquipment — полная замена (LWW, без version). ErrNoRows → ErrNotFound.
func (r *Repo) UpdateEquipment(ctx context.Context, id string, in EquipmentInput) (*Equipment, error) {
	var updatedID string
	err := r.pool.QueryRow(ctx,
		`UPDATE equipment SET plate_number = $2, updated_at = now()
		 WHERE id = $1::uuid AND deleted_at IS NULL RETURNING id`,
		id, in.PlateNumber).Scan(&updatedID)
	if err != nil {
		return nil, err
	}
	return r.GetEquipment(ctx, updatedID)
}

// equipmentLookupFilter — фильтры поиска автомобиля (значения уже провалидированы и приведены
// сервисом: plate/fragment — sqlutil.PlateCompact, clientID — uuid). Пустое поле не фильтрует.
type equipmentLookupFilter struct {
	plate    string // точный компактный госномер
	fragment string // подстрока компактного госномера
	clientID string
}

// LookupEquipment — действующие единицы действующих клиентов. В БД госномер хранится
// нормализованным (верхний регистр), сравнение — по PlateCompactSQL; подстрока — strpos (без
// LIKE-экранирования). Порядок: при fragment — сначала совпадения с начала номера, затем по
// позиции и номеру; иначе — свежие первыми.
func (r *Repo) LookupEquipment(ctx context.Context, f equipmentLookupFilter, limit, offset int) ([]EquipmentLookupItem, int, error) {
	const from = ` FROM equipment e
		 JOIN clients c ON c.id = e.client_id AND c.deleted_at IS NULL
		 WHERE e.deleted_at IS NULL`
	compact := sqlutil.PlateCompactSQL("e.plate_number")
	cond, args := "", []any{}
	arg := func(v any) string { args = append(args, v); return fmt.Sprintf("$%d", len(args)) }
	if f.plate != "" {
		cond += ` AND e.plate_number <> '' AND ` + compact + ` = ` + arg(f.plate)
	}
	fragPos := ""
	if f.fragment != "" {
		fragPos = `strpos(` + compact + `, ` + arg(f.fragment) + `)`
		cond += ` AND e.plate_number <> '' AND ` + fragPos + ` > 0`
	}
	if f.clientID != "" {
		cond += ` AND e.client_id = ` + arg(f.clientID) + `::uuid`
	}
	order := ` ORDER BY e.created_at DESC, e.id`
	if fragPos != "" {
		order = ` ORDER BY ` + fragPos + `, e.plate_number, e.created_at DESC, e.id`
	}
	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*)`+from+cond, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	page := fmt.Sprintf(` LIMIT %s OFFSET %s`, arg(limit), arg(offset))
	rows, err := r.pool.Query(ctx,
		`SELECT e.id, e.client_id, c.company_name, e.plate_number`+
			from+cond+order+page, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []EquipmentLookupItem{}
	for rows.Next() {
		var it EquipmentLookupItem
		if err := rows.Scan(&it.ID, &it.ClientID, &it.ClientName,
			&it.PlateNumber); err != nil {
			return nil, 0, err
		}
		out = append(out, it)
	}
	return out, total, rows.Err()
}
