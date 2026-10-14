# Design SQL: phase-6-order-card

> Owner: DDL, миграция, запросы. SQL — Go-константы в `*_repo.go` пакета `board` (as-is стиль фазы 5, без go:embed). Lock Order — §Lock Order (расширение ADR-104 фазы 5).

## Subtask Logs Table
Новая. DDL — §Subtask Logs Migration. `user_id` RESTRICT (ADR-007): пользователи не удаляются (фича `users` — только блокировка).

## Subtasks Table
As-is (`migrations/0005_board.up.sql:46-62`), без изменений DDL:
```sql
subtasks(id uuid PK DEFAULT gen_random_uuid(), task_id bigint FK tasks CASCADE, service_id uuid FK services RESTRICT,
  status text DEFAULT 'new' CHECK IN ('new','in_progress','done'), assignee_id uuid FK users SET NULL, due_date date NULL,
  price numeric(12,2) NULL CHECK(price IS NULL OR price>=0), equipment_id uuid FK equipment RESTRICT NULL,
  position int DEFAULT 0, created_at, updated_at)
-- индексы: subtasks_task_id_idx, subtasks_service_id_idx, subtasks_equipment_id_idx, subtasks_assignee_id_idx
```

## Subtask Products Table
As-is (`0005_board.up.sql:64-72`) + колонка `position` (ADR-103):
```sql
subtask_products(id uuid PK DEFAULT gen_random_uuid(), subtask_id uuid FK subtasks CASCADE, product_id uuid FK products RESTRICT,
  qty numeric(12,3) CHECK(qty>0), price numeric(12,2) NULL CHECK(price IS NULL OR price>=0))
-- индексы: subtask_products_subtask_id_idx, subtask_products_product_id_idx
```

## Task Equipment Table
As-is (`0005_board.up.sql:82-89`): `task_equipment(task_id bigint FK tasks CASCADE, equipment_id uuid FK equipment RESTRICT, completeness text DEFAULT '', condition text DEFAULT '', PK(task_id, equipment_id))`, индекс `task_equipment_equipment_id_idx`. Без изменений.

## Tasks Table
As-is. `description_html text DEFAULT ''` (`0005_board.up.sql:30`) начинает писаться (ADR-006). `version` строками НЕ инкрементируется (ADR-002).

## Subtask Logs Migration
`migrations/0006_subtask_logs.up.sql` (применение — только `make migrate-up`):
```sql
CREATE TABLE subtask_logs (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    subtask_id uuid NOT NULL REFERENCES subtasks(id) ON DELETE CASCADE,
    user_id    uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    text       text NOT NULL CHECK (btrim(text) <> '' AND char_length(text) <= 4000),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX subtask_logs_subtask_created_idx ON subtask_logs (subtask_id, created_at);
CREATE INDEX subtask_logs_user_id_idx ON subtask_logs (user_id);

ALTER TABLE subtask_products ADD COLUMN position int NOT NULL DEFAULT 0;
-- backfill (ADR-103): существующие строки (seed-demo, dev-БД фазы 5) — 1..N внутри услуги
UPDATE subtask_products sp SET position = o.rn
FROM (SELECT id, row_number() OVER (PARTITION BY subtask_id ORDER BY product_id, id) AS rn FROM subtask_products) o
WHERE o.id = sp.id;
```
`migrations/0006_subtask_logs.down.sql` (CM-1 MVP — простой DROP):
```sql
ALTER TABLE subtask_products DROP COLUMN IF EXISTS position;
DROP TABLE IF EXISTS subtask_logs;
```

## Lock Order
ADR-104 фазы 5 расширен (ADR-100): **columns (FOR SHARE) → tasks (FOR UPDATE) → subtasks (FOR UPDATE) → subtask_products (FOR UPDATE) → products (FOR UPDATE ORDER BY id, внутри Reconcile)**.
- Мутации услуг/товаров (вкл. смену статуса услуги — LOW фазы 5): Task Column Id → Lock Target Column (as-is) → Lock Task (as-is) → сверка `column_id` (иначе `errTaskMovedRetry`) → Lock Subtask / Lock Subtask Product → правка → `Reconcile(taskID, consumes_stock)` при изменении состава товаров.
- Оборудование: Lock Task (без колонки: колонку не читаем) → правка.
- Журнал: без блокировки заказа (склад не трогает); FK-проверка берёт KEY SHARE на `subtasks` — цикла с DELETE услуги (tasks→subtasks) нет.

## Task Column Id Query
```sql
SELECT column_id FROM tasks WHERE id = $1   -- 0 строк → TASK_NOT_FOUND
```

## Subtask Parent Query
```sql
SELECT task_id FROM subtasks WHERE id = $1                                  -- 0 → SUBTASK_NOT_FOUND
SELECT s.task_id, sp.subtask_id FROM subtask_products sp JOIN subtasks s ON s.id = sp.subtask_id
WHERE sp.id = $1                                                            -- 0 → SUBTASK_PRODUCT_NOT_FOUND
```

## Lock Subtask Query
```sql
SELECT id, status, assignee_id::text FROM subtasks WHERE id = $1 AND task_id = $2 FOR UPDATE
-- 0 строк (удалена/перенесена параллельно) → SUBTASK_NOT_FOUND
SELECT id FROM subtask_products WHERE id = $1 AND subtask_id = $2 FOR UPDATE -- 0 → SUBTASK_PRODUCT_NOT_FOUND
```

## Insert Subtask Query
Одна tx (AC-007, AC-008): услуга + копия комплекта.
```sql
-- $1 task_id, $2 service_id; 0 строк → 400 service_id (сервис заранее различает «не найдена» / «в архиве»)
INSERT INTO subtasks (task_id, service_id, price, position)
SELECT $1, sv.id, sv.price, coalesce((SELECT max(position) FROM subtasks WHERE task_id = $1), 0) + 1
FROM services sv WHERE sv.id = $2 AND sv.is_active
RETURNING id;

-- $1 subtask_id, $2 service_id: комплект с текущей ценой товара
INSERT INTO subtask_products (subtask_id, product_id, qty, price, position)
SELECT $1, spr.product_id, spr.qty, p.price, row_number() OVER (ORDER BY spr.position, spr.product_id)
FROM service_products spr JOIN products p ON p.id = spr.product_id
WHERE spr.service_id = $2;
```

## Update Subtask Query
```sql
UPDATE subtasks SET status = $2, assignee_id = $3, due_date = $4, price = $5::numeric, equipment_id = $6, updated_at = now()
WHERE id = $1
```
`equipment_id` проверяется до UPDATE: `SELECT 1 FROM task_equipment WHERE task_id = $1 AND equipment_id = $2`.

## Delete Subtask Query
```sql
DELETE FROM subtasks WHERE id = $1   -- CASCADE: subtask_products, subtask_logs
```

## Subtask Product Queries
```sql
-- Insert ($1 subtask_id, $2 product_id, $3 qty); 0 строк → 400 product_id
INSERT INTO subtask_products (subtask_id, product_id, qty, price, position)
SELECT $1, p.id, $3::numeric, p.price, coalesce((SELECT max(position) FROM subtask_products WHERE subtask_id = $1), 0) + 1
FROM products p WHERE p.id = $2 AND p.is_active
RETURNING id;
-- Update (LWW, D28)
UPDATE subtask_products SET qty = $2::numeric, price = $3::numeric WHERE id = $1;
-- Delete
DELETE FROM subtask_products WHERE id = $1;
```
После каждой — `StockEngine.Reconcile(tx, taskID, column.consumes_stock, actorID)` (as-is, `stock_engine.go`).

## Task Equipment Queries
```sql
-- Owner check ($1 equipment_id): client_id для сравнения с tasks.client_id (из LockTask)
SELECT client_id FROM equipment WHERE id = $1;
-- Insert (идемпотентно)
INSERT INTO task_equipment (task_id, equipment_id, completeness, condition) VALUES ($1, $2, $3, $4)
ON CONFLICT (task_id, equipment_id) DO NOTHING;
-- Update; 0 строк → TASK_EQUIPMENT_NOT_FOUND
UPDATE task_equipment SET completeness = $3, condition = $4 WHERE task_id = $1 AND equipment_id = $2;
-- In-use (удаление единицы): непустой → 409 TASK_EQUIPMENT_IN_USE details.subtasks
SELECT s.id, sv.name, s.status FROM subtasks s JOIN services sv ON sv.id = s.service_id
WHERE s.task_id = $1 AND s.equipment_id = $2 ORDER BY s.position, s.id;
-- Delete; 0 строк → TASK_EQUIPMENT_NOT_FOUND
DELETE FROM task_equipment WHERE task_id = $1 AND equipment_id = $2;
```

## Foreign Equipment Query
`PUT /tasks/:id` со сменой клиента, после LockTask, до UPDATE tasks (AC-004):
```sql
-- $1 task_id, $2 new client_id. Непустой → 409 TASK_EQUIPMENT_IN_USE details.subtasks
SELECT s.id, sv.name, s.status
FROM subtasks s JOIN services sv ON sv.id = s.service_id JOIN equipment e ON e.id = s.equipment_id
WHERE s.task_id = $1 AND e.client_id <> $2 ORDER BY s.position, s.id;
-- иначе
DELETE FROM task_equipment te USING equipment e
WHERE te.task_id = $1 AND e.id = te.equipment_id AND e.client_id <> $2;
```

## Subtask Log Queries
```sql
-- Insert ($1 subtask_id, $2 user_id, $3 text); 23503 subtask_id → SUBTASK_NOT_FOUND
INSERT INTO subtask_logs (subtask_id, user_id, text) VALUES ($1, $2, $3) RETURNING id;
-- Author/parent: 0 → SUBTASK_LOG_NOT_FOUND
SELECT l.user_id, s.task_id FROM subtask_logs l JOIN subtasks s ON s.id = l.subtask_id WHERE l.id = $1;
-- Update / Delete (после проверки автора); 0 строк → SUBTASK_LOG_NOT_FOUND
UPDATE subtask_logs SET text = $2, updated_at = now() WHERE id = $1;
DELETE FROM subtask_logs WHERE id = $1;
```

## Totals Expression
Каноничная формула (ADR-005, AC-013), аддитивна на всех уровнях — сумма отображённых строк = итог:
- `line_total = round(sp.qty * coalesce(sp.price, 0), 2)`
- `subtask_total = coalesce(s.price, 0) + Σ line_total` (услуга «по факту» — работа 0, товары входят)
- `task_total = Σ subtask_total`
```sql
-- фрагмент-скаляр, коррелирован с t.id (Board List / Get / Card)
(SELECT coalesce(sum(coalesce(s.price, 0) +
        coalesce((SELECT sum(round(sp.qty * coalesce(sp.price, 0), 2))
                  FROM subtask_products sp WHERE sp.subtask_id = s.id), 0)), 0)
 FROM subtasks s WHERE s.task_id = t.id)::text AS total_amount
```

## Last Log Expression
```sql
LEFT JOIN LATERAL (
  SELECT left(l.text, 120) AS text, char_length(l.text) > 120 AS cut, u.name AS author_name, l.created_at
  FROM subtasks s JOIN subtask_logs l ON l.subtask_id = s.id JOIN users u ON u.id = l.user_id
  WHERE s.task_id = t.id ORDER BY l.created_at DESC, l.id DESC LIMIT 1) ll ON true
```
Go: `cut` → добавить «…». Индексы: `subtasks_task_id_idx` + `subtask_logs_subtask_created_idx`.

## Board List Query
As-is `boardListSQL` (`tasks_repo.go`) + в SELECT: Totals Expression, `ll.text, ll.cut, ll.author_name, ll.created_at` (Last Log Expression — JOIN после `LEFT JOIN users u`). Фильтры/порядок/`LIMIT 500` — as-is. `TaskRepo.Get` — те же добавки + `t.description_html`. Сумма колонки — клиент (ADR-005).

## Card Queries
`GET /tasks/:id/card` — 7 запросов на пуле (без tx, read committed), без N+1:
```sql
-- 1. шапка: TaskRepo.Get (as-is + description_html/total_amount/last_log)
-- 2. колонка
SELECT id, name, is_final, consumes_stock FROM columns WHERE id = $1;
-- 3. оборудование + история (AC-006)
SELECT te.equipment_id, et.name, e.brand, e.model, e.serial, te.completeness, te.condition,
       coalesce((SELECT json_agg(json_build_object('task_id', o.id, 'title', o.title, 'created_at', o.created_at) ORDER BY o.id DESC)
                 FROM task_equipment x JOIN tasks o ON o.id = x.task_id
                 WHERE x.equipment_id = te.equipment_id AND x.task_id <> te.task_id), '[]'::json)
FROM task_equipment te JOIN equipment e ON e.id = te.equipment_id JOIN equipment_types et ON et.id = e.type_id
WHERE te.task_id = $1 ORDER BY et.name, e.brand, e.model, e.id;
-- 4. услуги
SELECT s.id, s.service_id, sv.name, s.status, s.assignee_id::text, u.name, u.is_active, s.due_date::text,
       s.price::text, s.equipment_id::text, s.position
FROM subtasks s JOIN services sv ON sv.id = s.service_id LEFT JOIN users u ON u.id = s.assignee_id
WHERE s.task_id = $1 ORDER BY s.position, s.id;
-- 5. товары всех услуг
SELECT sp.id, sp.subtask_id, sp.product_id, p.name, p.unit, sp.position, sp.qty::text, sp.price::text,
       round(sp.qty * coalesce(sp.price, 0), 2)::text
FROM subtask_products sp JOIN subtasks s ON s.id = sp.subtask_id JOIN products p ON p.id = sp.product_id
WHERE s.task_id = $1 ORDER BY sp.subtask_id, sp.position, sp.id;
-- 6. журнал всех услуг
SELECT l.id, l.subtask_id, l.text, l.user_id, u.name, l.created_at, l.updated_at
FROM subtask_logs l JOIN subtasks s ON s.id = l.subtask_id JOIN users u ON u.id = l.user_id
WHERE s.task_id = $1 ORDER BY l.subtask_id, l.created_at, l.id;
-- 7. остатки товаров карточки (AC-009)
SELECT p.id, p.stock::text, p.min_stock::text, coalesce(ts.qty, 0)::text
FROM products p LEFT JOIN task_stock ts ON ts.task_id = $1 AND ts.product_id = p.id
WHERE p.id IN (SELECT sp.product_id FROM subtask_products sp JOIN subtasks s ON s.id = sp.subtask_id WHERE s.task_id = $1)
ORDER BY p.id;
```
Go десятичной арифметики не делает: `subtask_total` — доп. скаляр в запросе 4 (`coalesce(s.price,0) + coalesce((SELECT sum(round(sp.qty*coalesce(sp.price,0),2)) FROM subtask_products sp WHERE sp.subtask_id = s.id),0)`), итог карточки — `task.total_amount` из запроса 1 (Totals Expression), отдельного поля нет.

## Print Header Query
```sql
SELECT cl.company_name, cl.contact_name,
       coalesce((SELECT string_agg(ph.phone, ', ' ORDER BY ph.position) FROM client_phones ph WHERE ph.client_id = cl.id), '')
FROM tasks t JOIN clients cl ON cl.id = t.client_id WHERE t.id = $1;
```
`client_phones.position` — as-is (`0003_clients_equipment.up.sql:16`). Остальное — Card Queries.
