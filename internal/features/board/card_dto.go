package board

import (
	"encoding/json"
	"time"

	"github.com/kitavrus/kanban-tree/pkg/numeric"
)

// Wire-типы карточки заказа — JSON только snake_case (§API-1). GET /api/v1/tasks/:id/card
// и card в ответах row-мутаций (ADR-001, design-go-layers.md §DTO OrderCard).

// OrderCardResponse — итог заказа только в task.total_amount (одно имя в GET /tasks и в
// карточке; отдельного поля total нет). Equipment/Subtasks/Stock — всегда массивы, не null
// (AC-001).
type OrderCardResponse struct {
	Task      TaskResponse    `json:"task"`
	Column    CardColumn      `json:"column"`
	Equipment []CardEquipment `json:"equipment"`
	Subtasks  []CardSubtask   `json:"subtasks"`
	Stock     []CardStock     `json:"stock"`
	// Warehouses — склады для select'а строки товара (D45/DS-005): активные по position +
	// архивные и мягко удалённые (is_deleted), на которые ещё ссылаются строки этого заказа.
	Warehouses []CardWarehouse `json:"warehouses"`
	// Attachments — вложения заказа (фаза 7, D21), всегда массив, порядок загрузки.
	Attachments []CardAttachment `json:"attachments"`
	// Transfers — документы «Перемещение» заказа (transfers-partners DS-012), новые сверху;
	// всегда массив.
	Transfers []CardTransfer `json:"transfers"`
}

// CardTransfer — документ заказа в блоке «Перемещения заказа». WarehouseName — склад
// назначения (приём от клиента) либо отправления (возврат клиенту). Amount — сумма документа
// (transfers-sales DS-014): Σ price × qty_sent товарных строк; 0.00 для приёма от клиента и
// накладной без товаров — в любом статусе (в итог заказа идут только «В пути» и «Получено»).
type CardTransfer struct {
	ID            int64          `json:"id"`
	Direction     string         `json:"direction"` // client_to_warehouse | warehouse_to_client
	Status        string         `json:"status"`
	WarehouseName string         `json:"warehouse_name"`
	ItemsCount    int            `json:"items_count"`
	CreatedAt     time.Time      `json:"created_at"`
	Amount        numeric.Amount `json:"amount"`
}

// CardAttachment — вложение заказа. URL — отдача файла с проверкой сессии (inline);
// CanDelete — автор == актор либо актор admin.
type CardAttachment struct {
	ID           string    `json:"id"`
	Kind         string    `json:"kind"` // photo | video | pdf
	OriginalName string    `json:"original_name"`
	ContentType  string    `json:"content_type"`
	Size         int64     `json:"size"`
	UserID       string    `json:"user_id"`
	AuthorName   string    `json:"author_name"`
	CreatedAt    time.Time `json:"created_at"`
	URL          string    `json:"url"`
	CanDelete    bool      `json:"can_delete"`
}

type CardColumn struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	IsFinal       bool   `json:"is_final"`
	ConsumesStock bool   `json:"consumes_stock"`
}

// CardEquipment — единица оборудования клиента, принятая в заказ (AC-004).
type CardEquipment struct {
	EquipmentID string `json:"equipment_id"`
	// IsDeleted — единица мягко удалена (soft-delete-clients DS-014): в заказе остаётся с
	// бейджем «удалён».
	IsDeleted    bool               `json:"is_deleted"`
	TypeName     string             `json:"type_name"`
	Brand        string             `json:"brand"`
	Model        string             `json:"model"`
	Serial       string             `json:"serial"`
	Completeness string             `json:"completeness"`
	Condition    string             `json:"condition"`
	History      []EquipmentHistory `json:"history"`
	// Местоположение единицы (transfers-core DS-008, читается с equipment): LocationStatus —
	// at_client|at_warehouse|in_transit; WarehouseID/Name — склад (только at_warehouse);
	// ActiveTransferID — незавершённый документ «Перемещение» единицы (ссылка «перемещение
	// #N»); TransitToName — пока единица в пути: название склада назначения этого документа
	// либо «клиент» (документ «склад → клиент», transfers-partners DS-013).
	LocationStatus   string  `json:"location_status"`
	WarehouseID      *string `json:"warehouse_id"`
	WarehouseName    *string `json:"warehouse_name"`
	ActiveTransferID *int64  `json:"active_transfer_id"`
	TransitToName    *string `json:"transit_to_name"`
	// ActiveTransferDirection — направление незавершённого документа единицы (null — документа
	// нет): исполнителю «перемещение #N» — ссылка только у документов с клиентом, документ
	// «склад → склад» ему недоступен (transfers-partners DS-018).
	ActiveTransferDirection *string `json:"active_transfer_direction"`
}

// EquipmentHistory — другие заказы с этой же единицей оборудования (AC-006).
type EquipmentHistory struct {
	TaskID    int64     `json:"task_id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
}

// CardSubtask — услуга заказа + её товары/журнал.
type CardSubtask struct {
	ID          string `json:"id"`
	ServiceID   string `json:"service_id"`
	ServiceName string `json:"service_name"`
	// ServiceIsDeleted — услуга мягко удалена из каталога: бейдж «удалён» (DS-014).
	ServiceIsDeleted bool            `json:"service_is_deleted"`
	Status           string          `json:"status"` // Enum SubtaskStatus
	AssigneeID       *string         `json:"assignee_id"`
	AssigneeName     *string         `json:"assignee_name"`
	AssigneeIsActive *bool           `json:"assignee_is_active"` // null без исполнителя (SD-007)
	DueDate          *string         `json:"due_date"`           // YYYY-MM-DD
	Price            *numeric.Amount `json:"price"`              // null = «по факту»
	EquipmentID      *string         `json:"equipment_id"`
	Position         int             `json:"position"`
	Total            numeric.Amount  `json:"total"` // subtask_total
	// Выплата исполнителю (D62–D64): флаг, дата отметки (null, пока не выплачено) и сумма
	// выплаты — отдельная от price (null = не задана).
	IsExecutorPaid bool                 `json:"is_executor_paid"`
	ExecutorPaidAt *time.Time           `json:"executor_paid_at"`
	ExecutorPayout *numeric.Amount      `json:"executor_payout"`
	Products       []CardSubtaskProduct `json:"products"`
	Logs           []CardSubtaskLog     `json:"logs"`
}

type CardSubtaskProduct struct {
	ID        string `json:"id"`
	ProductID string `json:"product_id"`
	// ProductIsDeleted — товар мягко удалён из каталога: бейдж «удалён» (DS-014).
	ProductIsDeleted bool            `json:"product_is_deleted"`
	Name             string          `json:"name"`
	Unit             string          `json:"unit"`
	Position         int             `json:"position"` // ADR-103; массив уже отсортирован
	Qty              numeric.Amount  `json:"qty"`
	Price            *numeric.Amount `json:"price"`
	LineTotal        numeric.Amount  `json:"line_total"`
	// WarehouseID/WarehouseName — склад списания строки (DS-005).
	WarehouseID   string `json:"warehouse_id"`
	WarehouseName string `json:"warehouse_name"`
}

// CardSubtaskLog — запись журнала работ услуги (AC-012). CanEdit — автор == актор либо
// актор admin (D27), вычисляется в сервисе (Actor неизвестен репозиторию).
type CardSubtaskLog struct {
	ID         string    `json:"id"`
	Text       string    `json:"text"`
	UserID     string    `json:"user_id"`
	AuthorName string    `json:"author_name"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	CanEdit    bool      `json:"can_edit"`
}

// CardWarehouse — склад в select'е строки товара карточки.
type CardWarehouse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	IsActive bool   `json:"is_active"`
	// IsDeleted — склад мягко удалён (soft-delete-warehouses-suppliers DS-014): присутствует,
	// только если на него ссылаются строки заказа; в выбор склада новой строки не попадает.
	IsDeleted bool `json:"is_deleted"`
}

// CardStock — остаток пары товар×склад, задействованной в карточке (AC-009, DS-005).
type CardStock struct {
	ProductID    string         `json:"product_id"`
	WarehouseID  string         `json:"warehouse_id"`
	Stock        numeric.Amount `json:"stock"`
	MinStock     numeric.Amount `json:"min_stock"`
	TaskReserved numeric.Amount `json:"task_reserved"` // task_stock.qty этого заказа по паре, 0 если нет
}

// OrderCardMutationResponse — ответ всех row-мутаций карточки (POST 201 / PUT 200 /
// DELETE 200): re-fetch-after-write, Warnings — as-is формат ADR-006 фазы 5 (пустой массив,
// не null, если Reconcile не запускался).
type OrderCardMutationResponse struct {
	Card     OrderCardResponse `json:"card"`
	Warnings []StockWarning    `json:"warnings"`
}

// --- DTO Row Requests (design-go-layers.md §DTO Row Requests) ---
// Все — strictJSON (лишнее поле → 400 BAD_REQUEST), normalize()/validate() на типе
// запроса (эталон tasks_validate.go). Правила — design-errors.md §Validation Fields.
// Поля объявлены здесь одним файлом (Package Layout); normalize/validate услуг/товаров/
// журнала реализуются по мере готовности сервисов (фаза 03) — в этой фазе реализованы
// только TaskEquipmentCreateRequest/TaskEquipmentUpdateRequest (task_equipment_validate.go).

type SubtaskCreateRequest struct {
	ServiceID string `json:"service_id"`
}

// SubtaskUpdateRequest — PUT /subtasks/:id: полная замена изменяемых полей строки (LWW);
// клиент шлёт всю строку.
//
// Выплата исполнителю (D62–D66) — НЕ часть полной замены: у этих полей есть второй
// писатель (отчёт «Выплаты», POST /executor-payouts/mark), поэтому поле, которого нет в
// теле, не меняется, и карточка шлёт их только когда правит именно их. IsExecutorPaid —
// nil → флаг прежний; перевод false→true допустим только при назначенном исполнителе
// (400 is_executor_paid: no_assignee). ExecutorPayout — нет поля → сумма прежняя, null →
// очистить, число → установить; правится независимо от флага. Вправе и admin, и executor.
type SubtaskUpdateRequest struct {
	Status         string          `json:"status"`
	AssigneeID     *string         `json:"assignee_id"`
	DueDate        *string         `json:"due_date"`
	Price          *numeric.Amount `json:"price"`
	EquipmentID    *string         `json:"equipment_id"`
	IsExecutorPaid *bool           `json:"is_executor_paid"`
	ExecutorPayout OptionalAmount  `json:"executor_payout"`
}

// SubtaskProductCreateRequest — WarehouseID пуст → склад по умолчанию (DS-005, D49).
type SubtaskProductCreateRequest struct {
	ProductID   string          `json:"product_id"`
	Qty         *numeric.Amount `json:"qty"` // цена копируется из products.price (AC-008)
	WarehouseID string          `json:"warehouse_id"`
}

// SubtaskProductUpdateRequest — WarehouseID пуст → склад строки не меняется; смена склада
// у списанной строки пересчитывает списание (возврат на старый, списание с нового, DS-005).
type SubtaskProductUpdateRequest struct {
	Qty         *numeric.Amount `json:"qty"`
	Price       *numeric.Amount `json:"price"` // null = без цены (в итог 0)
	WarehouseID string          `json:"warehouse_id"`
}

// TaskEquipmentCreateRequest — POST /tasks/:id/equipment (AC-004/AC-005). WarehouseID —
// упразднённый «склад приёма» (transfers-partners DS-045): поле принимается строгим разбором
// тела, но игнорируется при любом значении — единица остаётся там, где числится; на склад её
// приводит документ «клиент → склад».
type TaskEquipmentCreateRequest struct {
	EquipmentID  string          `json:"equipment_id"`
	Completeness string          `json:"completeness"`
	Condition    string          `json:"condition"`
	WarehouseID  json.RawMessage `json:"warehouse_id"`
}

// TaskEquipmentUpdateRequest — PUT /tasks/:id/equipment/:equipment_id.
type TaskEquipmentUpdateRequest struct {
	Completeness string `json:"completeness"`
	Condition    string `json:"condition"`
}

type SubtaskLogRequest struct {
	Text string `json:"text"`
}
