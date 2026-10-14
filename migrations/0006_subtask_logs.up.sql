-- Журнал работ услуги (фаза 6, AC-012) + позиция товара в комплекте услуги (ADR-103,
-- AC-014) — design-sql.md §Subtask Logs Migration.

CREATE TABLE subtask_logs (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    subtask_id uuid NOT NULL REFERENCES subtasks(id) ON DELETE CASCADE,
    user_id    uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,  -- ADR-007: пользователи не удаляются
    text       text NOT NULL CHECK (btrim(text) <> '' AND char_length(text) <= 4000),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX subtask_logs_subtask_created_idx ON subtask_logs (subtask_id, created_at);
CREATE INDEX subtask_logs_user_id_idx ON subtask_logs (user_id);

ALTER TABLE subtask_products ADD COLUMN position int NOT NULL DEFAULT 0;
-- backfill (ADR-103): существующие строки (seed-demo, dev-БД фазы 5) — 1..N внутри услуги.
UPDATE subtask_products sp SET position = o.rn
FROM (SELECT id, row_number() OVER (PARTITION BY subtask_id ORDER BY product_id, id) AS rn FROM subtask_products) o
WHERE o.id = sp.id;
