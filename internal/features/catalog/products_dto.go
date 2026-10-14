package catalog

import "time"

// Wire-типы товаров — JSON snake_case (§API-1); NUMERIC-поля — Amount (число на wire,
// строка внутри, без потери точности).

type ProductResponse struct {
	ID         string `json:"id"`
	CategoryID string `json:"category_id"`
	// CategoryIsDeleted — категория товара мягко удалена: бейдж «категория удалена» (DS-014).
	CategoryIsDeleted bool      `json:"category_is_deleted"`
	Name              string    `json:"name"`
	SKU               string    `json:"sku"`
	Unit              string    `json:"unit"`
	Price             Amount    `json:"price"`
	IsActive          bool      `json:"is_active"`
	Stock             Amount    `json:"stock"`
	MinStock          Amount    `json:"min_stock"`
	Box               string    `json:"box"`
	Place             string    `json:"place"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type ProductListResponse struct {
	Items  []ProductResponse `json:"items"`
	Total  int               `json:"total"`
	Limit  int               `json:"limit"`
	Offset int               `json:"offset"`
}

// ProductCreateRequest — POST /api/v1/products. is_active по умолчанию — true.
type ProductCreateRequest struct {
	CategoryID string `json:"category_id"`
	Name       string `json:"name"`
	SKU        string `json:"sku"`
	Unit       string `json:"unit"`
	Price      Amount `json:"price"`
	Stock      Amount `json:"stock"`
	MinStock   Amount `json:"min_stock"`
	Box        string `json:"box"`
	Place      string `json:"place"`
}

// ProductUpdateRequest — PUT /api/v1/products/:id: полная замена карточки товара.
// is_active обязателен (как equipmenttypes.TypeUpdateRequest) — архив/восстановление
// доступны и напрямую через PUT, и отдельными SSR-маршрутами.
type ProductUpdateRequest struct {
	CategoryID string `json:"category_id"`
	Name       string `json:"name"`
	SKU        string `json:"sku"`
	Unit       string `json:"unit"`
	Price      Amount `json:"price"`
	Stock      Amount `json:"stock"`
	MinStock   Amount `json:"min_stock"`
	Box        string `json:"box"`
	Place      string `json:"place"`
	IsActive   *bool  `json:"is_active"`
}
