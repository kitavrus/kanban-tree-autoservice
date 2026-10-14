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
	ID        string
	Name      string
	SKU       string
	Unit      string
	Price     string
	IsActive  bool
	Stock     string
	MinStock  string
	Box       string
	Place     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type ProductRepo struct {
	pool *pgxpool.Pool
}

func NewProductRepo(pool *pgxpool.Pool) *ProductRepo { return &ProductRepo{pool: pool} }

var (
	ErrProductNotFound = pgx.ErrNoRows
	errProductInUse    = errors.New("catalog: product in use")
	// errStockDeltaOutOfRange — переполнение qty_delta (см. Update ниже): PG 22003 на
	// вставку в stock_movements, не на карточку товара — маппится отдельно от
	// mapProductWriteErr (та ветка — про поля самой карточки).
	errStockDeltaOutOfRange = errors.New("catalog: stock delta out of range")
)

// productColumns — колонки товара (общие для SELECT и RETURNING).
const productColumns = `id, name, sku, unit, price::text, is_active,
	stock::text, min_stock::text, box, place, created_at, updated_at`

func scanProduct(row pgx.Row) (*Product, error) {
	var p Product
	if err := row.Scan(&p.ID, &p.Name, &p.SKU, &p.Unit, &p.Price, &p.IsActive,
		&p.Stock, &p.MinStock, &p.Box, &p.Place, &p.CreatedAt, &p.UpdatedAt); err != nil {
		return nil, err
	}
	return &p, nil
}

type ProductListParams struct {
	Q               string
	IncludeArchived bool
	Limit, Offset   int
}

// List — q ищет по name/sku/box/place (ILIKE). Мягко удалённые товары не отдаются
// ни при каком значении фильтра архива (soft-delete-catalog §6.9, DS-009).
func (r *ProductRepo) List(ctx context.Context, p ProductListParams) ([]Product, int, error) {
	conds := []string{"deleted_at IS NULL"}
	var args []any
	if !p.IncludeArchived {
		conds = append(conds, "is_active")
	}
	if p.Q != "" {
		like := "%" + sqlutil.EscapeLike(p.Q) + "%"
		args = append(args, like)
		n := len(args)
		conds = append(conds, fmt.Sprintf("(name ILIKE $%d OR sku ILIKE $%d OR box ILIKE $%d OR place ILIKE $%d)", n, n, n, n))
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

// Create — начальный остаток в stock_movements не пишется (нет «старого» значения для дельты).
func (r *ProductRepo) Create(ctx context.Context, in ProductInput) (*Product, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO products (name, sku, unit, price, stock, min_stock, box, place)
		VALUES ($1, $2, $3, $4::numeric, $5::numeric, $6::numeric, $7, $8)
		RETURNING `+productColumns,
		in.Name, in.SKU, in.Unit, in.Price, in.Stock, in.MinStock, in.Box, in.Place)
	p, err := scanProduct(row)
	if err != nil {
		return nil, mapProductWriteErr(err)
	}
	return p, nil
}

// Update — полная замена карточки в одной транзакции: строка товара под FOR NO KEY UPDATE с
// условием «не удалён» (удалённый → ErrProductNotFound, DS-026), UPDATE, и — если остаток
// изменился — запись stock_movements(reason=manual, delta=новое-старое, user) той же
// транзакцией (task.md: «delta=0 → записи нет»). Дельта считается в SQL (NUMERIC - NUMERIC).
func (r *ProductRepo) Update(ctx context.Context, id, userID string, in ProductInput) (*Product, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// oldStock — значение ДО записи, под блокировкой строки: дельта считается относительно
	// него, а не относительно уже применённого нового значения.
	var oldStock string
	if err := tx.QueryRow(ctx, `SELECT stock::text FROM products WHERE id = $1::uuid AND deleted_at IS NULL FOR NO KEY UPDATE`,
		id).Scan(&oldStock); err != nil {
		return nil, err // pgx.ErrNoRows == ErrProductNotFound
	}

	row := tx.QueryRow(ctx, `
		UPDATE products SET name = $2, sku = $3, unit = $4, price = $5::numeric,
			stock = $6::numeric, min_stock = $7::numeric, box = $8, place = $9, is_active = $10, updated_at = now()
		WHERE id = $1::uuid AND deleted_at IS NULL
		RETURNING `+productColumns,
		id, in.Name, in.SKU, in.Unit, in.Price, in.Stock, in.MinStock, in.Box, in.Place, in.IsActive)
	p, err := scanProduct(row)
	if err != nil {
		return nil, mapProductWriteErr(err)
	}

	// delta = новое-старое: оба конца уже провалидированы validateAmount(qtyScale), но
	// РАЗНОСТЬ двух валидных значений у противоположных границ сама может выйти за
	// IntDigits колонки (code review Блокер 1) → 400 на поле "stock", не 500.
	if _, err := tx.Exec(ctx, `
		INSERT INTO stock_movements (product_id, qty_delta, reason, user_id)
		SELECT $1::uuid, $2::numeric - $3::numeric, 'manual', $4::uuid
		WHERE ($2::numeric - $3::numeric) <> 0`,
		id, in.Stock, oldStock, userID); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "22003" {
			return nil, errStockDeltaOutOfRange
		}
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
// вместе с ним — он производный: заказ, чьи движения в нём записаны, держит товар своими
// строками. FK-нарушение (услуги, заказы; RESTRICT) → errProductInUse, журнал при отказе остаётся (одна транзакция).
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

// mapProductWriteErr — 0 строк RETURNING у Update — id не найден. validateAmount() в
// валидаторе уже не пускает price/stock/min_stock за пределы NUMERIC(12,2)/(12,3) до этой
// точки — 22003 здесь возможен только на непредвиденном пути (mapAmountOverflow).
func mapProductWriteErr(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrProductNotFound
	}
	return mapAmountOverflow(err)
}
