-- Мягкое удаление — ядро (docs/features/soft-delete-core/draft-spec.md §4.1, часть 1 мастер-
-- спеки soft-delete): DS-001 отметка удаления на заказе, DS-030 «кто, когда, почему» хранятся
-- вместе, DS-020 причина 1–500 символов. Режим MVP (DS-022): ALTER поверх 0017; существующие
-- заказы остаются действующими (три поля NULL).

ALTER TABLE tasks
    ADD COLUMN deleted_at timestamptz NULL,
    ADD COLUMN deleted_by uuid NULL REFERENCES users(id) ON DELETE RESTRICT,
    ADD COLUMN delete_reason text NULL,
    ADD CONSTRAINT tasks_deleted_chk CHECK (
        (deleted_at IS NULL AND deleted_by IS NULL AND delete_reason IS NULL)
        OR (deleted_at IS NOT NULL AND deleted_by IS NOT NULL AND delete_reason IS NOT NULL
            AND char_length(btrim(delete_reason)) BETWEEN 1 AND 500));

-- Список вкладки «Заказы» раздела «Удалённые»: сортировка по времени удаления (§6.3).
CREATE INDEX tasks_deleted_at_idx ON tasks (deleted_at DESC, id) WHERE deleted_at IS NOT NULL;
