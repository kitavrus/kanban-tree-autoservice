-- Откат: полные уникальные индексы названий возвращаются, отметки удаления стираются — мягко
-- удалённые товары, услуги, категории и типы оборудования снова становятся действующими (признак
-- архива is_active не меняется), а «кто, когда, почему» теряются безвозвратно.
--
-- Откат выполним на любых данных. Полный индекс не допускает одинаковых названий, а с частичным
-- (DS-017) название удалённой категории (в пределах вида) или типа могло быть занято снова.
-- Поэтому перед сменой индекса удалённая запись, чьё название (без учёта регистра и крайних
-- пробелов) занято другой записью, получает суффикс « (удал. N)»: N — наименьший номер от 1, при
-- котором название свободно; записи обходятся по (deleted_at, id). Действующие записи не
-- переименовываются; из нескольких одноимённых удалённых прежнее название сохраняет удалённая
-- последней. Название обрезается справа, только если с суффиксом не помещается в 100 символов
-- (лимит формы; в БД лимита на эти названия нет). Повторный up прежние названия НЕ возвращает.
-- Названия товаров и услуг не уникальны — их откат не переименовывает.
DO $$
DECLARE
    r         record;
    n         integer;
    suffix    text;
    candidate text;
BEGIN
    FOR r IN SELECT id, name FROM equipment_types WHERE deleted_at IS NOT NULL ORDER BY deleted_at, id LOOP
        CONTINUE WHEN NOT EXISTS (
            SELECT 1 FROM equipment_types o WHERE o.id <> r.id AND lower(btrim(o.name)) = lower(btrim(r.name)));
        n := 0;
        LOOP
            n := n + 1;
            suffix := ' (удал. ' || n || ')';
            candidate := rtrim(left(btrim(r.name), 100 - char_length(suffix))) || suffix;
            EXIT WHEN NOT EXISTS (
                SELECT 1 FROM equipment_types o WHERE o.id <> r.id AND lower(btrim(o.name)) = lower(candidate));
        END LOOP;
        UPDATE equipment_types SET name = candidate WHERE id = r.id;
    END LOOP;

    FOR r IN SELECT id, kind, name FROM catalog_categories WHERE deleted_at IS NOT NULL ORDER BY deleted_at, id LOOP
        CONTINUE WHEN NOT EXISTS (
            SELECT 1 FROM catalog_categories o
            WHERE o.id <> r.id AND o.kind = r.kind AND lower(btrim(o.name)) = lower(btrim(r.name)));
        n := 0;
        LOOP
            n := n + 1;
            suffix := ' (удал. ' || n || ')';
            candidate := rtrim(left(btrim(r.name), 100 - char_length(suffix))) || suffix;
            EXIT WHEN NOT EXISTS (
                SELECT 1 FROM catalog_categories o
                WHERE o.id <> r.id AND o.kind = r.kind AND lower(btrim(o.name)) = lower(candidate));
        END LOOP;
        UPDATE catalog_categories SET name = candidate WHERE id = r.id;
    END LOOP;
END $$;

DROP INDEX IF EXISTS equipment_types_name_lower_uq;
CREATE UNIQUE INDEX equipment_types_name_lower_uq ON equipment_types (lower(btrim(name)));

DROP INDEX IF EXISTS catalog_categories_kind_name_lower_uq;
CREATE UNIQUE INDEX catalog_categories_kind_name_lower_uq ON catalog_categories (kind, lower(btrim(name)));

DROP INDEX IF EXISTS equipment_types_deleted_at_idx;
ALTER TABLE equipment_types
    DROP CONSTRAINT IF EXISTS equipment_types_deleted_chk,
    DROP COLUMN IF EXISTS delete_reason,
    DROP COLUMN IF EXISTS deleted_by,
    DROP COLUMN IF EXISTS deleted_at;

DROP INDEX IF EXISTS catalog_categories_deleted_at_idx;
ALTER TABLE catalog_categories
    DROP CONSTRAINT IF EXISTS catalog_categories_deleted_chk,
    DROP COLUMN IF EXISTS delete_reason,
    DROP COLUMN IF EXISTS deleted_by,
    DROP COLUMN IF EXISTS deleted_at;

DROP INDEX IF EXISTS services_deleted_at_idx;
ALTER TABLE services
    DROP CONSTRAINT IF EXISTS services_deleted_chk,
    DROP COLUMN IF EXISTS delete_reason,
    DROP COLUMN IF EXISTS deleted_by,
    DROP COLUMN IF EXISTS deleted_at;

DROP INDEX IF EXISTS products_deleted_at_idx;
ALTER TABLE products
    DROP CONSTRAINT IF EXISTS products_deleted_chk,
    DROP COLUMN IF EXISTS delete_reason,
    DROP COLUMN IF EXISTS deleted_by,
    DROP COLUMN IF EXISTS deleted_at;
