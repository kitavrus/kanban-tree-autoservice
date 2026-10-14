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
