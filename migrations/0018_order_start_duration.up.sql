-- order-start-duration: срок заказа больше не вводится — заказ планируется датой начала
-- работ (start_date) и числом дней на работу (duration_days, 1–60). Плановое окончание —
-- вычисляемая колонка due_date = start_date + duration_days − 1 (доска: метка срока, фильтр
-- «Горит/просрочено»; календарь загрузки: занятость машиной места).
--
-- Бэкфилл существующих заказов: начало = день приёмки (created_at в поясе Asia/Almaty —
-- pkg/timezone.Default, пояс по умолчанию пользователей); дни — до прежнего срока
-- включительно (не больше 60), без срока либо со сроком раньше приёмки — 1.

ALTER TABLE tasks ADD COLUMN start_date date, ADD COLUMN duration_days int;

UPDATE tasks SET start_date = (created_at AT TIME ZONE 'Asia/Almaty')::date;
UPDATE tasks SET duration_days = CASE
    WHEN due_date IS NOT NULL AND due_date >= start_date THEN LEAST(due_date - start_date + 1, 60)
    ELSE 1
END;

-- DEFAULT — запасной путь для прямых INSERT (служебные/тестовые); API передаёт оба поля всегда.
ALTER TABLE tasks
    ALTER COLUMN start_date SET DEFAULT CURRENT_DATE,
    ALTER COLUMN start_date SET NOT NULL,
    ALTER COLUMN duration_days SET DEFAULT 1,
    ALTER COLUMN duration_days SET NOT NULL,
    ADD CONSTRAINT tasks_duration_days_chk CHECK (duration_days BETWEEN 1 AND 60);

-- Прежний срок заменяется вычисляемым плановым окончанием (индексов и зависимостей у колонки нет).
ALTER TABLE tasks DROP COLUMN due_date;
ALTER TABLE tasks ADD COLUMN due_date date GENERATED ALWAYS AS (start_date + duration_days - 1) STORED;
