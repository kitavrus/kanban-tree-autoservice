# light-progress: soft-delete-core (часть 1)

## Backend
- [x] B1 миграция 0018 ALTER tasks (deleted_at/by/reason, CHECK, индекс) + down
- [x] B2 общие заготовки: internal/features/softdelete (reason.go, kinds.go), board.ErrNotDeleted
- [x] B3 board: мягкое удаление заказа (TaskService.Delete + reason), DS-034
- [x] B4 board: TaskService.Restore, DS-039 column_replaced, warnings
- [x] B5 board: TaskService.Purge + роут под Handlers.PurgeEnabled / config.TestPurgeEnabled
- [x] B6 читатели: tasks list/get/card/print, stock pending, executor-payouts, equipment history, users counter, position.go
- [x] B7 мутации частей заказа → 404 (LockTask/TaskColumnID/Get + logs repo + attachments repo + payouts)
- [x] B8 колонки: TaskIDsForUpdate / OpenSubtasksForColumn / tasks_count — только действующие
- [x] B9 softdelete: API + routes + ssr.go + router.go + menu.go (шаблон list.html и deleted.js — см. F3)
  (go build ./... и go vet -tags=integration ./... — чисто после B1–B9)
- [x] B10 seeddemo.seedDeletedTask + тест (TS-F5, только заказ — клиент это часть 4); TEST_PURGE_ENABLED в scripts/e2e/run.sh, .env.example (TS-F2)

## Frontend (SSR)
- [x] F1 ui-dialog.js: KT.uiDeleteDialog + KT.uiToast; api.js: KT.api.delWith
- [x] F2 task-card.js: «Удалить» → KT.uiDeleteDialog
- [x] F3 web/ssr/softdelete/templates/pages/list.html + web/ssr/_shared/js/deleted.js

## Тесты Go
- [x] TS-001 unit softdelete/reason_test.go (+ parseQuery, kinds) — НАПИСАНО и зелёное
- [x] НАПИСАНО (ещё не прогнано): board/soft_delete_http_live_test.go — TS-001,002,003,004,005,006,007,012,013,033,049,052 (методы OrderCardLiveSuite, ExtraEnv TEST_PURGE_ENABLED=true)
- [x] НАПИСАНО: TS-008 + TS-013(files) в attachments_gc_http_live_test.go; LV-210 переписан в attachments_http_live_test.go; purge-disabled в board_http_live_test.go
- [x] НАПИСАНО: softdelete/softdelete_http_live_test.go — TS-009, 010, 011, 033, SSR
- [x] TS-048 — в TestTS017_DeleteTaskWithTransfers (order_card_revert_http_live_test.go)
- [x] TS-F6 правка существующих live-тестов (board_http_live, attachments, order_card_revert, stock_engine_integration, seeddemo)
- [x] ПРОГНАНО зелёное (-tags=integration): softdelete, board, executorpayouts, users, clients, seeddemo, server, transfers; unit `go test ./... -count=1` — зелёное
- [x] config unit-тест TEST_PURGE_ENABLED (loadTestPurgeEnabled)

## E2E (только --list)
- [x] TS-F1 purgeTask (служебный адрес) + _boardCleanup (fixtures/auth.ts, fixtures/test.ts)
- [x] TS-F3 softDelete/restore/listDeleted/deletedSummary + wire-типы в fixtures/board-types.ts
- [x] page objects: pages/DeletedPage.ts (новый), BoardPage/OrderCardPage.confirmDeleteTask с причиной
- [x] e2e/tests/soft-delete-core.spec.ts — SD-001…SD-008 (TS-034,035,036,037,038,039,046,047)
- [x] transfers-revert.spec.ts TR-024 переписан под DS-034
- [x] tsc --noEmit чисто; playwright --list (soft-delete-core, transfers-revert, board) — 31 тест, компилируется

## Self-checks
- [x] go build ./... / go vet -tags=integration
- [x] go test ./... -count=1
- [x] integration: board, softdelete, executorpayouts, users, clients, seeddemo, server, transfers
- [x] playwright --list
- [x] финальный повтор: gofmt -l пусто, golangci-lint «0 issues», go vet -tags=integration чисто, unit зелёные, integration board+softdelete зелёные
- E2E НЕ прогонялись (зона оркестратора): soft-delete-core, transfers-revert, board + регресс остальных доска-спек
