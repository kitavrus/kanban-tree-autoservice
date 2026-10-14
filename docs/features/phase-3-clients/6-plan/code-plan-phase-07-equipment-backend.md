### Phase 7: equipment — backend (внутри clients pkg)

**Layer:** backend
**Commit**: `feat(clients): оборудование клиента — DTO/валидация/сервис/репо/handler/роуты`
**Покрывает:** AC-007, AC-008, AC-009, AC-010, AC-012 (equipment-часть), AC-013 (D19: `POST /clients/:id/equipment` — admin+executor).

**Files to MODIFY (наполнить стабы Phase 5):**
- `internal/features/clients/dto.go` — доработать `EquipmentResponse{…TypeName,TypeIsActive}`/`EquipmentRequest`/`SpecDTO` полями по design-go-layers.md `## DTO Equipment` (если Phase 5 завела частично).
- `internal/features/clients/validate.go` — добавить `EquipmentInput{TypeID,Brand,Model,SKU,Serial,Note,Specs}.normalize()/validate()` (design-go-layers.md `## Validators`).
- `internal/features/clients/validate_test.go` — UT-005 (type_id/model/brand/sku/serial/note лимиты), UT-006 (specs nil→[]), UT-007 (пустые пары отброшены, v без k → key_required, k/v лимиты, 50/51, дубли k разрешены).
- `internal/features/clients/service.go` — добавить `ErrEquipmentNotFound`; `Service.{ListEquipment,GetEquipment,CreateEquipment,UpdateEquipment,DeleteEquipment,TypeOptions}` (design-go-layers.md, порядок: validate → `TypeActive` → repo).
- `internal/features/clients/repo.go` — добавить `{ListEquipment,GetEquipment,TypeActive,CreateEquipment,UpdateEquipment,DeleteEquipment,TypeOptions}`, SQL из design-sql.md (`## List/Get Equipment Query`, `## Equipment Type Check Query`, `## Create/Update/Delete Equipment Query`, `## Equipment Type Options Query`); ADR-102 (только SQL-чтение `equipment_types`, без Go-импорта), ADR-004 (архивный тип: create 400, update — `check(curTypeID)` в tx).
- `internal/features/clients/handler.go` — добавить `Handler.{ListEquipment,CreateEquipment,GetEquipment,UpdateEquipment,DeleteEquipment}`.
- `internal/features/clients/routes.go` — дополнить API-группу: `g.Get("/:id/equipment", h.ListEquipment)`, `g.Post("/:id/equipment", h.CreateEquipment)` (без admin-guard, D19), `e := api.Group("/equipment", a.RequireAPI())` (`GetEquipment` без admin; `UpdateEquipment/DeleteEquipment` + `admin`).
- `internal/features/clients/clients_http_live_test.go` — добавить LV-007 (delete клиента с оборудованием), LV-010, LV-011, LV-012, LV-013, LV-014, LV-015, LV-040 (equipment API часть).

**Run after:**
```bash
make build
make test-integration-scoped
```

**Tests in this phase:**
- Unit: UT-005, UT-006, UT-007.
- Live: LV-007, LV-010, LV-011, LV-012, LV-013, LV-014, LV-015, LV-040 (equipment часть).

**Definition of Done:**
- [x] `make build` — OK
- [x] `make test` — unit OK
- [x] `make test-integration-scoped` — OK
- [x] `make lint` — 0 ошибок
- [x] Adversarial (TD-4): архивный тип на create → 400 `archived` не 500 (LV-013); несуществующий/удалённый параллельно клиент на `POST .../equipment` → 404 `CLIENT_NOT_FOUND` (errClientGone, 23503); executor допущен на `POST .../equipment`, заблокирован на PUT/DELETE (D19, LV-040)
- [x] Тесты этой фазы зелёные до перехода (TD-1)
