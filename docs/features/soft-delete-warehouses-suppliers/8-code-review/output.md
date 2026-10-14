---
stage: 8-code-review
feature: soft-delete-warehouses-suppliers
tier: S
verdict: APPROVED
high: 0
medium: 0
low: 10
---
# Code Review: soft-delete-warehouses-suppliers — APPROVED
**Review level:** Slim (tier: S) + Sec 18 API-линза (spec-track merged, §ST-1)

Диапазон: 84c3315..6d85d32 (первый проход — 44beb03, повторный — `git diff 44beb03..6d85d32`). Источник правды: draft-spec.md (экстракт в input-bundle.md). Гейты сборки/тестов/E2E — по фактам оркестратора, не перегонялись. Проверено чтением кода, без прогона.

## Блокеры (обязательно исправить до мержа)
Нет.

## Серьёзные замечания (исправить в этом PR)
Нет. Оба замечания первого прохода закрыты коммитом 6d85d32:
1. **Списание заказом на удалённый склад — ЗАКРЫТО.** `internal/features/board/subtask_products_repo.go` `QtyIncreaseOnDeletedWarehouse` (склад `FOR SHARE`, сравнение `$2::numeric > qty`), `subtask_products_service.go:161–173` → 409 `WAREHOUSE_DELETED`. Иных путей увеличить списание действием над строкой нет: новая строка — `checkWarehouse(id, "")` → `WarehouseState` (удалённый = `not_found`); `CopyKit` — только `DefaultWarehouse`; смены товара у строки нет (`Update` — qty, price, warehouse); вставок в `subtask_products` в коде две (`subtask_products_repo.go`, `subtasks_repo.go`). Смена склада на действующий, уменьшение, удаление строки проходят. Перенос по колонкам и восстановление заказа не блокируются — решение оркестратора; долг виден и списывается (`TestDebtOnDeletedWarehouseVisibleAndWrittenOff`). Порядок блокировок: заказ → строка → склад (`FOR SHARE`) → `product_stock` — как у новой строки.
2. **Архивный товар на удалённом складе — ЗАКРЫТО.** `internal/features/board/stock_repo.go`: третья ветка `stockProductVisibleSQL`, `stockWarehouseVisibleSQL` (удалённый склад «проявляет» остаток любого товара), `p_archived` → состояние `ok`, `Summary` — `NOT p_archived`, `StockRow.is_archived`, `$AFORCE` для ответа корректировки. Дублей строк нет (`keys` — UNION, группировка по `p.id`; `p.is_active` в GROUP BY функционально зависим). Для действующих активных товаров первая ветка видимости и расчёт пар не изменены; счётчики и `products_total` прежние. Окно корректировки архивного товара — только удалённые склады с его остатком (`stock.js`).
3. **`DefaultWarehouse` — ЗАКРЫТО.** Под блокировкой требуется `exists && active`.

## Незначительные замечания (можно в следующем PR)
1. **[docs/features/soft-delete-warehouses-suppliers/draft-spec.md]** Код 409 `WAREHOUSE_DELETED`, поле `StockRow.is_archived`, правило «увеличение qty на удалённом складе запрещено; перенос по колонкам и восстановление заказа — нет» в спеке отсутствуют — внести решением DS (оркестратор заявил).
2. **[web/ssr/_shared/js/task-card.js `onRowError`, ветка `WAREHOUSE_DELETED`; stock.js бейдж `stock-row-archived`]** Новые ветки UI покрыты только live-тестами API; E2E нет.
3. **[internal/features/board/stock_repo.go `rows`]** У строки архивного товара `min_stock` и `pending` суммируются по активным складам (не обнулены, в отличие от состояния) — «Минимум» и «Прогноз» строки с бейджем «в архиве» ненулевые; косметика.
4. **[internal/features/board/stock_repo.go `Adjust`]** Архивный товар, попавший в отчёт из-за остатка на удалённом складе, через API корректируется и на активном складе (раньше — 404, строки в отчёте не было); экран такого склада не предлагает.
5. **[internal/features/board/subtask_products_repo.go `DefaultWarehouse`]** Ветка повтора по-прежнему без теста гонки.
6. **[transfers `Send`/`Unreceive`/`Restore` vs board `Adjust`, `ProductState` → `WarehouseState`, catalog `checkStockWarehouses`]** Порядок блокировок разный: transfers — `warehouses → products`, остальные — `products → warehouses`. Все `FOR SHARE`; цикл возможен только при одновременных удалении товара и удалении склада — теоретическая взаимоблокировка → 40P01 → 500.
7. **[internal/features/warehouses/soft_delete.go `softDelete`, `Purge`; service.go `withLastActiveGuard`]** Три копии цикла «блокировка активных складов + подсчёт» — вынести в помощник.
8. **[internal/features/board/stock_repo.go `Adjust`]** Удалённый товар + несуществующий `warehouse_id`: было 404 `PRODUCT_NOT_FOUND`, стало 400 `warehouse_id: invalid`.
9. **[internal/features/clients/history.go:116–137]** История клиента показывает названия удалённых складов без пометки «удалён»; «Итого» в «Остатках» включает остаток на удалённых складах, «Итого» формы товара — нет.
10. **[checks/e2e_drift.txt]** FAIL — ложный: чекер смотрит `scripts/e2e`; TS-045 покрыт SDWS-001, SDWS-002, WH-001, SUP-001. `contract` SKIP-RESOLVE — неприменим (Vue нет). ~89 предупреждений 409 уборки — известная утечка одноразового стенда.

## Ответы на вопросы оркестратора (первый проход, актуальны)
1. **Обходы запрета.** Создание/правка документа — `checkRefs` → `refFlags(…, true)` `FOR SHARE ORDER BY id`. `Send`/`Unreceive`/`Restore` — `checkLiveWarehouses`. Приём и отмена «В пути» без проверки — верно: «В пути» достижим только через `Send`, `Unreceive`, создание с отправкой (все под `FOR SHARE`); удалить можно только `requested`/`cancelled` (`transfers/repo.go:801`); удалённая заявка → удаление склада → её восстановление = 409 `TRANSFER_HAS_DELETED_REFS`. Строка заказа — `WarehouseState FOR SHARE` + `QtyIncreaseOnDeletedWarehouse`. Форма товара — `checkStockWarehouses FOR SHARE`. Оборудование на склад пишет только transfers из «В пути». `default_warehouse_id()` — только действующий.
2. **Блокировки.** `FOR UPDATE` только строк `warehouses` достаточно: каждая запись, создающая ссылку или остаток, берёт склад `FOR SHARE` и после ожидания перечитывает строку; `deleteBlockers` — один оператор с новым снимком после блокировок. Порядок складов — по возрастанию id; транзакции board берут один склад. Рабочих взаимоблокировок нет (LOW-6 — теоретическая).
3. **DS-044.** `InUse` и `deleteBlockers` считают только `requested`/`in_transit` с `deleted_at IS NULL`. «Последний активный»: одинаковый набор и порядок блокировок у архива, удаления, purge.
4. **DS-049.** Итоги = активные + ненулевые архивные и удалённые склады; минимум и потребность на них нулевые, состояние `ok`, в `Summary` не входят. `is_archived` склада у удалённого = false (JS сначала смотрит `is_deleted`). Запрос — фильтр по `warehouses` с `EXISTS` по `product_stock_warehouse_id_idx`. Сброс фильтра: `stock.js`, `board.js:342`.
5. **Поставщик.** Один UPDATE с условием; список и правка — `deleted_at IS NULL`; в новом документе и при смене — `not_found`; уже стоящий удалённый допускается; частичные индексы и `ON CONFLICT … WHERE deleted_at IS NULL` корректны.
6. **Гарды.** `/api/v1/warehouses` и `/api/v1/suppliers`: GET, POST, PUT `/:id` — `RequireAdmin(nil)`; DELETE `/:id`, POST `/:id/restore` — `softdelete.AdminOnly()`; группа — `RequireAPI()`. Маршрутов без admin-гарда нет.
7. **Миграция 0023.** up/down симметричны; down при одинаковых названиях упадёт — задокументировано, MVP (§CM-1). WH-001, SUP-001: проверка архива сохранена.
8. **Каскад E2E первого прогона.** В диффе причины нет (транзакции с `defer Rollback`, `rows` закрываются, вход и middleware не тронуты) — внешнее (стенд/проброс порта Docker). Оговорка: если журнал пишется по завершении запроса, зависшие запросы в нём не видны; при повторе снять `pg_stat_activity`, дамп горутин, `docker ps`. Два повторных полных прогона — 228 passed.

## Соответствие кода спеке (верификация реализации)
- [x] DS-021, DS-044 — ДА
- [x] DS-009, DS-017 — ДА
- [x] DS-045, DS-046, DS-047 — ДА
- [x] DS-042, DS-048, DS-049 — ДА (вкл. архивный товар на удалённом складе)
- [x] DS-003, DS-020, DS-024, DS-026, DS-028, DS-030 — ДА
- [x] §Impact #1–#13 — след в diff у каждой позиции — ДА
- [x] Sec 18: ключи snake_case; новые признаки — не-nullable bool; ошибки — `pkg/errorspkg`; конверты списков не менялись; N+1 нет — ДА (новые `WAREHOUSE_DELETED`, `is_archived` — вне спеки до внесения DS, LOW-1)
- [x] Security: admin-гард до поиска записи, параметры SQL, причина в журнал не пишется — ДА
- [x] Tests: live TS-031, 032, 033, 055, 057, 059, 060, 064, 065; 16 тестов гонок; фиксы — `TestQtyIncreaseRefusedOnDeletedWarehouse`, `TestRace_QtyIncreaseWaitsForWarehouseDelete`, `TestDebtOnDeletedWarehouseVisibleAndWrittenOff`, `TestArchivedProductStockOnDeletedWarehouse`, `TestArchivedProductOnArchivedWarehouse_VisibleInProductStocks`; TS-012; E2E SDWS-001…003 — ДА
- Monitoring / Runbook / Performance Budget / PRD — N/A (S-tier, PRD нет)

## Статус фаз
- [x] Все фазы закрыты: `0-setup/light-progress.md` — все пункты `- [x]` — ДА

## Итог
APPROVED: блокеров и серьёзных замечаний нет. До закрытия части — внести в draft-spec решения по `WAREHOUSE_DELETED` и `is_archived` (LOW-1).
