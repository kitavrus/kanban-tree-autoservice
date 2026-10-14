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
// float64/сырая строка (потеря точности NUMERIC недопустима). Склады (D45, DS-004):
// предупреждение — по паре товар×склад.
type StockWarning struct {
	ProductID     string         `json:"product_id"`
	WarehouseID   string         `json:"warehouse_id"`
	WarehouseName string         `json:"warehouse_name"`
	Name          string         `json:"name"`
	Stock         numeric.Amount `json:"stock"`
	MinStock      numeric.Amount `json:"min_stock"`
	Kind          string         `json:"kind"` // negative | below_min
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

// stockDelta — строка дельты Reconcile Query по паре товар×склад (DS-005).
// HasTarget/IsConsume вычислены прямо в SQL — Go не парсит десятичную строку на знак/ноль.
type stockDelta struct {
	ProductID   string
	WarehouseID string
	TargetQty   string // целевое количество task_stock (0, если !HasTarget)
	HasTarget   bool
	Delta       string // target - cur; может быть отрицательным
	IsConsume   bool   // Delta > 0 → списание (task_consume), иначе возврат (task_return)
}

// reconcileDeltasSQL — WITH-структура design-sql.md §Reconcile Query шаг 1, ключ —
// (product_id, warehouse_id) (DS-005). Смена склада у списанной строки даёт ДВЕ дельты:
// возврат на старый склад и списание с нового (TS-005).
const reconcileDeltasSQL = `
WITH demand AS (
    SELECT sp.product_id, sp.warehouse_id, sum(sp.qty) AS qty
    FROM subtask_products sp JOIN subtasks s ON s.id = sp.subtask_id
    WHERE s.task_id = $1 GROUP BY sp.product_id, sp.warehouse_id),
target AS (SELECT product_id, warehouse_id, qty FROM demand WHERE $2),
cur AS (SELECT product_id, warehouse_id, qty FROM task_stock WHERE task_id = $1)
SELECT coalesce(t.product_id, c.product_id) AS product_id,
       coalesce(t.warehouse_id, c.warehouse_id) AS warehouse_id,
       coalesce(t.qty, 0)::text AS target_qty,
       coalesce(t.qty, 0) > 0 AS has_target,
       (coalesce(t.qty, 0) - coalesce(c.qty, 0))::text AS delta,
       (coalesce(t.qty, 0) - coalesce(c.qty, 0)) > 0 AS is_consume
FROM target t FULL JOIN cur c ON c.product_id = t.product_id AND c.warehouse_id = t.warehouse_id
WHERE coalesce(t.qty, 0) <> coalesce(c.qty, 0)
ORDER BY 1, 2;`

// reconcileEnsureStockRowsSQL — DS-005: отсутствующая строка product_stock (= 0/0)
// создаётся ДО блокировки, чтобы FOR UPDATE было что блокировать.
const reconcileEnsureStockRowsSQL = `
INSERT INTO product_stock (product_id, warehouse_id)
SELECT u.p, u.w FROM unnest($1::uuid[], $2::uuid[]) AS u(p, w)
ORDER BY u.p, u.w
ON CONFLICT DO NOTHING`

// reconcileLockStockSQL — Lock Order (D34 + DS-005): product_stock FOR UPDATE в порядке
// (product_id, warehouse_id), ПОСЛЕ блокировки заказа вызывающей стороной.
const reconcileLockStockSQL = `
SELECT ps.product_id FROM product_stock ps
JOIN unnest($1::uuid[], $2::uuid[]) AS u(p, w) ON ps.product_id = u.p AND ps.warehouse_id = u.w
ORDER BY ps.product_id, ps.warehouse_id FOR UPDATE OF ps`

const reconcileApplyStockSQL = `
UPDATE product_stock SET stock = stock - $3::numeric
WHERE product_id = $1::uuid AND warehouse_id = $2::uuid`

const reconcileUpsertTaskStockSQL = `
INSERT INTO task_stock (task_id, product_id, warehouse_id, qty) VALUES ($1, $2::uuid, $3::uuid, $4::numeric)
ON CONFLICT (task_id, product_id, warehouse_id) DO UPDATE SET qty = EXCLUDED.qty`

const reconcileDeleteTaskStockSQL = `DELETE FROM task_stock WHERE task_id = $1 AND product_id = $2::uuid AND warehouse_id = $3::uuid`

const reconcileInsertMovementSQL = `
INSERT INTO stock_movements (product_id, warehouse_id, task_id, qty_delta, reason, user_id)
VALUES ($1::uuid, $2::uuid, $3, -$4::numeric, $5, $6::uuid)`

// reconcileWarningsSQL — Q-006 + DS-004: значения ПОСЛЕ применения дельт, по
// затронутым парам товар×склад.
const reconcileWarningsSQL = `
SELECT p.id, ps.warehouse_id, w.name, p.name, ps.stock::text, ps.min_stock::text,
       CASE WHEN ps.stock < 0 THEN 'negative' ELSE 'below_min' END AS kind
FROM product_stock ps
JOIN unnest($1::uuid[], $2::uuid[]) AS u(p, w) ON ps.product_id = u.p AND ps.warehouse_id = u.w
JOIN products p ON p.id = ps.product_id
JOIN warehouses w ON w.id = ps.warehouse_id
WHERE ps.stock <= ps.min_stock ORDER BY p.name, p.id, w.position, w.id`

type StockEngine struct{}

func NewStockEngine() *StockEngine { return &StockEngine{} }

// Reconcile — приводит task_stock/product_stock/stock_movements заказа taskID к целевому
// состоянию по парам товар×склад (ADR-104, DS-005): targetConsumes=true — списывает
// потребность подзадач, false — возвращает всё на склады (0 дельт — no-op).
// Вызывается ПОСЛЕ блокировки заказа (FOR UPDATE) вызывающей стороной; сам блокирует
// затронутые строки product_stock FOR UPDATE ORDER BY (product_id, warehouse_id).
func (e *StockEngine) Reconcile(ctx context.Context, tx pgx.Tx, taskID int64, targetConsumes bool, userID string) (ReconcileResult, error) {
	deltas, err := e.deltas(ctx, tx, taskID, targetConsumes)
	if err != nil {
		return ReconcileResult{}, err
	}
	if len(deltas) == 0 {
		return ReconcileResult{Warnings: []StockWarning{}}, nil
	}

	productIDs := make([]string, len(deltas))
	warehouseIDs := make([]string, len(deltas))
	for i, d := range deltas {
		productIDs[i] = d.ProductID
		warehouseIDs[i] = d.WarehouseID
	}
	if _, err := tx.Exec(ctx, reconcileEnsureStockRowsSQL, productIDs, warehouseIDs); err != nil {
		return ReconcileResult{}, err
	}
	lockRows, err := tx.Query(ctx, reconcileLockStockSQL, productIDs, warehouseIDs)
	if err != nil {
		return ReconcileResult{}, err
	}
	lockRows.Close()
	if err := lockRows.Err(); err != nil {
		return ReconcileResult{}, err
	}

	for _, d := range deltas {
		if _, err := tx.Exec(ctx, reconcileApplyStockSQL, d.ProductID, d.WarehouseID, d.Delta); err != nil {
			if isNumericOverflow(err) {
				return ReconcileResult{}, errStockOverflow
			}
			return ReconcileResult{}, err
		}

		if d.HasTarget {
			if _, err := tx.Exec(ctx, reconcileUpsertTaskStockSQL, taskID, d.ProductID, d.WarehouseID, d.TargetQty); err != nil {
				return ReconcileResult{}, err
			}
		} else if _, err := tx.Exec(ctx, reconcileDeleteTaskStockSQL, taskID, d.ProductID, d.WarehouseID); err != nil {
			return ReconcileResult{}, err
		}

		reason := StockMovementReasonTaskReturn
		if d.IsConsume {
			reason = StockMovementReasonTaskConsume
		}
		if _, err := tx.Exec(ctx, reconcileInsertMovementSQL, d.ProductID, d.WarehouseID, taskID, d.Delta, reason, userID); err != nil {
			return ReconcileResult{}, err
		}
	}

	warnings, err := e.warnings(ctx, tx, productIDs, warehouseIDs)
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
		if err := rows.Scan(&d.ProductID, &d.WarehouseID, &d.TargetQty, &d.HasTarget, &d.Delta, &d.IsConsume); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (e *StockEngine) warnings(ctx context.Context, tx pgx.Tx, productIDs, warehouseIDs []string) ([]StockWarning, error) {
	rows, err := tx.Query(ctx, reconcileWarningsSQL, productIDs, warehouseIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []StockWarning{}
	for rows.Next() {
		var w StockWarning
		var stock, minStock string
		if err := rows.Scan(&w.ProductID, &w.WarehouseID, &w.WarehouseName, &w.Name, &stock, &minStock, &w.Kind); err != nil {
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
