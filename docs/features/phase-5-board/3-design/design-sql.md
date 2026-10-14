# Design SQL: phase-5-board

> Owner: DDL, миграция `0005_board`, ключевые запросы. Пакет-потребитель — `internal/features/board` (SQL inline-константами в `*_repo.go`, эталон `internal/features/catalog/*_repo.go`; go:embed не используется в проекте). CM-1 = MVP: down = простой DROP.

## Baseline (as-is, затрагиваемое)
- `stock_movements` (`migrations/0004_catalog.up.sql:54-63`): `task_id uuid NULL` без FK; `reason CHECK IN (task_consume, task_return, manual)`; все строки — `manual`, `task_id IS NULL`.
- `clients` / `equipment` / `services` / `products` / `users` — `id uuid DEFAULT gen_random_uuid()`; без изменений схемы.

## Columns Table
```sql
CREATE TABLE columns (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name           text NOT NULL CHECK (btrim(name) <> '' AND char_length(name) <= 60),
    color          text NOT NULL CHECK (color ~ '^#[0-9A-Fa-f]{6}$'),
    is_final       boolean NOT NULL DEFAULT false,
    consumes_stock boolean NOT NULL DEFAULT false,
    position       int NOT NULL CHECK (position >= 1),
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX columns_position_idx ON columns (position, id);

-- Сид D32 (AC-008) — в самой миграции.
INSERT INTO columns (name, color, is_final, consumes_stock, position) VALUES
    ('Новые',          '#94A3B8', false, false, 1),
    ('Согласование',   '#7C3AED', false, false, 2),
    ('В работе',       '#2563EB', false, true,  3),
    ('Ждёт запчасти',  '#D97706', false, true,  4),
    ('Готово',         '#059669', true,  true,  5);
```
- Инвариант «≥1 нефинальная колонка» — сервисом под `FOR UPDATE` всей таблицы (ADR-104), не CHECK.
- `position` не UNIQUE: перенумерация 1..N в транзакции при create/update/delete (ADR-100 §колонки).

## Tasks Table
```sql
CREATE TABLE tasks (
    id               bigserial PRIMARY KEY,                       -- D37: сквозной #N, без переиспользования
    column_id        uuid NOT NULL REFERENCES columns(id) ON DELETE RESTRICT,
    client_id        uuid NOT NULL REFERENCES clients(id) ON DELETE RESTRICT,   -- опора D26 (AC-017)
    title            text NOT NULL CHECK (btrim(title) <> '' AND char_length(title) <= 200),
    description_html text NOT NULL DEFAULT '',                    -- фаза 6; в фазе 5 пишет только seed-demo
    assignee_id      uuid NULL REFERENCES users(id) ON DELETE SET NULL,
    due_date         date NULL,
    is_paid          boolean NOT NULL DEFAULT false,
    paid_at          timestamptz NULL,
    position         numeric NOT NULL,                            -- дробная позиция (ADR-001)
    created_by       uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    version          bigint NOT NULL DEFAULT 1,
    CONSTRAINT tasks_paid_at_chk CHECK (is_paid = (paid_at IS NOT NULL))
);
CREATE INDEX tasks_column_position_idx ON tasks (column_id, position, id);
CREATE INDEX tasks_client_id_idx       ON tasks (client_id);
CREATE INDEX tasks_assignee_id_idx     ON tasks (assignee_id);
```
- Имя FK `tasks_client_id_fkey` (дефолт PG) — ключ маппинга 23503 → `CLIENT_HAS_TASKS` (ADR-005).

## Subtasks Table
```sql
CREATE TABLE subtasks (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id      bigint NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    service_id   uuid NOT NULL REFERENCES services(id) ON DELETE RESTRICT,
    status       text NOT NULL DEFAULT 'new' CHECK (status IN ('new', 'in_progress', 'done')),
    assignee_id  uuid NULL REFERENCES users(id) ON DELETE SET NULL,
    due_date     date NULL,
    price        numeric(12,2) NULL CHECK (price IS NULL OR price >= 0),
    equipment_id uuid NULL REFERENCES equipment(id) ON DELETE RESTRICT,  -- опора D26 (AC-018)
    position     int NOT NULL DEFAULT 0,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX subtasks_task_id_idx      ON subtasks (task_id);
CREATE INDEX subtasks_service_id_idx   ON subtasks (service_id);
CREATE INDEX subtasks_equipment_id_idx ON subtasks (equipment_id);
CREATE INDEX subtasks_assignee_id_idx  ON subtasks (assignee_id);
```

## Subtask Products Table
```sql
CREATE TABLE subtask_products (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    subtask_id uuid NOT NULL REFERENCES subtasks(id) ON DELETE CASCADE,
    product_id uuid NOT NULL REFERENCES products(id) ON DELETE RESTRICT,
    qty        numeric(12,3) NOT NULL CHECK (qty > 0),
    price      numeric(12,2) NULL CHECK (price IS NULL OR price >= 0)
);
CREATE INDEX subtask_products_subtask_id_idx ON subtask_products (subtask_id);
CREATE INDEX subtask_products_product_id_idx ON subtask_products (product_id);
```

## Task Stock Table
```sql
CREATE TABLE task_stock (
    task_id    bigint NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    product_id uuid NOT NULL REFERENCES products(id) ON DELETE RESTRICT,
    qty        numeric(12,3) NOT NULL CHECK (qty > 0),   -- 0 → строка удаляется
    PRIMARY KEY (task_id, product_id)
);
CREATE INDEX task_stock_product_id_idx ON task_stock (product_id);
```

## Task Equipment Table
```sql
CREATE TABLE task_equipment (
    task_id      bigint NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    equipment_id uuid NOT NULL REFERENCES equipment(id) ON DELETE RESTRICT,
    completeness text NOT NULL DEFAULT '',
    condition    text NOT NULL DEFAULT '',
    PRIMARY KEY (task_id, equipment_id)
);
CREATE INDEX task_equipment_equipment_id_idx ON task_equipment (equipment_id);
```

## Stock Movements Migration
```sql
-- task_id: uuid → bigint. Данных с task_id нет (все строки manual, D44) — USING NULL.
ALTER TABLE stock_movements ALTER COLUMN task_id TYPE bigint USING NULL;
ALTER TABLE stock_movements
    ADD CONSTRAINT stock_movements_task_id_fkey
    FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE SET NULL;   -- SD-007
CREATE INDEX stock_movements_task_id_idx ON stock_movements (task_id);
```

## Board Migration
Файлы: `migrations/0005_board.up.sql`, `migrations/0005_board.down.sql`. Порядок up: Columns (+сид) → Tasks → Subtasks → Subtask Products → Task Stock → Task Equipment → Stock Movements Migration.
```sql
-- 0005_board.down.sql (CM-1 = MVP)
DROP INDEX IF EXISTS stock_movements_task_id_idx;
ALTER TABLE stock_movements DROP CONSTRAINT IF EXISTS stock_movements_task_id_fkey;
ALTER TABLE stock_movements ALTER COLUMN task_id TYPE uuid USING NULL;
DROP TABLE IF EXISTS task_equipment;
DROP TABLE IF EXISTS task_stock;
DROP TABLE IF EXISTS subtask_products;
DROP TABLE IF EXISTS subtasks;
DROP TABLE IF EXISTS tasks;
DROP TABLE IF EXISTS columns;
```

## Lock Order
Глобальный порядок блокировок (ADR-104), все операции склада — одна транзакция `READ COMMITTED`:
1. `columns` — целевая колонка `FOR SHARE` (move/PUT/create); вся таблица `ORDER BY id FOR UPDATE` (CRUD колонок, смена флагов).
2. `tasks` — `FOR UPDATE`, несколько — `ORDER BY id`.
3. `products` — `FOR UPDATE ORDER BY id`.
SQLSTATE `40P01`/`40001` → повтор транзакции до 3 раз (`withTx`), затем 500.

## Lock Target Column Query
```sql
SELECT id, is_final, consumes_stock FROM columns WHERE id = $1::uuid FOR SHARE;   -- 0 строк → 400 field column_id
```

## Lock Task Query
```sql
SELECT id, column_id, client_id, title, assignee_id, due_date, is_paid, paid_at, position, version
FROM tasks WHERE id = $1 FOR UPDATE;    -- 0 строк → TASK_NOT_FOUND
```

## Open Subtasks Query
```sql
-- D11/D22: проверка перед переносом в is_final (move и PUT со сменой column_id).
SELECT s.id, sv.name AS service_name, s.status
FROM subtasks s JOIN services sv ON sv.id = s.service_id
WHERE s.task_id = $1 AND s.status <> 'done'
ORDER BY s.position, s.id;              -- ≥1 строка → 409 SUBTASKS_NOT_DONE + details.subtasks[]
```

## Position Query
ADR-001. Соседи читаются только из целевой колонки и не равные самому заказу; «чужой»/несуществующий сосед игнорируется.
```sql
-- $1 target column, $2 task id, $3 prev_id (NULL), $4 next_id (NULL)
WITH prev AS (SELECT position FROM tasks WHERE id = $3 AND column_id = $1 AND id <> $2),
     next AS (SELECT position FROM tasks WHERE id = $4 AND column_id = $1 AND id <> $2),
     last AS (SELECT max(position) AS p FROM tasks WHERE column_id = $1 AND id <> $2)
SELECT CASE
    WHEN (SELECT position FROM prev) IS NOT NULL AND (SELECT position FROM next) IS NOT NULL
         THEN trim_scale(((SELECT position FROM prev) + (SELECT position FROM next)) / 2)
    WHEN (SELECT position FROM prev) IS NOT NULL THEN (SELECT position FROM prev) + 1024
    WHEN (SELECT position FROM next) IS NOT NULL THEN (SELECT position FROM next) - 1024
    ELSE COALESCE((SELECT p FROM last), 0) + 1024
END AS pos,
(SELECT position FROM prev) AS prev_pos, (SELECT position FROM next) AS next_pos;
```
- Ребалансировка, если `prev_pos >= next_pos` ИЛИ `scale(pos) > 20`: перенумеровать колонку и пересчитать pos повторно (одна итерация):
```sql
UPDATE tasks t SET position = r.rn * 1024
FROM (SELECT id, row_number() OVER (ORDER BY position, id) AS rn
      FROM tasks WHERE column_id = $1 AND id <> $2) r
WHERE t.id = r.id;                      -- version НЕ инкрементится (порядок ≠ правка заказа)
```

## Move Task Query
```sql
UPDATE tasks SET column_id = $2::uuid, position = $3::numeric,
       version = version + 1, updated_at = now()
WHERE id = $1
RETURNING id, column_id, client_id, title, assignee_id, due_date, is_paid, paid_at, position, version, created_at, updated_at;
```

## Update Task Query
```sql
-- column_id сменилась → position = append (Position Query с prev/next = NULL); иначе прежняя.
UPDATE tasks SET title = $3, client_id = $4::uuid, column_id = $5::uuid, assignee_id = $6::uuid,
       due_date = $7::date, is_paid = $8,
       paid_at = CASE WHEN $8 AND NOT is_paid THEN now() WHEN NOT $8 THEN NULL ELSE paid_at END,
       position = $9::numeric, version = version + 1, updated_at = now()
WHERE id = $1 AND version = $2
RETURNING ...;                          -- 0 строк при существующем id → TASK_VERSION_CONFLICT
```

## Insert Task Query
```sql
INSERT INTO tasks (column_id, client_id, title, assignee_id, due_date, is_paid, paid_at, position, created_by)
VALUES ($1::uuid, $2::uuid, $3, $4::uuid, $5::date, $6, CASE WHEN $6 THEN now() END, $7::numeric, $8::uuid)
RETURNING id, ..., version;
```

## Reconcile Query
Функция `StockEngine.Reconcile(ctx, tx, taskID int64, targetConsumes bool, userID string)` (ADR-104). Вызывается после блокировки заказа.
```sql
-- 1. Дельты (потребность vs фактически списанное), $2 = targetConsumes
WITH demand AS (
    SELECT sp.product_id, sum(sp.qty) AS qty
    FROM subtask_products sp JOIN subtasks s ON s.id = sp.subtask_id
    WHERE s.task_id = $1 GROUP BY sp.product_id),
target AS (SELECT product_id, qty FROM demand WHERE $2),
cur AS (SELECT product_id, qty FROM task_stock WHERE task_id = $1)
SELECT coalesce(t.product_id, c.product_id) AS product_id,
       coalesce(t.qty, 0) - coalesce(c.qty, 0) AS delta
FROM target t FULL JOIN cur c ON c.product_id = t.product_id
WHERE coalesce(t.qty, 0) <> coalesce(c.qty, 0)
ORDER BY 1;
-- 2. Блокировка товаров (порядок id, D34)
SELECT id FROM products WHERE id = ANY($1::uuid[]) ORDER BY id FOR UPDATE;
-- 3. На каждую дельту d (product p), target = целевое количество (0 при targetConsumes=false):
UPDATE products SET stock = stock - $2::numeric, updated_at = now() WHERE id = $1::uuid;   -- $2 = d
-- target > 0:
INSERT INTO task_stock (task_id, product_id, qty) VALUES ($1, $2::uuid, $3::numeric)        -- $3 = target
    ON CONFLICT (task_id, product_id) DO UPDATE SET qty = EXCLUDED.qty;
-- target = 0:
DELETE FROM task_stock WHERE task_id = $1 AND product_id = $2::uuid;
-- журнал ($3 = d):
INSERT INTO stock_movements (product_id, task_id, qty_delta, reason, user_id)
VALUES ($1::uuid, $2, -$3::numeric,
        CASE WHEN $3 > 0 THEN 'task_consume' ELSE 'task_return' END, $4::uuid);
-- 4. Warnings (Q-006) — по затронутым товарам, значения после UPDATE
SELECT id, name, stock::text, min_stock::text,
       CASE WHEN stock < 0 THEN 'negative' ELSE 'below_min' END AS kind
FROM products WHERE id = ANY($1::uuid[]) AND stock <= min_stock ORDER BY name, id;
```
- Инвариант после Reconcile: `task_stock(task) = target` (строк с qty=0 нет); Σ `stock_movements.qty_delta` по заказу = −Σ `task_stock.qty`.
- `stock` NUMERIC(12,3): переполнение 22003 → 500 + `stock_reconcile_errors_total` (MVP; объёмы мастерской малы).

## Board List Query
Q-003 (ADR-003). Один запрос карточек + `count(*) OVER ()` для `total`.
```sql
SELECT t.id, t.column_id, t.client_id, cl.company_name, t.title, t.assignee_id, u.name,
       t.due_date, t.is_paid, t.position, t.version,
       (SELECT count(*) FROM subtasks s WHERE s.task_id = t.id)                       AS subtasks_total,
       (SELECT count(*) FROM subtasks s WHERE s.task_id = t.id AND s.status = 'done') AS subtasks_done,
       (SELECT count(*) FROM subtasks s WHERE s.task_id = t.id AND s.status <> 'done'
                                           AND s.due_date < current_date)             AS subtasks_late,
       count(*) OVER () AS total
FROM tasks t
JOIN columns c  ON c.id = t.column_id
JOIN clients cl ON cl.id = t.client_id
LEFT JOIN users u ON u.id = t.assignee_id
WHERE ($1::uuid IS NULL OR t.client_id = $1)                                               -- client_id
  AND ($2::text IS NULL OR t.title ILIKE '%' || $2 || '%' ESCAPE '\' OR cl.company_name ILIKE '%' || $2 || '%' ESCAPE '\'
       OR t.id = $3::bigint)                                                                -- q (+#N)
  AND ($4::uuid IS NULL OR EXISTS (SELECT 1 FROM subtasks s JOIN services sv ON sv.id = s.service_id
                                   WHERE s.task_id = t.id AND sv.category_id = $4))        -- category_id
  AND ($5::boolean IS NULL OR t.is_paid = $5)                                              -- paid
  AND ($6::uuid IS NULL OR t.assignee_id = $6
       OR EXISTS (SELECT 1 FROM subtasks s WHERE s.task_id = t.id AND s.assignee_id = $6)) -- assignee (AC-014)
  AND ($7::text IS NULL OR (NOT c.is_final AND (
         ($7 = 'overdue' AND t.due_date < current_date)
      OR ($7 = 'soon'    AND t.due_date <= current_date + 1)
      OR EXISTS (SELECT 1 FROM subtasks s WHERE s.task_id = t.id AND s.status <> 'done'
                 AND s.due_date < current_date))))                                          -- due
ORDER BY c.position, c.id, t.position, t.id
LIMIT $8 OFFSET $9;
```
- `q` экранируется (`\`, `%`, `_`) эталоном `pkg/sqlutil`; `$3` = число, если `q` ~ `^#?\d{1,18}$`, иначе NULL.

## Stock Pending CTE
Q-002 (ADR-002). Общий CTE для списка, KPI и бейджа.
```sql
WITH pending AS (
    SELECT sp.product_id, sum(sp.qty) AS qty
    FROM subtask_products sp
    JOIN subtasks s ON s.id = sp.subtask_id
    JOIN tasks t    ON t.id = s.task_id
    JOIN columns c  ON c.id = t.column_id
    WHERE NOT c.consumes_stock AND NOT c.is_final
    GROUP BY sp.product_id),
rows AS (
    SELECT p.id, p.name, p.unit, p.box, p.place, p.stock, p.min_stock,
           coalesce(pd.qty, 0) AS pending, p.stock - coalesce(pd.qty, 0) AS forecast,
           CASE WHEN p.stock <= 0 THEN 'out'
                WHEN p.stock <= p.min_stock THEN 'low'
                WHEN p.stock - coalesce(pd.qty, 0) <= p.min_stock THEN 'will_low'
                ELSE 'ok' END AS state
    FROM products p LEFT JOIN pending pd ON pd.product_id = p.id
    WHERE p.is_active)
```

## Stock List Query
```sql
<Stock Pending CTE>
SELECT *, count(*) OVER () AS total FROM rows
WHERE ($1 = 'all' OR state <> 'ok')
  AND ($2::text IS NULL OR name ILIKE '%' || $2 || '%' ESCAPE '\')
ORDER BY stock - min_stock, name, id
LIMIT $3 OFFSET $4;
```

## Stock Summary Query
```sql
<Stock Pending CTE>
SELECT count(*)                                  AS products_total,
       count(*) FILTER (WHERE state = 'low')      AS low_count,
       count(*) FILTER (WHERE state = 'out')      AS out_count,
       count(*) FILTER (WHERE state = 'will_low') AS will_low_count,
       count(*) FILTER (WHERE state <> 'ok')      AS risk_count     -- бейдж меню (AC-016)
FROM rows;
```

## Stock Adjust Query
D44, эталон `catalog.ProductRepo.Update` (`products_repo.go:141-195`).
```sql
SELECT stock::text FROM products WHERE id = $1::uuid FOR UPDATE;          -- 0 строк → PRODUCT_NOT_FOUND
UPDATE products SET stock = $2::numeric, updated_at = now() WHERE id = $1::uuid;
INSERT INTO stock_movements (product_id, qty_delta, reason, user_id)
VALUES ($1::uuid, $2::numeric - $3::numeric, 'manual', $4::uuid);        -- только если дельта ≠ 0
```

## Column Recalc Query
AC-007: смена `consumes_stock` (после `SELECT ... FROM columns ORDER BY id FOR UPDATE`).
```sql
SELECT id FROM tasks WHERE column_id = $1::uuid ORDER BY id FOR UPDATE;
-- для каждого task: Reconcile(tx, id, newConsumes, userID); warnings — объединение по product_id
```

## Column Renumber Query
```sql
-- после insert/update(position)/delete; $1 = id двигаемой колонки, $2 = целевая позиция (1..N, clamp)
WITH ordered AS (
    SELECT id, row_number() OVER (ORDER BY position, (id = $1) DESC, id) AS rn FROM columns WHERE id <> $1)
UPDATE columns c SET position = CASE WHEN o.rn >= $2 THEN o.rn + 1 ELSE o.rn END
FROM ordered o WHERE c.id = o.id;
UPDATE columns SET position = $2 WHERE id = $1;
```

## Delete Column Query
```sql
SELECT EXISTS (SELECT 1 FROM tasks WHERE column_id = $1::uuid);   -- true → 409 COLUMN_NOT_EMPTY
SELECT count(*) FROM columns WHERE NOT is_final AND id <> $1::uuid;  -- 0 и удаляемая нефинальная → 409 LAST_NON_FINAL_COLUMN
DELETE FROM columns WHERE id = $1::uuid;                           -- 23503 (гонка) → COLUMN_NOT_EMPTY
```

## Delete Task Query
```sql
-- под FOR UPDATE заказа: Reconcile(target=false) → возврат всех task_stock
DELETE FROM tasks WHERE id = $1;        -- CASCADE subtasks/subtask_products/task_stock/task_equipment; stock_movements.task_id → NULL
```

## Client Delete Guard Query
Пакет `clients` (ADR-005), без импорта board.
```sql
SELECT EXISTS (SELECT 1 FROM tasks WHERE client_id = $1::uuid);    -- true → 409 CLIENT_HAS_TASKS
```

## Equipment Delete Guard Query
```sql
SELECT EXISTS (SELECT 1 FROM task_equipment WHERE equipment_id = $1::uuid)
    OR EXISTS (SELECT 1 FROM subtasks WHERE equipment_id = $1::uuid);   -- true → 409 EQUIPMENT_USED_IN_TASKS
```

## Assignees Query
```sql
SELECT id, name, role, count(*) OVER () FROM users WHERE is_active ORDER BY name, id LIMIT $1 OFFSET $2;
```

## Seed Demo Guard Query
```sql
SELECT count(*) FROM tasks;             -- > 0 → «заказы уже есть», exit 0 (AC-019)
SELECT id FROM users WHERE role = 'admin' AND is_active ORDER BY created_at LIMIT 1;  -- нет → exit 1 «make local-up-seed»
```
