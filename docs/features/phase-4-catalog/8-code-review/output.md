---
stage: 8-code-review
feature: phase-4-catalog
tier: M
verdict: APPROVED
high: 0
medium: 0
low: 0
---
# Code Review: phase-4-catalog — APPROVED
**Review level:** Standard (tier: M) — spec-track merged (Sec 18 API-линза), трек /pipeline-light; дельта-проверка fix-loop #2 (`956cdf4`, diff `HEAD~1..HEAD`) поверх ревью fix-loop #1 (`462b8d7`); источник истины `0-setup/task.md` + `docs/plan.md`.

Контекст (не findings): CM-1 MVP; US-1 swagger:no → Sec 15 n/a; Vue нет (D30) → Sec 11/12.2/FE-1 n/a; contract SKIP = n/a (нет TS-клиента); гейты оркестратора unit/lint ✅, integration catalog -race 26/26 ✅, catalog.spec 9/9 ✅ — pre-condition, не перегонялись.

## Закрытие замечаний (история итераций)
- Итерация 1 → fix-loop #1 (`462b8d7`): ✅ HIGH границы NUMERIC (`validateAmount`, `too_precise`/`too_large`/`invalid_format`, `errStockDeltaOutOfRange`, `mapAmountOverflow` → `AMOUNT_OUT_OF_RANGE`); ✅ N+1 (`ProductsBatch`); ✅ архивный товар в существующем комплекте (`existingServiceProductIDs` grandfathered); ✅ валидация до дедупа (`mergeProducts`); ✅ 5 LOW.
- Итерация 2 → fix-loop #2 (`956cdf4`):
  - ✅ MEDIUM индекс ошибки строки: `prepareProducts` возвращает `merged` отдельно, `in.Products` остаются исходными; `ServiceRepo.Create/Update(ctx, …, in, merged)` — `checkProducts` по исходным строкам (индекс = позиция в форме/запросе), `replaceServiceProducts` пишет `merged`. Дубли id в `ANY($1::uuid[])` безопасны. Тест `TestService_DuplicateThenArchived_ErrorIndexPointsAtArchivedRow` (`[A,A,B-архив]` → `products[2]`, без `products[1]`).
  - ✅ LOW невидимая ошибка суммы дублей: `services-form.html` выводит `.Errors "products"` (`data-testid="error-products"`). Тест `TestSSR_MergeOverflow_ShowsVisibleError` (422 + текст).

## Блокеры (обязательно исправить до мержа)
Нет.

## Серьёзные замечания (исправить в этом PR)
Нет.

## Незначительные замечания (можно в следующем PR)
Нет.

## Соответствие коду дизайну (верификация реализации)
- [x] Модель 0004 ↔ task.md (FK RESTRICT/CASCADE, CHECK, UNIQUE `(kind, lower(btrim(name)))`, stock может <0) — ДА
- [x] API §API-1, конверт `{items,total,limit,offset}`, лимиты — ДА
- [x] D16/D26 (`CATEGORY_NAME_TAKEN`/`CATEGORY_IN_USE`/`PRODUCT_IN_USE` + архив) — ДА
- [x] D44 stock_movements manual (tx, `FOR UPDATE`, delta, автор, delta=0 — без записи, переполнение → 400) — ДА
- [x] Комплект D10: дедуп суммой после построчной валидации, qty ≤0/лишняя точность/переполнение → 400, архивный/чужой товар → 400 с корректным индексом строки — ДА
- [x] Authz D19, IDOR/битый uuid 404, NUL/UTF-8 400, strict JSON, CSRF, XSS (`cat_error` — код из закрытого набора) — ДА
- [x] Общий код (equipmenttypes вкладки, router, app.css) — регрессий нет — ДА
- Swagger / Monitoring / Rollout / Runbook / Perf — N/A

## Sec 18 — API-контракт
- nullable `services.price` ↔ `*Amount` ↔ `null` — ДА
- error-body = errorspkg, каждый случай ошибки → своё поле/код — ДА
- N+1 list — ДА (устранено)
- breaking vs as-is — n/a (MVP)

## Покрытие (CT/тесты)
- E2E CT-001…CT-008 — ДА; live-сьюты (LiveSuite) с граничными/индексными/SSR-кейсами — ДА; unit валидаторов — ДА

## Статус фаз
- [x] Подфазы A/B + fix-loop #1/#2 закоммичены (`dfd06a5`, `6cb42a0`, `462b8d7`, `956cdf4`) — ДА

## Итог
APPROVED: 0 HIGH, 0 MEDIUM, 0 LOW.
