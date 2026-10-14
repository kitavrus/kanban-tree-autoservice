package board

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/kitavrus/kanban-tree/pkg/numeric"
)

// StockWarning — предупреждение об остатке после Reconcile (Q-006, ADR-006,
// design-go-layers.md §DTO StockWarning). Stock/MinStock — pkg/numeric.Amount, не
// float64/сырая строка (потеря точности NUMERIC недопустима).
type StockWarning struct {
	ProductID string         `json:"product_id"`
	Name      string         `json:"name"`
	Stock     numeric.Amount `json:"stock"`
	MinStock  numeric.Amount `json:"min_stock"`
	Kind      string         `json:"kind"` // negative | below_min
}

// ReconcileResult — результат StockEngine.Reconcile (design-go-layers.md §Repository
// Interface).
type ReconcileResult struct {
	Warnings  []StockWarning
	Movements int
}

// errStockOverflow — 22003 numeric_value_out_of_range на списание (design-errors.md
// §Error Mapping: «22003 в Reconcile → 500 INTERNAL + stock_reconcile_errors_total++»,
// MVP — объёмы мастерской малы).
var errStockOverflow = errors.New("board: stock reconcile overflow")

// stockDelta — строка дельты Reconcile Query (design-sql.md §Reconcile Query, шаг 1).
// HasTarget/IsConsume вычислены прямо в SQL — Go не парсит десятичную строку на знак/ноль.
type stockDelta struct {
	ProductID string
	TargetQty string // целевое количество task_stock (0, если !HasTarget)
	HasTarget bool
	Delta     string // target - cur; может быть отрицательным
	IsConsume bool   // Delta > 0 → списание (task_consume), иначе возврат (task_return)
}

// reconcileDeltasSQL — та же WITH-структура, что design-sql.md §Reconcile Query шаг 1,
// расширенная двумя производными булевыми колонками (has_target/is_consume) от ТЕХ ЖЕ
// выражений — избегает разбора знака/нуля десятичной строки в Go.
const reconcileDeltasSQL = `
WITH demand AS (
    SELECT sp.product_id, sum(sp.qty) AS qty
    FROM subtask_products sp JOIN subtasks s ON s.id = sp.subtask_id
    WHERE s.task_id = $1 GROUP BY sp.product_id),
target AS (SELECT product_id, qty FROM demand WHERE $2),
cur AS (SELECT product_id, qty FROM task_stock WHERE task_id = $1)
SELECT coalesce(t.product_id, c.product_id) AS product_id,
       coalesce(t.qty, 0)::text AS target_qty,
       coalesce(t.qty, 0) > 0 AS has_target,
       (coalesce(t.qty, 0) - coalesce(c.qty, 0))::text AS delta,
       (coalesce(t.qty, 0) - coalesce(c.qty, 0)) > 0 AS is_consume
FROM target t FULL JOIN cur c ON c.product_id = t.product_id
WHERE coalesce(t.qty, 0) <> coalesce(c.qty, 0)
ORDER BY 1;`

// reconcileLockProductsSQL — Lock Order (design-sql.md, ADR-104): products FOR NO KEY UPDATE
// ORDER BY id, ПОСЛЕ блокировки заказа вызывающей стороной (columns→tasks→products). Режим NO KEY
// UPDATE — тот же, что у правки остатка и удаления товара; вставки по внешнему ключу (FOR KEY
// SHARE) он не блокирует.
const reconcileLockProductsSQL = `SELECT id FROM products WHERE id = ANY($1::uuid[]) ORDER BY id FOR NO KEY UPDATE`

const reconcileApplyStockSQL = `UPDATE products SET stock = stock - $2::numeric, updated_at = now() WHERE id = $1::uuid`

const reconcileUpsertTaskStockSQL = `
INSERT INTO task_stock (task_id, product_id, qty) VALUES ($1, $2::uuid, $3::numeric)
ON CONFLICT (task_id, product_id) DO UPDATE SET qty = EXCLUDED.qty`

const reconcileDeleteTaskStockSQL = `DELETE FROM task_stock WHERE task_id = $1 AND product_id = $2::uuid`

const reconcileInsertMovementSQL = `
INSERT INTO stock_movements (product_id, task_id, qty_delta, reason, user_id)
VALUES ($1::uuid, $2, -$3::numeric, $4, $5::uuid)`

// reconcileWarningsSQL — design-sql.md §Reconcile Query, шаг 4 (Q-006): значения ПОСЛЕ
// применения дельт, по затронутым товарам.
const reconcileWarningsSQL = `
SELECT id, name, stock::text, min_stock::text,
       CASE WHEN stock < 0 THEN 'negative' ELSE 'below_min' END AS kind
FROM products WHERE id = ANY($1::uuid[]) AND stock <= min_stock ORDER BY name, id`

type StockEngine struct{}

func NewStockEngine() *StockEngine { return &StockEngine{} }

// Reconcile — приводит task_stock/products.stock/stock_movements заказа taskID к целевому
// состоянию (ADR-104): targetConsumes=true — резервирует/списывает потребность подзадач,
// false — возвращает всё на склад (0 деltas — no-op, напр. заказ без услуг/товаров).
// Вызывается ПОСЛЕ блокировки заказа (FOR UPDATE) вызывающей стороной; сам блокирует
// затронутые products FOR NO KEY UPDATE ORDER BY id (Lock Order: columns→tasks→products, D34).
func (e *StockEngine) Reconcile(ctx context.Context, tx pgx.Tx, taskID int64, targetConsumes bool, userID string) (ReconcileResult, error) {
	deltas, err := e.deltas(ctx, tx, taskID, targetConsumes)
	if err != nil {
		return ReconcileResult{}, err
	}
	if len(deltas) == 0 {
		return ReconcileResult{Warnings: []StockWarning{}}, nil
	}

	productIDs := make([]string, len(deltas))
	for i, d := range deltas {
		productIDs[i] = d.ProductID
	}
	lockRows, err := tx.Query(ctx, reconcileLockProductsSQL, productIDs)
	if err != nil {
		return ReconcileResult{}, err
	}
	lockRows.Close()
	if err := lockRows.Err(); err != nil {
		return ReconcileResult{}, err
	}

	for _, d := range deltas {
		if _, err := tx.Exec(ctx, reconcileApplyStockSQL, d.ProductID, d.Delta); err != nil {
			if isNumericOverflow(err) {
				return ReconcileResult{}, errStockOverflow
			}
			return ReconcileResult{}, err
		}

		if d.HasTarget {
			if _, err := tx.Exec(ctx, reconcileUpsertTaskStockSQL, taskID, d.ProductID, d.TargetQty); err != nil {
				return ReconcileResult{}, err
			}
		} else if _, err := tx.Exec(ctx, reconcileDeleteTaskStockSQL, taskID, d.ProductID); err != nil {
			return ReconcileResult{}, err
		}

		reason := StockMovementReasonTaskReturn
		if d.IsConsume {
			reason = StockMovementReasonTaskConsume
		}
		if _, err := tx.Exec(ctx, reconcileInsertMovementSQL, d.ProductID, taskID, d.Delta, reason, userID); err != nil {
			return ReconcileResult{}, err
		}
	}

	warnings, err := e.warnings(ctx, tx, productIDs)
	if err != nil {
		return ReconcileResult{}, err
	}
	return ReconcileResult{Warnings: warnings, Movements: len(deltas)}, nil
}

func (e *StockEngine) deltas(ctx context.Context, tx pgx.Tx, taskID int64, targetConsumes bool) ([]stockDelta, error) {
	rows, err := tx.Query(ctx, reconcileDeltasSQL, taskID, targetConsumes)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []stockDelta
	for rows.Next() {
		var d stockDelta
		if err := rows.Scan(&d.ProductID, &d.TargetQty, &d.HasTarget, &d.Delta, &d.IsConsume); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (e *StockEngine) warnings(ctx context.Context, tx pgx.Tx, productIDs []string) ([]StockWarning, error) {
	rows, err := tx.Query(ctx, reconcileWarningsSQL, productIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []StockWarning{}
	for rows.Next() {
		var w StockWarning
		var stock, minStock string
		if err := rows.Scan(&w.ProductID, &w.Name, &stock, &minStock, &w.Kind); err != nil {
			return nil, err
		}
		w.Stock = numeric.Amount(stock)
		w.MinStock = numeric.Amount(minStock)
		out = append(out, w)
	}
	return out, rows.Err()
}

func isNumericOverflow(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "22003"
}
