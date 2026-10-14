# Phase 06: Доска — итоги/deep-link, 360px и обе темы

**Layer:** frontend (исполнитель — `backend-dev`)
**Commit**: `feat(phase-6): итог/последняя запись на карточке доски, сумма колонки, deep-link /?task=N`

## Files to MODIFY
- `web/ssr/_shared/js/board.js` — `cardHTML`: `total_amount` (если >0) + `last_log` (текст+автор) на карточке доски (`task-card-total`, `task-card-last-log`); заголовок колонки — сумма `total_amount` видимых карточек в копейках (`Math.round(x*100)`, вывод `toFixed(2)`, `column-total`, ADR-005); `parseFilters` игнорирует `task`; deep-link `/?task=N` → `KT.taskCard.open(N)` при загрузке, закрытие карточки → `history.replaceState` без `task` (сохраняя прочие фильтры), несуществующий `#N` → `uiAlert` «Заказ не найден» + `replaceState` без `task` (ADR-104)
- `e2e/tests/order-card.spec.ts` — MODIFY: добавить OC-002 (deep-link открытие/закрытие/несуществующий `#N`)
- `e2e/tests/board.spec.ts` — MODIFY: добавить проверку `column-total` = сумма `task-card-total` видимых карточек (OC-014, часть на доске)

## Run after
```bash
npx playwright test e2e/tests/order-card.spec.ts e2e/tests/order-print.spec.ts e2e/tests/board.spec.ts e2e/tests/board-guards.spec.ts e2e/tests/stock.spec.ts --project=chromium --headed --workers=1
```

## Tests in this phase
- E2E: OC-002, OC-014 (design-tests.md §E2E) + полный регресс-набор `order-card.spec.ts order-print.spec.ts board.spec.ts board-guards.spec.ts stock.spec.ts` (design.md §Verification)

## Definition of Done
- [x] `npx playwright test e2e/tests/order-card.spec.ts e2e/tests/order-print.spec.ts e2e/tests/board.spec.ts e2e/tests/board-guards.spec.ts e2e/tests/stock.spec.ts --project=chromium --headed --workers=1` — зелёные (реальный прогон, 44/44 дважды подряд)
- [x] AC-001 (deep-link `/?task=N` — открытие/закрытие/сохранение фильтров/несуществующий заказ, OC-002), AC-013 (итог на карточке доски + `column-total` = сумма видимых, BD-021) — покрыты
- [x] UI-scope gate (0-setup/ui-scope-prefs.md `adaptive:min · themes:both`, §US-1): полный E2E-набор фазы прогнан на 375px (мобильный вьюпорт, OC-020 из фазы 05 — регресс) и в обеих темах (`data-bs-theme=light|dark`) без горизонтального скролла страницы и без overflow в новых элементах (`task-card-total`, `task-card-last-log`, `column-total`, отдельно проверено в BD-021)
- [x] Консоль браузера чистая на всём регресс-наборе фазы
- [x] `column-total` считается на клиенте (ADR-005) — не расширяет конверт `{items,total,limit,offset}` `GET /tasks`
- [x] Верификация design.md §Verification выполнена целиком: `go test ./internal/features/board/... -tags integration`, fuzz FZ-001…003 (регресс из фазы 03), `api_conventions_check.py`, `go_naming_check.py`, полный E2E-список — все зелёные
