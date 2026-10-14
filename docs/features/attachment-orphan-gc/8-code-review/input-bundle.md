# Review Bundle: attachment-orphan-gc — READY
**Base:** d24b7cb2e490d331936a6601151c3da8807fb6bb | **HEAD:** ac815c4 | **Generated:** 2026-09-27 18:27 | diff: 14 files changed, 794 insertions(+), 3 deletions(-)

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
| backend | 10 | 8-code-review/diff-backend.patch |
| other | 1 | 8-code-review/diff-other.patch |
| docs | 3 | — (артефакты, не ревьюятся построчно) |

### backend
- `M` cmd/server/main.go
- `M` internal/config/config.go
- `A` internal/config/config_test.go
- `A` internal/features/board/attachments_gc_http_live_test.go
- `A` internal/features/board/attachments_gc_integration_test.go
- `M` internal/features/board/attachments_http_live_test.go
- `M` internal/features/board/attachments_repo.go
- `M` internal/filestore/filestore.go
- `A` internal/filestore/orphans.go
- `A` internal/filestore/orphans_test.go

### other
- `M` .env.example

### docs
- `A` docs/features/attachment-orphan-gc/0-setup/light-progress.md
- `A` docs/features/attachment-orphan-gc/0-setup/pipeline-state.md
- `A` docs/features/attachment-orphan-gc/0-setup/timeline.jsonl

## Экстракты документов (вместо полного чтения)
### draft-spec.md
MISSING

### prd.md
MISSING

## Инвентарь доков (точечный grep разрешён, полное чтение — нет)
- 8-code-review/output.md — 9KB
- 8-code-review/input-bundle.md — 2KB
- 0-setup/light-progress.md — 1KB
- 0-setup/pipeline-state.md — 0KB
