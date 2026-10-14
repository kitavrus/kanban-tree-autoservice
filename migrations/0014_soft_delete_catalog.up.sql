-- Мягкое удаление — каталог (товары, услуги, категории услуг)
-- (docs/features/soft-delete-catalog/draft-spec.md §4.1–§4.4, часть 5 мастер-спеки soft-delete):
-- DS-014 отметка удаления рядом с архивом is_active (DS-009: состояния независимы), DS-030 «кто,
-- когда, почему» хранятся вместе, DS-020 причина 1–500 символов, DS-017 название уникально только
-- среди неудалённых. Режим MVP (DS-022): ALTER поверх 0021; существующие записи остаются
-- действующими (три поля NULL), признак архива не меняется.

ALTER TABLE products
    ADD COLUMN deleted_at timestamptz NULL,
    ADD COLUMN deleted_by uuid NULL REFERENCES users(id) ON DELETE RESTRICT,
    ADD COLUMN delete_reason text NULL,
    ADD CONSTRAINT products_deleted_chk CHECK (
        (deleted_at IS NULL AND deleted_by IS NULL AND delete_reason IS NULL)
        OR (deleted_at IS NOT NULL AND deleted_by IS NOT NULL AND delete_reason IS NOT NULL
            AND char_length(btrim(delete_reason)) BETWEEN 1 AND 500));

-- Список вкладки «Товары» раздела «Удалённые»: сортировка по времени удаления (§6.9).
CREATE INDEX products_deleted_at_idx ON products (deleted_at DESC, id) WHERE deleted_at IS NOT NULL;

ALTER TABLE services
    ADD COLUMN deleted_at timestamptz NULL,
    ADD COLUMN deleted_by uuid NULL REFERENCES users(id) ON DELETE RESTRICT,
    ADD COLUMN delete_reason text NULL,
    ADD CONSTRAINT services_deleted_chk CHECK (
        (deleted_at IS NULL AND deleted_by IS NULL AND delete_reason IS NULL)
        OR (deleted_at IS NOT NULL AND deleted_by IS NOT NULL AND delete_reason IS NOT NULL
            AND char_length(btrim(delete_reason)) BETWEEN 1 AND 500));

-- Список вкладки «Услуги» раздела «Удалённые»: сортировка по времени удаления (§6.9).
CREATE INDEX services_deleted_at_idx ON services (deleted_at DESC, id) WHERE deleted_at IS NOT NULL;

ALTER TABLE catalog_categories
    ADD COLUMN deleted_at timestamptz NULL,
    ADD COLUMN deleted_by uuid NULL REFERENCES users(id) ON DELETE RESTRICT,
    ADD COLUMN delete_reason text NULL,
    ADD CONSTRAINT catalog_categories_deleted_chk CHECK (
        (deleted_at IS NULL AND deleted_by IS NULL AND delete_reason IS NULL)
        OR (deleted_at IS NOT NULL AND deleted_by IS NOT NULL AND delete_reason IS NOT NULL
            AND char_length(btrim(delete_reason)) BETWEEN 1 AND 500));

-- Список вкладки «Категории» раздела «Удалённые»: сортировка по времени удаления (§6.9).
CREATE INDEX catalog_categories_deleted_at_idx ON catalog_categories (deleted_at DESC, id) WHERE deleted_at IS NOT NULL;

-- DS-017: название удалённой категории (в пределах вида) свободно — уникальный индекс
-- названий пересоздаётся частичным.
DROP INDEX catalog_categories_kind_name_lower_uq;
CREATE UNIQUE INDEX catalog_categories_kind_name_lower_uq ON catalog_categories (kind, lower(btrim(name)))
    WHERE deleted_at IS NULL;
