-- Клиенты, их телефоны и оборудование (фаза 3, design-sql.md).
CREATE TABLE clients (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    company_name     text NOT NULL CHECK (btrim(company_name) <> ''),
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

CREATE TABLE equipment (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id  uuid NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    plate_number text NOT NULL CHECK (btrim(plate_number) <> '' AND char_length(plate_number) <= 20), -- госномер автомобиля
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX equipment_client_id_idx ON equipment (client_id);
CREATE INDEX equipment_plate_number_idx ON equipment (plate_number);
