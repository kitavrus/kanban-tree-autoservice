package dto

import "time"

// Wire-типы кабинета поставщика (§6.12–§6.15). Поля reason, product_id, наши цены и автор
// заказа не отдаются (DS-046).

// AnswerRequest — PUT /api/v1/supplier/purchase-orders/:id/items/:item_id (§6.14).
// supplier_id/order_id в теле игнорируются: поставщик — из сессии, заказ — из URL (DS-053).
type AnswerRequest struct {
	Version       int64  `json:"version"`
	SupplierQty   *Num   `json:"supplier_qty"`
	SupplierPrice *Num   `json:"supplier_price"`
	SupplierNote  string `json:"supplier_note"`
}

// SupplierListItem — §6.12.
type SupplierListItem struct {
	ID            string     `json:"id"`
	NumberDisplay string     `json:"number_display"`
	Status        string     `json:"status" enums:"sent,ready,picked_up"`
	SentAt        *time.Time `json:"sent_at"`
	ItemsCount    int        `json:"items_count"`
	AnsweredCount int        `json:"answered_count"`
	TotalAmount   *string    `json:"total_amount"`
	IsChanged     bool       `json:"is_changed"`
}

type SupplierListResponse struct {
	Items  []SupplierListItem `json:"items"`
	Total  int                `json:"total"`
	Limit  int                `json:"limit"`
	Offset int                `json:"offset"`
}

// SupplierItem — строка карточки кабинета §6.13.
type SupplierItem struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	Sku             string  `json:"sku"`
	Unit            string  `json:"unit"`
	Qty             string  `json:"qty"`
	SupplierQty     *string `json:"supplier_qty"`
	SupplierPrice   *string `json:"supplier_price"`
	SupplierNote    string  `json:"supplier_note"`
	LineAmount      *string `json:"line_amount"`
	CustomerChanged bool    `json:"customer_changed"`
	Version         int64   `json:"version"`
}

// SupplierOrder — карточка кабинета §6.13 (ответ §6.15).
type SupplierOrder struct {
	ID            string         `json:"id"`
	NumberDisplay string         `json:"number_display"`
	Status        string         `json:"status" enums:"sent,ready,picked_up"`
	SentAt        *time.Time     `json:"sent_at"`
	Note          string         `json:"note"`
	NoteChanged   bool           `json:"note_changed"`
	FirmName      string         `json:"firm_name"`
	FirmPhone     string         `json:"firm_phone"`
	CanAnswer     bool           `json:"can_answer"`
	Items         []SupplierItem `json:"items"`
	TotalAmount   *string        `json:"total_amount"`
}

// AnswerResponse — ответ §6.14: строка + итог заказа.
type AnswerResponse struct {
	SupplierItem
	TotalAmount *string `json:"total_amount"`
}
