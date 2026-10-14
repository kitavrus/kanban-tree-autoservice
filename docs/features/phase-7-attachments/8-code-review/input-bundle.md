# Review Bundle: phase-7-attachments — READY
**Base:** e74edc5 | **HEAD:** 9ea9f09 | **Generated:** 2026-09-27 17:29 | diff: 49 files changed, 5918 insertions(+), 19 deletions(-)

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
| backend | 17 | 8-code-review/diff-backend.patch |
| e2e | 4 | 8-code-review/diff-e2e.patch |
| migrations | 2 | 8-code-review/diff-migrations.patch |
| other | 5 | 8-code-review/diff-other.patch |
| docs | 21 | — (артефакты, не ревьюятся построчно) |

### backend
- `M` internal/config/config.go
- `A` internal/features/board/attachments_handler.go
- `A` internal/features/board/attachments_http_live_test.go
- `A` internal/features/board/attachments_repo.go
- `A` internal/features/board/attachments_service.go
- `A` internal/features/board/attachments_validate.go
- `A` internal/features/board/attachments_validate_test.go
- `M` internal/features/board/card_dto.go
- `M` internal/features/board/card_service.go
- `M` internal/features/board/errors.go
- `M` internal/features/board/routes.go
- `M` internal/features/board/subtask_products_position_integration_test.go
- `M` internal/features/board/tasks_service.go
- `A` internal/filestore/filestore.go
- `A` internal/filestore/filestore_test.go
- `A` internal/server/middleware/body_limit.go
- `M` internal/server/router.go

### e2e
- `M` e2e/fixtures/auth.ts
- `M` e2e/fixtures/board-types.ts
- `M` e2e/pages/OrderCardPage.ts
- `A` e2e/tests/order-attachments.spec.ts

### migrations
- `A` migrations/0007_task_attachments.down.sql
- `A` migrations/0007_task_attachments.up.sql

### other
- `M` .env.example
- `M` web/ssr/_shared/app.css
- `M` web/ssr/_shared/js/task-card.js
- `M` web/ssr/_shared/js/ui-dialog.js
- `M` web/ssr/board/templates/pages/board.html

### docs
- `A` docs/features/phase-7-attachments/0-setup/idioms.md
- `A` docs/features/phase-7-attachments/0-setup/light-progress.md
- `A` docs/features/phase-7-attachments/0-setup/pipeline-state.md
- `A` docs/features/phase-7-attachments/0-setup/timeline.jsonl
- `A` docs/features/phase-7-attachments/8-code-review/checks/api_conventions.txt
- `A` docs/features/phase-7-attachments/8-code-review/checks/contract.txt
- `A` docs/features/phase-7-attachments/8-code-review/checks/date_bound.txt
- `A` docs/features/phase-7-attachments/8-code-review/checks/e2e_mode.txt
- `A` docs/features/phase-7-attachments/8-code-review/checks/fixture_sync.txt
- `A` docs/features/phase-7-attachments/8-code-review/checks/fuzz_verdict.txt
- `A` docs/features/phase-7-attachments/8-code-review/checks/go_naming.txt
- `A` docs/features/phase-7-attachments/8-code-review/checks/live_suite.txt
- `A` docs/features/phase-7-attachments/8-code-review/checks/rule_usage.txt
- `A` docs/features/phase-7-attachments/8-code-review/diff-backend.patch
- `A` docs/features/phase-7-attachments/8-code-review/diff-e2e.patch
- `A` docs/features/phase-7-attachments/8-code-review/diff-migrations.patch
- `A` docs/features/phase-7-attachments/8-code-review/diff-other.patch
- `A` docs/features/phase-7-attachments/8-code-review/input-bundle.md
- `A` docs/features/phase-7-attachments/8-code-review/output.md
- `A` docs/features/phase-7-attachments/9-validation/evidence/e2e-run.txt
- `A` docs/features/phase-7-attachments/9-validation/evidence/last-run.json

## Экстракты документов (вместо полного чтения)
### draft-spec.md
MISSING

### prd.md
MISSING

## Инвентарь доков (точечный grep разрешён, полное чтение — нет)
- 0-setup/idioms.md — 21KB
- 8-code-review/output.md — 11KB
- 0-setup/light-progress.md — 10KB
- 8-code-review/input-bundle.md — 4KB
- 0-setup/pipeline-state.md — 0KB
- 9-validation/evidence/last-run.json — 0KB
