-- Склад и перемещения оборудования заказа (подфаза C, docs/features/warehouses/draft-spec.md
-- DS-008, D55–D57). Текущий склад единицы — warehouse_id (NULL только «в пути»); маршрут
-- в пути — transit_from_warehouse_id → transit_to_warehouse_id (from нужен для отмены).

ALTER TABLE task_equipment
    ADD COLUMN warehouse_id uuid NULL REFERENCES warehouses(id) ON DELETE RESTRICT,
    ADD COLUMN status text NOT NULL DEFAULT 'at_warehouse',
    ADD COLUMN transit_from_warehouse_id uuid NULL REFERENCES warehouses(id) ON DELETE RESTRICT,
    ADD COLUMN transit_to_warehouse_id uuid NULL REFERENCES warehouses(id) ON DELETE RESTRICT;

-- Существующие строки → склад по умолчанию (первый активный по position; без активных —
-- первый по position) / at_warehouse. Не по имени: склад могли переименовать после 0008.
UPDATE task_equipment SET warehouse_id = coalesce(default_warehouse_id(),
    (SELECT id FROM warehouses ORDER BY position, id LIMIT 1));

-- DEFAULT — для вставок без явного склада (сид, служебные скрипты, D49); код приложения
-- склад передаёт явно.
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

-- Журнал перемещений единицы в рамках заказа. Строка переживает удаление единицы из
-- заказа (журнал), удаляется вместе с заказом/единицей оборудования.
CREATE TABLE task_equipment_movements (
    id                bigserial PRIMARY KEY,
    task_id           bigint NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    equipment_id      uuid NOT NULL REFERENCES equipment(id) ON DELETE CASCADE,
    action            text NOT NULL CHECK (action IN ('received', 'sent', 'arrived', 'cancelled')),
    from_warehouse_id uuid NULL REFERENCES warehouses(id) ON DELETE RESTRICT,
    to_warehouse_id   uuid NULL REFERENCES warehouses(id) ON DELETE RESTRICT,
    user_id           uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT, -- ADR-007: пользователи не удаляются
    created_at        timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX task_equipment_movements_task_equipment_idx ON task_equipment_movements (task_id, equipment_id, id);
CREATE INDEX task_equipment_movements_equipment_id_idx ON task_equipment_movements (equipment_id);
