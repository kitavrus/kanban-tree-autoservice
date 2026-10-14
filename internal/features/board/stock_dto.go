package board

import "github.com/kitavrus/kanban-tree/pkg/numeric"

// Wire-типы экрана «Остатки» (design-go-layers.md §DTO Stock) — JSON snake_case (§API-1),
// конверт списка {items,total,limit,offset}. Stock/MinStock/Pending/Forecast — pkg/numeric.
// Amount (NUMERIC-точность), НЕ float64.

// StockRow — строка списка/ответ Adjust (design-sql.md §Stock Pending CTE).
type StockRow struct {
	ProductID string `json:"product_id"`
	// IsDeleted — товар мягко удалён из каталога, но на складе числится его остаток: строка
	// показывается с пометкой «удалён» (soft-delete-catalog §6.9, DS-042).
	IsDeleted bool `json:"is_deleted"`
	// IsArchived — товар в архиве (не удалён), но на мягко удалённом складе числится его остаток:
	// строка показывается с пометкой «в архиве», остаток можно только списать (fix-loop 1 части 6).
	IsArchived bool           `json:"is_archived"`
	Name       string         `json:"name"`
	Unit       string         `json:"unit"`
	Box        string         `json:"box"`
	Place      string         `json:"place"`
	Stock      numeric.Amount `json:"stock"`
	MinStock   numeric.Amount `json:"min_stock"`
	Pending    numeric.Amount `json:"pending"` // потребность не взятых в работу заказов
	Forecast   numeric.Amount `json:"forecast"`
	State      string         `json:"state"` // out|low|will_low|ok
	// InTransit — «В пути»: строки перемещений в статусе in_transit (подфаза B, DS-006);
	// в подфазе A всегда 0.
	InTransit numeric.Amount `json:"in_transit"`
	// ByWarehouse — колонки складов (DS-004): по каждому складу отчёта (StockListResponse.
	// Warehouses), порядок position.
	ByWarehouse []StockWarehouseCell `json:"by_warehouse"`
}

// StockWarehouseCell — пара товар×склад в строке «Остатков».
type StockWarehouseCell struct {
	WarehouseID string         `json:"warehouse_id"`
	Stock       numeric.Amount `json:"stock"`
	MinStock    numeric.Amount `json:"min_stock"`
	Pending     numeric.Amount `json:"pending"`
	State       string         `json:"state"`
}

// StockWarehouse — склад отчёта (заголовки колонок и фильтр «Склад»): активный либо — с ненулевым
// остатком — архивный (IsArchived) или мягко удалённый (IsDeleted)
// (soft-delete-warehouses-suppliers DS-042, DS-049). У удалённого склада IsArchived = false.
type StockWarehouse struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	IsArchived bool   `json:"is_archived"`
	IsDeleted  bool   `json:"is_deleted"`
}

// StockListResponse — GET /api/v1/stock?warehouse_id=. Дефолты: limit=200 (1..1000), offset=0.
type StockListResponse struct {
	Warehouses []StockWarehouse `json:"warehouses"`
	Items      []StockRow       `json:"items"`
	Total      int              `json:"total"`
	Limit      int              `json:"limit"`
	Offset     int              `json:"offset"`
}

// StockSummaryResponse — GET /api/v1/stock/summary (Q-002/AC-016); RiskCount — источник
// бейджа меню «Остатки» (ADR-004).
type StockSummaryResponse struct {
	ProductsTotal int `json:"products_total"`
	LowCount      int `json:"low_count"`
	OutCount      int `json:"out_count"`
	WillLowCount  int `json:"will_low_count"`
	RiskCount     int `json:"risk_count"`
}

// StockAdjustRequest — PUT /api/v1/stock/:product_id (admin, D44). Stock — указатель:
// отличает «поле не передано» (nil → required) от «передано число 0». WarehouseID —
// склад правки (обязателен, DS-003).
type StockAdjustRequest struct {
	Stock       *numeric.Amount `json:"stock"`
	WarehouseID string          `json:"warehouse_id"`
}
