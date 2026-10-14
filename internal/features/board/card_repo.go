package board

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kitavrus/kanban-tree/pkg/numeric"
)

// CardRepo — чтение карточки заказа (design-sql.md §Card Queries, §Print Header Query):
// 7 запросов на пуле, без tx (read committed), без N+1. Запись строк услуг/товаров/
// журнала — subtasks_repo.go/subtask_products_repo.go/subtask_logs_repo.go (фаза 03).
type CardRepo struct {
	pool *pgxpool.Pool
}

func NewCardRepo(pool *pgxpool.Pool) *CardRepo { return &CardRepo{pool: pool} }

// printHeader — design-sql.md §Print Header Query, наполняет PrintVM (print_ssr.go, фаза 04).
type printHeader struct {
	CompanyName string
	ContactName string
	Phones      string
}

// subtaskLogRow — строка журнала работ (запрос 6), CanEdit считает сервис (нужен Actor).
type subtaskLogRow struct {
	ID         string
	SubtaskID  string
	Text       string
	UserID     string
	AuthorName string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// Column — запрос 2 (design-sql.md §Card Queries).
func (r *CardRepo) Column(ctx context.Context, id string) (*CardColumn, error) {
	var c CardColumn
	err := r.pool.QueryRow(ctx, `SELECT id, name, is_final, consumes_stock FROM columns WHERE id = $1 AND deleted_at IS NULL`, id).
		Scan(&c.ID, &c.Name, &c.IsFinal, &c.ConsumesStock)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errNotFound
	}
	return &c, err
}

// Equipment — запрос 3, история других заказов с той же единицей — json_agg (AC-006).
// Условие «единица не удалена» в соединении НЕ ставится (soft-delete-clients DS-014): мягко
// удалённая единица остаётся в заказе и отдаётся с is_deleted = true.
func (r *CardRepo) Equipment(ctx context.Context, taskID int64) ([]CardEquipment, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT te.equipment_id, e.deleted_at IS NOT NULL, et.name, e.brand, e.model, e.serial, te.completeness, te.condition,
		       coalesce((SELECT json_agg(json_build_object('task_id', o.id, 'title', o.title, 'created_at', o.created_at) ORDER BY o.id DESC)
		                 FROM task_equipment x JOIN tasks o ON o.id = x.task_id
		                 WHERE x.equipment_id = te.equipment_id AND x.task_id <> te.task_id
		                   AND o.deleted_at IS NULL), '[]'::json),
		       e.location_status, e.warehouse_id::text, wc.name, e.active_transfer_id,
		       CASE WHEN e.location_status <> 'in_transit' THEN NULL
		            WHEN tr.direction = 'warehouse_to_client' THEN 'клиент'
		            ELSE wt.name END,
		       tr.direction
		FROM task_equipment te JOIN equipment e ON e.id = te.equipment_id JOIN equipment_types et ON et.id = e.type_id
		LEFT JOIN warehouses wc ON wc.id = e.warehouse_id
		LEFT JOIN transfers tr ON tr.id = e.active_transfer_id
		LEFT JOIN warehouses wt ON wt.id = tr.to_warehouse_id
		WHERE te.task_id = $1 ORDER BY et.name, e.brand, e.model, e.id`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []CardEquipment{}
	for rows.Next() {
		var eq CardEquipment
		var historyJSON []byte
		if err := rows.Scan(&eq.EquipmentID, &eq.IsDeleted, &eq.TypeName, &eq.Brand, &eq.Model, &eq.Serial,
			&eq.Completeness, &eq.Condition, &historyJSON,
			&eq.LocationStatus, &eq.WarehouseID, &eq.WarehouseName, &eq.ActiveTransferID, &eq.TransitToName,
			&eq.ActiveTransferDirection); err != nil {
			return nil, err
		}
		eq.History = []EquipmentHistory{}
		if len(historyJSON) > 0 {
			if err := json.Unmarshal(historyJSON, &eq.History); err != nil {
				return nil, err
			}
		}
		out = append(out, eq)
	}
	return out, rows.Err()
}

// Transfers — действующие документы «Перемещение» заказа (transfers-partners DS-012), новые
// сверху; мягко удалённые не входят (soft-delete-transfers DS-026).
// Склад — назначения (приём от клиента) либо отправления (возврат клиенту).
func (r *CardRepo) Transfers(ctx context.Context, taskID int64) ([]CardTransfer, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT t.id, t.direction, t.status, coalesce(w.name, ''),
		       (SELECT count(*) FROM transfer_items i WHERE i.transfer_id = t.id)::int, t.created_at,
		       (SELECT round(coalesce(sum(round(i.price * i.qty_sent, 2)), 0), 2)
		        FROM transfer_items i WHERE i.transfer_id = t.id AND i.kind = 'product')::text
		FROM transfers t
		LEFT JOIN warehouses w ON w.id = coalesce(t.to_warehouse_id, t.from_warehouse_id)
		WHERE t.task_id = $1 AND t.deleted_at IS NULL ORDER BY t.id DESC`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []CardTransfer{}
	for rows.Next() {
		var t CardTransfer
		var amount string
		if err := rows.Scan(&t.ID, &t.Direction, &t.Status, &t.WarehouseName, &t.ItemsCount, &t.CreatedAt, &amount); err != nil {
			return nil, err
		}
		t.Amount = numeric.Amount(amount)
		out = append(out, t)
	}
	return out, rows.Err()
}

// Subtasks — запрос 4 + subtask_total (design-go-layers.md §Card Queries: Go десятичной
// арифметики не делает, скаляр считает SQL). Products/Logs подсовывает сервис (Card()).
func (r *CardRepo) Subtasks(ctx context.Context, taskID int64) ([]CardSubtask, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT s.id, s.service_id, sv.name, s.status, s.assignee_id::text, u.name, u.is_active,
		       s.due_date::text, s.price::text, s.equipment_id::text, s.position,
		       round(coalesce(s.price, 0) + coalesce((SELECT sum(round(sp.qty * coalesce(sp.price, 0), 2))
		                FROM subtask_products sp WHERE sp.subtask_id = s.id), 0), 2)::text AS subtask_total,
		       s.is_executor_paid, s.executor_paid_at, s.executor_payout::text, sv.deleted_at IS NOT NULL
		FROM subtasks s JOIN services sv ON sv.id = s.service_id LEFT JOIN users u ON u.id = s.assignee_id
		WHERE s.task_id = $1 ORDER BY s.position, s.id`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []CardSubtask{}
	for rows.Next() {
		var c CardSubtask
		var assigneeID, assigneeName, dueDate, equipmentID, price, payout sql.NullString
		var assigneeActive sql.NullBool
		var total string
		if err := rows.Scan(&c.ID, &c.ServiceID, &c.ServiceName, &c.Status, &assigneeID, &assigneeName,
			&assigneeActive, &dueDate, &price, &equipmentID, &c.Position, &total,
			&c.IsExecutorPaid, &c.ExecutorPaidAt, &payout, &c.ServiceIsDeleted); err != nil {
			return nil, err
		}
		if payout.Valid {
			v := numeric.Amount(payout.String)
			c.ExecutorPayout = &v
		}
		if assigneeID.Valid {
			v := assigneeID.String
			c.AssigneeID = &v
		}
		if assigneeName.Valid {
			v := assigneeName.String
			c.AssigneeName = &v
		}
		if assigneeActive.Valid {
			v := assigneeActive.Bool
			c.AssigneeIsActive = &v
		}
		if dueDate.Valid {
			v := dueDate.String
			c.DueDate = &v
		}
		if price.Valid {
			v := numeric.Amount(price.String)
			c.Price = &v
		}
		if equipmentID.Valid {
			v := equipmentID.String
			c.EquipmentID = &v
		}
		c.Total = numeric.Amount(total)
		c.Products = []CardSubtaskProduct{}
		c.Logs = []CardSubtaskLog{}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Products — запрос 5, сгруппированы по subtask_id (без N+1 — один запрос на все услуги).
func (r *CardRepo) Products(ctx context.Context, taskID int64) (map[string][]CardSubtaskProduct, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT sp.id, sp.subtask_id, sp.product_id, p.name, p.unit, sp.position, sp.qty::text, sp.price::text,
		       round(sp.qty * coalesce(sp.price, 0), 2)::text, sp.warehouse_id::text, w.name, p.deleted_at IS NOT NULL
		FROM subtask_products sp JOIN subtasks s ON s.id = sp.subtask_id JOIN products p ON p.id = sp.product_id
		JOIN warehouses w ON w.id = sp.warehouse_id
		WHERE s.task_id = $1 ORDER BY sp.subtask_id, sp.position, sp.id`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string][]CardSubtaskProduct{}
	for rows.Next() {
		var subtaskID string
		var pr CardSubtaskProduct
		var qty string
		var price sql.NullString
		var lineTotal string
		if err := rows.Scan(&pr.ID, &subtaskID, &pr.ProductID, &pr.Name, &pr.Unit, &pr.Position,
			&qty, &price, &lineTotal, &pr.WarehouseID, &pr.WarehouseName, &pr.ProductIsDeleted); err != nil {
			return nil, err
		}
		pr.Qty = numeric.Amount(qty)
		if price.Valid {
			v := numeric.Amount(price.String)
			pr.Price = &v
		}
		pr.LineTotal = numeric.Amount(lineTotal)
		out[subtaskID] = append(out[subtaskID], pr)
	}
	return out, rows.Err()
}

// Logs — запрос 6, сгруппированы по subtask_id.
func (r *CardRepo) Logs(ctx context.Context, taskID int64) (map[string][]subtaskLogRow, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT l.id, l.subtask_id, l.text, l.user_id, u.name, l.created_at, l.updated_at
		FROM subtask_logs l JOIN subtasks s ON s.id = l.subtask_id JOIN users u ON u.id = l.user_id
		WHERE s.task_id = $1 ORDER BY l.subtask_id, l.created_at, l.id`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string][]subtaskLogRow{}
	for rows.Next() {
		var lg subtaskLogRow
		if err := rows.Scan(&lg.ID, &lg.SubtaskID, &lg.Text, &lg.UserID, &lg.AuthorName,
			&lg.CreatedAt, &lg.UpdatedAt); err != nil {
			return nil, err
		}
		out[lg.SubtaskID] = append(out[lg.SubtaskID], lg)
	}
	return out, rows.Err()
}

// Stock — запрос 7, остатки пар товар×склад, задействованных в карточке (AC-009,
// DS-005): пары строк товаров заказа + пары уже списанного (task_stock); отсутствующая
// строка product_stock = 0/0.
func (r *CardRepo) Stock(ctx context.Context, taskID int64) ([]CardStock, error) {
	rows, err := r.pool.Query(ctx, `
		WITH pairs AS (
		    SELECT sp.product_id, sp.warehouse_id FROM subtask_products sp JOIN subtasks s ON s.id = sp.subtask_id WHERE s.task_id = $1
		    UNION
		    SELECT product_id, warehouse_id FROM task_stock WHERE task_id = $1)
		SELECT pr.product_id, pr.warehouse_id, coalesce(ps.stock, 0)::text, coalesce(ps.min_stock, 0)::text,
		       round(coalesce(ts.qty, 0), 3)::text
		FROM pairs pr
		LEFT JOIN product_stock ps ON ps.product_id = pr.product_id AND ps.warehouse_id = pr.warehouse_id
		LEFT JOIN task_stock ts ON ts.task_id = $1 AND ts.product_id = pr.product_id AND ts.warehouse_id = pr.warehouse_id
		ORDER BY pr.product_id, pr.warehouse_id`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []CardStock{}
	for rows.Next() {
		var cs CardStock
		var stock, minStock, reserved string
		if err := rows.Scan(&cs.ProductID, &cs.WarehouseID, &stock, &minStock, &reserved); err != nil {
			return nil, err
		}
		cs.Stock, cs.MinStock, cs.TaskReserved = numeric.Amount(stock), numeric.Amount(minStock), numeric.Amount(reserved)
		out = append(out, cs)
	}
	return out, rows.Err()
}

// Warehouses — склады select'а строки товара: активные + архивные и мягко удалённые (is_deleted,
// soft-delete-warehouses-suppliers DS-014), на которые ссылаются строки этого заказа (иначе
// выбранный склад пропал бы из select'а; удалённый — показывается с бейджем, в выбор не входит). Склады единиц
// оборудования заказа (DS-008) — тоже, чтобы бейдж показал архивный склад.
func (r *CardRepo) Warehouses(ctx context.Context, taskID int64) ([]CardWarehouse, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT w.id, w.name, w.is_active, w.deleted_at IS NOT NULL FROM warehouses w
		WHERE (w.is_active AND w.deleted_at IS NULL) OR w.id IN (
		    SELECT sp.warehouse_id FROM subtask_products sp JOIN subtasks s ON s.id = sp.subtask_id WHERE s.task_id = $1
		    UNION SELECT e.warehouse_id FROM task_equipment te JOIN equipment e ON e.id = te.equipment_id
		          WHERE te.task_id = $1)
		ORDER BY w.position, w.id`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []CardWarehouse{}
	for rows.Next() {
		var w CardWarehouse
		if err := rows.Scan(&w.ID, &w.Name, &w.IsActive, &w.IsDeleted); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// AllWarehouses — действующие склады по position (вкл. архивные), для фильтра «Склад» доски
// (DS-009): архивный склад может ещё держать оборудование; мягко удалённых в списке нет
// (soft-delete-warehouses-suppliers §8.3) — на них оборудования не бывает.
func (r *CardRepo) AllWarehouses(ctx context.Context) ([]CardWarehouse, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, name, is_active FROM warehouses WHERE deleted_at IS NULL ORDER BY position, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []CardWarehouse{}
	for rows.Next() {
		var w CardWarehouse
		if err := rows.Scan(&w.ID, &w.Name, &w.IsActive); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// PrintHeader — design-sql.md §Print Header Query (шапка акта, фаза 04).
func (r *CardRepo) PrintHeader(ctx context.Context, taskID int64) (*printHeader, error) {
	var h printHeader
	err := r.pool.QueryRow(ctx, `
		SELECT cl.company_name, cl.contact_name,
		       coalesce((SELECT string_agg(ph.phone, ', ' ORDER BY ph.position) FROM client_phones ph WHERE ph.client_id = cl.id), '')
		FROM tasks t JOIN clients cl ON cl.id = t.client_id WHERE t.id = $1 AND t.deleted_at IS NULL`, taskID).
		Scan(&h.CompanyName, &h.ContactName, &h.Phones)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errNotFound
	}
	return &h, err
}
