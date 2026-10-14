-- Откат склада оборудования заказа: журнал перемещений удаляется, колонки склада/статуса
-- снимаются (единицы «в пути» теряют маршрут — как и все прочие, склад больше не хранится).
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
