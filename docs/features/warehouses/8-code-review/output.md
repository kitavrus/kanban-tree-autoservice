---
stage: 8-code-review
feature: warehouses
tier: M
verdict: APPROVED
high: 0
medium: 3
low: 5
---
# Code Review: warehouses — APPROVED
**Review level:** Standard (tier: M) · Режим: spec-track merged (§ST-1, Sec 18 включена) · base 1a7281f..4a68b1e

Pre-condition (судейство, факт): build / go test ./... / golangci-lint 0 / integration diff-скоуп 18/18 / E2E 159/159 (`9-validation/evidence/last-run.json`). Чекеры в bundle: все ✅, кроме e2e_drift ❌. Это ложное срабатывание по формату: ID записаны как `WH-003 (TS-003, TS-007)`, а чекер ждёт `test('TS-NNN: …')`. Все 14 TS покрыты (проверил оркестратор через git grep). Дефектом не считается.

## Блокеры (обязательно исправить до мержа)
Нет.

## Серьёзные замечания (исправить в этом PR)
1. **[internal/features/board/task_equipment_validate.go:55, task_equipment_service.go:161]** `TaskEquipmentSendRequest.normalize()` только обрезает пробелы (TrimSpace), регистр не приводит. `uuid.Validate` принимает UUID в верхнем регистре, поэтому сравнение `in.ToWarehouseID == from` (from приходит из БД в нижнем регистре) не срабатывает. В итоге `SetInTransit` получает from = to, падает на CHECK `task_equipment_location_check` (23514) и отдаёт **500** вместо 400 `to_warehouse_id: same_warehouse` (TS-012). Тот же класс ошибки в `UpdateSubtaskProduct` (`in.WarehouseID != curWarehouse`, subtask_products_service.go:156): если у строки архивный склад, повторная отправка его же id в верхнем регистре даёт ложный 400 `archived`. Исправление — `strings.ToLower` в normalize у `TaskEquipmentSendRequest` / `TaskEquipmentCreateRequest` / `SubtaskProduct*Request`, как уже сделано в `stocktransfers/validate.go` `Input.normalize`.
2. **[internal/features/stocktransfers/repo.go:362-365, `lockPairs`]** Нарушен порядок блокировок D50/DS-005 на шаге ensure. `INSERT INTO product_stock … SELECT … FROM stock_transfer_items` идёт без `ORDER BY`, то есть строки вставляются в порядке позиций заявки, а не в порядке (product_id, warehouse_id). Сценарий: два одновременных send/receive/cancel со встречным порядком товаров, и пар товар×склад ещё нет. Каждый ждёт чужую незакоммиченную спекулятивную вставку, возникает deadlock 40P01 и 500 (данные не портятся, транзакция откатывается). В Reconcile такой проблемы нет: unnest массивов идёт по уже отсортированным дельтам (`ORDER BY 1, 2`). Исправление — `… WHERE i.transfer_id = $1 ORDER BY i.product_id ON CONFLICT DO NOTHING`.
3. **[internal/features/warehouses/service.go:88/110/128, `guardLastActive`]** Проверка D47 работает по схеме check-then-act и выполняется вне транзакции. Сначала идёт отдельный `count(*)` на pool, потом отдельный `repo.Update` / `SetActive`. Если одновременно архивировать два последних активных склада, оба запроса пройдут, и активных складов станет 0. После этого `default_warehouse_id()` вернёт NULL, а `errNoActiveWarehouse` (subtask_products_repo.go:67, task_equipment_repo.go:48) нигде не маппится. Итог: 500 на добавлении услуги, товара и оборудования. Исправление — транзакция с `SELECT id FROM warehouses WHERE is_active FOR UPDATE` перед архивированием, либо условный `UPDATE … WHERE … AND EXISTS (другой активный)`.

## Незначительные замечания (можно в следующем PR)
1. **[internal/features/board/subtask_products_repo.go:67, task_equipment_repo.go:48]** `errNoActiveWarehouse` не маппится и уходит как 500. Защитная ветка, но стоит отдавать 409 с понятным сообщением (связано с п.3).
2. **[migrations/0010_task_equipment_warehouses.up.sql:12]** `UPDATE task_equipment SET warehouse_id = (SELECT id FROM warehouses WHERE name = 'Алматы')`. Если 0008 и 0010 применены в разные релизы и между ними склад переименовали, получится NULL и миграция упадёт на `task_equipment_location_check`. Падение транзакционное, данные не теряются. Надёжнее `coalesce(…, default_warehouse_id())`.
3. **[migrations/0008_warehouses.down.sql:11]** down удаляет движения `transfer_*` из журнала, а остатки схлопывает суммой по складам. Это осознанно lossy и допустимо для режима to-be поверх main, но стоит упомянуть в комментарии вместе с потерей журнала.
4. **[internal/features/stocktransfers/repo.go:60]** `total` считается через `count(*) OVER ()`: при offset больше числа строк ответ `total=0`, хотя записи есть. В `warehouses/repo.go` total считается отдельным запросом, поведение стоит выровнять.
5. **[DS-008 vs D56]** В спеке склад приёма оборудования обязателен. В API он опционален: если не передан, берётся первый активный склад, а в UI select всегда заполнен. Это расхождение зафиксировано решением D56, оно безвредно. Если считать DS нормой, стоит поправить формулировку в спеке.

Открытые вопросы оркестратора (решение за пользователем, в счёт не идут): удаление из заказа оборудования, которое в пути; строки товаров не следуют за переездом оборудования (D58).

## Проверено без замечаний (особое внимание)
- **StockEngine.Reconcile** (board/stock_engine.go): дельты по паре товар×склад через FULL JOIN demand/task_stock. Смена склада у списанной строки даёт 2 дельты: task_return на старый склад и task_consume на новый, всё в одной транзакции UpdateSubtaskProduct (TS-005). Ensure-строки создаются до `FOR UPDATE ORDER BY (product_id, warehouse_id)`. Предупреждения считаются по паре после применения дельт. Порядок блокировок: заказ → subtask → product_stock.
- **Перемещения**: send даёт −qty_sent (transfer_out). receive: +факт к остатку; в журнал +qty_sent (transfer_in) и −недостача (transfer_loss), сумма совпадает с фактом. cancel in_transit даёт +qty_sent (transfer_return), cancel requested движений не пишет. Финальные статусы → 409, version сверяется до статуса, `FOR UPDATE` на строке перемещения стоит первым. `resolveReceive` заполняет все строки (по умолчанию факт = qty_sent), проверяет 0 ≤ факт ≤ отправлено, плюс CHECK в БД.
- **Deadlock-граф**: Reconcile (task → ps), перемещения (transfer → ps одного склада), ручная правка товара (ps одного товара, сортировка по warehouse_id), Adjust (одна строка). Порядок (product_id, warehouse_id) везде согласован, циклов нет; исключение — п.2.
- **Миграции 0008–0010**: в up данные переносятся до DROP колонок, NOT NULL ставится после backfill, PK task_stock пересобирается. В down task_stock схлопывается суммой, products восстанавливаются (остаток суммой, минимум/коробка/место с «Алматы»), порядок DROP функции и колонок корректный. Есть integration-тесты миграций (warehouses/stocktransfers/equipment_warehouses `*_migration_integration_test.go`).
- **Права**: `/api/v1/warehouses`, `/api/v1/stock-transfers` и SSR `/warehouses`, `/stock-transfers` закрыты `RequireAdmin`. `PUT /api/v1/stock/:product_id` и `PUT /products/:id` — admin. Переходы оборудования заказа — без admin, как правка оборудования (D19/DS-010, TS-014). Пункты меню помечены `AdminOnly`.
- **XSS**: новые `stock-transfers.js`, правки `stock.js`/`task-card.js`/`board.js` — каждое значение из данных проходит через `esc()` (& < > " '), включая атрибуты. Шаблоны — `html/template`, `template.HTML` не используется.
- **SQL**: только параметры `$N`, LIKE экранируется (`sqlutil.EscapeLike`), uuid валидируется до каста (кроме п.1, где 500 возникает из-за CHECK, а не из-за каста).

## Соответствие кода спеке (draft-spec DS/TS, design-delta)
- [x] DS-001…DS-012 реализованы, объекты дельты есть в diff — ДА
- [x] D45–D60 не противоречат спеке (D56 — уточнение, см. LOW-5) — ДА
- [x] Ошибки — `pkg/errorspkg`, 400 → `details.fields[]`, новые коды: `WAREHOUSE_NOT_FOUND`, `WAREHOUSE_LAST_ACTIVE`, `STOCK_TRANSFER_*`, `STOCK_OVERFLOW`, `TASK_EQUIPMENT_IN_TRANSIT`, `TASK_EQUIPMENT_NOT_IN_TRANSIT` — ДА
- [x] TS-001…TS-014 покрыты live/integration/E2E тестами (e2e_drift — ложный FAIL по формату) — ДА
- [ ] Monitoring/Alerts/Runbook/Perf budget — N/A (M-tier, spec-track; spans `task_equipment.transition`, `subtask_products.mutate` есть)
- [ ] Swagger — N/A (в проекте нет swag-аннотаций)

## Sec 18 — API-контракт (слитая линза mobile-api)
- §API-1: URL kebab (`/stock-transfers`, `/cancel-transit`), JSON snake_case — ДА (api_conventions ✅, contract ✅).
- Списки `{items,total,limit,offset}`: warehouses, stock-transfers, stock — ДА (у stock есть дополнительный ключ `warehouses`, конверт не нарушен). Мелочь с total — LOW-4.
- Nullable: `warehouse_id` и `transit_*` у оборудования — `*string` ↔ NULL в БД и CHECK по статусу. `qty_received` и `sent_*`/`received_*`/`cancelled_*` — nullable, согласовано с DDL — ДА.
- Breaking: в API товара `stock/min_stock/box/place` заменены на `stocks[]` + `total_stock` (D46). Это to-be дельта, внешних клиентов нет (SSR) — допустимо.
- N+1: журнал оборудования, ячейки по складам, остатки товаров — каждое одним batch-запросом — ДА.
- Каждый случай ошибки §6 отдаёт свой код — ДА, кроме 500 в п.1 и п.3.

## Прочее
- §GO-1: пакеты `warehouses`, `stocktransfers` — ок. UUID: проект использует `github.com/google/uuid` как as-is конвенцию (30 файлов в base, `pkg/ids` нет), поэтому Sec 4.1 = N/A.
- §CM-1: пометка «to-be дельта поверх main», внешнего прода нет, поэтому lossy down не флагается.

## Статус фаз
- [x] Подфазы A/B/C закрыты (коммиты 6a19d83, 0d5bb67, 4a68b1e; `0-setup/light-progress.md`) — ДА

## Итог
APPROVED: блокеров нет. 3 MEDIUM стоит исправить в этом PR, это точечные правки на 1–5 строк: регистр uuid при отправке оборудования, `ORDER BY` в ensure у `lockPairs`, атомарность защиты последнего активного склада.
