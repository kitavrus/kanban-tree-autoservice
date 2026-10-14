# Phase Bundle: phase-6-order-card / Phase 01 (backend) — generated 2026-09-27 11:38

> Твой вход — ЭТОТ файл. Upstream-доки (research/prd/design-*) напрямую НЕ читать —
> нужные слайсы дизайна ниже. Код фичи и её тесты — читать/писать можно (твоя фаза).

## Фаза плана (6-plan/code-plan-phase-01-migration-errors-lock.md)
# Phase 01: Миграция 0006, коды 16-22, lock-retry (ADR-100/103)

**Layer:** backend
**Commit**: `feat(phase-6): migration 0006 subtask_logs + position, errors 16-22, lock-retry`

## Files to CREATE
- `migrations/0006_subtask_logs.up.sql` — design-sql.md §Subtask Logs Migration: таблица `subtask_logs` (RESTRICT `user_id`, CHECK текста 1..4000, индексы `subtask_logs_subtask_created_idx`, `subtask_logs_user_id_idx`) + `ALTER TABLE subtask_products ADD COLUMN position` + backfill `row_number() OVER (PARTITION BY subtask_id ORDER BY product_id, id)` (ADR-103)
- `migrations/0006_subtask_logs.down.sql` — design-sql.md §Subtask Logs Migration (DROP COLUMN position, DROP TABLE subtask_logs) — CM-1 MVP, без rollback-шага
- `internal/features/board/subtask_products_position_integration_test.go` — `TestSubtaskProductsPosition_BackfillOnMigrate` (IT-020): down до 0005 → вставка 3 `subtask_products` одной услуги без `position` → up → `position` = 1..3, без нулей/дублей

## Files to MODIFY
- `internal/features/board/errors.go` — добавить блок «фаза 6» (design-errors.md §Sentinel Errors): `ErrSubtaskNotFound`, `ErrSubtaskProductNotFound`, `ErrSubtaskLogNotFound`, `ErrTaskEquipmentNotFound`, `ErrTaskEquipmentInUse`, `ErrSubtaskFinalColumn`, `ErrSubtaskLogForbidden` (Codes 16-22) + непубличный `errTaskMovedRetry` (ADR-100, наружу не течёт)
- `internal/features/board/tx.go` — `withTx` ретраит `errTaskMovedRetry` наравне с 40P01/40001 (×3, design-sql.md §Lock Order)

## Run after
```bash
make build
make lint
make migrate-up
make migrate-down   # затем снова migrate-up — round-trip 0006
```

## Tests in this phase
- Integration: `TestSubtaskProductsPosition_BackfillOnMigrate` (IT-020)

## Definition of Done
- [ ] `make build` — OK
- [ ] `make lint` — 0 ошибок
- [ ] `make migrate-up` / `make migrate-down` / `make migrate-up` — round-trip без ошибок на 0006
- [ ] `BASE=9580596 make test-integration-scoped` — OK (скоуп-дефолт §EE-5; миграция затрагивает `board` — при FULL-триггере весь `./...`)
- [ ] IT-020 зелёный (position 1..3, без нулей — ADR-103)
- [ ] Тесты этой фазы зелёные до перехода (TD-1, fix-before-next)
- [ ] Коды 16-22 — sentinel-ы объявлены `var Err... = errorspkg.New(...)` на пакете (эталон `errors.go:15-22` as-is), `.WithDetail` не мутирует sentinel
- [ ] `errTaskMovedRetry` — непубличный `errors.New`, не экспортируется, не попадает в HTTP-ответ (design-errors.md §Error Mapping: `errTaskMovedRetry`, 40P01, 40001 → ретрай ×3 → далее 500 `INTERNAL`)


## Слайс: 3-design/design-sql.md [full]
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

## Слайс: 3-design/design-errors.md [full]
# Design Errors: phase-6-order-card

> Owner: ЕДИНСТВЕННЫЙ канонический набор кодов фазы 6 (урок design-review фазы 5). Остальные design-файлы ссылаются на коды отсюда, новых не вводят. Коды 01–15 — as-is фазы 5 (`docs/features/phase-5-board/3-design/design-errors.md §Codes 01-15`), не переопределяются. Wire — `{code, message, details?}` (`pkg/errorspkg`), 400 → `details.fields[]`.

## Codes 16-22
Новые sentinel'ы — `internal/features/board/errors.go` (блок «фаза 6»). `.WithDetail` — копия, sentinel не мутируется.

| # | Code | HTTP | Sentinel | Сообщение (дословно) | details | Driver |
|---|---|---|---|---|---|---|
| 16 | `SUBTASK_NOT_FOUND` | 404 | `ErrSubtaskNotFound` | «Услуга не найдена» | — | AC-007 (правка/удаление услуги), AC-014 |
| 17 | `SUBTASK_PRODUCT_NOT_FOUND` | 404 | `ErrSubtaskProductNotFound` | «Товар услуги не найден» | — | AC-014 |
| 18 | `SUBTASK_LOG_NOT_FOUND` | 404 | `ErrSubtaskLogNotFound` | «Запись журнала не найдена» | — | AC-014 |
| 19 | `TASK_EQUIPMENT_NOT_FOUND` | 404 | `ErrTaskEquipmentNotFound` | «Оборудование не найдено в заказе» | — | AC-004, AC-014 |
| 20 | `TASK_EQUIPMENT_IN_USE` | 409 | `ErrTaskEquipmentInUse` | «Оборудование выбрано в услугах заказа — сначала смените его в услугах» | `subtasks[]` (`{id, service_name, status}`, тип `OpenSubtask` as-is) | AC-004 |
| 21 | `SUBTASK_FINAL_COLUMN` | 409 | `ErrSubtaskFinalColumn` | «Заказ в финальной колонке: нельзя добавить услугу или вернуть её из «Выполнена»» | — | AC-011 |
| 22 | `SUBTASK_LOG_FORBIDDEN` | 403 | `ErrSubtaskLogForbidden` | «Можно менять только свои записи» | — | AC-012 |

## Reused Codes
| Code | HTTP | Где в фазе 6 | Driver |
|---|---|---|---|
| `TASK_NOT_FOUND` (01) | 404 | `:id` в `/tasks/:id/card`, `/tasks/:id/subtasks`, `/tasks/:id/equipment*`; SSR печать → 404-страница `ssr.StatusPages.NotFound` | AC-001, AC-015 |
| `TASK_VERSION_CONFLICT` (06) | 409 | `PUT /tasks/:id` (as-is, расширен `description`) | AC-002 |
| `PAID_ADMIN_ONLY` (11) | 403 | as-is; порядок: version → paid → прочее | AC-003 |
| `FORBIDDEN` (12) | 403 | `DELETE /tasks/:id` executor (as-is) | AC-017 |
| `VALIDATION` (13) | 400 | поля — §Validation Fields | AC-004, AC-007, AC-008, AC-012 |
| `BAD_REQUEST` (14) | 400 | мусорное тело, лишнее поле (strictJSON), `qty:"5"` строкой | — (TD-4) |
| `UNAUTHORIZED` / CSRF | 401 / 403 | каркас `RequireAPI` as-is | AC-017 |

## Sentinel Errors
```go
// internal/features/board/errors.go — фаза 6 (Codes 16-22)
ErrSubtaskNotFound        = errorspkg.New("SUBTASK_NOT_FOUND", http.StatusNotFound, "Услуга не найдена")
ErrSubtaskProductNotFound = errorspkg.New("SUBTASK_PRODUCT_NOT_FOUND", http.StatusNotFound, "Товар услуги не найден")
ErrSubtaskLogNotFound     = errorspkg.New("SUBTASK_LOG_NOT_FOUND", http.StatusNotFound, "Запись журнала не найдена")
ErrTaskEquipmentNotFound  = errorspkg.New("TASK_EQUIPMENT_NOT_FOUND", http.StatusNotFound, "Оборудование не найдено в заказе")
ErrTaskEquipmentInUse     = errorspkg.New("TASK_EQUIPMENT_IN_USE", http.StatusConflict, "Оборудование выбрано в услугах заказа — сначала смените его в услугах")
ErrSubtaskFinalColumn     = errorspkg.New("SUBTASK_FINAL_COLUMN", http.StatusConflict, "Заказ в финальной колонке: нельзя добавить услугу или вернуть её из «Выполнена»")
ErrSubtaskLogForbidden    = errorspkg.New("SUBTASK_LOG_FORBIDDEN", http.StatusForbidden, "Можно менять только свои записи")

// внутренняя, наружу не течёт (ADR-100): колонка заказа сменилась между чтением column_id
// и LockTask → withTx повторяет попытку как 40P01/40001.
errTaskMovedRetry = errors.New("board: task moved during lock, retry")
```

## Error Mapping
| Источник | Условие | Результат |
|---|---|---|
| path `:id` задачи | не int64 > 0 / нет строки | 404 `TASK_NOT_FOUND` (до SQL — as-is `parseTaskID`) |
| path `/subtasks/:id` | не uuid (`uuid.Validate`) / нет строки / удалена параллельно (Lock → 0 строк) | 404 `SUBTASK_NOT_FOUND` |
| path `/subtask-products/:id` | не uuid / нет / удалена (в т.ч. каскадом с услугой) | 404 `SUBTASK_PRODUCT_NOT_FOUND` |
| path `/subtask-logs/:id` | не uuid / нет / удалена | 404 `SUBTASK_LOG_NOT_FOUND` |
| path `/tasks/:id/equipment/:equipment_id` | не uuid / строки `task_equipment` нет | 404 `TASK_EQUIPMENT_NOT_FOUND` |
| `DELETE /tasks/:id/equipment/:equipment_id` | единица в `subtasks.equipment_id` этого заказа | 409 `TASK_EQUIPMENT_IN_USE` + `details.subtasks[]` |
| `PUT /tasks/:id` смена `client_id` | оборудование прежнего клиента выбрано в услугах | 409 `TASK_EQUIPMENT_IN_USE` + `details.subtasks[]`; иначе чужие `task_equipment` удаляются в той же tx |
| `POST /tasks/:id/subtasks` | колонка заказа `is_final` | 409 `SUBTASK_FINAL_COLUMN` |
| `PUT /subtasks/:id` | колонка `is_final` И текущий status=`done` И новый ≠ `done` | 409 `SUBTASK_FINAL_COLUMN` |
| `PUT/DELETE /subtask-logs/:id` | `user_id` ≠ актор И актор не admin (проверка ПОСЛЕ 404) | 403 `SUBTASK_LOG_FORBIDDEN` |
| INSERT log / product | 23503 на `subtask_id` (услуга удалена между запросами) | 404 `SUBTASK_NOT_FOUND` |
| `errTaskMovedRetry`, 40P01, 40001 | `withTx` ретрай ×3 | далее 500 `INTERNAL` (as-is ADR-104 фазы 5) |
| 22003 в Reconcile | `errStockOverflow` | 500 `INTERNAL` (as-is) |

Порядок проверок в мутации строки: parse path → normalize/validate тела (400) → lock → 404 родителя/строки → 403 (журнал) → 409 бизнес-правила → 400 ссылочные поля (`service_id`, `product_id`, `equipment_id`, `assignee_id`).

## Validation Fields
Поля — ключи `details.fields[].name`. Сообщения — константы пакета (`msgRequired`/`msgNotFound` as-is + новые ниже).

| Эндпоинт | Поле | Правило | message |
|---|---|---|---|
| `POST/PUT /tasks` | `description` | NULL (PUT: не менять) или ≤10000 символов | «не длиннее 10000» |
| `POST /tasks/:id/subtasks` | `service_id` | uuid, услуга существует И `is_active` | «обязательно» / «не найдена» / «в архиве» |
| `PUT /subtasks/:id` | `status` | ∈ `new, in_progress, done` | «обязательно» / «недопустимое значение» |
| | `assignee_id` | NULL или активный пользователь (проверка только при смене значения, SD-007) | «не найден» |
| | `due_date` | NULL или `YYYY-MM-DD` | «неверная дата» |
| | `price` | NULL или ≥0, `numeric(12,2)` (Scale 10/2) | pkg/numeric `Msg*` |
| | `equipment_id` | NULL или единица из `task_equipment` этого заказа | «нет в оборудовании заказа» |
| `POST /subtasks/:id/products` | `product_id` | uuid, товар существует И `is_active` | «обязательно» / «не найден» / «в архиве» |
| | `qty` | обязательно, >0, `numeric(12,3)` (Scale 9/3) | «обязательно» / «должно быть больше 0» / pkg/numeric `Msg*` |
| `PUT /subtask-products/:id` | `qty` | как выше | как выше |
| | `price` | NULL или ≥0, Scale 10/2 | pkg/numeric `Msg*` |
| `POST /tasks/:id/equipment` | `equipment_id` | uuid, существует, `equipment.client_id = tasks.client_id` | «обязательно» / «не найдено» / «оборудование другого клиента» |
| `POST/PUT .../equipment` | `completeness`, `condition` | ≤1000 символов | «не длиннее 1000» |
| `POST /subtasks/:id/logs`, `PUT /subtask-logs/:id` | `text` | trim, 1..4000 | «обязательно» / «не длиннее 4000» |

- Ссылки из ТЕЛА → 400 `VALIDATION` (as-is правило фазы 5); 404 — только id в пути.
- Повторный `POST /tasks/:id/equipment` той же единицы — 201 без изменений (идемпотентно, `ON CONFLICT DO NOTHING`), не ошибка.

## Слайс: 3-design/design-go-layers.md [full]
# Design Go Layers: phase-6-order-card

> Owner: DTO, enum, сервис/репозиторий, хендлеры, роуты, SSR печати, frontend-модули. Всё — в пакете `internal/features/board` (файлы-по-сущности, D43, код вручную по эталону фазы 5). Коды ошибок — только `design-errors.md`. JSON — snake_case (§API-1). NUMERIC — `pkg/numeric.Amount`; nullable-деньги — `*numeric.Amount` (nil → `null`; `Amount("")` сериализуется в `0` — для NULL-цены недопустимо).

## Package Layout
| Файл (новый / изменён) | Содержимое |
|---|---|
| `card_dto.go` (new) | §DTO OrderCard, §DTO Row Requests |
| `card_repo.go` (new) | `CardRepo` — Card Queries, Print Header Query |
| `card_service.go` (new) | `CardService` (struct), `Card`, `lockRowsScope`, `cardMutation` |
| `subtasks_{repo,service,validate}.go`, `subtask_products_{repo,service,validate}.go`, `subtask_logs_{repo,service,validate}.go`, `task_equipment_{repo,service,validate}.go` (new) | методы `CardService`/репозитории по сущностям |
| `card_handler.go` (new) | `CardHandler` — все row-эндпоинты + `Card` |
| `print_ssr.go` (new) | печать актов (§Handler Print) |
| `description.go` (new) | `descriptionToHTML(text) string` (ADR-006) |
| `tasks_dto.go`, `tasks_repo.go`, `tasks_service.go`, `tasks_validate.go` (mod) | `description`, `total_amount`, `last_log`, cleanup оборудования при смене клиента |
| `tx.go` (mod) | `withTx` ретраит также `errTaskMovedRetry` (ADR-100) |
| `errors.go` (mod) | Codes 16-22 (`design-errors.md`) |
| `routes.go`, `ssr.go` (mod) | §Routes; `NewSSR(r, shell, status *ssr.StatusPages, firm config.FirmInfo, card *CardService)` |
| `internal/config/config.go` (mod) | `Firm FirmInfo` из `FIRM_NAME`, `FIRM_ADDRESS`, `FIRM_PHONE` (пустые допустимы) |
| `migrations/0006_subtask_logs.{up,down}.sql` (new) | `design-sql.md §Subtask Logs Migration` |

## Enum SubtaskStatus
As-is `enums.go`: `SubtaskStatusNew="new"`, `SubtaskStatusInProgress="in_progress"`, `SubtaskStatusDone="done"`, `SubtaskStatuses`. Sync-тест — as-is `enums_test.go` (против CHECK `0005_board.up.sql`).

## Enum PrintKind
Новый, `enums.go`: `PrintKindIntake="intake"`, `PrintKindHandover="handover"`, `PrintKinds`. Источник — path `/tasks/:id/print/:kind`; иное → 404. Тест — `enums_test.go` (+кейс).

## DTO Task Changes
```go
// TaskCard (элемент GET /tasks, встроен в TaskResponse) — добавить:
TotalAmount numeric.Amount `json:"total_amount"` // design-sql.md §Totals Expression, AC-013
LastLog     *TaskLastLog   `json:"last_log"`     // null — записей нет, AC-012

type TaskLastLog struct {
    Text       string    `json:"text"`        // ≤120 символов + «…» при обрезке
    AuthorName string    `json:"author_name"`
    CreatedAt  time.Time `json:"created_at"`
}

// TaskResponse — добавить:
DescriptionHTML string `json:"description_html"` // хранимый HTML (ADR-006), клиент берёт текст через DOMParser

// TaskCreateRequest (и TaskUpdateRequest через embed) — добавить:
Description *string `json:"description"` // plain text ≤10000; POST nil→""; PUT nil→не менять
```
`TaskMutationResponse` / `TaskDeleteResponse` — as-is.

## DTO OrderCard
```go
// GET /api/v1/tasks/:id/card и card в ответах row-мутаций (ADR-001).
type OrderCardResponse struct {
    Task      TaskResponse    `json:"task"`
    Column    CardColumn      `json:"column"`
    Equipment []CardEquipment `json:"equipment"` // всегда массив
    Subtasks  []CardSubtask   `json:"subtasks"`
    Stock     []CardStock     `json:"stock"`     // по каждому товару карточки, для подсказки AC-009
}
// Итог заказа — только task.total_amount (одно имя в GET /tasks и в карточке; отдельного `total` нет).
type CardColumn struct {
    ID            string `json:"id"`
    Name          string `json:"name"`
    IsFinal       bool   `json:"is_final"`
    ConsumesStock bool   `json:"consumes_stock"`
}
type CardEquipment struct {
    EquipmentID  string             `json:"equipment_id"`
    TypeName     string             `json:"type_name"`
    Brand        string             `json:"brand"`
    Model        string             `json:"model"`
    Serial       string             `json:"serial"`
    Completeness string             `json:"completeness"`
    Condition    string             `json:"condition"`
    History      []EquipmentHistory `json:"history"` // другие заказы с единицей, AC-006
}
type EquipmentHistory struct {
    TaskID    int64     `json:"task_id"`
    Title     string    `json:"title"`
    CreatedAt time.Time `json:"created_at"` // дата заказа для «Уже был в ремонте» (AC-006)
}
type CardSubtask struct {
    ID               string               `json:"id"`
    ServiceID        string               `json:"service_id"`
    ServiceName      string               `json:"service_name"`
    Status           string               `json:"status"`             // Enum SubtaskStatus
    AssigneeID       *string              `json:"assignee_id"`
    AssigneeName     *string              `json:"assignee_name"`
    AssigneeIsActive *bool                `json:"assignee_is_active"` // null без исполнителя; false → «неактивен» (SD-007)
    DueDate          *string              `json:"due_date"`           // YYYY-MM-DD
    Price            *numeric.Amount      `json:"price"`              // null = «по факту»
    EquipmentID      *string              `json:"equipment_id"`
    Position         int                  `json:"position"`
    Total            numeric.Amount       `json:"total"`              // subtask_total
    Products         []CardSubtaskProduct `json:"products"`
    Logs             []CardSubtaskLog     `json:"logs"`
}
type CardSubtaskProduct struct {
    ID        string          `json:"id"`
    ProductID string          `json:"product_id"`
    Name      string          `json:"name"`
    Unit      string          `json:"unit"`
    Position  int             `json:"position"` // ADR-103; массив уже отсортирован
    Qty       numeric.Amount  `json:"qty"`
    Price     *numeric.Amount `json:"price"`
    LineTotal numeric.Amount  `json:"line_total"`
}
type CardSubtaskLog struct {
    ID         string    `json:"id"`
    Text       string    `json:"text"`
    UserID     string    `json:"user_id"`
    AuthorName string    `json:"author_name"`
    CreatedAt  time.Time `json:"created_at"`
    UpdatedAt  time.Time `json:"updated_at"`
    CanEdit    bool      `json:"can_edit"` // автор == актор || актор admin (D27)
}
type CardStock struct {
    ProductID    string         `json:"product_id"`
    Stock        numeric.Amount `json:"stock"`
    MinStock     numeric.Amount `json:"min_stock"`
    TaskReserved numeric.Amount `json:"task_reserved"` // task_stock.qty этого заказа, 0 если нет
}
// Ответ всех row-мутаций (POST 201 / PUT 200 / DELETE 200).
type OrderCardMutationResponse struct {
    Card     OrderCardResponse `json:"card"`
    Warnings []StockWarning    `json:"warnings"` // as-is формат ADR-006 фазы 5; [] без Reconcile
}
```

## DTO Row Requests
Все — strictJSON (лишнее поле → 400 `BAD_REQUEST`), `normalize()/validate() []errorspkg.Field` на типе запроса (эталон `tasks_validate.go`). Правила — `design-errors.md §Validation Fields`.
```go
type SubtaskCreateRequest struct { ServiceID string `json:"service_id"` }
// PUT /subtasks/:id — полная замена изменяемых полей строки (LWW); клиент шлёт всю строку.
type SubtaskUpdateRequest struct {
    Status      string          `json:"status"`
    AssigneeID  *string         `json:"assignee_id"`
    DueDate     *string         `json:"due_date"`
    Price       *numeric.Amount `json:"price"`
    EquipmentID *string         `json:"equipment_id"`
}
type SubtaskProductCreateRequest struct {
    ProductID string          `json:"product_id"`
    Qty       *numeric.Amount `json:"qty"` // цена копируется из products.price (AC-008)
}
type SubtaskProductUpdateRequest struct {
    Qty   *numeric.Amount `json:"qty"`
    Price *numeric.Amount `json:"price"` // null = без цены (в итог 0)
}
type TaskEquipmentCreateRequest struct {
    EquipmentID  string `json:"equipment_id"`
    Completeness string `json:"completeness"`
    Condition    string `json:"condition"`
}
type TaskEquipmentUpdateRequest struct {
    Completeness string `json:"completeness"`
    Condition    string `json:"condition"`
}
type SubtaskLogRequest struct { Text string `json:"text"` }
```

## Service Interface
```go
type CardService struct { pool *pgxpool.Pool; tasks *TaskRepo; card *CardRepo; subtasks *SubtaskRepo;
    products *SubtaskProductRepo; logs *SubtaskLogRepo; equipment *TaskEquipmentRepo; engine *StockEngine }

func (s *CardService) Card(ctx context.Context, a Actor, taskID string) (*OrderCardResponse, error)
func (s *CardService) CreateSubtask(ctx context.Context, a Actor, taskID string, in SubtaskCreateRequest) (*OrderCardMutationResponse, error)
func (s *CardService) UpdateSubtask(ctx context.Context, a Actor, id string, in SubtaskUpdateRequest) (*OrderCardMutationResponse, error)
func (s *CardService) DeleteSubtask(ctx context.Context, a Actor, id string) (*OrderCardMutationResponse, error)
func (s *CardService) CreateSubtaskProduct(ctx context.Context, a Actor, subtaskID string, in SubtaskProductCreateRequest) (*OrderCardMutationResponse, error)
func (s *CardService) UpdateSubtaskProduct(ctx context.Context, a Actor, id string, in SubtaskProductUpdateRequest) (*OrderCardMutationResponse, error)
func (s *CardService) DeleteSubtaskProduct(ctx context.Context, a Actor, id string) (*OrderCardMutationResponse, error)
func (s *CardService) AddEquipment(ctx context.Context, a Actor, taskID string, in TaskEquipmentCreateRequest) (*OrderCardMutationResponse, error)
func (s *CardService) UpdateEquipment(ctx context.Context, a Actor, taskID, equipmentID string, in TaskEquipmentUpdateRequest) (*OrderCardMutationResponse, error)
func (s *CardService) RemoveEquipment(ctx context.Context, a Actor, taskID, equipmentID string) (*OrderCardMutationResponse, error)
func (s *CardService) CreateLog(ctx context.Context, a Actor, subtaskID string, in SubtaskLogRequest) (*OrderCardMutationResponse, error)
func (s *CardService) UpdateLog(ctx context.Context, a Actor, id string, in SubtaskLogRequest) (*OrderCardMutationResponse, error)
func (s *CardService) DeleteLog(ctx context.Context, a Actor, id string) (*OrderCardMutationResponse, error)
func (s *CardService) PrintData(ctx context.Context, a Actor, taskID string) (*PrintVM, error)

// lockRowsScope — Lock Order (design-sql.md §Lock Order): TaskColumnID → LockTargetColumn (FOR SHARE)
// → LockTask (FOR UPDATE) → task.ColumnID != col.ID ? errTaskMovedRetry.
func (s *CardService) lockRowsScope(ctx context.Context, tx pgx.Tx, taskID int64) (*Task, *columnFlags, error)
// cardMutation — Re-fetch-after-write: Card() ПОСЛЕ commit + warnings Reconcile.
func (s *CardService) cardMutation(ctx context.Context, a Actor, taskID int64, r ReconcileResult) (*OrderCardMutationResponse, error)
```
Поток мутации товара/услуги (одна `withTx`): resolve parent (Subtask Parent Query) → `lockRowsScope` → Lock Subtask(/Product) (`task_id` сверяется в WHERE) → бизнес-правила (`SUBTASK_FINAL_COLUMN`) → ссылочные поля → запись → `engine.Reconcile(ctx, tx, taskID, col.ConsumesStock, a.ID)` для операций, меняющих товары (create/delete услуги, create/update/delete товара); `UpdateSubtask` — без Reconcile, но под теми же блокировками. `tasks.version`/`tasks.updated_at` не трогаются (ADR-002). Span: `otel.Tracer("board")` — `subtasks.mutate`, `subtask_products.mutate`, `task_equipment.mutate`, `subtask_logs.mutate`, `tasks.card`, `tasks.print`; атрибуты `task_id`, `subtask_id`/`product_id`, `op`.

`TaskService.Update` (mod): после paid-гарда, если `client_id` сменился — Foreign Equipment Query (409 или DELETE чужих `task_equipment`) до `repo.Update`; `description`: nil → текущее значение (`LockTask` дополнительно читает `description_html`), иначе `descriptionToHTML`.

## Repository Interface
```go
func (r *TaskRepo) TaskColumnID(ctx context.Context, tx pgx.Tx, id int64) (string, error) // errNotFound
// LockTargetColumn, LockTask — as-is (LockTask + description_html)
type CardRepo struct{ pool *pgxpool.Pool }
func (r *CardRepo) Column(ctx context.Context, id string) (*CardColumn, error)
func (r *CardRepo) Equipment(ctx context.Context, taskID int64) ([]CardEquipment, error)
func (r *CardRepo) Subtasks(ctx context.Context, taskID int64) ([]CardSubtask, error)
func (r *CardRepo) Products(ctx context.Context, taskID int64) (map[string][]CardSubtaskProduct, error) // по subtask_id
func (r *CardRepo) Logs(ctx context.Context, taskID int64) (map[string][]subtaskLogRow, error)
func (r *CardRepo) Stock(ctx context.Context, taskID int64) ([]CardStock, error)
func (r *CardRepo) PrintHeader(ctx context.Context, taskID int64) (*printHeader, error)
type SubtaskRepo struct{}
func (r *SubtaskRepo) TaskIDOf(ctx context.Context, tx pgx.Tx, id string) (int64, error)
func (r *SubtaskRepo) Lock(ctx context.Context, tx pgx.Tx, id string, taskID int64) (*subtaskRow, error)
func (r *SubtaskRepo) Insert(ctx context.Context, tx pgx.Tx, taskID int64, serviceID string) (string, error) // + копия комплекта
func (r *SubtaskRepo) ServiceState(ctx context.Context, tx pgx.Tx, serviceID string) (exists, active bool, err error)
func (r *SubtaskRepo) Update(ctx context.Context, tx pgx.Tx, id string, in subtaskWrite) error
func (r *SubtaskRepo) Delete(ctx context.Context, tx pgx.Tx, id string) error
type SubtaskProductRepo struct{}
func (r *SubtaskProductRepo) ParentOf(ctx context.Context, tx pgx.Tx, id string) (taskID int64, subtaskID string, err error)
func (r *SubtaskProductRepo) Lock(ctx context.Context, tx pgx.Tx, id, subtaskID string) error
func (r *SubtaskProductRepo) ProductState(ctx context.Context, tx pgx.Tx, productID string) (exists, active bool, err error)
func (r *SubtaskProductRepo) Insert(ctx context.Context, tx pgx.Tx, subtaskID, productID string, qty numeric.Amount) (string, error)
func (r *SubtaskProductRepo) Update(ctx context.Context, tx pgx.Tx, id string, qty numeric.Amount, price *numeric.Amount) error
func (r *SubtaskProductRepo) Delete(ctx context.Context, tx pgx.Tx, id string) error
type SubtaskLogRepo struct{ pool *pgxpool.Pool }
func (r *SubtaskLogRepo) Insert(ctx context.Context, subtaskID, userID, text string) (string, error) // 23503 → errNotFound
func (r *SubtaskLogRepo) Owner(ctx context.Context, id string) (userID string, taskID int64, err error)
func (r *SubtaskLogRepo) Update(ctx context.Context, id, text string) error
func (r *SubtaskLogRepo) Delete(ctx context.Context, id string) error
type TaskEquipmentRepo struct{}
func (r *TaskEquipmentRepo) EquipmentClient(ctx context.Context, tx pgx.Tx, equipmentID string) (string, error)
func (r *TaskEquipmentRepo) Insert(ctx context.Context, tx pgx.Tx, taskID int64, in TaskEquipmentCreateRequest) error
func (r *TaskEquipmentRepo) Update(ctx context.Context, tx pgx.Tx, taskID int64, equipmentID string, in TaskEquipmentUpdateRequest) error
func (r *TaskEquipmentRepo) UsedBy(ctx context.Context, tx pgx.Tx, taskID int64, equipmentID string) ([]OpenSubtask, error)
func (r *TaskEquipmentRepo) Delete(ctx context.Context, tx pgx.Tx, taskID int64, equipmentID string) error
func (r *TaskEquipmentRepo) Attached(ctx context.Context, tx pgx.Tx, taskID int64, equipmentID string) (bool, error)
func (r *TaskEquipmentRepo) ForeignInUse(ctx context.Context, tx pgx.Tx, taskID int64, clientID string) ([]OpenSubtask, error)
func (r *TaskEquipmentRepo) DeleteForeign(ctx context.Context, tx pgx.Tx, taskID int64, clientID string) error
```
SQL — `design-sql.md` (одноимённые секции).

## Handler Card
`CardHandler{svc *CardService}` — один метод на эндпоинт (`card_handler.go`), `Actor` из `middleware.CurrentPrincipal` (as-is `TasksHandler`). Ответы: GET 200 `OrderCardResponse`; POST 201 / PUT 200 / DELETE 200 `OrderCardMutationResponse`. Ошибки — возврат `*errorspkg.AppError` в `FiberErrorHandler` (as-is).

## Handler Print
`print_ssr.go`, метод `(*SSR).print(c fiber.Ctx) error`: `kind` ∉ `PrintKinds` или TASK_NOT_FOUND → `StatusPages.NotFound`; иначе `r.Render(c.Response().BodyWriter(), "print-"+kind, "print", PrintVM)` + `Content-Type: text/html; charset=utf-8`. `no-store` — as-is `RequireSSR`.
```go
type PrintVM struct {
    Firm        config.FirmInfo // пустые поля → блок реквизитов опускается ({{ if }} по каждому)
    Kind        string          // intake | handover
    Card        OrderCardResponse
    ClientName  string; ContactName string; Phones string
    Description template.HTML   // task.description_html (ADR-006: хранится только серверно-экранированный HTML)
    PrintedAt   time.Time
}
```
Шаблоны: `web/ssr/board/templates/layouts/print.html` (`{{ define "print" }}`, без сайдбара/theme.js, `data-bs-theme="light"`, `/assets/ssr-shared/print.css` с `@media print`), `pages/print-intake.html`, `pages/print-handover.html`. Handover: все услуги со статусом, цена работы («по факту» — прочерк), товары qty×цена, итог `task.total_amount`, оплата (`is_paid`/`paid_at`).

## Routes
```go
// routes.go — добавить (все RequireAPI, CSRF на мутациях — каркас; admin-гардов нет, D19/AC-017)
t.Get("/:id/card", h.Card.Card)
t.Post("/:id/subtasks", h.Card.CreateSubtask)
t.Post("/:id/equipment", h.Card.AddEquipment)
t.Put("/:id/equipment/:equipment_id", h.Card.UpdateEquipment)
t.Delete("/:id/equipment/:equipment_id", h.Card.RemoveEquipment)
sub := api.Group("/subtasks", a.RequireAPI())
sub.Put("/:id", h.Card.UpdateSubtask); sub.Delete("/:id", h.Card.DeleteSubtask)
sub.Post("/:id/products", h.Card.CreateSubtaskProduct); sub.Post("/:id/logs", h.Card.CreateLog)
sp := api.Group("/subtask-products", a.RequireAPI())
sp.Put("/:id", h.Card.UpdateSubtaskProduct); sp.Delete("/:id", h.Card.DeleteSubtaskProduct)
sl := api.Group("/subtask-logs", a.RequireAPI())
sl.Put("/:id", h.Card.UpdateLog); sl.Delete("/:id", h.Card.DeleteLog)
// SSR (s != nil)
app.Get("/tasks/:id/print/:kind", a.RequireSSR(), s.print)
```
Новое оборудование из карточки — as-is `POST /api/v1/clients/:id/equipment` (без admin, D19) + `POST /api/v1/tasks/:id/equipment` (ADR-004). Справочники — as-is `GET /api/v1/services?q=`, `/products?q=`, `/equipment-types`, `/assignees`.

## Frontend Modules
SSR + vanilla JS + Bootstrap 5.3 (D30/D38). Element Plus/Vue — неприменимо.
- `web/ssr/_shared/js/task-card.js` (new, `KT.taskCard`): `open(taskId|null, defaultColumnId)`, `close()`; рендер разделов, row-сохранение, подсказка остатка, итоги. `board.js`: `openTaskForm` → `KT.taskCard.open`; `onChange` колбэк → `reloadAndRender()`; deep-link (ADR-104).
- `web/ssr/board/templates/pages/board.html` (mod): `#task-modal` → `modal-xl modal-fullscreen-lg-down modal-dialog-scrollable`; шапка — существующие поля + `#task-description` (textarea); контейнеры `#card-equipment`, `#card-subtasks`, `#card-total`; кнопки актов; `<script src="/assets/ssr-shared/js/task-card.js">` до `board.js`.
- Карточка доски (`cardHTML`): `total_amount` (если >0) + `last_log` (текст, автор); заголовок колонки — сумма `total_amount` видимых карточек в копейках (`Math.round(x*100)`), вывод `toFixed(2)` (ADR-005).
- Row-очередь (ADR-105): `rowQueue` по ключу строки (`subtask:<id>`, `product:<id>`, `equipment:<equipment_id>`, `log:<id>`); не более одного запроса строки в полёте; правки во время полёта копятся в `pending` строки и уходят ОДНИМ следующим запросом; тело PUT собирается в момент отправки = поля строки из последнего применённого `card` ⊕ `pending` (не из DOM и не из момента `change`). Ошибка → `pending` строки сбрасывается, значения — из последнего `card`.
- Row-сохранение (SD-008): `change` → `rowQueue.enqueue` → строка `aria-busy` → запрос → `applyCard(resp.card)` (patch: элементы, содержащие `document.activeElement`, не перезаписываются) → `showWarnings(resp.warnings)` (as-is формат фазы 5). Ошибка: 404 → `KT.uiAlert` «Строка уже удалена» + перечитать карточку; 400 → `is-invalid` + текст поля, откат значения; 409/403 → `KT.uiAlert(err.message)` + откат.
- Подсказка (AC-009): `avail = stock + task_reserved`; `need = Σ qty товара по всем услугам карточки`; `rest = avail − need`: `rest < 0` → «в наличии только {avail}»; `rest < min_stock` → «останется {rest} — минимум {min_stock}»; иначе «в наличии {avail}». Для товара в пикере до добавления — `stock/min_stock` из `GET /api/v1/products` (as-is).
- Новый заказ (SD-006): разделы оборудования/услуг `disabled` + подсказка «Сохраните заказ»; после `POST /tasks` — `open(id)` без закрытия модалки.
- Шапка: сохранение кнопкой (as-is PUT с `version`, `is_paid` отправляется всегда — board.js:590-593); смена клиента при наличии оборудования → `KT.uiConfirm` «Оборудование другого клиента будет убрано из заказа»; после 200 — `GET /card`.
- Удаление услуги — `KT.uiConfirm` «Услуга удалится вместе с товарами и журналом».
- Testid (контракт POM `e2e/pages/OrderCardPage.ts`): `card-description`, `card-equipment-add`, `card-equipment-new`, `card-equipment-row`, `card-equipment-completeness`, `card-equipment-condition`, `card-equipment-remove`, `card-equipment-history`, `card-subtask-add`, `card-subtask-row`, `card-subtask-status`, `card-subtask-assignee`, `card-subtask-due`, `card-subtask-price`, `card-subtask-equipment`, `card-subtask-delete`, `card-subtask-total`, `card-product-add`, `card-product-row`, `card-product-qty`, `card-product-price`, `card-product-hint`, `card-product-delete`, `card-log-text`, `card-log-add`, `card-log-row`, `card-log-edit`, `card-log-delete`, `card-total`, `card-print-intake`, `card-print-handover`, `card-warning`; на доске `task-card-total`, `task-card-last-log`, `column-total`. Admin-only (`task-delete`, `task-paid` enabled) — снятие testid атрибутом (as-is board.js:576-579); `card-log-edit/-delete` рендерятся только при `can_edit`.
- `web/ssr/_shared/print.css` (new): A4, `@media print` скрывает `.no-print` (кнопка «Печать»).

## Слайс: 3-design/design-tests.md [sections]
## Unit
| ID | Сценарий | AC |
|---|---|---|
| UT-001 | `descriptionToHTML`: экранирование `<>&"'`, пустые строки → абзацы, `\n` → `<br>`, `""` → `""` | AC-002 |
| UT-002 | validate row-запросов: границы qty/price (Scale), text trim/4000, status enum, nil vs value | AC-008, AC-012 |
| UT-003 | `PrintKinds` = `{intake, handover}` | AC-015 |

## Слайс: 1-research/output.md [sections]
## Паттерны в коде
- Многосущностный feature-пакет: файлы-по-сущности (dto/handler/repo/service/validate) в ОДНОМ пакете `board` — эталон `internal/features/board/errors.go:1-4`; фаза 6 добавляет `subtasks`-файлы в ТОТ ЖЕ пакет, не отдельную фичу.
- Lock Order (ADR-104): `columns → tasks → products`, FOR UPDATE/FOR SHARE в этом порядке внутри `withTx`; новые мутации `subtasks`/`subtask_products` ДОЛЖНЫ брать `LockTask` ПЕРЕД правкой — см. LOW code review ниже (Граничные случаи).
- Re-fetch-after-write: ответ mutation-хендлера из `repo.Get()` после commit, не RETURNING (`internal/features/board/tasks_service.go:330-340`).
- Sentinel-ошибки — общие `var Err... = errorspkg.New(...)` на пакете (`internal/features/board/errors.go:15-22`), `.WithDetail` — КОПИЯ (`pkg/errorspkg/errors.go:36-44`), sentinel не мутируется.
- Optimistic locking (`version`) — только `clients`/`tasks` (D28, `docs/plan.md:152`); `subtasks`/`subtask_products` — last-write-wins без version (паттерн уже проверен на `tasks.Move`).
- DTO-валидация в методах запроса `(in Request) normalize()/validate()` (`internal/features/board/tasks_validate.go:30-100`), не в handler/service.
- Денежные/количественные поля — `pkg/numeric.Amount` (строка в Go, число на wire) — обязателен для `subtask_products.qty/price`, `subtasks.price`, итогов заказа.
- `withTx` с ретраем 40P01/40001×3 (`internal/features/board/tx.go:16-29`) — обязателен для мутаций склада/нескольких таблиц.
- Testid снимается атрибутом (не только `d-none`) для элементов, видимых только одной роли (`web/ssr/_shared/js/board.js:576-579`).

## Слайс: AC-строки
#### AC-001: Полная карточка вместо простой формы
#### AC-002: Шапка заказа сохраняется кнопкой, конфликт не затирает
#### AC-003: Оплата — только администратор
#### AC-004: Принятое оборудование клиента в заказе
#### AC-005: Новое оборудование прямо из карточки
#### AC-006: История ремонтов единицы
#### AC-007: Услуги заказа из каталога
#### AC-008: Типовой комплект товаров услуги
#### AC-009: Подсказка наличия товара
#### AC-010: Списание при правке товаров в работе
#### AC-011: Статус услуги и финальная колонка
#### AC-012: Журнал работ услуги
#### AC-013: Итог заказа
#### AC-014: Одновременная правка услуг и товаров
#### AC-015: Акт приёма
#### AC-016: Акт выдачи
#### AC-017: Права исполнителя в карточке

## Паспорт (выжимка)
# Passport: phase-6-order-card (generated — руками не править; stage_bundle.py)
**Tier:** M | **Совместимость:** ? (pipeline-doctrine.md §CM-1; ? → уточни у оркестратора)
**Трек:** обычный (prefs нет — дефолт) — режимы агентов (`spec-track delta/micro/mini/merged`) включает ТОЛЬКО строка `Режим:` в prompt оркестратора; DS/TS-индексы ниже присутствуют на ЛЮБОМ треке при наличии draft-spec.
**Layout:** автодетект (tools/layout_resolve.py)
**UI-scope (pipeline-doctrine.md §US-1, 0-setup/ui-scope-prefs.md):** swagger:no | adaptive:min | themes:both

**Индексы:** AC 17 / DS 0 / TS 0 — полные списки: `0-setup/passport.md` (разрешён контрактом); конкретное решение — точечный grep по `DS-NNN`/`TS-NNN` в draft-spec §9/§11 (разрешён).
