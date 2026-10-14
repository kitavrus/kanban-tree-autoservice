-- Справочник «Поставщики» (docs/features/suppliers/draft-spec.md §4.1, DS-007): для будущих
-- документов «поставщик → склад» (часть transfers-partners). Удаления нет — архив через
-- is_active. Уникальность названия без регистра/крайних пробелов, архивные включены
-- (как warehouses_name_lower_uq). Правка — last-write-wins, колонки версии нет (DS-043).
CREATE TABLE suppliers (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name         text NOT NULL CHECK (btrim(name) <> '' AND char_length(name) <= 200),
    contact_name text NOT NULL DEFAULT '' CHECK (char_length(contact_name) <= 200),
    phone        text NOT NULL DEFAULT '' CHECK (char_length(phone) <= 50),
    address      text NOT NULL DEFAULT '' CHECK (char_length(address) <= 500),
    note         text NOT NULL DEFAULT '' CHECK (char_length(note) <= 1000),
    is_active    boolean NOT NULL DEFAULT true,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX suppliers_name_lower_uq ON suppliers (lower(btrim(name)));
