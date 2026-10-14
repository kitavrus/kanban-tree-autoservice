# light-progress: soft-delete-clients (часть 4)

После сжатия истории — перечитать §11 среза спеки и этот чеклист.

## Backend
- [x] миграция 0021 (clients + equipment: колонки, CHECK, индексы)
- [x] clients: repo/service/handler/routes — мягкое удаление, восстановление, purge, фильтры deleted_at
- [x] clients: ssr.go (reason, 404 удалённых, тосты, красные блоки)
- [x] clients: history.go / equipment_search.go — фильтр удалённых
- [x] board: client_is_deleted (TaskCard), is_deleted (CardEquipment), проверка клиента FOR KEY SHARE, единица not_found, notices restore
- [x] transfers: client_is_deleted, equipment_is_deleted, занятие единицы deleted_at IS NULL, restore → TRANSFER_HAS_DELETED_REFS
- [x] softdelete: ClientsSource, EquipmentSource, регистрация + purge в router.go
- [x] seeddemo + тест (integration ok)

## Frontend (SSR/JS)
- [x] шаблоны клиента/единицы: окно «Удаление», скрытое поле reason, красный блок
- [x] deleted.js: вкладки «Клиенты», «Оборудование», бейдж «вместе с клиентом», восстановление
- [x] board.js / task-card.js / transfers.js: бейджи «удалён»

## Тесты
- [x] unit (softdelete/clients_source_test.go)
- [x] (clients пакет зелёный: clients_soft_delete_http_live_test.go + migration_soft_delete_integration_test.go) integration LiveSuite: TS-012, 022, 023, 024, 025, 026, 033, 053, 058 + гонки
- [x] существующие live-тесты: 204→200, снятые запреты (TS-F6)
- [x] E2E: soft-delete-clients.spec.ts (TS-043), DeletedPage, auth.ts purge (TS-F3, TS-F9), clients.spec/equipment.spec (TS-F4)

## Self-check
- [x] go build / go vet
- [x] go test ./... -count=1
- [x] integration тронутых пакетов
- [x] e2e tsc + --list

## Заметки (после сжатия)
- Сделано: миграция, clients/soft_delete.go (repo+service), handler/routes/ssr, board+transfers пометки и проверки,
  softdelete/clients_source.go (+unit), router.go (источники + purge), JS/шаблоны (ui-dialog.js: form[data-delete-dialog], [data-kt-toast]).
- Осталось: seeddemo(+тест), правка существующих live-тестов (clients, board, transfers, softdelete summary), новые live-тесты
  clients_soft_delete_http_live_test.go, миграционный тест TS-012, E2E (auth.ts purge/softDelete, DeletedPage, clients.spec, equipment.spec, новая спека).
- Отступления: POST /subtasks не принимает equipment_id (as-is) — проверка только в PUT; вкладки — порядок реестра Kinds
  (… Колонки, Клиенты, Оборудование); удаление единицы — SELECT FOR NO KEY UPDATE + UPDATE (не один UPDATE) ради точного кода отказа.
- Прогресс 2: clients integration зелёный. Дальше: прогнать и поправить board / transfers / softdelete / server integration
  (204→200, снятые запреты, summary counts), затем E2E (auth.ts, DeletedPage.ts, clients.spec.ts, equipment.spec.ts, EquipmentPage.ts,
  ClientsPage.ts, новая soft-delete-clients.spec.ts), потом финальные self-check'и по одному вызову Bash.
- ИТОГ: всё сделано. go build/vet ok; go test ./... ok; integration clients/board/transfers/softdelete/server/seeddemo ok;
  e2e tsc ok, --list 26 тестов в 4 файлах. E2E не прогонялись (по условию).
