-- Откат части transfers-partners: остаются только документы «склад → склад». Документы с
-- клиентом и от поставщика удаляются (в модели 0014 их маршрут невыразим); единицы, занятые
-- такими документами, освобождаются («в пути» → «у клиента»); движения с новыми причинами
-- сохраняются как manual (остаток не пересчитывается).

-- ---- причины движений — прежний набор ----
UPDATE stock_movements SET reason = 'manual'
WHERE reason IN ('supplier_in', 'client_out', 'transfer_unreceive');
ALTER TABLE stock_movements DROP CONSTRAINT stock_movements_reason_check;
ALTER TABLE stock_movements ADD CONSTRAINT stock_movements_reason_check CHECK (reason IN (
    'task_consume', 'task_return', 'manual',
    'transfer_out', 'transfer_in', 'transfer_loss', 'transfer_return'));

-- ---- документы новых направлений удаляются ----
UPDATE equipment e
SET location_status = CASE WHEN e.location_status = 'in_transit' THEN 'at_client' ELSE e.location_status END,
    warehouse_id = CASE WHEN e.location_status = 'in_transit' THEN NULL ELSE e.warehouse_id END,
    active_transfer_id = NULL
FROM transfers t
WHERE t.id = e.active_transfer_id AND t.direction <> 'warehouse_to_warehouse';
DELETE FROM transfers WHERE direction <> 'warehouse_to_warehouse';

-- ---- transfer_items: факт снова не больше отправленного ----
UPDATE transfer_items SET qty_received = qty_sent WHERE qty_received > qty_sent;
ALTER TABLE transfer_items
    DROP CONSTRAINT IF EXISTS transfer_items_qty_received_check,
    ADD CONSTRAINT transfer_items_qty_received_check CHECK (qty_received >= 0 AND qty_received <= qty_sent);

-- ---- transfers: маршрут — снова только два склада ----
DROP INDEX IF EXISTS transfers_created_at_idx;
DROP INDEX IF EXISTS transfers_supplier_id_idx;
DROP INDEX IF EXISTS transfers_client_id_idx;
DROP INDEX IF EXISTS transfers_task_id_idx;
DROP INDEX IF EXISTS transfers_direction_idx;
ALTER TABLE transfers
    DROP CONSTRAINT IF EXISTS transfers_route_check,
    DROP COLUMN supplier_id,
    DROP COLUMN client_id,
    DROP COLUMN task_id,
    DROP COLUMN direction,
    ALTER COLUMN from_warehouse_id SET NOT NULL,
    ALTER COLUMN to_warehouse_id SET NOT NULL,
    ADD CONSTRAINT transfers_distinct_warehouses CHECK (from_warehouse_id <> to_warehouse_id);
