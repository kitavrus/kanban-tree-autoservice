### Phase 4: equipmenttypes — SSR frontend

**Layer:** frontend
**Commit**: `feat(equipmenttypes): SSR список/форма типов, каталог-меню, E2E ET-*`
**Покрывает:** AC-011, AC-012 (frontend-часть), ADR-006 (меню «Каталог» → `/catalog` 303).

**Files to CREATE:**
- `internal/features/equipmenttypes/ssr.go` — `SSR{svc,renderer,shell,status}` + методы `list/formNew/formEdit/create/update/remove/archive/restore` (эталон `users/ssr.go formError`); `ListVM/FormVM` (design-go-layers.md `## SSR View Models` equipmenttypes).
- `web/ssr/equipmenttypes/templates/pages/list.html` — чекбокс «Показать архивные» (`archived=1`), таблица (Название+бейдж `badge-off`, Характеристики через запятую/«—», Единиц `d-none d-md-table-cell`, кнопка «В архив»/«Вернуть»), actions «+ Тип оборудования» (`data-testid`: `types-table`, `types-empty`, `type-add`).
- `web/ssr/equipmenttypes/templates/pages/form.html` — `name`, `spec_keys` (через запятую); правка — «Удалить» (`data-confirm`), `InUse` → ошибка + кнопка «В архив» (`data-testid`: `type-name`, `type-spec-keys`, `type-delete`, `type-archive`).
- `e2e/pages/EquipmentTypesPage.ts` — POM (list/form/archive/restore/delete).
- `e2e/tests/equipment-types.spec.ts` — ET-001…004 (ET-021 — Phase 9).
- `e2e/fixtures/auth.ts` — MODIFY: добавить `createEquipmentType(t)`, `updateEquipmentType(id,t)` (wire-типы, TD-6.2).

**Files to MODIFY:**
- `internal/features/equipmenttypes/routes.go` — заменить SSR no-op на реальный `Register` SSR-группы: `app.Get("/catalog", …303 "/catalog/equipment-types")`, `app.Group("/catalog/equipment-types", RequireSSR, RequireAdmin(status.Forbidden))` (design-go-layers.md `## Routes`/`## SSR Routes`).
- `internal/server/router.go` — `typesR, err := ssr.NewRenderer(equipmenttypes.Root, ssr.SharedRoot)`, передать реальный `equipmenttypes.NewSSR(...)` в `Register` вместо nil-заглушки Phase 3.

**Run after:**
```bash
make build
cd e2e && npx tsc --noEmit -p .
npx playwright test tests/equipment-types.spec.ts --list
```

**Tests in this phase:**
- Live: LV-034 (SSR archive/restore/delete-in-use), LV-041 (SSR executor 403 — типы часть).
- E2E: ET-001 (создание+список), ET-002 (дубль имени активный/архивный + ссылка «Показать архивные»), ET-003 (удаление/архив), ET-004 («Показать архивные»+«Вернуть», URL `archived=1` переживает reload).

**Definition of Done:**
- [x] `make build` — OK; backend live-тесты фазы (LV-034, LV-041 типы) — `make test-integration-scoped` OK
- [x] `cd e2e && npx tsc --noEmit -p .` — OK
- [x] `npx playwright test tests/equipment-types.spec.ts --list` — компилируется
- [ ] E2E ET-001…004 реально прогнаны оркестратором: `npx playwright test tests/equipment-types.spec.ts --project=chromium --headed --workers=1` — зелёные, evidence TD-2 сохранён (за оркестратором, EE)
- [x] Adversarial (TD-4): дубль имени → 409 не 500 (ET-002 + LV-032/035); удаление используемого типа → 409 не потеря данных — реализовано в LV-034 (SSR), не в ET-003: `equipment`-записи заводятся только прямым SQL (нет UI/API создания оборудования до Phase 7) — дырка контракта отмечена, см. отчёт фазы
- [x] Тесты этой фазы зелёные до перехода (TD-1)
