-- Перемещения — отмена проведения и удаление (docs/features/transfers-revert/draft-spec.md,
-- часть 6 мастер-спеки transfers): DS-044 сведения о последнем откате на документе (кто,
-- когда, причина), DS-053 причина 1–500 символов. Режим MVP (DS-027): ALTER поверх 0016.
-- Причина движения transfer_unreceive уже разрешена CHECK'ом миграции 0015.

ALTER TABLE transfers
    ADD COLUMN unreceived_by uuid NULL REFERENCES users(id) ON DELETE RESTRICT,
    ADD COLUMN unreceived_at timestamptz NULL,
    ADD COLUMN unreceive_reason text NOT NULL DEFAULT '',
    ADD CONSTRAINT transfers_unreceive_reason_check CHECK (char_length(unreceive_reason) <= 500);
