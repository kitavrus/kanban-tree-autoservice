-- Заказ = ровно один автомобиль (one-car-order). Автомобиль заказа — tasks.car_id (справочник
-- equipment остаётся); прежние task_equipment (несколько единиц на заказ, комплектность и
-- состояние) и выбор автомобиля в услуге (subtasks.equipment_id) убираются. Комплектность и
-- состояние не переносятся — осознанно, пока не нужны.
--
-- tasks.client_id остаётся (фильтры доски, удаление клиентов) и равен владельцу автомобиля —
-- это гарантирует сервис: автомобиль задаётся только при создании заказа и потом не меняется.

ALTER TABLE tasks ADD COLUMN car_id uuid NULL REFERENCES equipment(id) ON DELETE RESTRICT;

-- 1) Автомобиль из task_equipment: единица клиента заказа, с которой чаще всего работали
--    (больше всего услуг заказа с subtasks.equipment_id = она), затем первая по госномеру и id.
--    Остальные единицы такого заказа из него уходят (их история этот заказ теряет — осознанно).
UPDATE tasks t SET car_id = x.equipment_id
FROM (
    SELECT DISTINCT ON (te.task_id) te.task_id, te.equipment_id
    FROM task_equipment te
    JOIN tasks o     ON o.id = te.task_id
    JOIN equipment e ON e.id = te.equipment_id AND e.client_id = o.client_id
    ORDER BY te.task_id,
             (SELECT count(*) FROM subtasks s WHERE s.task_id = te.task_id AND s.equipment_id = te.equipment_id) DESC,
             e.plate_number, e.id
) x
WHERE x.task_id = t.id;

-- 2) Заказ без автомобиля — первый автомобиль клиента заказа (действующий раньше удалённого,
--    затем по госномеру и id).
UPDATE tasks t SET car_id = x.id
FROM (
    SELECT DISTINCT ON (e.client_id) e.client_id, e.id
    FROM equipment e
    ORDER BY e.client_id, e.deleted_at IS NOT NULL, e.plate_number, e.id
) x
WHERE t.car_id IS NULL AND x.client_id = t.client_id;

-- 3) У клиента нет ни одного автомобиля — заводим ему автомобиль с госномером из названия
--    заказа (допустимые символы госномера, пробелы схлопнуты, верхний регистр, ≤ 20
--    символов; пусто — «БЕЗ НОМЕРА»). Одинаковый госномер одного клиента — один автомобиль.
--    Клиент мягко удалён — автомобиль тоже в удалённых «вместе с клиентом» (как при удалении
--    клиента, soft-delete-clients DS-015).
WITH need AS (
    SELECT t.id AS task_id, t.client_id,
           coalesce(nullif(btrim(left(btrim(regexp_replace(
               upper(regexp_replace(t.title, '[^A-Za-zА-Яа-яЁё0-9 -]+', ' ', 'g')),
               '\s+', ' ', 'g')), 20)), ''), 'БЕЗ НОМЕРА') AS plate
    FROM tasks t
    WHERE t.car_id IS NULL
), cars AS (
    INSERT INTO equipment (client_id, plate_number, deleted_at, deleted_by, delete_reason, deleted_with_client)
    SELECT DISTINCT n.client_id, n.plate, cl.deleted_at, cl.deleted_by, cl.delete_reason, cl.deleted_at IS NOT NULL
    FROM need n JOIN clients cl ON cl.id = n.client_id
    RETURNING id, client_id, plate_number
)
UPDATE tasks t SET car_id = c.id
FROM need n JOIN cars c ON c.client_id = n.client_id AND c.plate_number = n.plate
WHERE t.id = n.task_id;

ALTER TABLE tasks ALTER COLUMN car_id SET NOT NULL;
CREATE INDEX tasks_car_id_idx ON tasks (car_id);

DROP TABLE task_equipment;
ALTER TABLE subtasks DROP COLUMN equipment_id; -- индекс subtasks_equipment_id_idx уходит вместе с колонкой
