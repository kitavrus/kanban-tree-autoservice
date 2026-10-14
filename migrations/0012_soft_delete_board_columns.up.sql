-- Мягкое удаление — колонки доски (docs/features/soft-delete-board-columns/draft-spec.md §4.1,
-- часть 3 мастер-спеки soft-delete): DS-019 отметка удаления на колонке, DS-030 «кто, когда,
-- почему» хранятся вместе, DS-020 причина 1–500 символов. Режим MVP (DS-022): ALTER поверх
-- 0019; существующие колонки остаются действующими (три поля NULL).

ALTER TABLE columns
    ADD COLUMN deleted_at timestamptz NULL,
    ADD COLUMN deleted_by uuid NULL REFERENCES users(id) ON DELETE RESTRICT,
    ADD COLUMN delete_reason text NULL,
    ADD CONSTRAINT columns_deleted_chk CHECK (
        (deleted_at IS NULL AND deleted_by IS NULL AND delete_reason IS NULL)
        OR (deleted_at IS NOT NULL AND deleted_by IS NOT NULL AND delete_reason IS NOT NULL
            AND char_length(btrim(delete_reason)) BETWEEN 1 AND 500));

-- Список вкладки «Колонки» раздела «Удалённые»: сортировка по времени удаления (§6.3).
CREATE INDEX columns_deleted_at_idx ON columns (deleted_at DESC, id) WHERE deleted_at IS NOT NULL;
