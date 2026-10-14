### Phase 5: clients — backend core (клиент + телефоны + version)

**Layer:** backend
**Commit**: `feat(clients): DTO, валидация, сервис, репозиторий, handler, роуты — клиент+телефоны`
**Покрывает:** AC-001, AC-002, AC-003, AC-004, AC-005 (backend), AC-006 (backend). Equipment-часть — Phase 7.

**Files to CREATE:**
- `internal/features/clients/dto.go` — `PhoneDTO/ClientResponse/ClientListResponse/ClientCreateRequest/ClientUpdateRequest` (design-go-layers.md `## DTO Clients`; `EquipmentResponse`/`EquipmentListResponse`/`EquipmentRequest`/`SpecDTO` — заготовки полей, наполнение логики в Phase 7).
- `internal/features/clients/validate.go` — `ClientInput{CompanyName,Address,ContactName,ContactPosition,Phones}.normalize()/validate()` (design-go-layers.md `## Validators`).
- `internal/features/clients/validate_test.go` — UT-001 (company_name/address/contact_* лимиты), UT-002 (phones 0/20/21), UT-003 (пустые строки телефонов, note без phone), UT-004 (phone 32/33, invalid_format, note 200/201).
- `internal/features/clients/service.go` — sentinel `ErrClientNotFound/ErrClientVersionConflict` (equipment-ошибки — Phase 7) + `Service{List,Get,Create,Update,Delete}` (design-go-layers.md `## Service Interface` clients — только клиентские методы).
- `internal/features/clients/repo.go` — `Repo{ListClients,GetClient,CreateClient,UpdateClient,DeleteClient,ClientExists}`, SQL из design-sql.md (`## List Clients Query`, `## Client Phones Batch Query`, `## Get/Create/Update/Delete Client Query`); ADR-003 (EXISTS+digits-поиск), ADR-100 (`UPDATE…WHERE version=$n`+`SELECT EXISTS` при 0 строк), ADR-101 (`DELETE`+`INSERT…unnest…WITH ORDINALITY`).
- `internal/features/clients/handler.go` — `Handler{svc}.{List,Get,Create,Update,Delete}` (equipment-методы — Phase 7).
- `internal/features/clients/routes.go` — `Register(...)`: API `/clients` (`RequireAPI`, мутации `RequireAdmin(nil)`); equipment-подроуты и SSR-группа — заглушки до Phase 6/7.
- `internal/features/clients/clients_http_live_test.go` — `ClientsLiveSuite{testutil.LiveSuite}` (TD-8). LV-001, LV-002, LV-003, LV-004, LV-005, LV-006, LV-008 (клиентская часть), LV-020, LV-035 (clients часть), LV-036 (clients часть), LV-040 (clients API часть).

**Files to MODIFY:**
- `internal/server/router.go` — wiring `clients.NewService(pool)`, `clients.NewHandler`, `clients.Register(app, api, guard, h, nil)` (SSR — Phase 6).

**Run after:**
```bash
make build
make test-integration-scoped
```

**Tests in this phase:**
- Unit: UT-001, UT-002, UT-003, UT-004.
- Live: LV-001, LV-002, LV-003, LV-004, LV-005, LV-006, LV-008 (clients), LV-020 (конкурентность version), LV-035 (clients пагинация), LV-036, LV-040 (clients API).

**Definition of Done:**
- [ ] `make build` — OK
- [ ] `make test` — unit OK
- [ ] `make test-integration-scoped` — OK (LV-001…006, LV-008, LV-020, LV-035, LV-036, LV-040 зелёные)
- [ ] `make lint` — 0 ошибок
- [ ] `clients_http_live_test.go` через `testutil.LiveSuite`, `app.Test` в diff отсутствует (TD-8)
- [ ] Adversarial (TD-4): битый/несуществующий uuid → 404 не 500 (LV-008); старый `version` → 409 не перезапись (LV-006); конкурентная запись — детерминированный исход, один 200/один 409 (LV-020)
- [ ] Тесты этой фазы зелёные до перехода (TD-1)
