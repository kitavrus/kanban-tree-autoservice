# Phase 05: `make seed-demo` — пакет `internal/seeddemo`

**Layer:** backend
**Commit**: `feat(phase-5): make seed-demo — демо-данные прототипа одной командой`

## Files to CREATE
- `internal/seeddemo/seeddemo.go` — `Run(ctx context.Context, pool *pgxpool.Pool) (Result, error)` (ADR-105): guard `count(tasks)=0` (design-sql.md §Seed Demo Guard Query) — иначе сообщение + `nil error` (exit 0 в `cmd`); нужен активный admin (`role='admin' AND is_active`) — иначе `error` (exit 1 «make local-up-seed»); демо-исполнители `ON CONFLICT (login) DO NOTHING`; товары со стартовым stock + журнал `manual`; заказы через `board.StockEngine.Reconcile` (экспортирован, ADR-100/104); всё в одной транзакции
- `internal/seeddemo/seeddemo_integration_test.go` — `TestSeedDemo_Run_EmptyDB_CreatesFullDemoSet` (IT-001: клиенты, оборудование, каталог, заказы во всех колонках с услугами и товарами), `TestSeedDemo_Run_TasksExist_NoOpWithMessage`, `TestSeedDemo_Run_NoAdmin_ReturnsError`
- `cmd/seed-demo/main.go` — тонкая обёртка: читает `DATABASE_URL` (эталон `cmd/seed/main.go`) → `seeddemo.Run(ctx, pool)` → печать результата/ошибки, `os.Exit` по коду

## Files to MODIFY
- `Makefile` — добавить цель `seed-demo:` ВНЕ блока `project-bootstrap` (`. scripts/dev/env.sh && go run ./cmd/seed-demo`, эталон существующей `seed:`-цели)

## Run after
```bash
make build
make test-integration-scoped
```

## Tests in this phase
- Integration (`BaseSuite`, dockertest, build tag `integration`): `TestSeedDemo_Run_EmptyDB_CreatesFullDemoSet` (IT-001), `TestSeedDemo_Run_TasksExist_NoOpWithMessage`, `TestSeedDemo_Run_NoAdmin_ReturnsError`

## Definition of Done
- [x] `make build` — OK
- [x] `make test-integration-scoped` — OK (`internal/seeddemo/seeddemo_integration_test.go`)
- [x] `make lint` — 0 ошибок
- [x] Тесты этой фазы зелёные до перехода (TD-1)
- [x] AC-019 покрыт: `TestSeedDemo_Run_EmptyDB_CreatesFullDemoSet` — заказы во всех колонках с услугами и товарами; повтор → no-op «заказы уже есть»; без активного admin → ошибка «make local-up-seed» (E2E-уровень для AC-019 не создаётся — design-tests.md подтверждает: «не E2E: команда + БД», проверка Runbook-smoke — за оркестратором)
- [x] `seeddemo.Run` использует `board.StockEngine.Reconcile` (не дублирует Reconcile-логику копипастой)
- [x] Демо-исполнители — `ON CONFLICT (login) DO NOTHING`; существующий admin не изменяется
- [x] `make seed-demo` — цель Makefile существует и вне блока `project-bootstrap` (ручная проверка на локальной БД — Runbook, шаг 1)
