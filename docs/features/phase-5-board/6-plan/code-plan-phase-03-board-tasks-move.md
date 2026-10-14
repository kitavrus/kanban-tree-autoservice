# Phase 03: Board — заказы, move, комбинированная live-сьюта

**Layer:** backend
**Commit**: `feat(phase-5): board — заказы, перенос между колонками, live-сьюта`

## Files to CREATE
- `internal/features/board/tasks_dto.go` — `TaskCard`, `TaskListResponse`, `TaskResponse`, `TaskCreateRequest`, `TaskUpdateRequest`, `TaskMoveRequest`, `TaskMutationResponse`, `TaskDeleteResponse` (design-go-layers.md §DTO Task)
- `internal/features/board/tasks_validate.go` — `TaskCreateRequest.normalize()`/`validate()`, `TaskUpdateRequest` (+`version` обязателен) — правила design-errors.md §Validation Fields (`title`, `client_id`, `column_id`, `assignee_id`, `due_date`, `version`)
- `internal/features/board/tasks_filter.go` — `TaskFilter` + `ParseTaskFilter(c fiber.Ctx, a Actor) (TaskFilter, error)` (design-go-layers.md §DTO TaskFilter; `q`→`#N`/`N` regex `^#?\d{1,18}$`; `due`/`paid`/`assignee` enum парсинг)
- `internal/features/board/tasks_repo.go` — `TaskRepo`: `LockTargetColumn`, `LockTask`, `OpenSubtasks` (design-sql.md §Open Subtasks Query), `NextPosition` (делегирует `position.go`), `Move` (design-sql.md §Move Task Query), `Update` (§Update Task Query), `Insert` (§Insert Task Query), `Delete` (§Delete Task Query), `List` (§Board List Query, `q`-экранирование через `pkg/sqlutil`), `Get`
- `internal/features/board/tasks_service.go` — `TaskService`: `List`, `Get`, `Create`, `Update`, `Move`, `Delete` — алгоритм Move (design-go-layers.md §Service Interface, шаги 1–7); `ErrSubtasksNotDone.WithDetail("subtasks", list)` (ADR-102 — использовать `pkg/errorspkg.WithDetail`, НЕ отдельный тип ответа); span'ы `tasks.move`/`columns.recalc` (Monitoring Spec)
- `internal/features/board/tasks_handler.go` — `List` (`ParseTaskFilter` с `Actor` из `middleware.CurrentPrincipal(c)`), `Get`, `Create`(201), `Update`, `Move`, `Delete`(200 + `TaskDeleteResponse{warnings}`)
- `internal/features/board/assignees_handler.go` — `GET /api/v1/assignees` (design-sql.md §Assignees Query; только `is_active`, `limit` default 200 max 1000)
- `internal/features/board/tasks_validate_test.go` — `TestTaskCreateRequestValidate_EmptyTitle`, `TestTaskCreateRequestValidate_TitleTooLong`, `TestTaskCreateRequestValidate_MissingClientID`, `TestTaskUpdateRequestValidate_MissingVersion` (UT-002 заказы)
- `internal/features/board/tasks_filter_test.go` — `TestParseTaskFilter_HashNumber`, `TestParseTaskFilter_PlainNumber`, `TestParseTaskFilter_InvalidNumber`, `TestParseTaskFilter_DefaultsByRole`, `TestParseTaskFilter_EnumValidation` (UT-001)
- `internal/features/board/board_http_live_test.go` (`BoardLiveSuite`, `testutil.LiveSuite`, реальный бинарь+TCP, реальный login §TD-8) — методы по ID design-tests.md: `TestBoardLiveSuite_Columns_SeedPresent` (LV-001), `TestBoardLiveSuite_CreateTask` (LV-002), `TestBoardLiveSuite_UpdateTask_VersionIncrements` (LV-003), `TestBoardLiveSuite_Move_BetweenColumns` (LV-004), `TestBoardLiveSuite_Move_WithinColumn_PrevNext` (LV-005), `TestBoardLiveSuite_Move_ConsumesStock` (LV-006), `TestBoardLiveSuite_Move_NegativeStockWarning` (LV-007), `TestBoardLiveSuite_ColumnToggleConsumes_RecalcAll` (LV-008), `TestBoardLiveSuite_ColumnsCRUD` (LV-009), `TestBoardLiveSuite_DeleteTask_ReturnsStock` (LV-010), `TestBoardLiveSuite_AdminTogglesPaid` (LV-011), `TestBoardLiveSuite_Filters_Combination` (LV-020), `TestBoardLiveSuite_Filters_AssigneeDefaultByRole` (LV-021), `TestBoardLiveSuite_Filters_LimitOffsetTotal` (LV-022), `TestBoardLiveSuite_Auth_NoSession401` (LV-030), `TestBoardLiveSuite_Auth_NoCSRF403` (LV-031), `TestBoardLiveSuite_GarbageBody400` (LV-032), `TestBoardLiveSuite_TaskValidation400` (LV-033), `TestBoardLiveSuite_ColumnValidation400` (LV-034), `TestBoardLiveSuite_NotFound404` (LV-035), `TestBoardLiveSuite_Move_ForeignNeighborIgnored` (LV-036), `TestBoardLiveSuite_QueryEnumValidation400` (LV-037), `TestBoardLiveSuite_Executor_ColumnsForbidden` (LV-040), `TestBoardLiveSuite_Executor_DeleteTaskForbidden` (LV-041), `TestBoardLiveSuite_Executor_PaidAdminOnly` (LV-042), `TestBoardLiveSuite_Executor_TaskWriteAllowed` (LV-044), `TestBoardLiveSuite_Move_ToFinal_SubtasksNotDone` (LV-050), `TestBoardLiveSuite_Update_ToFinal_SubtasksNotDone` (LV-051), `TestBoardLiveSuite_Move_ToFinal_AllDone` (LV-052), `TestBoardLiveSuite_DeleteColumn_NotEmpty` (LV-053), `TestBoardLiveSuite_DeleteColumn_LastNonFinal` (LV-054), `TestBoardLiveSuite_Update_StaleVersion` (LV-055), `TestBoardLiveSuite_Race_ConcurrentMoveDifferentColumns` (LV-060, `-race`), `TestBoardLiveSuite_Race_FormVsMove` (LV-061), `TestBoardLiveSuite_Race_ToggleVsMove` (LV-062), `TestBoardLiveSuite_Race_ConcurrentMoveSharedProduct` (LV-063), `TestBoardLiveSuite_Race_PositionRebalance50Inserts` (LV-064), `TestBoardLiveSuite_Race_DeleteClientVsCreateTask` (LV-065), `TestBoardLiveSuite_Enums_SubtaskStatusMatchesCheckConstraint` (live-сверка `pg_get_constraintdef`, дополняет `enums_test.go` фазы 02)
- `internal/features/board/board_fuzz_test.go` — MODIFY: добавить `FuzzParseTaskFilter`, `FuzzTaskInputValidate`, `FuzzMoveBody` (`app.Test` — допустимое исключение §9.1)

## Files to MODIFY
- `internal/features/board/routes.go` — расширить `Register`: группа `/tasks` (List/Create/Get/Update/Move/Delete), `GET /assignees`; `Handlers` — добавить поля `Tasks *TasksHandler`, `Assignees *AssigneesHandler`
- `internal/features/board/stock_engine_integration_test.go` — MODIFY: расширить `IT-010` таблицу шагов до полного набора (consume, return, повторный consume, toggle колонки, delete заказа, manual adjust — manual adjust появится в фазе 04, здесь — заготовка без него) с использованием реального `TaskRepo`/`TaskService` вместо прямых SQL-вставок фазы 02

## Run after
```bash
make build
make test
make test-integration-scoped
```

## Tests in this phase
- Unit: `TestTaskCreateRequestValidate_EmptyTitle`, `TestParseTaskFilter_HashNumber`, `TestParseTaskFilter_DefaultsByRole`
- Fuzz (TD-7): `FuzzParseTaskFilter`, `FuzzTaskInputValidate`, `FuzzMoveBody`
- Live (`BoardLiveSuite`, TD-8): полный список выше (LV-001…LV-065, кроме LV-012/013/014/043 — фаза 04; кроме LV-043 stock)

## Definition of Done
- [x] `make build` — OK
- [x] `make test` — unit OK
- [x] `make test-integration-scoped` — OK (BASE=be769e5, FULL-триггер из-за ALTER TABLE stock_movements в 0005 — весь ./..., `-race`)
- [x] `make lint` — 0 ошибок (golangci-lint: 0 issues)
- [x] Тесты этой фазы зелёные до перехода (TD-1)
- [x] `board_http_live_test.go` (`testutil.LiveSuite`, реальный бинарь+TCP, реальный login) — все перечисленные LV-* зелёные (TD-8)
- [x] `go test -run=^$ -fuzz=FuzzParseTaskFilter -fuzztime=10s ./internal/features/board/` и аналогично для `FuzzTaskInputValidate`, `FuzzMoveBody` — зелёные (TD-7, класс 9.1); `FuzzMoveBody` использует `app.Test` — единственное разрешённое исключение (§9.1)
- [x] `ErrSubtasksNotDone.WithDetail("subtasks", list)` — используется `pkg/errorspkg.WithDetail` (фаза 01); отдельный ad-hoc тип ответа для move НЕ создан (helper-binding)
- [x] Инвариант склада (`assertStockInvariant`) вызван в конце LV-006…LV-010, LV-060…LV-063
- [x] AC-002, AC-003, AC-004, AC-005, AC-007 (сквозной, LV-008), AC-009, AC-010, AC-011, AC-012, AC-013, AC-014 — покрыты backend-уровнем (live)
- [x] `TaskCard.Position` — `string` (NUMERIC как есть, не Amount — только для отладки/сортировки клиента, design-go-layers.md §DTO Task)
- [x] `tasks.id` в пути — `strconv.ParseInt`>0 до SQL; uuid-пути — `uuid.Validate` (design-errors.md)

## Отчёт (фактические итоги гейта, 2026-09-27)
- `make build` — OK.
- `make test` — все пакеты `ok` (board 0.63s).
- `make lint` — `golangci-lint run ./...` → `0 issues.`
- `BASE=be769e5 make test-integration-scoped` — FULL-триггер (ALTER TABLE stock_movements
  в `migrations/0005_board.up.sql`) расширил скоуп до `./...`; все пакеты `ok`, включая
  `internal/features/board` (55.5s, `-race`, LiveSuite+StockEngineSuite+PositionSuite).
- Фазовый прогон отдельно: `go test -tags=integration ./internal/features/board/... -race`
  → `ok` (38.6s), включает 38 методов `BoardLiveSuite` (LV-001…LV-065, кроме LV-012/013/
  014/043 — фаза 04) + `stock_engine_integration_test.go` (включая новый `TestIT010_
  FullSequence_ViaTaskServiceAndColumnService`) + `position_integration_test.go`.
- Fuzz: `FuzzParseTaskFilter` (10s, 603700 execs, PASS), `FuzzTaskInputValidate` (10s,
  498409 execs, PASS), `FuzzMoveBody` (10s/15s wall, 116520 execs, PASS) — все три зелёные,
  без паник, без 5xx.
- **Design-решение (отклонение от буквального текста design-go-layers.md §Service
  Interface):** `TaskService` в `tasks_handler.go` — узкий package-private ИНТЕРФЕЙС
  (не поле `*TaskService` напрямую, как `ColumnService` в фазе 02), чтобы `FuzzMoveBody`
  могло подставить фейковую реализацию без БД (§9.1 — `app.Test`-исключение обязано
  работать без dockertest). Конкретный `tasks_service.go::TaskService` (экспортируемый
  concrete struct, как и `ColumnService`) удовлетворяет интерфейсу структурно, без явной
  декларации — production-код (routes.go/router.go) собирает конкретный тип как обычно.
- Обнаружены и исправлены 4 бага собственных LiveSuite-тестов (не production-кода) в
  процессе гейта: (1) `TestBoardLiveSuite_ColumnToggleConsumes_RecalcAll` ошибочно ожидал
  списание сразу при Create, хотя subtask_products добавлялись SQL уже ПОСЛЕ Create-транзак
  ции (Reconcile при создании — no-op по нулевому спросу, единообразие); (2)
  `TestBoardLiveSuite_DeleteTask_ReturnsStock` — та же причина, добавлен явный `move` в
  consumes-колонку перед DELETE; (3) DTO `stockWarningDTO.Stock/MinStock` были типизированы
  `string`, а `numeric.Amount` на wire — JSON-число без кавычек → `json.Number`; (4)
  `TestBoardLiveSuite_Race_PositionRebalance50Inserts` вставлял между ФИКСИРОВАННой парой
  соседей 50 раз подряд — `Move Task Query` не имеет UNIQUE на `position`, поэтому мидпоинт
  `(1024+2048)/2=1536` буквально повторялся; исправлено на прогрессивную вставку сразу
  после `t1` (каждый раз новый «правый» сосед = только что вставленная карточка) — реально
  бисекция до триггера ребалансировки. `TestBoardLiveSuite_Race_FormVsMove` (LV-061)
  переписан на детерминированный (не двумя реальными горутинами) сценарий: `Move Task
  Query` НЕ проверяет `version` (LWW «порядок ≠ правка заказа» — так в design-sql.md), т.е.
  при реальной гонке PUT/Move возможен результат [200,200] (PUT выигрывает лок первым, Move
  всё равно применяется поверх) — это корректное поведение по дизайну, а не баг; тест
  переписан как «move сначала (200, version+1) → PUT со старым захваченным version (409)»,
  что детерминированно проверяет тот же инвариант оптимистичной блокировки. Также все
  `Race_*`-тесts переведены с `NewBrowser()+Login()` на подряд НА КАЖДУЮ горутину на общий
  `s.admin` (тот же `*http.Client`/`cookiejar`, безопасны для конкурентного использования) —
  массовый повторный логин упирался в rate-limit неудачных попыток входа auth-сервиса.
- Роуты board (`/api/v1/columns`, `/tasks`, `/assignees`) смонтированы в
  `internal/server/router.go` — необходимо для `LiveSuite` уже в этой фазе; SSR (`/`,
  `/stock`) и снятие `home` с корня НЕ трогались (по инструкции — фаза 04).
