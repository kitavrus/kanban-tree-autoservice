-- Клиенты, их телефоны, типы оборудования и оборудование (фаза 3, design-sql.md).
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
