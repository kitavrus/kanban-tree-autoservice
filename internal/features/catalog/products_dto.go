package catalog

import "time"

// Wire-типы товаров — JSON snake_case (§API-1); NUMERIC-поля — Amount (число на wire,
// строка внутри, без потери точности).

type ProductResponse struct {
	ID         string `json:"id"`
	CategoryID string `json:"category_id"`
	// CategoryIsDeleted — категория товара мягко удалена: бейдж «категория удалена» (DS-014).
	CategoryIsDeleted bool   `json:"category_is_deleted"`
	Name              string `json:"name"`
	SKU               string `json:"sku"`
	Unit              string `json:"unit"`
	Price             Amount `json:"price"`
	IsActive          bool   `json:"is_active"`
	// Stocks — блок «Склады» (D46): все активные склады + архивные, где есть строка.
	Stocks []ProductStockResponse `json:"stocks"`
	// TotalStock — сумма остатков по всем складам («Итого»).
	TotalStock Amount    `json:"total_stock"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
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
	// Stocks — остатки по складам; не перечисленные склады не меняются (D46).
	Stocks []ProductStockRequest `json:"stocks"`
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
	// Stocks — остатки по складам; не перечисленные склады не меняются (D46).
	Stocks   []ProductStockRequest `json:"stocks"`
	IsActive *bool                 `json:"is_active"`
}

// ProductStockResponse — строка склада товара (DS-002). Отсутствующая строка = 0/0.
type ProductStockResponse struct {
	WarehouseID   string `json:"warehouse_id"`
	WarehouseName string `json:"warehouse_name"`
	IsActive      bool   `json:"is_active"`
	// Tracked — у товара есть строка на этом складе (D48); false — 0/0 по умолчанию.
	Tracked  bool   `json:"tracked"`
	Stock    Amount `json:"stock"`
	MinStock Amount `json:"min_stock"`
	Box      string `json:"box"`
	Place    string `json:"place"`
}

// ProductStockRequest — строка склада в POST/PUT /api/v1/products.
type ProductStockRequest struct {
	WarehouseID string `json:"warehouse_id"`
	Stock       Amount `json:"stock"`
	MinStock    Amount `json:"min_stock"`
	Box         string `json:"box"`
	Place       string `json:"place"`
}

func stocksFromRequest(in []ProductStockRequest) []ProductStockInput {
	out := make([]ProductStockInput, len(in))
	for i, s := range in {
		out[i] = ProductStockInput{WarehouseID: s.WarehouseID, Stock: s.Stock.String(), MinStock: s.MinStock.String(), Box: s.Box, Place: s.Place}
	}
	return out
}
