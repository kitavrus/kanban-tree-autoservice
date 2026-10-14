-- Откат перемещений товаров: движения transfer_* остаются в журнале (transfer_id обнуляется —
-- без таблицы ссылка теряет смысл), таблицы перемещений удаляются.
DROP INDEX IF EXISTS stock_movements_transfer_id_idx;
ALTER TABLE stock_movements DROP CONSTRAINT IF EXISTS stock_movements_transfer_id_fkey;
UPDATE stock_movements SET transfer_id = NULL WHERE transfer_id IS NOT NULL;
DROP TABLE stock_transfer_items;
DROP TABLE stock_transfers;
