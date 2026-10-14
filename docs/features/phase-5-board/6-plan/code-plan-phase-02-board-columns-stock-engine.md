# Phase 02: Board — колонки + `StockEngine.Reconcile`

**Layer:** backend
**Commit**: `feat(phase-5): board — колонки CRUD и ядро StockEngine.Reconcile`

## Files to CREATE
- `internal/features/board/errors.go` — sentinel'ы `ErrTaskNotFound`, `ErrColumnNotFound`, `ErrSubtasksNotDone`, `ErrColumnNotEmpty`, `ErrLastNonFinalColumn`, `ErrTaskVersionConflict`, `ErrStockProductNotFound`, `ErrPaidAdminOnly` (design-errors.md §Sentinel Errors); внутренние `errNotFound`, `errVersionConflict`, `errColumnHasTasks`
- `internal/features/board/enums.go` — `SubtaskStatuses`, `PaidFilters`, `DueFilters`, `AssigneeMe`/`AssigneeAll`, `StockScopes`, `StockStates`, `WarningKinds`, `ParseXxx` (design-go-layers.md §Enum*)
- `internal/features/board/enums_test.go` — `TestSubtaskStatuses_MatchExpectedSet`, `TestPaidFilters_MatchExpectedSet`, `TestDueFilters_MatchExpectedSet`, `TestStockScopes_MatchExpectedSet`, `TestStockStates_MatchExpectedSet`, `TestWarningKinds_MatchExpectedSet` (статическая сверка списков; live-сверка с `pg_get_constraintdef` — фаза 03, `board_http_live_test.go`)
- `internal/features/board/tx.go` — `withTx(ctx, pool, fn)` — retry на `40P01`/`40001` ×3 (ADR-104)
- `internal/features/board/position.go` — Position Query + ребалансировка (design-sql.md §Position Query, ADR-001)
- `internal/features/board/columns_dto.go` — `ColumnResponse`, `ColumnListResponse`, `ColumnInput`, `ColumnMutationResponse` (design-go-layers.md §DTO Column)
- `internal/features/board/columns_validate.go` — `ColumnInput.normalize()`/`validate()`: `name` trim 1..60, `color` `^#[0-9A-Fa-f]{6}$`, `position` int ≥1 (design-errors.md §Validation Fields)
- `internal/features/board/columns_validate_test.go` — `TestColumnInputValidate_EmptyName`, `TestColumnInputValidate_NameTooLong`, `TestColumnInputValidate_InvalidColor`, `TestColumnInputValidate_ValidInput` (UT-002 колонки)
- `internal/features/board/columns_repo.go` — `ColumnRepo`: `List`, `Insert`, `Update`, `Renumber` (design-sql.md §Column Renumber Query), `Delete` (design-sql.md §Delete Column Query), `LockAll` (`ORDER BY id FOR UPDATE`), `TaskIDsForUpdate` (design-sql.md §Column Recalc Query)
- `internal/features/board/columns_service.go` — `ColumnService`: `List`, `Create` (в конец), `Update` (смена `consumes_stock` → цикл `Reconcile` по `TaskIDsForUpdate`, span `columns.recalc`), `Delete` (guard `ErrColumnNotEmpty`/`ErrLastNonFinalColumn`)
- `internal/features/board/columns_handler.go` — `List`, `Create`(201), `Update`(200), `Delete`(204 без тела); `c.Bind().Body(&in)` → `errorspkg.BadRequest` при ошибке
- `internal/features/board/stock_engine.go` — `StockEngine` + `Reconcile(ctx, tx, taskID, targetConsumes, userID) (ReconcileResult{Warnings []StockWarning; Movements int}, error)` (design-sql.md §Reconcile Query, ADR-104); `StockWarning` DTO (design-go-layers.md §DTO StockWarning) — поля `Stock`/`MinStock` типа `numeric.Amount`
- `internal/features/board/routes.go` — `Register(app, api, a *middleware.Auth, h *Handlers, s *SSR)` — СТАБ: только группа `/columns` (`Handlers{Columns *ColumnsHandler}`); `admin := middleware.RequireAdmin(nil)` per-route
- `internal/features/board/board_fuzz_test.go` — `FuzzColumnInputValidate` (design-tests.md §Fuzz Targets: `#GGGGGG`, `#fff`, position MinInt seeds)
- `internal/features/board/stock_engine_integration_test.go` — `TestStockEngine_Reconcile_ConsumeThenReturn`, `TestStockEngine_Reconcile_ToggleColumnConsumes`, `TestStockEngine_Reconcile_Idempotent_NoDoubleConsume` (BaseSuite, dockertest `postgres:18-alpine`; заказы/услуги — прямой SQL insert, `TaskService` появится в фазе 03); хелпер `assertStockInvariant(t, pool)` (design-tests.md §Stock Invariant)

## Files to MODIFY
_(нет — пакет `board` ещё не подключён к `internal/server/router.go`, DI — фаза 04)_

## Run after
```bash
make build
make test
```

## Tests in this phase
- Unit: `TestColumnInputValidate_EmptyName`, `TestColumnInputValidate_InvalidColor`, `TestColumnInputValidate_ValidInput`, `TestSubtaskStatuses_MatchExpectedSet` (и аналоги для остальных enum'ов)
- Fuzz (TD-7): `FuzzColumnInputValidate`
- Integration (`BaseSuite`, build tag `integration`): `TestStockEngine_Reconcile_ConsumeThenReturn`, `TestStockEngine_Reconcile_ToggleColumnConsumes`, `TestStockEngine_Reconcile_Idempotent_NoDoubleConsume`

## Definition of Done
- [x] `make build` — OK
- [x] `make test` — unit OK
- [x] `make test-integration-scoped` — OK (`stock_engine_integration_test.go`, скоуп §EE-5; `BASE=be769e5` расширил скоуп до `./...` — shared-schema change в 0005_board.up.sql, все пакеты зелёные)
- [x] `make lint` — 0 ошибок
- [x] Тесты этой фазы зелёные до перехода (TD-1)
- [x] `go test -run=^$ -fuzz=FuzzColumnInputValidate -fuzztime=10s ./internal/features/board/` — зелёный (TD-7, класс 9.1)
- [x] `StockWarning.Stock`/`MinStock` — `pkg/numeric.Amount` (НЕ `float64`/сырая строка); helper-binding (§Helper usage binding общих правил code-plan.md)
- [x] AC-006 покрыт (CRUD колонок, unit+integration); AC-007 ядро (`Reconcile` при toggle `consumes_stock`) покрыто `TestStockEngine_Reconcile_ToggleColumnConsumes` — сквозной API-тест (LV-008) — фаза 03 (combined live suite)
- [x] Инвариант склада (design-tests.md §Stock Invariant) — `assertStockInvariant` проходит после каждого шага `TestStockEngine_Reconcile_*`
- [x] Порядок блокировок ADR-104 соблюдён в `Reconcile`: `columns`→`tasks`→`products FOR UPDATE ORDER BY id`
- [ ] `withTx` — retry `40P01`/`40001` ×3, затем 500: код реализован (`tx.go`), НО искусственная гонка (реальный 40P01/40001) этой фазой не покрыта — зафиксировано как gap для фазы 03 combined live suite (LV-062), как и предусмотрено формулировкой пункта
