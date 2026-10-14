# Phase 01: Миграция 0006, коды 16-22, lock-retry (ADR-100/103)

**Layer:** backend
**Commit**: `feat(phase-6): migration 0006 subtask_logs + position, errors 16-22, lock-retry`

## Files to CREATE
- `migrations/0006_subtask_logs.up.sql` — design-sql.md §Subtask Logs Migration: таблица `subtask_logs` (RESTRICT `user_id`, CHECK текста 1..4000, индексы `subtask_logs_subtask_created_idx`, `subtask_logs_user_id_idx`) + `ALTER TABLE subtask_products ADD COLUMN position` + backfill `row_number() OVER (PARTITION BY subtask_id ORDER BY product_id, id)` (ADR-103)
- `migrations/0006_subtask_logs.down.sql` — design-sql.md §Subtask Logs Migration (DROP COLUMN position, DROP TABLE subtask_logs) — CM-1 MVP, без rollback-шага
- `internal/features/board/subtask_products_position_integration_test.go` — `TestSubtaskProductsPosition_BackfillOnMigrate` (IT-020): down до 0005 → вставка 3 `subtask_products` одной услуги без `position` → up → `position` = 1..3, без нулей/дублей

## Files to MODIFY
- `internal/features/board/errors.go` — добавить блок «фаза 6» (design-errors.md §Sentinel Errors): `ErrSubtaskNotFound`, `ErrSubtaskProductNotFound`, `ErrSubtaskLogNotFound`, `ErrTaskEquipmentNotFound`, `ErrTaskEquipmentInUse`, `ErrSubtaskFinalColumn`, `ErrSubtaskLogForbidden` (Codes 16-22) + непубличный `errTaskMovedRetry` (ADR-100, наружу не течёт)
- `internal/features/board/tx.go` — `withTx` ретраит `errTaskMovedRetry` наравне с 40P01/40001 (×3, design-sql.md §Lock Order)

## Run after
```bash
make build
make lint
make migrate-up
make migrate-down   # затем снова migrate-up — round-trip 0006
```

## Tests in this phase
- Integration: `TestSubtaskProductsPosition_BackfillOnMigrate` (IT-020)

## Definition of Done
- [x] `make build` — OK
- [x] `make lint` — 0 ошибок
- [x] `make migrate-up` / `make migrate-down` / `make migrate-up` — round-trip без ошибок на 0006 (проверено на одноразовом контейнере, dev-стенд не трогался)
- [x] `BASE=9580596 make test-integration-scoped` — OK (скоуп-дефолт §EE-5; миграция затрагивает `board` — при FULL-триггере весь `./...`)
- [x] IT-020 зелёный (position 1..3, без нулей — ADR-103)
- [x] Тесты этой фазы зелёные до перехода (TD-1, fix-before-next)
- [x] Коды 16-22 — sentinel-ы объявлены `var Err... = errorspkg.New(...)` на пакете (эталон `errors.go:15-22` as-is), `.WithDetail` не мутирует sentinel
- [x] `errTaskMovedRetry` — непубличный `errors.New`, не экспортируется, не попадает в HTTP-ответ (design-errors.md §Error Mapping: `errTaskMovedRetry`, 40P01, 40001 → ретрай ×3 → далее 500 `INTERNAL`)
