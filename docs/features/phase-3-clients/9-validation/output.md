---
stage: 9-validation
feature: phase-3-clients
tier: M
---
# Validation: phase-3-clients — PASS

## Платформы
- Backend (Go): ✅
- Web (SSR html/template + vanilla JS, Vue нет — D30): ✅

## Гейты
- `go build ./...`, `go vet ./...` ✅; `go test ./... -race -count=1` ✅; `golangci-lint run ./...` — 0 issues ✅
- `BASE=d3de17e make test-integration-scoped` ✅ — 8 пакетов (auth, clients, equipmenttypes, users, server, server/middleware, ssr, sqlutil)
- E2E полный прогон №2 (скоуп `full` — вся сюита, 0-setup/e2e-scope-prefs.md): `npx playwright test --project=chromium --headed --workers=1 --retries=1 --reporter=list`
- **Счётчики: passed 57 / hard-fail 0 / flaky 0 / skipped 0**
- Артефакт §TD-2: `9-validation/evidence/last-run.json` (status=passed) + `9-validation/evidence/e2e-run.txt`
- Watch-gate §TD-5 (when=validation): видимый прогон `PWSLOWMO=600` спек clients/equipment/equipment-types — 24/25; CL-002 (21 телефон ≈ 60 действий) истёк по таймауту 30 с из-за slow-mo → `e2e/playwright.config.ts` timeout 180 с при PWSLOWMO (992728f), CL-002 перепроверен в slow-mo ✅; пользователь подтвердил «Продолжаем к QA».
- полный integration-регресс не гонялся (скоуп: task) — integration по diff-скоупу затронул все 8 Go-пакетов с тестами.

## Сценарий (AC trace)
- [x] AC-001 заведение клиента — CL-001, LV-001
- [x] AC-002 обязательные поля/лимиты — CL-002, UT-001…004, LV-003
- [x] AC-003 несколько телефонов — CL-003, CL-004
- [x] AC-004 список/поиск/сортировка — CL-005, CL-006, LV-004, TestList_Search_LeadingCountryDigit
- [x] AC-005 правка + конфликт version — CL-007, CL-020 (2 контекста), LV-005, LV-020
- [x] AC-006 удаление клиента (каскад) — CL-008, LV-007; «есть заказы → 409» — фаза 5 (решение пользователя, Tech Spec В-5)
- [x] AC-007 блок оборудования в карточке — CL-009 (equipment.spec), EQ-001
- [x] AC-008 заведение/правка единицы — EQ-001, EQ-002, EQ-005, LV-010…012
- [x] AC-009 характеристики по шаблону типа — EQ-003, EQ-004
- [x] AC-010 удаление единицы — EQ-006, LV-015; «было в заказе → 409» — фаза 5
- [x] AC-011 типы: список/заведение/уникальность — ET-001, ET-002, LV-030, LV-031
- [x] AC-012 удаление/архив типа — ET-003, ET-004, EQ-007, LV-034, LV-036
- [x] AC-013 права исполнителя — LV-040, LV-041, TestSSR_Guards (UI «Нет доступа» + API 403, POST оборудования 201)
- [x] AC-014 360px/1280px, тёмная тема — CL-021, EQ-021, ET-021

## Observability
- Monitoring & Alerting Spec — deferred в дизайне (MVP, прода нет) → проверка спанов не выполнялась.

## Mobile API live check
- validation skipped: no live API (dev-стенд не задеплоен; проверено локально E2E + live-сьютами)
