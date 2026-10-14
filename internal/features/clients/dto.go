package clients

import "time"

// Wire-типы — JSON только snake_case (§API-1); конверт списка {items,total,limit,offset}.

type PhoneDTO struct {
	Phone string `json:"phone"`
	Note  string `json:"note"`
}

type ClientResponse struct {
	ID              string     `json:"id"`
	CompanyName     string     `json:"company_name"`
	Address         string     `json:"address"`
	ContactName     string     `json:"contact_name"`
	ContactPosition string     `json:"contact_position"`
	Phones          []PhoneDTO `json:"phones"` // всегда массив, не null
	EquipmentCount  int        `json:"equipment_count"`
	Version         int64      `json:"version"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type ClientListResponse struct {
	Items  []ClientResponse `json:"items"`
	Total  int              `json:"total"`
	Limit  int              `json:"limit"`
	Offset int              `json:"offset"`
}

// ClientCreateRequest — POST /api/v1/clients.
type ClientCreateRequest struct {
	CompanyName     string     `json:"company_name"`
	Address         string     `json:"address"`
	ContactName     string     `json:"contact_name"`
	ContactPosition string     `json:"contact_position"`
	Phones          []PhoneDTO `json:"phones"`
}

// ClientUpdateRequest — PUT /api/v1/clients/:id: полная замена карточки + телефонов в
// одной транзакции (ADR-101). Version nil → 400 version: required (ADR-100, оптимистичная
// блокировка: UPDATE ... WHERE version=$n).
type ClientUpdateRequest struct {
	CompanyName     string     `json:"company_name"`
	Address         string     `json:"address"`
	ContactName     string     `json:"contact_name"`
	ContactPosition string     `json:"contact_position"`
	Phones          []PhoneDTO `json:"phones"`
	Version         *int64     `json:"version"`
}

// --- Equipment DTO — заготовки полей (design-go-layers.md `## DTO Equipment`),
// наполнение логики (сервис/репозиторий/хендлер) — Phase 7. ---

type SpecDTO struct {
	K string `json:"k"`
	V string `json:"v"`
}

type EquipmentResponse struct {
	ID           string `json:"id"`
	ClientID     string `json:"client_id"`
	TypeID       string `json:"type_id"`
	TypeName     string `json:"type_name"`
	TypeIsActive bool   `json:"type_is_active"` // ADR-004
	// TypeIsDeleted — тип единицы мягко удалён: бейдж «удалён» у типа (soft-delete-catalog DS-014).
	TypeIsDeleted bool      `json:"type_is_deleted"`
	PlateNumber   string    `json:"plate_number"` // госномер автомобиля
	Brand         string    `json:"brand"`
	Model         string    `json:"model"`
	SKU           string    `json:"sku"`
	Serial        string    `json:"serial"`
	Specs         []SpecDTO `json:"specs"` // всегда массив
	Note          string    `json:"note"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type EquipmentListResponse struct {
	Items  []EquipmentResponse `json:"items"`
	Total  int                 `json:"total"`
	Limit  int                 `json:"limit"`
	Offset int                 `json:"offset"`
}

// EquipmentRequest — POST /clients/:id/equipment и PUT /equipment/:id (полная замена, LWW).
type EquipmentRequest struct {
	TypeID      string    `json:"type_id"`
	PlateNumber string    `json:"plate_number"`
	Brand       string    `json:"brand"`
	Model       string    `json:"model"`
	SKU         string    `json:"sku"`
	Serial      string    `json:"serial"`
	Specs       []SpecDTO `json:"specs"`
	Note        string    `json:"note"`
}

// EquipmentLookupItem — автомобиль, найденный по госномеру, с владельцем (GET
// /api/v1/equipment?plate_number=…, диалог «Новый заказ»). ClientName — clients.company_name
// (ФИО/название клиента).
type EquipmentLookupItem struct {
	ID          string `json:"id"`
	ClientID    string `json:"client_id"`
	ClientName  string `json:"client_name"`
	TypeID      string `json:"type_id"`
	TypeName    string `json:"type_name"`
	PlateNumber string `json:"plate_number"`
	Brand       string `json:"brand"`
	Model       string `json:"model"`
	Serial      string `json:"serial"`
}

type EquipmentLookupResponse struct {
	Items  []EquipmentLookupItem `json:"items"`
	Total  int                   `json:"total"`
	Limit  int                   `json:"limit"`
	Offset int                   `json:"offset"`
}
