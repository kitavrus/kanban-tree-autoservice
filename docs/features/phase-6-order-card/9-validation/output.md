---
stage: 9-validation
feature: phase-6-order-card
tier: M
---
# Validation: phase-6-order-card — PASS

## Платформы
- Backend (Go): ✅
- Web (SSR html/template + vanilla JS на Bootstrap, Vue нет — D30): ✅

## Гейты
- `go build ./...` ✅; `go test ./... -race -count=1` — 11 пакетов ok ✅; `golangci-lint run ./...` — 0 issues ✅
- `BASE=9580596 make test-integration-scoped` ✅ — FULL-триггер (миграция 0006) → 14 пакетов ok (board: OrderCardLiveSuite, PrintLiveSuite, CardSuite, BoardLiveSuite, StockLiveSuite)
- Чекеры (`--paths internal/features/board`): go-naming PASS, api-conventions PASS, live-suite PASS; review_bundle 8 pass / 1 skip (contract n/a — клиент vanilla JS)
- E2E полный прогон №2 (скоуп `full`, 0-setup/e2e-scope-prefs.md): `npx playwright test --project=chromium --headed --workers=1 --retries=1 --reporter=list`
- **Счётчики: passed 113 / hard-fail 0 / flaky 0 / skipped 0**
- Артефакт §TD-2: `9-validation/evidence/last-run.json` (status=passed) + `9-validation/evidence/e2e-run.txt`
- Watch-gate §TD-5: пропущен — per-run override пользователя «автоматический режим, без вопросов» (0-setup/watch-gate-prefs.md enabled: no).
- полный integration-регресс не гонялся (скоуп: task) — FULL-триггер тула всё равно расширил прогон до `./...`.

## Сценарий (AC trace, design-tests.md §Coverage AC)
- [x] AC-001 полная карточка вместо простой формы, deep-link /?task=N — LV-101/111, OC-001/002/020
- [x] AC-002 шапка кнопкой, конфликт version — LV-102/130/131, OC-003/004, OC-025
- [x] AC-003 оплата только admin — LV-161, OC-005
- [x] AC-004 принятое оборудование, TASK_EQUIPMENT_IN_USE — LV-108/142/143/144, OC-006
- [x] AC-005 новое оборудование из карточки — LV-109, OC-007
- [x] AC-006 история ремонтов единицы — LV-110, OC-008
- [x] AC-007 услуги из каталога — LV-103/104/145/146, OC-009/022
- [x] AC-008 комплект товаров услуги, position — LV-105/171, IT-020, OC-010/021
- [x] AC-009 подсказка наличия — OC-010
- [x] AC-010 списание при правке товаров в работе — LV-120…123, IT-001, OC-011
- [x] AC-011 статус услуги и финальная колонка — LV-140/141/150, OC-012
- [x] AC-012 журнал работ (своя/чужая) — LV-107/163, OC-013
- [x] AC-013 итог заказа, итог/журнал на доске, сумма колонки — LV-106, OC-014, BD-021
- [x] AC-014 одновременная правка строк (LWW, rowQueue) — IT-010…013, LV-130/170, OC-015/016/022/023/024/026
- [x] AC-015 акт приёма — LV-180/182, OC-017
- [x] AC-016 акт выдачи (все услуги со статусом, SD-009) — LV-181, OC-018
- [x] AC-017 права исполнителя — LV-160/162/164, OC-019

## Observability
- Спаны `tasks.card`, `tasks.print`, `subtasks.mutate`, `subtask_products.mutate`, `subtask_logs.mutate`, `task_equipment.mutate` — в коде (Code Review HIGH закрыт); экспорт в Jaeger не проверялся — алерты/стенд мониторинга deferred (MVP, прода нет).

## Mobile API live check
- validation skipped: no live API (dev-стенд не задеплоен; проверено локально E2E + live-сьютами)
