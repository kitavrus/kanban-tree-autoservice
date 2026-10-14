-- Мягкое удаление — поставщики: отметка удаления рядом с архивом is_active (состояния
-- независимы), «кто, когда, почему» хранятся вместе, причина 1–500 символов, название
-- уникально только среди неудалённых.

ALTER TABLE suppliers
    ADD COLUMN deleted_at timestamptz NULL,
    ADD COLUMN deleted_by uuid NULL REFERENCES users(id) ON DELETE RESTRICT,
    ADD COLUMN delete_reason text NULL,
    ADD CONSTRAINT suppliers_deleted_chk CHECK (
        (deleted_at IS NULL AND deleted_by IS NULL AND delete_reason IS NULL)
        OR (deleted_at IS NOT NULL AND deleted_by IS NOT NULL AND delete_reason IS NOT NULL
            AND char_length(btrim(delete_reason)) BETWEEN 1 AND 500));

-- Список вкладки «Поставщики» раздела «Удалённые»: сортировка по времени удаления.
CREATE INDEX suppliers_deleted_at_idx ON suppliers (deleted_at DESC, id) WHERE deleted_at IS NOT NULL;

-- Название удалённого поставщика свободно — уникальный индекс названий частичный.
DROP INDEX suppliers_name_lower_uq;
CREATE UNIQUE INDEX suppliers_name_lower_uq ON suppliers (lower(btrim(name)))
    WHERE deleted_at IS NULL;
