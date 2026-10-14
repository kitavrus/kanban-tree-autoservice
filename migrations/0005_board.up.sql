-- Доска задач: колонки, заказы (BIGSERIAL, D37), подзадачи (услуги в заказе), комплект
-- товаров подзадачи, фактически зарезервированные/списанные товары заказа, оборудование
-- заказа; миграция task_id склада uuid→bigint (фаза 5, design-sql.md).

CREATE TABLE columns (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name           text NOT NULL CHECK (btrim(name) <> '' AND char_length(name) <= 60),
    color          text NOT NULL CHECK (color ~ '^#[0-9A-Fa-f]{6}$'),
    is_final       boolean NOT NULL DEFAULT false,
    consumes_stock boolean NOT NULL DEFAULT false,
    position       int NOT NULL CHECK (position >= 1),
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX columns_position_idx ON columns (position, id);

-- Сид D32 (AC-008) — колонки по умолчанию.
INSERT INTO columns (name, color, is_final, consumes_stock, position) VALUES
    ('Новые',          '#94A3B8', false, false, 1),
    ('Согласование',   '#7C3AED', false, false, 2),
    ('В работе',       '#2563EB', false, true,  3),
    ('Ждёт запчасти',  '#D97706', false, true,  4),
    ('Готово',         '#059669', true,  true,  5);

CREATE TABLE tasks (
    id               bigserial PRIMARY KEY,                       -- D37: сквозной #N, без переиспользования
    column_id        uuid NOT NULL REFERENCES columns(id) ON DELETE RESTRICT,
    client_id        uuid NOT NULL REFERENCES clients(id) ON DELETE RESTRICT,   -- опора D26 (AC-017)
    title            text NOT NULL CHECK (btrim(title) <> '' AND char_length(title) <= 200),
    description_html text NOT NULL DEFAULT '',                    -- фаза 6; в фазе 5 пишет только seed-demo
    assignee_id      uuid NULL REFERENCES users(id) ON DELETE SET NULL,
    due_date         date NULL,
    is_paid          boolean NOT NULL DEFAULT false,
    paid_at          timestamptz NULL,
    position         numeric NOT NULL,                            -- дробная позиция (ADR-001)
    created_by       uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    version          bigint NOT NULL DEFAULT 1,
    CONSTRAINT tasks_paid_at_chk CHECK (is_paid = (paid_at IS NOT NULL))
);
CREATE INDEX tasks_column_position_idx ON tasks (column_id, position, id);
CREATE INDEX tasks_client_id_idx       ON tasks (client_id);
CREATE INDEX tasks_assignee_id_idx     ON tasks (assignee_id);

CREATE TABLE subtasks (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id      bigint NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    service_id   uuid NOT NULL REFERENCES services(id) ON DELETE RESTRICT,
    status       text NOT NULL DEFAULT 'new' CHECK (status IN ('new', 'in_progress', 'done')),
    assignee_id  uuid NULL REFERENCES users(id) ON DELETE SET NULL,
    due_date     date NULL,
    price        numeric(12,2) NULL CHECK (price IS NULL OR price >= 0),
    equipment_id uuid NULL REFERENCES equipment(id) ON DELETE RESTRICT,  -- опора D26 (AC-018)
    position     int NOT NULL DEFAULT 0,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX subtasks_task_id_idx      ON subtasks (task_id);
CREATE INDEX subtasks_service_id_idx   ON subtasks (service_id);
CREATE INDEX subtasks_equipment_id_idx ON subtasks (equipment_id);
CREATE INDEX subtasks_assignee_id_idx  ON subtasks (assignee_id);

CREATE TABLE subtask_products (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    subtask_id uuid NOT NULL REFERENCES subtasks(id) ON DELETE CASCADE,
    product_id uuid NOT NULL REFERENCES products(id) ON DELETE RESTRICT,
    qty        numeric(12,3) NOT NULL CHECK (qty > 0),
    price      numeric(12,2) NULL CHECK (price IS NULL OR price >= 0)
);
CREATE INDEX subtask_products_subtask_id_idx ON subtask_products (subtask_id);
CREATE INDEX subtask_products_product_id_idx ON subtask_products (product_id);

CREATE TABLE task_stock (
    task_id    bigint NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    product_id uuid NOT NULL REFERENCES products(id) ON DELETE RESTRICT,
    qty        numeric(12,3) NOT NULL CHECK (qty > 0),   -- 0 → строка удаляется
    PRIMARY KEY (task_id, product_id)
);
CREATE INDEX task_stock_product_id_idx ON task_stock (product_id);

CREATE TABLE task_equipment (
    task_id      bigint NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    equipment_id uuid NOT NULL REFERENCES equipment(id) ON DELETE RESTRICT,
    completeness text NOT NULL DEFAULT '',
    condition    text NOT NULL DEFAULT '',
    PRIMARY KEY (task_id, equipment_id)
);
CREATE INDEX task_equipment_equipment_id_idx ON task_equipment (equipment_id);

-- task_id: uuid → bigint. Данных с task_id нет (все строки manual, D44) — USING NULL.
ALTER TABLE stock_movements ALTER COLUMN task_id TYPE bigint USING NULL;
ALTER TABLE stock_movements
    ADD CONSTRAINT stock_movements_task_id_fkey
    FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE SET NULL;   -- SD-007
CREATE INDEX stock_movements_task_id_idx ON stock_movements (task_id);
