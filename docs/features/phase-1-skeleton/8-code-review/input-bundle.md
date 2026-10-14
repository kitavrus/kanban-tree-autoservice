# Review Bundle: phase-1-skeleton — READY
**Base:** 7b4dd9b | **HEAD:** 10c665d | **Generated:** 2026-09-25 13:18 | diff: 82 files changed, 8211 insertions(+), 5 deletions(-)

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
| backend | 33 | 8-code-review/diff-backend.patch |
| migrations | 2 | 8-code-review/diff-migrations.patch |
| other | 28 | 8-code-review/diff-other.patch |
| docs | 19 | — (артефакты, не ревьюятся построчно) |

### backend
- `A` cmd/migrate/main.go
- `A` cmd/seed/main.go
- `A` cmd/server/main.go
- `A` go.mod
- `A` go.sum
- `A` internal/config/auth.go
- `A` internal/config/config.go
- `A` internal/features/auth/dto.go
- `A` internal/features/auth/handler.go
- `A` internal/features/auth/repo.go
- `A` internal/features/auth/routes.go
- `A` internal/features/auth/service.go
- `A` internal/features/home/handler.go
- `A` internal/features/users/dto.go
- `A` internal/features/users/handler.go
- `A` internal/features/users/repo.go
- `A` internal/features/users/routes.go
- `A` internal/features/users/service.go
- `A` internal/observability/logger.go
- `A` internal/observability/metrics.go
- `A` internal/observability/otel.go
- `A` internal/server/middleware/auth.go
- `A` internal/server/middleware/logging.go
- `A` internal/server/router.go
- `A` internal/server/shell_http_live_test.go
- `A` internal/ssr/menu.go
- `A` internal/ssr/render.go
- `A` internal/ssr/render_test.go
- `A` internal/ssr/shell.go
- `A` pkg/errorspkg/errors.go
- `A` pkg/testutil/base_suite.go
- `A` pkg/testutil/live_suite.go
- `A` pkg/testutil/login.go

### migrations
- `A` migrations/0001_init.down.sql
- `A` migrations/0001_init.up.sql

### other
- `A` .env.example
- `M` .gitignore
- `A` .golangci.yml
- `A` Makefile
- `A` README.md
- `A` docker-compose.yml
- `A` e2e/fixtures/test.ts
- `A` e2e/package-lock.json
- `A` e2e/package.json
- `A` e2e/pages/ShellPage.ts
- `A` e2e/playwright.config.ts
- `A` e2e/tests/shell.spec.ts
- `A` e2e/tsconfig.json
- `A` project/.scaffold-lock.json
- `A` project/manifest.json
- `A` scripts/dev/env.sh
- `A` scripts/dev/up.sh
- `A` web/ssr/_layout/templates/layouts/base.html
- `A` web/ssr/_shared/app.css
- `A` web/ssr/_shared/favicon.svg
- `A` web/ssr/_shared/js/api.js
- `A` web/ssr/_shared/js/combobox.js
- `A` web/ssr/_shared/js/theme.js
- `A` web/ssr/_shared/js/ui-dialog.js
- `A` web/ssr/_shared/vendor/bootstrap/LICENSE
- `A` web/ssr/_shared/vendor/bootstrap/bootstrap.bundle.min.js
- `A` web/ssr/_shared/vendor/bootstrap/bootstrap.min.css
- `A` web/ssr/home/templates/pages/index.html

### docs
- `A` docs/features/phase-1-skeleton/0-setup/pipeline-state.md
- `A` docs/features/phase-1-skeleton/0-setup/timeline.jsonl
- `A` docs/features/phase-1-skeleton/8-code-review/checks/api_conventions.txt
- `A` docs/features/phase-1-skeleton/8-code-review/checks/contract.txt
- `A` docs/features/phase-1-skeleton/8-code-review/checks/date_bound.txt
- `A` docs/features/phase-1-skeleton/8-code-review/checks/e2e_mode.txt
- `A` docs/features/phase-1-skeleton/8-code-review/checks/fixture_sync.txt
- `A` docs/features/phase-1-skeleton/8-code-review/checks/fuzz_verdict.txt
- `A` docs/features/phase-1-skeleton/8-code-review/checks/go_naming.txt
- `A` docs/features/phase-1-skeleton/8-code-review/checks/live_suite.txt
- `A` docs/features/phase-1-skeleton/8-code-review/checks/rule_usage.txt
- `A` docs/features/phase-1-skeleton/8-code-review/diff-backend.patch
- `A` docs/features/phase-1-skeleton/8-code-review/diff-migrations.patch
- `A` docs/features/phase-1-skeleton/8-code-review/diff-other.patch
- `A` docs/features/phase-1-skeleton/8-code-review/input-bundle.md
- `A` docs/features/phase-1-skeleton/8-code-review/output.md
- `A` docs/features/phase-1-skeleton/9-validation/evidence/e2e-run.txt
- `A` docs/features/phase-1-skeleton/9-validation/evidence/last-run.json
- `M` docs/plan.md

## Экстракты документов (вместо полного чтения)
### draft-spec.md
MISSING

### prd.md
MISSING

## Инвентарь доков (точечный grep разрешён, полное чтение — нет)
- 8-code-review/output.md — 11KB
- 8-code-review/input-bundle.md — 4KB
- 0-setup/pipeline-state.md — 0KB
- 9-validation/evidence/last-run.json — 0KB
