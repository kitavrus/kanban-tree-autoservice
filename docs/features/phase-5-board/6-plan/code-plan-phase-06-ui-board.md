# Phase 06: UI доски (board.js DnD, формы) + E2E доски

**Layer:** frontend
**Commit**: `feat(phase-5): UI доски — DnD, фильтры, формы заказа/колонки + E2E`

> Vertical-capability фаза (backend уже готов, фазы 02–04) — E2E прогоняется СРАЗУ в этой фазе (TD-1 per-capability gate), не откладывается.

## Files to CREATE
- `web/ssr/_shared/js/board.js` — `KT.board`: загрузка (`/api/v1/columns`, `/api/v1/tasks?<query>`, `/api/v1/assignees`, `/api/v1/catalog-categories?kind=service`); рендер колонок/карточек; HTML5 DnD (`dragstart/dragover/drop`, вычисление `prev_id`/`next_id` по DOM); оптимистичный перенос + откат на ошибке; меню карточки `⋯` «Переместить в → <колонка>» (Bootstrap dropdown, ADR-106, testid `task-card-menu-{id}`/`task-move-to-{columnId}`); фильтры ↔ `location.search` (`history.replaceState`); формы заказа/колонки (combobox клиента D6, блокировка «Сохранить» до ответа — защита от двойного клика); toast warnings со ссылкой `/stock`; `ui-dialog` (as-is) для 409 (`SUBTASKS_NOT_DONE` — список услуг; `TASK_VERSION_CONFLICT` — «Заказ уже изменён»)
- `e2e/fixtures/board.ts` — API-хелперы (create/update/move/delete заказа и колонки через `AdminApi`) + `pg`-хелпер вставки `subtasks`/`subtask_products`/`task_equipment` для заказа, созданного через API (ADR-103, по `E2E_DATABASE_URL`)
- `e2e/fixtures/board-types.ts` — TS-зеркало (§TD-6.2): `Column`, `TaskCard`, `Task`, `StockWarning`, `StockRow`, `StockSummary`, `ApiErrorSubtasks` — 1:1 по json-тегам `design-go-layers.md` §DTO Column/Task/StockWarning, комментарий `// source: internal/features/board/*_dto.go`
- `e2e/pages/BoardPage.ts` — POM (эталон `e2e/pages/ClientsPage.ts`): локаторы по testid-контракту (design-go-layers.md §Frontend Modules: `board-column-{id}`, `task-card-{id}`, `filter-*`, `task-form`, `task-*`, `column-form`, `column-*`, `toast-warnings`)
- `e2e/tests/board.spec.ts` — `BD-001` (карточка `#N`+название+клиент+ответственный+срок+оплата, цвет полосы), `BD-002` (DnD между колонками + перестановка + reload сохраняет), `BD-003` (заказ с открытой услугой через pg-фикстуру `board.ts` → DnD в «Готово» отклонён с ui-dialog; услуга done → перенос проходит), `BD-004` (товар stock=5, заказ с товаром 2 → «В работе»: `/stock` показывает 3; обратно — 5), `BD-005` (товар stock=1, потребность 3 → перенос проходит, toast warnings со ссылкой «Остатки»), `BD-006` (CRUD колонок admin, executor не видит «+Колонка»), `BD-007` (toggle `consumes_stock` пересчитывает остаток), `BD-008` (дефолтные колонки D32 видны в UI), `BD-009` (создание/правка формой, двойной клик «Сохранить» — 1 запрос), `BD-010` (конкурентная правка — версия), `BD-011` (admin/executor `is_paid`), `BD-012` (удаление заказа admin, товары вернулись, у executor кнопки нет), `BD-013` (фильтры сочетаются, URL, «Сбросить»), `BD-014` («Мои» по умолчанию executor, «Все» показывает чужие)

## Files to MODIFY
- `web/ssr/board/templates/pages/board.html` — заполнить стаб (фаза 04): `{{block "actions"}}` (+Заказ; +Колонка — `{{if .Shell.IsAdmin}}`), фильтр-бар (`filter-client|q|category|paid|assignee|due|reset`), `#board`/`.board-columns` (горизонтальный скролл контейнера), модалки `task-modal`/`column-modal`, `{{block "scripts"}}` → подключить `board.js`
- `web/ssr/_shared/app.css` — `.board-columns` (flex, `overflow-x:auto`, `min-width:0`), `.board-col` (280px; ≤576px — 85vw), `.task-card` (`border-left: 4px solid var(--col-color)`), `.task-card.dragging`/`.drop-target`, overdue — `--bs-danger*`; только `--bs-*` токены (themes:both)
- `e2e/package.json` — devDependency `pg`, `@types/pg` (ADR-103, только e2e — в бинарь/образ не попадает)

## Run after
```bash
npx playwright test e2e/tests/board.spec.ts --project=chromium --headed --workers=1
```

## Tests in this phase
- E2E (Playwright, реальный прогон с артефактом — TD-2/TD-4/TD-6): `BD-001, BD-002, BD-003, BD-004, BD-005, BD-006, BD-007, BD-008, BD-009, BD-010, BD-011, BD-012, BD-013, BD-014`

## Definition of Done
- [x] `make build` — OK (Go-сторона не тронута, но пересборка не ломается)
- [x] `make lint` — 0 ошибок (Go; JS/CSS — отдельного линтера в проекте нет, `node --check` на board.js и `tsc --noEmit`/`playwright test --list` на e2e — чисто)
- [x] **Playwright E2E доски зелёный** — fix-loop итерация 1: `npx playwright test tests/board.spec.ts --project=chromium --headed --workers=1` — 14/14 passed, воспроизведено 3 прогона подряд без флейков
- [x] Тесты этой фазы зелёные до перехода (TD-1)
- [x] testid-контракт (`board-column-{id}`, `task-card-{id}`, `filter-*`, `task-form`/`task-*`, `column-form`/`column-*`, `task-card-menu-{id}`/`task-move-to-{columnId}`, `toast-warnings`) соблюдён
- [x] `KT.refreshNavBadge()` вызывается после успешных move/PUT(смена колонки)/POST/DELETE заказа/PUT колонки (ADR-004)
- [x] JS рендерит пользовательский текст через `textContent`/экранирование (`esc()`), без `innerHTML` с неэкранированными данными (Security §)
- [x] `e2e/fixtures/board.ts` использует `pg`-хелпер (ADR-103), НЕ создаёт тестовый HTTP-эндпоинт в сервере
- [x] console чистая — 3 прогона подряд без единого не-allowlisted `console.error`/`pageerror`/4xx-5xx (allowHttpErrors на ожидаемые 409/403, TD-4)
- [x] AC-001…AC-014 (UI) реализованы и подтверждены зелёным прогоном E2E
