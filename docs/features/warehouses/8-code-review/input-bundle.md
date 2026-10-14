# Review Bundle: warehouses — CHECKS-FAILED
**Base:** 1a7281f0f7a2c2c0d73dc2e2ede50941e9cc8f0f | **HEAD:** 4a68b1e | **Generated:** 2026-10-02 10:45 | diff: 111 files changed, 8771 insertions(+), 510 deletions(-)

> Линзы ревью: читай ЭТОТ бандл + patch своей линзы. Полное чтение research/draft-spec/PRD
> и перегон чекеров ЗАПРЕЩЕНЫ (вердикты ниже; детали — checks/<name>.txt; точечный grep по DS-NNN разрешён).

## Чекеры (детерминированные — НЕ перегонять)
| Чекер | Вердикт | exit | Нарушения (первые строки) |
|---|---|---|---|
| go_naming | ✅ | 0 | — |
| api_conventions | ✅ | 0 | — |
| contract | ✅ | 0 | — |
| date_bound | ✅ | 0 | — |
| fuzz_verdict | ✅ | 0 | — |
| live_suite | ✅ | 0 | — |
| fixture_sync | ✅ | 0 | — |
| e2e_mode | ✅ | 0 | — |
| rule_usage | ✅ | 0 | — |
| e2e_drift | ❌ | 1 | e2e-drift: FAIL (режим: TS (spec-track §ST-1.4), doc: 14 id, code: 0 в 0 файлах, проблем: 14)<br>DRIFT: TS-001 есть в draft-spec.md, теста test('TS-001: …') нет (сценарий без теста)<br>DRIFT: TS-002 есть в draft-spec.md, теста test('TS-002: …') нет (сценарий без теста)<br>DRIFT: TS-003 есть в draft-spec.md, теста test('TS-003: …') нет (сценарий без теста)<br>DRIFT: TS-004 есть в draft-spec.md, теста test('TS-004: …') нет (сценарий без теста)<br>DRIFT: TS-005 есть в draft-spec.md, теста test('TS-005: …') нет (сценарий без теста)<br>DRIFT: TS-006 есть в draft-spec.md, теста test('TS-006: …') нет (сценарий без теста)<br>DRIFT: TS-007 есть в draft-spec.md, теста test('TS-007: …') нет (сценарий без теста) |

## Diff по линзам
| Линза | Файлов | Patch |
|---|---|---|
| backend | 74 | 8-code-review/diff-backend.patch |
| migrations | 6 | 8-code-review/diff-migrations.patch |
| other | 24 | 8-code-review/diff-other.patch |
| docs | 7 | — (артефакты, не ревьюятся построчно) |

### backend
- `M` internal/features/board/board_fuzz_test.go
- `M` internal/features/board/board_http_live_test.go
- `M` internal/features/board/card_dto.go
- `M` internal/features/board/card_handler.go
- `M` internal/features/board/card_integration_test.go
- `M` internal/features/board/card_repo.go
- `M` internal/features/board/card_service.go
- `M` internal/features/board/card_validate_test.go
- `M` internal/features/board/enums.go
- `A` internal/features/board/equipment_warehouse_validate_test.go
- `A` internal/features/board/equipment_warehouses_http_live_test.go
- `A` internal/features/board/equipment_warehouses_migration_integration_test.go
- `M` internal/features/board/errors.go
- `M` internal/features/board/order_card_http_live_test.go
- `M` internal/features/board/print_http_live_test.go
- `M` internal/features/board/routes.go
- `M` internal/features/board/ssr.go
- `M` internal/features/board/stock_dto.go
- `M` internal/features/board/stock_engine.go
- `M` internal/features/board/stock_engine_integration_test.go
- `M` internal/features/board/stock_handler.go
- `M` internal/features/board/stock_http_live_test.go
- `M` internal/features/board/stock_repo.go
- `M` internal/features/board/stock_service.go
- `M` internal/features/board/subtask_products_repo.go
- `M` internal/features/board/subtask_products_service.go
- `M` internal/features/board/subtask_products_validate.go
- `M` internal/features/board/subtasks_repo.go
- `M` internal/features/board/subtasks_service.go
- `M` internal/features/board/task_equipment_repo.go
- `M` internal/features/board/task_equipment_service.go
- `M` internal/features/board/task_equipment_validate.go
- `M` internal/features/board/tasks_dto.go
- `M` internal/features/board/tasks_filter.go
- `M` internal/features/board/tasks_repo.go
- `M` internal/features/board/tasks_service.go
- `A` internal/features/board/warehouses_card_http_live_test.go
- `M` internal/features/catalog/catalog_http_live_test.go
- `M` internal/features/catalog/products_dto.go
- `M` internal/features/catalog/products_handler.go
- `M` internal/features/catalog/products_repo.go
- `M` internal/features/catalog/products_service.go
- `A` internal/features/catalog/products_stock.go
- `M` internal/features/catalog/products_validate.go
- `M` internal/features/catalog/products_validate_test.go
- `M` internal/features/catalog/services_http_live_test.go
- `M` internal/features/catalog/ssr_products.go
- `M` internal/features/clients/clients_http_live_test.go
- `M` internal/features/clients/history.go
- `A` internal/features/stocktransfers/dto.go
- `A` internal/features/stocktransfers/handler.go
- `A` internal/features/stocktransfers/migration_integration_test.go
- `A` internal/features/stocktransfers/repo.go
- `A` internal/features/stocktransfers/routes.go
- `A` internal/features/stocktransfers/service.go
- `A` internal/features/stocktransfers/ssr.go
- `A` internal/features/stocktransfers/stocktransfers_http_live_test.go
- `A` internal/features/stocktransfers/validate.go
- `A` internal/features/stocktransfers/validate_test.go
- `A` internal/features/warehouses/dto.go
- `A` internal/features/warehouses/handler.go
- `A` internal/features/warehouses/migration_integration_test.go
- `A` internal/features/warehouses/repo.go
- `A` internal/features/warehouses/routes.go
- `A` internal/features/warehouses/service.go
- `A` internal/features/warehouses/ssr.go
- `A` internal/features/warehouses/validate.go
- `A` internal/features/warehouses/validate_test.go
- `A` internal/features/warehouses/warehouses_http_live_test.go
- `M` internal/seeddemo/seeddemo.go
- `M` internal/seeddemo/seeddemo_integration_test.go
- `A` internal/seeddemo/split_stock_test.go
- `M` internal/server/router.go
- `M` internal/ssr/menu.go

### migrations
- `A` migrations/0008_warehouses.down.sql
- `A` migrations/0008_warehouses.up.sql
- `A` migrations/0009_stock_transfers.down.sql
- `A` migrations/0009_stock_transfers.up.sql
- `A` migrations/0010_task_equipment_warehouses.down.sql
- `A` migrations/0010_task_equipment_warehouses.up.sql

### other
- `M` e2e/fixtures/auth.ts
- `M` e2e/fixtures/board-types.ts
- `M` e2e/pages/BoardPage.ts
- `M` e2e/pages/CatalogPage.ts
- `M` e2e/pages/OrderCardPage.ts
- `M` e2e/pages/StockPage.ts
- `A` e2e/pages/StockTransfersPage.ts
- `A` e2e/tests/equipment-warehouses.spec.ts
- `M` e2e/tests/order-card.spec.ts
- `A` e2e/tests/stock-transfers.spec.ts
- `A` e2e/tests/warehouses.spec.ts
- `M` web/ssr/_shared/js/board.js
- `A` web/ssr/_shared/js/stock-transfers.js
- `M` web/ssr/_shared/js/stock.js
- `M` web/ssr/_shared/js/task-card.js
- `M` web/ssr/_shared/js/ui-dialog.js
- `M` web/ssr/board/templates/pages/board.html
- `M` web/ssr/board/templates/pages/stock.html
- `M` web/ssr/catalog/templates/pages/products-form.html
- `M` web/ssr/catalog/templates/pages/products-list.html
- `M` web/ssr/clients/templates/pages/equipment-history.html
- `A` web/ssr/stocktransfers/templates/pages/list.html
- `A` web/ssr/warehouses/templates/pages/form.html
- `A` web/ssr/warehouses/templates/pages/list.html

### docs
- `A` docs/features/warehouses/0-setup/draft-spec.slice-light.md
- `A` docs/features/warehouses/0-setup/idioms.md
- `A` docs/features/warehouses/0-setup/light-progress.md
- `A` docs/features/warehouses/0-setup/pipeline-state.md
- `A` docs/features/warehouses/0-setup/timeline.jsonl
- `A` docs/features/warehouses/draft-spec.md
- `M` docs/plan.md

## Экстракты документов (вместо полного чтения)
### draft-spec.md (экстракт: шапка + DS + дельта + Impact + TS)
# Draft Spec — warehouses (Склады)

**Tier:** M (пользователь явно выбрал /pipeline-light; реализация в 3 подфазах A/B/C)
**Совместимость:** to-be дельта поверх main (1a7281f); существующие остатки/заказы/оборудование → склад «Алматы».
**UI-scope:** SSR (Go html/template + vanilla JS, Bootstrap 5.3, D38); справочники вручную (D43, scaffold skip).

## 1. Решения (DS)

- DS-001 (W1). Справочник `warehouses` (id uuid, name уникально без регистра/пробелов вкл. архивные, position, is_active, timestamps). Admin: список/создать/переименовать/архивировать (удаления нет). Сид миграции: «Алматы» (position 1), «Москва» (2). Экран «Склады» в сайдбаре (admin).
- DS-002 (W2). Остаток по складам: `product_stock(product_id, warehouse_id, stock numeric(12,3) default 0, min_stock numeric(12,3) ≥0, box text, place text, PK(product_id, warehouse_id))`. Миграция переносит `products.stock/min_stock/box/place` в строку склада «Алматы» и удаляет эти колонки из `products`. Цена/артикул/название — общие. Отсутствующая строка = 0 остаток, 0 минимум.
- DS-003 (W2). Карточка товара: блок «Склады» — по строке на каждый активный склад (остаток, минимум, коробка, место); ручная правка остатка (admin) → `stock_movements(reason=manual, warehouse_id)`.
- DS-004 (W2). Экран «Остатки»: фильтр по складу (все/конкретный); колонки по складам + «Итого» + «В пути» (сумма строк перемещений в статусе in_transit). KPI «ниже минимума» считается по паре товар×склад. Бейдж в сайдбаре — тоже по парам.
- DS-005 (W5). `subtask_products.warehouse_id` NOT NULL (миграция — Алматы) и `task_stock` PK → (task_id, product_id, warehouse_id). Списание/возврат (StockEngine.Reconcile, D34) идёт по паре товар×склад; блокировка `FOR UPDATE` строк `product_stock` в порядке (product_id, warehouse_id); отсутствующая строка создаётся (INSERT … ON CONFLICT DO NOTHING) перед блокировкой. Отрицательный остаток разрешён (предупреждение, как раньше). При добавлении товара в услугу заказа склад по умолчанию = текущий склад оборудования услуги (если оборудование на складе, не в пути), иначе первый активный склад по position; у строки товара склад можно сменить (select) — смена склада у уже списанной строки пересчитывает списание (возврат на старый, списание с нового) в одной транзакции. Комплект услуги (service_products) копируется с тем же складом по умолчанию.
- DS-006 (W4). Перемещение товаров: `stock_transfers(id bigserial — сквозной #N, from_warehouse_id, to_warehouse_id (≠ from), status ∈ requested|in_transit|received|cancelled, note, created_by, sent_by/sent_at, received_by/received_at, cancelled_by/cancelled_at, timestamps, version)` + `stock_transfer_items(id, transfer_id, product_id, qty_sent numeric(12,3) >0, qty_received numeric(12,3) NULL ≥0)`. Переходы:
  - создать как «Заявка» (requested) или сразу «Отправить» (in_transit);
  - requested → in_transit: строки редактируемы до отправки; при отправке −qty_sent с источника (`transfer_out`);
  - in_transit → received: ввод фактического qty_received по строкам (по умолчанию = qty_sent, 0 ≤ qty_received ≤ qty_sent); на получателе пишется `transfer_in` +qty_sent и, при недостаче, `transfer_loss` −(qty_sent−qty_received) — сумма = факт (журнал явно показывает потерю);
  - requested → cancelled (без движений); in_transit → cancelled (+qty_sent на источник, `transfer_return`);
  - received/cancelled — финальные, правка запрещена (409). Конкурентность: `version` (409 при рассинхроне) + FOR UPDATE.
  - Экран «Перемещения» (admin, master-detail): список с фильтром по статусу/складу, карточка с строками, кнопки по статусу. Права: только admin (DS-010).
- DS-007. `stock_movements`: + `warehouse_id uuid NOT NULL` (миграция — Алматы), + `transfer_id bigint NULL FK SET NULL`; reason расширен: `task_consume|task_return|manual|transfer_out|transfer_in|transfer_loss|transfer_return`.
- DS-008 (W3). Оборудование заказа: `task_equipment` + `warehouse_id uuid NULL` (текущий склад; NULL только когда в пути) , `status ∈ at_warehouse|in_transit` (default at_warehouse), `transit_to_warehouse_id NULL`. Миграция: существующие строки → Алматы/at_warehouse. При добавлении оборудования в заказ склад приёма обязателен (select, по умолчанию первый активный). Действия в карточке заказа: «Отправить на склад…» (выбор склада ≠ текущего → in_transit, from→to), «Принять» (in_transit → at_warehouse на складе назначения), «Отменить отправку» (обратно на исходный склад). Журнал `task_equipment_movements(id, task_id, equipment_id, action ∈ received|sent|arrived|cancelled, from_warehouse_id NULL, to_warehouse_id NULL, user_id, created_at)` — блок «Перемещения» в оборудовании карточки заказа (история). Права: как правка оборудования заказа (DS-010).
- DS-009. Доска: фильтр «Склад» (заказы, у которых хоть одно оборудование на складе/едет на склад X). Карточка на доске — бейдж «в пути» если есть оборудование in_transit.
- DS-010 (W6). Права: справочник складов, перемещения товаров, ручная правка остатков — admin; отправка/приём оборудования заказа — те же права, что редактирование оборудования заказа (как сейчас).
- DS-011. API (§API-1): `/api/v1/warehouses` (GET list {items,total,limit,offset}, POST, PUT /:id), `/api/v1/stock-transfers` (GET list, POST, GET/PUT /:id, POST /:id/send, /:id/receive, /:id/cancel), `/api/v1/tasks/:id/equipment/:equipment_id/send|receive|cancel-transit`; JSON snake_case. Ошибки — `pkg/errorspkg`.
- DS-012. seed-demo: остатки раскиданы по обоим складам, одно перемещение in_transit, одно received; часть оборудования демо-заказов в Москве, одно в пути.

## 11. Тестовые сценарии

- TS-001 Миграция: после up существующий остаток товара лежит на «Алматы», `products` без stock-колонок; down восстанавливает.
- TS-002 Склады CRUD: создать, дубль имени (регистр/пробелы, вкл. архивные) → 400 details.fields[name]; переименовать; архивировать; executor → 403.
- TS-003 Ручная правка остатка товара на складе Москва → строка product_stock + movement manual с warehouse_id; на Алматы не влияет.
- TS-004 Списание: заказ с товаром на складе Москва переходит в consumes_stock-колонку → минус на Москве, Алматы не меняется; обратный перенос — возврат на Москву.
- TS-005 Смена склада у списанной строки товара → возврат на старый склад, списание с нового, 2+2 движения.
- TS-006 Склад по умолчанию строки товара = склад оборудования услуги; без оборудования — первый активный склад.
- TS-007 Экран «Остатки»: фильтр по складу, колонки складов, «Итого», «В пути»; ниже-минимума по паре товар×склад.
- TS-008 Перемещение: заявка → правка строк → отправка (минус на источнике, «в пути») → приёмка по факту с недостачей (приход + transfer_loss) — итоговые остатки верны.
- TS-009 Отмена in_transit → возврат на источник; отмена requested → без движений; правка received → 409; from = to → 400.
- TS-010 Конкурентность перемещения: устаревший version → 409; двойной receive → 409.
- TS-011 Оборудование заказа: добавить с складом приёма; отправить в Москву (в пути, from→to); принять; история из 3 записей в карточке.
- TS-012 Отмена отправки оборудования → обратно на исходный склад; принять не-in_transit → 409; отправить на тот же склад → 400.
- TS-013 Фильтр доски по складу и бейдж «в пути».
- TS-014 Права: executor не видит «Склады»/«Перемещения» (403 API), но может отправить/принять оборудование заказа (если может править оборудование).

### prd.md
MISSING

## Инвентарь доков (точечный grep разрешён, полное чтение — нет)
- 8-code-review/input-bundle.md — 19KB
- 0-setup/idioms.md — 12KB
- draft-spec.md — 10KB
- 0-setup/light-progress.md — 6KB
- 0-setup/draft-spec.slice-light.md — 3KB
- 0-setup/pipeline-state.md — 0KB
