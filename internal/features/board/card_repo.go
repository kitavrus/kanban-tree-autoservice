package board

import (
	"context"
	"database/sql"
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

// Car — запрос 3: автомобиль заказа (tasks.car_id). Условие «автомобиль не удалён» НЕ
// ставится (soft-delete-clients DS-014): мягко удалённый автомобиль остаётся в заказе и
// отдаётся с is_deleted = true. 0 строк (заказ удалён параллельно) → errNotFound.
func (r *CardRepo) Car(ctx context.Context, taskID int64) (*CardCar, error) {
	var c CardCar
	err := r.pool.QueryRow(ctx, `
		SELECT e.id, e.plate_number, e.deleted_at IS NOT NULL
		FROM tasks t JOIN equipment e ON e.id = t.car_id
		WHERE t.id = $1 AND t.deleted_at IS NULL`, taskID).Scan(&c.EquipmentID, &c.PlateNumber, &c.IsDeleted)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errNotFound
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// Subtasks — запрос 4 + subtask_total (design-go-layers.md §Card Queries: Go десятичной
// арифметики не делает, скаляр считает SQL). Products/Logs подсовывает сервис (Card()).
func (r *CardRepo) Subtasks(ctx context.Context, taskID int64) ([]CardSubtask, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT s.id, s.service_id, sv.name, s.status, s.assignee_id::text, u.name, u.is_active,
		       s.due_date::text, s.price::text, s.position,
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
		var assigneeID, assigneeName, dueDate, price, payout sql.NullString
		var assigneeActive sql.NullBool
		var total string
		if err := rows.Scan(&c.ID, &c.ServiceID, &c.ServiceName, &c.Status, &assigneeID, &assigneeName,
			&assigneeActive, &dueDate, &price, &c.Position, &total,
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
		       round(sp.qty * coalesce(sp.price, 0), 2)::text, p.deleted_at IS NOT NULL
		FROM subtask_products sp JOIN subtasks s ON s.id = sp.subtask_id JOIN products p ON p.id = sp.product_id
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
			&qty, &price, &lineTotal, &pr.ProductIsDeleted); err != nil {
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

// Stock — запрос 7, остатки товаров, задействованных в карточке (AC-009): товары строк
// заказа + уже списанные (task_stock).
func (r *CardRepo) Stock(ctx context.Context, taskID int64) ([]CardStock, error) {
	rows, err := r.pool.Query(ctx, `
		WITH ids AS (
		    SELECT sp.product_id FROM subtask_products sp JOIN subtasks s ON s.id = sp.subtask_id WHERE s.task_id = $1
		    UNION
		    SELECT product_id FROM task_stock WHERE task_id = $1)
		SELECT p.id, p.stock::text, p.min_stock::text, round(coalesce(ts.qty, 0), 3)::text
		FROM ids JOIN products p ON p.id = ids.product_id
		LEFT JOIN task_stock ts ON ts.task_id = $1 AND ts.product_id = p.id
		ORDER BY p.id`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []CardStock{}
	for rows.Next() {
		var cs CardStock
		var stock, minStock, reserved string
		if err := rows.Scan(&cs.ProductID, &stock, &minStock, &reserved); err != nil {
			return nil, err
		}
		cs.Stock, cs.MinStock, cs.TaskReserved = numeric.Amount(stock), numeric.Amount(minStock), numeric.Amount(reserved)
		out = append(out, cs)
	}
	return out, rows.Err()
}

// PrintHeader — design-sql.md §Print Header Query (шапка акта, фаза 04).
func (r *CardRepo) PrintHeader(ctx context.Context, taskID int64) (*printHeader, error) {
	var h printHeader
	err := r.pool.QueryRow(ctx, `
		SELECT cl.company_name,
		       coalesce((SELECT string_agg(ph.phone, ', ' ORDER BY ph.position) FROM client_phones ph WHERE ph.client_id = cl.id), '')
		FROM tasks t JOIN clients cl ON cl.id = t.client_id WHERE t.id = $1 AND t.deleted_at IS NULL`, taskID).
		Scan(&h.CompanyName, &h.Phones)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errNotFound
	}
	return &h, err
}
