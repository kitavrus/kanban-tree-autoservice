-- Откат складов (TS-001): остаток товара = сумма по складам, минимум/коробка/место — со
-- склада «Алматы» (иначе первого по position); task_stock схлопывается суммой по товару.

ALTER TABLE stock_movements DROP CONSTRAINT stock_movements_reason_check;
DELETE FROM stock_movements WHERE reason NOT IN ('task_consume', 'task_return', 'manual');
ALTER TABLE stock_movements ADD CONSTRAINT stock_movements_reason_check
    CHECK (reason IN ('task_consume', 'task_return', 'manual'));
DROP INDEX IF EXISTS stock_movements_warehouse_id_idx;
ALTER TABLE stock_movements DROP COLUMN transfer_id, DROP COLUMN warehouse_id;

CREATE TEMP TABLE task_stock_rollup AS
SELECT task_id, product_id, sum(qty) AS qty FROM task_stock GROUP BY task_id, product_id;
DELETE FROM task_stock;
ALTER TABLE task_stock DROP CONSTRAINT task_stock_pkey;
ALTER TABLE task_stock DROP COLUMN warehouse_id;
ALTER TABLE task_stock ADD PRIMARY KEY (task_id, product_id);
INSERT INTO task_stock (task_id, product_id, qty) SELECT task_id, product_id, qty FROM task_stock_rollup;
DROP TABLE task_stock_rollup;

DROP INDEX IF EXISTS subtask_products_warehouse_id_idx;
ALTER TABLE subtask_products DROP COLUMN warehouse_id;

ALTER TABLE products
    ADD COLUMN stock     numeric(12,3) NOT NULL DEFAULT 0,
    ADD COLUMN min_stock numeric(12,3) NOT NULL DEFAULT 0 CHECK (min_stock >= 0),
    ADD COLUMN box       text NOT NULL DEFAULT '',
    ADD COLUMN place     text NOT NULL DEFAULT '';

UPDATE products p SET stock = s.total
FROM (SELECT product_id, sum(stock) AS total FROM product_stock GROUP BY product_id) s
WHERE s.product_id = p.id;

UPDATE products p SET min_stock = x.min_stock, box = x.box, place = x.place
FROM (
    SELECT DISTINCT ON (ps.product_id) ps.product_id, ps.min_stock, ps.box, ps.place
    FROM product_stock ps JOIN warehouses w ON w.id = ps.warehouse_id
    ORDER BY ps.product_id, (w.name = 'Алматы') DESC, w.position, w.id
) x
WHERE x.product_id = p.id;

DROP TABLE product_stock;
DROP FUNCTION default_warehouse_id();
DROP TABLE warehouses;
