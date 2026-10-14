# Review Bundle: executor-payouts — READY
**Base:** 489a7414d92e01c4951b0647f5b31c37330b93b7 | **HEAD:** c17b338 | **Generated:** 2026-10-05 20:06 | diff: 65 files changed, 9716 insertions(+), 60 deletions(-)

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
| e2e_mode | ✅ | 0 | — |
| rule_usage | ✅ | 0 | — |

## Diff по линзам
| Линза | Файлов | Patch |
|---|---|---|
| backend | 28 | 8-code-review/diff-backend.patch |
| migrations | 2 | 8-code-review/diff-migrations.patch |
| other | 12 | 8-code-review/diff-other.patch |
| docs | 23 | — (артефакты, не ревьюятся построчно) |

### backend
- `M` internal/features/board/card_dto.go
- `M` internal/features/board/card_repo.go
- `M` internal/features/board/errors.go
- `A` internal/features/board/executor_payouts_http_live_test.go
- `A` internal/features/board/executor_payouts_test.go
- `A` internal/features/board/optional_amount.go
- `M` internal/features/board/order_card_http_live_test.go
- `M` internal/features/board/subtasks_repo.go
- `M` internal/features/board/subtasks_service.go
- `M` internal/features/board/subtasks_validate.go
- `M` internal/features/board/tasks_dto.go
- `M` internal/features/board/tasks_filter.go
- `M` internal/features/board/tasks_repo.go
- `M` internal/features/board/tasks_service.go
- `A` internal/features/executorpayouts/dto.go
- `A` internal/features/executorpayouts/executorpayouts_http_live_test.go
- `A` internal/features/executorpayouts/handler.go
- `A` internal/features/executorpayouts/migration_integration_test.go
- `A` internal/features/executorpayouts/repo.go
- `A` internal/features/executorpayouts/routes.go
- `A` internal/features/executorpayouts/service.go
- `A` internal/features/executorpayouts/ssr.go
- `A` internal/features/executorpayouts/validate.go
- `A` internal/features/executorpayouts/validate_test.go
- `M` internal/seeddemo/seeddemo.go
- `M` internal/seeddemo/seeddemo_integration_test.go
- `M` internal/server/router.go
- `M` internal/ssr/menu.go

### migrations
- `A` migrations/0011_executor_payouts.down.sql
- `A` migrations/0011_executor_payouts.up.sql

### other
- `M` e2e/fixtures/auth.ts
- `M` e2e/fixtures/board-types.ts
- `M` e2e/fixtures/board.ts
- `M` e2e/pages/BoardPage.ts
- `A` e2e/pages/ExecutorPayoutsPage.ts
- `M` e2e/pages/OrderCardPage.ts
- `A` e2e/tests/executor-payouts.spec.ts
- `M` web/ssr/_shared/js/board.js
- `A` web/ssr/_shared/js/executor-payouts.js
- `M` web/ssr/_shared/js/task-card.js
- `M` web/ssr/board/templates/pages/board.html
- `A` web/ssr/executorpayouts/templates/pages/list.html

### docs
- `A` docs/features/executor-payouts/0-setup/idioms.md
- `A` docs/features/executor-payouts/0-setup/light-progress.md
- `A` docs/features/executor-payouts/0-setup/pipeline-state.md
- `A` docs/features/executor-payouts/0-setup/timeline.jsonl
- `A` docs/features/executor-payouts/0-setup/timeline.md
- `A` docs/features/executor-payouts/11-report/output.md
- `A` docs/features/executor-payouts/8-code-review/checks/api_conventions.txt
- `A` docs/features/executor-payouts/8-code-review/checks/contract.txt
- `A` docs/features/executor-payouts/8-code-review/checks/date_bound.txt
- `A` docs/features/executor-payouts/8-code-review/checks/e2e_mode.txt
- `A` docs/features/executor-payouts/8-code-review/checks/fixture_sync.txt
- `A` docs/features/executor-payouts/8-code-review/checks/fuzz_verdict.txt
- `A` docs/features/executor-payouts/8-code-review/checks/go_naming.txt
- `A` docs/features/executor-payouts/8-code-review/checks/live_suite.txt
- `A` docs/features/executor-payouts/8-code-review/checks/rule_usage.txt
- `A` docs/features/executor-payouts/8-code-review/diff-backend.patch
- `A` docs/features/executor-payouts/8-code-review/diff-migrations.patch
- `A` docs/features/executor-payouts/8-code-review/diff-other.patch
- `A` docs/features/executor-payouts/8-code-review/input-bundle.md
- `A` docs/features/executor-payouts/8-code-review/output.md
- `A` docs/features/executor-payouts/9-validation/evidence/e2e-run.txt
- `A` docs/features/executor-payouts/9-validation/evidence/last-run.json
- `M` docs/plan.md

## Экстракты документов (вместо полного чтения)
### draft-spec.md
MISSING

### prd.md
MISSING

## Инвентарь доков (точечный grep разрешён, полное чтение — нет)
- 0-setup/idioms.md — 26KB
- 0-setup/light-progress.md — 9KB
- 8-code-review/output.md — 7KB
- 11-report/output.md — 6KB
- 8-code-review/input-bundle.md — 5KB
- 0-setup/timeline.md — 4KB
- 0-setup/pipeline-state.md — 0KB
- 9-validation/evidence/last-run.json — 0KB
