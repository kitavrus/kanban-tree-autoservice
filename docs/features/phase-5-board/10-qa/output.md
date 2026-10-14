---
stage: 10-qa
feature: phase-5-board
tier: M
verdict: APPROVED
high: 0
medium: 0
low: 1
---
# QA: phase-5-board — APPROVED
**HIGH:** 0  **MEDIUM:** 0  **LOW:** 1

**Профиль:** Бизнес-фича
**Tier:** M
**Артефакты, которые читал:** `10-qa/input-bundle.md` (validation/output.md, 8-code-review/output.md+mobile.md, 3-design/output.md+design-errors.md+design-go-layers.md+design-tests.md+design-sql.md, паспорт AC-001…AC-020); live API http://localhost:8080 (admin/admin1234, sidorov/demo12345); dev-БД `kanban_tree_dev_pg` (docker exec psql).

## Прогнанные классы проверок

- [x] Класс 1 (семантика DTO) — 6 кейсов (AC-016 бейдж=risk_count, AC-010 version conflict, AC-011 PAID_ADMIN_ONLY×2) — `evidence/class1-semantics.txt`
- [x] Класс 2 (regression смежных фич) — D26 (clients/equipment), catalog SERVICE_IN_USE/PRODUCT_IN_USE, RequireSSR no-store, errorspkg/CSRF каркас — `evidence/class2-regression.txt`
  - [ ] Sub 2.a (list ↔ detail parity) — n/a: диф не меняет пару list+detail с user-зависимыми полями (TaskCard одинаков в list/detail, полей вида has_voted/is_owner нет в контракте)
  - [ ] Sub 2.b (N+1 detection через OTEL) — n/a: Monitoring/OTLP deferred (MVP, `9-validation/output.md` §Observability), Jaeger не поднят на dev-стенде
  - [x] Sub 2.c (pagination correctness) — `limit=-1` → 400 VALIDATION (не 500); дефолты/лимиты из контракта подтверждены `GET /columns` (limit100/total76), `GET /tasks` (limit500/total640+), `GET /assignees` (limit200/total637) — `evidence/class1-semantics.txt`, `class3-edge-cases.txt`
- [x] Класс 3 (edge cases) — 10 кейсов (>5 для M) — `evidence/class3-edge-cases.txt`
- [x] Класс 4 (data integrity) — применимо: да (touches.db=true, миграция 0005) — `evidence/class4-data-integrity.txt`
  - [ ] Sub 4.b (rolling deploy compatibility) — n/a: профиль «Бизнес-фича», не Миграция/Рефакторинг
- [x] Класс 5 (state machine & lifecycle) — применимо: да, проверено 2 перехода (SUBTASKS_NOT_DONE→done→move; финализация non-final колонки при наличии других non-final) — `evidence/class5-6-state-authz.txt`
- [x] Класс 6 (authorization depth & data privacy) — применимо: да (FORBIDDEN×3, UNAUTHORIZED, AC-014 assignee-scope) — `evidence/class5-6-state-authz.txt`
- [ ] Класс 7 (quotas & rate limits) — применимо: нет — в дизайне фичи (design-errors.md, contract) нет per-user quota/rate-limit триггеров (лимиты проекта из qa.md §7 — не про phase-5-board)
- [ ] Класс 8 (notification delivery / FCM) — применимо: нет — фича не создаёт push/device_tokens
- [x] Класс 9 (fuzzing Go native + Playwright) — применимо: да (Go-fuzz); go-fuzz crashes: 0/4 таргета, pw-fuzz: n/a (design fuzz-verdict SKIP, tier M, risk reversible — подтверждено `9-validation/output.md`) — `evidence/class9-go-fuzz.txt`
- [x] Класс 10 (supply chain / dependency audit) — применимо: да (e2e/package.json новая devDependency `pg`); govulncheck: 0 reachable, npm audit high+: 0 — `evidence/class10-supply-chain.txt`

## Findings

### HIGH (блокеры)
Нет.

### MEDIUM (advisory)
Нет.

### LOW (nice-to-have)

#### [L-01] LAST_NON_FINAL_COLUMN не воспроизведён живьём на общем dev-стенде
**Класс:** 5
**Локация:** `internal/features/board/columns_service.go` (инвариант «≥1 нефинальная колонка», ADR-104)
**Причина:** на общем dev-стенде 76 колонок (наследие прошлых E2E-прогонов), из них десятки non-final — довести до состояния «последняя нефинальная» без разрушения shared-данных других сессий нельзя.
**Статус:** не блокер — сценарий уже покрыт изолированным `TestBoardLiveSuite_Update_LastNonFinal_Rejected` (`board_http_live_test.go:1001`, code-review HIGH-1 закрыт итерацией 1) и явно помечен в `design-tests.md` как ограничение общего E2E-стенда («LV-054 — только LV-054, на общем стенде нельзя финализировать все колонки»). QA полагается на этот live-тест как достаточное покрытие; фиксирую как LOW для прозрачности отчёта.

## Резюме

APPROVED. Все 20 AC (AC-001…AC-020) сверены с live-поведением там, где применимо к QA-глубине (класс 1: DTO-семантика, класс 3/5: state machine и бизнес-правила поверх покрытия validation/code-review). Регрессий на смежных фичах (clients D26, catalog SERVICE_IN_USE/PRODUCT_IN_USE, errorspkg/CSRF, RequireSSR no-store) не найдено — все ошибки типизированы (409/400/403/404), ни одного 500 на 10+ edge cases и 4 Go-fuzz таргетах (0 крашей). Data integrity (класс 4): инвариант «остаток = стартовый + Σ stock_movements» подтверждён на серии из 4 операций (consume/return/consume/return), FK `stock_movements.task_id` ON DELETE SET NULL и каскадное удаление subtasks/task_stock/task_equipment работают корректно. Supply chain (класс 10): 0 reachable-уязвимостей, привнесённых фичей. Единственная запись — LOW-01 (ограничение общего dev-стенда для LAST_NON_FINAL_COLUMN, уже закрыто изолированным live-тестом в validation/code-review) — не блокирует переход в Report.
