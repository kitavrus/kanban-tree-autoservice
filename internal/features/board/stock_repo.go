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

// stockPairsCTE — Stock Pending CTE (Q-002) по парам товар×склад (D45/D48, DS-004):
// pending — потребность ещё не взятых в работу заказов (колонка не consumes_stock и не
// финальная) по складу строки товара; pairs — «учитываемые» пары: есть строка
// product_stock либо потребность (отсутствующая пара без потребности = 0/0, не риск);
// только активные товары и склады. state вычисляется по паре. $1 — фильтр склада
// (NULL — все склады). transit — «В пути» (D50, DS-004/DS-006): сумма qty_sent строк
// перемещений in_transit по товару; при фильтре склада — только входящие в этот склад.
// Учитываются направления «склад → склад» и «поставщик → склад» (transfers-partners DS-047).
//
// Мягко удалённый товар (soft-delete-catalog §6.9, DS-042): с нулевым остатком на всех складах
// в отчёт не входит; с ненулевым хотя бы на одном — входит строкой с is_deleted = true
// независимо от признака архива (остаток, появившийся после удаления, не теряется из вида).
// Параметр $FORCE — товар, строка которого нужна независимо от остатка: ответ ручной
// корректировки, обнулившей остаток удалённого товара.
//
// Склады (soft-delete-warehouses-suppliers DS-042, DS-049): кроме действующих активных в отчёт
// входят архивные и мягко удалённые склады с ненулевым остатком хотя бы одного товара отчёта
// ($WH_VISIBLE) — иначе остаток на них (в т.ч. удалённого товара только на архивном складе) был
// бы виден нулями и его нечем было бы списать. Пара на таком складе (wh_live = false) несёт
// только остаток: минимум и потребность у неё нулевые, состояние — 'ok' (склад не пополняется),
// поэтому в счётчики риска и бейдж меню она не входит. on_deleted_wh — у товара есть ненулевой
// остаток на удалённом складе: строка показывается при любом scope, как строка удалённого товара.
const stockPairsCTETemplate = `
WITH pending AS (
    SELECT sp.product_id, sp.warehouse_id, sum(sp.qty) AS qty
    FROM subtask_products sp
    JOIN subtasks s ON s.id = sp.subtask_id
    JOIN tasks t    ON t.id = s.task_id
    JOIN columns c  ON c.id = t.column_id
    WHERE NOT c.consumes_stock AND NOT c.is_final AND t.deleted_at IS NULL
    GROUP BY sp.product_id, sp.warehouse_id),
keys AS (
    SELECT product_id, warehouse_id FROM product_stock
    UNION
    SELECT product_id, warehouse_id FROM pending),
pairs AS (
    SELECT k.product_id, k.warehouse_id, w.position AS wh_position,
           coalesce(ps.stock, 0) AS stock,
           CASE WHEN w.is_active AND w.deleted_at IS NULL THEN coalesce(ps.min_stock, 0) ELSE 0 END AS min_stock,
           coalesce(ps.box, '') AS box, coalesce(ps.place, '') AS place,
           CASE WHEN w.is_active AND w.deleted_at IS NULL THEN coalesce(pd.qty, 0) ELSE 0 END AS pending,
           p.deleted_at IS NOT NULL AS is_deleted,
           (NOT p.is_active AND p.deleted_at IS NULL) AS p_archived,
           (w.is_active AND w.deleted_at IS NULL) AS wh_live, w.deleted_at IS NOT NULL AS wh_deleted
    FROM keys k
    JOIN products p   ON p.id = k.product_id AND $VISIBLE
    JOIN warehouses w ON w.id = k.warehouse_id AND $WH_VISIBLE
    LEFT JOIN product_stock ps ON ps.product_id = k.product_id AND ps.warehouse_id = k.warehouse_id
    LEFT JOIN pending pd       ON pd.product_id = k.product_id AND pd.warehouse_id = k.warehouse_id),
pair_states AS (
    SELECT pairs.*, stock - pending AS forecast,
           CASE WHEN NOT wh_live OR p_archived THEN 'ok'
                WHEN stock <= 0 THEN 'out'
                WHEN stock <= min_stock THEN 'low'
                WHEN stock - pending <= min_stock THEN 'will_low'
                ELSE 'ok' END AS state
    FROM pairs
    WHERE $1::uuid IS NULL OR warehouse_id = $1::uuid),
transit AS (
    SELECT i.product_id, sum(i.qty_sent) AS qty
    FROM transfer_items i
    JOIN transfers st ON st.id = i.transfer_id
    WHERE i.kind = 'product' AND st.status = 'in_transit' AND st.deleted_at IS NULL
      AND st.direction IN ('warehouse_to_warehouse', 'supplier_to_warehouse')
      AND ($1::uuid IS NULL OR st.to_warehouse_id = $1::uuid)
    GROUP BY i.product_id),
rows AS (
    SELECT p.id, p.name, p.unit, p.deleted_at IS NOT NULL AS is_deleted,
           (NOT p.is_active AND p.deleted_at IS NULL) AS is_archived,
           coalesce(bool_or(ps.wh_deleted AND ps.stock <> 0), false) AS on_deleted_wh,
           coalesce((SELECT tr.qty FROM transit tr WHERE tr.product_id = p.id), 0) AS in_transit,
           coalesce((array_agg(ps.box ORDER BY ps.wh_position) FILTER (WHERE ps.box <> ''))[1], '') AS box,
           coalesce((array_agg(ps.place ORDER BY ps.wh_position) FILTER (WHERE ps.place <> ''))[1], '') AS place,
           coalesce(sum(ps.stock), 0) AS stock, coalesce(sum(ps.min_stock), 0) AS min_stock,
           coalesce(sum(ps.pending), 0) AS pending, coalesce(sum(ps.forecast), 0) AS forecast,
           CASE coalesce(max(CASE ps.state WHEN 'out' THEN 3 WHEN 'low' THEN 2 WHEN 'will_low' THEN 1 ELSE 0 END), 0)
                WHEN 3 THEN 'out' WHEN 2 THEN 'low' WHEN 1 THEN 'will_low' ELSE 'ok' END AS state
    FROM products p LEFT JOIN pair_states ps ON ps.product_id = p.id
    WHERE $VISIBLE
    GROUP BY p.id, p.name, p.unit, p.deleted_at, p.is_active)
`

// stockProductVisibleSQL — товар (алиас p) входит в отчёт «Остатки»: действующий активный
// либо мягко удалённый с ненулевым остатком хотя бы на одном складе (DS-042), либо — fix-loop 1
// части 6 — АРХИВНЫЙ действующий с ненулевым остатком на мягко удалённом складе: форма товара
// удалённые склады не показывает, и без строки в отчёте такой остаток не был бы виден нигде и
// молча блокировал бы удаление товара (409 PRODUCT_HAS_STOCK). Остаток архивного товара на
// архивном складе виден и правится в форме товара — в отчёт он не входит, как и раньше.
const stockProductVisibleSQL = `((p.deleted_at IS NULL AND p.is_active)
        OR (p.deleted_at IS NOT NULL AND (EXISTS (SELECT 1 FROM product_stock x WHERE x.product_id = p.id AND x.stock <> 0)$FORCE))
        OR (p.deleted_at IS NULL AND NOT p.is_active AND (EXISTS (
                SELECT 1 FROM product_stock sa JOIN warehouses sw ON sw.id = sa.warehouse_id
                WHERE sa.product_id = p.id AND sa.stock <> 0 AND sw.deleted_at IS NOT NULL)$AFORCE)))`

// stockWarehouseVisibleSQL — склад (алиас w) входит в отчёт «Остатки»: действующий активный либо
// архивный/мягко удалённый с ненулевым остатком хотя бы одного товара отчёта (DS-042, DS-049).
// Удалённый склад «проявляет» остаток любого товара (и архивного — он тоже входит в отчёт);
// архивный склад — остаток действующего активного либо удалённого товара (архивный товар на
// архивном складе виден в форме товара и склад в отчёт не приводит).
const stockWarehouseVisibleSQL = `((w.is_active AND w.deleted_at IS NULL)
        OR EXISTS (SELECT 1 FROM product_stock ws JOIN products wp ON wp.id = ws.product_id
                   WHERE ws.warehouse_id = w.id AND ws.stock <> 0
                     AND (w.deleted_at IS NOT NULL OR wp.deleted_at IS NOT NULL OR wp.is_active)))`

// stockPairsSQL — stockPairsCTETemplate; force — SQL-параметр («$2::uuid») удалённого товара,
// чья строка нужна и при нулевом остатке, либо "".
func stockPairsSQL(force string) string { return stockPairsSQLFor(force, false) }

// stockPairsSQLFor — forceArchived: принудительное включение действует и для АРХИВНОГО товара —
// ответ корректировки, обнулившей его остаток на удалённом складе. Вне этого случая архивный
// товар без остатка на удалённом складе в отчёт не входит (корректировка — 404, как раньше).
func stockPairsSQLFor(force string, forceArchived bool) string {
	visible := stockProductVisibleSQL
	aforce := ""
	if force != "" {
		visible = strings.ReplaceAll(visible, "$FORCE", " OR p.id = "+force)
		if forceArchived {
			aforce = " OR p.id = " + force
		}
	} else {
		visible = strings.ReplaceAll(visible, "$FORCE", "")
	}
	visible = strings.ReplaceAll(visible, "$AFORCE", aforce)
	q := strings.ReplaceAll(stockPairsCTETemplate, "$WH_VISIBLE", stockWarehouseVisibleSQL)
	return strings.ReplaceAll(q, "$VISIBLE", visible)
}

// stockPairsCTE — отчёт без принудительно включённого товара (список, счётчики, колонки складов).
var stockPairsCTE = stockPairsSQL("")

type StockRepo struct {
	pool *pgxpool.Pool
}

func NewStockRepo(pool *pgxpool.Pool) *StockRepo { return &StockRepo{pool: pool} }

// StockListParams — фильтры экрана «Остатки». WarehouseID == "" — все склады.
type StockListParams struct {
	Scope, Q, WarehouseID string
	Limit, Offset         int
}

func nullableUUID(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// List — Stock List Query. scope=StockScopeAll — все активные товары, иначе (StockScopeRisky)
// только товары, у которых хоть одна пара (в фильтре склада) не 'ok'. q экранирует repo.
// Удалённый товар с остатком и товар с остатком на удалённом складе показываются при любом scope
// (DS-042: остаток не теряется из вида).
// Колонки по складам (by_warehouse) — отдельным запросом по товарам страницы.
func (r *StockRepo) List(ctx context.Context, p StockListParams) ([]StockRow, int, error) {
	var like *string
	if p.Q != "" {
		s := "%" + sqlutil.EscapeLike(p.Q) + "%"
		like = &s
	}
	rows, err := r.pool.Query(ctx, stockPairsCTE+`
SELECT id, is_deleted, is_archived, name, unit, box, place, stock::text, min_stock::text, pending::text, forecast::text, state,
       in_transit::text, count(*) OVER () AS total
FROM rows
WHERE ($2 = 'all' OR state <> 'ok' OR is_deleted OR on_deleted_wh)
  AND ($3::text IS NULL OR name ILIKE $3 ESCAPE '\')
ORDER BY stock - min_stock, name, id
LIMIT $4 OFFSET $5`, nullableUUID(p.WarehouseID), p.Scope, like, p.Limit, p.Offset)
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
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	rows.Close()
	if err := r.fillByWarehouse(ctx, r.pool, out); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

func scanStockRow(rows pgx.Row, totalDst *int) (StockRow, error) {
	var row StockRow
	var stock, minStock, pending, forecast, inTransit string
	dest := []any{&row.ProductID, &row.IsDeleted, &row.IsArchived, &row.Name, &row.Unit, &row.Box, &row.Place,
		&stock, &minStock, &pending, &forecast, &row.State, &inTransit}
	if totalDst != nil {
		dest = append(dest, totalDst)
	}
	if err := rows.Scan(dest...); err != nil {
		return row, err
	}
	row.Stock, row.MinStock, row.Pending, row.Forecast, row.InTransit =
		numeric.Amount(stock), numeric.Amount(minStock), numeric.Amount(pending), numeric.Amount(forecast), numeric.Amount(inTransit)
	return row, nil
}

type querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// fillByWarehouse — колонки складов: по каждому складу отчёта (активные + архивные и удалённые
// с ненулевым остатком, DS-049) — пара товар×склад (отсутствующая = 0/0, state 'ok'), независимо
// от фильтра склада (колонки — всегда все). Набор складов — тот же, что у Warehouses.
func (r *StockRepo) fillByWarehouse(ctx context.Context, q querier, items []StockRow) error {
	if len(items) == 0 {
		return nil
	}
	ids := make([]string, len(items))
	idx := make(map[string]int, len(items))
	for i := range items {
		ids[i] = items[i].ProductID
		idx[items[i].ProductID] = i
		items[i].ByWarehouse = []StockWarehouseCell{}
	}
	rows, err := q.Query(ctx, stockPairsCTE+`
SELECT p.id::text, w.id::text,
       coalesce(ps.stock, 0)::text, coalesce(ps.min_stock, 0)::text, coalesce(ps.pending, 0)::text,
       coalesce(ps.state, 'ok')
FROM products p
CROSS JOIN warehouses w
LEFT JOIN pair_states ps ON ps.product_id = p.id AND ps.warehouse_id = w.id
WHERE p.id = ANY($2::uuid[]) AND `+stockWarehouseVisibleSQL+`
ORDER BY p.id, w.position, w.id`, nil, ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var pid string
		var c StockWarehouseCell
		var stock, minStock, pending string
		if err := rows.Scan(&pid, &c.WarehouseID, &stock, &minStock, &pending, &c.State); err != nil {
			return err
		}
		c.Stock, c.MinStock, c.Pending = numeric.Amount(stock), numeric.Amount(minStock), numeric.Amount(pending)
		if i, ok := idx[pid]; ok {
			items[i].ByWarehouse = append(items[i].ByWarehouse, c)
		}
	}
	return rows.Err()
}

// Warehouses — склады колонок и фильтра экрана «Остатки» (по position): активные и — DS-049 —
// архивные и мягко удалённые с ненулевым остатком (is_archived / is_deleted; у удалённого
// склада is_archived = false — пометка одна).
func (r *StockRepo) Warehouses(ctx context.Context) ([]StockWarehouse, error) {
	rows, err := r.pool.Query(ctx, `
SELECT w.id::text, w.name, NOT w.is_active AND w.deleted_at IS NULL, w.deleted_at IS NOT NULL
FROM warehouses w WHERE `+stockWarehouseVisibleSQL+` ORDER BY w.position, w.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []StockWarehouse{}
	for rows.Next() {
		var w StockWarehouse
		if err := rows.Scan(&w.ID, &w.Name, &w.IsArchived, &w.IsDeleted); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// WarehouseHidden — склад id мягко удалён и в отчёт не входит (остатка на нём нет): фильтр по
// такому складу — 400 warehouse_id: not_found (soft-delete-warehouses-suppliers §6.5).
func (r *StockRepo) WarehouseHidden(ctx context.Context, id string) (bool, error) {
	var hidden bool
	err := r.pool.QueryRow(ctx, `
SELECT EXISTS (SELECT 1 FROM warehouses w WHERE w.id = $1::uuid AND w.deleted_at IS NOT NULL
               AND NOT `+stockWarehouseVisibleSQL+`)`, id).Scan(&hidden)
	return hidden, err
}

// Summary — Stock Summary Query (Q-002/AC-016) по ПАРАМ товар×склад (DS-004); фильтр
// склада — warehouseID ("" — все). RiskCount по всем складам = бейдж меню. Мягко удалённые
// товары в счётчики не входят (soft-delete-catalog §6.9, DS-042); остатки на архивных и
// удалённых складах — тоже: их пары всегда 'ok' (wh_live = false).
func (r *StockRepo) Summary(ctx context.Context, warehouseID string) (*StockSummaryResponse, error) {
	var resp StockSummaryResponse
	err := r.pool.QueryRow(ctx, stockPairsCTE+`
SELECT (SELECT count(*) FROM products WHERE is_active AND deleted_at IS NULL)  AS products_total,
       count(*) FILTER (WHERE state = 'low')      AS low_count,
       count(*) FILTER (WHERE state = 'out')      AS out_count,
       count(*) FILTER (WHERE state = 'will_low') AS will_low_count,
       count(*) FILTER (WHERE state <> 'ok')      AS risk_count
FROM pair_states WHERE NOT is_deleted AND NOT p_archived AND wh_live`, nullableUUID(warehouseID)).Scan(&resp.ProductsTotal, &resp.LowCount, &resp.OutCount, &resp.WillLowCount, &resp.RiskCount)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// errWarehouseNotFound — склад правки остатка не существует или в архиве.
var errWarehouseNotFound = errors.New("board: warehouse not found")

// errWarehouseDeleted — склад правки остатка мягко удалён и остатка этого товара на нём нет
// (для обычных адресов такого склада нет, DS-026) → 400 warehouse_id: not_found.
var errWarehouseDeleted = errors.New("board: warehouse deleted")

// errWarehouseWriteOffOnly — корректировка остатка на мягко удалённом складе не в сторону нуля
// (soft-delete-warehouses-suppliers DS-048: такой остаток можно только списать).
var errWarehouseWriteOffOnly = errors.New("board: deleted warehouse stock can only be written off")

// errStockIncrease — корректировка остатка мягко удалённого товара не в сторону нуля (DS-042:
// остаток удалённого товара можно только списать).
var errStockIncrease = errors.New("board: deleted product stock can only be written off")

// Adjust — правка остатка пары товар×склад вручную одной транзакцией (D44, DS-003);
// строка product_stock создаётся при отсутствии и блокируется FOR UPDATE; запись в
// stock_movements(reason=manual, warehouse_id) — только при ненулевой дельте. Ответ —
// свежая строка товара по всем складам (re-fetch-after-write).
//
// Мягко удалённый товар (soft-delete-catalog §6.9, DS-042): корректировка разрешена только
// чтобы остаток СПИСАТЬ —
//   - только по паре товар×склад, где остаток не равен нулю (проверка под FOR UPDATE строки
//     product_stock; строка для пары без остатка не создаётся): пары нет либо остаток нулевой →
//     errNotFound (для обычных адресов такого товара нет, DS-026). Склад при этом может быть и
//     архивным — остаток с него тоже надо уметь списать;
//   - новое значение — между нулём и текущим остатком включительно (уменьшение по модулю,
//     знак не меняется): увеличение либо смена знака → errStockIncrease — иначе удалённому
//     товару создавался бы новый остаток в обход запрета DS-021.
//
// Мягко удалённый склад (soft-delete-warehouses-suppliers DS-042, DS-048): то же правило
// «только списать» — пары нет либо остаток нулевой → errWarehouseDeleted (строка остатка не
// создаётся); увеличение либо смена знака → errWarehouseWriteOffOnly. Если удалены и товар, и
// склад — отвечают ошибки товара.
//
// Строка склада берётся FOR SHARE после товара и до product_stock: удаление склада (FOR UPDATE
// строк warehouses, затем чтение остатков) либо уже видно — ветка «только списать», либо ждёт
// корректировку и затем видит её остаток (409 WAREHOUSE_IN_USE) — остаток на удалённом складе в
// обход запрета не появляется.
//
// Строка товара берётся FOR SHARE первой (порядок «товар → product_stock» общий с удалением
// товара): удаление, идущее одновременно, либо уже видно, либо ждёт корректировку и видит её
// остаток под блокировкой строк product_stock — остаток не теряется.
func (r *StockRepo) Adjust(ctx context.Context, productID, warehouseID, userID string, newStock numeric.Amount) (*StockRow, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var productDeleted bool
	err = tx.QueryRow(ctx, `SELECT deleted_at IS NOT NULL FROM products WHERE id = $1::uuid FOR SHARE`, productID).Scan(&productDeleted)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errNotFound
	}
	if err != nil {
		return nil, err
	}
	var warehouseActive, warehouseDeleted bool
	err = tx.QueryRow(ctx, `SELECT is_active, deleted_at IS NOT NULL FROM warehouses WHERE id = $1::uuid FOR SHARE`, warehouseID).
		Scan(&warehouseActive, &warehouseDeleted)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errWarehouseNotFound
	}
	if err != nil {
		return nil, err
	}
	var oldStock string
	if productDeleted || warehouseDeleted {
		var hasStock, writeOff bool
		err := tx.QueryRow(ctx, `
			SELECT stock::text, stock <> 0, $3::numeric BETWEEN least(stock, 0) AND greatest(stock, 0)
			FROM product_stock WHERE product_id = $1::uuid AND warehouse_id = $2::uuid FOR UPDATE`,
			productID, warehouseID, newStock.String()).Scan(&oldStock, &hasStock, &writeOff)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && !hasStock) {
			if productDeleted {
				return nil, errNotFound
			}
			return nil, errWarehouseDeleted
		}
		if err != nil {
			return nil, err
		}
		if !writeOff {
			if productDeleted {
				return nil, errStockIncrease
			}
			return nil, errWarehouseWriteOffOnly
		}
	} else {
		if !warehouseActive {
			return nil, errWarehouseNotFound
		}
		if _, err := tx.Exec(ctx, `INSERT INTO product_stock (product_id, warehouse_id) VALUES ($1::uuid, $2::uuid) ON CONFLICT DO NOTHING`,
			productID, warehouseID); err != nil {
			return nil, err
		}
		if err := tx.QueryRow(ctx, `SELECT stock::text FROM product_stock WHERE product_id = $1::uuid AND warehouse_id = $2::uuid FOR UPDATE`,
			productID, warehouseID).Scan(&oldStock); err != nil {
			return nil, err
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE product_stock SET stock = $3::numeric WHERE product_id = $1::uuid AND warehouse_id = $2::uuid`,
		productID, warehouseID, newStock.String()); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO stock_movements (product_id, warehouse_id, qty_delta, reason, user_id)
		SELECT $1::uuid, $2::uuid, $3::numeric - $4::numeric, 'manual', $5::uuid
		WHERE ($3::numeric - $4::numeric) <> 0`,
		productID, warehouseID, newStock.String(), oldStock, userID); err != nil {
		if isNumericOverflow(err) {
			return nil, errStockOverflow
		}
		return nil, err
	}

	// $2 — принудительно включённый товар: удалённый товар, остаток которого только что обнулён,
	// из отчёта уходит, но ответ корректировки — его свежая строка (200, §8.3).
	row, err := scanStockRow(tx.QueryRow(ctx, stockPairsSQLFor("$2::uuid", warehouseDeleted)+`
SELECT id, is_deleted, is_archived, name, unit, box, place, stock::text, min_stock::text, pending::text, forecast::text, state, in_transit::text
FROM rows WHERE id = $2::uuid`, nil, productID), nil)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errNotFound
	}
	if err != nil {
		return nil, err
	}
	items := []StockRow{row}
	if err := r.fillByWarehouse(ctx, tx, items); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &items[0], nil
}
