-- Календарь загрузки сервиса: сколько машин одновременно стоит в ремонте по дням и есть ли
-- свободные места. Вместимость (машин одновременно) — настройка сервиса, 5–12.
-- Машина занимает место со дня приёмки (tasks.created_at) до выдачи — переноса заказа в
-- финальную колонку (tasks.finished_at, ставит триггер), а пока не выдана — до срока
-- (tasks.due_date) или бессрочно, если срока нет или он прошёл.

-- Настройки сервиса — одна строка (id всегда true).
CREATE TABLE shop_settings (
    id         boolean PRIMARY KEY DEFAULT true CHECK (id),
    max_cars   int NOT NULL DEFAULT 8 CHECK (max_cars BETWEEN 5 AND 12),
    updated_at timestamptz NOT NULL DEFAULT now()
);
INSERT INTO shop_settings DEFAULT VALUES;

-- Момент выдачи: заказ попал в финальную колонку. Уход из финальной колонки — снова NULL.
ALTER TABLE tasks ADD COLUMN finished_at timestamptz NULL;
UPDATE tasks t SET finished_at = t.updated_at
  FROM columns c WHERE c.id = t.column_id AND c.is_final;

-- Триггер покрывает все пути смены колонки (создание, перенос, правка, восстановление).
-- Перенос между финальными колонками сохраняет исходный момент выдачи.
CREATE FUNCTION tasks_set_finished_at() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF EXISTS (SELECT 1 FROM columns WHERE id = NEW.column_id AND is_final) THEN
        NEW.finished_at := COALESCE(NEW.finished_at, now());
    ELSE
        NEW.finished_at := NULL;
    END IF;
    RETURN NEW;
END $$;

CREATE TRIGGER tasks_finished_at_trg
    BEFORE INSERT OR UPDATE OF column_id ON tasks
    FOR EACH ROW EXECUTE FUNCTION tasks_set_finished_at();

-- Выборка календаря: незавершённые заказы + выданные в окне просмотра.
CREATE INDEX tasks_finished_at_idx ON tasks (finished_at) WHERE deleted_at IS NULL;
