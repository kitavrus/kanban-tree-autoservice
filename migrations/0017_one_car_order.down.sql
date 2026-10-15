-- Откат one-car-order: task_equipment и subtasks.equipment_id восстанавливаются из
-- tasks.car_id (комплектность/состояние — пустые: в 0017 они не хранились).

CREATE TABLE task_equipment (
    task_id      bigint NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    equipment_id uuid NOT NULL REFERENCES equipment(id) ON DELETE RESTRICT,
    completeness text NOT NULL DEFAULT '',
    condition    text NOT NULL DEFAULT '',
    PRIMARY KEY (task_id, equipment_id)
);
CREATE INDEX task_equipment_equipment_id_idx ON task_equipment (equipment_id);
INSERT INTO task_equipment (task_id, equipment_id) SELECT id, car_id FROM tasks;

ALTER TABLE subtasks ADD COLUMN equipment_id uuid NULL REFERENCES equipment(id) ON DELETE RESTRICT;
UPDATE subtasks s SET equipment_id = t.car_id FROM tasks t WHERE t.id = s.task_id;
CREATE INDEX subtasks_equipment_id_idx ON subtasks (equipment_id);

DROP INDEX tasks_car_id_idx;
ALTER TABLE tasks DROP COLUMN car_id;
