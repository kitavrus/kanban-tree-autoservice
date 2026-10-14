# light-progress — warehouses

## Подфаза A

- [x] Разведка: где используется products.stock/min_stock/box/place, task_stock, stock_movements, StockEngine
- [x] Миграция 0008_warehouses up/down (warehouses + сид, product_stock + перенос, drop products.stock*, subtask_products.warehouse_id, task_stock PK, stock_movements.warehouse_id/transfer_id/reason CHECK)
- [x] Пакет internal/features/warehouses (repo/service/handler, API /api/v1/warehouses GET/POST/PUT)
- [x] SSR экран «Склады» + пункт сайдбара (admin)
- [x] catalog: product_stock в карточке товара (блок «Склады»), ручная правка остатка по складу
- [x] Экран «Остатки»: фильтр склада, колонки складов, Итого, В пути(0 до B), KPI/бейдж по парам
- [x] board: subtask_products.warehouse_id, склад по умолчанию, смена склада, StockEngine по паре
- [x] seed-demo: остатки на «Алматы»
- [x] docs/plan.md D45+
- [x] Unit-тесты
- [x] Integration LiveSuite: warehouses, catalog, board, seeddemo, clients — зелёные
- [x] E2E: warehouses.spec.ts + правки stock/catalog/board
- [x] TS-001 миграция
- [x] TS-002 склады CRUD
- [x] TS-003 ручная правка Москва
- [x] TS-004 списание по складу
- [x] TS-005 смена склада у списанной строки
- [x] TS-006 склад по умолчанию (ветка «первый активный»; склад оборудования — подфаза C)
- [x] TS-007 экран Остатки
- [x] TS-014 executor не видит Склады
- [x] Self-checks: go build, go vet, go test ./..., integration пакетов, playwright --list

> Решения по ходу: API товара — `stocks[]`/`total_stock` вместо stock/min_stock/box/place (D46); /api/v1/warehouses целиком admin; последний активный склад не архивируется (D47).

> Итог self-check (2026-10-02): go build/vet (+tags=integration) OK; gofmt чисто; `go test ./... -count=1` — все ok; integration warehouses/catalog/board/seeddemo/clients — ok; GO-1/API-1 чекеры PASS; `npx tsc --noEmit` OK; `playwright --list` — 68 тестов в 6 специях. E2E-прогон — за оркестратором.
> Открыто: TS-006 ветка «склад оборудования услуги» — в подфазе C (нужен task_equipment.warehouse_id, DS-008).

## Подфаза B

- [x] Разведка: stock_repo «В пути», журнал движений (в UI не выводится — ссылки нет), паттерн warehouses
- [x] Миграция 0009_stock_transfers up/down (stock_transfers, stock_transfer_items, FK stock_movements.transfer_id SET NULL)
- [x] Пакет internal/features/stocktransfers (dto/validate/repo/service/handler/routes/ssr), API /api/v1/stock-transfers
- [x] «В пути» на экране «Остатки» (board/stock_repo.go transit CTE)
- [x] Router + пункт меню «Перемещения» (admin)
- [x] SSR-шаблон + js/stock-transfers.js (master-detail, модалки приёмки/отмены)
- [x] Unit-тесты validate
- [x] Integration LiveSuite: stocktransfers (TS-008/009/010/014, В пути), migration 0009
- [x] E2E: stock-transfers.spec.ts + POM + фикстуры adminApi
- [x] docs/plan.md D50+
- [x] Self-checks
- [x] TS-008 заявка → правка → отправка → приёмка с недостачей
- [x] TS-009 отмены, финальные статусы 409, from = to → 400
- [x] TS-010 устаревший version 409, двойной/параллельный receive
- [x] TS-007 «В пути» (все склады / фильтр склада-получателя)
- [x] TS-014 executor → 403 на перемещения, нет пункта меню

> Решения: D50–D54 в docs/plan.md (пакет stocktransfers, API/409-коды, дубли строк → 400, приёмка/журнал, экран).
> Итог self-check (2026-10-02): go build/vet (+tags=integration) OK; gofmt чисто; `go test ./... -count=1` — все ok; integration stocktransfers/board/warehouses/server — ok; GO-1/API-1 чекеры PASS; node --check JS OK; `npx tsc --noEmit` OK; `playwright --list` stock-transfers+warehouses+stock — 13 тестов. E2E-прогон — за оркестратором.
> Журнал движений в UI не выводится — ссылка «из журнала на перемещение» не делалась (D54).

## Подфаза C

- [x] Разведка: task_equipment repo/service/handler, права, карточка заказа (JS/SSR), tasks_filter.go, history.go, seeddemo
- [x] Миграция 0010_task_equipment_warehouses up/down
- [x] Backend: склад приёма при добавлении + received; send/receive/cancel-transit (FOR UPDATE, 409/400); журнал
- [x] DS-005: SubtaskProductRepo.DefaultWarehouse — склад оборудования услуги
- [x] DS-009: фильтр доски «Склад» + бейдж «в пути»
- [x] История оборудования клиента — склад/статус (если ложится)
- [x] SSR/JS карточки заказа: select склада, бейдж, действия, модалка, история
- [x] seed-demo DS-012
- [x] docs/plan.md D55+
- [x] Unit-тесты
- [x] Integration LiveSuite
- [x] E2E: equipment-warehouses.spec.ts + правки существующих
- [x] Self-checks
- [x] TS-006 склад по умолчанию — склад оборудования услуги (в пути → первый активный)
- [x] TS-011 добавить со складом приёма → отправить → принять, история 3 записи
- [x] TS-012 отмена отправки, 409 на неверный переход, 400 на тот же склад
- [x] TS-013 фильтр доски «Склад» + бейдж «в пути»
- [x] TS-014 executor отправляет/принимает оборудование

> Решения: D55–D60 в docs/plan.md (колонка transit_from, API/409-коды, склад по умолчанию, фильтр доски SSR-опциями, seed-demo).
> Итог self-check (2026-10-02): go build/vet (+tags=integration) OK; gofmt чисто; node --check OK; `go test ./... -count=1` — все ok; integration board/seeddemo/clients/stocktransfers/server — ok; GO-1/API-1 чекеры PASS; `npx tsc --noEmit` OK; `playwright --list` 5 специй — 62 теста. E2E-прогон — за оркестратором.
