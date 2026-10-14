package warehouses

import "time"

// Wire-типы — JSON только snake_case (§API-1); конверт списка {items,total,limit,offset}.

type WarehouseResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Position  int       `json:"position"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type WarehouseListResponse struct {
	Items  []WarehouseResponse `json:"items"`
	Total  int                 `json:"total"`
	Limit  int                 `json:"limit"`
	Offset int                 `json:"offset"`
}

// WarehouseCreateRequest — POST /api/v1/warehouses. Новый склад активен, position — в конец.
type WarehouseCreateRequest struct {
	Name string `json:"name"`
}

// WarehouseUpdateRequest — PUT /api/v1/warehouses/:id: переименование + архив/возврат.
// is_active не передан (nil) — 400 is_active: required (как equipmenttypes).
type WarehouseUpdateRequest struct {
	Name     string `json:"name"`
	IsActive *bool  `json:"is_active"`
}
