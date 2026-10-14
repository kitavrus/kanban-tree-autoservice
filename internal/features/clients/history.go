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

// История единицы оборудования: заказы, куда она принималась (task_equipment) или где по
// ней выполнялись услуги (subtasks.equipment_id), + услуги, товары комплекта услуг и
// журнал работ; текущее местоположение единицы и её документы «Перемещение» (transfers-core
// DS-046 — вместо склада единицы «в каждом заказе»). Привязка «услуга → оборудование» уже хранится в subtasks.equipment_id,
// товары и журнал — через услугу; таблицы доски читаются напрямую SQL (как TypeOptions,
// ADR-102), без Go-импорта пакета board.

// HistoryOrder — заказ, в котором участвовала единица.
type HistoryOrder struct {
	TaskID       int64
	Title        string
	ColumnName   string
	ColumnColor  string
	IsFinal      bool
	IsPaid       bool
	CreatedAt    time.Time
	Accepted     bool // единица принята в заказ (есть строка task_equipment)
	Completeness string
	Condition    string
	Services     []HistoryService
}

// EquipmentLocation — текущее местоположение единицы (equipment, transfers-core DS-008).
type EquipmentLocation struct {
	Status           string // at_client | at_warehouse | in_transit
	WarehouseID      *string
	WarehouseName    *string
	ActiveTransferID *int64  // незавершённый документ единицы
	TransitToName    *string // пока единица в пути: склад назначения документа либо «клиент»
}

// Label — подпись бейджа: «у клиента» / «на складе X» / «в пути → Y».
func (l EquipmentLocation) Label() string {
	switch l.Status {
	case LocationAtWarehouse:
		if l.WarehouseName != nil {
			return "на складе " + *l.WarehouseName
		}
		return "на складе"
	case LocationInTransit:
		if l.TransitToName != nil {
			return "в пути → " + *l.TransitToName
		}
		return "в пути"
	}
	return "у клиента"
}

// BadgeClass — цвет бейджа местоположения (обе темы — классы Bootstrap text-bg-*).
func (l EquipmentLocation) BadgeClass() string {
	switch l.Status {
	case LocationAtWarehouse:
		return "text-bg-info"
	case LocationInTransit:
		return "text-bg-warning"
	}
	return "text-bg-secondary"
}

// HistoryTransfer — документ «Перемещение», в строках которого стоит единица.
type HistoryTransfer struct {
	ID        int64
	Status    string // requested | in_transit | received | cancelled
	FromName  string
	ToName    string
	CreatedAt time.Time
}

func (t HistoryTransfer) StatusLabel() string {
	switch t.Status {
	case "in_transit":
		return "В пути"
	case "received":
		return "Получено"
	case "cancelled":
		return "Отменён"
	}
	return "Заявка"
}

func (t HistoryTransfer) StatusClass() string {
	switch t.Status {
	case "in_transit":
		return "text-bg-warning"
	case "received":
		return "text-bg-success"
	case "cancelled":
		return "text-bg-secondary"
	}
	return "text-bg-primary"
}

// EquipmentLocation — местоположение единицы. ErrNoRows → ErrNotFound.
func (r *Repo) EquipmentLocation(ctx context.Context, equipmentID string) (*EquipmentLocation, error) {
	var l EquipmentLocation
	err := r.pool.QueryRow(ctx, `
		SELECT e.location_status, e.warehouse_id::text, w.name, e.active_transfer_id,
		       CASE WHEN e.location_status <> 'in_transit' THEN NULL
		            WHEN tr.direction = 'warehouse_to_client' THEN 'клиент'
		            ELSE wt.name END
		FROM equipment e
		LEFT JOIN warehouses w  ON w.id = e.warehouse_id
		LEFT JOIN transfers tr  ON tr.id = e.active_transfer_id
		LEFT JOIN warehouses wt ON wt.id = tr.to_warehouse_id
		WHERE e.id = $1::uuid AND e.deleted_at IS NULL`, equipmentID).
		Scan(&l.Status, &l.WarehouseID, &l.WarehouseName, &l.ActiveTransferID, &l.TransitToName)
	if err != nil {
		return nil, err
	}
	return &l, nil
}

// EquipmentTransfers — документы единицы, новые сверху. У документа с клиентом
// (transfers-partners DS-002) одна из точек маршрута — не склад: приём «клиент → склад X»,
// возврат «склад X → клиент»; единица бывает только в документах со складом хотя бы на одном
// конце, поэтому отсутствующая точка — всегда клиент.
func (r *Repo) EquipmentTransfers(ctx context.Context, equipmentID string) ([]HistoryTransfer, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT t.id, t.status, coalesce(fw.name, 'клиент'), coalesce(tw.name, 'клиент'), t.created_at
		FROM transfer_items i
		JOIN transfers t        ON t.id = i.transfer_id
		LEFT JOIN warehouses fw ON fw.id = t.from_warehouse_id
		LEFT JOIN warehouses tw ON tw.id = t.to_warehouse_id
		WHERE i.equipment_id = $1::uuid AND t.deleted_at IS NULL
		ORDER BY t.id DESC`, equipmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []HistoryTransfer{}
	for rows.Next() {
		var t HistoryTransfer
		if err := rows.Scan(&t.ID, &t.Status, &t.FromName, &t.ToName, &t.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// HistoryService — услуга заказа, выполнявшаяся по этой единице.
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

// EquipmentHistory — заказы по единице, новые сверху; услуги внутри заказа — по позиции.
// Мягко удалённые заказы в истории не показываются (soft-delete-core DS-010): услуги, товары
// и записи ниже привязываются к заказам первого запроса.
func (r *Repo) EquipmentHistory(ctx context.Context, equipmentID string) ([]HistoryOrder, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT t.id, t.title, c.name, c.color, c.is_final, t.is_paid, t.created_at,
		       te.task_id IS NOT NULL, coalesce(te.completeness, ''), coalesce(te.condition, '')
		FROM tasks t
		JOIN columns c ON c.id = t.column_id
		LEFT JOIN task_equipment te ON te.task_id = t.id AND te.equipment_id = $1::uuid
		WHERE t.id IN (SELECT task_id FROM task_equipment WHERE equipment_id = $1::uuid
		               UNION SELECT task_id FROM subtasks WHERE equipment_id = $1::uuid)
		  AND t.deleted_at IS NULL
		ORDER BY t.created_at DESC, t.id DESC`, equipmentID)
	if err != nil {
		return nil, err
	}
	orders := []HistoryOrder{}
	idx := map[int64]int{}
	for rows.Next() {
		var o HistoryOrder
		if err := rows.Scan(&o.TaskID, &o.Title, &o.ColumnName, &o.ColumnColor, &o.IsFinal, &o.IsPaid,
			&o.CreatedAt, &o.Accepted, &o.Completeness, &o.Condition); err != nil {
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
		JOIN services sv ON sv.id = s.service_id
		LEFT JOIN users u ON u.id = s.assignee_id
		WHERE s.equipment_id = $1::uuid
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
		JOIN products p ON p.id = sp.product_id
		WHERE s.equipment_id = $1::uuid
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
		JOIN users u ON u.id = l.user_id
		WHERE s.equipment_id = $1::uuid
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

// EquipmentHistoryData — единица, её заказы, текущее местоположение и документы.
type EquipmentHistoryData struct {
	Equipment EquipmentResponse
	Orders    []HistoryOrder
	Location  EquipmentLocation
	Transfers []HistoryTransfer
}

// EquipmentHistory — единица + её заказы + местоположение + документы. Невалидный/
// несуществующий id → ErrEquipmentNotFound.
func (s *Service) EquipmentHistory(ctx context.Context, id string) (*EquipmentHistoryData, error) {
	e, err := s.GetEquipment(ctx, id)
	if err != nil {
		return nil, err
	}
	orders, err := s.repo.EquipmentHistory(ctx, id)
	if err != nil {
		return nil, err
	}
	loc, err := s.repo.EquipmentLocation(ctx, id)
	if errors.Is(err, ErrNotFound) {
		return nil, ErrEquipmentNotFound // удалена между запросами
	}
	if err != nil {
		return nil, err
	}
	transfers, err := s.repo.EquipmentTransfers(ctx, id)
	if err != nil {
		return nil, err
	}
	return &EquipmentHistoryData{Equipment: *e, Orders: orders, Location: *loc, Transfers: transfers}, nil
}

// EquipmentHistoryVM — страница «История оборудования».
type EquipmentHistoryVM struct {
	Shell      ssr.Shell
	Equipment  EquipmentResponse
	ClientName string
	Orders     []HistoryOrder
	Services   int
	Location   EquipmentLocation
	Transfers  []HistoryTransfer
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
	vm := EquipmentHistoryVM{Equipment: d.Equipment, Orders: d.Orders, Location: d.Location, Transfers: d.Transfers}
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
// невелика). Карточка заказа раскрывает по нему «Историю работ» (доступно и executor).
// transfers-core DS-046: рядом с конвертом заказов — location (текущее местоположение
// единицы) и transfers (её документы «Перемещение», новые сверху). ---

type HistoryOrderDTO struct {
	TaskID       int64               `json:"task_id"`
	Title        string              `json:"title"`
	ColumnName   string              `json:"column_name"`
	ColumnColor  string              `json:"column_color"`
	IsFinal      bool                `json:"is_final"`
	IsPaid       bool                `json:"is_paid"`
	CreatedAt    time.Time           `json:"created_at"`
	Accepted     bool                `json:"accepted"`
	Completeness string              `json:"completeness"`
	Condition    string              `json:"condition"`
	Services     []HistoryServiceDTO `json:"services"`
}

// EquipmentLocationDTO — текущее местоположение единицы.
type EquipmentLocationDTO struct {
	LocationStatus   string  `json:"location_status"`
	WarehouseID      *string `json:"warehouse_id"`
	WarehouseName    *string `json:"warehouse_name"`
	ActiveTransferID *int64  `json:"active_transfer_id"`
	TransitToName    *string `json:"transit_to_name"`
}

// HistoryTransferDTO — документ «Перемещение» единицы (#N, маршрут, статус, дата).
type HistoryTransferDTO struct {
	ID                int64     `json:"id"`
	Status            string    `json:"status"`
	FromWarehouseName string    `json:"from_warehouse_name"`
	ToWarehouseName   string    `json:"to_warehouse_name"`
	CreatedAt         time.Time `json:"created_at"`
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
	Items     []HistoryOrderDTO    `json:"items"`
	Total     int                  `json:"total"`
	Limit     int                  `json:"limit"`
	Offset    int                  `json:"offset"`
	Location  EquipmentLocationDTO `json:"location"`
	Transfers []HistoryTransferDTO `json:"transfers"` // всегда массив
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
	transfers := make([]HistoryTransferDTO, len(d.Transfers))
	for i, t := range d.Transfers {
		transfers[i] = HistoryTransferDTO{ID: t.ID, Status: t.Status, FromWarehouseName: t.FromName,
			ToWarehouseName: t.ToName, CreatedAt: t.CreatedAt}
	}
	location := EquipmentLocationDTO{LocationStatus: d.Location.Status, WarehouseID: d.Location.WarehouseID,
		WarehouseName: d.Location.WarehouseName, ActiveTransferID: d.Location.ActiveTransferID,
		TransitToName: d.Location.TransitToName}
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
			Accepted: o.Accepted, Completeness: o.Completeness, Condition: o.Condition, Services: svcs}
	}
	return EquipmentHistoryListResponse{Items: items, Total: len(items), Limit: len(items), Offset: 0,
		Location: location, Transfers: transfers}
}

// EquipmentHistory — GET /api/v1/equipment/:id/history (RequireAPI, без admin — чтение).
func (h *Handler) EquipmentHistory(c fiber.Ctx) error {
	d, err := h.svc.EquipmentHistory(c.Context(), c.Params("id"))
	if err != nil {
		return err
	}
	return c.JSON(historyDTO(d))
}
