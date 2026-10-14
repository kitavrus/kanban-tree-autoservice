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

## 2. Подфазы

- **A** — склады (DS-001), остатки по складам (DS-002…005, DS-007 частично), миграция 0008.
- **B** — перемещения товаров (DS-006, DS-007).
- **C** — склад и перемещения оборудования заказа (DS-008, DS-009), seed-demo (DS-012).

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
