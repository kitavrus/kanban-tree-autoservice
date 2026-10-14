-- CM-1 = MVP: down — простой DROP в обратном порядке up.
DROP INDEX IF EXISTS stock_movements_task_id_idx;
ALTER TABLE stock_movements DROP CONSTRAINT IF EXISTS stock_movements_task_id_fkey;
ALTER TABLE stock_movements ALTER COLUMN task_id TYPE uuid USING NULL;
DROP TABLE IF EXISTS task_equipment;
DROP TABLE IF EXISTS task_stock;
DROP TABLE IF EXISTS subtask_products;
DROP TABLE IF EXISTS subtasks;
DROP TABLE IF EXISTS tasks;
DROP TABLE IF EXISTS columns;
