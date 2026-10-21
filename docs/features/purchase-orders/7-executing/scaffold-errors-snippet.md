# Errors snippet: purchase-orders <!-- scaffold stub -->

Shared-файлы pkg/errorspkg скаффолдер НЕ редактирует (идемпотентность).
backend-dev мержит блоки ниже в фазе errors и раскомментирует case-ветки в mappers/helpers.go.

## pkg/errorspkg/errors.go
```go
var ErrForbidden = errors.New("403 FORBIDDEN")
var ErrPurchaseOrderNotFound = errors.New("404 PURCHASE_ORDER_NOT_FOUND")
var ErrPurchaseOrderNotFound = errors.New("executor, заказ создан другим")
var ErrPurchaseOrderVersionConflict = errors.New("«Заказ изменился, обновите страницу»")
var ErrPurchaseOrderReadOnly = errors.New("«Поставщик уже собрал заказ — правка недоступна»")
var ErrPurchaseOrderReadOnly = errors.New("«Заказ закрыт — правка недоступна»")
var ErrPurchaseOrderReadOnly = errors.New("«Заказ отменён — правка недоступна»")
var ErrForbidden = errors.New("«Удалять может только администратор»")
var ErrPurchaseOrderInvalidStatus = errors.New("409 PURCHASE_ORDER_INVALID_STATUS")
var ErrPurchaseOrderNoSupplier = errors.New("«Список на базар не отправляется — отметьте «Забрали»»")
var ErrPurchaseOrderEmpty = errors.New("«Добавьте хотя бы одну строку»")
var ErrPurchaseOrderInvalidStatus = errors.New("статус s:ready/s:picked_up/s:cancelled")
var ErrPurchaseOrderInvalidStatus = errors.New("статус s:ready (поставщик успел нажать «Собрано»)")
var ErrPurchaseOrderInvalidStatus = errors.New("статус s:draft/s:picked_up/s:cancelled")
var ErrPurchaseOrderNotReady = errors.New("sent, у поставщика есть активный вход")
var ErrPurchaseOrderNotReady = errors.New("«Сначала отправьте заказ поставщику»")
var ErrPurchaseOrderInvalidStatus = errors.New("«Заказ уже закрыт — отмена недоступна»")
var ErrTODOName = errors.New("заказа нет или executor открывает чужой")
var ErrForbidden = errors.New("«Раздел только для поставщиков»")
var ErrPurchaseOrderNotFound = errors.New("чужой, черновик, отменён, удалён или нет")
var ErrPurchaseOrderNotFound = errors.New("заказ или строка чужие или не найдены")
var ErrPurchaseOrderReadOnly = errors.New("«Заказ уже отмечен «Собрано»»")
var ErrPurchaseOrderReadOnly = errors.New("409 PURCHASE_ORDER_READ_ONLY")
var ErrPurchaseOrderReadOnly = errors.New("заказ отозван или отменён (нет в кабинете)")
var ErrPurchaseItemVersionConflict = errors.New("«Строку изменили, обновите страницу»")
var ErrPurchaseOrderNotFound = errors.New("чужой, отозван, отменён или нет")
var ErrPurchaseOrderItemsIncomplete = errors.New("«Заполните «Соберу» во всех строках» (+ details.item_ids)")
var ErrPurchaseOrderItemsIncomplete = errors.New("«Укажите цену у строк, которые соберёте» (+ details.item_ids)")
var ErrTODOName = errors.New("страница «Нет доступа»")
var ErrTODOName = errors.New("чужой, черновик, отменён")
var ErrForbidden = errors.New("«Нет доступа» (as-is)")
var ErrSupplierHasOpenPurchases = errors.New("«У поставщика есть незакрытые закупки: З-0012, З-0015»")
```

## pkg/errorspkg/codes.go (SupportMessageCodes)
```go
"PURCHASE_ORDERS_FORBIDDEN": "403 FORBIDDEN", // HTTP 403
"PURCHASE_ORDERS_PURCHASE_ORDER_NOT_FOUND": "404 PURCHASE_ORDER_NOT_FOUND", // HTTP 404
"PURCHASE_ORDERS_PURCHASE_ORDER_NOT_FOUND": "executor, заказ создан другим", // HTTP 404
"PURCHASE_ORDERS_PURCHASE_ORDER_VERSION_CONFLICT": "«Заказ изменился, обновите страницу»", // HTTP 409
"PURCHASE_ORDERS_PURCHASE_ORDER_READ_ONLY": "«Поставщик уже собрал заказ — правка недоступна»", // HTTP 409
"PURCHASE_ORDERS_PURCHASE_ORDER_READ_ONLY": "«Заказ закрыт — правка недоступна»", // HTTP 409
"PURCHASE_ORDERS_PURCHASE_ORDER_READ_ONLY": "«Заказ отменён — правка недоступна»", // HTTP 409
"PURCHASE_ORDERS_FORBIDDEN": "«Удалять может только администратор»", // HTTP 403
"PURCHASE_ORDERS_PURCHASE_ORDER_INVALID_STATUS": "409 PURCHASE_ORDER_INVALID_STATUS", // HTTP 409
"PURCHASE_ORDERS_PURCHASE_ORDER_NO_SUPPLIER": "«Список на базар не отправляется — отметьте «Забрали»»", // HTTP 409
"PURCHASE_ORDERS_PURCHASE_ORDER_EMPTY": "«Добавьте хотя бы одну строку»", // HTTP 409
"PURCHASE_ORDERS_PURCHASE_ORDER_INVALID_STATUS": "статус s:ready/s:picked_up/s:cancelled", // HTTP 409
"PURCHASE_ORDERS_PURCHASE_ORDER_INVALID_STATUS": "статус s:ready (поставщик успел нажать «Собрано»)", // HTTP 409
"PURCHASE_ORDERS_PURCHASE_ORDER_INVALID_STATUS": "статус s:draft/s:picked_up/s:cancelled", // HTTP 409
"PURCHASE_ORDERS_PURCHASE_ORDER_NOT_READY": "sent, у поставщика есть активный вход", // HTTP 409
"PURCHASE_ORDERS_PURCHASE_ORDER_NOT_READY": "«Сначала отправьте заказ поставщику»", // HTTP 409
"PURCHASE_ORDERS_PURCHASE_ORDER_INVALID_STATUS": "«Заказ уже закрыт — отмена недоступна»", // HTTP 409
"PURCHASE_ORDERS_TODO_NAME": "заказа нет или executor открывает чужой", // HTTP 404
"PURCHASE_ORDERS_FORBIDDEN": "«Раздел только для поставщиков»", // HTTP 403
"PURCHASE_ORDERS_PURCHASE_ORDER_NOT_FOUND": "чужой, черновик, отменён, удалён или нет", // HTTP 404
"PURCHASE_ORDERS_PURCHASE_ORDER_NOT_FOUND": "заказ или строка чужие или не найдены", // HTTP 404
"PURCHASE_ORDERS_PURCHASE_ORDER_READ_ONLY": "«Заказ уже отмечен «Собрано»»", // HTTP 409
"PURCHASE_ORDERS_PURCHASE_ORDER_READ_ONLY": "409 PURCHASE_ORDER_READ_ONLY", // HTTP 409
"PURCHASE_ORDERS_PURCHASE_ORDER_READ_ONLY": "заказ отозван или отменён (нет в кабинете)", // HTTP 409
"PURCHASE_ORDERS_PURCHASE_ITEM_VERSION_CONFLICT": "«Строку изменили, обновите страницу»", // HTTP 409
"PURCHASE_ORDERS_PURCHASE_ORDER_NOT_FOUND": "чужой, отозван, отменён или нет", // HTTP 404
"PURCHASE_ORDERS_PURCHASE_ORDER_ITEMS_INCOMPLETE": "«Заполните «Соберу» во всех строках» (+ details.item_ids)", // HTTP 409
"PURCHASE_ORDERS_PURCHASE_ORDER_ITEMS_INCOMPLETE": "«Укажите цену у строк, которые соберёте» (+ details.item_ids)", // HTTP 409
"PURCHASE_ORDERS_TODO_NAME": "страница «Нет доступа»", // HTTP 403
"PURCHASE_ORDERS_TODO_NAME": "чужой, черновик, отменён", // HTTP 404
"PURCHASE_ORDERS_FORBIDDEN": "«Нет доступа» (as-is)", // HTTP 403
"PURCHASE_ORDERS_SUPPLIER_HAS_OPEN_PURCHASES": "«У поставщика есть незакрытые закупки: З-0012, З-0015»", // HTTP 409
```
