-- Откат ядра перемещений: строка заказа снова хранит склад (берётся со склада единицы, иначе
-- склад по умолчанию), все единицы — at_warehouse (маршруты «в пути» и занятие документом не
-- восстанавливаются); журнал task_equipment_movements создаётся пустым; строки-единицы
-- документов удаляются; таблицы возвращают имена stock_transfers / stock_transfer_items.

-- ---- task_equipment: склад/статус/маршрут + журнал (как 0010) ----
ALTER TABLE task_equipment
    ADD COLUMN warehouse_id uuid NULL REFERENCES warehouses(id) ON DELETE RESTRICT,
    ADD COLUMN status text NOT NULL DEFAULT 'at_warehouse',
    ADD COLUMN transit_from_warehouse_id uuid NULL REFERENCES warehouses(id) ON DELETE RESTRICT,
    ADD COLUMN transit_to_warehouse_id uuid NULL REFERENCES warehouses(id) ON DELETE RESTRICT;

UPDATE task_equipment te
SET warehouse_id = coalesce(e.warehouse_id, default_warehouse_id(),
    (SELECT id FROM warehouses ORDER BY position, id LIMIT 1))
FROM equipment e
WHERE e.id = te.equipment_id;

ALTER TABLE task_equipment ALTER COLUMN warehouse_id SET DEFAULT default_warehouse_id();
ALTER TABLE task_equipment ADD CONSTRAINT task_equipment_status_check
    CHECK (status IN ('at_warehouse', 'in_transit'));
ALTER TABLE task_equipment ADD CONSTRAINT task_equipment_location_check CHECK (
    (status = 'at_warehouse' AND warehouse_id IS NOT NULL
        AND transit_from_warehouse_id IS NULL AND transit_to_warehouse_id IS NULL)
 OR (status = 'in_transit' AND warehouse_id IS NULL
        AND transit_from_warehouse_id IS NOT NULL AND transit_to_warehouse_id IS NOT NULL
        AND transit_from_warehouse_id <> transit_to_warehouse_id));
CREATE INDEX task_equipment_warehouse_id_idx ON task_equipment (warehouse_id);
CREATE INDEX task_equipment_transit_to_warehouse_id_idx ON task_equipment (transit_to_warehouse_id);

CREATE TABLE task_equipment_movements (
    id                bigserial PRIMARY KEY,
    task_id           bigint NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    equipment_id      uuid NOT NULL REFERENCES equipment(id) ON DELETE CASCADE,
    action            text NOT NULL CHECK (action IN ('received', 'sent', 'arrived', 'cancelled')),
    from_warehouse_id uuid NULL REFERENCES warehouses(id) ON DELETE RESTRICT,
    to_warehouse_id   uuid NULL REFERENCES warehouses(id) ON DELETE RESTRICT,
    user_id           uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at        timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX task_equipment_movements_task_equipment_idx ON task_equipment_movements (task_id, equipment_id, id);
CREATE INDEX task_equipment_movements_equipment_id_idx ON task_equipment_movements (equipment_id);

-- ---- equipment: местоположение и занятие документом снимаются ----
DROP INDEX IF EXISTS equipment_active_transfer_id_idx;
DROP INDEX IF EXISTS equipment_warehouse_id_idx;
ALTER TABLE equipment
    DROP CONSTRAINT IF EXISTS equipment_location_check,
    DROP CONSTRAINT IF EXISTS equipment_location_status_check,
    DROP COLUMN active_transfer_id,
    DROP COLUMN warehouse_id,
    DROP COLUMN location_status;

-- ---- transfer_items: только товарные строки ----
DELETE FROM transfer_items WHERE kind <> 'product';
DROP INDEX IF EXISTS transfer_items_equipment_id_idx;
ALTER TABLE transfer_items
    DROP CONSTRAINT IF EXISTS transfer_items_transfer_id_equipment_id_key,
    DROP CONSTRAINT IF EXISTS transfer_items_kind_check,
    DROP COLUMN equipment_id,
    DROP COLUMN kind,
    ALTER COLUMN product_id SET NOT NULL,
    ALTER COLUMN qty_sent SET NOT NULL;

-- ---- имена таблиц, последовательности, индексов, ограничений — обратно ----
DO $$
DECLARE r record;
BEGIN
    FOR r IN
        SELECT c.conname, c.conrelid::regclass AS tbl
        FROM pg_constraint c
        WHERE c.conrelid IN ('transfers'::regclass, 'transfer_items'::regclass)
          AND c.conname LIKE 'transfer%'
    LOOP
        EXECUTE format('ALTER TABLE %s RENAME CONSTRAINT %I TO %I',
            r.tbl, r.conname, 'stock_' || r.conname);
    END LOOP;
END $$;

ALTER INDEX transfer_items_product_id_idx RENAME TO stock_transfer_items_product_id_idx;
ALTER INDEX transfers_to_warehouse_id_idx RENAME TO stock_transfers_to_warehouse_id_idx;
ALTER INDEX transfers_from_warehouse_id_idx RENAME TO stock_transfers_from_warehouse_id_idx;
ALTER INDEX transfers_status_idx RENAME TO stock_transfers_status_idx;
ALTER SEQUENCE transfers_id_seq RENAME TO stock_transfers_id_seq;
ALTER TABLE transfer_items RENAME TO stock_transfer_items;
ALTER TABLE transfers RENAME TO stock_transfers;
