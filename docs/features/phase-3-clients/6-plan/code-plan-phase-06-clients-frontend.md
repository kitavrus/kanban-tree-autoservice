### Phase 6: clients — SSR frontend (базовые сценарии)

**Layer:** frontend
**Commit**: `feat(clients): SSR список/форма клиента, row-list телефонов, PRG, E2E CL-*`
**Покрывает:** AC-001, AC-002, AC-003, AC-004, AC-005 (ADR-005 conflict-перерисовка), AC-006 (ADR-103 confirm-удаление). Блок «Оборудование клиента» (AC-007) — верстается пустым в Phase 6, наполняется Phase 8.

**Files to CREATE:**
- `web/ssr/_shared/js/row-list.js` — NEW, `KT.rowList` (ADR-002): `[data-row-list]`/`[data-row-add]`/`[data-row-del]`/`<template data-row-template>`, `data-row-min="1"` (последняя строка телефонов очищается, не удаляется), `KT.rowList.fill(el, rows)`.
- `internal/features/clients/ssr.go` — `SSR{svc,renderer,shell,status}.{list,formNew,formEdit,create,update,remove}` (equipment-методы — Phase 7); `formError` (эталон users) с веткой `ErrClientVersionConflict` → `vm.Conflict=true` (ADR-005); `ListVM/FormVM/PhoneRow` (design-go-layers.md `## SSR View Models` clients).
- `web/ssr/clients/templates/pages/list.html` — GET-форма `q` (`clients-filters`,`filter-q`,«Сбросить»), таблица `clients-table` (Компания-ссылка, Адрес `d-none d-md-table-cell`, Представитель+должность, Телефоны+примечания, Оборудование `d-none d-md-table-cell`), «Клиенты не найдены» (`clients-empty`), «всего N» (`clients-total`), «+ Клиент» (`client-add`).
- `web/ssr/clients/templates/pages/form.html` — поля + `row-list` телефонов (`phone`/`phone_note`/`phone-add`/`phone-del`), hidden `_csrf`/`version`, conflict-блок (`form-error`,`client-conflict-reload`), удаление `data-confirm` (`client-delete`); блок «Оборудование клиента» — верстка с «Оборудования пока нет» (наполнение реальными данными — Phase 8).
- `e2e/pages/ClientsPage.ts` — POM (list/form/conflict/delete).
- `e2e/tests/clients.spec.ts` — CL-001…010 (CL-011 XSS, CL-020 конкурентность, CL-021 adaptive — Phase 9).
- `e2e/fixtures/auth.ts` — MODIFY: `createClient(c)`, `updateClient(id,c)`, `getClient(id)` (wire-типы).

**Files to MODIFY:**
- `web/ssr/_shared/js/ui-dialog.js` — делегированный `submit`-обработчик `form[data-confirm]` → `KT.uiConfirm(text, okText)` → `HTMLFormElement.prototype.submit.call(form)` (ADR-103); `form[data-once]` → disable кнопки на submit (двойной submit, Tech Spec).
- `web/ssr/_shared/app.css` — `.row-list` (flex, wrap на ≤576px, adaptive:min).
- `internal/features/clients/routes.go` — заменить SSR no-op на реальный `Register` SSR-группы `/clients` (`RequireSSR`+`RequireAdmin(status.Forbidden)`): `""`,`/new`,`POST ""`,`/:id`,`POST /:id`,`POST /:id/delete` (equipment-подроуты — Phase 7).
- `internal/server/router.go` — `clientsR, err := ssr.NewRenderer(clients.Root, ssr.SharedRoot)`, передать `clients.NewSSR(...)` в `Register`; `shell := ssr.NewShellBuilder("/", "/users", "/clients", "/catalog")`.

**Run after:**
```bash
make build
cd e2e && npx tsc --noEmit -p .
npx playwright test tests/clients.spec.ts --list
```

**Tests in this phase:**
- Live: LV-021 (SSR PRG create + 409 conflict сохраняет ввод), LV-022 (SSR delete + CSRF 403), LV-041 (SSR executor 403 — clients часть).
- E2E: CL-001 (создание+PRG+flash), CL-002 (exhaustive валидация, TD-6.5), CL-003 (исправление после 422), CL-004 (add/del строк телефонов), CL-005 (поиск+URL-персистентность), CL-006 (сортировка+колонка Оборудование), CL-007 (правка), CL-008 (удаление+confirm), CL-010 (executor меню+прямые URL).

**Definition of Done:**
- [x] `make build` — OK; live-тесты фазы (`go test -tags=integration ./internal/features/clients/... -count=1`) — OK
- [x] `cd e2e && npx tsc --noEmit -p .` — OK
- [x] `npx playwright test tests/clients.spec.ts --list` — компилируется (8 тестов)
- [ ] E2E CL-001…008, CL-010 реально прогнаны: `npx playwright test tests/clients.spec.ts --project=chromium --headed --workers=1` — за оркестратором (EE, dev-агент headed-прогон не запускает)
- [x] TD-6.5 (exhaustive форма, CL-002): каждое поле отдельно (required/границы/формат/too_many), а не только happy-path
- [x] Adversarial (TD-4): 409 conflict не теряет ввод телефонов (ADR-005, LV-021), CSRF-403 без токена (LV-022), executor → «Нет доступа» не 200 (CL-010, LV-041)
- [x] Тесты этой фазы зелёные до перехода (TD-1) — unit/lint/live зелёные; E2E headed — ожидает прогона оркестратора

**Fix-loop 1 (CL-006):** сортировка списка фиксированная, по компании (AC-004, Tech Spec В-4), без UI-переключателя — sort-параметр API не требуется. `internal/features/clients/repo.go:78` уже сортирует `ORDER BY lower(c.company_name), c.id` (регистронезависимо, детерминированный tiebreak) — Go не менялся. Добавлен `e2e/tests/clients.spec.ts` CL-006: 3 клиента через `AdminApi.createClient` в обратном алфавитном порядке/разным регистром → порядок строк в списке проверяется по `data-company`; колонка «Оборудование» (`data-testid="client-equipment-count"`, добавлен в `list.html`) проверена на `0` (ненулевой счётчик — Phase 8, EQ-*, т.к. API оборудования появляется в Phase 7).
