// Package constants — статусы, источники строк и границы полей закупок по дефициту
// (docs/features/purchase-orders/draft-spec.md §4–§6). Единый источник правды для
// валидаторов, сервиса, SQL-фильтров и шаблонов.
package constants

import "fmt"

// Статусы заказа (§5, DS-011).
const (
	StatusDraft     = "draft"
	StatusSent      = "sent"
	StatusReady     = "ready"
	StatusPickedUp  = "picked_up"
	StatusCancelled = "cancelled"
)

// Statuses — все статусы (фильтр списка §6.1).
var Statuses = []string{StatusDraft, StatusSent, StatusReady, StatusPickedUp, StatusCancelled}

// SupplierStatuses — статусы, видимые в кабинете поставщика (§6.12, DS-019).
var SupplierStatuses = []string{StatusSent, StatusReady, StatusPickedUp}

// OpenStatuses — «незакрытые» заказы: блокируют архив и удаление поставщика (DS-043).
var OpenStatuses = []string{StatusDraft, StatusSent, StatusReady}

// Источник строки (§4.2, DS-022).
const (
	ReasonForOrders = "for_orders"
	ReasonBelowMin  = "below_min"
	ReasonManual    = "manual"
)

// Reasons — допустимые источники строки.
var Reasons = []string{ReasonForOrders, ReasonBelowMin, ReasonManual}

// Границы (DS-047, DS-036, DS-057).
const (
	MaxItems        = 200
	NoteMax         = 1000
	SupplierNoteMax = 500
	CancelReasonMax = 500

	ListDefaultLimit        = 50
	ListMaxLimit            = 100
	SuggestionsDefaultLimit = 500
	SuggestionsMaxLimit     = 500
)

// QtyIntDigits/QtyFracDigits — numeric(12,3) ≤ 99999.999 (DS-029, DS-047);
// PriceIntDigits/PriceFracDigits — numeric(12,2) ≤ 9999999.99.
const (
	QtyIntDigits    = 5
	QtyFracDigits   = 3
	PriceIntDigits  = 7
	PriceFracDigits = 2
)

// BazaarName — получатель заказа без поставщика (DS-006).
const BazaarName = "Базар (сами)"

// In — v входит в list.
func In(v string, list []string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

// NumberDisplay — «З-» + номер, дополненный нулями до 4 знаков: З-0001, после 9999 — З-10000
// (DS-056).
func NumberDisplay(n int64) string { return fmt.Sprintf("З-%04d", n) }

// StatusTitles — подписи бейджей статуса (§8.1).
var StatusTitles = map[string]string{
	StatusDraft:     "Черновик",
	StatusSent:      "Отправлен",
	StatusReady:     "Собран",
	StatusPickedUp:  "Забрали",
	StatusCancelled: "Отменён",
}
