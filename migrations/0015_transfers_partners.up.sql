-- Перемещения — клиент и поставщик (docs/features/transfers-partners/draft-spec.md, часть 4
-- мастер-спеки transfers): DS-002 направление и точки маршрута (клиент, поставщик), DS-012
-- заказ документа с клиентом, DS-015 факт приёмки от поставщика может превышать заказанное,
-- DS-040 три новые причины движений остатка. Режим MVP (DS-027): ALTER поверх 0014.

-- ---- DS-002, DS-012: направление, заказ, клиент, поставщик ----
ALTER TABLE transfers
    ADD COLUMN direction text NOT NULL DEFAULT 'warehouse_to_warehouse',
    ADD COLUMN task_id bigint NULL REFERENCES tasks(id) ON DELETE RESTRICT,
    ADD COLUMN client_id uuid NULL REFERENCES clients(id) ON DELETE RESTRICT,
    ADD COLUMN supplier_id uuid NULL REFERENCES suppliers(id) ON DELETE RESTRICT,
    ALTER COLUMN from_warehouse_id DROP NOT NULL,
    ALTER COLUMN to_warehouse_id DROP NOT NULL;
-- direction — без DEFAULT: код приложения передаёт направление явно (существующие строки уже
-- получили warehouse_to_warehouse).
ALTER TABLE transfers ALTER COLUMN direction DROP DEFAULT;

-- CHECK маршрута заменяет transfers_distinct_warehouses: набор заполненных точек
-- соответствует направлению; для склад → склад — from <> to.
ALTER TABLE transfers
    DROP CONSTRAINT transfers_distinct_warehouses,
    ADD CONSTRAINT transfers_route_check CHECK (
        (direction = 'warehouse_to_warehouse'
            AND from_warehouse_id IS NOT NULL AND to_warehouse_id IS NOT NULL
            AND from_warehouse_id <> to_warehouse_id
            AND task_id IS NULL AND client_id IS NULL AND supplier_id IS NULL)
     OR (direction = 'client_to_warehouse'
            AND from_warehouse_id IS NULL AND to_warehouse_id IS NOT NULL
            AND task_id IS NOT NULL AND client_id IS NOT NULL AND supplier_id IS NULL)
     OR (direction = 'warehouse_to_client'
            AND from_warehouse_id IS NOT NULL AND to_warehouse_id IS NULL
            AND task_id IS NOT NULL AND client_id IS NOT NULL AND supplier_id IS NULL)
     OR (direction = 'supplier_to_warehouse'
            AND from_warehouse_id IS NULL AND to_warehouse_id IS NOT NULL
            AND task_id IS NULL AND client_id IS NULL AND supplier_id IS NOT NULL));

CREATE INDEX transfers_direction_idx ON transfers (direction);
CREATE INDEX transfers_task_id_idx ON transfers (task_id);
CREATE INDEX transfers_client_id_idx ON transfers (client_id);
CREATE INDEX transfers_supplier_id_idx ON transfers (supplier_id);
CREATE INDEX transfers_created_at_idx ON transfers (created_at);

-- ---- DS-015: факт приёмки больше не ограничен отправленным на уровне БД ----
-- Имя авто-CHECK'а колонки (0009) зависит от версии PG — ищем по определению.
DO $$
DECLARE r record;
BEGIN
    FOR r IN
        SELECT c.conname
        FROM pg_constraint c
        WHERE c.conrelid = 'transfer_items'::regclass AND c.contype = 'c'
          AND pg_get_constraintdef(c.oid) LIKE '%qty_received <= qty_sent%'
    LOOP
        EXECUTE format('ALTER TABLE transfer_items DROP CONSTRAINT %I', r.conname);
    END LOOP;
END $$;
ALTER TABLE transfer_items
    ADD CONSTRAINT transfer_items_qty_received_check CHECK (qty_received >= 0);

-- ---- DS-040: причины движений — сразу три (client_out — часть 5, transfer_unreceive — часть 6) ----
ALTER TABLE stock_movements DROP CONSTRAINT stock_movements_reason_check;
ALTER TABLE stock_movements ADD CONSTRAINT stock_movements_reason_check CHECK (reason IN (
    'task_consume', 'task_return', 'manual',
    'transfer_out', 'transfer_in', 'transfer_loss', 'transfer_return',
    'supplier_in', 'client_out', 'transfer_unreceive'));
