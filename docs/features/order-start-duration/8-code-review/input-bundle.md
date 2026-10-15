# Review Bundle: order-start-duration — CHECKS-FAILED
**Base:** 999e1f069f0429fe0436bc2ab590bf90546e8c8d | **HEAD:** 9367baa | **Generated:** 2026-10-15 19:50 | diff: 72 files changed, 6639 insertions(+), 462 deletions(-)

> Линзы ревью: читай ЭТОТ бандл + patch своей линзы. Полное чтение research/draft-spec/PRD
> и перегон чекеров ЗАПРЕЩЕНЫ (вердикты ниже; детали — checks/<name>.txt; точечный grep по DS-NNN разрешён).

## Чекеры (детерминированные — НЕ перегонять)
| Чекер | Вердикт | exit | Нарушения (первые строки) |
|---|---|---|---|
| go_naming | ✅ | 0 | — |
| api_conventions | ✅ | 0 | — |
| contract | ✅ | 0 | — |
| date_bound | ✅ | 0 | — |
| fuzz_verdict | ✅ | 0 | — |
| live_suite | ✅ | 0 | — |
| fixture_sync | ✅ | 0 | — |
| e2e_mode | ❌ | 1 | E2E-MODE FAIL HEADLESS-RUN: прогон [2026-10-15 16:36:12] без `--headed` — §TD-3 запрещает headless (команда: npx playwright test --project=chromium --workers=1 --retries=1 --reporter=list  # full suite, headless by user request (E2E_HEADLESS=1 make e2e-isolated-full, clean tmpfs stand))<br>E2E-MODE FAIL HEADLESS-RUN: прогон [2026-10-15 19:47:03] без `--headed` — §TD-3 запрещает headless (команда: npx playwright test --project=chromium --workers=1 --retries=1 --reporter=list  # full suite, headless by user request (E2E_HEADLESS=1 make e2e-isolated-full, clean tmpfs stand)) |
| rule_usage | ✅ | 0 | — |

## Diff по линзам
| Линза | Файлов | Patch |
|---|---|---|
| backend | 32 | 8-code-review/diff-backend.patch |
| migrations | 2 | 8-code-review/diff-migrations.patch |
| other | 18 | 8-code-review/diff-other.patch |
| docs | 20 | — (артефакты, не ревьюятся построчно) |

### backend
- `M` internal/features/board/board_fuzz_test.go
- `M` internal/features/board/board_http_live_test.go
- `M` internal/features/board/car_order_http_live_test.go
- `A` internal/features/board/migration_order_start_duration_integration_test.go
- `M` internal/features/board/one_car_helpers_live_test.go
- `M` internal/features/board/order_card_http_live_test.go
- `M` internal/features/board/print_http_live_test.go
- `M` internal/features/board/soft_delete_http_live_test.go
- `M` internal/features/board/stock_engine_integration_test.go
- `M` internal/features/board/tasks_car_validate_test.go
- `M` internal/features/board/tasks_dto.go
- `M` internal/features/board/tasks_repo.go
- `M` internal/features/board/tasks_service.go
- `M` internal/features/board/tasks_validate.go
- `M` internal/features/board/tasks_validate_test.go
- `M` internal/features/board/timezone_http_live_test.go
- `M` internal/features/calendar/calendar_http_live_test.go
- `M` internal/features/calendar/dto.go
- `M` internal/features/calendar/gantt.go
- `M` internal/features/calendar/gantt_test.go
- `M` internal/features/calendar/repo.go
- `M` internal/features/calendar/service.go
- `M` internal/features/calendar/service_test.go
- `M` internal/features/calendar/ssr.go
- `M` internal/features/clients/clients_soft_delete_http_live_test.go
- `M` internal/features/softdelete/softdelete_http_live_test.go
- `M` internal/seeddemo/data_catalog.go
- `M` internal/seeddemo/data_people.go
- `M` internal/seeddemo/data_tasks.go
- `A` internal/seeddemo/plan_test.go
- `M` internal/seeddemo/seeddemo.go
- `M` internal/seeddemo/seeddemo_integration_test.go

### migrations
- `A` migrations/0018_order_start_duration.down.sql
- `A` migrations/0018_order_start_duration.up.sql

### other
- `M` e2e/fixtures/auth.ts
- `M` e2e/fixtures/board-types.ts
- `M` e2e/pages/BoardPage.ts
- `M` e2e/pages/NewOrderDialog.ts
- `M` e2e/pages/OrderCardPage.ts
- `M` e2e/tests/board.spec.ts
- `M` e2e/tests/calendar-gantt.spec.ts
- `M` e2e/tests/calendar-new-order.spec.ts
- `M` e2e/tests/new-order-capacity.spec.ts
- `M` e2e/tests/order-card.spec.ts
- `A` e2e/tests/order-plan.spec.ts
- `M` e2e/tests/user-timezone.spec.ts
- `M` web/ssr/_layout/templates/partials/task-modal.html
- `M` web/ssr/_shared/app.css
- `M` web/ssr/_shared/js/board.js
- `M` web/ssr/_shared/js/calendar.js
- `M` web/ssr/_shared/js/task-card.js
- `M` web/ssr/calendar/templates/pages/calendar.html

### docs
- `A` docs/features/order-start-duration/0-setup/idioms.md
- `A` docs/features/order-start-duration/0-setup/light-progress.md
- `A` docs/features/order-start-duration/0-setup/pipeline-state.md
- `A` docs/features/order-start-duration/0-setup/timeline.jsonl
- `A` docs/features/order-start-duration/8-code-review/checks/api_conventions.txt
- `A` docs/features/order-start-duration/8-code-review/checks/contract.txt
- `A` docs/features/order-start-duration/8-code-review/checks/date_bound.txt
- `A` docs/features/order-start-duration/8-code-review/checks/e2e_mode.txt
- `A` docs/features/order-start-duration/8-code-review/checks/fixture_sync.txt
- `A` docs/features/order-start-duration/8-code-review/checks/fuzz_verdict.txt
- `A` docs/features/order-start-duration/8-code-review/checks/go_naming.txt
- `A` docs/features/order-start-duration/8-code-review/checks/live_suite.txt
- `A` docs/features/order-start-duration/8-code-review/checks/rule_usage.txt
- `A` docs/features/order-start-duration/8-code-review/diff-backend.patch
- `A` docs/features/order-start-duration/8-code-review/diff-migrations.patch
- `A` docs/features/order-start-duration/8-code-review/diff-other.patch
- `A` docs/features/order-start-duration/8-code-review/input-bundle.md
- `A` docs/features/order-start-duration/8-code-review/output.md
- `A` docs/features/order-start-duration/9-validation/evidence/e2e-run.txt
- `A` docs/features/order-start-duration/9-validation/evidence/last-run.json

## Экстракты документов (вместо полного чтения)
### draft-spec.md
MISSING

### prd.md
MISSING

## Инвентарь доков (точечный grep разрешён, полное чтение — нет)
- 8-code-review/output.md — 10KB
- 0-setup/light-progress.md — 7KB
- 0-setup/idioms.md — 6KB
- 8-code-review/input-bundle.md — 5KB
- 0-setup/pipeline-state.md — 1KB
- 9-validation/evidence/last-run.json — 0KB
