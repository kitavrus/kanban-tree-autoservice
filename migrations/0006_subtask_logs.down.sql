-- CM-1 = MVP: down — простой DROP в обратном порядке up, без rollback-шага.
ALTER TABLE subtask_products DROP COLUMN IF EXISTS position;
DROP TABLE IF EXISTS subtask_logs;
