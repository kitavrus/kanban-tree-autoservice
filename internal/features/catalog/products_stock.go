package catalog

import (
	"context"
	"errors"
	"fmt"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
	"github.com/kitavrus/kanban-tree/pkg/sqlutil"
)

// Остатки товара по складам (D45/D46, docs/features/warehouses/draft-spec.md DS-002/DS-003):
// product_stock(product_id, warehouse_id, stock, min_stock, box, place). Отсутствующая
// строка = 0/0, пустые коробка/место.

// msgDuplicate — склад повторяется в stocks[] одного запроса.
const msgDuplicate = "duplicate"

// ProductStockInput — строка склада в запросе создания/правки товара. Склады, не
// перечисленные в stocks[], не меняются (частичная правка).
type ProductStockInput struct {
	WarehouseID string
	Stock       string
	MinStock    string
	Box         string
	Place       string
}

// ProductStockRow — строка блока «Склады» карточки товара (repo-модель).
type ProductStockRow struct {
	WarehouseID   string
	WarehouseName string
	IsActive      bool
	Tracked       bool // есть строка product_stock (иначе — 0/0 по умолчанию)
	Stock         string
	MinStock      string
	Box           string
	Place         string
}

// errStockWarehouseInvalid — stocks[Index].warehouse_id не существует.
// Deleted — склад мягко удалён: для обычных адресов его нет → not_found (а не invalid).
type errStockWarehouseInvalid struct {
	Index   int
	Deleted bool
}

// message — код ошибки поля stocks[i].warehouse_id.
func (e *errStockWarehouseInvalid) message() string {
	if e.Deleted {
		return msgNotFound
	}
	return msgInvalid
}

func (e *errStockWarehouseInvalid) Error() string {
	return fmt.Sprintf("catalog: stocks[%d] warehouse invalid", e.Index)
}

// errStockDeltaOutOfRangeAt — переполнение qty_delta движения по stocks[Index].
type errStockDeltaOutOfRangeAt struct{ Index int }

func (e *errStockDeltaOutOfRangeAt) Error() string {
	return fmt.Sprintf("catalog: stocks[%d] delta out of range", e.Index)
}

func normStocks(in []ProductStockInput) []ProductStockInput {
	out := make([]ProductStockInput, len(in))
	for i, s := range in {
		out[i] = ProductStockInput{
			WarehouseID: normStr(s.WarehouseID),
			Stock:       normAmount(s.Stock),
			MinStock:    normAmount(s.MinStock),
			Box:         normStr(s.Box),
			Place:       normStr(s.Place),
		}
	}
	return out
}

// validateStocks — ошибки полей stocks[i].*: склад обязателен/uuid/без повторов, остаток
// может быть < 0 (D44), минимум — нет; коробка/место — до 100 символов.
func validateStocks(stocks []ProductStockInput) []errorspkg.Field {
	var f []errorspkg.Field
	seen := map[string]bool{}
	for i, s := range stocks {
		p := fmt.Sprintf("stocks[%d].", i)
		add := func(name, msg string) { f = append(f, errorspkg.Field{Name: p + name, Message: msg}) }
		switch {
		case s.WarehouseID == "":
			add("warehouse_id", msgRequired)
		case uuid.Validate(s.WarehouseID) != nil:
			add("warehouse_id", msgInvalid)
		case seen[s.WarehouseID]:
			add("warehouse_id", msgDuplicate)
		}
		seen[s.WarehouseID] = true
		if code := validateAmount(s.Stock, qtyScale, true); code != "" {
			add("stock", code)
		}
		if code := validateAmount(s.MinStock, qtyScale, false); code != "" {
			add("min_stock", code)
		}
		for _, kv := range [][2]string{{"box", s.Box}, {"place", s.Place}} {
			if utf8.RuneCountInString(kv[1]) > boxPlaceMax {
				add(kv[0], msgTooLong)
			}
			if sqlutil.HasInvalidText(kv[1]) {
				add(kv[0], msgInvalidText)
			}
		}
	}
	return f
}

// checkStockWarehouses — все stocks[].warehouse_id существуют и не удалены (архивный склад
// допустим — правка уже лежащего там остатка; мягко удалённый — как несуществующий,
// soft-delete-warehouses-suppliers DS-014). Строки складов берутся FOR SHARE по возрастанию id до
// конца транзакции (после строки товара, до product_stock): удаление склада (FOR UPDATE тех же
// строк) либо уже видно → not_found, либо ждёт правку и затем видит её остаток.
func checkStockWarehouses(ctx context.Context, tx pgx.Tx, stocks []ProductStockInput) error {
	if len(stocks) == 0 {
		return nil
	}
	ids := make([]string, len(stocks))
	for i, s := range stocks {
		ids[i] = s.WarehouseID
	}
	rows, err := tx.Query(ctx, `SELECT id::text, deleted_at IS NOT NULL FROM warehouses WHERE id = ANY($1::uuid[]) ORDER BY id FOR SHARE`, ids)
	if err != nil {
		return err
	}
	ok, deleted := map[string]bool{}, map[string]bool{}
	for rows.Next() {
		var id string
		var del bool
		if err := rows.Scan(&id, &del); err != nil {
			rows.Close()
			return err
		}
		ok[id], deleted[id] = !del, del
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for i, s := range stocks {
		if !ok[s.WarehouseID] {
			return &errStockWarehouseInvalid{Index: i, Deleted: deleted[s.WarehouseID]}
		}
	}
	return nil
}

// emptyStockCond — строка «пустая» (0/0, без коробки/места): такую пару для склада, где у
// товара ещё нет строки, не создаём (D48: учитываемая пара = есть строка product_stock;
// иначе каждый товар на каждом складе формы попадал бы в «нет в наличии»).
const emptyStockCond = `($3::numeric = 0 AND $4::numeric = 0 AND $5 = '' AND $6 = '')`

// insertStocks — Create: строки складов нового товара (движение не пишется — нет «старого»
// значения, как и до складов). Строка первого активного склада создаётся всегда (товар
// учитывается хотя бы на складе по умолчанию, как раньше products.stock); пустые строки
// прочих складов пропускаются.
func insertStocks(ctx context.Context, tx pgx.Tx, productID string, stocks []ProductStockInput) error {
	for _, s := range stocks {
		if _, err := tx.Exec(ctx, `
			INSERT INTO product_stock (product_id, warehouse_id, stock, min_stock, box, place)
			SELECT $1::uuid, $2::uuid, $3::numeric, $4::numeric, $5, $6
			WHERE NOT `+emptyStockCond,
			productID, s.WarehouseID, s.Stock, s.MinStock, s.Box, s.Place); err != nil {
			return mapAmountOverflow(err)
		}
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO product_stock (product_id, warehouse_id)
		SELECT $1::uuid, id FROM warehouses WHERE is_active AND deleted_at IS NULL ORDER BY position, id LIMIT 1
		ON CONFLICT DO NOTHING`, productID)
	return err
}

// updateStocks — Update: по каждой строке — строка product_stock создаётся при отсутствии,
// блокируется FOR UPDATE (порядок — warehouse_id, D34), пишется новое значение и при
// изменении остатка — stock_movements(reason=manual, warehouse_id) той же транзакцией
// (DS-003, TS-003).
func updateStocks(ctx context.Context, tx pgx.Tx, productID, userID string, stocks []ProductStockInput) error {
	if len(stocks) == 0 {
		return nil
	}
	order := make([]int, len(stocks))
	for i := range order {
		order[i] = i
	}
	// Lock order — по warehouse_id (product_id один).
	for i := 1; i < len(order); i++ {
		for j := i; j > 0 && stocks[order[j]].WarehouseID < stocks[order[j-1]].WarehouseID; j-- {
			order[j], order[j-1] = order[j-1], order[j]
		}
	}
	for _, idx := range order {
		s := stocks[idx]
		_, err := tx.Exec(ctx, `
			INSERT INTO product_stock (product_id, warehouse_id)
			SELECT $1::uuid, $2::uuid
			WHERE NOT `+emptyStockCond+`
			   OR EXISTS (SELECT 1 FROM product_stock WHERE product_id = $1::uuid AND warehouse_id = $2::uuid)
			ON CONFLICT DO NOTHING`,
			productID, s.WarehouseID, s.Stock, s.MinStock, s.Box, s.Place)
		if err != nil {
			return mapAmountOverflow(err)
		}
		var oldStock string
		err = tx.QueryRow(ctx, `SELECT stock::text FROM product_stock WHERE product_id = $1::uuid AND warehouse_id = $2::uuid FOR UPDATE`,
			productID, s.WarehouseID).Scan(&oldStock)
		if errors.Is(err, pgx.ErrNoRows) {
			continue // пустая строка для склада без строки — не создаём (D48)
		}
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE product_stock SET stock = $3::numeric, min_stock = $4::numeric, box = $5, place = $6
			WHERE product_id = $1::uuid AND warehouse_id = $2::uuid`,
			productID, s.WarehouseID, s.Stock, s.MinStock, s.Box, s.Place); err != nil {
			return mapAmountOverflow(err)
		}
		// Разность двух валидных значений у противоположных границ может выйти за NUMERIC(12,3)
		// (code review Блокер 1 фазы 4) → 400 по полю stocks[i].stock, не 500.
		if _, err := tx.Exec(ctx, `
			INSERT INTO stock_movements (product_id, warehouse_id, qty_delta, reason, user_id)
			SELECT $1::uuid, $2::uuid, $3::numeric - $4::numeric, 'manual', $5::uuid
			WHERE ($3::numeric - $4::numeric) <> 0`,
			productID, s.WarehouseID, s.Stock, oldStock, userID); err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "22003" {
				return &errStockDeltaOutOfRangeAt{Index: idx}
			}
			return err
		}
	}
	return nil
}

// stockRowsSQL — блок «Склады»: все активные склады (LEFT JOIN — отсутствующая строка =
// 0/0) + архивные, где у товара есть строка; порядок — position склада. Мягко удалённых складов
// в блоке нет (soft-delete-warehouses-suppliers §6.5, DS-014) — и в «Итого» их остаток не входит
// (он виден в отчёте «Остатки» с пометкой, DS-042).
const stockRowsSQL = `
SELECT p.id::text, w.id::text, w.name, w.is_active, ps.product_id IS NOT NULL,
       coalesce(ps.stock, 0)::text, coalesce(ps.min_stock, 0)::text,
       coalesce(ps.box, ''), coalesce(ps.place, ''),
       (sum(coalesce(ps.stock, 0)) OVER (PARTITION BY p.id))::text
FROM products p
CROSS JOIN warehouses w
LEFT JOIN product_stock ps ON ps.product_id = p.id AND ps.warehouse_id = w.id
WHERE p.id = ANY($1::uuid[]) AND w.deleted_at IS NULL AND (w.is_active OR ps.product_id IS NOT NULL)
ORDER BY p.id, w.position, w.id`

// ProductStocks — блок «Склады» товара + «Итого» (сумма по всем складам, в SQL — без
// десятичной арифметики в Go).
type ProductStocks struct {
	Rows  []ProductStockRow
	Total string
}

// StocksBatch — строки складов для набора товаров одним запросом (без N+1).
func (r *ProductRepo) StocksBatch(ctx context.Context, productIDs []string) (map[string]ProductStocks, error) {
	out := map[string]ProductStocks{}
	if len(productIDs) == 0 {
		return out, nil
	}
	rows, err := r.pool.Query(ctx, stockRowsSQL, productIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var pid, total string
		var s ProductStockRow
		if err := rows.Scan(&pid, &s.WarehouseID, &s.WarehouseName, &s.IsActive, &s.Tracked, &s.Stock, &s.MinStock, &s.Box, &s.Place, &total); err != nil {
			return nil, err
		}
		ps := out[pid]
		ps.Rows = append(ps.Rows, s)
		ps.Total = total
		out[pid] = ps
	}
	return out, rows.Err()
}

// WarehouseOption — активный склад для формы товара.
type WarehouseOption struct {
	ID   string
	Name string
}

// ActiveWarehouses — активные склады по position (форма нового товара).
func (r *ProductRepo) ActiveWarehouses(ctx context.Context) ([]WarehouseOption, error) {
	rows, err := r.pool.Query(ctx, `SELECT id::text, name FROM warehouses WHERE is_active AND deleted_at IS NULL ORDER BY position, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []WarehouseOption{}
	for rows.Next() {
		var w WarehouseOption
		if err := rows.Scan(&w.ID, &w.Name); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}
