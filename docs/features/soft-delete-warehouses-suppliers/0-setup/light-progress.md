# light-progress: soft-delete-warehouses-suppliers (часть 6)

## Backend
- [x] миграция 0023 (колонки, частичные индексы, CHECK, default_warehouse_id) + down
- [x] warehouses: repo/service/handler/routes/ssr — delete, restore, InUse (DS-044), фильтры «не удалён»
- [x] suppliers: repo/service/handler/routes — delete, restore, фильтры
- [x] softdelete: источники warehouses/suppliers, реестр, purge-виды, router.go
- [x] transfers: *_is_deleted поля, not_found при создании/правке (FOR SHARE), Restore (DS-045), Unreceive (DS-046), Send (DS-047)
- [x] board: CardWarehouse.is_deleted, subtask products warehouse not_found, фильтр доски, stock report (DS-049), Adjust (DS-048), summary
- [x] catalog: stocks без удалённых складов, not_found
- [x] equipment на склад / task_equipment — проверка «не удалён»
- [x] seeddemo + тест

## Frontend (SSR/JS)
- [x] warehouses: кнопка «Удалить» + форма data-delete-dialog
- [x] suppliers: кнопка «Удалить» в форме
- [x] deleted: вкладки «Склады», «Поставщики»
- [x] transfers: бейджи «удалён»
- [x] карточка заказа: бейдж у склада строки
- [x] stock: бейджи «в архиве»/«удалён», окно корректировки

## Тесты
- [x] TS-012 миграция (warehouses, suppliers)
- [x] TS-031, TS-057 склад (live) + гонки
- [x] TS-032 поставщик (live)
- [x] TS-033 права
- [x] TS-055, TS-065 остатки
- [x] TS-059, TS-060, TS-064 transfers
- [x] softdelete: виды warehouses/suppliers (TS-009/011), purge
- [x] E2E TS-045, TS-F3, TS-F11
- [x] существующие тесты приведены

## Self-check
- [x] go build / go vet
- [x] go test ./... -count=1
- [x] integration тронутых пакетов
- [x] tsc, playwright --list

## Итог
- unit: `go test ./... -count=1` — зелёный
- integration (`-p 1 -tags=integration`): warehouses, suppliers, transfers, board, catalog, softdelete, seeddemo — зелёные
- E2E: НЕ прогонялись (по условию) — только `tsc --noEmit` и `playwright --list`
