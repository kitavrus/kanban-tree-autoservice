package catalog

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kitavrus/kanban-tree/pkg/sqlutil"
)

// Service — repo-модель услуги (без комплекта — грузится отдельно ServiceProductRow,
// re-fetch-after-write). Price == nil — «по факту» (NULL в БД).
type Service struct {
	ID         string
	CategoryID string
	Name       string
	Price      *string
	IsActive   bool
	// CategoryIsDeleted — категория услуги мягко удалена (soft-delete-catalog §6.9).
	CategoryIsDeleted bool
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// ServiceProductRow — строка комплекта с денормализованными полями товара (JOIN products).
type ServiceProductRow struct {
	ProductID string
	Name      string
	Unit      string
	Price     string
	Qty       string
	// IsDeleted — товар строки мягко удалён из каталога: строка остаётся с бейджем (DS-014).
	IsDeleted bool
}

type ServiceRepo struct {
	pool *pgxpool.Pool
}

func NewServiceRepo(pool *pgxpool.Pool) *ServiceRepo { return &ServiceRepo{pool: pool} }

var (
	ErrServiceNotFound        = pgx.ErrNoRows
	errServiceCategoryInvalid = errors.New("catalog: service category invalid")
	// errServiceInUse — 23503 на subtasks_service_id_fkey (миграция 0005): заказ ссылается
	// на услугу — удалить нельзя (ADR-005, классификация по имени constraint, не по тексту).
	errServiceInUse = errors.New("catalog: service in use")
)

// errServiceProductInvalid — строка комплекта ссылается на несуществующий/архивный товар
// (task.md: «архивный/несуществующий товар → 400 по полю строки»).
type errServiceProductInvalid struct {
	Index int
	// Deleted — товар мягко удалён (400 products[i].product_id: not_found, DS-014); иначе —
	// не существует либо в архиве (as-is invalid).
	Deleted bool
}

func (e *errServiceProductInvalid) Error() string {
	return fmt.Sprintf("catalog: service product invalid at index %d", e.Index)
}

// serviceColumns — колонки услуги; признак «категория удалена» — подзапросом (как
// productColumns: и RETURNING, и SELECT).
const serviceColumns = `id, category_id, name, price::text, is_active,
	(SELECT c.deleted_at IS NOT NULL FROM catalog_categories c WHERE c.id = services.category_id),
	created_at, updated_at`

func scanService(row pgx.Row) (*Service, error) {
	var s Service
	if err := row.Scan(&s.ID, &s.CategoryID, &s.Name, &s.Price, &s.IsActive, &s.CategoryIsDeleted, &s.CreatedAt, &s.UpdatedAt); err != nil {
		return nil, err
	}
	return &s, nil
}

type ServiceListParams struct {
	CategoryID      string
	Q               string
	IncludeArchived bool
	Limit, Offset   int
}

// List — q ищет по названию услуги (в отличие от товаров — нет артикула/коробки/места).
// Мягко удалённые услуги не отдаются ни при каком фильтре архива (DS-009).
func (r *ServiceRepo) List(ctx context.Context, p ServiceListParams) ([]Service, int, error) {
	conds := []string{"deleted_at IS NULL"}
	var args []any
	add := func(cond string, val any) {
		args = append(args, val)
		conds = append(conds, fmt.Sprintf(cond, len(args)))
	}
	if p.CategoryID != "" {
		add("category_id = $%d::uuid", p.CategoryID)
	}
	if !p.IncludeArchived {
		conds = append(conds, "is_active")
	}
	if p.Q != "" {
		add("name ILIKE $%d", "%"+sqlutil.EscapeLike(p.Q)+"%")
	}
	where := ""
	if len(conds) > 0 {
		where = " WHERE " + strings.Join(conds, " AND ")
	}

	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM services`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	q := `SELECT ` + serviceColumns + ` FROM services` + where +
		fmt.Sprintf(" ORDER BY lower(name), id LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2)
	rows, err := r.pool.Query(ctx, q, append(args, p.Limit, p.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []Service{}
	for rows.Next() {
		s, err := scanService(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *s)
	}
	return out, total, rows.Err()
}

func (r *ServiceRepo) Get(ctx context.Context, id string) (*Service, error) {
	return scanService(r.pool.QueryRow(ctx, `SELECT `+serviceColumns+` FROM services WHERE id = $1::uuid AND deleted_at IS NULL`, id))
}

// Products — комплект услуги, порядок по position (SSR/API — «Название ×qty»). Товар
// соединяется без условия «не удалён»: строка удалённого товара остаётся с признаком (DS-014).
func (r *ServiceRepo) Products(ctx context.Context, serviceID string) ([]ServiceProductRow, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT sp.product_id, p.name, p.unit, p.price::text, sp.qty::text, p.deleted_at IS NOT NULL
		FROM service_products sp JOIN products p ON p.id = sp.product_id
		WHERE sp.service_id = $1::uuid ORDER BY sp.position`, serviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ServiceProductRow{}
	for rows.Next() {
		var row ServiceProductRow
		if err := rows.Scan(&row.ProductID, &row.Name, &row.Unit, &row.Price, &row.Qty, &row.IsDeleted); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// ProductsBatch — комплекты сразу для набора услуг (Sec 18/Серьёзное 1 — N+1: раньше
// ServiceService.List дёргал Products() на КАЖДУЮ услугу страницы отдельно, до 501
// round-trip на limit=500; здесь — один запрос WHERE service_id = ANY($1::uuid[]),
// группировка в Go, эталон — clients phonesBatch).
func (r *ServiceRepo) ProductsBatch(ctx context.Context, serviceIDs []string) (map[string][]ServiceProductRow, error) {
	out := map[string][]ServiceProductRow{}
	if len(serviceIDs) == 0 {
		return out, nil
	}
	rows, err := r.pool.Query(ctx, `
		SELECT sp.service_id::text, sp.product_id, p.name, p.unit, p.price::text, sp.qty::text, p.deleted_at IS NOT NULL
		FROM service_products sp JOIN products p ON p.id = sp.product_id
		WHERE sp.service_id = ANY($1::uuid[]) ORDER BY sp.service_id, sp.position`, serviceIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var serviceID string
		var row ServiceProductRow
		if err := rows.Scan(&serviceID, &row.ProductID, &row.Name, &row.Unit, &row.Price, &row.Qty, &row.IsDeleted); err != nil {
			return nil, err
		}
		out[serviceID] = append(out[serviceID], row)
	}
	return out, rows.Err()
}

// existingServiceProductIDs — product_id, УЖЕ присутствующие в комплекте этой услуги (до
// правки) — грандфазер-набор для checkProducts на Update (code review Medium#2).
func existingServiceProductIDs(ctx context.Context, tx pgx.Tx, serviceID string) (map[string]bool, error) {
	out := map[string]bool{}
	rows, err := tx.Query(ctx, `SELECT product_id::text FROM service_products WHERE service_id = $1::uuid`, serviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

// productState — состояние товара для проверки новой строки комплекта.
type productState struct{ active, deleted bool }

// productStates — товары из набора id одним запросом (ANY($1::uuid[])) под FOR SHARE до конца
// транзакции записи услуги, по возрастанию id: удаление товара (UPDATE строки) либо уже видно,
// либо ждёт — новая строка комплекта не сошлётся на товар, удалённый «между проверкой и
// записью».
func productStates(ctx context.Context, tx pgx.Tx, ids []string) (map[string]productState, error) {
	out := map[string]productState{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := tx.Query(ctx, `SELECT id::text, is_active, deleted_at IS NOT NULL FROM products
		WHERE id = ANY($1::uuid[]) ORDER BY id FOR SHARE`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var st productState
		if err := rows.Scan(&id, &st.active, &st.deleted); err != nil {
			return nil, err
		}
		out[id] = st
	}
	return out, rows.Err()
}

// checkProducts — первая строка со ссылкой на несуществующий/архивный/удалённый товар → индекс
// (удалённый — Deleted, §4.2: при добавлении строки → not_found).
// grandfathered — product_id, уже присутствовавшие в ТЕКУЩЕМ комплекте этой услуги (nil
// на Create — там прошлого комплекта не существует): для них is_active НЕ проверяется —
// существование уже гарантировано FK service_products.product_id (code review Medium#2:
// task.md запрещает архивный товар «при добавлении» в комплект, не при пересохранении уже
// существующей строки — иначе PRODUCT_IN_USE → архив делает услугу несохраняемой вовсе).
func checkProducts(ctx context.Context, tx pgx.Tx, products []ServiceProductInput, grandfathered map[string]bool) error {
	ids := make([]string, 0, len(products))
	for _, p := range products {
		if grandfathered[p.ProductID] {
			continue
		}
		ids = append(ids, p.ProductID)
	}
	states, err := productStates(ctx, tx, ids)
	if err != nil {
		return err
	}
	for i, p := range products {
		if grandfathered[p.ProductID] {
			continue
		}
		st, ok := states[p.ProductID]
		if !ok || st.deleted || !st.active {
			return &errServiceProductInvalid{Index: i, Deleted: ok && st.deleted}
		}
	}
	return nil
}

// replaceServiceProducts — DELETE+INSERT всего комплекта (проще диффа, product_id RESTRICT
// не мешает — FK защищает удаление самого товара, не строк service_products).
func replaceServiceProducts(ctx context.Context, tx pgx.Tx, serviceID string, products []ServiceProductInput) error {
	if _, err := tx.Exec(ctx, `DELETE FROM service_products WHERE service_id = $1::uuid`, serviceID); err != nil {
		return err
	}
	for i, p := range products {
		if _, err := tx.Exec(ctx,
			`INSERT INTO service_products (service_id, product_id, qty, position) VALUES ($1::uuid, $2::uuid, $3::numeric, $4)`,
			serviceID, p.ProductID, p.Qty, i); err != nil {
			// qty ограничен validateAmount(qtyScale) до дедупа и mergeProducts()
			// проверяет сумму дублей — 22003 здесь непредвиденный путь, страховка
			// (code review Блокер 1).
			return mapAmountOverflow(err)
		}
	}
	return nil
}

func servicePriceArg(in ServiceInput) *string {
	if !in.HasPrice {
		return nil
	}
	return &in.Price
}

// Create — категория (kind=service) + все товары комплекта активны, одной транзакцией.
// in.Products — ИСХОДНЫЕ (не смерженные) строки: checkProducts проверяет существование/
// активность по ним, чтобы errServiceProductInvalid.Index указывал на строку, которую
// реально видит SSR-форма/API-клиент (code review fix-loop 2 Medium); merged — уже
// продедуплицированный список (services_service.prepareProducts) для записи в БД.
func (r *ServiceRepo) Create(ctx context.Context, in ServiceInput, merged []ServiceProductInput) (*Service, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if err := lockCategory(ctx, tx, in.CategoryID, KindService, "", errServiceCategoryInvalid); err != nil {
		return nil, err
	}
	if err := checkProducts(ctx, tx, in.Products, nil); err != nil {
		return nil, err
	}

	row := tx.QueryRow(ctx, `
		INSERT INTO services (category_id, name, price)
		VALUES ($1::uuid, $2, $3::numeric)
		RETURNING `+serviceColumns,
		in.CategoryID, in.Name, servicePriceArg(in))
	s, err := scanService(row)
	if err != nil {
		// price ограничен validateAmount(priceScale) в валидаторе — 22003 здесь
		// непредвиденный путь, страховка (code review Блокер 1).
		return nil, mapAmountOverflow(err)
	}
	if err := replaceServiceProducts(ctx, tx, s.ID, merged); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s, nil
}

// Update — полная замена карточки + комплекта (DELETE+INSERT), одной транзакцией.
// in.Products/merged — см. Create выше (исходные для checkProducts, смерженные для записи).
func (r *ServiceRepo) Update(ctx context.Context, id string, in ServiceInput, merged []ServiceProductInput) (*Service, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Строка услуги — первой, под FOR NO KEY UPDATE с условием «не удалена» (удалённая →
	// ErrServiceNotFound, DS-026); затем категория и товары комплекта (FOR SHARE).
	var curCategory string
	if err := tx.QueryRow(ctx, `SELECT category_id::text FROM services WHERE id = $1::uuid AND deleted_at IS NULL FOR NO KEY UPDATE`,
		id).Scan(&curCategory); err != nil {
		return nil, err // pgx.ErrNoRows == ErrServiceNotFound
	}
	if err := lockCategory(ctx, tx, in.CategoryID, KindService, curCategory, errServiceCategoryInvalid); err != nil {
		return nil, err
	}
	grandfathered, err := existingServiceProductIDs(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	if err := checkProducts(ctx, tx, in.Products, grandfathered); err != nil {
		return nil, err
	}

	row := tx.QueryRow(ctx, `
		UPDATE services SET category_id = $2::uuid, name = $3, price = $4::numeric, is_active = $5, updated_at = now()
		WHERE id = $1::uuid AND deleted_at IS NULL
		RETURNING `+serviceColumns,
		id, in.CategoryID, in.Name, servicePriceArg(in), in.IsActive)
	s, err := scanService(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrServiceNotFound
		}
		// price ограничен validateAmount(priceScale) в валидаторе — 22003 здесь
		// непредвиденный путь, страховка (code review Блокер 1).
		return nil, mapAmountOverflow(err)
	}
	if err := replaceServiceProducts(ctx, tx, id, merged); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s, nil
}

func (r *ServiceRepo) SetActive(ctx context.Context, id string, active bool) error {
	tag, err := r.pool.Exec(ctx, `UPDATE services SET is_active = $2, updated_at = now() WHERE id = $1::uuid AND deleted_at IS NULL`, id, active)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrServiceNotFound
	}
	return nil
}

// Purge — служебная ФИЗИЧЕСКАЯ уборка услуги автотестами (§6.9, DS-027), действующей либо мягко
// удалённой, по правилам прежнего безвозвратного удаления. CASCADE подчищает service_products; 23503 на subtasks_service_id_fkey
// (миграция 0005 — заказ ссылается на услугу) → errServiceInUse. Классификация по имени
// constraint (не по тексту, ADR-005): «любой FK» здесь не годится — у услуги есть и
// service_products (CASCADE, никогда не 23503), и в будущем другие ссылки.
func (r *ServiceRepo) Purge(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM services WHERE id = $1::uuid`, id)
	if err != nil {
		var pgErr *pgconn.PgError
		// PG на ON DELETE RESTRICT отдаёт то 23503 (foreign_key_violation), то 23001
		// (restrict_violation) — оба варианта классифицируются по имени constraint.
		if errors.As(err, &pgErr) && (pgErr.Code == "23503" || pgErr.Code == "23001") &&
			pgErr.ConstraintName == "subtasks_service_id_fkey" {
			return errServiceInUse
		}
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrServiceNotFound
	}
	return nil
}
