# Phase Bundle: phase-5-board / Phase 03 (backend) — generated 2026-09-26 23:58

> Твой вход — ЭТОТ файл. Upstream-доки (research/prd/design-*) напрямую НЕ читать —
> нужные слайсы дизайна ниже. Код фичи и её тесты — читать/писать можно (твоя фаза).

## Фаза плана (6-plan/code-plan-phase-03-board-tasks-move.md)
# Phase 03: Board — заказы, move, комбинированная live-сьюта

**Layer:** backend
**Commit**: `feat(phase-5): board — заказы, перенос между колонками, live-сьюта`

## Files to CREATE
- `internal/features/board/tasks_dto.go` — `TaskCard`, `TaskListResponse`, `TaskResponse`, `TaskCreateRequest`, `TaskUpdateRequest`, `TaskMoveRequest`, `TaskMutationResponse`, `TaskDeleteResponse` (design-go-layers.md §DTO Task)
- `internal/features/board/tasks_validate.go` — `TaskCreateRequest.normalize()`/`validate()`, `TaskUpdateRequest` (+`version` обязателен) — правила design-errors.md §Validation Fields (`title`, `client_id`, `column_id`, `assignee_id`, `due_date`, `version`)
- `internal/features/board/tasks_filter.go` — `TaskFilter` + `ParseTaskFilter(c fiber.Ctx, a Actor) (TaskFilter, error)` (design-go-layers.md §DTO TaskFilter; `q`→`#N`/`N` regex `^#?\d{1,18}$`; `due`/`paid`/`assignee` enum парсинг)
- `internal/features/board/tasks_repo.go` — `TaskRepo`: `LockTargetColumn`, `LockTask`, `OpenSubtasks` (design-sql.md §Open Subtasks Query), `NextPosition` (делегирует `position.go`), `Move` (design-sql.md §Move Task Query), `Update` (§Update Task Query), `Insert` (§Insert Task Query), `Delete` (§Delete Task Query), `List` (§Board List Query, `q`-экранирование через `pkg/sqlutil`), `Get`
- `internal/features/board/tasks_service.go` — `TaskService`: `List`, `Get`, `Create`, `Update`, `Move`, `Delete` — алгоритм Move (design-go-layers.md §Service Interface, шаги 1–7); `ErrSubtasksNotDone.WithDetail("subtasks", list)` (ADR-102 — использовать `pkg/errorspkg.WithDetail`, НЕ отдельный тип ответа); span'ы `tasks.move`/`columns.recalc` (Monitoring Spec)
- `internal/features/board/tasks_handler.go` — `List` (`ParseTaskFilter` с `Actor` из `middleware.CurrentPrincipal(c)`), `Get`, `Create`(201), `Update`, `Move`, `Delete`(200 + `TaskDeleteResponse{warnings}`)
- `internal/features/board/assignees_handler.go` — `GET /api/v1/assignees` (design-sql.md §Assignees Query; только `is_active`, `limit` default 200 max 1000)
- `internal/features/board/tasks_validate_test.go` — `TestTaskCreateRequestValidate_EmptyTitle`, `TestTaskCreateRequestValidate_TitleTooLong`, `TestTaskCreateRequestValidate_MissingClientID`, `TestTaskUpdateRequestValidate_MissingVersion` (UT-002 заказы)
- `internal/features/board/tasks_filter_test.go` — `TestParseTaskFilter_HashNumber`, `TestParseTaskFilter_PlainNumber`, `TestParseTaskFilter_InvalidNumber`, `TestParseTaskFilter_DefaultsByRole`, `TestParseTaskFilter_EnumValidation` (UT-001)
- `internal/features/board/board_http_live_test.go` (`BoardLiveSuite`, `testutil.LiveSuite`, реальный бинарь+TCP, реальный login §TD-8) — методы по ID design-tests.md: `TestBoardLiveSuite_Columns_SeedPresent` (LV-001), `TestBoardLiveSuite_CreateTask` (LV-002), `TestBoardLiveSuite_UpdateTask_VersionIncrements` (LV-003), `TestBoardLiveSuite_Move_BetweenColumns` (LV-004), `TestBoardLiveSuite_Move_WithinColumn_PrevNext` (LV-005), `TestBoardLiveSuite_Move_ConsumesStock` (LV-006), `TestBoardLiveSuite_Move_NegativeStockWarning` (LV-007), `TestBoardLiveSuite_ColumnToggleConsumes_RecalcAll` (LV-008), `TestBoardLiveSuite_ColumnsCRUD` (LV-009), `TestBoardLiveSuite_DeleteTask_ReturnsStock` (LV-010), `TestBoardLiveSuite_AdminTogglesPaid` (LV-011), `TestBoardLiveSuite_Filters_Combination` (LV-020), `TestBoardLiveSuite_Filters_AssigneeDefaultByRole` (LV-021), `TestBoardLiveSuite_Filters_LimitOffsetTotal` (LV-022), `TestBoardLiveSuite_Auth_NoSession401` (LV-030), `TestBoardLiveSuite_Auth_NoCSRF403` (LV-031), `TestBoardLiveSuite_GarbageBody400` (LV-032), `TestBoardLiveSuite_TaskValidation400` (LV-033), `TestBoardLiveSuite_ColumnValidation400` (LV-034), `TestBoardLiveSuite_NotFound404` (LV-035), `TestBoardLiveSuite_Move_ForeignNeighborIgnored` (LV-036), `TestBoardLiveSuite_QueryEnumValidation400` (LV-037), `TestBoardLiveSuite_Executor_ColumnsForbidden` (LV-040), `TestBoardLiveSuite_Executor_DeleteTaskForbidden` (LV-041), `TestBoardLiveSuite_Executor_PaidAdminOnly` (LV-042), `TestBoardLiveSuite_Executor_TaskWriteAllowed` (LV-044), `TestBoardLiveSuite_Move_ToFinal_SubtasksNotDone` (LV-050), `TestBoardLiveSuite_Update_ToFinal_SubtasksNotDone` (LV-051), `TestBoardLiveSuite_Move_ToFinal_AllDone` (LV-052), `TestBoardLiveSuite_DeleteColumn_NotEmpty` (LV-053), `TestBoardLiveSuite_DeleteColumn_LastNonFinal` (LV-054), `TestBoardLiveSuite_Update_StaleVersion` (LV-055), `TestBoardLiveSuite_Race_ConcurrentMoveDifferentColumns` (LV-060, `-race`), `TestBoardLiveSuite_Race_FormVsMove` (LV-061), `TestBoardLiveSuite_Race_ToggleVsMove` (LV-062), `TestBoardLiveSuite_Race_ConcurrentMoveSharedProduct` (LV-063), `TestBoardLiveSuite_Race_PositionRebalance50Inserts` (LV-064), `TestBoardLiveSuite_Race_DeleteClientVsCreateTask` (LV-065), `TestBoardLiveSuite_Enums_SubtaskStatusMatchesCheckConstraint` (live-сверка `pg_get_constraintdef`, дополняет `enums_test.go` фазы 02)
- `internal/features/board/board_fuzz_test.go` — MODIFY: добавить `FuzzParseTaskFilter`, `FuzzTaskInputValidate`, `FuzzMoveBody` (`app.Test` — допустимое исключение §9.1)

## Files to MODIFY
- `internal/features/board/routes.go` — расширить `Register`: группа `/tasks` (List/Create/Get/Update/Move/Delete), `GET /assignees`; `Handlers` — добавить поля `Tasks *TasksHandler`, `Assignees *AssigneesHandler`
- `internal/features/board/stock_engine_integration_test.go` — MODIFY: расширить `IT-010` таблицу шагов до полного набора (consume, return, повторный consume, toggle колонки, delete заказа, manual adjust — manual adjust появится в фазе 04, здесь — заготовка без него) с использованием реального `TaskRepo`/`TaskService` вместо прямых SQL-вставок фазы 02

## Run after
```bash
make build
make test
make test-integration-scoped
```

## Tests in this phase
- Unit: `TestTaskCreateRequestValidate_EmptyTitle`, `TestParseTaskFilter_HashNumber`, `TestParseTaskFilter_DefaultsByRole`
- Fuzz (TD-7): `FuzzParseTaskFilter`, `FuzzTaskInputValidate`, `FuzzMoveBody`
- Live (`BoardLiveSuite`, TD-8): полный список выше (LV-001…LV-065, кроме LV-012/013/014/043 — фаза 04; кроме LV-043 stock)

## Definition of Done
- [ ] `make build` — OK
- [ ] `make test` — unit OK
- [ ] `make test-integration-scoped` — OK (скоуп §EE-5; полный прогон — по prefs)
- [ ] `make lint` — 0 ошибок
- [ ] Тесты этой фазы зелёные до перехода (TD-1)
- [ ] `board_http_live_test.go` (`testutil.LiveSuite`, реальный бинарь+TCP, реальный login) — все перечисленные LV-* зелёные (TD-8)
- [ ] `go test -run=^$ -fuzz=FuzzParseTaskFilter -fuzztime=10s ./internal/features/board/` и аналогично для `FuzzTaskInputValidate`, `FuzzMoveBody` — зелёные (TD-7, класс 9.1); `FuzzMoveBody` использует `app.Test` — единственное разрешённое исключение (§9.1)
- [ ] `ErrSubtasksNotDone.WithDetail("subtasks", list)` — используется `pkg/errorspkg.WithDetail` (фаза 01); отдельный ad-hoc тип ответа для move НЕ создан (helper-binding)
- [ ] Инвариант склада (`assertStockInvariant`) вызван в конце LV-006…LV-010, LV-060…LV-063
- [ ] AC-002, AC-003, AC-004, AC-005, AC-007 (сквозной, LV-008), AC-009, AC-010, AC-011, AC-012, AC-013, AC-014 — покрыты backend-уровнем (live)
- [ ] `TaskCard.Position` — `string` (NUMERIC как есть, не Amount — только для отладки/сортировки клиента, design-go-layers.md §DTO Task)
- [ ] `tasks.id` в пути — `strconv.ParseInt`>0 до SQL; uuid-пути — `uuid.Validate` (design-errors.md)


## Слайс: 3-design/design-sql.md [full]
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

## Слайс: 3-design/design-errors.md [full]
# Design Errors: phase-5-board

> Owner: коды ошибок, sentinel'ы, маппинг. Проект использует строковые коды `errorspkg.AppError.Code` (не числовые supportMessage) — заголовок `Codes` ниже нумерует строки таблицы 01–15. Wire — `{code, message, details?}` (`pkg/errorspkg/errors.go`).

## errorspkg Extension
ADR-102. Нужен `details.subtasks[]` для `SUBTASKS_NOT_DONE` — текущий `errDetails` несёт только `fields`.
```go
// pkg/errorspkg/errors.go
type AppError struct {
    Code    string         `json:"code"`
    HTTP    int            `json:"-"`
    Message string         `json:"message"`
    Fields  []Field        `json:"-"`
    Extra   map[string]any `json:"-"` // доп. ключи details (snake_case); sentinel не мутируется
}

// WithDetail — КОПИЯ ошибки с доп. ключом details (sentinel'ы — общие var, мутировать нельзя).
func (e *AppError) WithDetail(key string, v any) *AppError

// FiberErrorHandler: details = {"fields": Fields (если есть)} ∪ Extra; details опускается, если оба пусты.
```
- Wire VALIDATION не меняется (`details.fields[]`). Существующие live-тесты фаз 1–4 — регресс-гейт.

## Codes 01-15
| # | Code | HTTP | Sentinel (пакет) | Сообщение | Driver |
|---|---|---|---|---|---|
| 01 | `TASK_NOT_FOUND` | 404 | `ErrTaskNotFound` (board) | «Заказ не найден» | AC-009 |
| 02 | `COLUMN_NOT_FOUND` | 404 | `ErrColumnNotFound` (board) | «Колонка не найдена» | AC-006 |
| 03 | `SUBTASKS_NOT_DONE` | 409 | `ErrSubtasksNotDone` (board) + `details.subtasks[]` | «Есть незавершённые услуги — в финальную колонку нельзя» | AC-003 |
| 04 | `COLUMN_NOT_EMPTY` | 409 | `ErrColumnNotEmpty` (board) | «В колонке есть заказы» | AC-006 |
| 05 | `LAST_NON_FINAL_COLUMN` | 409 | `ErrLastNonFinalColumn` (board) | «Нужна хотя бы одна нефинальная колонка» | AC-006 |
| 06 | `TASK_VERSION_CONFLICT` | 409 | `ErrTaskVersionConflict` (board) | «Заказ уже изменён, обновите» | AC-010 |
| 07 | `CLIENT_HAS_TASKS` | 409 | `ErrClientHasTasks` (clients) | «У клиента есть заказы — удалить нельзя» | AC-017 |
| 08 | `EQUIPMENT_USED_IN_TASKS` | 409 | `ErrEquipmentUsedInTasks` (clients) | «Оборудование было в заказе — удалить нельзя» | AC-018 |
| 09 | `SERVICE_IN_USE` | 409 | `ErrServiceInUse` (catalog) | «Услуга используется в заказах — удалить нельзя, отправьте её в архив» | ADR-005 (новый FK `subtasks.service_id`) |
| 10 | `PRODUCT_NOT_FOUND` | 404 | `ErrStockProductNotFound` (board; код = catalog) | «Товар не найден» | AC-015 |
| 11 | `PAID_ADMIN_ONLY` | 403 | `ErrPaidAdminOnly` (board) | «Оплату меняет только администратор» | AC-011 |
| 12 | `FORBIDDEN` | 403 | `middleware.RequireAdmin(nil)` (как есть) — только роль-гард маршрута | «Недостаточно прав» | AC-006, AC-012, AC-015 |
| 13 | `VALIDATION` | 400 | `errorspkg.Validation(...)` | `details.fields[]` | AC-006, AC-009 |
| 14 | `BAD_REQUEST` | 400 | strictJSON / тип поля | мусорное тело (TD-4) | — |
| 15 | `PRODUCT_IN_USE` | 409 | `ErrProductInUse` (catalog, as-is) — текст расширить | «Товар используется в услугах, заказах или журнале склада — удалить нельзя, отправьте его в архив» | ADR-005 |

- Уточнение spec (§Обработка ошибок «404 клиент/ответственный — в форме ошибка поля»): ссылки ИЗ ТЕЛА (`client_id`, `column_id`, `assignee_id`) → 400 `VALIDATION` с `details.fields[{name, message:"не найден"}]` (эталон `catalog` `category_id`); 404 — только id в пути. Пользователь видит ошибку поля — поведение spec сохранено.
- 403 различимы по `code`: `FORBIDDEN` — маршрут закрыт ролью; `PAID_ADMIN_ONLY` — маршрут доступен, запрещено только изменение `is_paid` (клиент блокирует переключатель оплаты, форма остаётся рабочей).
- `tasks.id` в пути: `strconv.ParseInt` > 0, иначе `TASK_NOT_FOUND` (до SQL). uuid-пути (`columns/:id`, `stock/:product_id`) — `uuid.Validate`, иначе 404.

## Details Subtasks
```json
{"code":"SUBTASKS_NOT_DONE","message":"...","details":{"subtasks":[{"id":"<uuid>","service_name":"Замена картриджа","status":"in_progress"}]}}
```

## Validation Fields
| Поле | Правило | message |
|---|---|---|
| `title` | trim, 1..200 | «обязательно» / «не длиннее 200» |
| `client_id` | uuid, существует | «обязательно» / «не найден» |
| `column_id` | uuid, существует | «обязательно» / «не найдена» |
| `assignee_id` | NULL или uuid активного пользователя (проверка только при смене значения) | «не найден» |
| `due_date` | NULL или `YYYY-MM-DD` | «неверная дата» |
| `version` (PUT задачи) | обязателен, ≥1 | «обязательно» |
| `name` (колонка) | trim, 1..60 | «обязательно» / «не длиннее 60» |
| `color` | `^#[0-9A-Fa-f]{6}$` | «цвет в формате #RRGGBB» |
| `position` (колонка) | int ≥1 (clamp к N) | «неверная позиция» |
| `prev_id`/`next_id` | int64 или null (тип — strictJSON 400) | — |
| `stock` (Остатки) | число, NUMERIC(12,3) (валидатор `pkg/numeric`) | «слишком большое» / «не больше 3 знаков» |
| query `paid`/`due`/`assignee`/`scope` | enum (design-go-layers §Enum), иначе 400 field | «неверное значение» |
| query `limit`/`offset` | int, `limit` 1..1000, `offset` ≥0 | «неверное значение» |

## Sentinel Errors
```go
// internal/features/board/errors.go
var (
    ErrTaskNotFound         = errorspkg.New("TASK_NOT_FOUND", http.StatusNotFound, "Заказ не найден")
    ErrColumnNotFound       = errorspkg.New("COLUMN_NOT_FOUND", http.StatusNotFound, "Колонка не найдена")
    ErrSubtasksNotDone      = errorspkg.New("SUBTASKS_NOT_DONE", http.StatusConflict, "Есть незавершённые услуги — в финальную колонку нельзя")
    ErrColumnNotEmpty       = errorspkg.New("COLUMN_NOT_EMPTY", http.StatusConflict, "В колонке есть заказы")
    ErrLastNonFinalColumn   = errorspkg.New("LAST_NON_FINAL_COLUMN", http.StatusConflict, "Нужна хотя бы одна нефинальная колонка")
    ErrTaskVersionConflict  = errorspkg.New("TASK_VERSION_CONFLICT", http.StatusConflict, "Заказ уже изменён, обновите")
    ErrStockProductNotFound = errorspkg.New("PRODUCT_NOT_FOUND", http.StatusNotFound, "Товар не найден")
    ErrPaidAdminOnly        = errorspkg.New("PAID_ADMIN_ONLY", http.StatusForbidden, "Оплату меняет только администратор")
)
// репозиторные (внутренние): errNotFound, errVersionConflict, errColumnHasTasks (23503 columns FK)

// internal/features/clients/service.go — добавить
ErrClientHasTasks       = errorspkg.New("CLIENT_HAS_TASKS", http.StatusConflict, "У клиента есть заказы — удалить нельзя")
ErrEquipmentUsedInTasks = errorspkg.New("EQUIPMENT_USED_IN_TASKS", http.StatusConflict, "Оборудование было в заказе — удалить нельзя")

// internal/features/catalog/services_service.go — добавить
ErrServiceInUse = errorspkg.New("SERVICE_IN_USE", http.StatusConflict, "Услуга используется в заказах — удалить нельзя, отправьте её в архив")
```

## Error Mapping
| Источник | Условие | → |
|---|---|---|
| board repo | `pgx.ErrNoRows` на Lock Task | `ErrTaskNotFound` |
| board repo | UPDATE ... `version=$2` 0 строк, id есть | `ErrTaskVersionConflict` |
| board repo | 23503 на `tasks_column_id_fkey` при DELETE columns | `ErrColumnNotEmpty` |
| board repo | 23503 на `tasks_client_id_fkey`/`tasks_assignee_id_fkey` при INSERT/UPDATE (гонка с удалением) | 400 field `client_id`/`assignee_id` |
| board tx | 40P01 / 40001 | повтор ×3 (`withTx`), затем 500 INTERNAL |
| board tx | 22003 в Reconcile | 500 INTERNAL + `stock_reconcile_errors_total++` |
| clients repo | 23503 `tasks_client_id_fkey` (гонка EXISTS→DELETE) | `ErrClientHasTasks` |
| clients repo | 23503 `task_equipment_equipment_id_fkey` / `subtasks_equipment_id_fkey` | `ErrEquipmentUsedInTasks` |
| catalog services repo | 23503 `subtasks_service_id_fkey` | `ErrServiceInUse` |
| catalog products repo | 23503 (любой FK, as-is) | `ErrProductInUse` |
| SSR clients/equipment/services delete | AppError 409 | PRG-flash с `Message` (as-is механизм `ssr.go remove`) |
- Классификация по `pgErr.ConstraintName` (не по тексту). Неизвестная ошибка → 500 без текста (as-is `FiberErrorHandler`).

## Слайс: 3-design/design-go-layers.md [full]
# Design Go Layers: phase-5-board

> Owner: пакеты, DTO, enum'ы, интерфейсы service/repo, хендлеры, роуты, SSR/JS-модули. Эталон — плоский многосущностный пакет `internal/features/catalog` (префиксы файлов по сущности, D43), код вручную без scaffold. Нейминг — CLAUDE.md §GO-1; URL/JSON — §API-1.

## Package Layout
`internal/features/board` (package `board`, один пакет — ADR-100):
| Файл | Содержимое |
|---|---|
| `errors.go` | sentinel'ы (design-errors §Sentinel Errors) |
| `enums.go` | константы §Enum, `ParseXxx` |
| `columns_dto.go` / `columns_validate.go` / `columns_repo.go` / `columns_service.go` / `columns_handler.go` | колонки |
| `tasks_dto.go` / `tasks_validate.go` / `tasks_filter.go` / `tasks_repo.go` / `tasks_service.go` / `tasks_handler.go` | заказы, фильтры, move |
| `position.go` | Position Query + ребалансировка (design-sql §Position Query) |
| `stock_engine.go` | `StockEngine.Reconcile` + warnings (ADR-104) — экспортируется для `internal/seeddemo` |
| `stock_dto.go` / `stock_repo.go` / `stock_service.go` / `stock_handler.go` | «Остатки», summary, adjust, `NavBadges` |
| `assignees_handler.go` | `GET /api/v1/assignees` |
| `tx.go` | `withTx(ctx, pool, fn)` — retry 40P01/40001 ×3 |
| `ssr.go` | `Root = "web/ssr/board/templates"`, страницы `/` и `/stock` |
| `routes.go` | `Register(app, api, a, h *Handlers, s *SSR)` |
| `*_validate_test.go`, `*_fuzz_test.go`, `board_http_live_test.go`, `stock_engine_integration_test.go` | тесты (design-tests) |

Прочие изменения:
| Путь | Изменение |
|---|---|
| `pkg/numeric/amount.go` (+`amount_validate.go`) | перенос `Amount` + валидатора NUMERIC из `catalog` (ADR-101) |
| `internal/features/catalog/amount.go` | `type Amount = numeric.Amount` (alias), валидатор — вызов `numeric` |
| `internal/features/catalog/services_repo.go` / `services_service.go` | 23503 → `ErrServiceInUse` |
| `internal/features/catalog/products_service.go` | текст `ErrProductInUse` |
| `internal/features/clients/repo.go` / `service.go` | D26 guard'ы (design-sql §Client/Equipment Delete Guard Query) |
| `pkg/errorspkg/errors.go` | `Extra` + `WithDetail` (design-errors §errorspkg Extension) |
| `internal/ssr/menu.go` | `NavItem.Badge int`; `BuildMenu(enabled, isAdmin, badges map[string]int)` |
| `internal/ssr/shell.go` | `BadgeSource` + `ShellBuilder.WithBadges` (ADR-004) |
| `internal/server/router.go` | `NewShellBuilder("/", "/stock", "/users", "/catalog", "/clients").WithBadges(...)`; `board.Register`; удалить `home` |
| `internal/features/home/`, `web/ssr/home/` | удалить (доска занимает `/`, ADR-100) |
| `internal/seeddemo/seeddemo.go` | `Run(ctx, pool) (Result, error)` — AC-019 (ADR-105) |
| `cmd/seed-demo/main.go` | тонкая обёртка: DATABASE_URL → `seeddemo.Run` |
| `Makefile` | цель `seed-demo:` ВНЕ блока `project-bootstrap` (`. scripts/dev/env.sh && go run ./cmd/seed-demo`) |
| `migrations/0005_board.{up,down}.sql` | design-sql |
| `e2e/package.json` | devDependency `pg`, `@types/pg` (ADR-103) |

## Enum SubtaskStatus
`new`, `in_progress`, `done` — источник: `board/enums.go::SubtaskStatuses` = CHECK `subtasks.status` (0005); sync-тест `enums_test.go` (сверка с `pg_get_constraintdef` в live-сьюте).

## Enum PaidFilter
`all` (дефолт), `paid`, `unpaid` — `board/enums.go::PaidFilters`.

## Enum DueFilter
`all` (дефолт), `soon` (просрочено ИЛИ срок сегодня/завтра ИЛИ есть просроченная услуга), `overdue` (просрочено ИЛИ просроченная услуга); финальные колонки не входят — `board/enums.go::DueFilters` (ADR-003, по `prototype/index.html:589-596,784-785`).

## Enum AssigneeFilter
`me`, `all`, `<uuid пользователя>`; отсутствует → по роли: executor=`me`, admin=`all` (AC-014) — `board/enums.go::AssigneeMe/AssigneeAll`.

## Enum StockScope
`risky` (дефолт), `all` — `board/enums.go::StockScopes`.

## Enum StockState
`out`, `low`, `will_low`, `ok` — вычисляется SQL (design-sql §Stock Pending CTE); `board/enums.go::StockStates`.

## Enum WarningKind
`negative`, `below_min` — `board/enums.go::WarningKinds`.

## Enum StockMovementReason
`task_consume`, `task_return`, `manual` — as-is CHECK `0004_catalog.up.sql:59`; board пишет `task_consume`/`task_return`.

## DTO Column
```go
type ColumnResponse struct {
    ID            string `json:"id"`
    Name          string `json:"name"`
    Color         string `json:"color"`
    IsFinal       bool   `json:"is_final"`
    ConsumesStock bool   `json:"consumes_stock"`
    Position      int    `json:"position"`
    TasksCount    int    `json:"tasks_count"`
}
type ColumnListResponse struct { Items []ColumnResponse `json:"items"`; Total, Limit, Offset int /* json total/limit/offset */ }
// Порядок: position, id. Дефолты: limit=100 (1..1000), offset=0.
type ColumnInput struct {           // POST и PUT /api/v1/columns[/:id]
    Name          string `json:"name"`
    Color         string `json:"color"`
    IsFinal       bool   `json:"is_final"`
    ConsumesStock bool   `json:"consumes_stock"`
    Position      *int   `json:"position"` // POST: nil → в конец; PUT: nil → без перемещения
}
type ColumnMutationResponse struct {
    Column   ColumnResponse `json:"column"`
    Warnings []StockWarning `json:"warnings"` // не nil: [] (TS без null)
}
```

## DTO Task
```go
type TaskCard struct {              // элемент списка доски
    ID            int64   `json:"id"`
    ColumnID      string  `json:"column_id"`
    ClientID      string  `json:"client_id"`
    ClientName    string  `json:"client_name"`
    Title         string  `json:"title"`
    AssigneeID    *string `json:"assignee_id"`
    AssigneeName  *string `json:"assignee_name"`
    DueDate       *string `json:"due_date"`       // YYYY-MM-DD
    IsPaid        bool    `json:"is_paid"`
    Position      string  `json:"position"`       // NUMERIC строкой — клиенту только для отладки/сортировки
    Version       int64   `json:"version"`
    SubtasksTotal int     `json:"subtasks_total"`
    SubtasksDone  int     `json:"subtasks_done"`
    SubtasksLate  int     `json:"subtasks_late"`
}
type TaskListResponse struct { Items []TaskCard `json:"items"`; Total, Limit, Offset int }
// Гарантия порядка items (контракт): columns.position, columns.id, tasks.position, tasks.id —
// т.е. по колонкам слева направо, внутри колонки сверху вниз; стабилен между запросами.
// Дефолты: limit=500 (1..1000), offset=0.
type TaskResponse struct {          // GET /tasks/:id и в mutation-ответах
    TaskCard
    PaidAt    *time.Time `json:"paid_at"`
    CreatedAt time.Time  `json:"created_at"`
    UpdatedAt time.Time  `json:"updated_at"`
}
type TaskCreateRequest struct {
    Title      string  `json:"title"`
    ClientID   string  `json:"client_id"`
    ColumnID   string  `json:"column_id"`
    AssigneeID *string `json:"assignee_id"`
    DueDate    *string `json:"due_date"`
    IsPaid     *bool   `json:"is_paid"`   // executor: nil|false, иначе 403
}
type TaskUpdateRequest struct {
    TaskCreateRequest
    Version *int64 `json:"version"`       // обязателен (§FE-1.1)
}
type TaskMoveRequest struct {
    ColumnID string `json:"column_id"`
    PrevID   *int64 `json:"prev_id"`       // карточка СВЕРХУ после вставки (ADR-001)
    NextID   *int64 `json:"next_id"`       // карточка СНИЗУ
}
type TaskMutationResponse struct {
    Task     TaskResponse   `json:"task"`
    Warnings []StockWarning `json:"warnings"`
}
type TaskDeleteResponse struct {
    Warnings []StockWarning `json:"warnings"`
}
```
- `description_html` в фазе 5 не входит в DTO (D25, 5 полей формы) — PUT его не перезаписывает.

## DTO TaskFilter
```go
type TaskFilter struct {            // tasks_filter.go: ParseTaskFilter(c fiber.Ctx, a Actor) (TaskFilter, error)
    ClientID   *string  // client_id (uuid; невалидный → 400 field)
    Q          string   // q, trim, ≤100
    TaskNo     *int64   // из q ~ ^#?\d{1,18}$
    CategoryID *string  // category_id (uuid)
    Paid       *bool    // paid: all→nil
    AssigneeID *string  // assignee: me→a.ID, all→nil, uuid
    Due        string   // "" | soon | overdue
    Limit      int      // default 500, 1..1000
    Offset     int      // default 0
}
```

## DTO StockWarning
Q-006 (ADR-006):
```go
type StockWarning struct {
    ProductID string         `json:"product_id"`
    Name      string         `json:"name"`
    Stock     numeric.Amount `json:"stock"`
    MinStock  numeric.Amount `json:"min_stock"`
    Kind      string         `json:"kind"` // negative | below_min
}
```

## DTO Stock
```go
type StockRow struct {
    ProductID string         `json:"product_id"`
    Name      string         `json:"name"`
    Unit      string         `json:"unit"`
    Box       string         `json:"box"`
    Place     string         `json:"place"`
    Stock     numeric.Amount `json:"stock"`
    MinStock  numeric.Amount `json:"min_stock"`
    Pending   numeric.Amount `json:"pending"`    // потребность не взятых в работу заказов
    Forecast  numeric.Amount `json:"forecast"`
    State     string         `json:"state"`      // out|low|will_low|ok
}
type StockListResponse struct { Items []StockRow `json:"items"`; Total, Limit, Offset int }
type StockSummaryResponse struct {
    ProductsTotal int `json:"products_total"`
    LowCount      int `json:"low_count"`
    OutCount      int `json:"out_count"`
    WillLowCount  int `json:"will_low_count"`
    RiskCount     int `json:"risk_count"`
}
type StockAdjustRequest struct { Stock *numeric.Amount `json:"stock"` }   // обязателен
// ответ adjust — StockRow
```

## DTO Assignee
```go
type AssigneeResponse struct { ID string `json:"id"`; Name string `json:"name"`; Role string `json:"role"` }
type AssigneeListResponse struct { Items []AssigneeResponse `json:"items"`; Total, Limit, Offset int }
// Порядок: name, id. Дефолты: limit=200 (1..1000), offset=0; только is_active.
```

## TS Mirror
§TD-6.2: `e2e/fixtures/board-types.ts` — интерфейсы `Column`, `TaskCard`, `Task`, `StockWarning`, `StockRow`, `StockSummary`, `ApiErrorSubtasks` 1:1 по json-тегам выше, с комментарием-источником `// source: internal/features/board/*_dto.go`. Wire snake_case, TS-поля — snake (без camelize).

## Service Interface
```go
// Actor — из middleware.CurrentPrincipal(c) (эталон catalog/products_handler.go:77-80); ssr.Viewer id не несёт.
type Actor struct { ID string; IsAdmin bool }
type ColumnService interface {
    List(ctx) (*ColumnListResponse, error)
    Create(ctx, userID string, in ColumnInput) (*ColumnMutationResponse, error)
    Update(ctx, id, userID string, in ColumnInput) (*ColumnMutationResponse, error) // смена consumes_stock → Recalc (AC-007)
    Delete(ctx, id string) error
}
type TaskService interface {
    List(ctx, f TaskFilter) (*TaskListResponse, error)
    Get(ctx, id string) (*TaskResponse, error)
    Create(ctx, a Actor, in TaskCreateRequest) (*TaskMutationResponse, error)
    Update(ctx, a Actor, id string, in TaskUpdateRequest) (*TaskMutationResponse, error)
    Move(ctx, a Actor, id string, in TaskMoveRequest) (*TaskMutationResponse, error)
    Delete(ctx, a Actor, id string) (*TaskDeleteResponse, error)
}
type StockService interface {
    List(ctx, scope, q string, limit, offset int) (*StockListResponse, error)
    Summary(ctx) (*StockSummaryResponse, error)
    Adjust(ctx, productID, userID string, in StockAdjustRequest) (*StockRow, error)
    NavBadges(ctx) map[string]int  // ssr.BadgeSource; ошибка → slog.Warn + nil
}
```
Алгоритм `TaskService.Move` (Driver AC-002/003/004):
1. `id` → int64; `withTx`:
2. Lock Target Column `FOR SHARE` (нет → 400 field `column_id`).
3. Lock Task `FOR UPDATE` (нет → `ErrTaskNotFound`).
4. Если `target.is_final` и `column_id` сменилась ИЛИ заказ уже не в final → Open Subtasks; ≥1 → `ErrSubtasksNotDone.WithDetail("subtasks", list)`. (Перестановка внутри той же финальной колонки — без проверки.)
5. Position Query (+ребалансировка) → Move Task Query (`version+1`).
6. `engine.Reconcile(tx, id, target.consumes_stock, a.ID)` → warnings.
7. span `tasks.move` (attrs: `task_id`, `from_column`, `to_column`, `movements_count`).
`Update`: те же шаги 2–6 при смене `column_id` (позиция = в конец); `is_paid` ≠ текущего у executor → `ErrPaidAdminOnly` (403 `PAID_ADMIN_ONLY`) (проверка ПОСЛЕ Lock Task, до UPDATE); UPDATE по `version`.
`Create`: Lock Target Column → проверка client/assignee → Insert (позиция в конец) → Reconcile (no-op без услуг; единообразие).
`Delete`: только admin (роут + повторная проверка роли в сервисе) → Lock Task → Reconcile(false) → Delete Task Query.

## Repository Interface
Структуры `*ColumnRepo`, `*TaskRepo`, `*StockRepo` над `*pgxpool.Pool`; методы, работающие в транзакции, принимают `pgx.Tx`:
```go
// TaskRepo
LockTargetColumn(ctx, tx pgx.Tx, columnID string) (columnFlags, error)
LockTask(ctx, tx pgx.Tx, id int64) (*Task, error)
OpenSubtasks(ctx, tx pgx.Tx, id int64) ([]OpenSubtask, error)
NextPosition(ctx, tx pgx.Tx, columnID string, taskID int64, prev, next *int64) (string, error) // position.go
Move(ctx, tx pgx.Tx, id int64, columnID, pos string) (*Task, error)
Update(ctx, tx pgx.Tx, id int64, version int64, in taskRow) (*Task, error)
Insert(ctx, tx pgx.Tx, in taskRow, createdBy string) (*Task, error)
Delete(ctx, tx pgx.Tx, id int64) error
List(ctx, f TaskFilter) ([]TaskCard, int, error)
Get(ctx, id int64) (*Task, error)
// ColumnRepo
LockAll(ctx, tx pgx.Tx) ([]Column, error)            // ORDER BY id FOR UPDATE
Insert/Update/Renumber/Delete(ctx, tx, ...)
TaskIDsForUpdate(ctx, tx pgx.Tx, columnID string) ([]int64, error)
// StockEngine
Reconcile(ctx, tx pgx.Tx, taskID int64, consumes bool, userID string) (ReconcileResult{Warnings []StockWarning; Movements int}, error)
// StockRepo
List/Summary/Adjust(...)
```

## Handler Columns
`columns_handler.go`: `List`, `Create`(201), `Update`(200), `Delete`(204 без тела — удаление колонки склад не трогает). Bind — `c.Bind().Body(&in)` (strictJSON), ошибка → `errorspkg.BadRequest`.

## Handler Tasks
`tasks_handler.go`: `List` (ParseTaskFilter с `Actor`), `Get`, `Create`(201), `Update`, `Move`, `Delete`(200 + `TaskDeleteResponse{warnings}` — удаление возвращает товары на склад, warnings нужны клиенту).

## Handler Stock
`stock_handler.go`: `List` (`scope`,`q`,`limit` default 200 max 1000,`offset`), `Summary`, `Adjust`.

## Routes
```go
func Register(app *fiber.App, api fiber.Router, a *middleware.Auth, h *Handlers, s *SSR) {
    admin := middleware.RequireAdmin(nil)
    col := api.Group("/columns", a.RequireAPI())
    col.Get("", h.Columns.List); col.Post("", admin, h.Columns.Create)
    col.Put("/:id", admin, h.Columns.Update); col.Delete("/:id", admin, h.Columns.Delete)

    t := api.Group("/tasks", a.RequireAPI())
    t.Get("", h.Tasks.List); t.Post("", h.Tasks.Create)
    t.Get("/:id", h.Tasks.Get); t.Put("/:id", h.Tasks.Update)
    t.Post("/:id/move", h.Tasks.Move); t.Delete("/:id", admin, h.Tasks.Delete)

    st := api.Group("/stock", a.RequireAPI())
    st.Get("", h.Stock.List); st.Get("/summary", h.Stock.Summary)
    st.Put("/:product_id", admin, h.Stock.Adjust)

    api.Get("/assignees", a.RequireAPI(), h.Assignees.List)

    if s == nil { return }
    app.Get("/", a.RequireSSR(), s.board)
    app.Get("/stock", a.RequireSSR(), s.stock)
}
```
- Path-параметр `:product_id` — допустим (§API-1 разрешает `:id`-параметры; snake внутри `:name` — не сегмент URL). Сегменты `columns`, `tasks`, `move`, `stock`, `summary`, `assignees` — kebab-валидны.

## Handler SSR
`ssr.go`: `board` → `Page(c, "board", BoardVM{Shell, ViewerID, IsAdmin})`, `stock` → `Page(c, "stock", StockVM{Shell})`. Данные грузит JS (rich-экран, D30); SSR отдаёт только каркас + `data-*` атрибуты (`data-viewer-id`, `data-is-admin`).

## Shell Badge
ADR-004:
```go
// internal/ssr/shell.go
type BadgeSource interface{ NavBadges(ctx context.Context) map[string]int }
func (b ShellBuilder) WithBadges(src BadgeSource) ShellBuilder // копия
// Build: badges := b.src.NavBadges(c.Context()) (если src != nil) → BuildMenu(enabled, isAdmin, badges)
// menu.go: NavItem{..., Badge int}
```
`base.html`: у пункта `{{.URL}}` = `/stock` рендерится `<span class="badge rounded-pill text-bg-danger ms-auto" data-nav-badge="{{.URL}}" data-testid="nav-badge-stock" {{if not .Badge}}hidden{{end}}>{{.Badge}}</span>` (span есть всегда — цель JS-обновления).

## Frontend Modules
Стек D30/D38: SSR html/template + vanilla JS + vendored Bootstrap 5.3; Element Plus/Vue — НЕ применимо.
| Файл | Роль |
|---|---|
| `web/ssr/board/templates/pages/board.html` | каркас доски: `{{block "actions"}}` (+Заказ; +Колонка — `{{if .Shell.IsAdmin}}`), фильтр-бар, `#board` (`.board-columns` — горизонтальный скролл контейнера), модалки `task-modal`, `column-modal`, `{{block "scripts"}}` → `board.js` |
| `web/ssr/board/templates/pages/stock.html` | KPI-плитки, переключатель risky/all, таблица, модалка правки остатка (admin), скрипт `stock.js` |
| `web/ssr/_shared/js/board.js` | `KT.board`: загрузка (`/api/v1/columns`, `/api/v1/tasks?<query>`, `/api/v1/assignees`, `/api/v1/catalog-categories?kind=service`), рендер колонок/карточек, HTML5 DnD (`dragstart/dragover/drop`, вычисление prev/next по DOM), оптимистичный перенос + откат на ошибке, меню карточки «Переместить в…» (ADR-106), фильтры ↔ `location.search` (`history.replaceState`), формы заказа/колонки (combobox клиента D6, блокировка «Сохранить» до ответа), toast warnings со ссылкой `/stock`, `ui-dialog` для 409 |
| `web/ssr/_shared/js/stock.js` | `KT.stock`: summary + list, правка остатка (admin) |
| `web/ssr/_shared/js/nav-badge.js` | `KT.refreshNavBadge()` → `GET /api/v1/stock/summary` → `[data-nav-badge="/stock"]`; подключается в `base.html` после `api.js` |
| `web/ssr/_shared/app.css` | `.board-columns` (flex, `overflow-x:auto`, `min-width:0`), `.board-col` (ширина 280px; ≤576px — 85vw), `.task-card` (`border-left: 4px solid var(--col-color)`), `.task-card.dragging/.drop-target`, overdue — `--bs-danger*`; только `--bs-*` токены (themes:both) |
Инвалидация бейджа: `KT.refreshNavBadge()` после успешных move / PUT со сменой колонки / POST / DELETE заказа / PUT колонки / PUT stock.

testid (E2E-контракт POM): `board-column-{id}`, `task-card-{id}`, `task-card-menu-{id}`, `task-move-to-{columnId}`, `filter-client|q|category|paid|assignee|due|reset`, `task-form`, `task-title`, `task-client`, `task-column`, `task-assignee`, `task-due`, `task-paid`, `task-save`, `task-delete`, `column-form`, `column-name|color|final|consumes|position|save|delete`, `toast-warnings`, `stock-kpi-low|out|will-low`, `stock-row-{productId}`, `stock-edit-{productId}`, `stock-scope`.

## Слайс: 3-design/design-tests.md [sections]
секции по [Uu]nit|[Ii]ntegration|Strategy|Стратег не найдены

## Слайс: 1-research/output.md [sections]
## Паттерны в коде
- Плоская фича без вложенных каталогов — эталон `clients` (`internal/features/clients/*`, D43 `docs/plan.md:167`); многосущностная — префиксы файлов по сущности в одном пакете (`internal/features/catalog/*`).
- `Register(app,api,a,h,s)`, `s==nil`→SSR не регистрируется (`internal/features/clients/routes.go:14,30-32`); `admin:=middleware.RequireAdmin(nil)` per-route, не групповой (`:15-28`).
- Ошибки — только `pkg/errorspkg`, sentinel-`*AppError` в `var(...)` блоке `service.go` (`internal/features/clients/service.go:17-22`); валидация ручная в `service.go`/`validate.go`: `in.normalize()`→`in.validate()`→`errorspkg.Validation(f...)`.
- NUMERIC-поля — `type Amount string` с кастомным `Marshal/UnmarshalJSON` (`internal/features/catalog/amount.go:9,14-33`), не `float64`.
- Оптимистичная блокировка `version` — `ErrXVersionConflict`+`errors.Is(err,errVersionConflict)`, поле `version *int64` в теле, не If-Match (§FE-1.1) (`internal/features/clients/service.go:141-164`).
- `uuid.Validate(id)` до любого SQL-запроса (`internal/features/clients/service.go:112,142,167,193,222,240,284,323`); все id-PK — `uuid DEFAULT gen_random_uuid()` в БД, кроме `tasks.id BIGSERIAL`(D37) — Go не генерирует UUID.
- `pool.QueryRow/Exec` напрямую из `*pgxpool.Pool`, без ORM (`internal/features/clients/repo.go:1`).
- SSR — один `ssr.Renderer` на фичу, единственный layout — `web/ssr/_layout/templates/layouts/base.html`; сайдбар растёт через `enabled`-список `ssr.NewShellBuilder(...)` (`router.go:77`) — путь фичи нужно явно добавить, иначе пункт скрыт даже если уже в `menu.go`.
- Тесты — `BaseSuite`(dockertest)→`LiveSuite`(+реальный бинарь+HTTP), auth только через реальный login (§TD-8); E2E — POM-класс на фичу+`AdminApi` fixture, TS-типы дублируют Go DTO с комментарием источника (§TD-6.2).

## Слайс: AC-строки
#### AC-001: Доска с колонками и карточками
#### AC-002: Статус заказа — это колонка
#### AC-003: Финальная колонка только для завершённых заказов
#### AC-004: Списание товаров при запуске в работу
#### AC-005: Минус на складе не блокирует, но предупреждает
#### AC-006: Управление колонками
#### AC-007: Смена «списывать товары» у колонки
#### AC-008: Колонки по умолчанию
#### AC-009: Создание и правка заказа простой формой
#### AC-010: Одновременная правка заказа
#### AC-011: Оплата меняется только администратором
#### AC-012: Удаление заказа
#### AC-013: Фильтры доски
#### AC-014: «Мои» по умолчанию для исполнителя
#### AC-015: Экран «Остатки»
#### AC-016: Счётчик рисковых товаров в меню
#### AC-017: Нельзя удалить клиента с заказами
#### AC-018: Нельзя удалить оборудование, бывшее в заказе
#### AC-019: Демо-данные одной командой
#### AC-020: Работа на телефоне и в обеих темах

## Паспорт (выжимка)
# Passport: phase-5-board (generated — руками не править; stage_bundle.py)
**Tier:** M | **Совместимость:** ? (pipeline-doctrine.md §CM-1; ? → уточни у оркестратора)
**Трек:** обычный (prefs нет — дефолт) — режимы агентов (`spec-track delta/micro/mini/merged`) включает ТОЛЬКО строка `Режим:` в prompt оркестратора; DS/TS-индексы ниже присутствуют на ЛЮБОМ треке при наличии draft-spec.
**Layout:** автодетект (tools/layout_resolve.py)
**UI-scope (pipeline-doctrine.md §US-1, 0-setup/ui-scope-prefs.md):** swagger:no | adaptive:min | themes:both

**Индексы:** AC 20 / DS 0 / TS 0 — полные списки: `0-setup/passport.md` (разрешён контрактом); конкретное решение — точечный grep по `DS-NNN`/`TS-NNN` в draft-spec §9/§11 (разрешён).
