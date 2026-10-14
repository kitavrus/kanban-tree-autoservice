# light-progress: soft-delete-transfers (часть 2)

## Backend
- [x] миграция 0019 (transfers: deleted_at/deleted_by/delete_reason, CHECK, индекс) + down
- [x] transfers: errors (TASK_DELETED, TRANSFER_CLIENT_MISMATCH), DTO task_is_deleted, DeleteRequest {reason, version}
- [x] transfers: repo — deleted_at IS NULL во всех чтениях; SoftDelete; Restore; Purge; checkRefs TASK_DELETED
- [x] transfers: service Delete/Restore/Purge, handler, routes (AdminOnly)
- [x] softdelete: TransfersSource + Register в router
- [x] board: снять TASK_HAS_TRANSFERS при удалении заказа; читатели (card, tasks total, stock in-transit) без удалённых; смена клиента считает только действующие; Purge tasks убирает мягко удалённые документы; purge kind transfers
- [x] clients: history без удалённых документов
- [x] seeddemo / прочие читатели transfers (seeddemo не трогаем; warehouses.InUse — вне спеки, в отчёт)

## Frontend (SSR)
- [x] transfers.js: удаление через KT.uiDeleteDialog + delWith {reason, version}; бейдж «заказ удалён»; TASK_DELETED тост
- [x] deleted.js + list.html: вкладка «Перемещения», restore
- [x] task-card.js: убрать ветку TASK_HAS_TRANSFERS, вторая строка пояснения

## Тесты
- [x] TS-012 миграция (integration)
- [x] TS-014 удаление заказа с документами
- [x] TS-015 документ удалённого заказа + TASK_DELETED
- [x] TS-016 удаление документа
- [x] TS-017 восстановление заявки
- [x] TS-018 гонка создание↔удаление заказа
- [x] TS-033 права
- [x] TS-050 смена клиента / TRANSFER_CLIENT_MISMATCH
- [x] TS-051 служебная уборка
- [x] softdelete live: вид transfers в списке и summary
- [x] unit: валидатор DeleteRequest
- [x] TS-F6 существующие Go live-тесты приведены
- [x] TS-F3/F7 e2e/fixtures/auth.ts
- [x] TS-F4 существующие e2e-спеки приведены
- [x] TS-040, TS-041 E2E (новая спека)

## Self-check
- [x] go build / vet
- [x] go test ./... -count=1
- [x] integration: transfers, board, softdelete, clients
- [x] e2e: tsc --noEmit, playwright --list

## Заметки
- integration transfers/board/softdelete — зелёные (первый прогон); clients ещё прогнать
- warehouses.InUse считает мягко удалённую заявку (не в §Impact) — в отчёт
- equipment delete: строки удалённого документа держат единицу (EQUIPMENT_HAS_TRANSFERS) — тест TS018 доски обновлён, в отчёт
- Все пункты закрыты. Self-check: go build/vet, go test ./... (unit), integration transfers/board/softdelete/clients — ok; e2e tsc — чисто, --list — 39 тестов в 6 файлах. E2E не прогонялись (правило задачи).
