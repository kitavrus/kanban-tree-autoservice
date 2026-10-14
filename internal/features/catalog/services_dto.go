package catalog

import "time"

// Wire-типы услуг — JSON snake_case (§API-1); Price *Amount — nil/JSON null = «по факту»
// (task.md: «цена работы, пусто = "по факту"»).

// ServiceProductResponse — строка комплекта услуги с денормализованными полями товара
// (название/ед./цена — re-fetch-after-write, чтобы клиент не делал доп. запросы).
type ServiceProductResponse struct {
	ProductID string `json:"product_id"`
	// ProductIsDeleted — товар строки мягко удалён из каталога: бейдж «удалён» (DS-014).
	ProductIsDeleted bool   `json:"product_is_deleted"`
	Name             string `json:"name"`
	Unit             string `json:"unit"`
	Price            Amount `json:"price"`
	Qty              Amount `json:"qty"`
}

type ServiceResponse struct {
	ID         string `json:"id"`
	CategoryID string `json:"category_id"`
	// CategoryIsDeleted — категория услуги мягко удалена: бейдж «категория удалена» (DS-014).
	CategoryIsDeleted bool                     `json:"category_is_deleted"`
	Name              string                   `json:"name"`
	Price             *Amount                  `json:"price"`
	IsActive          bool                     `json:"is_active"`
	Products          []ServiceProductResponse `json:"products"`
	CreatedAt         time.Time                `json:"created_at"`
	UpdatedAt         time.Time                `json:"updated_at"`
}

type ServiceListResponse struct {
	Items  []ServiceResponse `json:"items"`
	Total  int               `json:"total"`
	Limit  int               `json:"limit"`
	Offset int               `json:"offset"`
}

// ServiceProductRequest — строка комплекта в теле POST/PUT (D10: дубли product_id
// объединяются суммой qty на сервере, порядок и дедуп — ServiceInput.normalize()).
type ServiceProductRequest struct {
	ProductID string `json:"product_id"`
	Qty       Amount `json:"qty"`
}

// ServiceCreateRequest — POST /api/v1/services. Price отсутствует/null — «по факту»;
// is_active по умолчанию — true; пустой Products — допустим (task.md).
type ServiceCreateRequest struct {
	CategoryID string                  `json:"category_id"`
	Name       string                  `json:"name"`
	Price      *Amount                 `json:"price"`
	Products   []ServiceProductRequest `json:"products"`
}

// ServiceUpdateRequest — PUT /api/v1/services/:id: полная замена карточки и комплекта
// (is_active обязателен, как ProductUpdateRequest).
type ServiceUpdateRequest struct {
	CategoryID string                  `json:"category_id"`
	Name       string                  `json:"name"`
	Price      *Amount                 `json:"price"`
	Products   []ServiceProductRequest `json:"products"`
	IsActive   *bool                   `json:"is_active"`
}
