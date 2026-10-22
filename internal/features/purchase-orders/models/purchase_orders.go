// Package models — строки БД закупок (§4.1–§4.2) в том виде, в каком их читает repository.
// Числа numeric — строки (::text из БД, без потери точности); суммы вычисляются в SQL
// (DS-071) и не хранятся.
package models

import "time"

// PurchaseOrder — шапка заказа с вычисленными полями для списка/карточки.
type PurchaseOrder struct {
	ID            string
	Number        int64
	SupplierID    *string
	SupplierName  string // '' у Базара
	SupplierPhone string
	Status        string
	Note          string
	NoteChangedAt *time.Time
	Version       int64
	CreatedBy     string
	CreatedByName string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	SentAt        *time.Time
	ReadyAt       *time.Time
	PickedUpAt    *time.Time
	CancelledAt   *time.Time
	CancelReason  *string

	// SupplierHasLogin — у поставщика есть активный вход (DS-059).
	SupplierHasLogin bool
	// ItemsCount / AnsweredCount / TotalAmount / IsChanged — агрегаты строк (§6.1, §6.12).
	ItemsCount    int
	AnsweredCount int
	TotalAmount   *string
	IsChanged     bool
	NoteChanged   bool
	// HasDiscrepancy — хотя бы в одной строке поставщик ответил количеством ≠ заказанному.
	HasDiscrepancy bool

	Items []PurchaseOrderItem
}

// Head — заблокированная (FOR UPDATE/SHARE) шапка: вход проверок переходов §5.
type Head struct {
	ID               string
	Number           int64
	SupplierID       *string
	Status           string
	Version          int64
	CreatedBy        string
	Note             string
	SentAt           *time.Time
	ItemsCount       int
	SupplierHasLogin bool
	// SupplierAvailable — поставщик активен и не удалён (строка поставщика взята FOR SHARE в той
	// же транзакции, DS-043); у Базара — true.
	SupplierAvailable bool
}

// IsBazaar — заказ без поставщика (DS-006).
func (h Head) IsBazaar() bool { return h.SupplierID == nil }

// Suggestion — строка дефицита §6.10.
type Suggestion struct {
	ProductID    string
	Name         string
	Sku          string
	Unit         string
	Stock        string
	MinStock     string
	Pending      string
	ToOrder      string
	Reason       string
	Tasks        []SuggestionTask
	InOpenOrders []OpenOrderRef
}

// SuggestionTask — заказ клиента, дающий pending.
type SuggestionTask struct {
	ID    int64
	Title string
}

// OpenOrderRef — «уже в открытом заказе З-0012 — 5 шт.» (DS-004).
type OpenOrderRef struct {
	ID     string
	Number int64
	Qty    string
}
