-- Перемещения — накладная клиенту (docs/features/transfers-sales/draft-spec.md, часть 5
-- мастер-спеки transfers): DS-011 цена продажи у товарной строки документа «склад → клиент»,
-- DS-039 цена по умолчанию — из каталога (подставляет приложение при сохранении). Режим MVP
-- (DS-027): ALTER поверх 0015. Существующие строки получают price = NULL.

ALTER TABLE transfer_items
    ADD COLUMN price numeric(12,2) NULL;

-- Строка-единица цены не несёт; цена товарной строки вне «склад → клиент» — правило
-- валидатора сервиса (направление живёт в шапке документа, CHECK строки его не видит).
ALTER TABLE transfer_items
    DROP CONSTRAINT transfer_items_kind_check,
    ADD CONSTRAINT transfer_items_kind_check CHECK (
        (kind = 'product' AND product_id IS NOT NULL AND qty_sent IS NOT NULL
            AND equipment_id IS NULL)
     OR (kind = 'equipment' AND equipment_id IS NOT NULL AND product_id IS NULL
            AND qty_sent IS NULL AND qty_received IS NULL AND price IS NULL)),
    ADD CONSTRAINT transfer_items_price_check CHECK (price IS NULL OR price >= 0);
