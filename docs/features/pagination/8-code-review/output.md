---
stage: code-review
feature: pagination
tier: S
verdict: APPROVED
high: 0
medium: 0
low: 3
---
# Code Review: pagination — APPROVED
**Review level:** Slim + Sec 18 API-линза (tier: S, spec-track merged) — повторное ревью после fix-loop

Дельта `91b2eb3..5372d21` (dff6fbc, 5c6def0, 5372d21): только `web/ssr/_shared/js/board.js`, e2e (`CatalogPage.ts`, `board-guards.spec.ts`, `catalog.spec.ts`, `pagination.spec.ts`), `api-delta.md`. Go не менялся (build/unit/lint/integration зелёные с первого ревью). Судейство: E2E 137/138 → CT-003 поправлен → catalog+board-guards+pagination 17/17. Чекеры 9/10; e2e_drift ❌ — ложное (TS-001…006 unit/LiveSuite), не finding.

## Блокеры (обязательно исправить до мержа)
Нет.

## Серьёзные замечания (исправить в этом PR)
Нет.

## Незначительные замечания (можно в следующем PR)
1. **[e2e/tests/pagination.spec.ts]** (перенесено, LOW-3 первого ревью) Нет E2E на локальный пересчёт DS-004 при переносе карточки между колонками с догрузкой (`columnTotals`/`colLoaded`/rollback).
2. **[web/ssr/_shared/js/board.js renderBoard `colUnknown` / «Показать заказы»]** Ветка MEDIUM-2 (обрезка `limit=1000` после окна `per_column`) не покрыта автотестом — воспроизведение требует >1000 карточек в окне. Допустимо на S; кандидат на unit-тест JS или LiveSuite с `per_column=1&limit=1` + E2E с подменой ответа через `page.route`.
3. **[internal/features/catalog/ssr_products.go / ssr_services.go — редирект после save/archive/delete] known-issue, as-is, вне скоупа DS.** После сохранения/архивации товара/услуги редирект ведёт на первую страницу списка без фильтра (`/catalog/products?flash=…`). С DS-005 (100 на страницу) пользователь может не увидеть свою запись в выдаче. Рекомендация на следующую итерацию: редирект с `?q=<имя>` или с сохранёнными фильтрами/offset исходного списка. E2E адаптированы через `gotoProductsByName`/`gotoServicesByName` (BD-019a, CT-003, CT-005).

## Верификация фиксов первого ревью
- **MEDIUM-1 (гонка догрузки) — ИСПРАВЛЕНО.** `state.loadGen` растёт в `loadTasks`; устаревший ответ `loadTasks` и `loadMoreColumn` игнорируется (then/catch/finally); `moreInFlight = {}` сбрасывается при перезагрузке, `finally` старого поколения не трогает новый флаг. Попутно закрыта и прежняя гонка двух `loadTasks`.
- **MEDIUM-2 (хвостовые колонки без карточек) — ИСПРАВЛЕНО на клиенте.** `tasksTruncated = items.length < total`; колонка без записи в `columnTotals` → «Показать заказы» (`column-more-<id>`, offset 0); ответ даёт `column_total` (или `columnShown` при пустом ответе) → колонка становится «известной», дальше работает «Показать ещё». Перенос в/из «неизвестной» колонки не создаёт запись (`hasOwnProperty`) — кнопка не прячется; дубли отсекает дедуп по id. Ограничение зафиксировано в `api-delta.md` «Известное ограничение»; серверный контракт не менялся.
- **LOW-1 (кнопка залипает disabled) — ИСПРАВЛЕНО.** После ошибки (то же поколение) кнопка снова активна; повторный клик при in-flight — `return` без `disabled`.
- **LOW-2 (`column_total=0` из мутаций) — ИСПРАВЛЕНО.** `applyTaskPatch` удаляет `column_total` из патча.
- **E2E-правки** — поиск своей записи через `?q=` на постраничном списке (корректная адаптация к DS-005, не маскировка); TS-009 — 102 товара в своей категории.
- **XSS:** новая разметка — только числа, `col.id` (uuid) и статические строки. OK.

## Slim-секции
- **Security (Sec 6):** дельта не трогает auth/SQL/ввод; новых зависимостей нет. OK.
- **Architecture fit / Error Handling:** без изменений против первого ревью. OK.
- **Tests:** unit + LiveSuite (TS-001…006) + E2E TS-007…010 зелёные; пробелы — LOW-1/2.
- **Rollback / breaking:** миграций нет; API только аддитивно. OK.

## Sec 18 — API-контракт
Без изменений против первого ревью: `column_total` всегда присутствует, конверт `{items,total,limit,offset}` сохранён, ошибки VALIDATION по полю, N+1 нет. Документировано ограничение `limit` после окна `per_column`.

## Соответствие коду дизайну (draft-spec.md + api-delta.md)
- [x] DS-001…DS-008 — ДА (DS-001 дополнен клиентской компенсацией обрезки, задокументировано в api-delta)
- Monitoring/Alerts/Rollout/Runbook/Perf Budget — N/A (S-tier)

## Статус фаз
- [x] S-tier /pipeline-light — fix-loop закрыт (dff6fbc, 5c6def0, 5372d21), гейты зелёные.
