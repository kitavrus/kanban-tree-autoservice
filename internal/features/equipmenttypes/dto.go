package equipmenttypes

import "time"

// Wire-типы — JSON только snake_case (§API-1); конверт списка {items,total,limit,offset}.

type TypeResponse struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	SpecKeys       []string  `json:"spec_keys"` // всегда массив
	IsActive       bool      `json:"is_active"`
	EquipmentCount int       `json:"equipment_count"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type TypeListResponse struct {
	Items  []TypeResponse `json:"items"`
	Total  int            `json:"total"`
	Limit  int            `json:"limit"`
	Offset int            `json:"offset"`
}

// TypeCreateRequest — POST /api/v1/equipment-types. is_active не передаётся — активен.
type TypeCreateRequest struct {
	Name     string   `json:"name"`
	SpecKeys []string `json:"spec_keys"`
}

// TypeUpdateRequest — PUT /api/v1/equipment-types/:id: полная замена карточки типа.
// is_active не передан (nil) — 400 is_active: required (как users.UpdateRequest).
type TypeUpdateRequest struct {
	Name     string   `json:"name"`
	SpecKeys []string `json:"spec_keys"`
	IsActive *bool    `json:"is_active"`
}
