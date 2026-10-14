---
stage: 6-plan
feature: phase-3-clients
tier: M
---
# Code Plan: Клиенты и оборудование (фаза 3)
**Дата:** 2026-09-25
**На основе:** `3-design/{output,design-sql,design-errors,design-go-layers,design-tests}.md`
**Совместимость:** MVP (CM-1) — простой DROP в down-миграции, без dual-write/rollback-шагов.
**Статусы:** чекбоксы фаз ниже (носитель статусов M-tier) — `- [ ]` → `- [x]` по завершении фазы (fix-before-next, TD-1).

## Обзор фаз
| # | Фаза | Layer | Файл |
|---|------|-------|------|
| 1 | Shared regression guard (EscapeLike, NavActive, меню «Каталог») | backend | [phase-01](code-plan-phase-01-shared-guard.md) |
| 2 | DB Migration 0003 | backend | [phase-02](code-plan-phase-02-migration.md) |
| 3 | equipmenttypes — backend (DTO/validate/service/repo/handler/routes/DI) | backend | [phase-03](code-plan-phase-03-equipmenttypes-backend.md) |
| 4 | equipmenttypes — SSR frontend + E2E ET-* | frontend | [phase-04](code-plan-phase-04-equipmenttypes-frontend.md) |
| 5 | clients — backend core (клиент+телефоны+version) | backend | [phase-05](code-plan-phase-05-clients-backend.md) |
| 6 | clients — SSR frontend + E2E CL-* (базовые) | frontend | [phase-06](code-plan-phase-06-clients-frontend.md) |
| 7 | equipment — backend (внутри clients pkg) | backend | [phase-07](code-plan-phase-07-equipment-backend.md) |
| 8 | equipment — SSR frontend + E2E EQ-* | frontend | [phase-08](code-plan-phase-08-equipment-frontend.md) |
| 9 | Hardening: concurrency, executor-права, XSS, adaptive/themes, полный регресс | frontend | [phase-09](code-plan-phase-09-hardening.md) |

## Статус фаз
- [x] Phase 1 — Shared regression guard
- [x] Phase 2 — DB Migration 0003
- [x] Phase 3 — equipmenttypes backend
- [x] Phase 4 — equipmenttypes frontend
- [x] Phase 5 — clients backend core
- [x] Phase 6 — clients frontend
- [x] Phase 7 — equipment backend
- [x] Phase 8 — equipment frontend
- [x] Phase 9 — Hardening

## Зависимости между фазами
- 1 → все (shared-код: `pkg/sqlutil.EscapeLike`, `Shell.NavActive`, меню — общая ось риска регресса на users/shell).
- 2 → 3, 5, 7 (таблицы нужны всем backend-фазам).
- 3 → 4 (backend до SSR той же фичи, TD-1).
- 3 → 5, 7 (ADR-102: clients.Repo/equipment читает `equipment_types` SQL-запросом — таблица и данные типов нужны для ручной проверки/сидов; Go-импорта нет).
- 5 → 6 → 7 → 8 (клиент раньше оборудования: карточка клиента — контейнер блока оборудования, AC-007).
- 8 → 9 (кросс-cutting сценарии трогают обе фичи).

## Общие правила
- Каждая фаза = один атомарный коммит, `make build` проходит.
- Backend-гейт фазы: `make build && make test && make lint && make test-integration-scoped`.
- Frontend-гейт фазы (SSR, нет frontend_phase_gate.sh для Vue): `cd e2e && npx tsc --noEmit -p .` + `npx playwright test <specs> --list` + реальный прогон оркестратором (`make e2e SPECS=...`, TD-3: chromium/headed/workers=1) + TD-2 evidence.
- E2E на каждом vertical-capability сразу (TD-1) — не откладывать в Phase 9; Phase 9 — только кросс-cutting (concurrency/adaptive/perms/regression), не первое появление функционального E2E.
- Фикстуры/сиды — в той же фазе, что схема/DTO (TD-1); сид через `AdminApi` (ADR-001 — SQL-сид `seed-demo` не создаётся в фазе 3).
- swagger:no — `make swagger` не гоняется, Phase 11 (Swagger) отсутствует. Monitoring/Runbook/Perf Budget — deferred в дизайне, Phase 11.5/11.6 отсутствуют. Enum-фазы (4.5) нет — enum-полей нет.
- UUID: только `uuid.Validate` на вход (нет `pkg/ids` в проекте — генерация в БД `DEFAULT gen_random_uuid()`, паттерн users, новых полей — не менять).
