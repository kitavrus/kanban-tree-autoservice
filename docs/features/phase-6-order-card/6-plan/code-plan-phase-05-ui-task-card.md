# Phase 05: UI карточки заказа `task-card.js`

**Layer:** frontend (исполнитель — `backend-dev`, `frontend_phase_gate.sh` неприменим — SSR+vanilla JS проект)
**Commit**: `feat(phase-6): task-card.js — карточка заказа, rowQueue, журнал, печать`

## Files to CREATE
- `web/ssr/_shared/js/task-card.js` (`KT.taskCard`, design-go-layers.md §Frontend Modules): `open(taskId|null, defaultColumnId)`, `close()`; рендер шапки/оборудования/услуг+товаров/журнала/итогов; подсказка остатка (AC-009: `avail = stock + task_reserved`, `need = Σ qty`, `rest = avail − need`); `rowQueue` (ADR-105: ключ строки `subtask:<id>`/`product:<id>`/`equipment:<equipment_id>`/`log:<id>`, не более 1 запроса строки в полёте, `pending` копится, тело PUT собирается из последнего `card` ⊕ `pending`); row-сохранение (`change` → `enqueue` → `aria-busy` → `applyCard` без перезаписи полей с фокусом → `showWarnings`); новый заказ — оборудование/услуги `disabled` до `POST /tasks`; смена клиента с оборудованием → `KT.uiConfirm`; удаление услуги → `KT.uiConfirm`
- `e2e/pages/OrderCardPage.ts` — POM по testid-контракту (design-go-layers.md §Frontend Modules, полный список `card-*`)
- `e2e/tests/order-card.spec.ts` — OC-001, OC-003…OC-013, OC-015, OC-016, OC-019, OC-020, OC-021, OC-022, OC-023 (design-tests.md §E2E)
- `e2e/tests/order-print.spec.ts` — OC-017, OC-018

## Files to MODIFY
- `web/ssr/board/templates/pages/board.html` — `#task-modal` → `modal-xl modal-fullscreen-lg-down modal-dialog-scrollable`; шапка + `#task-description` (textarea); контейнеры `#card-equipment`, `#card-subtasks`, `#card-total`; кнопки актов (`card-print-intake`, `card-print-handover`); `<script src="/assets/ssr-shared/js/task-card.js">` ДО `board.js`
- `web/ssr/_shared/js/board.js` — `openTaskForm` делегирует в `KT.taskCard.open`; после `POST /tasks` — `open(id)` без закрытия модалки (SD-006)

## Run after
```bash
npx playwright test e2e/tests/order-card.spec.ts e2e/tests/order-print.spec.ts --project=chromium --headed --workers=1
```

## Tests in this phase
- E2E: OC-001, OC-003…OC-013, OC-015…OC-023 (design-tests.md §E2E), полный список — `e2e/tests/order-card.spec.ts`, `e2e/tests/order-print.spec.ts`

## Definition of Done
- [x] `npx playwright test e2e/tests/order-card.spec.ts e2e/tests/order-print.spec.ts --project=chromium --headed --workers=1` — зелёные (реальный прогон, 21/21; полный набор фазы — order-card+order-print+board+board-guards+stock — 42/42 дважды подряд)
- [x] AC-001 (открытие карточки, разделы неактивны до сохранения нового заказа — OC-001), AC-002 (шапка+description сохраняются кнопкой), AC-003 (executor: чекбокс оплаты disabled, не сбрасывается), AC-004 (оборудование+confirm смены клиента), AC-005 (новое оборудование из карточки), AC-006 (история — «Уже был в ремонте»), AC-007 (услуга: цена/исполнитель/срок/статус/единица без кнопки «Сохранить»), AC-008 (товары qty/цена, комплект), AC-009 (подсказка остатка 3 состояния), AC-010 (склад: предупреждение + «Остатки»), AC-011 (финальная колонка блокирует), AC-012 (журнал: свои/чужие, `can_edit`), AC-014 (self-race OC-022/023, две вкладки OC-015/016), AC-015/AC-016 (акты в новой вкладке, светлая тема), AC-017 (нет кнопки удаления у executor) — покрыты соответствующими OC-*
- [x] TD-4: OC-021 (qty `-1`/`abc` → ошибка у поля, откат значения, склад не изменился) — happy + adversarial на этой capability
- [x] Self-race (ADR-105, OC-022/023): `page.route` задержка первого PUT строки → в сети ≤1 in-flight PUT этой строки, второй несёт накопленные поля; после reload — все значения сохранены (проверено счётчиком in-flight в page.route)
- [x] `applyCard` не перезаписывает поле, содержащее `document.activeElement` (`rerenderPreserveFocus` в task-card.js; косвенно подтверждено отсутствием сброса фокуса/значения в OC-022/023)
- [x] Testid `card-log-edit`/`card-log-delete` рендерятся только при `can_edit=true` (OC-013); admin-only элементы — снятие атрибутом, не `d-none` (as-is board.js)
- [x] Консоль браузера чистая на всех прогнанных сценариях (без непойманных ошибок JS) — `allowHttpErrors` auto-fixture (test.ts) проверяет это на каждом тесте
- [x] testdata/E2E-фикстуры (`e2e/fixtures/board.ts` из фазы 03) используются без прямых `pg`-вставок в новых спеках
