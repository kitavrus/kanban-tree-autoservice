-- Мягкое удаление — документы перемещения (docs/features/soft-delete-transfers/draft-spec.md
-- §4.1, часть 2 мастер-спеки soft-delete): DS-018 отметка удаления на документе (только
-- «Заявка» и «Отменён»), DS-030 «кто, когда, почему» хранятся вместе, DS-020 причина 1–500
-- символов. Режим MVP (DS-022): ALTER поверх 0018; существующие документы остаются
-- действующими (три поля NULL).

ALTER TABLE transfers
    ADD COLUMN deleted_at timestamptz NULL,
    ADD COLUMN deleted_by uuid NULL REFERENCES users(id) ON DELETE RESTRICT,
    ADD COLUMN delete_reason text NULL,
    ADD CONSTRAINT transfers_deleted_chk CHECK (
        ((deleted_at IS NULL AND deleted_by IS NULL AND delete_reason IS NULL)
         OR (deleted_at IS NOT NULL AND deleted_by IS NOT NULL AND delete_reason IS NOT NULL
             AND char_length(btrim(delete_reason)) BETWEEN 1 AND 500))
        AND (deleted_at IS NULL OR status IN ('requested', 'cancelled')));

-- Список вкладки «Перемещения» раздела «Удалённые»: сортировка по времени удаления (§6.3).
CREATE INDEX transfers_deleted_at_idx ON transfers (deleted_at DESC, id) WHERE deleted_at IS NOT NULL;
