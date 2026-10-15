# Review Bundle: one-car-order — READY
**Base:** cd5ab2947ecf51c7e8fb61763d441a818e18ebee | **HEAD:** 65f3c01 | **Generated:** 2026-10-15 15:03 | diff: 92 files changed, 8564 insertions(+), 2142 deletions(-)

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
| backend | 50 | 8-code-review/diff-backend.patch |
| migrations | 2 | 8-code-review/diff-migrations.patch |
| other | 20 | 8-code-review/diff-other.patch |
| docs | 20 | — (артефакты, не ревьюятся построчно) |

### backend
- `M` internal/features/board/attachments_gc_integration_test.go
- `M` internal/features/board/attachments_http_live_test.go
- `M` internal/features/board/board_fuzz_test.go
- `M` internal/features/board/board_http_live_test.go
- `M` internal/features/board/car_order_http_live_test.go
- `M` internal/features/board/card_dto.go
- `M` internal/features/board/card_handler.go
- `M` internal/features/board/card_integration_test.go
- `M` internal/features/board/card_repo.go
- `M` internal/features/board/card_service.go
- `M` internal/features/board/card_validate_test.go
- `M` internal/features/board/columns_soft_delete_http_live_test.go
- `M` internal/features/board/errors.go
- `A` internal/features/board/migration_one_car_order_integration_test.go
- `A` internal/features/board/one_car_helpers_live_test.go
- `M` internal/features/board/order_card_http_live_test.go
- `M` internal/features/board/pagination_http_live_test.go
- `M` internal/features/board/position_integration_test.go
- `M` internal/features/board/print_http_live_test.go
- `M` internal/features/board/routes.go
- `M` internal/features/board/soft_delete_catalog_http_live_test.go
- `M` internal/features/board/soft_delete_http_live_test.go
- `M` internal/features/board/stock_engine_integration_test.go
- `M` internal/features/board/subtask_products_position_integration_test.go
- `M` internal/features/board/subtasks_repo.go
- `M` internal/features/board/subtasks_service.go
- `M` internal/features/board/subtasks_validate.go
- `D` internal/features/board/task_equipment_repo.go
- `D` internal/features/board/task_equipment_service.go
- `D` internal/features/board/task_equipment_validate.go
- `M` internal/features/board/tasks_car_validate_test.go
- `M` internal/features/board/tasks_dto.go
- `M` internal/features/board/tasks_repo.go
- `M` internal/features/board/tasks_service.go
- `M` internal/features/board/tasks_validate.go
- `M` internal/features/board/tasks_validate_test.go
- `M` internal/features/board/timezone_http_live_test.go
- `M` internal/features/calendar/calendar_http_live_test.go
- `M` internal/features/catalog/catalog_soft_delete_http_live_test.go
- `M` internal/features/catalog/services_http_live_test.go
- `M` internal/features/clients/clients_http_live_test.go
- `M` internal/features/clients/clients_soft_delete_http_live_test.go
- `M` internal/features/clients/history.go
- `M` internal/features/executorpayouts/executorpayouts_http_live_test.go
- `M` internal/features/softdelete/softdelete_http_live_test.go
- `M` internal/features/users/users_http_live_test.go
- `M` internal/seeddemo/data_tasks.go
- `M` internal/seeddemo/seeddemo.go
- `M` internal/seeddemo/seeddemo_integration_test.go
- `M` internal/server/router.go

### migrations
- `A` migrations/0017_one_car_order.down.sql
- `A` migrations/0017_one_car_order.up.sql

### other
- `M` e2e/fixtures/auth.ts
- `M` e2e/fixtures/board-types.ts
- `M` e2e/fixtures/board.ts
- `M` e2e/pages/OrderCardPage.ts
- `M` e2e/tests/board.spec.ts
- `M` e2e/tests/calendar-new-order.spec.ts
- `M` e2e/tests/new-order-autofill.spec.ts
- `M` e2e/tests/new-order-plate.spec.ts
- `M` e2e/tests/order-card.spec.ts
- `M` e2e/tests/order-print.spec.ts
- `M` e2e/tests/polish.spec.ts
- `M` e2e/tests/soft-delete-clients.spec.ts
- `M` e2e/tests/user-timezone.spec.ts
- `M` web/ssr/_layout/templates/partials/task-modal.html
- `M` web/ssr/_shared/app.css
- `D` web/ssr/_shared/js/equipment-history.js
- `M` web/ssr/_shared/js/task-card.js
- `M` web/ssr/board/templates/pages/print-handover.html
- `M` web/ssr/board/templates/pages/print-intake.html
- `M` web/ssr/clients/templates/pages/equipment-history.html

### docs
- `A` docs/features/one-car-order/0-setup/idioms.md
- `A` docs/features/one-car-order/0-setup/light-progress.md
- `A` docs/features/one-car-order/0-setup/pipeline-state.md
- `A` docs/features/one-car-order/0-setup/timeline.jsonl
- `A` docs/features/one-car-order/8-code-review/checks/api_conventions.txt
- `A` docs/features/one-car-order/8-code-review/checks/contract.txt
- `A` docs/features/one-car-order/8-code-review/checks/date_bound.txt
- `A` docs/features/one-car-order/8-code-review/checks/e2e_mode.txt
- `A` docs/features/one-car-order/8-code-review/checks/fixture_sync.txt
- `A` docs/features/one-car-order/8-code-review/checks/fuzz_verdict.txt
- `A` docs/features/one-car-order/8-code-review/checks/go_naming.txt
- `A` docs/features/one-car-order/8-code-review/checks/live_suite.txt
- `A` docs/features/one-car-order/8-code-review/checks/rule_usage.txt
- `A` docs/features/one-car-order/8-code-review/diff-backend.patch
- `A` docs/features/one-car-order/8-code-review/diff-migrations.patch
- `A` docs/features/one-car-order/8-code-review/diff-other.patch
- `A` docs/features/one-car-order/8-code-review/input-bundle.md
- `A` docs/features/one-car-order/8-code-review/output.md
- `A` docs/features/one-car-order/9-validation/evidence/e2e-run.txt
- `A` docs/features/one-car-order/9-validation/evidence/last-run.json

## Экстракты документов (вместо полного чтения)
### draft-spec.md
MISSING

### prd.md
MISSING

## Инвентарь доков (точечный grep разрешён, полное чтение — нет)
- 0-setup/idioms.md — 30KB
- 8-code-review/output.md — 17KB
- 0-setup/light-progress.md — 8KB
- 8-code-review/input-bundle.md — 6KB
- 0-setup/pipeline-state.md — 2KB
- 9-validation/evidence/last-run.json — 0KB
