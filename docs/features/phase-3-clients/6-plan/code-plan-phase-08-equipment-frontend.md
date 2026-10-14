### Phase 8: equipment — SSR frontend

**Layer:** frontend
**Commit**: `feat(clients): форма оборудования, шаблоны типов в форме, E2E EQ-*`
**Покрывает:** AC-007, AC-008, AC-009, AC-010, AC-012 (equipment-часть).

**Files to CREATE:**
- `web/ssr/clients/templates/pages/equipment-form.html` — `select.form-select name=type_id` (combobox, `data-spec-keys` JSON per option), brand/model/sku/serial/note, `row-list` характеристик (`spec_k`+`<datalist>`, `spec_v`), удаление (`equipment-delete`, `data-confirm`), «Отмена» → карточка клиента (design-go-layers.md `## SSR Templates`).
- `e2e/pages/EquipmentPage.ts` — POM (форма единицы, specs-строки, combobox).
- `e2e/tests/equipment.spec.ts` — EQ-001…007 (EQ-021 adaptive — Phase 9).
- `e2e/fixtures/auth.ts` — MODIFY: `createEquipment(clientId, e)` (wire-типы).

**Files to MODIFY:**
- `internal/features/clients/ssr.go` — доработать `{eqNew,eqCreate,eqEdit,eqUpdate,eqRemove}` + наполнить блок «Оборудование клиента» в `FormVM.Equipment`/`EquipmentCount` (карточка, `formError` для equipment-ошибок); `EquipmentFormVM`/`SpecRow`/`TypeOptionVM` (design-go-layers.md `## SSR View Models`).
- `internal/features/clients/routes.go` — дополнить SSR-группу `/clients`: `/:id/equipment/new`, `POST /:id/equipment`; новую SSR-группу `q := app.Group("/equipment", RequireSSR, RequireAdmin(status.Forbidden))`: `/:id`, `POST /:id`, `POST /:id/delete`.
- `web/ssr/clients/templates/pages/form.html` — наполнить блок «Оборудование клиента» реальными строками (модель, тип+«(в архиве)», SN, ссылка `/equipment/{id}`, «+ Оборудование»).
- `web/ssr/_shared/js/row-list.js` — (если Phase 6 не покрыла) добавить `KT.rowList.fill(el, rows)` для перерисовки specs при смене типа.
- `web/ssr/clients/templates/pages/equipment-form.html` — inline-скрипт в `{{ define "scripts" }}`: `change` на `type_id` → `JSON.parse(dataset.specKeys)` → merge заполненных+шаблона → `KT.rowList.fill` (ADR-002); обновление `<datalist>`.

**Run after:**
```bash
make build
cd e2e && npx tsc --noEmit -p .
npx playwright test tests/equipment.spec.ts --list
```

**Tests in this phase:**
- E2E: EQ-001 (создание, combobox-поиск типа), EQ-002 (валидация: тип/модель/границы), EQ-003 (правка), EQ-004 (подстановка/сохранение при смене типа), EQ-005 (своя характеристика, key_required), EQ-006 (удаление), EQ-007 (архивный тип не в списке для новой, сохраняется у существующей).

**Definition of Done:**
- [x] `make build` — OK
- [x] `cd e2e && npx tsc --noEmit -p .` — OK
- [x] `npx playwright test tests/equipment.spec.ts --list` — компилируется (7 тестов)
- [ ] E2E EQ-001…007 реально прогнаны: `npx playwright test tests/equipment.spec.ts --project=chromium --headed --workers=1` — зелёные, evidence TD-2 сохранён (за оркестратором, EE) — fix-loop 1: EQ-004 упал (спутанные строки после смены типа), причина в inline-JS формы устранена (см. `equipment-form.html`), перепрогон за оркестратором
- [ ] Регресс clients.spec.ts (карточка теперь содержит реальный блок оборудования) остаётся зелёным (за оркестратором, EE)
- [x] Adversarial (TD-4): «v без k» → «Укажите характеристику» не тихий сбой (EQ-005, покрыто тестом + `TestSSR_Equipment_ValidationRerendersForm`); архивный тип недоступен в селекторе новой единицы (EQ-007, покрыто тестом)
- [x] Go-гейт фазы зелёный: `go build`/`go vet`/`go test`/`golangci-lint`/`go test -tags=integration ./internal/features/clients/...` — все OK
