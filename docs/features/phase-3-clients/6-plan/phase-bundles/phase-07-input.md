# Phase Bundle: phase-3-clients / Phase 07 (backend) — generated 2026-09-26 00:44

> Твой вход — ЭТОТ файл. Upstream-доки (research/prd/design-*) напрямую НЕ читать —
> нужные слайсы дизайна ниже. Код фичи и её тесты — читать/писать можно (твоя фаза).

## Фаза плана (6-plan/code-plan-phase-07-equipment-backend.md)
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


## Слайс: 3-design/design-sql.md [full]
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

## Слайс: 3-design/design-go-layers.md [full]
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

## Слайс: 3-design/design-tests.md [sections]
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

## Mock Strategy
Моков нет: repository/service — через живую БД (BaseSuite/LiveSuite, dockertest PG18 + Redis); внешних клиентов нет.

## Слайс: 1-research/output.md [sections]
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
