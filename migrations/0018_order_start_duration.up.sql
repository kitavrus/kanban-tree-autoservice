-- order-start-duration: срок заказа больше не вводится — заказ планируется датой начала
-- работ (start_date) и числом дней на работу (duration_days, 1–60). Плановое окончание —
-- вычисляемая колонка due_date = start_date + duration_days − 1 (доска: метка срока, фильтр
-- «Горит/просрочено»; календарь загрузки: занятость машиной места).
--
-- Бэкфилл существующих заказов (пояс — Asia/Almaty, pkg/timezone.Default, пояс по умолчанию
-- пользователей): начало = день приёмки (created_at); дни —
--   * прежний срок не раньше приёмки → до него включительно (не больше 60);
--   * срока нет или он раньше приёмки, заказ НЕ выдан (колонка не финальная) → до дня миграции
--     включительно (не больше 60): плановое окончание — сегодня («Горит», но не «просрочено»);
--   * срока нет или он раньше приёмки, заказ выдан (финальная колонка) → 1.

ALTER TABLE tasks ADD COLUMN start_date date, ADD COLUMN duration_days int;

UPDATE tasks SET start_date = (created_at AT TIME ZONE 'Asia/Almaty')::date;
UPDATE tasks t SET duration_days = CASE
    WHEN t.due_date IS NOT NULL AND t.due_date >= t.start_date THEN LEAST(t.due_date - t.start_date + 1, 60)
    WHEN NOT c.is_final THEN LEAST(GREATEST((now() AT TIME ZONE 'Asia/Almaty')::date - t.start_date + 1, 1), 60)
    ELSE 1
END
FROM columns c WHERE c.id = t.column_id;

-- DEFAULT — запасной путь для прямых INSERT (служебные/тестовые); API передаёт оба поля всегда.
-- «Сегодня» — в том же поясе, что и бэкфилл (не в поясе сессии БД).
ALTER TABLE tasks
    ALTER COLUMN start_date SET DEFAULT (now() AT TIME ZONE 'Asia/Almaty')::date,
    ALTER COLUMN start_date SET NOT NULL,
    ALTER COLUMN duration_days SET DEFAULT 1,
    ALTER COLUMN duration_days SET NOT NULL,
    ADD CONSTRAINT tasks_duration_days_chk CHECK (duration_days BETWEEN 1 AND 60);

-- Прежний срок заменяется вычисляемым плановым окончанием (индексов и зависимостей у колонки нет).
ALTER TABLE tasks DROP COLUMN due_date;
ALTER TABLE tasks ADD COLUMN due_date date GENERATED ALWAYS AS (start_date + duration_days - 1) STORED;
