---
stage: 8-code-review
feature: phase-3-clients
tier: M
verdict: APPROVED
high: 0
medium: 0
low: 2
---
# Mobile API Review (8-code-review): phase-3-clients — APPROVED

**Артефакты, которые читал:**
- `docs/features/phase-3-clients/8-code-review/input-bundle.md`
- `docs/features/phase-3-clients/8-code-review/diff-backend.patch` (полностью: dto/handler/service/repo/validate/routes для clients и equipmenttypes, router.go)
- `3-design/output.md` §Контракт + «Правила контракта», `3-design/design-errors.md`, `3-design/design-go-layers.md` §DTO Clients/Equipment/EquipmentTypes

**Tier:** M | swagger:no (US-1 — swagger.json не существует, пункты n/a; ревью по DTO-структурам/роутам)

## Battle-test против контракта

- `internal/features/clients/dto.go` — `ClientResponse`/`ClientCreateRequest`/`ClientUpdateRequest`/`PhoneDTO`/`SpecDTO`/`EquipmentResponse`/`EquipmentRequest` 1:1 совпадают с design-go-layers.md §DTO Clients/Equipment (поля, json-теги snake_case, `Version *int64` в `ClientUpdateRequest`).
- `internal/features/equipmenttypes/dto.go` — `TypeResponse`/`TypeCreateRequest`/`TypeUpdateRequest` 1:1 с §DTO EquipmentTypes (`IsActive *bool` в Update).
- Nullable/массивы: `toResponse`/`toEquipmentResponse` (clients/service.go) приводят `nil` → `[]PhoneDTO{}`/`[]SpecDTO{}`; `EquipmentCount`, `Specs` — всегда массив, соответствует design-комментарию «всегда массив, не null». Расхождений `*string` vs `string` по DTO не найдено.
- Конверт списков `{items,total,limit,offset}` — `ClientListResponse`/`EquipmentListResponse`/`TypeListResponse` соблюдают конвенцию §API-1 на всех трёх list-эндпоинтах.
- Лимиты пагинации (service.go): clients `defaultLimit=50/maxLimit=200`, equipment `equipmentDefaultLimit=200/equipmentMaxLimit=200`, equipment-types `defaultLimit=500/maxLimit=500` — совпадают с таблицей output.md §Правила контракта. Мусор/отрицательные значения → тихий откат на дефолт (`limit<=0`→default, `min(limit,max)`, `offset=max(offset,0)`), не 500 — подтверждено `TestList_GarbageParams_DefaultsNot500`.
- URL kebab-case, `/api/v1` префикс — `router.go`/`routes.go`: `/clients`, `/equipment`, `/equipment-types` — соответствует §API-1 (api_conventions checker уже ✅ в bundle).
- Коды ошибок/статусы (design-errors.md → фактический код):
  - `CLIENT_VERSION_CONFLICT` 409 (`service.go:Update`, подтверждено `clients_http_live_test.go` LV-006: PUT со старым version → 409, карточка не перезаписана).
  - `version:nil` → 400 `required` (ADR-100) — `Service.Update` добавляет field-ошибку `version` до вызова repo.
  - `EQUIPMENT_TYPE_NAME_TAKEN`/`NAME_ARCHIVED` — `mapNameConflict` в repo различает по `is_active` строки-конфликта на 23505; подтверждено `TestCreate_DuplicateName_TakenVsArchived` (active/archived case).
  - `EQUIPMENT_TYPE_IN_USE` 409 на DELETE используемого типа — `repo.Delete` ловит и 23503, и 23001 (explicit RESTRICT); подтверждено `TestDelete_InUse409_FreeDeletes`.
  - 404 `CLIENT_NOT_FOUND`/`EQUIPMENT_NOT_FOUND`/`EQUIPMENT_TYPE_NOT_FOUND` на битый/несуществующий uuid — `uuid.Validate` в сервисе до похода в БД, единообразно во всех Get/Update/Delete.
  - `POST /clients/:id/equipment`, клиент удалён параллельно → `errClientGone` → `CLIENT_NOT_FOUND` (404, не 500) — `service.go:CreateEquipment`.
- `PUT /equipment/:id`, `PUT /equipment-types/:id` — осознанный LWW без `version` в DTO (design-комментарий «D28 — version только у clients»); подтверждено live-тестом `TestUpdateEquipment...LWW` (`version клиента не изменился после правок оборудования`, второй PUT без учёта версии применяется).
- `include_archived` для исполнителя — `equipmenttypes/service.go:List`: `includeArchived := q.IncludeArchived && isAdmin`, форсирует `false` для не-admin независимо от query. Подтверждено `TestAuthz_ExecutorReadOnlyActive_MutationsForbidden_AnonUnauthorized`: `include_archived=true` от исполнителя не показывает архивный тип.
- N+1: `ListClients` (clients/repo.go) — телефоны батчем через `phonesBatch` (один `ANY($1::uuid[])` запрос на страницу), `equipment_count` — inline-подзапрос в том же SELECT, не отдельный запрос на строку. `equipmenttypes.List` — `equipment_count` тоже inline-подзапрос в проекции. N+1 не найден ни на одном list-эндпоинте.

## Findings

### HIGH (блокеры)
(нет)

### MEDIUM (advisory)
(нет)

### LOW (nice-to-have)

#### [L-01] Нет conditional GET для lookup-справочника `/equipment-types`
**Класс:** UX/UI
**Локация:** `internal/features/equipmenttypes/handler.go` → `List`
**Проблема:** справочник типов (до 500 записей) отдаётся полностью на каждый запрос без ETag/If-Modified-Since; мобильный клиент дергает его на каждое открытие формы оборудования.
**Рекомендация:** не блокирует M-tier; кандидат на будущую оптимизацию, если справочник вырастет.

#### [L-02] `strconv.Atoi`/`ParseBool` игнорируют ошибку молча
**Класс:** Контракт
**Локация:** `internal/features/clients/handler.go:List/ListEquipment`, `internal/features/equipmenttypes/handler.go:List` — `limit, _ := strconv.Atoi(...)`, `includeArchived, _ := strconv.ParseBool(...)`
**Проблема:** осознанный «тихий откат» на дефолт по договору контракта (output.md §Пагинация) — не баг, но при недокументированном мусоре в `include_archived` (например, `include_archived=1`) `ParseBool` эту форму поддерживает, а нечисловая строка → `false` без сигнала клиенту.
**Рекомендация:** не требует правки — поведение соответствует явному design-решению «тихий откат, паттерн users»; отмечено для полноты записи.

## Резюме

HIGH=0 → APPROVED. Все проверенные пункты (DTO/nullable, envelope пагинации, лимиты default/max, error-коды/статусы, version в PUT клиента, LWW equipment/equipment-types, include_archived для исполнителя, N+1) соответствуют дизайну `3-design/output.md`/`design-go-layers.md`/`design-errors.md` и подтверждены live-тестами (`clients_http_live_test.go`, `equipmenttypes_http_live_test.go`). Breaking changes — n/a (новые эндпоинты, baseline swagger отсутствует, swagger:no).
