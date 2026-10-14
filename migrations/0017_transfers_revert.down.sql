-- Откат части transfers-revert: сведения о последней отмене проведения удаляются; статусы
-- документов и компенсирующие движения transfer_unreceive сохраняются.

ALTER TABLE transfers
    DROP CONSTRAINT IF EXISTS transfers_unreceive_reason_check,
    DROP COLUMN unreceive_reason,
    DROP COLUMN unreceived_at,
    DROP COLUMN unreceived_by;
