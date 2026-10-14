# Phase 08: D26 E2E-гарды + полировка 360px/тёмная тема

**Layer:** frontend (+ проверка backend-гардов фазы 01 на UI-уровне)
**Commit**: `feat(phase-5): D26 E2E-гарды, адаптив 360px и тёмная тема`

## Files to CREATE
- `e2e/tests/board-guards.spec.ts` — `BD-017` (удаление клиента с заказом в `/clients` — сообщение, клиент на месте, AC-017), `BD-018` (удаление оборудования из `task_equipment`/`subtasks.equipment_id` через pg-фикстуру `board.ts` — сообщение, AC-018), `LV-059`-эквивалент на UI (попытка удалить товар, использованный в `subtask_products`/`task_stock`, из `/catalog` — сообщение `PRODUCT_IN_USE`, не 500), удаление услуги, использованной в `subtasks` (`SERVICE_IN_USE`)
- `e2e/tests/board.spec.ts` — MODIFY: добавить `BD-020` (360×800, обе темы: `/`, форма заказа, `/stock` — `hasHorizontalOverflow()` = false; доска скроллится внутри `.board-columns`; перенос через меню «Переместить в…» — ADR-106 клавиатурная/touch альтернатива)

## Files to MODIFY
- `web/ssr/_shared/app.css` — финальные правки overflow на 360px (`.board-col` 85vw ≤576px — проверить фактически), тёмная тема — только `--bs-*` токены, без хардкод-цветов
- `web/ssr/board/templates/pages/board.html`, `stock.html` — точечные правки вёрстки по итогам BD-020 (если найдены переполнения)

## Run after
```bash
npx playwright test e2e/tests/board-guards.spec.ts e2e/tests/board.spec.ts --project=chromium --headed --workers=1
```

## Tests in this phase
- E2E (Playwright, реальный прогон): `BD-017`, `BD-018`, `PRODUCT_IN_USE`/`SERVICE_IN_USE` UI-кейсы (board-guards.spec.ts), `BD-020`

## Definition of Done
- [x] `make lint` — 0 ошибок
- [x] **Playwright E2E-гейт зелёный**: `board-guards.spec.ts` (BD-017, BD-018, PRODUCT_IN_USE/SERVICE_IN_USE — сообщение вместо 500) + `BD-020` в `board.spec.ts` (360×800 обе темы — `hasHorizontalOverflow()=false` на `/`, форме заказа, `/stock`; меню «Переместить в…» работает без DnD)
- [x] Тесты этой фазы зелёные до перехода (TD-1) — 50/50, 2 прогона подряд без флейков
- [x] AC-017, AC-018 — UI-уровень (BD-017, BD-018) закрыт поверх backend-гардов фазы 01
- [x] AC-020 закрыт: доска/форма/«Остатки» без горизонтального скролла страницы на 360px, `.board-columns` — единственный горизонтальный скролл, светлая и тёмная темы корректны
- [x] `internal/features/home`, `web/ssr/home` — подтверждено отсутствие (удалены в фазе 04); в репозитории не осталось ссылок/импортов `home`
- [x] console чистая на всех новых сценариях, ожидаемые 409/404 — через `allowHttpErrors` (TD-4)
