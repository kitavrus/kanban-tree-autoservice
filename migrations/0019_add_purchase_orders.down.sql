-- Закупки по дефициту: откат 0019 (MVP, DS-030 — учётки поставщиков удаляются вместе с ролью).
DROP TABLE IF EXISTS purchase_order_items;
DROP TABLE IF EXISTS purchase_orders;
DROP SEQUENCE IF EXISTS purchase_orders_number_seq;

DELETE FROM users WHERE role = 'supplier';
DROP INDEX IF EXISTS users_supplier_id_idx;
ALTER TABLE users
    DROP CONSTRAINT IF EXISTS users_supplier_chk,
    DROP COLUMN IF EXISTS supplier_id,
    DROP CONSTRAINT users_role_check;
ALTER TABLE users ADD CONSTRAINT users_role_check CHECK (role IN ('admin', 'executor'));
