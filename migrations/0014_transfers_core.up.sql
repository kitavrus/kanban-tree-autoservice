-- Перемещения — ядро (docs/features/transfers-core/draft-spec.md, часть 3 мастер-спеки
-- transfers): DS-030 переименование stock_transfers → transfers; DS-001 строки-единицы
-- оборудования (kind, equipment_id); DS-008/DS-041 местоположение и занятие документом —
-- на самой единице (equipment); DS-029/DS-054 перенос местоположения со строк заказа,
-- журнал task_equipment_movements удаляется. Режим MVP (DS-027): без переходного периода.

-- ---- DS-030: переименование таблиц, последовательности, индексов, ограничений ----
ALTER TABLE stock_transfers RENAME TO transfers;
ALTER TABLE stock_transfer_items RENAME TO transfer_items;
ALTER SEQUENCE stock_transfers_id_seq RENAME TO transfers_id_seq;
ALTER INDEX stock_transfers_status_idx RENAME TO transfers_status_idx;
ALTER INDEX stock_transfers_from_warehouse_id_idx RENAME TO transfers_from_warehouse_id_idx;
ALTER INDEX stock_transfers_to_warehouse_id_idx RENAME TO transfers_to_warehouse_id_idx;
ALTER INDEX stock_transfer_items_product_id_idx RENAME TO transfer_items_product_id_idx;

-- Ограничения (PK, FK, CHECK, UNIQUE) — по каталогу: имена авто-CHECK'ов зависят от версии PG.
DO $$
DECLARE r record;
BEGIN
    FOR r IN
        SELECT c.conname, c.conrelid::regclass AS tbl
        FROM pg_constraint c
        WHERE c.conrelid IN ('transfers'::regclass, 'transfer_items'::regclass)
          AND c.conname LIKE 'stock\_transfer%'
    LOOP
        EXECUTE format('ALTER TABLE %s RENAME CONSTRAINT %I TO %I',
            r.tbl, r.conname, regexp_replace(r.conname, '^stock_transfer', 'transfer'));
    END LOOP;
END $$;

-- ---- DS-001: тип строки и единица оборудования ----
ALTER TABLE transfer_items
    ADD COLUMN kind text NOT NULL DEFAULT 'product',
    ADD COLUMN equipment_id uuid NULL REFERENCES equipment(id) ON DELETE RESTRICT,
    ALTER COLUMN product_id DROP NOT NULL,
    ALTER COLUMN qty_sent DROP NOT NULL;
-- kind — без DEFAULT: код приложения передаёт тип строки явно.
ALTER TABLE transfer_items ALTER COLUMN kind DROP DEFAULT;

ALTER TABLE transfer_items
    ADD CONSTRAINT transfer_items_kind_check CHECK (
        (kind = 'product' AND product_id IS NOT NULL AND qty_sent IS NOT NULL
            AND equipment_id IS NULL)
     OR (kind = 'equipment' AND equipment_id IS NOT NULL AND product_id IS NULL
            AND qty_sent IS NULL AND qty_received IS NULL)),
    ADD CONSTRAINT transfer_items_transfer_id_equipment_id_key UNIQUE (transfer_id, equipment_id);
CREATE INDEX transfer_items_equipment_id_idx ON transfer_items (equipment_id);

-- ---- DS-008, DS-041: местоположение и занятие документом — на единице ----
ALTER TABLE equipment
    ADD COLUMN location_status text NOT NULL DEFAULT 'at_client',
    ADD COLUMN warehouse_id uuid NULL REFERENCES warehouses(id) ON DELETE RESTRICT,
    ADD COLUMN active_transfer_id bigint NULL REFERENCES transfers(id) ON DELETE SET NULL;

-- DS-054: завершённый заказ = заказ в ПОСЛЕДНЕЙ колонке доски (наибольший position, при
-- равных — больший id; is_final не используется). Единица хотя бы в одном незавершённом
-- заказе → «на складе»: склад её строки самого свежего незавершённого заказа (больший
-- tasks.id); для «в пути» — склад отправления (DS-029). Остальные остаются «у клиента».
UPDATE equipment e
SET location_status = 'at_warehouse', warehouse_id = src.warehouse_id
FROM (
    SELECT DISTINCT ON (te.equipment_id)
           te.equipment_id,
           CASE WHEN te.status = 'in_transit' THEN te.transit_from_warehouse_id
                ELSE te.warehouse_id END AS warehouse_id
    FROM task_equipment te
    JOIN tasks t ON t.id = te.task_id
    WHERE t.column_id <> (SELECT id FROM columns ORDER BY position DESC, id DESC LIMIT 1)
    ORDER BY te.equipment_id, t.id DESC
) src
WHERE e.id = src.equipment_id AND src.warehouse_id IS NOT NULL;

ALTER TABLE equipment
    ADD CONSTRAINT equipment_location_status_check
        CHECK (location_status IN ('at_client', 'at_warehouse', 'in_transit')),
    ADD CONSTRAINT equipment_location_check CHECK (
        ((location_status = 'at_warehouse') = (warehouse_id IS NOT NULL))
        AND (location_status <> 'in_transit' OR active_transfer_id IS NOT NULL));
CREATE INDEX equipment_warehouse_id_idx ON equipment (warehouse_id);
CREATE INDEX equipment_active_transfer_id_idx ON equipment (active_transfer_id);

-- ---- DS-008, DS-029: строка заказа теряет склад/статус/маршрут, журнал удаляется ----
DROP TABLE task_equipment_movements;
DROP INDEX IF EXISTS task_equipment_transit_to_warehouse_id_idx;
DROP INDEX IF EXISTS task_equipment_warehouse_id_idx;
ALTER TABLE task_equipment
    DROP CONSTRAINT IF EXISTS task_equipment_location_check,
    DROP CONSTRAINT IF EXISTS task_equipment_status_check,
    DROP COLUMN transit_to_warehouse_id,
    DROP COLUMN transit_from_warehouse_id,
    DROP COLUMN status,
    DROP COLUMN warehouse_id;
