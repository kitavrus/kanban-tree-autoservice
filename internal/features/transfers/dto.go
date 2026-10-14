package transfers

import (
	"encoding/json"
	"time"

	"github.com/kitavrus/kanban-tree/pkg/numeric"
)

// Wire-типы — JSON только snake_case (§API-1); конверт списка {items,total,limit,offset}.

// Статусы перемещения (transfers-core §5).
const (
	StatusRequested = "requested"
	StatusInTransit = "in_transit"
	StatusReceived  = "received"
	StatusCancelled = "cancelled"
)

// Statuses — допустимые значения фильтра списка.
var Statuses = []string{StatusRequested, StatusInTransit, StatusReceived, StatusCancelled}

// Направления документа (transfers-partners DS-002); после создания не меняется.
const (
	DirClientToWarehouse    = "client_to_warehouse"
	DirWarehouseToClient    = "warehouse_to_client"
	DirWarehouseToWarehouse = "warehouse_to_warehouse"
	DirSupplierToWarehouse  = "supplier_to_warehouse"
)

// Directions — допустимые значения direction (тело создания, фильтр списка).
var Directions = []string{DirClientToWarehouse, DirWarehouseToClient, DirWarehouseToWarehouse, DirSupplierToWarehouse}

// clientDirections — направления с клиентом: единственные, доступные исполнителю (DS-018).
var clientDirections = []string{DirClientToWarehouse, DirWarehouseToClient}

// isClientDirection — документ с клиентом (заказ обязателен, DS-012).
func isClientDirection(d string) bool {
	return d == DirClientToWarehouse || d == DirWarehouseToClient
}

// Тип строки документа (DS-001): товар с количеством либо единица оборудования клиента.
const (
	KindProduct   = "product"
	KindEquipment = "equipment"
)

// Местоположение единицы оборудования (DS-008, equipment.location_status).
const (
	locationAtClient    = "at_client"
	locationAtWarehouse = "at_warehouse"
	locationInTransit   = "in_transit"
)

// Причины движений перемещения (CHECK stock_movements_reason_check миграций 0008, 0015).
const (
	reasonOut    = "transfer_out"
	reasonIn     = "transfer_in"
	reasonLoss   = "transfer_loss"
	reasonReturn = "transfer_return"
	// reasonSupplierIn — приход факта от поставщика на склад назначения (DS-040).
	reasonSupplierIn = "supplier_in"
	// reasonClientOut — списание товара накладной клиенту при отправке (transfers-sales DS-011).
	reasonClientOut = "client_out"
	// reasonUnreceive — компенсирующее движение отмены проведения: −факт приёмки на складе
	// назначения (transfers-revert DS-017).
	reasonUnreceive = "transfer_unreceive"
)

// ItemInput — строка документа в запросе создания/правки. kind = product — product_id +
// qty_sent; kind = equipment — equipment_id; «чужое» поле → 400 not_allowed (§6.2). Price —
// цена продажи: только товарная строка накладной «склад → клиент» (transfers-sales DS-011);
// нет поля / null → цена товара из каталога на момент сохранения (DS-039).
type ItemInput struct {
	Kind        string          `json:"kind"`
	ProductID   string          `json:"product_id"`
	EquipmentID string          `json:"equipment_id"`
	QtySent     numeric.Amount  `json:"qty_sent"`
	Price       *numeric.Amount `json:"price"`
}

// TransferCreateRequest — POST /api/v1/transfers. send=true — сразу «Отправить»
// (in_transit), иначе «Заявка» (requested); receive=true — создать, отправить и принять одной
// транзакцией (только client_to_warehouse, DS-013). Набор точек маршрута — по direction
// (§6.2). ClientID принимается разбором тела, но игнорируется: клиент документа выводится из
// заказа (DS-012).
type TransferCreateRequest struct {
	Direction       string      `json:"direction"`
	FromWarehouseID string      `json:"from_warehouse_id"`
	ToWarehouseID   string      `json:"to_warehouse_id"`
	TaskID          *int64      `json:"task_id"`
	ClientID        *string     `json:"client_id"`
	SupplierID      string      `json:"supplier_id"`
	Note            string      `json:"note"`
	Items           []ItemInput `json:"items"`
	Send            bool        `json:"send"`
	Receive         bool        `json:"receive"`
}

// TransferUpdateRequest — PUT /api/v1/transfers/:id (только requested). Direction и TaskID
// после создания не меняются: переданы → 400 not_allowed (§6.4); ClientID игнорируется.
type TransferUpdateRequest struct {
	Direction       *string     `json:"direction"`
	FromWarehouseID string      `json:"from_warehouse_id"`
	ToWarehouseID   string      `json:"to_warehouse_id"`
	TaskID          *int64      `json:"task_id"`
	ClientID        *string     `json:"client_id"`
	SupplierID      string      `json:"supplier_id"`
	Note            string      `json:"note"`
	Items           []ItemInput `json:"items"`
	Version         *int        `json:"version"`
}

// TransferActionRequest — POST /:id/send и /:id/cancel.
type TransferActionRequest struct {
	Version *int `json:"version"`
}

// TransferUnreceiveRequest — POST /:id/unreceive («Отменить проведение», только admin):
// обязательная причина 1–500 символов после trim (transfers-revert DS-053).
type TransferUnreceiveRequest struct {
	Version *int   `json:"version"`
	Reason  string `json:"reason"`
}

// TransferDeleteRequest — тело DELETE /api/v1/transfers/:id (soft-delete-transfers §6.1,
// DS-028): причина 1–500 символов после обрезки пробелов и версия документа (раньше — query).
// Version — сырой JSON: нет / null → required, не целое ≥ 1 → invalid (а не «битое тело»).
type TransferDeleteRequest struct {
	Reason  *string         `json:"reason"`
	Version json.RawMessage `json:"version"`
}

// TransferDeleteResponse — DELETE /api/v1/transfers/:id: номер мягко удалённого документа и
// момент удаления (UTC).
type TransferDeleteResponse struct {
	ID        int64     `json:"id"`
	DeletedAt time.Time `json:"deleted_at"`
}

// TransferRestoreResponse — POST /api/v1/transfers/:id/restore: номер восстановленного
// документа.
type TransferRestoreResponse struct {
	ID int64 `json:"id"`
}

// ReceiveItemInput — факт по товарной строке при приёмке; строка без записи — факт =
// qty_sent; строки-единицы факта не имеют (DS-037).
type ReceiveItemInput struct {
	ID          string         `json:"id"`
	QtyReceived numeric.Amount `json:"qty_received"`
}

// TransferReceiveRequest — POST /:id/receive.
type TransferReceiveRequest struct {
	Version *int               `json:"version"`
	Items   []ReceiveItemInput `json:"items"`
}

// StockWarning — остаток пары товар×склад после списания с источника (отрицательный или
// на/ниже минимума; разрешено, как D17/D18 — только предупреждение).
type StockWarning struct {
	ProductID     string         `json:"product_id"`
	WarehouseID   string         `json:"warehouse_id"`
	WarehouseName string         `json:"warehouse_name"`
	Name          string         `json:"name"`
	Stock         numeric.Amount `json:"stock"`
	MinStock      numeric.Amount `json:"min_stock"`
	Kind          string         `json:"kind"` // negative | below_min
}

// TransferItemResponse — строка документа: у товара заполнены product_*/unit/qty_*, у
// единицы — equipment_* (остальное null). Price / LineTotal (price × qty_sent, 2 знака) —
// только товарная строка накладной «склад → клиент», иначе null (transfers-sales DS-011).
type TransferItemResponse struct {
	ID        string  `json:"id"`
	Kind      string  `json:"kind"`
	ProductID *string `json:"product_id"`
	// ProductIsDeleted — товар строки мягко удалён из каталога (soft-delete-catalog DS-014);
	// false у строки-единицы.
	ProductIsDeleted bool    `json:"product_is_deleted"`
	ProductName      *string `json:"product_name"`
	Unit             *string `json:"unit"`
	EquipmentID      *string `json:"equipment_id"`
	// EquipmentIsDeleted — единица строки мягко удалена (soft-delete-clients DS-014); false у
	// товарной строки.
	EquipmentIsDeleted  bool            `json:"equipment_is_deleted"`
	EquipmentName       *string         `json:"equipment_name"` // «тип марка модель»
	EquipmentSerial     *string         `json:"equipment_serial"`
	EquipmentClientName *string         `json:"equipment_client_name"`
	QtySent             *numeric.Amount `json:"qty_sent"`
	QtyReceived         *numeric.Amount `json:"qty_received"`
	Price               *numeric.Amount `json:"price"`
	LineTotal           *numeric.Amount `json:"line_total"`
}

// TransferResponse — карточка перемещения (re-fetch-after-write). Warnings — только в
// ответе отправки и отмены проведения (иначе пустой массив). Unreceived* — последняя отмена
// проведения (кто, когда, причина; transfers-revert DS-044): сохраняются и после повторной
// приёмки; у документа без отката — null и пустая причина. Точки маршрута заполнены по direction: склады,
// заказ + клиент (документы с клиентом), поставщик — остальное null. Amount — сумма накладной:
// Σ line_total товарных строк; 0.00 вне «склад → клиент» и без товаров (DS-038).
type TransferResponse struct {
	ID                int64   `json:"id"`
	Direction         string  `json:"direction"`
	FromWarehouseID   *string `json:"from_warehouse_id"`
	FromWarehouseName *string `json:"from_warehouse_name"`
	// FromWarehouseIsDeleted — склад отправления мягко удалён (soft-delete-warehouses-suppliers
	// DS-014); false, если склада отправления нет.
	FromWarehouseIsDeleted bool    `json:"from_warehouse_is_deleted"`
	ToWarehouseID          *string `json:"to_warehouse_id"`
	ToWarehouseName        *string `json:"to_warehouse_name"`
	// ToWarehouseIsDeleted — склад назначения мягко удалён; false, если склада назначения нет.
	ToWarehouseIsDeleted bool    `json:"to_warehouse_is_deleted"`
	TaskID               *int64  `json:"task_id"`
	TaskTitle            *string `json:"task_title"`
	TaskIsDeleted        bool    `json:"task_is_deleted"`
	ClientID             *string `json:"client_id"`
	ClientName           *string `json:"client_name"`
	// ClientIsDeleted — клиент документа мягко удалён (soft-delete-clients DS-014); false,
	// если клиента нет.
	ClientIsDeleted bool    `json:"client_is_deleted"`
	SupplierID      *string `json:"supplier_id"`
	SupplierName    *string `json:"supplier_name"`
	// SupplierIsDeleted — поставщик документа мягко удалён (DS-014); false, если поставщика нет.
	SupplierIsDeleted bool                   `json:"supplier_is_deleted"`
	Status            string                 `json:"status"`
	Note              string                 `json:"note"`
	CreatedBy         string                 `json:"created_by"`
	CreatedByName     string                 `json:"created_by_name"`
	SentBy            *string                `json:"sent_by"`
	SentByName        *string                `json:"sent_by_name"`
	SentAt            *time.Time             `json:"sent_at"`
	ReceivedBy        *string                `json:"received_by"`
	ReceivedByName    *string                `json:"received_by_name"`
	ReceivedAt        *time.Time             `json:"received_at"`
	CancelledBy       *string                `json:"cancelled_by"`
	CancelledByName   *string                `json:"cancelled_by_name"`
	CancelledAt       *time.Time             `json:"cancelled_at"`
	UnreceivedBy      *string                `json:"unreceived_by"`
	UnreceivedByName  *string                `json:"unreceived_by_name"`
	UnreceivedAt      *time.Time             `json:"unreceived_at"`
	UnreceiveReason   string                 `json:"unreceive_reason"`
	CreatedAt         time.Time              `json:"created_at"`
	UpdatedAt         time.Time              `json:"updated_at"`
	Version           int                    `json:"version"`
	Amount            numeric.Amount         `json:"amount"`
	Items             []TransferItemResponse `json:"items"`
	Warnings          []StockWarning         `json:"warnings"`
}

// TransferSummary — строка списка (без строк документа). TaskIsDeleted — заказ документа мягко
// удалён (бейдж «заказ удалён», soft-delete-transfers DS-007; false, если заказа нет). Amount — сумма накладной (0.00 вне
// «склад → клиент»). UnreceivedAt — последняя отмена проведения (значок «↩» у документа,
// возвращённого откатом в «В пути»).
type TransferSummary struct {
	ID                int64   `json:"id"`
	Direction         string  `json:"direction"`
	FromWarehouseID   *string `json:"from_warehouse_id"`
	FromWarehouseName *string `json:"from_warehouse_name"`
	// FromWarehouseIsDeleted — склад отправления мягко удалён (soft-delete-warehouses-suppliers
	// DS-014); false, если склада отправления нет.
	FromWarehouseIsDeleted bool    `json:"from_warehouse_is_deleted"`
	ToWarehouseID          *string `json:"to_warehouse_id"`
	ToWarehouseName        *string `json:"to_warehouse_name"`
	// ToWarehouseIsDeleted — склад назначения мягко удалён; false, если склада назначения нет.
	ToWarehouseIsDeleted bool    `json:"to_warehouse_is_deleted"`
	TaskID               *int64  `json:"task_id"`
	TaskIsDeleted        bool    `json:"task_is_deleted"`
	ClientID             *string `json:"client_id"`
	ClientName           *string `json:"client_name"`
	// ClientIsDeleted — клиент документа мягко удалён (soft-delete-clients DS-014).
	ClientIsDeleted bool    `json:"client_is_deleted"`
	SupplierID      *string `json:"supplier_id"`
	SupplierName    *string `json:"supplier_name"`
	// SupplierIsDeleted — поставщик документа мягко удалён (DS-014); false, если поставщика нет.
	SupplierIsDeleted bool           `json:"supplier_is_deleted"`
	Status            string         `json:"status"`
	Note              string         `json:"note"`
	ItemsCount        int            `json:"items_count"`
	CreatedByName     string         `json:"created_by_name"`
	UnreceivedAt      *time.Time     `json:"unreceived_at"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
	Version           int            `json:"version"`
	Amount            numeric.Amount `json:"amount"`
}

type TransferListResponse struct {
	Items  []TransferSummary `json:"items"`
	Total  int               `json:"total"`
	Limit  int               `json:"limit"`
	Offset int               `json:"offset"`
}
