package users

import "time"

// Wire-типы — JSON только snake_case (§API-1); конверт списка {items,total,limit,offset}.

type UserResponse struct {
	ID       string `json:"id"`
	Login    string `json:"login"`
	Name     string `json:"name"`
	Role     string `json:"role"`
	Phone    string `json:"phone"`
	IsActive bool   `json:"is_active"`
	Timezone string `json:"timezone"` // имя IANA (DS-031)
	// SupplierID / SupplierName — привязка учётки роли supplier к поставщику
	// (purchase-orders §6.17, DS-007); null / "" у admin и executor.
	SupplierID   *string   `json:"supplier_id"`
	SupplierName string    `json:"supplier_name"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	// ActiveTasks — только в списке (GET /api/v1/users): заказы вне финальной колонки, где
	// сотрудник — ответственный или исполнитель незавершённой услуги.
	ActiveTasks *int `json:"active_tasks,omitempty"`
}

type ListResponse struct {
	Items  []UserResponse `json:"items"`
	Total  int            `json:"total"`
	Limit  int            `json:"limit"`
	Offset int            `json:"offset"`
}

// CreateRequest — POST /api/v1/users. is_active не передан — активен; timezone не
// передан — пояс по умолчанию (DS-056).
type CreateRequest struct {
	Login    string  `json:"login"`
	Name     string  `json:"name"`
	Role     string  `json:"role"`
	Phone    string  `json:"phone"`
	Password string  `json:"password"`
	IsActive *bool   `json:"is_active"`
	Timezone *string `json:"timezone"`
	// SupplierID — обязателен при role=supplier, иначе null (purchase-orders §6.17).
	SupplierID *string `json:"supplier_id"`
}

// UpdateRequest — PUT /api/v1/users/:id: полная замена карточки (D28 — last-write-wins,
// без version). Пароль пустой/не передан — не меняется; timezone не передан — не меняется.
type UpdateRequest struct {
	Login    string  `json:"login"`
	Name     string  `json:"name"`
	Role     string  `json:"role"`
	Phone    string  `json:"phone"`
	IsActive *bool   `json:"is_active"`
	Password string  `json:"password"`
	Timezone *string `json:"timezone"`
	// SupplierID — обязателен при role=supplier, иначе null (purchase-orders §6.17).
	SupplierID *string `json:"supplier_id"`
}

// TimezoneRequest — PUT /api/v1/users/me/timezone. Других полей нет: id — из сессии,
// посторонние ключи тела (user_id и т.п.) игнорируются.
type TimezoneRequest struct {
	Timezone string `json:"timezone"`
}

type TimezoneResponse struct {
	Timezone string `json:"timezone"`
}
