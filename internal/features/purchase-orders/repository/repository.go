// Package repository — доступ к purchase_orders / purchase_order_items (pgx/v5).
//
// Конкурентность (§3, DS-032): каждая правка и каждый переход читают шапку заказа под
// блокировкой строки (FOR UPDATE; ответ поставщика — FOR SHARE) в одной транзакции и
// передают её в check — бизнес-правила (статус, версия, владелец) решает service. Второй
// конкурентный запрос ждёт блокировку и видит уже новый статус/версию → 409. Порядок
// блокировок везде один: заказ, затем строки — без взаимоблокировок.
package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kitavrus/kanban-tree/internal/features/board"
	"github.com/kitavrus/kanban-tree/internal/features/purchase-orders/models"
)

var (
	// ErrNotFound — заказа/строки нет (или удалён, или не виден по фильтру владельца).
	ErrNotFound = errors.New("purchase-orders: not found")
	// ErrNotDeleted — восстановление не удалённого заказа.
	ErrNotDeleted = errors.New("purchase-orders: not deleted")
	// ErrSupplierUnavailable — поставщик не найден, в архиве или удалён (§6.2).
	ErrSupplierUnavailable = errors.New("purchase-orders: supplier unavailable")
)

// ProductsUnavailableError — запчасти новых строк не найдены, в архиве или удалены (DS-055).
type ProductsUnavailableError struct {
	ProductIDs map[string]bool
}

func (e *ProductsUnavailableError) Error() string { return "purchase-orders: products unavailable" }

// HeadCheck — проверка шапки под блокировкой (service): ошибка — откат без изменений.
type HeadCheck func(h models.Head) error

// UpdateCheck — проверка правки §6.4: шапка и текущие строки заказа.
type UpdateCheck func(h models.Head, existing []models.ExistingItem) error

// AnswerCheck — проверка ответа поставщика §6.14: шапка и строка.
type AnswerCheck func(h models.Head, item models.AnswerState) error

// ReadyCheck — проверка «Собрано» §6.15: шапка и все строки.
type ReadyCheck func(h models.Head, items []models.AnswerState) error

type Repository struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

// ListFilter — список §6.1: Status "" — все; CreatedBy "" — все (admin).
type ListFilter struct {
	Status    string
	CreatedBy string
	Limit     int
	Offset    int
}

func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func scanOrder(row pgx.Row) (*models.PurchaseOrder, error) {
	var o models.PurchaseOrder
	var changedItems bool
	err := row.Scan(&o.ID, &o.Number, &o.SupplierID, &o.SupplierName, &o.SupplierPhone,
		&o.Status, &o.Note, &o.NoteChangedAt, &o.Version,
		&o.CreatedBy, &o.CreatedByName, &o.CreatedAt, &o.UpdatedAt,
		&o.SentAt, &o.ReadyAt, &o.PickedUpAt, &o.CancelledAt, &o.CancelReason,
		&o.SupplierHasLogin, &o.ItemsCount, &o.AnsweredCount, &o.TotalAmount,
		&o.NoteChanged, &changedItems)
	if err != nil {
		return nil, err
	}
	o.IsChanged = o.NoteChanged || changedItems
	return &o, nil
}

func (r *Repository) queryOrders(ctx context.Context, q string, args ...any) ([]models.PurchaseOrder, error) {
	rows, err := r.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("purchase-orders list: %w", err)
	}
	defer rows.Close()
	out := []models.PurchaseOrder{}
	for rows.Next() {
		o, err := scanOrder(rows)
		if err != nil {
			return nil, fmt.Errorf("purchase-orders list scan: %w", err)
		}
		out = append(out, *o)
	}
	return out, rows.Err()
}

// List — §6.1, без строк (агрегаты в шапке).
func (r *Repository) List(ctx context.Context, f ListFilter) ([]models.PurchaseOrder, int, error) {
	var total int
	if err := r.pool.QueryRow(ctx, countSQL, nullable(f.Status), nullable(f.CreatedBy)).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("purchase-orders count: %w", err)
	}
	out, err := r.queryOrders(ctx, listSQL, nullable(f.Status), nullable(f.CreatedBy), f.Limit, f.Offset)
	return out, total, err
}

// Get — карточка §6.3 со строками; createdBy "" — любой автор.
func (r *Repository) Get(ctx context.Context, id, createdBy string) (*models.PurchaseOrder, error) {
	return r.getOne(ctx, getSQL, id, nullable(createdBy))
}

// SupplierList — кабинет §6.12: заказы поставщика supplierID в sent/ready/picked_up.
func (r *Repository) SupplierList(ctx context.Context, supplierID, status string, limit, offset int) ([]models.PurchaseOrder, int, error) {
	var total int
	if err := r.pool.QueryRow(ctx, supplierCountSQL, supplierID, nullable(status)).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("purchase-orders supplier count: %w", err)
	}
	out, err := r.queryOrders(ctx, supplierListSQL, supplierID, nullable(status), limit, offset)
	return out, total, err
}

// SupplierGet — карточка кабинета §6.13 (только видимые статусы своего поставщика).
func (r *Repository) SupplierGet(ctx context.Context, supplierID, id string) (*models.PurchaseOrder, error) {
	return r.getOne(ctx, supplierGetSQL, supplierID, id)
}

func (r *Repository) getOne(ctx context.Context, q string, args ...any) (*models.PurchaseOrder, error) {
	o, err := scanOrder(r.pool.QueryRow(ctx, q, args...))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("purchase-orders get: %w", err)
	}
	items, err := r.items(ctx, o.ID)
	if err != nil {
		return nil, err
	}
	o.Items = items
	return o, nil
}

func (r *Repository) items(ctx context.Context, orderID string) ([]models.PurchaseOrderItem, error) {
	rows, err := r.pool.Query(ctx, itemsSQL, []string{orderID})
	if err != nil {
		return nil, fmt.Errorf("purchase-orders items: %w", err)
	}
	defer rows.Close()
	out := []models.PurchaseOrderItem{}
	for rows.Next() {
		var it models.PurchaseOrderItem
		if err := rows.Scan(&it.ID, &it.OrderID, &it.ProductID, &it.Position, &it.Qty, &it.Reason,
			&it.Name, &it.Sku, &it.Unit, &it.SupplierQty, &it.SupplierPrice, &it.SupplierNote,
			&it.SupplierAnsweredAt, &it.CustomerChangedAt, &it.Version,
			&it.LineAmount, &it.HasDiscrepancy, &it.CustomerChanged); err != nil {
			return nil, fmt.Errorf("purchase-orders items scan: %w", err)
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// ---- транзакции ----

func (r *Repository) inTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// lockHead — шапка под блокировкой lock ("FOR UPDATE OF o" / "FOR SHARE OF o"); supplierID ""
// — любой поставщик.
func lockHead(ctx context.Context, tx pgx.Tx, id, supplierID, lock string) (models.Head, error) {
	var h models.Head
	err := tx.QueryRow(ctx, headSQL+lock, id, nullable(supplierID)).Scan(&h.ID, &h.Number, &h.SupplierID, &h.Status,
		&h.Version, &h.CreatedBy, &h.Note, &h.SentAt, &h.ItemsCount, &h.SupplierHasLogin)
	if errors.Is(err, pgx.ErrNoRows) {
		return h, ErrNotFound
	}
	if err != nil {
		return h, fmt.Errorf("purchase-orders lock: %w", err)
	}
	return h, nil
}

const (
	forUpdate = "FOR UPDATE OF o"
	forShare  = "FOR SHARE OF o"
)

type productSnap struct{ name, sku, unit string }

// products — снимок активных запчастей новых строк; недоступные → ProductsUnavailableError.
func products(ctx context.Context, tx pgx.Tx, lines []models.ItemInput) (map[string]productSnap, error) {
	var ids []string
	for _, l := range lines {
		if l.ID == "" {
			ids = append(ids, l.ProductID)
		}
	}
	out := map[string]productSnap{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := tx.Query(ctx, productsSQL, ids)
	if err != nil {
		return nil, fmt.Errorf("purchase-orders products: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var p productSnap
		if err := rows.Scan(&id, &p.name, &p.sku, &p.unit); err != nil {
			return nil, err
		}
		out[id] = p
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	missing := map[string]bool{}
	for _, id := range ids {
		if _, ok := out[id]; !ok {
			missing[id] = true
		}
	}
	if len(missing) > 0 {
		return nil, &ProductsUnavailableError{ProductIDs: missing}
	}
	return out, nil
}

func insertItem(ctx context.Context, tx pgx.Tx, orderID string, pos int, l models.ItemInput, p productSnap, changed bool) error {
	_, err := tx.Exec(ctx, insertItemSQL, orderID, l.ProductID, pos, l.Qty, l.Reason, changed, p.name, p.sku, p.unit)
	if err != nil {
		return fmt.Errorf("purchase-orders insert item: %w", err)
	}
	return nil
}

// Create — черновик §6.2: шапка + строки в одной транзакции (§4.1).
func (r *Repository) Create(ctx context.Context, createdBy string, supplierID *string, note string, lines []models.ItemInput) (string, error) {
	var id string
	err := r.inTx(ctx, func(tx pgx.Tx) error {
		if supplierID != nil {
			var one int
			err := tx.QueryRow(ctx, supplierAvailableSQL, *supplierID).Scan(&one)
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrSupplierUnavailable
			}
			if err != nil {
				return err
			}
		}
		snaps, err := products(ctx, tx, lines)
		if err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, insertOrderSQL, supplierID, note, createdBy).Scan(&id); err != nil {
			return fmt.Errorf("purchase-orders insert: %w", err)
		}
		for i, l := range lines {
			if err := insertItem(ctx, tx, id, i, l, snaps[l.ProductID], false); err != nil {
				return err
			}
		}
		return nil
	})
	return id, err
}

// Update — правка §6.4: строки сверяются с lines (новые — INSERT, отсутствующие — DELETE,
// изменённые — UPDATE); в уже отправлявшемся заказе изменённые и новые строки сбрасывают
// ответ и получают «Изменено заказчиком» (DS-034, DS-048).
func (r *Repository) Update(ctx context.Context, id, note string, lines []models.ItemInput, check UpdateCheck) error {
	return r.inTx(ctx, func(tx pgx.Tx) error {
		h, err := lockHead(ctx, tx, id, "", forUpdate)
		if err != nil {
			return err
		}
		existing, err := existingItems(ctx, tx, id)
		if err != nil {
			return err
		}
		if err := check(h, existing); err != nil {
			return err
		}
		byID := make(map[string]models.ExistingItem, len(existing))
		for _, e := range existing {
			byID[e.ID] = e
		}
		keep := []string{}
		for _, l := range lines {
			if l.ID != "" {
				keep = append(keep, l.ID)
			}
		}
		if _, err := tx.Exec(ctx, deleteMissingItemsSQL, id, keep); err != nil {
			return fmt.Errorf("purchase-orders delete items: %w", err)
		}
		snaps, err := products(ctx, tx, lines)
		if err != nil {
			return err
		}
		wasSent := h.SentAt != nil
		for i, l := range lines {
			if l.ID == "" {
				if err := insertItem(ctx, tx, id, i, l, snaps[l.ProductID], wasSent); err != nil {
					return err
				}
				continue
			}
			q, args := updateItemPosSQL, []any{l.ID, id, l.Reason, i}
			if byID[l.ID].Qty != l.Qty {
				q, args = updateItemQtySQL, []any{l.ID, id, l.Qty, l.Reason, i, wasSent}
			}
			if _, err := tx.Exec(ctx, q, args...); err != nil {
				return fmt.Errorf("purchase-orders update item: %w", err)
			}
		}
		if _, err := tx.Exec(ctx, updateHeadSQL, id, note); err != nil {
			return fmt.Errorf("purchase-orders update: %w", err)
		}
		return nil
	})
}

func existingItems(ctx context.Context, tx pgx.Tx, id string) ([]models.ExistingItem, error) {
	rows, err := tx.Query(ctx, existingItemsSQL, id)
	if err != nil {
		return nil, fmt.Errorf("purchase-orders existing items: %w", err)
	}
	defer rows.Close()
	var out []models.ExistingItem
	for rows.Next() {
		var e models.ExistingItem
		if err := rows.Scan(&e.ID, &e.ProductID, &e.Qty, &e.Reason); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// transition — переход под блокировкой шапки: check, затем statements по порядку.
func (r *Repository) transition(ctx context.Context, id string, check HeadCheck, stmts ...stmt) error {
	return r.inTx(ctx, func(tx pgx.Tx) error {
		h, err := lockHead(ctx, tx, id, "", forUpdate)
		if err != nil {
			return err
		}
		if err := check(h); err != nil {
			return err
		}
		for _, s := range stmts {
			if _, err := tx.Exec(ctx, s.sql, s.args...); err != nil {
				return fmt.Errorf("purchase-orders transition: %w", err)
			}
		}
		return nil
	})
}

type stmt struct {
	sql  string
	args []any
}

// Send — draft → sent (§6.6): снимок name/sku/unit обновляется (DS-054).
func (r *Repository) Send(ctx context.Context, id, actorID string, check HeadCheck) error {
	return r.transition(ctx, id, check, stmt{refreshSnapshotSQL, []any{id}}, stmt{sendSQL, []any{id, actorID}})
}

// Recall — sent → draft (§6.7): ответы сохраняются (DS-035).
func (r *Repository) Recall(ctx context.Context, id string, check HeadCheck) error {
	return r.transition(ctx, id, check, stmt{recallSQL, []any{id}})
}

// PickUp — → picked_up (§6.8): остатки не меняются (DS-010).
func (r *Repository) PickUp(ctx context.Context, id, actorID string, check HeadCheck) error {
	return r.transition(ctx, id, check, stmt{pickUpSQL, []any{id, actorID}})
}

// Cancel — → cancelled (§6.9).
func (r *Repository) Cancel(ctx context.Context, id, actorID, reason string, check HeadCheck) error {
	return r.transition(ctx, id, check, stmt{cancelSQL, []any{id, actorID, reason}})
}

// SoftDelete — мягкое удаление черновика (§6.5).
func (r *Repository) SoftDelete(ctx context.Context, id, actorID, reason string, check HeadCheck) error {
	return r.transition(ctx, id, check, stmt{softDeleteSQL, []any{id, actorID, reason}})
}

// Restore — восстановление из раздела «Удалённые» (DS-072).
func (r *Repository) Restore(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, restoreSQL, id)
	if err != nil {
		return fmt.Errorf("purchase-orders restore: %w", err)
	}
	if tag.RowsAffected() > 0 {
		return nil
	}
	var one int
	err = r.pool.QueryRow(ctx, `SELECT 1 FROM purchase_orders WHERE id = $1::uuid`, id).Scan(&one)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	return ErrNotDeleted
}

// Purge — физическое удаление (служебная уборка автотестов, soft-delete-core DS-027);
// строки — каскадом (DS-070).
func (r *Repository) Purge(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM purchase_orders WHERE id = $1::uuid`, id)
	if err != nil {
		return fmt.Errorf("purchase-orders purge: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ---- кабинет поставщика ----

// Answer — ответ по строке §6.14: шапка FOR SHARE (против одновременных переходов), строка
// FOR UPDATE (версия строки). Возвращает обновлённую строку и итог заказа.
func (r *Repository) Answer(ctx context.Context, supplierID, orderID, itemID, actorID string, a models.SupplierAnswer, check AnswerCheck) (*models.PurchaseOrderItem, *string, error) {
	err := r.inTx(ctx, func(tx pgx.Tx) error {
		h, err := lockHead(ctx, tx, orderID, supplierID, forShare)
		if err != nil {
			return err
		}
		var st models.AnswerState
		err = tx.QueryRow(ctx, answerItemSQL, itemID, orderID).Scan(&st.ItemID, &st.Version, &st.SupplierQty, &st.SupplierPrice)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("purchase-orders answer item: %w", err)
		}
		if err := check(h, st); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, answerSQL, itemID, orderID, a.SupplierQty, a.SupplierPrice, a.SupplierNote, actorID)
		if err != nil {
			return fmt.Errorf("purchase-orders answer: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	items, err := r.items(ctx, orderID)
	if err != nil {
		return nil, nil, err
	}
	var total *string
	if err := r.pool.QueryRow(ctx, orderTotalSQL, orderID).Scan(&total); err != nil {
		return nil, nil, err
	}
	for i := range items {
		if items[i].ID == itemID {
			return &items[i], total, nil
		}
	}
	return nil, nil, ErrNotFound
}

// Ready — «Собрано» §6.15 (sent → ready) под блокировкой шапки своего поставщика.
func (r *Repository) Ready(ctx context.Context, supplierID, id, actorID string, check ReadyCheck) error {
	return r.inTx(ctx, func(tx pgx.Tx) error {
		h, err := lockHead(ctx, tx, id, supplierID, forUpdate)
		if err != nil {
			return err
		}
		rows, err := tx.Query(ctx, answerStatesSQL, id)
		if err != nil {
			return err
		}
		var states []models.AnswerState
		for rows.Next() {
			var st models.AnswerState
			if err := rows.Scan(&st.ItemID, &st.Version, &st.SupplierQty, &st.SupplierPrice); err != nil {
				rows.Close()
				return err
			}
			states = append(states, st)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		if err := check(h, states); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, readySQL, id, actorID); err != nil {
			return fmt.Errorf("purchase-orders ready: %w", err)
		}
		return nil
	})
}

// ---- подсказка дефицита §6.10 ----

// suggestionsSQL — to_order = max(0, pending + min_stock − stock); pending — общий CTE «Остатков»
// (board.PendingCTE, DS-064). $1 тип (NULL — все), $2 limit, $3 offset.
const suggestionsSQL = board.PendingCTE + `,
deficit AS (
    SELECT p.id, p.name, p.sku, p.unit, p.stock, p.min_stock, coalesce(pd.qty, 0) AS pending,
           coalesce(pd.qty, 0) + p.min_stock - p.stock AS to_order,
           CASE WHEN coalesce(pd.qty, 0) > p.stock THEN 'for_orders' ELSE 'below_min' END AS reason
    FROM products p LEFT JOIN pending pd ON pd.product_id = p.id
    WHERE p.is_active AND p.deleted_at IS NULL)
SELECT id::text, name, sku, unit, stock::text, min_stock::text, pending::text, to_order::text, reason,
       count(*) OVER ()
FROM deficit
WHERE to_order > 0 AND ($1::text IS NULL OR reason = $1)
ORDER BY (reason = 'for_orders') DESC, lower(name), id
LIMIT $2 OFFSET $3`

// Suggestions — строки дефицита; createdBy "" — «уже в открытом заказе» по всем заказам
// (admin), иначе только по заказам этого автора (executor, DS-078).
func (r *Repository) Suggestions(ctx context.Context, reason, createdBy string, limit, offset int) ([]models.Suggestion, int, error) {
	rows, err := r.pool.Query(ctx, suggestionsSQL, nullable(reason), limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("purchase-orders suggestions: %w", err)
	}
	out := []models.Suggestion{}
	total := 0
	for rows.Next() {
		var s models.Suggestion
		if err := rows.Scan(&s.ProductID, &s.Name, &s.Sku, &s.Unit, &s.Stock, &s.MinStock, &s.Pending,
			&s.ToOrder, &s.Reason, &total); err != nil {
			rows.Close()
			return nil, 0, err
		}
		out = append(out, s)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if len(out) == 0 {
		return out, 0, nil
	}
	ids := make([]string, len(out))
	idx := make(map[string]int, len(out))
	for i := range out {
		ids[i] = out[i].ProductID
		idx[out[i].ProductID] = i
	}
	trows, err := r.pool.Query(ctx, board.PendingTasksSQL, ids)
	if err != nil {
		return nil, 0, fmt.Errorf("purchase-orders suggestion tasks: %w", err)
	}
	for trows.Next() {
		var pid string
		var t models.SuggestionTask
		if err := trows.Scan(&pid, &t.ID, &t.Title); err != nil {
			trows.Close()
			return nil, 0, err
		}
		out[idx[pid]].Tasks = append(out[idx[pid]].Tasks, t)
	}
	trows.Close()
	if err := trows.Err(); err != nil {
		return nil, 0, err
	}
	orows, err := r.pool.Query(ctx, openOrdersSQL, ids, nullable(createdBy))
	if err != nil {
		return nil, 0, fmt.Errorf("purchase-orders open orders: %w", err)
	}
	defer orows.Close()
	for orows.Next() {
		var pid string
		var ref models.OpenOrderRef
		if err := orows.Scan(&pid, &ref.ID, &ref.Number, &ref.Qty); err != nil {
			return nil, 0, err
		}
		out[idx[pid]].InOpenOrders = append(out[idx[pid]].InOpenOrders, ref)
	}
	return out, total, orows.Err()
}

// SupplierOption — получатель в выпадающем списке «Новый заказ» (§8.2).
type SupplierOption struct {
	ID   string
	Name string
}

// ActiveSuppliers — активные, не удалённые поставщики по алфавиту (§8.2).
func (r *Repository) ActiveSuppliers(ctx context.Context) ([]SupplierOption, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id::text, name FROM suppliers WHERE is_active AND deleted_at IS NULL ORDER BY lower(name), id`)
	if err != nil {
		return nil, fmt.Errorf("purchase-orders suppliers: %w", err)
	}
	defer rows.Close()
	out := []SupplierOption{}
	for rows.Next() {
		var o SupplierOption
		if err := rows.Scan(&o.ID, &o.Name); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}
