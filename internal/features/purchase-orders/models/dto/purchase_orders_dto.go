// Package dto — wire-типы API закупок (§6, CLAUDE.md §API-1): JSON только snake_case, списки —
// конверт {items,total,limit,offset}. Числа numeric — строки ("5.000", "1250.00").
package dto

import (
	"bytes"
	"encoding/json"
	"errors"
	"time"
)

// Num — число numeric во входящем JSON: строка ("4", "2450.5") либо JSON-число (4). Хранит
// исходный текст; формат и диапазон проверяют validators.
type Num string

var errNumType = errors.New("dto: number must be a string or a JSON number")

func (n *Num) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 {
		return errNumType
	}
	switch b[0] {
	case '"':
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*n = Num(s)
		return nil
	case '{', '[', 't', 'f', 'n':
		return errNumType
	}
	*n = Num(b)
	return nil
}

// ---- запросы ----

// ItemRequest — строка заказа в POST/PUT (§6.2, §6.4). ID — только в PUT (null — новая).
type ItemRequest struct {
	ID        *string `json:"id"`
	ProductID string  `json:"product_id"`
	Qty       Num     `json:"qty"`
	Reason    string  `json:"reason" enums:"for_orders,below_min,manual"`
}

// CreateRequest — POST /api/v1/purchase-orders (§6.2). SupplierID nil — «Базар».
type CreateRequest struct {
	SupplierID *string       `json:"supplier_id"`
	Note       string        `json:"note"`
	Items      []ItemRequest `json:"items"`
}

// UpdateRequest — PUT /api/v1/purchase-orders/:id (§6.4).
type UpdateRequest struct {
	Version int64         `json:"version"`
	Note    string        `json:"note"`
	Items   []ItemRequest `json:"items"`
}

// VersionRequest — POST send/recall/pick-up (§6.6–§6.8).
type VersionRequest struct {
	Version int64 `json:"version"`
}

// ReasonRequest — POST cancel (§6.9) и DELETE (§6.5).
type ReasonRequest struct {
	Version int64   `json:"version"`
	Reason  *string `json:"reason"`
}

// ---- ответы ----

// ListItem — строка списка §6.1.
type ListItem struct {
	ID            string     `json:"id"`
	Number        int64      `json:"number"`
	NumberDisplay string     `json:"number_display"`
	SupplierID    *string    `json:"supplier_id"`
	RecipientName string     `json:"recipient_name"`
	Status        string     `json:"status" enums:"draft,sent,ready,picked_up,cancelled"`
	ItemsCount    int        `json:"items_count"`
	TotalAmount   *string    `json:"total_amount"`
	CreatedByName string     `json:"created_by_name"`
	CreatedAt     time.Time  `json:"created_at"`
	SentAt        *time.Time `json:"sent_at"`
	PickedUpAt    *time.Time `json:"picked_up_at"`
}

// ListResponse — конверт §API-1.
type ListResponse struct {
	Items  []ListItem `json:"items"`
	Total  int        `json:"total"`
	Limit  int        `json:"limit"`
	Offset int        `json:"offset"`
}

// OrderItem — строка карточки §6.3.
type OrderItem struct {
	ID              string  `json:"id"`
	ProductID       string  `json:"product_id"`
	Name            string  `json:"name"`
	Sku             string  `json:"sku"`
	Unit            string  `json:"unit"`
	Qty             string  `json:"qty"`
	Reason          string  `json:"reason" enums:"for_orders,below_min,manual"`
	SupplierQty     *string `json:"supplier_qty"`
	SupplierPrice   *string `json:"supplier_price"`
	SupplierNote    string  `json:"supplier_note"`
	LineAmount      *string `json:"line_amount"`
	HasDiscrepancy  bool    `json:"has_discrepancy"`
	CustomerChanged bool    `json:"customer_changed"`
	Version         int64   `json:"version"`
}

// Order — карточка §6.3 (ответ §6.2–§6.9).
type Order struct {
	ID               string      `json:"id"`
	Number           int64       `json:"number"`
	NumberDisplay    string      `json:"number_display"`
	Status           string      `json:"status" enums:"draft,sent,ready,picked_up,cancelled"`
	Version          int64       `json:"version"`
	SupplierID       *string     `json:"supplier_id"`
	RecipientName    string      `json:"recipient_name"`
	RecipientPhone   string      `json:"recipient_phone"`
	SupplierHasLogin bool        `json:"supplier_has_login"`
	Note             string      `json:"note"`
	CanEdit          bool        `json:"can_edit"`
	CreatedByName    string      `json:"created_by_name"`
	CreatedAt        time.Time   `json:"created_at"`
	SentAt           *time.Time  `json:"sent_at"`
	ReadyAt          *time.Time  `json:"ready_at"`
	PickedUpAt       *time.Time  `json:"picked_up_at"`
	CancelledAt      *time.Time  `json:"cancelled_at"`
	CancelReason     *string     `json:"cancel_reason"`
	Items            []OrderItem `json:"items"`
	TotalAmount      *string     `json:"total_amount"`
}

// SuggestionTask / OpenOrderRef / Suggestion — §6.10.
type SuggestionTask struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
}

type OpenOrderRef struct {
	ID            string `json:"id"`
	NumberDisplay string `json:"number_display"`
	Qty           string `json:"qty"`
}

type Suggestion struct {
	ProductID    string           `json:"product_id"`
	Name         string           `json:"name"`
	Sku          string           `json:"sku"`
	Unit         string           `json:"unit"`
	Stock        string           `json:"stock"`
	MinStock     string           `json:"min_stock"`
	Pending      string           `json:"pending"`
	ToOrder      string           `json:"to_order"`
	Reason       string           `json:"reason" enums:"for_orders,below_min"`
	Tasks        []SuggestionTask `json:"tasks"`
	InOpenOrders []OpenOrderRef   `json:"in_open_orders"`
	Checked      bool             `json:"checked"`
}

type SuggestionsResponse struct {
	Items  []Suggestion `json:"items"`
	Total  int          `json:"total"`
	Limit  int          `json:"limit"`
	Offset int          `json:"offset"`
}
