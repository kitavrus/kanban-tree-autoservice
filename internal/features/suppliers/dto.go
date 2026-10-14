package suppliers

import "time"

// Wire-типы — JSON только snake_case (§API-1); конверт списка {items,total,limit,offset}.

type SupplierResponse struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	ContactName string    `json:"contact_name"`
	Phone       string    `json:"phone"`
	Address     string    `json:"address"`
	Note        string    `json:"note"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type SupplierListResponse struct {
	Items  []SupplierResponse `json:"items"`
	Total  int                `json:"total"`
	Limit  int                `json:"limit"`
	Offset int                `json:"offset"`
}

// SupplierCreateRequest — POST /api/v1/suppliers. Новый поставщик активен.
type SupplierCreateRequest struct {
	Name        string `json:"name"`
	ContactName string `json:"contact_name"`
	Phone       string `json:"phone"`
	Address     string `json:"address"`
	Note        string `json:"note"`
}

// SupplierUpdateRequest — PUT /api/v1/suppliers/:id: правка полей + архив/возврат.
// is_active не передан (nil) — 400 is_active: required .
type SupplierUpdateRequest struct {
	Name        string `json:"name"`
	ContactName string `json:"contact_name"`
	Phone       string `json:"phone"`
	Address     string `json:"address"`
	Note        string `json:"note"`
	IsActive    *bool  `json:"is_active"`
}
