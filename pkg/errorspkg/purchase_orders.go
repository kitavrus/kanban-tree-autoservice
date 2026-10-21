package errorspkg

import (
	"net/http"
	"strings"
)

// Ошибки закупок по дефициту (docs/features/purchase-orders/draft-spec.md §6). Один код —
// несколько текстов (сообщение зависит от статуса заказа), поэтому sentinel на пару
// «код + текст»; клиент ветвится по code, текст показывает как есть.

const (
	CodePurchaseOrderNotFound        = "PURCHASE_ORDER_NOT_FOUND"
	CodePurchaseOrderVersionConflict = "PURCHASE_ORDER_VERSION_CONFLICT"
	CodePurchaseOrderReadOnly        = "PURCHASE_ORDER_READ_ONLY"
	CodePurchaseOrderInvalidStatus   = "PURCHASE_ORDER_INVALID_STATUS"
	CodePurchaseOrderNoSupplier      = "PURCHASE_ORDER_NO_SUPPLIER"
	CodePurchaseOrderEmpty           = "PURCHASE_ORDER_EMPTY"
	CodePurchaseOrderNotReady        = "PURCHASE_ORDER_NOT_READY"
	CodePurchaseOrderItemsIncomplete = "PURCHASE_ORDER_ITEMS_INCOMPLETE"
	CodePurchaseItemVersionConflict  = "PURCHASE_ITEM_VERSION_CONFLICT"
	CodeSupplierHasOpenPurchases     = "SUPPLIER_HAS_OPEN_PURCHASES"
	CodePurchaseSupplierUnavailable  = "PURCHASE_ORDER_SUPPLIER_UNAVAILABLE"
)

func conflict(code, msg string) *AppError { return New(code, http.StatusConflict, msg) }

var (
	// 404 — нет, удалён, чужой (executor: создан другим; supplier: другой поставщик,
	// черновик, отменён) — §6.3–§6.9, §6.13–§6.15.
	ErrPurchaseOrderNotFound = New(CodePurchaseOrderNotFound, http.StatusNotFound, "Заказ не найден")

	// 403.
	ErrPurchaseOrderForbidden      = Forbidden("Нет доступа")
	ErrPurchaseOrderDeleteAdmin    = Forbidden("Удалять может только администратор")
	ErrPurchaseSupplierCabinetOnly = Forbidden("Раздел только для поставщиков")

	// 409 — версия (§3 «Конкурентность»).
	ErrPurchaseOrderVersionConflict = conflict(CodePurchaseOrderVersionConflict, "Заказ изменился, обновите страницу")
	ErrPurchaseItemVersionConflict  = conflict(CodePurchaseItemVersionConflict, "Строку изменили, обновите страницу")

	// 409 READ_ONLY — правка нами (§6.4).
	ErrPurchaseOrderReadOnlyReady     = conflict(CodePurchaseOrderReadOnly, "Поставщик уже собрал заказ — правка недоступна")
	ErrPurchaseOrderReadOnlyClosed    = conflict(CodePurchaseOrderReadOnly, "Заказ закрыт — правка недоступна")
	ErrPurchaseOrderReadOnlyCancelled = conflict(CodePurchaseOrderReadOnly, "Заказ отменён — правка недоступна")
	// 409 READ_ONLY — ответ поставщика (§6.14).
	ErrPurchaseAnswerReadOnlyReady    = conflict(CodePurchaseOrderReadOnly, "Заказ уже отмечен «Собрано»")
	ErrPurchaseAnswerReadOnlyPickedUp = conflict(CodePurchaseOrderReadOnly, "Заказ уже забрали")
	ErrPurchaseAnswerReadOnlyRecalled = conflict(CodePurchaseOrderReadOnly, "Заказ отозван заказчиком")

	// 409 INVALID_STATUS — переходы §5.
	ErrPurchaseOrderDeleteNotDraft   = conflict(CodePurchaseOrderInvalidStatus, "Удалить можно только черновик")
	ErrPurchaseOrderAlreadySent      = conflict(CodePurchaseOrderInvalidStatus, "Заказ уже отправлен")
	ErrPurchaseOrderSendNotDraft     = conflict(CodePurchaseOrderInvalidStatus, "Отправить можно только черновик")
	ErrPurchaseOrderRecallReady      = conflict(CodePurchaseOrderInvalidStatus, "Поставщик уже собрал заказ — отозвать нельзя")
	ErrPurchaseOrderRecallNotSent    = conflict(CodePurchaseOrderInvalidStatus, "Отозвать можно только отправленный заказ")
	ErrPurchaseOrderAlreadyClosed    = conflict(CodePurchaseOrderInvalidStatus, "Заказ уже закрыт")
	ErrPurchaseOrderIsCancelled      = conflict(CodePurchaseOrderInvalidStatus, "Заказ отменён")
	ErrPurchaseOrderCancelClosed     = conflict(CodePurchaseOrderInvalidStatus, "Заказ уже закрыт — отмена недоступна")
	ErrPurchaseOrderAlreadyCancelled = conflict(CodePurchaseOrderInvalidStatus, "Заказ уже отменён")
	ErrPurchaseOrderAlreadyReady     = conflict(CodePurchaseOrderInvalidStatus, "Заказ уже отмечен «Собрано»")
	ErrPurchaseOrderAlreadyPickedUp  = conflict(CodePurchaseOrderInvalidStatus, "Заказ уже забрали")

	// 409 — прочие guard'ы переходов.
	ErrPurchaseOrderNoSupplier        = conflict(CodePurchaseOrderNoSupplier, "Список на базар не отправляется — отметьте «Забрали»")
	ErrPurchaseOrderEmpty             = conflict(CodePurchaseOrderEmpty, "Добавьте хотя бы одну строку")
	ErrPurchaseOrderNotReadyWait      = conflict(CodePurchaseOrderNotReady, "Дождитесь отметки поставщика «Собрано»")
	ErrPurchaseOrderNotReadySendFirst = conflict(CodePurchaseOrderNotReady, "Сначала отправьте заказ поставщику")
	// «Собрано» с неполными строками (§6.15): details.item_ids — строки без ответа/цены.
	ErrPurchaseOrderItemsNoQty = conflict(CodePurchaseOrderItemsIncomplete, "Заполните «Соберу» во всех строках")
	// Восстановление черновика архивного/удалённого поставщика (DS-043, DS-072): вернуло бы
	// такому поставщику незакрытую закупку.
	ErrPurchaseOrderRestoreSupplierUnavailable = conflict(CodePurchaseSupplierUnavailable, "Поставщик недоступен — восстановить черновик нельзя")
	ErrPurchaseOrderItemsNoPrice               = conflict(CodePurchaseOrderItemsIncomplete, "Укажите цену у строк, которые соберёте")
)

// SupplierHasOpenPurchases — 409 архива/удаления поставщика с незакрытыми закупками
// (§6.18, DS-043): текст перечисляет номера («З-0012, З-0015»), details.numbers — они же.
func SupplierHasOpenPurchases(numbers []string) *AppError {
	e := conflict(CodeSupplierHasOpenPurchases, "У поставщика есть незакрытые закупки: "+strings.Join(numbers, ", "))
	return e.WithDetail("numbers", numbers)
}
