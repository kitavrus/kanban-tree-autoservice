-- Откат части transfers-sales: документ «склад → клиент» снова везёт только оборудование.
-- Товарные строки накладных удаляются (в модели 0015 товар в этом направлении недопустим);
-- их движения client_out / transfer_return сохраняются, остаток не пересчитывается.

DELETE FROM transfer_items i
USING transfers t
WHERE t.id = i.transfer_id AND t.direction = 'warehouse_to_client' AND i.kind = 'product';

ALTER TABLE transfer_items
    DROP CONSTRAINT IF EXISTS transfer_items_price_check,
    DROP CONSTRAINT transfer_items_kind_check,
    DROP COLUMN price,
    ADD CONSTRAINT transfer_items_kind_check CHECK (
        (kind = 'product' AND product_id IS NOT NULL AND qty_sent IS NOT NULL
            AND equipment_id IS NULL)
     OR (kind = 'equipment' AND equipment_id IS NOT NULL AND product_id IS NULL
            AND qty_sent IS NULL AND qty_received IS NULL));
