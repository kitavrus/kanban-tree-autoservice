# Review Bundle: phase-2-auth — READY
**Base:** b7f600b | **HEAD:** ca186d5 | **Generated:** 2026-09-25 13:50 | diff: 66 files changed, 8056 insertions(+), 367 deletions(-)

> Линзы ревью: читай ЭТОТ бандл + patch своей линзы. Полное чтение research/draft-spec/PRD
> и перегон чекеров ЗАПРЕЩЕНЫ (вердикты ниже; детали — checks/<name>.txt; точечный grep по DS-NNN разрешён).

## Чекеры (детерминированные — НЕ перегонять)
| Чекер | Вердикт | exit | Нарушения (первые строки) |
|---|---|---|---|
| go_naming | ✅ | 0 | — |
| api_conventions | ✅ | 0 | — |
| contract | ⚠️ SKIP-RESOLVE (Go-код в diff есть — НЕ зелёно, §GO-1: перегенерируй bundle с --paths/--go) | 0 | — |
| date_bound | ✅ | 0 | — |
| fuzz_verdict | ✅ | 0 | — |
| live_suite | ✅ | 0 | — |
| fixture_sync | ✅ | 0 | — |
| e2e_mode | ✅ | 0 | — |
| rule_usage | ✅ | 0 | — |

## Diff по линзам
| Линза | Файлов | Patch |
|---|---|---|
| backend | 31 | 8-code-review/diff-backend.patch |
| migrations | 2 | 8-code-review/diff-migrations.patch |
| other | 14 | 8-code-review/diff-other.patch |
| docs | 19 | — (артефакты, не ревьюятся построчно) |

### backend
- `M` cmd/seed/main.go
- `M` go.mod
- `M` internal/config/auth.go
- `A` internal/features/auth/auth_http_live_test.go
- `M` internal/features/auth/dto.go
- `M` internal/features/auth/handler.go
- `A` internal/features/auth/handler_test.go
- `M` internal/features/auth/repo.go
- `M` internal/features/auth/routes.go
- `M` internal/features/auth/service.go
- `M` internal/features/home/handler.go
- `M` internal/features/users/dto.go
- `M` internal/features/users/handler.go
- `M` internal/features/users/repo.go
- `M` internal/features/users/routes.go
- `M` internal/features/users/service.go
- `A` internal/features/users/ssr.go
- `A` internal/features/users/users_http_live_test.go
- `A` internal/features/users/validate.go
- `A` internal/features/users/validate_test.go
- `A` internal/server/json.go
- `M` internal/server/middleware/auth.go
- `A` internal/server/middleware/csrf.go
- `A` internal/server/middleware/csrf_test.go
- `M` internal/server/router.go
- `M` internal/server/shell_http_live_test.go
- `A` internal/session/store.go
- `A` internal/session/store_integration_test.go
- `M` internal/ssr/render.go
- `A` internal/ssr/status.go
- `M` pkg/testutil/login.go

### migrations
- `A` migrations/0002_users_roles.down.sql
- `A` migrations/0002_users_roles.up.sql

### other
- `M` .env.example
- `A` e2e/fixtures/auth.ts
- `M` e2e/fixtures/test.ts
- `A` e2e/pages/LoginPage.ts
- `A` e2e/pages/UsersPage.ts
- `A` e2e/tests/auth.spec.ts
- `M` e2e/tests/shell.spec.ts
- `A` e2e/tests/users.spec.ts
- `A` web/ssr/_layout/templates/layouts/bare.html
- `A` web/ssr/_layout/templates/pages/status.html
- `M` web/ssr/_shared/app.css
- `A` web/ssr/auth/templates/pages/login.html
- `A` web/ssr/users/templates/pages/form.html
- `A` web/ssr/users/templates/pages/list.html

### docs
- `A` docs/features/phase-2-auth/0-setup/pipeline-state.md
- `A` docs/features/phase-2-auth/0-setup/timeline.jsonl
- `A` docs/features/phase-2-auth/8-code-review/checks/api_conventions.txt
- `A` docs/features/phase-2-auth/8-code-review/checks/contract.txt
- `A` docs/features/phase-2-auth/8-code-review/checks/date_bound.txt
- `A` docs/features/phase-2-auth/8-code-review/checks/e2e_mode.txt
- `A` docs/features/phase-2-auth/8-code-review/checks/fixture_sync.txt
- `A` docs/features/phase-2-auth/8-code-review/checks/fuzz_verdict.txt
- `A` docs/features/phase-2-auth/8-code-review/checks/go_naming.txt
- `A` docs/features/phase-2-auth/8-code-review/checks/live_suite.txt
- `A` docs/features/phase-2-auth/8-code-review/checks/rule_usage.txt
- `A` docs/features/phase-2-auth/8-code-review/diff-backend.patch
- `A` docs/features/phase-2-auth/8-code-review/diff-migrations.patch
- `A` docs/features/phase-2-auth/8-code-review/diff-other.patch
- `A` docs/features/phase-2-auth/8-code-review/input-bundle.md
- `A` docs/features/phase-2-auth/8-code-review/output.md
- `A` docs/features/phase-2-auth/9-validation/evidence/e2e-run.txt
- `A` docs/features/phase-2-auth/9-validation/evidence/last-run.json
- `M` docs/plan.md

## Экстракты документов (вместо полного чтения)
### draft-spec.md
MISSING

### prd.md
MISSING

## Инвентарь доков (точечный grep разрешён, полное чтение — нет)
- 8-code-review/output.md — 10KB
- 8-code-review/input-bundle.md — 3KB
- 0-setup/pipeline-state.md — 0KB
- 9-validation/evidence/last-run.json — 0KB
