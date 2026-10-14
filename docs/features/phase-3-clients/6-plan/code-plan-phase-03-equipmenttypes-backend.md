### Phase 3: equipmenttypes — backend

**Layer:** backend
**Commit**: `feat(equipmenttypes): DTO, валидация, сервис, репозиторий, handler, роуты, DI`
**Покрывает:** AC-011, AC-012 (backend-часть); AC-013 (`!isAdmin → IncludeArchived=false`, ADR-004).

**Files to CREATE:**
- `internal/features/equipmenttypes/dto.go` — `TypeResponse/TypeListResponse/TypeCreateRequest/TypeUpdateRequest` (design-go-layers.md `## DTO EquipmentTypes`).
- `internal/features/equipmenttypes/validate.go` — `Input{Name, SpecKeys, IsActive}.normalize()/validate()` (design-go-layers.md `## Validators`, коды `required/too_long/too_many`).
- `internal/features/equipmenttypes/validate_test.go` — UT-008 (name required/100/101; spec_keys trim/дедуп case-insensitive/порядок первого вхождения; 50/51), UT-009 (SSR-разбор `spec_keys` строки через запятую).
- `internal/features/equipmenttypes/service.go` — sentinel `ErrTypeNotFound/ErrTypeNameTaken/ErrTypeNameArchived/ErrTypeInUse` (design-errors.md) + `Service{List,Get,Create,Update,SetActive,Delete}` (design-go-layers.md `## Service Interface` equipmenttypes).
- `internal/features/equipmenttypes/repo.go` — `Repo{List,Get,Create,Update,SetActive,Delete}`, SQL-константы из design-sql.md (`## List/Get/Create/Update/Set Equipment Type Active/Delete Equipment Type Query`); repo-уровень `ErrNotFound=pgx.ErrNoRows`, `errNameTaken`, `errNameArchived` (23505 → дочитать `is_active`), `errInUse` (23503).
- `internal/features/equipmenttypes/handler.go` — `Handler{svc}.{List,Get,Create,Update,Delete}` (API `/equipment-types`); `Bind().Body` ошибка → `BAD_REQUEST` «Некорректное тело запроса»; `limit/offset` тихий откат; `isAdmin` через `middleware.CurrentPrincipal(c).IsAdmin()` → `include_archived` фильтр (ADR-004).
- `internal/features/equipmenttypes/routes.go` — `Register(app, api, a *middleware.Auth, h *Handler, s *SSR)`: API-группа `/equipment-types` (`RequireAPI`; мутации + `RequireAdmin(nil)`), SSR-группа регистрируется как no-op заглушка (реализация — Phase 4), `/new` до `/:id`.
- `internal/features/equipmenttypes/equipmenttypes_http_live_test.go` — `TypesLiveSuite{testutil.LiveSuite}` (TD-8), сиды через API, уникальные имена. LV-030, LV-031, LV-032, LV-033, LV-035 (equipment-types часть), LV-036, LV-040 (API executor часть).

**Files to MODIFY:**
- `internal/server/router.go` — wiring `equipmenttypes.NewService(pool)`, `equipmenttypes.NewHandler`, `equipmenttypes.Register(app, api, guard, h, nil)` (SSR-хендлер — Phase 4, передать позже или nil-заглушку по паттерну проекта).

**Run after:**
```bash
make build
make test-integration-scoped
```

**Tests in this phase:**
- Unit: `TestInput_Validate` (UT-008), `TestParseSpecKeys` (UT-009).
- Live: `TestTypesLiveSuite` — LV-030, LV-031, LV-032, LV-033, LV-035, LV-036, LV-040 (API-часть).

**Definition of Done:**
- [ ] `make build` — OK
- [ ] `make test` — unit OK
- [ ] `make test-integration-scoped` — OK (LV-030…036, LV-040 API-часть зелёные)
- [ ] `make lint` — 0 ошибок
- [ ] `{feature}_http_live_test.go` через `testutil.LiveSuite` (реальный бинарь+TCP), `app.Test` в diff отсутствует (TD-8, `live_suite_check.py`)
- [ ] Тесты этой фазы зелёные до перехода (TD-1)
