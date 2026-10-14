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

// Product — repo-модель товара. Numeric-поля читаются как text (`::text` в SELECT) —
// без потери точности и без зависимости от decimal-библиотеки.
type Product struct {
	ID         string
	CategoryID string
	Name       string
	SKU        string
	Unit       string
	Price      string
	IsActive   bool
	// CategoryIsDeleted — категория товара мягко удалена (soft-delete-catalog §6.9).
	CategoryIsDeleted bool
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type ProductRepo struct {
	pool *pgxpool.Pool
}

func NewProductRepo(pool *pgxpool.Pool) *ProductRepo { return &ProductRepo{pool: pool} }

var (
	ErrProductNotFound        = pgx.ErrNoRows
	errProductInUse           = errors.New("catalog: product in use")
	errProductCategoryInvalid = errors.New("catalog: product category invalid")
	// errCategoryDeleted — выбранная категория мягко удалена (400 category_id: not_found, DS-014).
	errCategoryDeleted = errors.New("catalog: category deleted")
)

// productColumns — колонки товара; признак «категория удалена» — подзапросом, чтобы список
// работал и в RETURNING (INSERT/UPDATE), и в SELECT. Категория соединяется без условия «не
// удалена»: товар удалённой категории остаётся действующим (DS-014).
const productColumns = `id, category_id, name, sku, unit, price::text, is_active,
	(SELECT c.deleted_at IS NOT NULL FROM catalog_categories c WHERE c.id = products.category_id),
	created_at, updated_at`

func scanProduct(row pgx.Row) (*Product, error) {
	var p Product
	if err := row.Scan(&p.ID, &p.CategoryID, &p.Name, &p.SKU, &p.Unit, &p.Price, &p.IsActive,
		&p.CategoryIsDeleted, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return nil, err
	}
	return &p, nil
}

type ProductListParams struct {
	CategoryID      string
	Q               string
	IncludeArchived bool
	Limit, Offset   int
}

// List — q ищет по name/sku и коробке/месту на любом складе (ILIKE, DS-002); карточка неактивной категории всё
// равно отдаёт товары (фильтр по товару, не по категории). Мягко удалённые товары не отдаются
// ни при каком значении фильтра архива (soft-delete-catalog §6.9, DS-009).
func (r *ProductRepo) List(ctx context.Context, p ProductListParams) ([]Product, int, error) {
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
		like := "%" + sqlutil.EscapeLike(p.Q) + "%"
		args = append(args, like)
		n := len(args)
		conds = append(conds, fmt.Sprintf("(name ILIKE $%d OR sku ILIKE $%d OR EXISTS (SELECT 1 FROM product_stock ps WHERE ps.product_id = products.id AND (ps.box ILIKE $%d OR ps.place ILIKE $%d)))", n, n, n, n))
	}
	where := ""
	if len(conds) > 0 {
		where = " WHERE " + strings.Join(conds, " AND ")
	}

	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM products`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	q := `SELECT ` + productColumns + ` FROM products` + where +
		fmt.Sprintf(" ORDER BY lower(name), id LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2)
	rows, err := r.pool.Query(ctx, q, append(args, p.Limit, p.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []Product{}
	for rows.Next() {
		pr, err := scanProduct(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *pr)
	}
	return out, total, rows.Err()
}

func (r *ProductRepo) Get(ctx context.Context, id string) (*Product, error) {
	return scanProduct(r.pool.QueryRow(ctx, `SELECT `+productColumns+` FROM products WHERE id = $1::uuid AND deleted_at IS NULL`, id))
}

// lockCategory — категория вида kind под FOR SHARE до конца транзакции записи товара/услуги
// (удаление категории, UPDATE, ждёт либо уже видно): нет либо другой вид → invalid; мягко
// удалена → errCategoryDeleted, кроме случая keepID == id — запись сохраняется без смены уже
// удалённой категории (§4.1, DS-014).
func lockCategory(ctx context.Context, tx pgx.Tx, id, kind, keepID string, invalid error) error {
	var deleted bool
	err := tx.QueryRow(ctx, `SELECT deleted_at IS NOT NULL FROM catalog_categories WHERE id = $1::uuid AND kind = $2 FOR SHARE`,
		id, kind).Scan(&deleted)
	if errors.Is(err, pgx.ErrNoRows) {
		return invalid
	}
	if err != nil {
		return err
	}
	if deleted && id != keepID {
		return errCategoryDeleted
	}
	return nil
}

// Create — категория обязана существовать, иметь kind='product' и не быть удалённой
// (lockCategory, FOR SHARE до конца транзакции). Строки складов (in.Stocks) пишутся той же транзакцией (D46).
func (r *ProductRepo) Create(ctx context.Context, in ProductInput) (*Product, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	if err := lockCategory(ctx, tx, in.CategoryID, KindProduct, "", errProductCategoryInvalid); err != nil {
		return nil, err
	}
	row := tx.QueryRow(ctx, `
		INSERT INTO products (category_id, name, sku, unit, price)
		VALUES ($1::uuid, $2, $3, $4, $5::numeric)
		RETURNING `+productColumns,
		in.CategoryID, in.Name, in.SKU, in.Unit, in.Price)
	p, err := scanProduct(row)
	if err != nil {
		return nil, mapProductWriteErr(err, true)
	}
	if err := checkStockWarehouses(ctx, tx, in.Stocks); err != nil {
		return nil, err
	}
	if err := insertStocks(ctx, tx, p.ID, in.Stocks); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return p, nil
}

// Update — полная замена карточки в одной транзакции: строка товара под FOR NO KEY UPDATE с
// условием «не удалён» (удалённый → ErrProductNotFound, DS-026; порядок блокировок «товар →
// категория → product_stock» общий с удалением товара), проверка категории, UPDATE, и —
// по перечисленным складам (in.Stocks) — правка строк product_stock с записью
// stock_movements(reason=manual, warehouse_id, delta=новое-старое, user) при изменении
// остатка (task.md: «delta=0 → записи нет»; DS-003). Дельта считается в SQL.
func (r *ProductRepo) Update(ctx context.Context, id, userID string, in ProductInput) (*Product, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var curCategory string
	if err := tx.QueryRow(ctx, `SELECT category_id::text FROM products WHERE id = $1::uuid AND deleted_at IS NULL FOR NO KEY UPDATE`,
		id).Scan(&curCategory); err != nil {
		return nil, err // pgx.ErrNoRows == ErrProductNotFound
	}
	if err := lockCategory(ctx, tx, in.CategoryID, KindProduct, curCategory, errProductCategoryInvalid); err != nil {
		return nil, err
	}

	row := tx.QueryRow(ctx, `
		UPDATE products SET category_id = $2::uuid, name = $3, sku = $4, unit = $5, price = $6::numeric,
			is_active = $7, updated_at = now()
		WHERE id = $1::uuid AND deleted_at IS NULL
		RETURNING `+productColumns,
		id, in.CategoryID, in.Name, in.SKU, in.Unit, in.Price, in.IsActive)
	p, err := scanProduct(row)
	if err != nil {
		return nil, mapProductWriteErr(err, false)
	}
	if err := checkStockWarehouses(ctx, tx, in.Stocks); err != nil {
		return nil, err
	}
	if err := updateStocks(ctx, tx, id, userID, in.Stocks); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return p, nil
}

func (r *ProductRepo) SetActive(ctx context.Context, id string, active bool) error {
	tag, err := r.pool.Exec(ctx, `UPDATE products SET is_active = $2, updated_at = now() WHERE id = $1::uuid AND deleted_at IS NULL`, id, active)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrProductNotFound
	}
	return nil
}

// Purge — служебная ФИЗИЧЕСКАЯ уборка товара автотестами (§6.9, DS-027): действующий либо мягко
// удалённый товар. Журнал склада товара (stock_movements) ссылкой не считается и убирается
// вместе с ним — он производный: заказ и документ, чьи движения в нём записаны, держат товар
// своими строками; строки остатков уходят каскадом FK. FK-нарушение (услуги, заказы,
// документы; RESTRICT) → errProductInUse, журнал при отказе остаётся (одна транзакция).
func (r *ProductRepo) Purge(ctx context.Context, id string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `DELETE FROM stock_movements WHERE product_id = $1::uuid`, id); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `DELETE FROM products WHERE id = $1::uuid`, id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && (pgErr.Code == "23503" || pgErr.Code == "23001") {
			return errProductInUse
		}
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrProductNotFound
	}
	return tx.Commit(ctx)
}

// mapProductWriteErr — 0 строк RETURNING: Create — категория невалидна (INSERT ...
// WHERE EXISTS), Update — id не найден (категория уже проверена раньше отдельным
// запросом, поэтому здесь всегда ErrProductNotFound). validateAmount() в валидаторе уже
// не пускает price/stock/min_stock за пределы NUMERIC(12,2)/(12,3) до этой точки — 22003
// здесь возможен только на непредвиденном пути; раньше он молча маппился на
// errProductCategoryInvalid (400 по ЧУЖОМУ полю category_id, code review Блокер 1) —
// теперь mapAmountOverflow не привязывает его к конкретному полю формы.
func mapProductWriteErr(err error, isCreate bool) error {
	if errors.Is(err, pgx.ErrNoRows) {
		if isCreate {
			return errProductCategoryInvalid
		}
		return ErrProductNotFound
	}
	return mapAmountOverflow(err)
}
