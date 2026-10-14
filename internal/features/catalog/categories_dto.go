package catalog

import "time"

// Wire-типы категорий — JSON snake_case (§API-1); конверт списка {items,total,limit,offset}.

// CategoryResponse — категория каталога. ItemCount — число товаров/услуг этого вида
// в категории (панель слева).
type CategoryResponse struct {
	ID        string    `json:"id"`
	Kind      string    `json:"kind"`
	Name      string    `json:"name"`
	IsActive  bool      `json:"is_active"`
	ItemCount int       `json:"item_count"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type CategoryListResponse struct {
	Items  []CategoryResponse `json:"items"`
	Total  int                `json:"total"`
	Limit  int                `json:"limit"`
	Offset int                `json:"offset"`
}

// CategoryCreateRequest — POST /api/v1/catalog-categories. is_active по умолчанию — true.
type CategoryCreateRequest struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

// CategoryUpdateRequest — PUT /api/v1/catalog-categories/:id: только переименование
// (kind неизменяем, is_active — отдельные маршруты архива/восстановления).
type CategoryUpdateRequest struct {
	Name string `json:"name"`
}
