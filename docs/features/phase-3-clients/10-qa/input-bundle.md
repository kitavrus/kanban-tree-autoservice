# Input Bundle: phase-3-clients / стадия 10-qa — generated 2026-09-26 07:04

> Твой ЕДИНСТВЕННЫЙ документный вход (пайп-контракт). Upstream-доки напрямую НЕ читать;
> точечный grep по конкретному ID (DS-NNN/AC-NNN/TS-NNN) — разрешён. Отсутствие нужного
> факта в бандле = сигнал оркестратору (дырка контракта), не повод читать всё.

## Источник: AC-строки (2-spec-interview/prd.md) [ac]
#### AC-001: Заведение клиента
#### AC-002: Проверка обязательных полей клиента
#### AC-003: Несколько телефонов
#### AC-004: Список и поиск клиентов
#### AC-005: Правка клиента и конфликт одновременной правки
#### AC-006: Удаление клиента
#### AC-007: Оборудование клиента в карточке
#### AC-008: Заведение и правка единицы оборудования
#### AC-009: Характеристики по шаблону типа
#### AC-010: Удаление единицы оборудования
#### AC-011: Типы оборудования — список и заведение
#### AC-012: Удаление и архив типа оборудования
#### AC-013: Права исполнителя
#### AC-014: Работа на узком экране и в тёмной теме

## Источник: 9-validation/output.md [full]
---
stage: 9-validation
feature: phase-3-clients
tier: M
---
# Validation: phase-3-clients — PASS

## Платформы
- Backend (Go): ✅
- Web (SSR html/template + vanilla JS, Vue нет — D30): ✅

## Гейты
- `go build ./...`, `go vet ./...` ✅; `go test ./... -race -count=1` ✅; `golangci-lint run ./...` — 0 issues ✅
- `BASE=d3de17e make test-integration-scoped` ✅ — 8 пакетов (auth, clients, equipmenttypes, users, server, server/middleware, ssr, sqlutil)
- E2E полный прогон №2 (скоуп `full` — вся сюита, 0-setup/e2e-scope-prefs.md): `npx playwright test --project=chromium --headed --workers=1 --retries=1 --reporter=list`
- **Счётчики: passed 57 / hard-fail 0 / flaky 0 / skipped 0**
- Артефакт §TD-2: `9-validation/evidence/last-run.json` (status=passed) + `9-validation/evidence/e2e-run.txt`
- Watch-gate §TD-5 (when=validation): видимый прогон `PWSLOWMO=600` спек clients/equipment/equipment-types — 24/25; CL-002 (21 телефон ≈ 60 действий) истёк по таймауту 30 с из-за slow-mo → `e2e/playwright.config.ts` timeout 180 с при PWSLOWMO (992728f), CL-002 перепроверен в slow-mo ✅; пользователь подтвердил «Продолжаем к QA».
- полный integration-регресс не гонялся (скоуп: task) — integration по diff-скоупу затронул все 8 Go-пакетов с тестами.

## Сценарий (AC trace)
- [x] AC-001 заведение клиента — CL-001, LV-001
- [x] AC-002 обязательные поля/лимиты — CL-002, UT-001…004, LV-003
- [x] AC-003 несколько телефонов — CL-003, CL-004
- [x] AC-004 список/поиск/сортировка — CL-005, CL-006, LV-004, TestList_Search_LeadingCountryDigit
- [x] AC-005 правка + конфликт version — CL-007, CL-020 (2 контекста), LV-005, LV-020
- [x] AC-006 удаление клиента (каскад) — CL-008, LV-007; «есть заказы → 409» — фаза 5 (решение пользователя, Tech Spec В-5)
- [x] AC-007 блок оборудования в карточке — CL-009 (equipment.spec), EQ-001
- [x] AC-008 заведение/правка единицы — EQ-001, EQ-002, EQ-005, LV-010…012
- [x] AC-009 характеристики по шаблону типа — EQ-003, EQ-004
- [x] AC-010 удаление единицы — EQ-006, LV-015; «было в заказе → 409» — фаза 5
- [x] AC-011 типы: список/заведение/уникальность — ET-001, ET-002, LV-030, LV-031
- [x] AC-012 удаление/архив типа — ET-003, ET-004, EQ-007, LV-034, LV-036
- [x] AC-013 права исполнителя — LV-040, LV-041, TestSSR_Guards (UI «Нет доступа» + API 403, POST оборудования 201)
- [x] AC-014 360px/1280px, тёмная тема — CL-021, EQ-021, ET-021

## Observability
- Monitoring & Alerting Spec — deferred в дизайне (MVP, прода нет) → проверка спанов не выполнялась.

## Mobile API live check
- validation skipped: no live API (dev-стенд не задеплоен; проверено локально E2E + live-сьютами)

## Источник: 8-code-review/output.md [full]
---
stage: 8-code-review
feature: phase-3-clients
tier: M
verdict: APPROVED
high: 0
medium: 0
low: 4
---
# Code Review: phase-3-clients — APPROVED
**Review level:** Standard (tier: M) — дельта-перепроверка после fix-loop #2 (коммит 1d0506f, `git diff HEAD~1 HEAD`). Sec 1,2,3,4,6,7,8,10,12,14,17; Sec 11/12.2 n/a (SSR html/template + vanilla JS, D30/D38); Sec 15 n/a (`swagger:no`); CM-1 = MVP.

Чекеры (bundle перегенерирован, не перегонялись): go_naming ✅ · api_conventions ✅ · date_bound ✅ · fuzz_verdict ✅ · live_suite ✅ · fixture_sync ✅ · e2e_mode ✅ · rule_usage ✅ · contract SKIP → n/a (TS-клиента нет). Гейты оркестратора: build/vet ✅, unit ✅, lint ✅, integration clients -race ✅; E2E затронутых спек 25/25 ✅ (fix-loop #1), полный 57/57 — `9-validation/evidence/last-run.json`.

## Статус замечаний
| Итерация | Замечание | Статус |
|---|---|---|
| #1 MEDIUM-1 | NUL/невалидный UTF-8 → 500 | ✅ закрыто (25b7784, `sqlutil.HasInvalidText` → `invalid_text`) |
| #1 MEDIUM-2 | SSR-перерисовка не нормализована | ✅ закрыто (25b7784) |
| #1 MEDIUM-3 / #2 MEDIUM-1 | поиск телефона 7↔8 + регрессия частичного ввода | ✅ закрыто (1d0506f) — см. ниже |
| #1 MEDIUM-4 | UT-010 / метки LV | ✅ закрыто (25b7784) |
| #1 LOW 23503 / TOCTOU / PRG-flash | — | ✅ закрыто (25b7784) |
| рекомендация | `invalid_text` в design-errors §Codes Fields | ✅ добавлено (1d0506f) |

Проверка фикса 1d0506f (`internal/features/clients/service.go` `searchDigits` → `(digits, digitsAlt)`; `repo.go` `phoneDigitsExpr` / `phoneAltDigitsExpr`, `$2`/`$3` через OR):
- `+7 999 555` → `7999555` / alt `999555`; хранимый `+7 (999) 555-11-22` → raw `79995551122` ✓ (и alt `9995551122` ✓) — регрессия устранена.
- `8 (999) 555` → alt `999555` ⊂ `9995551122` ✓ — пример ADR-003 «8999123 → +7 (999) 123-…» выполняется.
- `89995551122` ↔ `+79995551122` — alt `9995551122` = alt хранимого ✓; ввод без префикса (`999 111`) — raw ✓.
- Параметризация сохранена (`$1..$3`, цифры только `[0-9]`); `digits[0]` безопасен (len ≥ `minSearchDigits`).
- Тесты: UT-010 `TestSearchDigits` (10 кейсов), live `TestList_Search_LeadingCountryDigit` (7↔8 полный, частичный, регрессии `+7 704 111`/`8 (704) 111`, негатив).

## Блокеры (обязательно исправить до мержа)
Нет.

## Серьёзные замечания (исправить в этом PR)
Нет.

## Незначительные замечания (можно в следующем PR)
1. **[internal/features/clients/service.go `searchDigits`]** `digitsAlt` не проверяется на `minSearchDigits`: запрос из 3 цифр с ведущей 7/8 (`755`, `812`) даёт alt из 2 цифр (`55`, `12`) → широкая выдача (любой номер с «55» после обрезки ведущей 7/8). Результат — надмножество корректного, не потеря; фикс: `digitsAlt` только при `len(digits)-1 >= minSearchDigits` + кейс в UT-010.
2. **[internal/features/clients/ssr.go list/formEdit]** SSR-список `Limit: maxLimit` (200) без пагинации; блок оборудования карточки обрезается на 200 — known issue MVP, зафиксировать в Report.
3. **[internal/features/equipmenttypes/handler.go:67]** Проверка `is_active` в handler с ранним return — остальные ошибки полей не возвращаются вместе; known issue.
4. **[internal/features/equipmenttypes/ssr.go update]** read-then-write `IsActive: cur.IsActive` — параллельный архив может быть откатан (LWW); known issue.

## Соответствие коду дизайну (верификация реализации)
- [x] Dataflow в коде соответствует design — ДА
- [x] Все SQL queries из 3-design/design-sql.md реализованы — ДА (поиск по телефону — два OR-сравнения, EXISTS сохранён, `total` без дублей)
- [x] Go слои по 3-design/design-go-layers.md — ДА (плоский пакет — отступление Tech Spec В-1)
- [x] Error handling соответствует 3-design/design-errors.md — ДА (включая `invalid_text`)
- [x] ADR-003 (поиск по телефону) — ДА
- [x] PRG-редиректы по §SSR Routes — ДА
- [x] Infrastructure / Swagger / Vue — N/A
- [x] Monitoring / Алерты / Rollout / Runbook / Perf Budget — N/A (deferred, MVP / M-tier)

## PRD соответствие
- [x] Каждое ADR имеет Driver — ДА
- [x] AC-001…AC-014 покрыты тестами (матрица design-tests.md) — ДА
- [x] Success Metric «0 молча потерянных правок» наблюдаема (LV-020/CL-020) — ДА
- [x] Non-Goals не реализованы — ДА

## Статус фаз
- [x] Все фазы закрыты: `6-plan/code-plan.md` Phase 1–9 `- [x]` — ДА

## Итог
APPROVED: блокеров и серьёзных замечаний нет; регрессия поиска телефона устранена. 4 LOW — 1 новое (порог длины `digitsAlt`, точность выдачи) + 3 known issues MVP.

## Источник: 8-code-review/mobile.md [verdict]
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

## Источник: 8-code-review/frontend-arch.md [verdict]
MISSING

## Источник: 3-design/output.md [sections]
## Контракт
Конвенции: API-1 (kebab URL, snake_case JSON, списки `{items,total,limit,offset}`); FE-1-wire: `version` в body. Ошибки — `pkg/errorspkg` `{code,message,details?}`. DTO — `design-go-layers.md`.

### API `/api/v1`
| Метод | Путь | Гард | Request | Response | Driver |
|---|---|---|---|---|---|
| GET | `/clients?q=&limit=&offset=` | RequireAPI | — | `ClientListResponse` 200 | AC-004 |
| POST | `/clients` | RequireAPI+RequireAdmin | `ClientCreateRequest` | `ClientResponse` 201 | AC-001, AC-002, AC-003 |
| GET | `/clients/:id` | RequireAPI | — | `ClientResponse` 200 | AC-005 |
| PUT | `/clients/:id` | RequireAPI+RequireAdmin | `ClientUpdateRequest` (+`version`), полная замена вкл. `phones[]` (replace-all) | `ClientResponse` 200 / 409 `CLIENT_VERSION_CONFLICT` | AC-005 |
| DELETE | `/clients/:id` | RequireAPI+RequireAdmin | — | 204 | AC-006 |
| GET | `/clients/:id/equipment?limit=&offset=` | RequireAPI | — | `EquipmentListResponse` 200 | AC-007 |
| POST | `/clients/:id/equipment` | RequireAPI (admin+executor, D19) | `EquipmentRequest` | `EquipmentResponse` 201 | AC-008, AC-013 |
| GET | `/equipment/:id` | RequireAPI | — | `EquipmentResponse` 200 | AC-008 |
| PUT | `/equipment/:id` | RequireAPI+RequireAdmin | `EquipmentRequest`, **полная замена, LWW** (не patch: отсутствующее поле = пустое значение, `specs` заменяется целиком) | `EquipmentResponse` 200 | AC-008, AC-009 |
| DELETE | `/equipment/:id` | RequireAPI+RequireAdmin | — | 204 | AC-010 |
| GET | `/equipment-types?include_archived=&q=&limit=&offset=` | RequireAPI (`include_archived` — только admin, ADR-004) | — | `TypeListResponse` 200 | AC-011, AC-013 |
| POST | `/equipment-types` | RequireAPI+RequireAdmin | `TypeCreateRequest` | `TypeResponse` 201 / 409 `EQUIPMENT_TYPE_NAME_TAKEN` / 409 `EQUIPMENT_TYPE_NAME_ARCHIVED` | AC-011 |
| GET | `/equipment-types/:id` | RequireAPI | — | `TypeResponse` 200 | AC-011 |
| PUT | `/equipment-types/:id` | RequireAPI+RequireAdmin | `TypeUpdateRequest` (+`is_active`), **полная замена, LWW** | `TypeResponse` 200 / 409 `…NAME_TAKEN` / `…NAME_ARCHIVED` | AC-011, AC-012 |
| DELETE | `/equipment-types/:id` | RequireAPI+RequireAdmin | — | 204 / 409 `EQUIPMENT_TYPE_IN_USE` | AC-012 |

### Правила контракта (fix-loop 1, mobile-review)
- **Блокировка правок (D28):** только `PUT /clients/:id` защищён `version` → 409 `CLIENT_VERSION_CONFLICT` (ADR-100). `PUT /equipment/:id` и `PUT /equipment-types/:id` — осознанно last-write-wins, `version` в DTO нет, 409 по конкурентной правке не бывает (не пропуск: AC-008/AC-011 конфликт правки не требуют; D28 — `version` только у клиентов и заказов). Правка оборудования `clients.version` не меняет.
- **Семантика PUT:** все три PUT — полная замена ресурса (не patch); клиент шлёт весь объект, полученный из GET, с изменениями.
- **Пагинация** (`limit`/`offset`; пусто/мусор/≤0 → дефолт, > max → max, `offset<0` → 0; тихий откат, паттерн users):

| Эндпоинт | default `limit` | max `limit` | Примечание |
|---|---|---|---|
| `GET /clients` | 50 | 200 | `total` — счётчик для «всего N» |
| `GET /clients/:id/equipment` | 200 | 200 | карточка показывает всё (единиц у клиента ≪ 200) |
| `GET /equipment-types` | 500 | 500 | lookup-справочник: без `limit` отдаёт все активные типы для селектора (AC-008/AC-009); клиент сверяет `total ≤ limit` |

  SSR-селектор типа в форме единицы API не использует — `clients.Repo.TypeOptions` без лимита (все активные + текущий архивный, ADR-004).
- **Повторное создание:** idempotency-key на `POST /clients`, `POST /clients/:id/equipment`, `POST /equipment-types` НЕ вводится — решение Tech Spec (§Технические ограничения «Двойной submit»: disable кнопки + PRG, серверный дедуп не делается — нет естественного ключа). У типов дубль отсекается уникальностью имени (409). Клиенты API обязаны блокировать повторную отправку сами.
- **Язык `message`:** все `message` новых ошибок — русские; `BAD_REQUEST` новых хендлеров — «Некорректное тело запроса» (design-errors.md). Машинное ветвление — только по `code`.

### SSR
Маршруты и PRG-редиректы — `design-go-layers.md` §SSR Routes (все: RequireSSR + RequireAdmin(status.Forbidden); AC-013; ADR-005).

Ошибка формы: тот же POST, форма перерисовывается с вводом (400-валидация → 422, 409 → 409, 404 → `status.NotFound`) — паттерн `users/ssr.go formError`.

## Источник: 3-design/design-errors.md [full]
---
stage: 3-design
feature: phase-3-clients
tier: M
---
# Design Errors: phase-3-clients

Формат — `pkg/errorspkg` без изменений: `{code,message,details?}`, 400 `VALIDATION` с `details.fields[{name,message}]` (message — код, SSR переводит в текст). Коды — строковые (как `LOGIN_TAKEN`), числовых supportMessage в проекте нет.

## Sentinel Errors
`internal/features/clients/service.go`:
```go
var (
	ErrClientNotFound        = errorspkg.New("CLIENT_NOT_FOUND", http.StatusNotFound, "Клиент не найден")
	ErrEquipmentNotFound     = errorspkg.New("EQUIPMENT_NOT_FOUND", http.StatusNotFound, "Оборудование не найдено")
	ErrClientVersionConflict = errorspkg.New("CLIENT_VERSION_CONFLICT", http.StatusConflict,
		"Карточку уже изменил другой пользователь — ваши правки не сохранены")
)
```
`internal/features/equipmenttypes/service.go`:
```go
var (
	ErrTypeNotFound          = errorspkg.New("EQUIPMENT_TYPE_NOT_FOUND", http.StatusNotFound, "Тип оборудования не найден")
	ErrTypeNameTaken         = errorspkg.New("EQUIPMENT_TYPE_NAME_TAKEN", http.StatusConflict, "Такой тип уже есть")
	ErrTypeNameArchived = errorspkg.New("EQUIPMENT_TYPE_NAME_ARCHIVED", http.StatusConflict,
		"Такой тип уже есть в архиве — верните его из архива")                 // ADR-007: отдельный код → клиент показывает CTA «Вернуть из архива» по code, не по тексту
	ErrTypeInUse = errorspkg.New("EQUIPMENT_TYPE_IN_USE", http.StatusConflict,
		"Тип используется в оборудовании клиентов — удалить нельзя, отправьте его в архив")
)
```
Repo-уровень (неэкспортируемые, маппинг pg → домен в repo, `errors.Is` в сервисе — паттерн users): `ErrNotFound = pgx.ErrNoRows`, `errVersionConflict`, `errNameTaken`, `errNameArchived`, `errInUse` (23503), `errClientGone` (23503 на `equipment.client_id`).

НЕ вводятся (фаза 5, Tech Spec D26): `CLIENT_HAS_ORDERS`, `EQUIPMENT_IN_ORDERS`.

## Codes Fields
Коды `details.fields[].message` (константы в `validate.go` каждой фичи; `required/too_long/invalid_format/invalid` — как users):
| Код | Смысл | Поля |
|---|---|---|
| `required` | пусто после trim | `company_name`, `phones`, `phones[i].phone`, `model`, `type_id`, `name`, `version`, `is_active` |
| `too_long` | превышение rune-лимита | все строковые, `specs[i].k`, `specs[i].v`, `spec_keys[i]` |
| `too_many` | превышение числа строк | `phones` (>20), `specs` (>50), `spec_keys` (>50 после дедупа) |
| `invalid_format` | телефон без цифр | `phones[i].phone` |
| `invalid` | битый uuid | `type_id` |
| `not_found` | тип не существует | `type_id` |
| `archived` | тип в архиве (создание/смена) | `type_id` |
| `key_required` | значение без названия характеристики | `specs[i].k` |
| `invalid_text` | NUL-байт или невалидный UTF-8 (code review fix-loop #1, §TD-4: мусор → 400, не 500 от Postgres 22021) | все пользовательские строки clients / equipment / equipment-types |

Индекс `i` — позиция в НОРМАЛИЗОВАННОМ списке (пустые строки уже отброшены); SSR перерисовывает форму нормализованным списком → индексы совпадают со строками формы.

## Error Mapping
### API (`errorspkg.FiberErrorHandler`, без изменений)
| Ситуация | HTTP | code |
|---|---|---|
| невалидный JSON / неизвестные поля (`strictJSON`) | 400 | `BAD_REQUEST`, message «Некорректное тело запроса» (новые хендлеры фазы 3; `users` — as-is «invalid body», не переписывается молча) |
| ошибки полей | 400 | `VALIDATION` + `details.fields` |
| битый/несуществующий `:id` | 404 | `CLIENT_NOT_FOUND` / `EQUIPMENT_NOT_FOUND` / `EQUIPMENT_TYPE_NOT_FOUND` |
| POST equipment, клиента нет / удалён параллельно | 404 | `CLIENT_NOT_FOUND` |
| устаревший `version` | 409 | `CLIENT_VERSION_CONFLICT` |
| дубль имени активного типа | 409 | `EQUIPMENT_TYPE_NAME_TAKEN` |
| дубль имени архивного типа (ADR-007) | 409 | `EQUIPMENT_TYPE_NAME_ARCHIVED` — клиент находит тип `GET /equipment-types?include_archived=true&q=<name>` и предлагает `PUT … is_active:true` |
| DELETE используемого типа | 409 | `EQUIPMENT_TYPE_IN_USE` |
| executor на admin-мутации | 403 | `FORBIDDEN` (`RequireAdmin(nil)`) |
| без сессии | 401 | `UNAUTHORIZED` (`RequireAPI`) |

### SSR (`formError` в `clients/ssr.go`, `equipmenttypes/ssr.go` — эталон `users/ssr.go`)
| Ошибка | Статус | Отображение |
|---|---|---|
| `VALIDATION` | 422 | текст у поля (`fieldText`), ввод сохранён |
| `ErrClientVersionConflict` | 409 | `vm.Error` = message + `vm.Conflict` (ссылка «Открыть актуальную версию», ADR-005) |
| `ErrTypeNameTaken` | 409 | текст у поля `name` |
| `ErrTypeNameArchived` | 409 | текст у поля `name` + ссылка «Показать архивные» (`/catalog/equipment-types?archived=1`) |
| `ErrTypeInUse` | 409 | форма типа: `vm.Error` + видимая кнопка «В архив» |
| `*NotFound` | 404 | `status.NotFound` |
| прочее ≥500 | 500 | пробрасывается (`FiberErrorHandler`) |

### fieldText (SSR, тексты)
| Код | Текст |
|---|---|
| `required` | «Обязательное поле»; `phones` — «Укажите хотя бы один телефон»; `type_id` — «Выберите тип» |
| `too_long` | «Слишком длинное значение» |
| `too_many` | `phones` — «Не больше 20 телефонов»; `specs`/`spec_keys` — «Не больше 50 характеристик» |
| `invalid_format` | «Номер должен содержать цифры» |
| `invalid` / `not_found` | «Тип не найден» |
| `archived` | «Тип в архиве — выберите другой» |
| `key_required` | «Укажите характеристику» |
| `invalid_text` | «Недопустимое значение» |

## Источник: 3-design/design-go-layers.md [sections]
## DTO Clients
`internal/features/clients/dto.go` (JSON snake_case, API-1):
```go
type PhoneDTO struct {
	Phone string `json:"phone"`
	Note  string `json:"note"`
}
type ClientResponse struct {
	ID              string     `json:"id"`
	CompanyName     string     `json:"company_name"`
	Address         string     `json:"address"`
	ContactName     string     `json:"contact_name"`
	ContactPosition string     `json:"contact_position"`
	Phones          []PhoneDTO `json:"phones"`          // всегда массив, не null
	EquipmentCount  int        `json:"equipment_count"`
	Version         int64      `json:"version"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}
type ClientListResponse struct {
	Items  []ClientResponse `json:"items"`
	Total  int              `json:"total"`
	Limit  int              `json:"limit"`
	Offset int              `json:"offset"`
}
type ClientCreateRequest struct {
	CompanyName     string     `json:"company_name"`
	Address         string     `json:"address"`
	ContactName     string     `json:"contact_name"`
	ContactPosition string     `json:"contact_position"`
	Phones          []PhoneDTO `json:"phones"`
}
type ClientUpdateRequest struct {
	CompanyName     string     `json:"company_name"`
	Address         string     `json:"address"`
	ContactName     string     `json:"contact_name"`
	ContactPosition string     `json:"contact_position"`
	Phones          []PhoneDTO `json:"phones"`
	Version         *int64     `json:"version"`          // nil → 400 version: required (ADR-100)
}
```

## DTO Equipment
```go
type SpecDTO struct {
	K string `json:"k"`
	V string `json:"v"`
}
type EquipmentResponse struct {
	ID           string    `json:"id"`
	ClientID     string    `json:"client_id"`
	TypeID       string    `json:"type_id"`
	TypeName     string    `json:"type_name"`
	TypeIsActive bool      `json:"type_is_active"`   // ADR-004
	Brand        string    `json:"brand"`
	Model        string    `json:"model"`
	SKU          string    `json:"sku"`
	Serial       string    `json:"serial"`
	Specs        []SpecDTO `json:"specs"`            // всегда массив
	Note         string    `json:"note"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
type EquipmentListResponse struct {
	Items  []EquipmentResponse `json:"items"`
	Total  int                 `json:"total"`
	Limit  int                 `json:"limit"`
	Offset int                 `json:"offset"`
}
// EquipmentRequest — POST /clients/:id/equipment и PUT /equipment/:id (полная замена, LWW — output.md §Правила контракта).
type EquipmentRequest struct {
	TypeID string    `json:"type_id"`
	Brand  string    `json:"brand"`
	Model  string    `json:"model"`
	SKU    string    `json:"sku"`
	Serial string    `json:"serial"`
	Specs  []SpecDTO `json:"specs"`
	Note   string    `json:"note"`
}
```

## DTO EquipmentTypes
`internal/features/equipmenttypes/dto.go`:
```go
type TypeResponse struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	SpecKeys       []string  `json:"spec_keys"`       // всегда массив
	IsActive       bool      `json:"is_active"`
	EquipmentCount int       `json:"equipment_count"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
type TypeListResponse struct {
	Items  []TypeResponse `json:"items"`
	Total  int            `json:"total"`
	Limit  int            `json:"limit"`
	Offset int            `json:"offset"`
}
type TypeCreateRequest struct {
	Name     string   `json:"name"`
	SpecKeys []string `json:"spec_keys"`
}
type TypeUpdateRequest struct {
	Name     string   `json:"name"`
	SpecKeys []string `json:"spec_keys"`
	IsActive *bool    `json:"is_active"`   // nil → 400 is_active: required (как users.UpdateRequest)
}
```

## Источник: 3-design/design-tests.md [full]
---
stage: 3-design
feature: phase-3-clients
tier: M
---
# Design Tests: phase-3-clients

Доктрина: TD-1 (послойно, fix-before-next; E2E на capability), TD-4 (adversarial), TD-6 (POM, глубина форм), TD-8 (HTTP-integration только `testutil.LiveSuite`/`Browser`, `app.Test` запрещён). US-1: adaptive:min (нет overflow ~360px + 1280px), themes:both. E2E — `e2e/`, `--project=chromium --headed --workers=1` (TD-3), запускает оркестратор.

ID: `UT-NNN` unit · `LV-NNN` live (Go, тег `integration`) · E2E: `CL-NNN` клиенты, `EQ-NNN` оборудование, `ET-NNN` типы (заголовок Playwright-теста = `test('CL-001: …')`).

## Матрица AC → тесты
| AC | Unit | Live | E2E |
|---|---|---|---|
| AC-001 | UT-001, UT-003 | LV-001, LV-021 | CL-001, CL-011 |
| AC-002 | UT-002, UT-004 | LV-002, LV-036 | CL-002, CL-003 |
| AC-003 | UT-003 | LV-001, LV-003 | CL-004 |
| AC-004 | UT-010 | LV-004, LV-035 | CL-005, CL-006 |
| (ADR-006 меню) | UT-011 | — | CL-010 |
| AC-005 | — | LV-005, LV-006, LV-008, LV-020, LV-021 | CL-007, CL-020 |
| AC-006 | — | LV-007, LV-022 | CL-008 |
| AC-007 | — | LV-010 | CL-009 |
| AC-008 | UT-005, UT-006 | LV-011, LV-012, LV-013 | EQ-001, EQ-002, EQ-003 |
| AC-009 | UT-007 | LV-014 | EQ-004, EQ-005 |
| AC-010 | — | LV-015 | EQ-006 |
| AC-011 | UT-008 | LV-030, LV-031, LV-035 | ET-001, ET-002 |
| AC-012 | UT-009 | LV-032, LV-033, LV-034, LV-013 | ET-003, ET-004, EQ-007 |
| AC-013 | — | LV-040, LV-041 | CL-010 |
| AC-014 | — | — | CL-021, EQ-021, ET-021 |

## Unit
`internal/features/clients/validate_test.go`, `internal/features/equipmenttypes/validate_test.go`, `pkg/sqlutil/like_test.go`, `internal/ssr/shell_test.go` (table-driven, эталон `users/validate_test.go`).
| ID | Что |
|---|---|
| UT-001 | ClientInput: trim; `company_name` пусто → required, 200/201 rune (кириллица) → ok/too_long; address 500/501; contact_* 200/201 |
| UT-002 | phones: 0 после отброса → `phones` required; 20 ok / 21 too_many |
| UT-003 | строки телефонов: пустые (phone+note) отбрасываются; note без phone → `phones[i].phone` required; индекс после нормализации |
| UT-004 | phone: 32/33 → too_long; «abc» / «()-» → invalid_format; «+7 (999) 1» ok; note 200/201 |
| UT-005 | EquipmentInput: type_id пусто → required, «xyz» → invalid; model required/200/201; brand/sku/serial 200/201; note 1000/1001 |
| UT-006 | specs `nil` → `[]` (не null в JSON) |
| UT-007 | specs: пустые пары отбрасываются; v без k → `specs[i].k` key_required; k 100/101, v 500/501; 50/51 → too_many; дубли k разрешены |
| UT-008 | types Input: name required/100/101; spec_keys trim, пустые/дубли case-insensitive отброшены, порядок первого вхождения; 50/51 после дедупа |
| UT-009 | SSR-разбор `spec_keys` «a, b,,A » → `["a","b"]` |
| UT-010 | digits-нормализация поиска (ADR-003): «8 (999) 12» → `%899912%`; «ab1» → `''`; «12» (<3 цифр) → `''`; `sqlutil.EscapeLike` `%`/`_`/`\` |
| UT-011 | `Shell.NavActive`: `/` точно; `/clients` на `/clients/abc` true; `/users` на `/users-x` false; `/catalog` на `/catalog/equipment-types` true |

## Live (TD-8)
Файлы: `internal/features/clients/clients_http_live_test.go` (`ClientsLiveSuite{testutil.LiveSuite{BinaryPkg: "./cmd/server"}}`), `internal/features/equipmenttypes/equipmenttypes_http_live_test.go` (`TypesLiveSuite`). `SetupSuite`: `SeedUser` admin + executor, `NewBrowser().Login(...)`; тип/клиенты сидятся через API в тесте (уникальные имена). `make test-integration-scoped`.
| ID | Сценарий | Ожидание |
|---|---|---|
| LV-001 | POST /clients (2 телефона + пустая строка) → GET | 201; `phones` 2 в порядке ввода; `version`=1; `equipment_count`=0 |
| LV-002 | POST мусор: `{}`, `null`, неизвестное поле, `company_name` 201 rune, `phones: []` | 400 `BAD_REQUEST`/`VALIDATION` с полями; не 500 |
| LV-003 | PUT с другим набором телефонов (замена, удаление, перестановка) | 200; набор = присланный, `version`=2 |
| LV-004 | GET /clients?q=: часть компании (регистр), ФИО, телефон «8999…» против «+7 (999)…», `%`/`_` литерально; клиент с 3 совпавшими телефонами | каждый найден 1 раз; `total` без дублей; сортировка `lower(company_name)`; `limit`/`offset`/мусорный `limit` → дефолт |
| LV-005 | PUT без `version` / `"version":"x"` | 400 (`version` required / `BAD_REQUEST`) |
| LV-006 | PUT со старым `version` | 409 `CLIENT_VERSION_CONFLICT`; данные и телефоны не изменены |
| LV-007 | DELETE клиента с 2 единицами → GET клиента, GET /equipment/:id; повторный DELETE | 204; 404/404; повтор 404 |
| LV-008 | битый uuid / несуществующий id: GET/PUT/DELETE /clients/:id, /equipment/:id, /clients/:id/equipment | 404, не 500 |
| LV-010 | GET /clients/:id/equipment: 0 и 2 единицы | конверт `{items,total,limit,offset}`; `type_name` |
| LV-011 | POST /clients/:id/equipment happy → GET /clients/:id | 201; `equipment_count`=1; `clients.version` не изменился |
| LV-012 | POST без type_id / несуществующий type_id / без model | 400 поля `type_id` required/not_found, `model` required |
| LV-013 | архивный тип: POST с ним → 400 `archived`; PUT существующей единицы без смены типа → 200; PUT со сменой на архивный → 400 | как указано; GET единицы `type_is_active`=false |
| LV-014 | specs: пустые пары отброшены, v без k → 400, смена `spec_keys` типа не меняет `specs` единицы | ✓ |
| LV-015 | DELETE /equipment/:id → GET; повтор | 204 → 404; повтор 404 |
| LV-020 | конкурентность (TD-4): 2 Browser admin, одновременно PUT с одним `version` (goroutine + `sync.WaitGroup`) | ровно один 200, один 409; итог = данные победителя, `version`=2, телефоны не смешаны |
| LV-021 | SSR PRG: POST /clients (form, `_csrf`, `phone` ×2) → 303 `/clients/{id}?flash=created`; POST /clients/:id со старым version → 409 HTML содержит текст AC-005 и введённые телефоны | ✓ |
| LV-022 | SSR POST /clients/:id/delete, /equipment/:id/delete → 303; без `_csrf` → 403 | ✓ |
| LV-030 | POST/PUT /equipment-types happy; spec_keys нормализованы | 201/200 |
| LV-031 | дубль имени « Лазер » vs «лазер» (POST и PUT-переименование); дубль архивного | 409 `EQUIPMENT_TYPE_NAME_TAKEN`; для архивного — 409 `EQUIPMENT_TYPE_NAME_ARCHIVED` (ветвление по `code`) |
| LV-032 | DELETE неиспользуемого → 204; используемого → 409 `EQUIPMENT_TYPE_IN_USE`, тип цел | ✓ |
| LV-033 | PUT `is_active:false` → GET list без `include_archived` не содержит, с `include_archived=true` (admin) содержит; PUT без `is_active` → 400 | ✓ |
| LV-035 | пагинация (M-02): `/equipment-types` без `limit` → `limit`=500, 60 типов все в `items`; `limit=9999` → 500; `limit=abc`/`-1` → 500; `offset=-5` → 0. `/clients` без `limit` → 50, `limit=9999` → 200. `/clients/:id/equipment` → 200 | ✓ |
| LV-036 | мусорный body новых хендлеров → 400 `BAD_REQUEST`, message «Некорректное тело запроса» | ✓ |
| LV-034 | SSR POST /catalog/equipment-types/:id/archive|restore → 303; delete используемого → 409 HTML с кнопкой «В архив» | ✓ |
| LV-040 | executor API: GET clients/equipment/types — 200; `include_archived=true` → только активные; POST /clients/:id/equipment → 201; POST/PUT/DELETE clients, PUT/DELETE equipment, POST/PUT/DELETE types → 403 и данные не изменены | ✓ |
| LV-041 | executor SSR: /clients, /clients/:id, /equipment/:id, /catalog/equipment-types → 403 «Нет доступа»; без сессии API → 401, SSR → редирект на /login | ✓ |

## E2E (Playwright)
Файлы: `e2e/tests/clients.spec.ts`, `e2e/tests/equipment.spec.ts`, `e2e/tests/equipment-types.spec.ts`.
POM: `e2e/pages/ClientsPage.ts` (list/form/conflict/delete), `e2e/pages/EquipmentPage.ts` (форма единицы, specs-строки), `e2e/pages/EquipmentTypesPage.ts`; переиспользуют `ShellPage` (dialog `ui-dialog-ok`, combobox `combobox-dropdown`).
Фикстуры: `e2e/fixtures/auth.ts` `AdminApi` + `createClient(c)`, `updateClient(id, c)`, `getClient(id)`, `createEquipmentType(t)`, `updateEquipmentType(id, t)`, `createEquipment(clientId, e)`; типы payload — wire-типы (TD-6.2, `e2e/tsconfig.json`); имена — `uniq()`; executor — `makeUser({role:'executor'})` + `loginAs`.
| ID | Сценарий |
|---|---|
| CL-001 | создание клиента (все поля, 2 телефона + пустая строка) → PRG → карточка с flash; в списке компания и оба телефона с примечаниями |
| CL-002 | exhaustive валидация (TD-6.5): пустая компания; все телефоны пустые; 201 символ компании; телефон «abc»; телефон 33 символа — сообщение у поля, остальной ввод сохранён |
| CL-003 | исправление ошибки после 422 → сохранение проходит |
| CL-004 | добавить/удалить строки телефонов (`phone-add`/`phone-del`), последняя строка не удаляется; порядок сохранения |
| CL-005 | поиск по части компании / ФИО / телефону с другими разделителями; URL `?q=`; reload и «назад» сохраняют; «Клиенты не найдены»; «Сбросить» |
| CL-006 | список: сортировка по компании, колонка «Оборудование» = N (seed через API) |
| CL-007 | правка карточки → flash «Изменения сохранены», значения обновлены |
| CL-008 | удаление: confirm содержит «N единиц оборудования» → отмена ничего не меняет → ok → список без клиента |
| CL-009 | блок «Оборудование клиента»: пусто «Оборудования пока нет»; с единицей — модель, тип, SN; архивный тип с пометкой «(в архиве)» |
| CL-010 | executor: пунктов «Клиенты»/«Каталог» нет в меню; прямые URL `/clients`, `/catalog/equipment-types`, `/equipment/:id` → «Нет доступа» |
| CL-011 | XSS: `<img src=x onerror=…>` в компании/телефоне/характеристике отображается текстом (список, карточка, confirm) — образец фазы 1 |
| CL-020 | конкурентность (TD-4): 2 browser-контекста открывают карточку; A сохраняет; B сохраняет → текст AC-005, ввод B сохранён (вкл. телефоны); «Открыть актуальную версию» → значения A; B сохраняет повторно → успех |
| CL-021 | adaptive:min + themes: `/clients` и карточка на 360×640 и 1280×800 — `scrollWidth <= clientWidth`; тёмная тема — страница рендерится без overflow |
| EQ-001 | новая единица из карточки: тип через combobox-поиск, модель, SN → PRG в карточку, единица в блоке |
| EQ-002 | валидация: без типа / без модели / 201 символ модели — сообщения у полей, ввод сохранён |
| EQ-003 | правка единицы → карточка клиента с flash; значения обновлены |
| EQ-004 | выбор типа подставляет строки шаблона; смена типа: заполненные сохраняются, пустые заменены шаблоном нового |
| EQ-005 | своя характеристика вне шаблона, удаление строки, «v без k» → «Укажите характеристику»; пустые строки не сохраняются |
| EQ-006 | удаление единицы через confirm → исчезает из карточки |
| EQ-007 | архивный тип не предлагается для новой единицы; у существующей показан «(в архиве)» и сохраняется без смены |
| EQ-021 | adaptive:min: форма единицы 360/1280 без overflow; строки характеристик переносятся |
| ET-001 | создание типа (name + keys через запятую с дублями/пробелами) → список: название, ключи, «0» единиц |
| ET-002 | дубль названия (регистр/пробелы) → «Такой тип уже есть»; дубль архивного → «…в архиве — верните его из архива» + ссылка «Показать архивные» |
| ET-003 | удаление неиспользуемого (confirm) → исчез; используемого → сообщение + «В архив» → в архиве |
| ET-004 | «Показать архивные» (URL `?archived=1`, reload сохраняет); «Вернуть» → тип снова активен |
| ET-021 | adaptive:min: `/catalog/equipment-types` 360/1280 без overflow |

## Mock Strategy
Моков нет: repository/service — через живую БД (BaseSuite/LiveSuite, dockertest PG18 + Redis); внешних клиентов нет.

## Fuzz (TD-7)
Класс 9: n/a — tier M, `risk=reversible`, по запросу не заказан. `pw-fuzz` — вердикт frontend-architect (не пересуживается здесь).

## Test Data / Helpers
- Go: хелперы в live-сьюте — `createType(name, keys)`, `createClient(company, phones...)`, `createEquipment(clientID, typeID, model)` через `Browser.Do`.
- Сид миграции/фикстуры: схема 0003 + фикстуры `AdminApi` — в той же фазе (TD-1, `fixture_sync_check.py`).
- Не тестируем: D26 (фаза 5), нагрузку, миграцию `specs` при смене шаблона (по дизайну не мигрирует — проверяется LV-014).

## Источник: 3-design/design-sql.md [full]
---
stage: 3-design
feature: phase-3-clients
tier: M
---
# Design SQL: phase-3-clients

Конвенции проекта (as-is): SQL — строковые константы в `repo.go` пакета фичи (не go:embed, эталон `internal/features/users/repo.go`); id — `DEFAULT gen_random_uuid()` (pgcrypto из 0001); параметр id — `$1::uuid` (сервис уже провалидировал `uuid.Validate`). Миграции — golang-migrate, применение только `make migrate-up`.

Baseline (затрагивается только ссылкой): `users` (0001/0002) — без изменений.

## Clients Migration
`migrations/0003_clients_equipment.up.sql`:
```sql
CREATE TABLE clients (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    company_name     text NOT NULL CHECK (btrim(company_name) <> ''),
    address          text NOT NULL DEFAULT '',
    contact_name     text NOT NULL DEFAULT '',
    contact_position text NOT NULL DEFAULT '',
    version          bigint NOT NULL DEFAULT 1,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE client_phones (
    id        uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id uuid NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    position  smallint NOT NULL,
    phone     text NOT NULL CHECK (phone ~ '[0-9]'),
    note      text NOT NULL DEFAULT '',
    UNIQUE (client_id, position)          -- порядок ввода; индекс покрывает FK client_id
);

CREATE TABLE equipment_types (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name       text NOT NULL CHECK (btrim(name) <> ''),
    spec_keys  text[] NOT NULL DEFAULT '{}',
    is_active  boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
-- ADR-007: единая уникальность, архивные включены
CREATE UNIQUE INDEX equipment_types_name_lower_uq ON equipment_types (lower(btrim(name)));

CREATE TABLE equipment (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id  uuid NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    type_id    uuid NOT NULL REFERENCES equipment_types(id) ON DELETE RESTRICT,
    brand      text NOT NULL DEFAULT '',
    model      text NOT NULL CHECK (btrim(model) <> ''),
    sku        text NOT NULL DEFAULT '',
    serial     text NOT NULL DEFAULT '',
    specs      jsonb NOT NULL DEFAULT '[]' CHECK (jsonb_typeof(specs) = 'array'),
    note       text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX equipment_client_id_idx ON equipment (client_id);
CREATE INDEX equipment_type_id_idx   ON equipment (type_id);   -- RESTRICT-проверка и equipment_count типа
```
`migrations/0003_clients_equipment.down.sql` (CM-1 MVP — простой DROP):
```sql
DROP TABLE IF EXISTS equipment;
DROP TABLE IF EXISTS equipment_types;
DROP TABLE IF EXISTS client_phones;
DROP TABLE IF EXISTS clients;
```
CHECK-и — страховка на уровне БД; основная валидация и тексты — Go (`design-errors.md`). Длины — только в Go (rune-лимиты Tech Spec).

## Clients Table
Колонки — `## Clients Migration`. `version` — только clients (D28). `equipment_count` — вычисляемое в запросах, не колонка.

## Equipment Types Table
`spec_keys text[]` — порядок сохраняется, дедуп/trim — Go. `is_active=false` = архив.

## Equipment Table
`specs` — `[{"k":"…","v":"…"}]` как прислано после нормализации (ADR-002/AC-009); смена `spec_keys` типа не мигрирует.

## List Clients Query
Где: `clients.Repo.ListClients(ctx, ListParams{Q, Digits, Limit, Offset})`. `$1` = `'%'||escapeLike(q)||'%'`, `$2` = `'%'||digits||'%'` либо `''` (ADR-003: digits заполняется, если в q ≥3 цифр и q ~ `^[0-9+()\- ]+$`). Без q — WHERE опускается.
```sql
-- where (при q):
WHERE (c.company_name ILIKE $1 OR c.contact_name ILIKE $1
       OR EXISTS (SELECT 1 FROM client_phones p
                  WHERE p.client_id = c.id
                    AND (p.phone ILIKE $1
                         OR ($2 <> '' AND regexp_replace(p.phone, '\D', '', 'g') LIKE $2))))

SELECT count(*) FROM clients c <where>;

SELECT c.id, c.company_name, c.address, c.contact_name, c.contact_position,
       c.version, c.created_at, c.updated_at,
       (SELECT count(*) FROM equipment e WHERE e.client_id = c.id) AS equipment_count
FROM clients c <where>
ORDER BY lower(c.company_name), c.id
LIMIT $3 OFFSET $4;
```
`escapeLike` → вынести из `users/repo.go` в `pkg/sqlutil.EscapeLike` (3 потребителя: users, clients, equipmenttypes — правило «2+ фичи → pkg/»); `users` переводится на неё без смены поведения.

## Client Phones Batch Query
Телефоны страницы списка — один запрос (без N+1):
```sql
SELECT client_id, phone, note FROM client_phones
WHERE client_id = ANY($1::uuid[]) ORDER BY client_id, position;
```

## Get Client Query
```sql
SELECT c.id, c.company_name, c.address, c.contact_name, c.contact_position,
       c.version, c.created_at, c.updated_at,
       (SELECT count(*) FROM equipment e WHERE e.client_id = c.id)
FROM clients c WHERE c.id = $1::uuid;
-- + Client Phones Batch Query с $1 = [id]
```

## Create Client Query
Транзакция:
```sql
INSERT INTO clients (company_name, address, contact_name, contact_position)
VALUES ($1, $2, $3, $4)
RETURNING id, company_name, address, contact_name, contact_position, version, created_at, updated_at;

INSERT INTO client_phones (client_id, position, phone, note)
SELECT $1::uuid, t.ord - 1, t.phone, t.note
FROM unnest($2::text[], $3::text[]) WITH ORDINALITY AS t(phone, note, ord);
```

## Update Client Query
Транзакция (ADR-100):
```sql
UPDATE clients SET company_name = $2, address = $3, contact_name = $4, contact_position = $5,
       version = version + 1, updated_at = now()
WHERE id = $1::uuid AND version = $6
RETURNING id, company_name, address, contact_name, contact_position, version, created_at, updated_at;
-- 0 строк (pgx.ErrNoRows):
SELECT EXISTS (SELECT 1 FROM clients WHERE id = $1::uuid);   -- true → errVersionConflict, false → ErrNotFound

DELETE FROM client_phones WHERE client_id = $1::uuid;
-- + INSERT client_phones из Create Client Query
```
`equipment_count` в ответе PUT — отдельным `SELECT count(*) FROM equipment WHERE client_id=$1` внутри tx.

## Delete Client Query
```sql
DELETE FROM clients WHERE id = $1::uuid;   -- RowsAffected()==0 → ErrNotFound; каскад: client_phones, equipment
```
D26 («клиент с заказами») — фаза 5.

## List Equipment By Client Query
```sql
SELECT count(*) FROM equipment WHERE client_id = $1::uuid;

SELECT e.id, e.client_id, e.type_id, t.name, t.is_active, e.brand, e.model, e.sku, e.serial,
       e.specs, e.note, e.created_at, e.updated_at
FROM equipment e JOIN equipment_types t ON t.id = e.type_id
WHERE e.client_id = $1::uuid
ORDER BY e.created_at, e.id
LIMIT $2 OFFSET $3;
```
Сервис до запроса проверяет существование клиента (`SELECT EXISTS … clients`) → 404 `CLIENT_NOT_FOUND` вместо пустого списка.

## Get Equipment Query
`SELECT <те же колонки> FROM equipment e JOIN equipment_types t ON t.id = e.type_id WHERE e.id = $1::uuid;`

## Equipment Type Check Query
Валидация `type_id` (ADR-004):
```sql
SELECT is_active FROM equipment_types WHERE id = $1::uuid;  -- ErrNoRows → type_id: not_found; false → type_id: archived
```
Update: проверка `archived` только если `type_id` ≠ текущий (`SELECT type_id FROM equipment WHERE id=$1::uuid` в той же tx).

## Create Equipment Query
```sql
INSERT INTO equipment (client_id, type_id, brand, model, sku, serial, specs, note)
VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7::jsonb, $8)
RETURNING id;
-- затем Get Equipment Query (type_name/type_is_active)
```
23503 на `client_id` (клиент удалён параллельно) → `ErrNotFound` клиента → 404 `CLIENT_NOT_FOUND`.

## Update Equipment Query
```sql
UPDATE equipment SET type_id = $2::uuid, brand = $3, model = $4, sku = $5, serial = $6,
       specs = $7::jsonb, note = $8, updated_at = now()
WHERE id = $1::uuid RETURNING id;   -- ErrNoRows → EQUIPMENT_NOT_FOUND; LWW (D28)
```
`client_id` не меняется (перенос единицы между клиентами — вне скоупа).

## Delete Equipment Query
```sql
DELETE FROM equipment WHERE id = $1::uuid RETURNING client_id;   -- ErrNoRows → EQUIPMENT_NOT_FOUND; client_id → PRG-редирект
```
D26 («было в заказе») — фаза 5.

## Equipment Type Options Query
Опции `<select>` формы единицы (из `clients.Repo`, ADR-102):
```sql
SELECT id, name, spec_keys, is_active FROM equipment_types
WHERE is_active OR id = $1::uuid          -- $1 = текущий type_id единицы; для новой — NULL
ORDER BY lower(name), id;
```

## List Equipment Types Query
`equipmenttypes.Repo.List(ctx, ListParams{Q, IncludeArchived, Limit, Offset})`:
```sql
-- where: [NOT include_archived → is_active] [q → name ILIKE $n]
SELECT count(*) FROM equipment_types <where>;
SELECT t.id, t.name, t.spec_keys, t.is_active, t.created_at, t.updated_at,
       (SELECT count(*) FROM equipment e WHERE e.type_id = t.id) AS equipment_count
FROM equipment_types t <where>
ORDER BY lower(t.name), t.id LIMIT $n OFFSET $m;
```

## Get Equipment Type Query
Та же проекция `WHERE t.id = $1::uuid`.

## Create Equipment Type Query
```sql
INSERT INTO equipment_types (name, spec_keys) VALUES ($1, $2::text[])
RETURNING id, name, spec_keys, is_active, created_at, updated_at;
-- 23505 (equipment_types_name_lower_uq) → вне tx:
SELECT is_active FROM equipment_types WHERE lower(btrim(name)) = lower(btrim($1));  -- true → errNameTaken, false → errNameArchived (409 `EQUIPMENT_TYPE_NAME_ARCHIVED`)
```

## Update Equipment Type Query
```sql
UPDATE equipment_types SET name = $2, spec_keys = $3::text[], is_active = $4, updated_at = now()
WHERE id = $1::uuid RETURNING id, name, spec_keys, is_active, created_at, updated_at;
-- ErrNoRows → EQUIPMENT_TYPE_NOT_FOUND; 23505 → как в Create (строка-конфликт ≠ id)
```

## Set Equipment Type Active Query
SSR archive/restore (без перезаписи имени):
```sql
UPDATE equipment_types SET is_active = $2, updated_at = now() WHERE id = $1::uuid;   -- RowsAffected 0 → NOT_FOUND
```

## Delete Equipment Type Query
```sql
DELETE FROM equipment_types WHERE id = $1::uuid;
-- RowsAffected 0 → EQUIPMENT_TYPE_NOT_FOUND; pgconn 23503 (equipment_type_id_fkey, RESTRICT) → EQUIPMENT_TYPE_IN_USE
```

## Паспорт (выжимка)
# Passport: phase-3-clients (generated — руками не править; stage_bundle.py)
**Tier:** M | **Совместимость:** ? (pipeline-doctrine.md §CM-1; ? → уточни у оркестратора)
**Трек:** обычный (prefs нет — дефолт) — режимы агентов (`spec-track delta/micro/mini/merged`) включает ТОЛЬКО строка `Режим:` в prompt оркестратора; DS/TS-индексы ниже присутствуют на ЛЮБОМ треке при наличии draft-spec.
**Layout:** автодетект (tools/layout_resolve.py)
**UI-scope (pipeline-doctrine.md §US-1, 0-setup/ui-scope-prefs.md):** swagger:no | adaptive:min | themes:both

**Индексы:** AC 14 / DS 0 / TS 0 — полные списки: `0-setup/passport.md` (разрешён контрактом); конкретное решение — точечный grep по `DS-NNN`/`TS-NNN` в draft-spec §9/§11 (разрешён).
