-- Склады (D45–D49, docs/features/warehouses/draft-spec.md): подфаза A — справочник складов,
-- остатки по складам (product_stock), склад строки товара услуги, stock_movements.warehouse_id.
-- Существующие остатки/строки/движения → склад «Алматы».

-- DS-001: справочник складов. Уникальность имени без регистра/крайних пробелов, архивные
-- включены (ADR-007, как equipment_types).
CREATE TABLE warehouses (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name       text NOT NULL CHECK (btrim(name) <> '' AND char_length(name) <= 100),
    position   int NOT NULL CHECK (position >= 1),
    is_active  boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX warehouses_name_lower_uq ON warehouses (lower(btrim(name)));

INSERT INTO warehouses (name, position) VALUES ('Алматы', 1), ('Москва', 2);

-- D49: склад по умолчанию = первый активный по position (DS-005). DEFAULT колонок
-- subtask_products/stock_movements.warehouse_id — для вставок без явного склада (сид,
-- служебные скрипты); код приложения склад передаёт явно.
CREATE FUNCTION default_warehouse_id() RETURNS uuid
    LANGUAGE sql STABLE
    AS $$ SELECT id FROM warehouses WHERE is_active ORDER BY position, id LIMIT 1 $$;

-- DS-002: остаток по паре товар×склад. Отсутствующая строка = 0 остаток / 0 минимум.
CREATE TABLE product_stock (
    product_id   uuid NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    warehouse_id uuid NOT NULL REFERENCES warehouses(id) ON DELETE RESTRICT,
    stock        numeric(12,3) NOT NULL DEFAULT 0,
    min_stock    numeric(12,3) NOT NULL DEFAULT 0 CHECK (min_stock >= 0),
    box          text NOT NULL DEFAULT '',
    place        text NOT NULL DEFAULT '',
    PRIMARY KEY (product_id, warehouse_id)
);
CREATE INDEX product_stock_warehouse_id_idx ON product_stock (warehouse_id);

INSERT INTO product_stock (product_id, warehouse_id, stock, min_stock, box, place)
SELECT p.id, w.id, p.stock, p.min_stock, p.box, p.place
FROM products p CROSS JOIN (SELECT id FROM warehouses WHERE name = 'Алматы') w;

ALTER TABLE products
    DROP COLUMN stock,
    DROP COLUMN min_stock,
    DROP COLUMN box,
    DROP COLUMN place;

-- DS-005: склад строки товара услуги заказа + task_stock по паре товар×склад.
ALTER TABLE subtask_products ADD COLUMN warehouse_id uuid NULL REFERENCES warehouses(id) ON DELETE RESTRICT;
UPDATE subtask_products SET warehouse_id = (SELECT id FROM warehouses WHERE name = 'Алматы');
ALTER TABLE subtask_products ALTER COLUMN warehouse_id SET NOT NULL;
ALTER TABLE subtask_products ALTER COLUMN warehouse_id SET DEFAULT default_warehouse_id();
CREATE INDEX subtask_products_warehouse_id_idx ON subtask_products (warehouse_id);

ALTER TABLE task_stock ADD COLUMN warehouse_id uuid NULL REFERENCES warehouses(id) ON DELETE RESTRICT;
UPDATE task_stock SET warehouse_id = (SELECT id FROM warehouses WHERE name = 'Алматы');
ALTER TABLE task_stock ALTER COLUMN warehouse_id SET NOT NULL;
ALTER TABLE task_stock DROP CONSTRAINT task_stock_pkey;
ALTER TABLE task_stock ADD PRIMARY KEY (task_id, product_id, warehouse_id);

-- DS-007: склад движения + ссылка на перемещение (таблица stock_transfers — подфаза B,
-- FK добавится там), расширенный набор reason.
ALTER TABLE stock_movements ADD COLUMN warehouse_id uuid NULL REFERENCES warehouses(id) ON DELETE RESTRICT;
UPDATE stock_movements SET warehouse_id = (SELECT id FROM warehouses WHERE name = 'Алматы');
ALTER TABLE stock_movements ALTER COLUMN warehouse_id SET NOT NULL;
ALTER TABLE stock_movements ALTER COLUMN warehouse_id SET DEFAULT default_warehouse_id();
ALTER TABLE stock_movements ADD COLUMN transfer_id bigint NULL;
CREATE INDEX stock_movements_warehouse_id_idx ON stock_movements (warehouse_id);
ALTER TABLE stock_movements DROP CONSTRAINT stock_movements_reason_check;
ALTER TABLE stock_movements ADD CONSTRAINT stock_movements_reason_check CHECK (reason IN (
    'task_consume', 'task_return', 'manual',
    'transfer_out', 'transfer_in', 'transfer_loss', 'transfer_return'));
