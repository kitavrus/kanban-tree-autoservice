package models

import "time"

// PurchaseOrderItem — строка заказа (§4.2) с вычисленными флагами.
type PurchaseOrderItem struct {
	ID                 string
	OrderID            string
	ProductID          string
	Position           int
	Qty                string
	Reason             string
	Name               string
	Sku                string
	Unit               string
	SupplierQty        *string
	SupplierPrice      *string
	SupplierNote       string
	SupplierAnsweredAt *time.Time
	CustomerChangedAt  *time.Time
	Version            int64

	// LineAmount — supplier_qty × supplier_price, округление до 0.01 (DS-009); nil — нет цены.
	LineAmount *string
	// HasDiscrepancy — ответ задан и «Соберу» ≠ «Просим» (§6.3).
	HasDiscrepancy bool
	// CustomerChanged — «Изменено заказчиком» действует: правка после отправки, поставщик ещё
	// не ответил заново (DS-048).
	CustomerChanged bool
}

// ItemInput — строка из запроса создания/правки (уже провалидирована и сведена по
// product_id сервисом). ID == "" — новая строка.
type ItemInput struct {
	ID        string
	ProductID string
	Qty       string
	Reason    string
}

// ExistingItem — строка заказа в момент правки (сверка §6.4).
type ExistingItem struct {
	ID        string
	ProductID string
	Qty       string
	Reason    string
}

// SupplierAnswer — ответ поставщика по строке (§6.14).
type SupplierAnswer struct {
	Version       int64
	SupplierQty   *string
	SupplierPrice *string
	SupplierNote  string
}

// AnswerState — строка в момент ответа поставщика: вход проверок §6.14/§6.15.
type AnswerState struct {
	ItemID        string
	Version       int64
	SupplierQty   *string
	SupplierPrice *string
}
