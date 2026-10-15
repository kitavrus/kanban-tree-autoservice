-- Откат order-start-duration: due_date снова обычная колонка, вводимая вручную, со значением
-- планового окончания (start_date + duration_days − 1); начало и дни на работу удаляются.

ALTER TABLE tasks ALTER COLUMN due_date DROP EXPRESSION;
ALTER TABLE tasks DROP COLUMN start_date, DROP COLUMN duration_days; -- tasks_duration_days_chk — вместе с колонкой
