-- Откат: функция склада по умолчанию и полные уникальные индексы названий возвращаются, отметки
-- удаления стираются — мягко удалённые склады и поставщики снова становятся действующими
-- (признак архива is_active не меняется), а «кто, когда, почему» теряются безвозвратно.
--
-- Откат выполним на любых данных. Полный индекс не допускает одинаковых названий, а с частичным
-- (DS-017) название удалённой записи могло быть занято снова. Поэтому перед сменой индекса
-- удалённая запись, чьё название (без учёта регистра и крайних пробелов) занято другой записью,
-- получает суффикс « (удал. N)»: N — наименьший номер от 1, при котором название свободно;
-- записи обходятся по (deleted_at, id). Действующие записи не переименовываются; из нескольких
-- одноимённых удалённых прежнее название сохраняет удалённая последней. Название обрезается
-- справа, только если с суффиксом не помещается в лимит поля (склад — 100, поставщик — 200
-- символов). Повторный up прежние названия НЕ возвращает.
CREATE OR REPLACE FUNCTION default_warehouse_id() RETURNS uuid
    LANGUAGE sql STABLE
    AS $$ SELECT id FROM warehouses WHERE is_active ORDER BY position, id LIMIT 1 $$;

DO $$
DECLARE
    r         record;
    n         integer;
    suffix    text;
    candidate text;
BEGIN
    FOR r IN SELECT id, name FROM suppliers WHERE deleted_at IS NOT NULL ORDER BY deleted_at, id LOOP
        CONTINUE WHEN NOT EXISTS (
            SELECT 1 FROM suppliers o WHERE o.id <> r.id AND lower(btrim(o.name)) = lower(btrim(r.name)));
        n := 0;
        LOOP
            n := n + 1;
            suffix := ' (удал. ' || n || ')';
            candidate := rtrim(left(btrim(r.name), 200 - char_length(suffix))) || suffix;
            EXIT WHEN NOT EXISTS (
                SELECT 1 FROM suppliers o WHERE o.id <> r.id AND lower(btrim(o.name)) = lower(candidate));
        END LOOP;
        UPDATE suppliers SET name = candidate WHERE id = r.id;
    END LOOP;

    FOR r IN SELECT id, name FROM warehouses WHERE deleted_at IS NOT NULL ORDER BY deleted_at, id LOOP
        CONTINUE WHEN NOT EXISTS (
            SELECT 1 FROM warehouses o WHERE o.id <> r.id AND lower(btrim(o.name)) = lower(btrim(r.name)));
        n := 0;
        LOOP
            n := n + 1;
            suffix := ' (удал. ' || n || ')';
            candidate := rtrim(left(btrim(r.name), 100 - char_length(suffix))) || suffix;
            EXIT WHEN NOT EXISTS (
                SELECT 1 FROM warehouses o WHERE o.id <> r.id AND lower(btrim(o.name)) = lower(candidate));
        END LOOP;
        UPDATE warehouses SET name = candidate WHERE id = r.id;
    END LOOP;
END $$;

DROP INDEX IF EXISTS suppliers_name_lower_uq;
CREATE UNIQUE INDEX suppliers_name_lower_uq ON suppliers (lower(btrim(name)));

DROP INDEX IF EXISTS warehouses_name_lower_uq;
CREATE UNIQUE INDEX warehouses_name_lower_uq ON warehouses (lower(btrim(name)));

DROP INDEX IF EXISTS suppliers_deleted_at_idx;
ALTER TABLE suppliers
    DROP CONSTRAINT IF EXISTS suppliers_deleted_chk,
    DROP COLUMN IF EXISTS delete_reason,
    DROP COLUMN IF EXISTS deleted_by,
    DROP COLUMN IF EXISTS deleted_at;

DROP INDEX IF EXISTS warehouses_deleted_at_idx;
ALTER TABLE warehouses
    DROP CONSTRAINT IF EXISTS warehouses_deleted_chk,
    DROP COLUMN IF EXISTS delete_reason,
    DROP COLUMN IF EXISTS deleted_by,
    DROP COLUMN IF EXISTS deleted_at;
