-- Мягкое удаление — склады и поставщики
-- (docs/features/soft-delete-warehouses-suppliers/draft-spec.md §4.1, §4.2, часть 6 мастер-спеки
-- soft-delete): DS-021/DS-014 отметка удаления рядом с архивом is_active (DS-009: состояния
-- независимы), DS-030 «кто, когда, почему» хранятся вместе, DS-020 причина 1–500 символов,
-- DS-017 название уникально только среди неудалённых. Режим MVP (DS-022): ALTER поверх 0022;
-- существующие записи остаются действующими (три поля NULL), признак архива не меняется.

ALTER TABLE warehouses
    ADD COLUMN deleted_at timestamptz NULL,
    ADD COLUMN deleted_by uuid NULL REFERENCES users(id) ON DELETE RESTRICT,
    ADD COLUMN delete_reason text NULL,
    ADD CONSTRAINT warehouses_deleted_chk CHECK (
        (deleted_at IS NULL AND deleted_by IS NULL AND delete_reason IS NULL)
        OR (deleted_at IS NOT NULL AND deleted_by IS NOT NULL AND delete_reason IS NOT NULL
            AND char_length(btrim(delete_reason)) BETWEEN 1 AND 500));

-- Список вкладки «Склады» раздела «Удалённые»: сортировка по времени удаления (§6.5).
CREATE INDEX warehouses_deleted_at_idx ON warehouses (deleted_at DESC, id) WHERE deleted_at IS NOT NULL;

ALTER TABLE suppliers
    ADD COLUMN deleted_at timestamptz NULL,
    ADD COLUMN deleted_by uuid NULL REFERENCES users(id) ON DELETE RESTRICT,
    ADD COLUMN delete_reason text NULL,
    ADD CONSTRAINT suppliers_deleted_chk CHECK (
        (deleted_at IS NULL AND deleted_by IS NULL AND delete_reason IS NULL)
        OR (deleted_at IS NOT NULL AND deleted_by IS NOT NULL AND delete_reason IS NOT NULL
            AND char_length(btrim(delete_reason)) BETWEEN 1 AND 500));

-- Список вкладки «Поставщики» раздела «Удалённые»: сортировка по времени удаления (§6.5).
CREATE INDEX suppliers_deleted_at_idx ON suppliers (deleted_at DESC, id) WHERE deleted_at IS NOT NULL;

-- DS-017: название удалённого склада и поставщика свободно — уникальные индексы названий
-- пересоздаются частичными.
DROP INDEX warehouses_name_lower_uq;
CREATE UNIQUE INDEX warehouses_name_lower_uq ON warehouses (lower(btrim(name)))
    WHERE deleted_at IS NULL;

DROP INDEX suppliers_name_lower_uq;
CREATE UNIQUE INDEX suppliers_name_lower_uq ON suppliers (lower(btrim(name)))
    WHERE deleted_at IS NULL;

-- DS-021: склад по умолчанию — действующий (первый активный неудалённый по position, id).
CREATE OR REPLACE FUNCTION default_warehouse_id() RETURNS uuid
    LANGUAGE sql STABLE
    AS $$ SELECT id FROM warehouses WHERE is_active AND deleted_at IS NULL ORDER BY position, id LIMIT 1 $$;
