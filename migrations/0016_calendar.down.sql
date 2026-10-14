DROP INDEX IF EXISTS tasks_finished_at_idx;
DROP TRIGGER IF EXISTS tasks_finished_at_trg ON tasks;
DROP FUNCTION IF EXISTS tasks_set_finished_at();
ALTER TABLE tasks DROP COLUMN IF EXISTS finished_at;
DROP TABLE IF EXISTS shop_settings;
