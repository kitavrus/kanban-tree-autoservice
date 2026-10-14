# Phase Bundle: phase-3-clients / Phase 01 (frontend) — generated 2026-09-25 21:00

> Твой вход — ЭТОТ файл. Upstream-доки (research/prd/design-*) напрямую НЕ читать —
> нужные слайсы дизайна ниже. Код фичи и её тесты — читать/писать можно (твоя фаза).

## Фаза плана (6-plan/code-plan-phase-01-shared-guard.md)
### Phase 1: Shared regression guard

**Layer:** backend
**Commit**: `refactor(shared): вынос EscapeLike в pkg/sqlutil, Shell.NavActive, меню «Каталог»`

Риск: код используется существующими `users`/`shell` e2e — ломать нельзя (регресс-гейт AC-006 меню, UT-011).

**Files to CREATE:**
- `pkg/sqlutil/like.go` — `EscapeLike(s string) string` (экранирование `%`/`_`/`\` для ILIKE), перенос логики из `internal/features/users/repo.go`.
- `pkg/sqlutil/like_test.go` — `TestEscapeLike_SpecialChars` (UT-010, часть про `%`/`_`/`\`).

**Files to MODIFY:**
- `internal/features/users/repo.go` — заменить локальную реализацию escape на `sqlutil.EscapeLike`, поведение не меняется.
- `internal/ssr/shell.go` — добавить `func (s Shell) NavActive(url string) bool` (ADR-006: `url=="/"` точное совпадение, иначе `ActivePath==url || strings.HasPrefix(ActivePath, url+"/")`).
- `internal/ssr/shell_test.go` — `TestShell_NavActive` (UT-011: `/` точно; `/clients` на `/clients/abc` true; `/users` на `/users-x` false; `/catalog` на `/catalog/equipment-types` true).
- `internal/ssr/menu.go` — пункт «Услуги и товары» → `{Title: "Каталог", Icon: "🧰", URL: "/catalog", AdminOnly: true}` (ADR-006).
- `web/ssr/_layout/templates/layouts/base.html` — `{{ if $.Shell.NavActive .URL }} active{{ end }}` вместо `eq .URL $.Shell.ActivePath`; `aria-current` сохранить.
- `internal/ssr/render_test.go` — обновить снапшот/ассерт меню, если завязан на старый пункт «Услуги и товары».

**Run after:**
```bash
make build
```

**Tests in this phase:**
- Unit: `TestEscapeLike_SpecialChars`, `TestShell_NavActive`.

**Definition of Done:**
- [ ] `make build` — OK
- [ ] `make test` — unit OK (`pkg/sqlutil`, `internal/ssr`, `internal/features/users`)
- [ ] `make lint` — 0 ошибок
- [ ] Регресс: `cd e2e && npx playwright test tests/shell.spec.ts tests/users.spec.ts --project=chromium --headed --workers=1` зелёный (подсветка `/users`/`/` не сломана, ADR-006 риск) — evidence TD-2
- [ ] Тесты этой фазы зелёные до перехода (TD-1)


## Слайс: 3-design/design-frontend-architecture.md [full]
MISSING

## Слайс: 3-design/design-go-layers.md [sections]
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

## Слайс: 3-design/output.md [sections]
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

## Слайс: 3-design/design-errors.md [full]
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

## Слайс: 3-design/design-e2e-strategy.md [head]
MISSING

## Слайс: 1-research/output.md [sections]
## Frontend (SSR-слой, D30 — Vue отсутствует, это фронтенд проекта)
- Существующие SSR-фичи: `web/ssr/users/templates/pages/list.html`, `web/ssr/users/templates/pages/form.html` (список+PRG-форма), `web/ssr/auth/templates/pages/login.html`, `web/ssr/home/templates/pages/index.html`, общие `web/ssr/_layout/templates/layouts/base.html`, `web/ssr/_layout/templates/layouts/bare.html`, `web/ssr/_layout/templates/pages/status.html`.
- `web/ssr/_layout/templates/layouts/base.html:54-58` — порядок подключения JS: `bootstrap.bundle.min.js`→`theme.js`→`api.js`→`ui-dialog.js`→`combobox.js`.
- `web/ssr/_shared/js/ui-dialog.js:81-84` — `KT.uiDialog/uiPrompt/uiConfirm/uiAlert`, Bootstrap Modal, `data-testid="ui-dialog"`/`ui-dialog-ok`/`ui-dialog-cancel`/`ui-dialog-input` — замена нативных prompt/confirm/alert прототипа (`prototype/index.html:1381-1383`).
- `web/ssr/_shared/js/combobox.js:26-157` — автообогащение ЛЮБОГО `select.form-select` (кроме `[multiple]`/`[data-native]`) в поисковый комбобокс (D6), `data-testid="combobox-dropdown"`, работает внутри открытой Bootstrap-модалки (combobox.js:69-70) — аналог `enhanceSelect` прототипа.
- Нет генерического JS для динамических списков (телефоны 1..N, характеристики N) — прототип использует ручной JS `drawPhones()`/`drawSpecs()` (`prototype/index.html:1352-1361`, `prototype/index.html:1413-1421`); в SSR-стеке проекта такого паттерна ещё нет — писать заново (chistый JS по образцу прототипа или AJAX-фрагмент `r.Fragment`).
- Переиспользуемый образец карточки/PRG-формы — `internal/features/users/ssr.go:1` целиком; общая 403/404-страница — `web/ssr/_layout/templates/pages/status.html:1`.
- Нормативных Vue-компонентов (`components/ui/`, StatusBadge и т.п.) в проекте НЕТ — не тот стек (SSR/Bootstrap, не Vue+Element Plus).
- Стек: vendored Bootstrap 5.3 (`web/ssr/_shared/vendor/bootstrap/bootstrap.min.css:1`, `web/ssr/_shared/vendor/bootstrap/bootstrap.bundle.min.js:1`), без CDN/Vue/React (D38); токены — `web/ssr/_shared/app.css:1` (primary `#2563EB`); тёмная тема — `data-bs-theme` через `web/ssr/_shared/js/theme.js:1` (US-1 `themes:both`).

### Расхождение с `/z-scaffold` (~/.claude/skills/z-scaffold/SKILL.md — вход для решения scaffold vs вручную)
1. z-scaffold штампует СВОЙ `layouts/base.html` на фичу → конфликт с общим `web/ssr/_layout/templates/layouts/base.html` (два `{{define "base"}}`) — обязателен к удалению (рецепт `internal/ssr/render.go:13-19`).
2. Сгенерированный `BaseVM` — БЕЗ поля `Shell` — добавляется руками в `models/dto/view_model.go:1` (z-scaffold-путь), иначе общий layout не отрендерит сайдбар/тему/CSRF.
3. z-scaffold разбивает на пакеты `handler/`(по файлу на действие)+`mappers/helpers.go:1`+`validators/form.go:1` — РАСХОДИТСЯ с фактическим паттерном: `internal/features/users` — один плоский пакет (`dto.go`/`handler.go`/`service.go`/`repo.go`/`ssr.go`/`validate.go`/`routes.go`), без под-пакетов.
4. Путь `web/ssr/{feature}/templates` — СОВПАДАЕТ (`internal/features/users/ssr.go:14`) — единственный несовпадающий с расхождением пункт.
5. z-scaffold заворачивает ошибки сервиса (кроме валидации формы) в `fiber.ErrInternalServerError`/`ErrNotFound` — расходится с `formError()` (users/ssr.go:136-156), которая через `errors.As` вытаскивает произвольные бизнес-коды (409 и т.п.) — доработка руками, если используется scaffold.
6. Пагинация z-scaffold — классическая нумерованная; фактический паттерн — простой счётчик `Total`/`Items` без номеров страниц (`internal/features/users/ssr.go:29-37`, `web/ssr/users/templates/pages/list.html:41`).
7. Динамические JSONB-характеристики оборудования по внешнему шаблону (`equipment_types.spec_keys`) — вне штатных типов полей манифеста z-scaffold — ручная доработка формы в любом случае.

**Для Design:** зафиксировать явный выбор — плоский пакет (как users) или разбитые z-scaffold-пакеты; в обоих случаях 3 шага D38-рецепта обязательны, а JSONB-характеристики дорабатываются руками.

### Contracts
- Защищённые: весь `/api/v1/users*` (RequireAPI+RequireAdmin), SSR `/users*` (RequireSSR+RequireAdmin) — по аналогии `/clients`/`/catalog` тоже `AdminOnly:true` в `internal/ssr/menu.go:25-26`.
- Публичные: `/health`, `/ready`, `/metrics`, `/login`, `/assets/ssr-shared/*`.
- CSRF — на все мутации всех маршрутов включая `/api/v1` (`internal/server/middleware/csrf.go:30-63` — глобально).
- `.env` — новых переменных фаза 3 не требует (DATABASE_URL/REDIS_ADDR/PORT уже покрывают).
- Rate limiting — только login (D41), вне зоны фичи.
- Не-REST протоколы — отсутствуют.

**Enum coverage:** US-1 `swagger:no` → §API-1/§8.1 n/a, swag-аннотации не пишутся. DB CHECK-образец: `users.role CHECK (role IN ('admin','executor'))` (migrations/0002_users_roles.up.sql:4). Целевая схема фазы 3 (clients/client_phones/equipment_types/equipment) enum-полей НЕ содержит — только text/boolean `is_active`/`TEXT[]`/`JSONB`.

### Error responses (образец формата)
```json
{"code":"VALIDATION","message":"validation failed","details":{"fields":[{"name":"login","message":"required"}]}}
{"code":"LOGIN_TAKEN","message":"Логин уже занят"}
```
Источник: `pkg/errorspkg/errors.go:46-76`. 401/403/404/500 — тот же `{code,message}` без `details`.

### Infrastructure
- Docker Compose — `postgres:18`+`redis:8` уже подняты фазой 1 (не читалось построчно — `touches.infra=false`); фаза 3 новых сервисов не добавляет.
- CI/CD — не обнаружено ни одного файла CI в `git ls-files` (нет `.gitlab-ci.yml`/`.github/workflows`).
- Dockerfile — не обнаружен в `git ls-files`.
- Внешние сервисы — Redis (сессии, `internal/session/store.go:9`), Postgres 18; новых не вводит.
- Secrets — `.env.example`, локальная разработка (CM-1=MVP, прода нет).
- Мониторинг/логирование — `internal/observability/logger.go:9` (slog `NewLogger`), `internal/observability/metrics.go:25` (Prometheus `/metrics`, `internal/server/router.go:58`), `internal/observability/otel.go:18` (`InitTracer`, `cmd/server/main.go:38`) — фаза 3 новых метрик не требует.

### Tests
- Unit: `internal/features/users/validate_test.go:1`, `internal/server/middleware/csrf_test.go:1`, `internal/ssr/render_test.go:1` — обычный `go test`.
- Integration (§TD-8, тег `integration`): `pkg/testutil/base_suite.go:22` — `BaseSuite` (dockertest PG18+Redis, миграции применены, `SeedUser`); `pkg/testutil/live_suite.go:20` — `LiveSuite` (боевой бинарь `go build ./cmd/server`, реальный TCP, `/health`-опрос); `pkg/testutil/login.go:30` — `Browser` (свой cookie jar, редиректы не следуются, `CSRFToken()`/`Request()`/`Do()`/`Login()` через реальный `/api/v1/auth/login`, auth-стабы запрещены).
  - Образец: `internal/features/users/users_http_live_test.go:20-27` — `UsersLiveSuite{LiveSuite{BinaryPkg:"./cmd/server"}}`, покрывает happy-path/authz/валидацию-мусор→400/дубликат→409/конкурентность(`LastAdminSuite`, гонка на `FOR UPDATE`)/SSR PRG.
  - `internal/features/auth/auth_http_live_test.go:1`, `internal/server/shell_http_live_test.go:1` — тот же паттерн.
  - `app.Test` НЕ используется нигде в существующих тестах — соответствует запрету §TD-8 на новые `app.Test`-сьюты.
- Vue-тестов нет (Vue отсутствует). E2E — Playwright:
  - `e2e/playwright.config.ts:7-22` — `projects:[chromium, devices['Desktop Chrome']]`, `workers:1`, `fullyParallel:false`, `PWSLOWMO` env (§TD-3/§TD-5 соблюдены).
  - `e2e/fixtures/test.ts:24-92` — `allowHttpErrors` (auto, ловит console.error/pageerror/HTTP≥400, §TD-4), `uniq`, `makeUser`, `loginAs`, `adminPage`, `runAdmin`/`adminApi` (worker-scope).
  - `e2e/fixtures/auth.ts:32-82` — `loginContext()` (реальный вход, csrf из cookie `kt_csrf`), `AdminApi` (сидирование данных), `SEED_ADMIN={login:'admin',password:'admin1234'}`, `RUN` — метка прогона (не литерал в ассертах, §TD-6.2).
  - `e2e/pages/ShellPage.ts:5-75` — POM общего layout (sidebar/theme/dialog/combobox, §TD-6.1); `e2e/pages/UsersPage.ts:4-77` — образец экрана: полноценные страницы `/users/new`,`/users/:id` (PRG), НЕ модалки — расходится с прототипом, где карточка клиента/оборудования — Bootstrap-модалка.
  - `e2e/tests/users.spec.ts:8-233` — образец глубины TD-6: happy path (US-001), exhaustive-валидация (US-002, TD-6.5), занятый логин (US-003), правка (US-004/005), блокировка+фильтры-в-URL (US-010..013), конкурентность last-write-wins (US-020, D28-для-users), адаптив 360/1280px без overflow (US-021, `adaptive:min`).
- Coverage — `coverage.out`/threshold не обнаружены.

## Паттерны в коде
- Один плоский Go-пакет на фичу (`internal/features/{feature}`), НЕ разбит на handler/mappers/validators (расходится с z-scaffold SSR).
- Sentinel `*errorspkg.AppError` пакетными переменными, маппинг pg-ошибок→доменные в репозитории, матчинг в сервисе `errors.Is`.
- Ручная field-валидация кодами сообщений (`msgRequired` и т.п.), не текстом — SSR транслирует код в текст.
- SSR VM обязательно несёт `Shell ssr.Shell`, заполняется `shell.Build(c,title)` перед рендером (D38).
- PRG для форм: `POST /{feature}` → `303 See Other` → `?flash=created`; ошибка формы — тот же метод, соответствующий HTTP-код, форма перерисовывается с `vm.Errors`.
- Whitelist для пользовательского ввода, идущего в SQL как идентификатор (сортировка/фильтр-роль) — тихий откат к дефолту, не 500.
- `escapeLike()` — экранирование ILIKE-паттернов при поиске по подстроке.
- Права читаются из БД на каждый запрос, не кэшируются в сессии.
- Все HTTP-интеграционные тесты — через `testutil.LiveSuite`, НЕ `app.Test` (§TD-8).
- E2E — POM (`e2e/pages/*Page.ts`) + `data-testid` на все интерактивные элементы; `ShellPage` переиспользуется каждой фичей.
- UUID/ID: единой Go-точки генерации (`pkg/ids`) НЕТ — генерация только в БД `DEFAULT gen_random_uuid()`; Go лишь валидирует уже присвоенный uuid (`uuid.Validate`, users/service.go:94,137). Новые таблицы следуют тому же паттерну.

## Слайс: AC-строки
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

## Паспорт (выжимка)
# Passport: phase-3-clients (generated — руками не править; stage_bundle.py)
**Tier:** M | **Совместимость:** ? (pipeline-doctrine.md §CM-1; ? → уточни у оркестратора)
**Трек:** обычный (prefs нет — дефолт) — режимы агентов (`spec-track delta/micro/mini/merged`) включает ТОЛЬКО строка `Режим:` в prompt оркестратора; DS/TS-индексы ниже присутствуют на ЛЮБОМ треке при наличии draft-spec.
**Layout:** автодетект (tools/layout_resolve.py)
**UI-scope (pipeline-doctrine.md §US-1, 0-setup/ui-scope-prefs.md):** swagger:no | adaptive:min | themes:both

**Индексы:** AC 14 / DS 0 / TS 0 — полные списки: `0-setup/passport.md` (разрешён контрактом); конкретное решение — точечный grep по `DS-NNN`/`TS-NNN` в draft-spec §9/§11 (разрешён).
