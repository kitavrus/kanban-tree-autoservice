package board

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kitavrus/kanban-tree/pkg/numeric"
	"github.com/kitavrus/kanban-tree/pkg/sqlutil"
)

// stockPendingCTETemplate — design-sql.md §Stock Pending CTE (Q-002): общий подзапрос для
// List/Summary/бейджа меню — потребность ещё не взятых в работу заказов (колонка не
// consumes_stock и не финальная) и вычисленное state товара. Склад один — остаток в товаре.
//
// Мягко удалённый товар (soft-delete-catalog §6.9, DS-042): с нулевым остатком в отчёт не
// входит; с ненулевым — входит строкой с is_deleted = true (остаток, появившийся после
// удаления, не теряется из вида). $FORCE — товар, строка которого нужна независимо от
// остатка: ответ ручной корректировки, обнулившей остаток удалённого товара.
const stockPendingCTETemplate = PendingCTE + `,
rows AS (
    SELECT p.id, p.deleted_at IS NOT NULL AS is_deleted, p.name, p.unit, p.box, p.place,
           p.stock, p.min_stock,
           coalesce(pd.qty, 0) AS pending, p.stock - coalesce(pd.qty, 0) AS forecast,
           CASE WHEN p.stock <= 0 THEN 'out'
                WHEN p.stock <= p.min_stock THEN 'low'
                WHEN p.stock - coalesce(pd.qty, 0) <= p.min_stock THEN 'will_low'
                ELSE 'ok' END AS state
    FROM products p LEFT JOIN pending pd ON pd.product_id = p.id
    WHERE (p.deleted_at IS NULL AND p.is_active)
       OR (p.deleted_at IS NOT NULL AND (p.stock <> 0$FORCE)))
`

// PendingCTE — «WITH pending AS (…)»: потребность в товаре ещё не взятых в работу заказов
// (колонка не consumes_stock и не финальная, заказ не удалён). Единый источник расчёта для
// «Остатков» и подсказки дефицита закупок (purchase-orders §6.10, DS-064); экспортирован
// без изменения поведения /stock (DS-005).
const PendingCTE = `
WITH pending AS (
    SELECT sp.product_id, sum(sp.qty) AS qty
    FROM subtask_products sp
    JOIN subtasks s ON s.id = sp.subtask_id
    JOIN tasks t    ON t.id = s.task_id
    JOIN columns c  ON c.id = t.column_id
    WHERE NOT c.consumes_stock AND NOT c.is_final AND t.deleted_at IS NULL
    GROUP BY sp.product_id)`

// PendingTasksSQL — заказы клиентов, дающие pending по товарам $1 (uuid[]): тот же фильтр,
// что у PendingCTE (purchase-orders §6.10 items[].tasks[]).
const PendingTasksSQL = `
SELECT DISTINCT sp.product_id::text, t.id, t.title
FROM subtask_products sp
JOIN subtasks s ON s.id = sp.subtask_id
JOIN tasks t    ON t.id = s.task_id
JOIN columns c  ON c.id = t.column_id
WHERE NOT c.consumes_stock AND NOT c.is_final AND t.deleted_at IS NULL
  AND sp.product_id = ANY($1::uuid[])
ORDER BY 1, 3, 2`

// stockPendingSQL — CTE с принудительно включённым товаром force (SQL-параметр «$1::uuid»)
// либо без него ("").
func stockPendingSQL(force string) string {
	if force == "" {
		return strings.ReplaceAll(stockPendingCTETemplate, "$FORCE", "")
	}
	return strings.ReplaceAll(stockPendingCTETemplate, "$FORCE", " OR p.id = "+force)
}

var stockPendingCTE = stockPendingSQL("")

const stockRowColumns = `id, is_deleted, name, unit, box, place, stock::text, min_stock::text, pending::text, forecast::text, state`

type StockRepo struct {
	pool *pgxpool.Pool
}

func NewStockRepo(pool *pgxpool.Pool) *StockRepo { return &StockRepo{pool: pool} }

// StockListParams — фильтры экрана «Остатки».
type StockListParams struct {
	Scope, Q      string
	Limit, Offset int
}

// List — Stock List Query. scope=StockScopeAll — все активные товары, иначе (StockScopeRisky)
// только state <> 'ok'. Удалённый товар с остатком показывается при любом scope (DS-042).
// q — сырой ввод, экранирует repo.
func (r *StockRepo) List(ctx context.Context, p StockListParams) ([]StockRow, int, error) {
	var like *string
	if p.Q != "" {
		s := "%" + sqlutil.EscapeLike(p.Q) + "%"
		like = &s
	}
	rows, err := r.pool.Query(ctx, stockPendingCTE+`
SELECT `+stockRowColumns+`, count(*) OVER () AS total
FROM rows
WHERE ($1 = 'all' OR state <> 'ok' OR is_deleted)
  AND ($2::text IS NULL OR name ILIKE $2 ESCAPE '\')
ORDER BY stock - min_stock, name, id
LIMIT $3 OFFSET $4`, p.Scope, like, p.Limit, p.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var total int
	out := []StockRow{}
	for rows.Next() {
		row, err := scanStockRow(rows, &total)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, row)
	}
	return out, total, rows.Err()
}

func scanStockRow(rows pgx.Row, totalDst *int) (StockRow, error) {
	var row StockRow
	var stock, minStock, pending, forecast string
	dest := []any{&row.ProductID, &row.IsDeleted, &row.Name, &row.Unit, &row.Box, &row.Place,
		&stock, &minStock, &pending, &forecast, &row.State}
	if totalDst != nil {
		dest = append(dest, totalDst)
	}
	if err := rows.Scan(dest...); err != nil {
		return row, err
	}
	row.Stock, row.MinStock, row.Pending, row.Forecast =
		numeric.Amount(stock), numeric.Amount(minStock), numeric.Amount(pending), numeric.Amount(forecast)
	return row, nil
}

// Summary — Stock Summary Query (Q-002/AC-016); RiskCount = бейдж меню. Мягко удалённые
// товары в счётчики не входят (soft-delete-catalog §6.9, DS-042).
func (r *StockRepo) Summary(ctx context.Context) (*StockSummaryResponse, error) {
	var resp StockSummaryResponse
	err := r.pool.QueryRow(ctx, stockPendingCTE+`
SELECT count(*)                                   AS products_total,
       count(*) FILTER (WHERE state = 'low')      AS low_count,
       count(*) FILTER (WHERE state = 'out')      AS out_count,
       count(*) FILTER (WHERE state = 'will_low') AS will_low_count,
       count(*) FILTER (WHERE state <> 'ok')      AS risk_count
FROM rows WHERE NOT is_deleted`).Scan(&resp.ProductsTotal, &resp.LowCount, &resp.OutCount, &resp.WillLowCount, &resp.RiskCount)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// errStockIncrease — корректировка остатка мягко удалённого товара не в сторону нуля (DS-042:
// остаток удалённого товара можно только списать).
var errStockIncrease = errors.New("board: deleted product stock can only be written off")

// Adjust — правка остатка товара вручную одной транзакцией (D44); запись в
// stock_movements(reason=manual) — только при ненулевой дельте. Ответ — свежая строка
// (re-fetch-after-write: pending/forecast/state — вычисляемые поля).
//
// Мягко удалённый товар (soft-delete-catalog §6.9, DS-042): корректировка разрешена только
// чтобы остаток СПИСАТЬ — нулевой остаток → errNotFound (для обычных адресов такого товара
// нет, DS-026); новое значение вне [min(0, stock), max(0, stock)] → errStockIncrease.
//
// Строка товара берётся FOR NO KEY UPDATE: удаление товара, идущее одновременно (тот же режим той
// же строки, затем проверка остатка), либо уже видно, либо ждёт корректировку и видит её остаток.
func (r *StockRepo) Adjust(ctx context.Context, productID, userID string, newStock numeric.Amount) (*StockRow, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var oldStock string
	var deleted, hasStock, writeOff bool
	err = tx.QueryRow(ctx, `
		SELECT stock::text, deleted_at IS NOT NULL, stock <> 0,
		       $2::numeric BETWEEN least(stock, 0) AND greatest(stock, 0)
		FROM products WHERE id = $1::uuid FOR NO KEY UPDATE`, productID, newStock.String()).
		Scan(&oldStock, &deleted, &hasStock, &writeOff)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errNotFound
	}
	if err != nil {
		return nil, err
	}
	if deleted {
		if !hasStock {
			return nil, errNotFound
		}
		if !writeOff {
			return nil, errStockIncrease
		}
	}

	if _, err := tx.Exec(ctx, `UPDATE products SET stock = $2::numeric, updated_at = now() WHERE id = $1::uuid`,
		productID, newStock.String()); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO stock_movements (product_id, qty_delta, reason, user_id)
		SELECT $1::uuid, $2::numeric - $3::numeric, 'manual', $4::uuid
		WHERE ($2::numeric - $3::numeric) <> 0`,
		productID, newStock.String(), oldStock, userID); err != nil {
		if isNumericOverflow(err) {
			return nil, errStockOverflow
		}
		return nil, err
	}

	// $1 — принудительно включённый товар: удалённый товар, остаток которого только что обнулён,
	// из отчёта уходит, но ответ корректировки — его свежая строка (200, §8.3).
	row, err := scanStockRow(tx.QueryRow(ctx, stockPendingSQL("$1::uuid")+`
SELECT `+stockRowColumns+` FROM rows WHERE id = $1::uuid`, productID), nil)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errNotFound
	}
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &row, nil
}
