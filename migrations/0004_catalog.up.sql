-- Каталог: категории услуг, товары, услуги, комплект услуги, ручные движения остатка
-- (фаза 4, docs/features/phase-4-catalog/0-setup/task.md; полная схема — сразу, подфаза B
-- добавит только Go-код для services/service_products).

CREATE TABLE catalog_categories (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    kind       text NOT NULL CHECK (kind IN ('service')),
    name       text NOT NULL CHECK (btrim(name) <> ''),
    position   integer NOT NULL DEFAULT 0,
    is_active  boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
-- D16: дубль без учёта регистра/пробелов в рамках одного вида — архивные включены.
CREATE UNIQUE INDEX catalog_categories_kind_name_lower_uq ON catalog_categories (kind, lower(btrim(name)));

CREATE TABLE products (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name       text NOT NULL CHECK (btrim(name) <> ''),
    sku        text NOT NULL DEFAULT '',
    unit       text NOT NULL DEFAULT '',
    price      numeric(12,2) NOT NULL DEFAULT 0 CHECK (price >= 0),
    is_active  boolean NOT NULL DEFAULT true,
    stock      numeric(12,3) NOT NULL DEFAULT 0,
    min_stock  numeric(12,3) NOT NULL DEFAULT 0 CHECK (min_stock >= 0),
    box        text NOT NULL DEFAULT '',
    place      text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE services (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    category_id uuid NOT NULL REFERENCES catalog_categories(id) ON DELETE RESTRICT,
    name        text NOT NULL CHECK (btrim(name) <> ''),
    price       numeric(12,2) CHECK (price IS NULL OR price >= 0), -- NULL = «по факту»
    is_active   boolean NOT NULL DEFAULT true,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX services_category_id_idx ON services (category_id);

CREATE TABLE service_products (
    service_id uuid NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    product_id uuid NOT NULL REFERENCES products(id) ON DELETE RESTRICT,
    qty        numeric(12,3) NOT NULL CHECK (qty > 0),
    position   integer NOT NULL DEFAULT 0,
    PRIMARY KEY (service_id, product_id)
);
CREATE INDEX service_products_product_id_idx ON service_products (product_id); -- RESTRICT-проверка

CREATE TABLE stock_movements (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id uuid NOT NULL REFERENCES products(id) ON DELETE RESTRICT,
    task_id    uuid NULL, -- FK на задачи появится в фазе 5
    qty_delta  numeric(12,3) NOT NULL CHECK (qty_delta <> 0),
    reason     text NOT NULL CHECK (reason IN ('task_consume', 'task_return', 'manual')),
    user_id    uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX stock_movements_product_id_idx ON stock_movements (product_id);
