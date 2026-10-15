package clients

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/kitavrus/kanban-tree/internal/ssr"
	"github.com/kitavrus/kanban-tree/pkg/numeric"
)

// История автомобиля: его заказы (заказ = ровно один автомобиль, tasks.car_id — one-car-order)
// + все услуги этих заказов, товары услуг и журнал работ. Таблицы доски читаются напрямую
// SQL (как TypeOptions, ADR-102), без Go-импорта пакета board.

// HistoryOrder — заказ автомобиля.
type HistoryOrder struct {
	TaskID      int64
	Title       string
	ColumnName  string
	ColumnColor string
	IsFinal     bool
	IsPaid      bool
	CreatedAt   time.Time
	Services    []HistoryService
}

// HistoryService — услуга заказа автомобиля.
type HistoryService struct {
	ID       string
	Name     string
	Status   string
	Assignee string
	Price    string
	DueDate  *time.Time
	Products []HistoryProduct
	Logs     []HistoryLog
}

// StatusLabel / StatusClass — подпись и цвет бейджа статуса услуги (new/in_progress/done).
func (s HistoryService) StatusLabel() string {
	switch s.Status {
	case "done":
		return "Выполнена"
	case "in_progress":
		return "В работе"
	}
	return "Новая"
}

func (s HistoryService) StatusClass() string {
	switch s.Status {
	case "done":
		return "text-bg-success"
	case "in_progress":
		return "text-bg-primary"
	}
	return "text-bg-secondary"
}

type HistoryProduct struct {
	Name  string
	Unit  string
	Qty   string
	Price string
}

type HistoryLog struct {
	Author    string
	Text      string
	CreatedAt time.Time
}

// EquipmentHistory — заказы автомобиля, новые сверху; услуги внутри заказа — по позиции.
// Мягко удалённые заказы в истории не показываются (soft-delete-core DS-010): услуги, товары
// и записи ниже привязываются к заказам первого запроса.
func (r *Repo) EquipmentHistory(ctx context.Context, equipmentID string) ([]HistoryOrder, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT t.id, t.title, c.name, c.color, c.is_final, t.is_paid, t.created_at
		FROM tasks t
		JOIN columns c ON c.id = t.column_id
		WHERE t.car_id = $1::uuid AND t.deleted_at IS NULL
		ORDER BY t.created_at DESC, t.id DESC`, equipmentID)
	if err != nil {
		return nil, err
	}
	orders := []HistoryOrder{}
	idx := map[int64]int{}
	for rows.Next() {
		var o HistoryOrder
		if err := rows.Scan(&o.TaskID, &o.Title, &o.ColumnName, &o.ColumnColor, &o.IsFinal, &o.IsPaid,
			&o.CreatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		idx[o.TaskID] = len(orders)
		orders = append(orders, o)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(orders) == 0 {
		return orders, nil
	}

	type svcRef struct{ order, svc int }
	svcIdx := map[string]svcRef{}
	rows, err = r.pool.Query(ctx, `
		SELECT s.id, s.task_id, sv.name, s.status, coalesce(nullif(u.name, ''), u.login, ''),
		       coalesce(s.price::text, ''), s.due_date
		FROM subtasks s
		JOIN tasks t ON t.id = s.task_id
		JOIN services sv ON sv.id = s.service_id
		LEFT JOIN users u ON u.id = s.assignee_id
		WHERE t.car_id = $1::uuid
		ORDER BY s.task_id, s.position, s.id`, equipmentID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var s HistoryService
		var taskID int64
		if err := rows.Scan(&s.ID, &taskID, &s.Name, &s.Status, &s.Assignee, &s.Price, &s.DueDate); err != nil {
			rows.Close()
			return nil, err
		}
		oi, ok := idx[taskID]
		if !ok {
			continue
		}
		s.Price = trimNumber(s.Price)
		svcIdx[s.ID] = svcRef{oi, len(orders[oi].Services)}
		orders[oi].Services = append(orders[oi].Services, s)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(svcIdx) == 0 {
		return orders, nil
	}

	rows, err = r.pool.Query(ctx, `
		SELECT sp.subtask_id, p.name, p.unit, sp.qty::text, coalesce(sp.price::text, '')
		FROM subtask_products sp
		JOIN subtasks s ON s.id = sp.subtask_id
		JOIN tasks t ON t.id = s.task_id
		JOIN products p ON p.id = sp.product_id
		WHERE t.car_id = $1::uuid
		ORDER BY sp.subtask_id, sp.position, sp.id`, equipmentID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var sid string
		var p HistoryProduct
		if err := rows.Scan(&sid, &p.Name, &p.Unit, &p.Qty, &p.Price); err != nil {
			rows.Close()
			return nil, err
		}
		if ref, ok := svcIdx[sid]; ok {
			p.Qty, p.Price = trimNumber(p.Qty), trimNumber(p.Price)
			s := &orders[ref.order].Services[ref.svc]
			s.Products = append(s.Products, p)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	rows, err = r.pool.Query(ctx, `
		SELECT l.subtask_id, coalesce(nullif(u.name, ''), u.login), l.text, l.created_at
		FROM subtask_logs l
		JOIN subtasks s ON s.id = l.subtask_id
		JOIN tasks t ON t.id = s.task_id
		JOIN users u ON u.id = l.user_id
		WHERE t.car_id = $1::uuid
		ORDER BY l.created_at, l.id`, equipmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var sid string
		var l HistoryLog
		if err := rows.Scan(&sid, &l.Author, &l.Text, &l.CreatedAt); err != nil {
			return nil, err
		}
		if ref, ok := svcIdx[sid]; ok {
			s := &orders[ref.order].Services[ref.svc]
			s.Logs = append(s.Logs, l)
		}
	}
	return orders, rows.Err()
}

// trimNumber — numeric::text без хвостовых нулей: «2.000» → «2», «1500.50» → «1500.5».
func trimNumber(s string) string {
	if !strings.Contains(s, ".") {
		return s
	}
	s = strings.TrimRight(s, "0")
	return strings.TrimSuffix(s, ".")
}

// EquipmentHistoryData — единица и её заказы.
type EquipmentHistoryData struct {
	Equipment EquipmentResponse
	Orders    []HistoryOrder
}

// EquipmentHistory — единица + её заказы. Невалидный/несуществующий id → ErrEquipmentNotFound.
func (s *Service) EquipmentHistory(ctx context.Context, id string) (*EquipmentHistoryData, error) {
	e, err := s.GetEquipment(ctx, id)
	if err != nil {
		return nil, err
	}
	orders, err := s.repo.EquipmentHistory(ctx, id)
	if err != nil {
		return nil, err
	}
	return &EquipmentHistoryData{Equipment: *e, Orders: orders}, nil
}

// EquipmentHistoryVM — страница «История оборудования».
type EquipmentHistoryVM struct {
	Shell      ssr.Shell
	Equipment  EquipmentResponse
	ClientName string
	Orders     []HistoryOrder
	Services   int
}

// eqHistory — GET /equipment/:id/history.
func (h *SSR) eqHistory(c fiber.Ctx) error {
	d, err := h.svc.EquipmentHistory(c.Context(), c.Params("id"))
	if errors.Is(err, ErrEquipmentNotFound) {
		return h.status.NotFound(c)
	}
	if err != nil {
		return err
	}
	vm := EquipmentHistoryVM{Equipment: d.Equipment, Orders: d.Orders}
	if cl, err := h.svc.Get(c.Context(), d.Equipment.ClientID); err == nil {
		vm.ClientName = cl.CompanyName
	}
	for _, o := range d.Orders {
		vm.Services += len(o.Services)
	}
	vm.Shell = h.shell.Build(c, "История оборудования")
	return h.r.Page(c, "equipment-history", vm)
}

// --- JSON API: GET /api/v1/equipment/:id/history (§API-1: snake_case, список —
// {items, total, limit, offset}; отдаётся целиком, без пагинации — история одной единицы
// невелика). Карточка заказа раскрывает по нему «Историю работ» (доступно и executor). ---

type HistoryOrderDTO struct {
	TaskID      int64               `json:"task_id"`
	Title       string              `json:"title"`
	ColumnName  string              `json:"column_name"`
	ColumnColor string              `json:"column_color"`
	IsFinal     bool                `json:"is_final"`
	IsPaid      bool                `json:"is_paid"`
	CreatedAt   time.Time           `json:"created_at"`
	Services    []HistoryServiceDTO `json:"services"`
}

type HistoryServiceDTO struct {
	ID           string              `json:"id"`
	Name         string              `json:"name"`
	Status       string              `json:"status"`
	AssigneeName string              `json:"assignee_name"`
	Price        *numeric.Amount     `json:"price"` // null — цена не указана
	DueDate      *string             `json:"due_date"`
	Products     []HistoryProductDTO `json:"products"`
	Logs         []HistoryLogDTO     `json:"logs"`
}

type HistoryProductDTO struct {
	Name  string          `json:"name"`
	Unit  string          `json:"unit"`
	Qty   numeric.Amount  `json:"qty"`
	Price *numeric.Amount `json:"price"`
}

type HistoryLogDTO struct {
	AuthorName string    `json:"author_name"`
	Text       string    `json:"text"`
	CreatedAt  time.Time `json:"created_at"`
}

type EquipmentHistoryListResponse struct {
	Items  []HistoryOrderDTO `json:"items"`
	Total  int               `json:"total"`
	Limit  int               `json:"limit"`
	Offset int               `json:"offset"`
}

func amountPtr(s string) *numeric.Amount {
	if s == "" {
		return nil
	}
	a := numeric.Amount(s)
	return &a
}

func historyDTO(d *EquipmentHistoryData) EquipmentHistoryListResponse {
	orders := d.Orders
	items := make([]HistoryOrderDTO, len(orders))
	for i, o := range orders {
		svcs := make([]HistoryServiceDTO, len(o.Services))
		for j, s := range o.Services {
			prods := make([]HistoryProductDTO, len(s.Products))
			for k, p := range s.Products {
				prods[k] = HistoryProductDTO{Name: p.Name, Unit: p.Unit, Qty: numeric.Amount(p.Qty), Price: amountPtr(p.Price)}
			}
			logs := make([]HistoryLogDTO, len(s.Logs))
			for k, l := range s.Logs {
				logs[k] = HistoryLogDTO{AuthorName: l.Author, Text: l.Text, CreatedAt: l.CreatedAt}
			}
			var due *string
			if s.DueDate != nil {
				d := s.DueDate.Format(time.DateOnly)
				due = &d
			}
			svcs[j] = HistoryServiceDTO{ID: s.ID, Name: s.Name, Status: s.Status, AssigneeName: s.Assignee,
				Price: amountPtr(s.Price), DueDate: due, Products: prods, Logs: logs}
		}
		items[i] = HistoryOrderDTO{TaskID: o.TaskID, Title: o.Title, ColumnName: o.ColumnName,
			ColumnColor: o.ColumnColor, IsFinal: o.IsFinal, IsPaid: o.IsPaid, CreatedAt: o.CreatedAt,
			Services: svcs}
	}
	return EquipmentHistoryListResponse{Items: items, Total: len(items), Limit: len(items), Offset: 0}
}

// EquipmentHistory — GET /api/v1/equipment/:id/history (RequireAPI, без admin — чтение).
func (h *Handler) EquipmentHistory(c fiber.Ctx) error {
	d, err := h.svc.EquipmentHistory(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return c.JSON(historyDTO(d))
}
