# Phase 03: Услуги, товары, журнал работ, фикстуры E2E→API

**Layer:** backend
**Commit**: `feat(phase-6): subtasks/subtask_products CRUD + Reconcile, журнал работ, фикстуры API`

## Files to CREATE
- `internal/features/board/subtasks_repo.go` — `SubtaskRepo`: `TaskIDOf`, `Lock`, `Insert` (+копия комплекта, design-sql.md §Insert Subtask Query), `ServiceState`, `Update` (§Update Subtask Query, `equipment_id` проверка из `task_equipment`), `Delete` (§Delete Subtask Query, CASCADE products+logs)
- `internal/features/board/subtasks_service.go` — методы `CardService`: `CreateSubtask` (409 `SUBTASK_FINAL_COLUMN` если `column.is_final`), `UpdateSubtask` (409 `SUBTASK_FINAL_COLUMN` при `done`→не-`done` в финальной; без Reconcile, но под теми же блокировками ADR-100), `DeleteSubtask` (Reconcile после каскадного удаления товаров)
- `internal/features/board/subtasks_validate.go` — `SubtaskCreateRequest`/`SubtaskUpdateRequest` `normalize()/validate()` (design-errors.md §Validation Fields: `status` enum, `assignee_id` активный при смене — SD-007, `due_date` YYYY-MM-DD, `price` ≥0 Scale 10/2, `equipment_id` из `task_equipment` заказа)
- `internal/features/board/subtask_products_repo.go` — `SubtaskProductRepo`: `ParentOf`, `Lock`, `ProductState`, `Insert` (design-sql.md §Subtask Product Queries), `Update` (LWW), `Delete`
- `internal/features/board/subtask_products_service.go` — методы `CardService`: `CreateSubtaskProduct`, `UpdateSubtaskProduct`, `DeleteSubtaskProduct` (каждая — `engine.Reconcile(tx, taskID, col.ConsumesStock, a.ID)` после записи)
- `internal/features/board/subtask_products_validate.go` — `SubtaskProductCreateRequest`/`UpdateRequest` `normalize()/validate()` (`qty` >0 Scale 9/3, `price` ≥0 или null Scale 10/2)
- `internal/features/board/subtask_logs_repo.go` — `SubtaskLogRepo{pool}`: `Insert` (23503 → `errNotFound`), `Owner`, `Update`, `Delete` (design-sql.md §Subtask Log Queries)
- `internal/features/board/subtask_logs_service.go` — методы `CardService`: `CreateLog`, `UpdateLog`, `DeleteLog` (403 `SUBTASK_LOG_FORBIDDEN` — автор ≠ актор и актор не admin, D27; без блокировки заказа)
- `internal/features/board/subtask_logs_validate.go` — `SubtaskLogRequest.normalize()/validate()` (`text` trim 1..4000)

## Files to MODIFY
- `internal/features/board/card_service.go` — довключить поля `subtasks`/`products`/`logs`-репозитории в `CardService`; `lockRowsScope`/`cardMutation` используются всеми новыми методами (design-go-layers.md §Service Interface: «Поток мутации товара/услуги»)
- `internal/features/board/card_handler.go` — добавить методы `CreateSubtask`, `UpdateSubtask`, `DeleteSubtask`, `CreateSubtaskProduct`, `UpdateSubtaskProduct`, `DeleteSubtaskProduct`, `CreateLog`, `UpdateLog`, `DeleteLog`
- `internal/features/board/routes.go` — группы `subtasks`, `subtask-products`, `subtask-logs` (design-go-layers.md §Routes: `t.Post("/:id/subtasks", ...)`, `sub.Put/Delete("/:id", ...)`, `sub.Post("/:id/products"|"/:id/logs", ...)`, `sp.Put/Delete("/:id", ...)`, `sl.Put/Delete("/:id", ...)`)
- `internal/features/board/card_dto.go` — довключить `normalize()/validate()` методы на Row Requests, объявленных в фазе 02 (типы уже есть, поведение — здесь), если не покрыто `*_validate.go` файлами
- `internal/features/board/card_validate_test.go` — MODIFY: добавить `SubtaskUpdateRequest`, `SubtaskProductCreateRequest`/`UpdateRequest`, `SubtaskLogRequest` кейсы (полный UT-002: границы qty/price Scale, text trim/4000, status enum, nil vs value)
- `internal/features/board/card_integration_test.go` — MODIFY: расширить `CardSuite` до полного покрытия: `TestCardSuite_Reconcile_ConsumesStock` (LV-120…123), `TestCardSuite_StockInvariant` (IT-001), `TestCardSuite_Concurrency_UpdateSubtaskProduct` (IT-010), `TestCardSuite_Concurrency_MoveVsUpdate` (IT-011), `TestCardSuite_Concurrency_DeleteVsUpdate` (IT-012), `TestCardSuite_Concurrency_TaskMovedRetry` (IT-013)
- `internal/features/board/order_card_http_live_test.go` — MODIFY: добавить `TestOrderCardLiveSuite_Subtask_CreateWithKit` (LV-103), `TestOrderCardLiveSuite_Subtask_UpdateFields` (LV-104), `TestOrderCardLiveSuite_SubtaskProduct_CRUD` (LV-105), `TestOrderCardLiveSuite_Totals` (LV-106), `TestOrderCardLiveSuite_Logs_CRUD` (LV-107), `TestOrderCardLiveSuite_Version_UnaffectedByRowMutations` (LV-130), `TestOrderCardLiveSuite_Version_ConflictOnHeader` (LV-131), `TestOrderCardLiveSuite_FinalColumn_BlocksCreate` (LV-140), `TestOrderCardLiveSuite_FinalColumn_BlocksUndone` (LV-141), `TestOrderCardLiveSuite_Subtask_EquipmentNotInOrder400` (LV-145), `TestOrderCardLiveSuite_Subtask_InactiveAssignee400` (LV-146), `TestOrderCardLiveSuite_Subtask_ArchivedServiceOrProduct400` (LV-147), `TestOrderCardLiveSuite_DeleteSubtask_CascadesLogsProducts` (LV-148), `TestOrderCardLiveSuite_Access_ExecutorRowEndpoints` (LV-160), `TestOrderCardLiveSuite_Logs_ForbiddenForOthers` (LV-163), `TestOrderCardLiveSuite_NoSession401` (LV-164), `TestOrderCardLiveSuite_InvalidPathIDs404` (LV-170, дополнить subtasks/products/logs), `TestOrderCardLiveSuite_InvalidInput400` (LV-171), `TestOrderCardLiveSuite_GarbageBody400` (LV-172), `TestOrderCardLiveSuite_IdempotentDelete404` (LV-173), `TestOrderCardLiveSuite_LogOnDeletedSubtask404` (LV-174)
- `internal/features/board/board_http_live_test.go` — MODIFY: добавить `TestBoardLiveSuite_Move_ToFinal_WithAPISubtask` (LV-150, регресс фазы 5: услуга создана через новое API, а не `pg`)
- `internal/features/board/board_fuzz_test.go` — MODIFY: добавить `FuzzSubtaskUpdateBody` (FZ-001, `PUT /subtasks/:id`), `FuzzSubtaskProductBody` (FZ-002, `POST/PUT` товара), `FuzzSubtaskLogAndEquipmentBody` (FZ-003, `POST /subtasks/:id/logs`, `POST /tasks/:id/equipment`) — seeds design-tests.md §Fuzzing, `app.Test` допустим (§9.1)
- `e2e/fixtures/board.ts` — MODIFY: заменить `pg`-вставки на API-хелперы (ADR-101): `addSubtask(taskId, {serviceId, status?, dueDate?, products?})` (`POST /tasks/:id/subtasks` → при `products` DELETE скопированных + `POST /subtasks/:id/products`; при `status`/`dueDate` — `PUT /subtasks/:id`), `attachEquipment(taskId, equipmentId)` (`POST /tasks/:id/equipment`); удалить импорт `pg` из `e2e/package.json`, если других использований нет
- `e2e/fixtures/board-types.ts` — MODIFY: добавить типы `OrderCard`, `CardSubtask`, `CardSubtaskProduct`, `CardEquipment`, `CardSubtaskLog` (зеркало DTO фазы 02/03)
- `e2e/tests/board.spec.ts`, `e2e/tests/board-guards.spec.ts`, `e2e/tests/stock.spec.ts` — MODIFY при необходимости под новые сигнатуры фикстур (регресс-гейт ADR-101); без изменения сценариев, если фикстуры совместимы по интерфейсу

## Run after
```bash
make build
make test
make test-integration-scoped
```

## Tests in this phase
- Unit: `card_validate_test.go` полный набор UT-002 (qty/price Scale, text trim, status enum)
- Fuzz (TD-7, класс 9.1): `FuzzSubtaskUpdateBody`, `FuzzSubtaskProductBody`, `FuzzSubtaskLogAndEquipmentBody`
- Integration (`CardSuite`): IT-001, IT-010, IT-011, IT-012, IT-013
- Live (`OrderCardLiveSuite`): LV-103…107, LV-130/131, LV-140/141/145…148, LV-160/163/164, LV-170…174
- Live regression (`BoardLiveSuite`): LV-150
- E2E regression: `board.spec.ts board-guards.spec.ts stock.spec.ts --project=chromium --headed --workers=1` зелёные после перевода фикстур на API

## Definition of Done
- [x] `make build` — OK
- [x] `make test` — unit OK
- [x] `make test-integration-scoped` — OK (BASE=9580596 → FULL-триггер миграцией 0006, полный `./...` с `-race`, все пакеты зелёные)
- [x] `make lint` — 0 ошибок
- [x] Тесты этой фазы зелёные до перехода (TD-1)
- [x] `order_card_http_live_test.go` — все перечисленные LV-* зелёные (TD-8); `board_http_live_test.go` LV-150 зелёный
- [x] `go test -run=^$ -fuzz=FuzzSubtaskUpdateBody -fuzztime=10s ./internal/features/board/` и аналогично для `FuzzSubtaskProductBody`, `FuzzSubtaskLogAndEquipmentBody` — зелёные, без паник, только 400/404/409/2xx (TD-7, класс 9.1)
- [x] Инвариант склада (IT-001: `stock_after − stock_before = Σ qty_delta`, `task_stock.qty = Σ subtask_products.qty` для consumes) — выполнен после LV-120…123 и IT-010…013
- [x] AC-007, AC-008, AC-009 (подсказка остатка использует `CardRepo.Stock` фазы 02 + реальные товары этой фазы), AC-010, AC-011, AC-012, AC-013 (полный — с новыми услугами/товарами), AC-014 (конкурентность, ADR-100/errTaskMovedRetry) — покрыты
- [x] AC-017 (права исполнителя на row-эндпоинтах, `SUBTASK_LOG_FORBIDDEN`) — LV-160/163/164 зелёные
- [x] `e2e/fixtures/board.ts` не импортирует `pg` (ADR-101, `pg`/`@types/pg` удалены из `e2e/package.json`); `board.spec.ts`/`board-guards.spec.ts`/`stock.spec.ts` зелёные на новых фикстурах — реальный прогон Playwright дважды подряд после рестарта сервера: **21/21 passed** оба раза (50.8s, 51.2s)
- [x] `Reconcile` вызывается ПОСЛЕ commit-эквивалентной точки внутри той же `withTx` для create/delete услуги и create/update/delete товара; `UpdateSubtask` — без Reconcile, но под `lockRowsScope` (helper-binding: НЕ дублировать блокировку вручную, использовать `lockRowsScope` фазы 02)
- [x] testdata/фикстуры (`e2e/fixtures/board.ts`, seed использующийся в LiveSuite) приведены к новой схеме (`position`, `subtask_logs`) — TD-1/TD-6.2

## Побочные находки (исправлены в этой фазе)
- **Баг форматирования денег**: `coalesce(NULL, 0)` в `subtask_total`/`total_amount`/`task_reserved` терял NUMERIC-scale (печатался `"0"` вместо `"0.00"`/`"0.000"`) для «по факту»-услуг без товаров — обнаружен новыми live-тестами (LV-104), исправлен оборачиванием в `round(...,2)`/`round(...,3)` в `card_repo.go`/`tasks_repo.go`.
- **TearDownTest CardSuite**: `stock_movements.user_id` (`ON DELETE RESTRICT`) блокировал очистку тестового пользователя после Reconcile-тестов — добавлен `DELETE FROM stock_movements WHERE user_id=$1` перед `DELETE FROM users`.
