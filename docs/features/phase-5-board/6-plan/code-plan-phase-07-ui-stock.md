# Phase 07: UI «Остатки» + E2E

**Layer:** frontend
**Commit**: `feat(phase-5): UI «Остатки» — KPI, таблица, правка остатка + E2E`

> Vertical-capability фаза (Stock API готов — фаза 04) — E2E прогоняется сразу.

## Files to CREATE
- `web/ssr/_shared/js/stock.js` — `KT.stock`: `summary` (KPI-плитки) + `list` (таблица, переключатель `risky`/`all`), модалка правки остатка (admin, `PUT /api/v1/stock/:product_id`) → после успеха `KT.refreshNavBadge()`
- `e2e/pages/StockPage.ts` — POM: локаторы `stock-kpi-low|out|will-low`, `stock-row-{productId}`, `stock-edit-{productId}`, `stock-scope`
- `e2e/tests/stock.spec.ts` — `BD-015` (KPI, таблица рисковых с прогнозом; admin правит остаток; executor — без кнопки правки), `BD-016` (бейдж «Остатков» виден executor и admin; после move с уходом в минимум — +1 без перезагрузки)

## Files to MODIFY
- `web/ssr/board/templates/pages/stock.html` — заполнить стаб (фаза 04): KPI-плитки, переключатель `risky`/`all`, таблица, модалка правки остатка (admin), `{{block "scripts"}}` → `stock.js`
- `web/ssr/_shared/app.css` — стили KPI-плиток/таблицы «Остатков» (только `--bs-*` токены)

## Run after
```bash
npx playwright test e2e/tests/stock.spec.ts --project=chromium --headed --workers=1
```

## Tests in this phase
- E2E (Playwright, реальный прогон): `BD-015`, `BD-016`

## Definition of Done
- [x] `make lint` — 0 ошибок
- [x] **Playwright E2E «Остатков» зелёный** — happy (BD-015 KPI/таблица/правка) + authz (BD-015 executor без кнопки правки) + live-update (BD-016 бейдж +1 без перезагрузки после move в общем E2E-стенде) — прогон `npx playwright test e2e/tests/stock.spec.ts tests/board.spec.ts --project=chromium --headed --workers=1` — 16/16, 2 прогона подряд
- [x] Тесты этой фазы зелёные до перехода (TD-1)
- [x] testid-контракт (`stock-kpi-*`, `stock-row-{id}`, `stock-edit-{id}`, `stock-scope`) соблюдён дословно
- [x] `KT.refreshNavBadge()` вызывается после `PUT /stock/:product_id` (ADR-004)
- [x] console чистая, ожидаемые статусы через `allowHttpErrors` (TD-4)
- [x] AC-015, AC-016 (UI) закрыты
