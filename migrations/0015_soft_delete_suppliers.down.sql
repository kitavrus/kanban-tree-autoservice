-- Откат: отметки удаления стираются, уникальный индекс названий снова полный.
DROP INDEX IF EXISTS suppliers_name_lower_uq;
CREATE UNIQUE INDEX suppliers_name_lower_uq ON suppliers (lower(btrim(name)));

DROP INDEX IF EXISTS suppliers_deleted_at_idx;
ALTER TABLE suppliers
    DROP CONSTRAINT IF EXISTS suppliers_deleted_chk,
    DROP COLUMN IF EXISTS delete_reason,
    DROP COLUMN IF EXISTS deleted_by,
    DROP COLUMN IF EXISTS deleted_at;
