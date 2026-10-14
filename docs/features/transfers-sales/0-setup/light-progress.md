# light-progress: transfers-sales (часть 5)

## Шаги
- [x] 0. Разведка кода (transfers dto/validator/service/repo, board totalAmountSelectSQL, card, print, JS, e2e)
- [x] 1. Миграция 0016 (migrations/0016_transfers_sales.{up,down}.sql) + тест migration_sales_integration_test.go (НЕ прогнан)
- [x] 2. transfers: DTO (price, line_total, amount), валидатор цены, товар в warehouse_to_client — код готов, go build ок
- [x] 3. transfers repo: insertItems(direction) с ценой, amountSelectSQL в Get/List, sendsStock() → client_out/transfer_return, resolveReceive для клиентских — пустые факты
- [x] 4. board: transfersAmountSQL + totalAmountSelectSQL, Task/TaskResponse.TransfersAmount, CardTransfer.Amount — go build ок
- [x] 5. Печатная форма «Выдача»: print-transfers-amount (print-handover.html)
- [x] 6. UI transfers.js: DIR.warehouse_to_client {products, priced}; testid: st-item-price, st-item-price-error, st-item-total, st-amount, st-view-price, st-view-total, st-row-amount, st-price-head; товары грузятся и исполнителю; node --check ок
- [x] 7. UI task-card.js: card-transfer-amount, card-transfers-amount(-row) над card-total; node --check ок
- [x] 8. seeddemo: seedInvoices (demoInvoices: products[3] received/paid, products[4] in_transit/unpaid) + assertInvoices; `go test ./internal/seeddemo/... -tags=integration` — ok
- [x] 9a. Go-тесты transfers: validate_sales_test.go (TestTS003_*, TestTS014_SendsStock…), transfers_sales_http_live_test.go (TS002/TS003/TS007/TS014), правка части 4: validate_partners_test.go (TS002 матрица, TS008), transfers_partners_http_live_test.go (убран кейс «товар в возврате клиенту»), DTO в transfers_http_live_test.go; unit + integration transfers — ok
- [x] 9b. Go-тесты board: order_card_sales_http_live_test.go (TestTS014_OrderTotalIncludesSentInvoices, вкл. печать «Выдача»), правки order_card_http_live_test.go (DTO, Totals), print_http_live_test.go, equipment_location_http_live_test.go:231 (Amount "0.00"); integration board — ok
- [x] 10. E2E: fixtures/auth.ts + board-types.ts (wire-типы), TransfersPage / OrderCardPage, tests/transfers-sales.spec.ts — TR-017…TR-020 (TS-030); tsc + `playwright --list` ок; ФАКТИЧЕСКИ НЕ ПРОГОНЯЛИСЬ (зона оркестратора)
- [x] 11. Self-check'и: go build, go vet -tags=integration (transfers, board, seeddemo), go test ./... (unit), integration transfers / board / seeddemo — ok; node --check transfers.js, task-card.js — ok

## Fix-loop 1
- [x] MEDIUM: суммы формы — BigInt (toScaled / lineCents / fmtCents / rowCents в transfers.js), несохранённая правка не перетирает серверный line_total; TR-021
- [x] LOW: пустая цена — превью по цене каталога + placeholder
- [x] LOW: Go-тесты округления на «половине» (transfers live, board live)
- [x] LOW: assertInvoices — stock >= 0
- self-check'и по одному вызову — все ok; E2E не запускались

## Итог
Все пункты закрыты. seeddemo: накладные на products[2] (received, оплаченный заказ) и products[5] (in_transit, неоплаченный).

## Решения
- Деньги на wire — JSON-число (numeric.Amount, как subtask_products.price), не строка; price в запросе — *numeric.Amount (нет поля/null → цена каталога); строка "abc" → 400 разбора тела (as-built).
- Коды ошибок цены — по спеке: negative / invalid / not_allowed (не коды board invalid_format/too_precise).
- Сумма = Σ round(price × qty_sent, 2) по строкам (как итог услуг в board), а не round(Σ).
- transfers_amount — в TaskResponse (карточка, GET /tasks/:id, mutation-ответы), не в TaskCard списка.
- 0016 down: товарные строки накладных «склад → клиент» удаляются (в модели 0015 недопустимы), остаток не пересчитывается.
