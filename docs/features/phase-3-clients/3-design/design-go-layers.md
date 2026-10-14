---
stage: 3-design
feature: phase-3-clients
tier: M
---
# Design Go layers: phase-3-clients

Эталон — плоский пакет `internal/features/users` (без под-пакетов): `dto.go` / `validate.go` / `repo.go` / `service.go` / `handler.go` / `ssr.go` / `routes.go`. Сервис принимает `*pgxpool.Pool`, строит свой `Repo`. Ошибки — `*errorspkg.AppError` (design-errors.md). swagger:no — swag-аннотаций нет.

## Файлы
| Путь | Содержимое |
|---|---|
| `migrations/0003_clients_equipment.{up,down}.sql` | design-sql.md |
| `internal/features/equipmenttypes/{dto,validate,repo,service,handler,ssr,routes}.go` | типы оборудования |
| `internal/features/equipmenttypes/validate_test.go` | unit |
| `internal/features/equipmenttypes/equipmenttypes_http_live_test.go` | live (TD-8) |
| `internal/features/clients/{dto,validate,repo,service,handler,ssr,routes}.go` | клиенты + телефоны + оборудование |
| `internal/features/clients/validate_test.go` | unit |
| `internal/features/clients/clients_http_live_test.go` | live (TD-8) |
| `web/ssr/clients/templates/pages/{list,form,equipment-form}.html` | SSR |
| `web/ssr/equipmenttypes/templates/pages/{list,form}.html` | SSR |
| `web/ssr/_shared/js/row-list.js` | NEW, ADR-002 |
| `web/ssr/_shared/js/ui-dialog.js` | MODIFY: `form[data-confirm]` (ADR-103) |
| `web/ssr/_layout/templates/layouts/base.html` | MODIFY: `NavActive` (ADR-006) |
| `web/ssr/_shared/app.css` | MODIFY: `.row-list` строки (flex, wrap на ≤576px — adaptive:min) |
| `internal/ssr/menu.go`, `internal/ssr/shell.go` (+ `shell_test.go`/`render_test.go`) | MODIFY: «Каталог», `NavActive` |
| `internal/server/router.go` | MODIFY: wiring |
| `pkg/sqlutil/like.go` (+ `like_test.go`) | NEW: `EscapeLike` (из users) |
| `internal/features/users/repo.go` | MODIFY: `sqlutil.EscapeLike` |

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

## Validators
Нормализация + валидация — методы `Input` в `validate.go` (паттерн `users.Input.normalize()/validate()`), общие для API и SSR. Длины — `utf8.RuneCountInString`. Лимиты — Tech Spec §Лимиты (константы в `validate.go`).

`clients.ClientInput{CompanyName, Address, ContactName, ContactPosition string; Phones []PhoneDTO}`:
- `normalize()`: trim всех строк; строки телефонов с пустыми `phone` И `note` отбрасываются.
- `validate() []errorspkg.Field`: `company_name` required/≤200; `address` ≤500; `contact_*` ≤200; `phones` len 0 → required, >20 → too_many; на строку: `phone` пусто → required, >32 → too_long, нет цифры (`[0-9]`) → invalid_format; `note` ≤200.

`clients.EquipmentInput{TypeID, Brand, Model, SKU, Serial, Note string; Specs []SpecDTO}`:
- `normalize()`: trim; пары с пустыми `k` И `v` отбрасываются; `nil` specs → `[]`.
- `validate()`: `type_id` пусто → required, `uuid.Validate`≠nil → invalid; `model` required/≤200; `brand/sku/serial` ≤200; `note` ≤1000; `specs` >50 → too_many; на пару: `k` пусто при `v` → key_required, `k` >100, `v` >500 → too_long. Дубли `k` разрешены.
- Активность/существование типа — сервис (БД), не validate.

`equipmenttypes.Input{Name string; SpecKeys []string; IsActive bool}`:
- `normalize()`: trim `name`; `spec_keys` — trim, пустые и дубли (`strings.ToLower`) отбрасываются молча, порядок первого вхождения.
- `validate()`: `name` required/≤100; `spec_keys` >50 → too_many; ключ >100 → too_long (`spec_keys[i]`).
- SSR-форма типа: `spec_keys` — одна строка через запятую (эталон прототипа) → `strings.Split(",")` → `normalize()`.

## Repository Interface
Конкретные структуры `*Repo` (как users, без интерфейсов). SQL — design-sql.md.
```go
// clients
func NewRepo(pool *pgxpool.Pool) *Repo
func (r *Repo) ListClients(ctx context.Context, p ListParams) ([]Client, int, error)         // + phones batch
func (r *Repo) GetClient(ctx context.Context, id string) (*Client, error)                    // ErrNotFound
func (r *Repo) CreateClient(ctx context.Context, in ClientInput) (*Client, error)            // tx
func (r *Repo) UpdateClient(ctx context.Context, id string, version int64, in ClientInput) (*Client, error) // tx; ErrNotFound | errVersionConflict
func (r *Repo) DeleteClient(ctx context.Context, id string) error                             // ErrNotFound
func (r *Repo) ClientExists(ctx context.Context, id string) (bool, error)
func (r *Repo) ListEquipment(ctx context.Context, clientID string, limit, offset int) ([]Equipment, int, error)
func (r *Repo) GetEquipment(ctx context.Context, id string) (*Equipment, error)
func (r *Repo) TypeActive(ctx context.Context, typeID string) (active bool, err error)       // ErrNotFound
func (r *Repo) CreateEquipment(ctx context.Context, clientID string, in EquipmentInput) (*Equipment, error) // errClientGone
func (r *Repo) UpdateEquipment(ctx context.Context, id string, in EquipmentInput, check func(curTypeID string) error) (*Equipment, error) // tx
func (r *Repo) DeleteEquipment(ctx context.Context, id string) (clientID string, err error)
func (r *Repo) TypeOptions(ctx context.Context, currentTypeID string) ([]TypeOption, error)   // ADR-004

// equipmenttypes
func NewRepo(pool *pgxpool.Pool) *Repo
func (r *Repo) List(ctx context.Context, p ListParams) ([]Type, int, error)
func (r *Repo) Get(ctx context.Context, id string) (*Type, error)
func (r *Repo) Create(ctx context.Context, in Input) (*Type, error)           // errNameTaken | errNameArchived
func (r *Repo) Update(ctx context.Context, id string, in Input) (*Type, error)
func (r *Repo) SetActive(ctx context.Context, id string, active bool) error
func (r *Repo) Delete(ctx context.Context, id string) error                   // ErrNotFound | errInUse
```
Модели (repo-типы): `Client{ID, CompanyName, Address, ContactName, ContactPosition string; Phones []PhoneDTO; EquipmentCount int; Version int64; CreatedAt, UpdatedAt time.Time}`, `Equipment{… TypeName string; TypeIsActive bool; Specs []SpecDTO …}`, `TypeOption{ID, Name string; SpecKeys []string; IsActive bool}`, `Type{…; EquipmentCount int}`. `specs` — `json.Marshal` → `$n::jsonb`; скан в `[]byte` → `json.Unmarshal`.

## Service Interface
```go
// clients
func NewService(pool *pgxpool.Pool) *Service
func (s *Service) List(ctx context.Context, q ListQuery) (*ClientListResponse, error)   // limit default 50 / max 200; offset ≥0; digits (ADR-003)
func (s *Service) Get(ctx context.Context, id string) (*ClientResponse, error)          // uuid.Validate → ErrClientNotFound
func (s *Service) Create(ctx context.Context, in ClientInput) (*ClientResponse, error)
func (s *Service) Update(ctx context.Context, id string, version *int64, in ClientInput) (*ClientResponse, error)
func (s *Service) Delete(ctx context.Context, id string) error
func (s *Service) ListEquipment(ctx context.Context, clientID string, limit, offset int) (*EquipmentListResponse, error) // 404 клиента; limit default 200 / max 200
func (s *Service) GetEquipment(ctx context.Context, id string) (*EquipmentResponse, error)
func (s *Service) CreateEquipment(ctx context.Context, clientID string, in EquipmentInput) (*EquipmentResponse, error)
func (s *Service) UpdateEquipment(ctx context.Context, id string, in EquipmentInput) (*EquipmentResponse, error)
func (s *Service) DeleteEquipment(ctx context.Context, id string) (clientID string, err error)
func (s *Service) TypeOptions(ctx context.Context, currentTypeID string) ([]TypeOption, error)

// equipmenttypes
func NewService(pool *pgxpool.Pool) *Service
func (s *Service) List(ctx context.Context, q ListQuery, isAdmin bool) (*TypeListResponse, error) // !isAdmin → IncludeArchived=false (ADR-004); limit default 500 / max 500 (lookup)
func (s *Service) Get(ctx context.Context, id string) (*TypeResponse, error)
func (s *Service) Create(ctx context.Context, in Input) (*TypeResponse, error)
func (s *Service) Update(ctx context.Context, id string, in Input) (*TypeResponse, error)
func (s *Service) SetActive(ctx context.Context, id string, active bool) error
func (s *Service) Delete(ctx context.Context, id string) error
```
Порядок в `Update`/`Create` клиента: `uuid.Validate(id)` → `normalize` → `validate` (+ `version==nil` → field `version` required) → repo → маппинг repo-ошибок в sentinel. Equipment create/update: validate → `TypeActive` (create: not_found/archived → Validation; update: `check(curTypeID)` в tx — архивный допустим, если `type_id` не менялся).

## Handler
`handler.go` каждой фичи — `Handler{svc *Service}` + методы `List/Get/Create/Update/Delete` (+ clients: `ListEquipment/CreateEquipment/GetEquipment/UpdateEquipment/DeleteEquipment`). Тело: `c.Bind().Body(&req)` err → `errorspkg.BadRequest("Некорректное тело запроса")`; `limit/offset` — `strconv.Atoi` с тихим откатом; DELETE → `c.SendStatus(fiber.StatusNoContent)`; POST → 201. `isAdmin` для типов — `middleware.CurrentPrincipal(c).IsAdmin()`.

## Routes
`routes.go` (Fiber v3, `middleware.RequireAdmin(nil)` по-маршрутно для API):
```go
// clients.Register(app *fiber.App, api fiber.Router, a *middleware.Auth, h *Handler, s *SSR)
admin := middleware.RequireAdmin(nil)
g := api.Group("/clients", a.RequireAPI())
g.Get("", h.List); g.Post("", admin, h.Create)
g.Get("/:id", h.Get); g.Put("/:id", admin, h.Update); g.Delete("/:id", admin, h.Delete)
g.Get("/:id/equipment", h.ListEquipment); g.Post("/:id/equipment", h.CreateEquipment) // D19
e := api.Group("/equipment", a.RequireAPI())
e.Get("/:id", h.GetEquipment); e.Put("/:id", admin, h.UpdateEquipment); e.Delete("/:id", admin, h.DeleteEquipment)

p := app.Group("/clients", a.RequireSSR(), middleware.RequireAdmin(s.status.Forbidden))
p.Get("", s.list); p.Get("/new", s.formNew); p.Post("", s.create)
p.Get("/:id", s.formEdit); p.Post("/:id", s.update); p.Post("/:id/delete", s.remove)
p.Get("/:id/equipment/new", s.eqNew); p.Post("/:id/equipment", s.eqCreate)
q := app.Group("/equipment", a.RequireSSR(), middleware.RequireAdmin(s.status.Forbidden))
q.Get("/:id", s.eqEdit); q.Post("/:id", s.eqUpdate); q.Post("/:id/delete", s.eqRemove)

// equipmenttypes.Register — аналогично: API /equipment-types (GET — RequireAPI; мутации — +admin);
// SSR: app.Get("/catalog", …303) и app.Group("/catalog/equipment-types", RequireSSR, RequireAdmin(status.Forbidden)):
// "" list, "/new", POST "", "/:id", POST "/:id", POST "/:id/delete|archive|restore"
```
`/new` регистрируется до `/:id`.

## SSR Routes
Все: RequireSSR + RequireAdmin(status.Forbidden) (AC-013). Контракт API — `output.md` §Контракт.
| Метод | Путь | Действие | Успех (PRG 303) |
|---|---|---|---|
| GET | `/clients?q=` | список | — |
| GET | `/clients/new` | форма создания | — |
| POST | `/clients` | создать | `/clients/{id}?flash=created` |
| GET | `/clients/:id` | карточка (форма + блок оборудования) | — |
| POST | `/clients/:id` | сохранить (hidden `version`) | `/clients/{id}?flash=saved` |
| POST | `/clients/:id/delete` | удалить (confirm, ADR-103) | `/clients?flash=deleted` |
| GET | `/clients/:id/equipment/new` | форма новой единицы | — |
| POST | `/clients/:id/equipment` | создать единицу | `/clients/{client_id}?flash=eq_created` |
| GET | `/equipment/:id` | форма единицы | — |
| POST | `/equipment/:id` | сохранить | `/clients/{client_id}?flash=eq_saved` |
| POST | `/equipment/:id/delete` | удалить (confirm) | `/clients/{client_id}?flash=eq_deleted` |
| GET | `/catalog` | 303 → `/catalog/equipment-types` | — |
| GET | `/catalog/equipment-types?archived=1` | список типов | — |
| GET | `/catalog/equipment-types/new` | форма | — |
| POST | `/catalog/equipment-types` | создать | `/catalog/equipment-types?flash=created` |
| GET | `/catalog/equipment-types/:id` | форма | — |
| POST | `/catalog/equipment-types/:id` | сохранить | `/catalog/equipment-types?flash=saved` |
| POST | `/catalog/equipment-types/:id/delete` | удалить (confirm) | `…?flash=deleted`; 409 IN_USE → форма типа с ошибкой + кнопка «В архив» |
| POST | `/catalog/equipment-types/:id/archive` | в архив | `…?flash=archived` (фильтр сохраняется: `archived` из формы) |
| POST | `/catalog/equipment-types/:id/restore` | вернуть | `…?archived=1&flash=restored` |

## SSR View Models
Все VM несут `Shell ssr.Shell` (D38), `shell.Build(c, title)` перед `r.Page`.
```go
// clients
type ListVM struct { Shell ssr.Shell; Q, Flash string; Items []ClientResponse; Total int }
type PhoneRow struct { Phone, Note string }
type FormVM struct {
	Shell ssr.Shell; IsNew bool; ID string; Version int64
	CompanyName, Address, ContactName, ContactPosition string
	Phones []PhoneRow                   // новая форма — 1 пустая строка
	Equipment []EquipmentResponse       // блок AC-007 (только правка)
	EquipmentCount int                  // текст confirm удаления (ADR-103)
	Flash, Error string; Conflict bool  // ADR-005
	Errors map[string]string            // "phones[0].phone" → текст
}
type SpecRow struct { K, V string }
type EquipmentFormVM struct {
	Shell ssr.Shell; IsNew bool; ID, ClientID, ClientName string
	TypeID, Brand, Model, SKU, Serial, Note string
	Specs []SpecRow; Types []TypeOptionVM   // TypeOptionVM{ID, Name, SpecKeysJSON string, IsActive, Selected bool}
	Error string; Errors map[string]string
}
// equipmenttypes
type ListVM struct { Shell ssr.Shell; Archived bool; Flash string; Items []TypeResponse; Total int }
type FormVM struct { Shell ssr.Shell; IsNew bool; ID, Name, SpecKeys string; IsActive bool; InUse bool; Error string; Errors map[string]string }
```
Массивы формы читаются `c.Request().PostArgs().PeekMulti("phone")` / `"phone_note"` / `"spec_k"` / `"spec_v"` и склеиваются по индексу (короткий массив дополняется ""). `version` — `strconv.ParseInt(c.FormValue("version"))`, мусор → `nil` → 400 required. Новая единица: тип не предвыбран (placeholder «Выберите тип»), `specs` пусто; строки шаблона подставляет JS при выборе типа (ADR-002); при ошибке валидации сервер перерисовывает введённое.

Тексты flash: clients — `created` «Клиент создан», `saved` «Изменения сохранены», `deleted` «Клиент удалён», `eq_created` «Оборудование добавлено», `eq_saved` «Оборудование сохранено», `eq_deleted` «Оборудование удалено»; types — `created/saved/deleted/archived/restored`.

## SSR Templates
Разметка — Bootstrap 5.3 + `app.css`, эталон полей/текстов — `prototype/index.html` (renderClients/openClient/openEquipment/renderEqTypes/openEqType); каждая страница — `{{ define "content" }}` + опц. `actions`/`scripts` в общем `base`.
- `clients/list.html`: GET-форма `q` (`data-testid="clients-filters"`, `filter-q`, «Сбросить»); `table-responsive` таблица `clients-table`: Компания (ссылка `/clients/{id}`), Адрес `d-none d-md-table-cell`, Представитель + должность `muted`, Телефоны (все, с примечанием, `<br>`), Оборудование `d-none d-md-table-cell`; пусто → «Клиенты не найдены» (`clients-empty`); «всего N» (`clients-total`); actions: «+ Клиент» (`client-add`).
- `clients/form.html`: поля + `row-list` телефонов (`phone`, `phone_note`, `phone-add`, `phone-del`); hidden `_csrf`, `version`; conflict-блок (`form-error`, `client-conflict-reload`); на правке — блок «Оборудование клиента» (строки: модель, тип + «(в архиве)», SN; ссылка `/equipment/{id}`; пусто «Оборудования пока нет»; «+ Оборудование» → `/clients/{id}/equipment/new`); отдельная форма удаления `data-confirm` (`client-delete`).
- `clients/equipment-form.html`: `select.form-select name=type_id` (combobox D6) с `data-spec-keys`; brand/model/sku/serial/note; `row-list` характеристик (`spec_k` с `<datalist>` ключей типа, `spec_v`); удаление (`equipment-delete`, `data-confirm`); «Отмена» → карточка клиента.
- `equipmenttypes/list.html`: чекбокс «Показать архивные» (GET `archived=1`, `onchange=submit`); таблица: Название (+ бейдж `badge-off` «в архиве»), Характеристики по умолчанию (через запятую / «—»), Единиц у клиентов `d-none d-md-table-cell`, кнопка «В архив»/«Вернуть» (POST-форма); подсказка прототипа под таблицей; actions «+ Тип оборудования».
- `equipmenttypes/form.html`: `name`, `spec_keys` (через запятую); на правке — «Удалить» (`data-confirm`), при `InUse` — ошибка + кнопка «В архив».

## JS
- `web/ssr/_shared/js/row-list.js` (NEW, ≤80 строк, IIFE `KT.rowList`, без зависимостей): на `DOMContentLoaded` инициализирует все `[data-row-list]`; `[data-row-add]` клонирует `<template data-row-template>` в `[data-row-items]`, фокус на первый input; `[data-row-del]` удаляет строку (для телефонов последняя строка очищается, не удаляется — `data-row-min="1"`); `KT.rowList.fill(listEl, [{k,v}])` — перерисовка (для смены типа). Подключение — `{{ define "scripts" }}<script src="/assets/ssr-shared/js/row-list.js"></script>{{ end }}`.
- `equipment-form.html` inline-скрипт: `change` на `select[name=type_id]` → `JSON.parse(opt.dataset.specKeys||'[]')` → заполненные пары оставить, пустые заменить ключами шаблона (без дублей k) → `KT.rowList.fill`; обновить `<datalist>`.
- `ui-dialog.js` (MODIFY): `document.addEventListener('submit', …)` для `form[data-confirm]` → `KT.uiConfirm(text, form.dataset.confirmOk)` → `HTMLFormElement.prototype.submit.call(form)`; submit-кнопка `disabled` на время (двойной submit, Tech Spec).
- Двойной submit обычных форм: `form[data-once]` → кнопка `disabled` при submit (там же в `ui-dialog.js`, 5 строк).

## Shared SSR (ADR-006)
- `internal/ssr/shell.go`: `func (s Shell) NavActive(url string) bool`.
- `base.html`: `{{ if $.Shell.NavActive .URL }} active{{ end }}` (+ `aria-current`).
- `internal/ssr/menu.go`: `{Title: "Каталог", Icon: "🧰", URL: "/catalog", AdminOnly: true}`.

## DI / Wiring
`internal/server/router.go` (после users):
```go
shell := ssr.NewShellBuilder("/", "/users", "/clients", "/catalog")
clientsR, err := ssr.NewRenderer(clients.Root, ssr.SharedRoot)          // Root = "web/ssr/clients/templates"
typesR, err := ssr.NewRenderer(equipmenttypes.Root, ssr.SharedRoot)     // Root = "web/ssr/equipmenttypes/templates"
clientsSvc := clients.NewService(pool)
clients.Register(app, api, guard, clients.NewHandler(clientsSvc), clients.NewSSR(clientsSvc, clientsR, shell, status))
typesSvc := equipmenttypes.NewService(pool)
equipmenttypes.Register(app, api, guard, equipmenttypes.NewHandler(typesSvc), equipmenttypes.NewSSR(typesSvc, typesR, shell, status))
```
Имена шаблонов страниц уникальны в пределах рендерера фичи (`list`, `form`, `equipment-form`).
