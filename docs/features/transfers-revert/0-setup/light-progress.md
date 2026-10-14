# light-progress: transfers-revert

## Шаги
- [x] 1a. Миграция 0017 (unreceived_by/at/reason + CHECK) — файлы up/down написаны
- [x] 1b. миграционный тест 0017 — migration_revert_integration_test.go написан (не прогнан)
- [x] 10a. unit validate_revert_test.go (TestTS010_ValidateReason, TestTS016_ParseVersion, TestTS010_TS016_StatusConflictMessages, TestTS012_UnreceiveBlockedError) — go test transfers OK; validate_test.go и transfers_equipment_http_live_test.go — новый текст TS-009
- [x] 2. transfers: DTO/getTransfer/List — поля отката (в списке — только unreceived_at для значка)
- [x] 3. transfers: repo.Unreceive (+ unsettleEquipment), validateReason, сервис, handler, route
- [x] 4. transfers: repo.Delete, сервис, handler, route
- [x] 5. transfers: текст 409 cancel полученного (TS-009) — код; тест ещё поправить
- [x] 6. board: DELETE task → TASK_HAS_TRANSFERS + каскад отменённых (deleteTaskTransfers) — код; тест equipment_location_http_live_test.go:291 поправить
- [x] 7. UI transfers.js / list.html: кнопки st-unreceive / st-delete, модалка #st-unreceive-modal, строка st-card-unreceived, значок st-row-unreceived (node --check OK)
- [x] 8. UI task-card.js: showTransfersError (ссылки task-error-transfer-link в #task-error), строка про отменённые в подтверждении
- [x] 9. seeddemo: полученный приход поставщика стал «откатан и принят заново» (supplier_in +20, transfer_unreceive −20, supplier_in +18), cancelledReturn у demoReturnTask; seeddemo_integration_test поправлен (8 сочетаний, net журнала) — прогон integration ещё НЕ делал
- [x] 10. Go-тесты unit (валидатор причины)
- [x] 11a. transfers live (transfers_revert_http_live_test.go): TS-010 ×5, TS-011, TS-012, TS-016, хвосты TS-009/TS-015 — `go test ./internal/features/transfers/... -tags=integration` OK (вкл. миграционный 0017)
- [x] 11b. board live (order_card_revert_http_live_test.go): TestTS013_UnreceiveInvoiceOfPaidOrder, TestTS017_DeleteTaskWithTransfers (старый TestDeleteTask_WithTransfers409 удалён), TestTS018_DeleteRequestFreesEquipment — board integration OK
- [x] 11c. seeddemo integration OK
- [x] 11d. clients / warehouses integration OK (регресс)
- [x] 12a. E2E фикстуры: auth.ts (типы Transfer.unreceived_*, unreceiveTransfer, deleteTransfer, removeTransferForCleanup, purgeTask), test.ts (_boardCleanup → purgeTask, 409 больше не глотается), board-types.ts (ApiErrorTaskHasTransfers) — tsc OK
- [x] 12b. E2E page objects (TransfersPage, OrderCardPage) + спека tests/transfers-revert.spec.ts: TR-022, TR-023 (TS-033), TR-024 (TS-037), TR-025 (TS-038), TR-026 (исполнитель) — tsc OK, `--list` OK (E2E фактически НЕ прогонялись — зона оркестратора)
- [x] 13. Self-check'и: go build OK; go test ./... OK; integration transfers / board / seeddemo / clients / warehouses OK; node --check transfers.js, task-card.js OK; e2e tsc OK; playwright --list OK (43 теста в 5 файлах). НЕ проверено: фактический прогон E2E, UI в браузере.

- [x] 14. Fix-loop 1: live TestTS012_UnreceiveBlocked_EquipmentNotAtDestination и TestTS012_ParallelUnreceiveVsNewDocument (-race OK, оба исхода); stockWarnings(receivedOnly) — без строк с фактом 0; docs/plan.md D51 + D73–D79; OC-001 / BD-009 регистрируют заказ на уборку; ограничитель уборки (TransferCleanupGuard); aria-labelledby у модалки. Self-check'и OK, E2E не прогонялись.

## Решения
- Р1. Отменённые документы при удалении заказа удаляет сам board SQL-запросом в транзакции удаления (`deleteTaskTransfers` в board/tasks_service.go) — board уже читает таблицу transfers SQL'ом; импорта пакета transfers нет (нет цикла).
- Р2. Отступление от §3 п.4: строки transfers при удалении заказа НЕ берутся FOR UPDATE — иначе порядок «заказ → документ» против «документ → заказ (FOR KEY SHARE)» в transfers.Update даёт deadlock. Гонок нет: создание документа ждёт LockTask, cancelled терминален, страховка FK RESTRICT.
- Р3. unreceive/DELETE документа для исполнителя — всегда 403 (проверка роли первой, до поиска документа; существование не раскрывается), в т.ч. чужое направление.
- Р4. received_at в doReceive: now() → clock_timestamp() — порядок приёмок по общей единице = порядок взятия блокировки её строки (иначе «более поздний» документ с ранним стартом транзакции обходил бы DS-023).
- Р5. 409 TRANSFER_UNRECEIVE_BLOCKED называет САМЫЙ поздний полученный документ с единицей (с него начинают откат). Единица не на месте без мешающего документа (местоположение правили вне перемещений) → тот же код без details.transfer_id, текст «Оборудование находится не там, куда его привезло это перемещение».
- Р7. Диалог причины — отдельная Bootstrap-модалка в list.html (как модалка приёмки), а не KT.uiDialog: uiDialog закрывается по OK и не умеет ошибку под полем / блокировку кнопки на время запроса / ссылку в диалоге. «Удалить» — KT.uiDialog.
- Р8. Отказ удаления заказа (TASK_HAS_TRANSFERS) — в шапке карточки (#task-error, as-built конвенция карточки), ссылки на документы; не тост.
- Р6. Причина отката: кроме required/too_long — invalid_text (NUL), как у note.
