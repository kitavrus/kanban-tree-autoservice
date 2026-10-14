---
stage: 9-validation
feature: phase-5-board
tier: M
---
# Validation: phase-5-board — PASS

## Платформы
- Backend (Go): ✅
- Web (SSR html/template + vanilla JS на Bootstrap, Vue нет — D30): ✅

## Гейты
- `go build ./...`, `go vet ./...` ✅; `go test ./... -race -count=1` — 11 пакетов ok ✅; `golangci-lint run ./...` — 0 issues ✅
- `BASE=be769e5 make test-integration-scoped` ✅ — FULL-триггер (ALTER TABLE stock_movements в 0005) → 14 пакетов ok (board, catalog, clients, seeddemo, server, …)
- Чекеры review_bundle: 9 pass / 0 fail (go-naming, api-conventions, live-suite, …); fuzz-verdict SKIP (tier M, risk reversible)
- E2E полный прогон №2 (скоуп `full`, 0-setup/e2e-scope-prefs.md): `npx playwright test --project=chromium --headed --workers=1 --retries=1 --reporter=list`
- **Счётчики: passed 87 / hard-fail 0 / flaky 0 / skipped 0**
- Артефакт §TD-2: `9-validation/evidence/last-run.json` (status=passed) + `9-validation/evidence/e2e-run.txt`
- Watch-gate §TD-5 (when=validation): `PWSLOWMO=600` board/board-guards/stock — 21/21; пользователь: «Продолжаем к QA».
- полный integration-регресс не гонялся (скоуп: task) — но FULL-триггер тула расширил прогон до `./...`.
- `make seed-demo` на dev-БД: создано (исполнители 3, клиенты 8, оборудование 8, товары 11, услуги 8, заказы 6); повторный вызов — no-op ✅

## Сценарий (AC trace)
- [x] AC-001 доска с колонками и карточками — BD-001, LV-001…
- [x] AC-002 статус = колонка, перенос/порядок, reload — BD-002, LV-060/064 (ребалансировка, 70 вставок)
- [x] AC-003 финальная колонка только без незавершённых услуг — BD-003, LV-050/051; + PUT колонки is_final с открытыми услугами → 409 (TestBoardLiveSuite_Update_ToFinal_ColumnHasOpenSubtasks)
- [x] AC-004 списание/возврат — BD-004, IT-010, TestStockEngine_*
- [x] AC-005 минус и минимум — предупреждения — BD-005
- [x] AC-006 управление колонками (admin) — BD-006, LV-054, TestBoardLiveSuite_Update_LastNonFinal_Rejected
- [x] AC-007 смена «списывать товары» пересчитывает склад — BD-007, LV-062
- [x] AC-008 колонки D32 — BD-008
- [x] AC-009 простая форма заказа — BD-009
- [x] AC-010 конфликт одновременной правки — BD-010, …StaleVersion_PaidChanged_VersionConflictWins
- [x] AC-011 оплата — только admin — BD-011, LV-042
- [x] AC-012 удаление заказа admin, товары на склад — BD-012
- [x] AC-013 фильтры в URL — BD-013
- [x] AC-014 «Мои» по умолчанию у исполнителя — BD-014
- [x] AC-015 экран «Остатки» — BD-015, StockLiveSuite
- [x] AC-016 счётчик в меню — BD-016, SH-001
- [x] AC-017 клиент с заказами не удаляется — BD-017, LV-056
- [x] AC-018 оборудование из заказа не удаляется — BD-018, LV-057
- [x] AC-019 make seed-demo — IT-001 (SeedDemoSuite) + ручная проверка на dev-БД
- [x] AC-020 360px и обе темы — BD-020

## Observability
- Monitoring & Alerting Spec — deferred в дизайне (MVP, прода нет) → проверка спанов не выполнялась.

## Mobile API live check
- validation skipped: no live API (dev-стенд не задеплоен; проверено локально E2E + live-сьютами)
