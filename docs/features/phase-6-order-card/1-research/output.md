---
stage: 1-research
feature: phase-6-order-card
tier: M
---
# Research: phase-6-order-card

## Triage
```yaml
scope: cross-feature
tier: M
touches:
  db: true
  fe: false
  infra: false
  external: false
risk: reversible
novelty: standard-crud
decisions: []
```
Обоснование: фича в `docs/plan.md:6` треком `/pipeline (M)` явно (D31, `docs/plan.md:155`). Не Vue — SSR + vanilla JS (`web/ssr/_shared/js/board.js:1`), `touches.fe=false` формально. `scope=cross-feature`: новая таблица subtask_logs, новые эндпоинты subtasks/subtask-products/subtask-logs/task-equipment/print, расширение tasks/board.js/SSR-печати, переиспользование API оборудования из фичи clients.

## Module
- Модуль: `github.com/kitavrus/kanban-tree` (go.mod:1), Go 1.26.0 (go.mod:3).
- Зависимости: `gofiber/fiber/v3 v3.5.0` (go.mod:10), `jackc/pgx/v5 v5.11.0` (go.mod:13), `google/uuid v1.6.0` (go.mod:12), `golang-migrate/v4 v4.20.1` (go.mod:11), `ory/dockertest/v3` (go.mod:14), `redis/go-redis/v9` (go.mod:16), OTel (go.mod:19-22).
- ID-генерация — БД `gen_random_uuid()` DEFAULT (миграции), приложение UUID само не генерирует (кроме `google/uuid.Validate` для проверки формата входа); `pkg/ids`-обёртки (CLAUDE.md §8 шаблон) в проекте НЕТ — это другой проект/иной паттерн.

## Что уже есть в коде

### Backend
- Пакет `internal/features/board` (`internal/features/board/errors.go:1-4`) — многосущностный (columns/tasks/subtasks-заготовка/stock), файлы по сущности: `{entity}_dto.go`/`_handler.go`/`_repo.go`/`_service.go`/`_validate.go`.
- `internal/features/board/tasks_dto.go:10-61` — `TaskCard`(:10-25)/`TaskResponse`(:38-43)/`TaskCreateRequest`+`TaskUpdateRequest`(:47-61) — ТОЛЬКО простые поля (title/client_id/column_id/assignee_id/due_date/is_paid); НЕТ description_html/subtasks[]/equipment[] в теле (только счётчики `subtasks_total/done/late`, :22-24). `OpenSubtask` (:86-90 — id/service_name/status) — только в деталях `SUBTASKS_NOT_DONE`.
- `internal/features/board/tasks_service.go:19-340` — `Actor{ID,IsAdmin}` (:19-22) из `middleware.CurrentPrincipal`. `Create` (:62-124): normalize→validate→paid-guard(executor не может is_paid=true→`ErrPaidAdminOnly`)→`withTx`: `LockTargetColumn`(FOR SHARE)→`ClientExists`→`ActiveAssigneeExists`→`NextPosition`→`Insert`→`Reconcile`(no-op без подзадач). `Update` (:129-229): `LockTargetColumn`→`LockTask`(FOR UPDATE)→version-check ДО paid-guard (порядок — фикс MEDIUM-2, :150-157)→columnChanged?→`OpenSubtasks`-гард на is_final(:188-196)→`NextPosition`→`Update`→`Reconcile`. `Delete` (:294-325) — только admin, повторная проверка роли (:299-301), `Reconcile(false)` возвращает склад ДО удаления. `mutationResponse` (:330-340) — Re-fetch-after-write: ответ из `repo.Get()` ПОСЛЕ commit, не RETURNING.
- `internal/features/board/tasks_repo.go:55-285` — `LockTargetColumn`(:72-83, FOR SHARE), `LockTask`(:86-99, FOR UPDATE), `OpenSubtasks`(:103-122, `subtasks JOIN services WHERE task_id=$1 AND status<>'done'`), `NextPosition`→`position.go:1`, `Move/Update/Insert/Delete`(:131-183), `ClientExists/ActiveAssigneeExists`(:188-200), `Get`(:205-224, JOIN clients+users + инлайн-сабселекты subtasks_total/done/late), `List`+`boardListSQL`(:228-285).
- `internal/features/board/stock_engine.go:50-149` — `StockEngine.Reconcile(ctx,tx,taskID,targetConsumes,userID)` (:97-149) — УЖЕ общий движок для фазы 6: demand из `subtask_products JOIN subtasks` (`reconcileDeltasSQL`, :50-63) — вставка строк `subtask_products` фазой 6 автоматически участвует в Reconcile БЕЗ правки движка. Lock Order — products FOR UPDATE ORDER BY id (:67) ПОСЛЕ блокировки заказа вызывающей стороной (columns→tasks→products, ADR-104).
- `internal/features/board/columns_service.go:21-150` — эталон `LockAll`+`becomesFinal` (:121-150): под `LockAll` (вся таблица columns FOR UPDATE) проверяет `OpenSubtasksForColumn` перед `is_final=true`.
- `internal/features/board/tx.go:16-49` — `withTx`: READ COMMITTED + retry×3 на `40P01`/`40001`.
- `internal/features/board/enums.go:8-72` — `SubtaskStatus{new,in_progress,done}`(:8-14, CHECK subtasks.status), `PaidFilter/DueFilter/AssigneeFilter/StockScope/StockState/WarningKind/StockMovementReason`(:16-72) — источник правды для sync-тестов (`internal/features/board/enums_test.go:1`), сверка с `pg_get_constraintdef` в `internal/features/board/board_http_live_test.go:1`.
- `internal/features/board/errors.go:14-35` — sentinel'ы `ErrTaskNotFound`, `ErrColumnNotFound`, `ErrSubtasksNotDone`(409), `ErrColumnNotEmpty`(409), `ErrLastNonFinalColumn`(409), `ErrTaskVersionConflict`(409), `ErrStockProductNotFound`(404), `ErrPaidAdminOnly`(403); внутренние `errNotFound/errColumnHasTasks/errVersionConflict`(:27-35) мапятся сервисом через `errors.Is`.
- `pkg/errorspkg/errors.go:19-95` — `AppError{Code,HTTP,Message,Fields,Extra}`(:19-25), `New/Validation/Forbidden/Conflict/NotFound/Internal`(:29-58), `WithDetail`— КОПИЯ без мутации sentinel(:36-44, ADR-102), `FiberErrorHandler`(:68-95) — единый Fiber error handler, JSON `{code,message,details:{fields[],...Extra}}`.
- Валидация — ручная, в методах DTO (`in.normalize()`/`in.validate()`), не в handler/service — эталон `internal/features/board/tasks_validate.go:30-100`: `utf8.RuneCountInString`, `uuid.Validate`(google/uuid), `sqlutil.HasInvalidText`, `time.Parse(dateLayout)`. Коды `msgNotFound/msgInvalidDate` — `internal/features/board/tasks_validate.go:17-18`.
- `internal/features/board/routes.go:10-50` — `Handlers{Columns,Tasks,Assignees,Stock}`(:10-15). `/columns`(:22-26, CRUD admin кроме List), `/tasks`(:29-35 — Get/Post/Put/Move без admin, Delete только admin, D19 на :28), `/assignees`(:37), `/stock`(:40-43, PUT admin), SSR `/`,`/stock`(:48-49). НЕТ `/tasks/:id/subtasks`, `/subtasks/:id`, `/subtask-logs`, `/tasks/:id/equipment`, `/subtask-products`, `/tasks/:id/print` — все в `docs/plan.md:87-96`, в коде отсутствуют.
- `internal/features/clients/routes.go:10-48` — оборудование клиента (фаза 3) уже реализовано: `GET/POST /clients/:id/equipment`(:22-23, POST БЕЗ admin — D19, :10-13), `/equipment/:id` GET(RequireAPI)/PUT,DELETE(admin)(:25-28); SSR `/equipment/:id` — ТОЛЬКО admin(`ssrAdmin`, :33,44-47) — расхождение с API (D19 разрешает создание оборудования через API карточки заказа, не через SSR-форму клиента).
- `internal/features/clients/dto.go:56-93` — `SpecDTO{K,V}`, `EquipmentResponse`(ID/ClientID/TypeID/TypeName/TypeIsActive/Brand/Model/SKU/Serial/Specs[]/Note/CreatedAt/UpdatedAt), `EquipmentRequest`; комментарий :53-54 «наполнение — Phase 7» устарел — фактически наполнено в фазе 3.
- `internal/features/clients/repo.go:391-505` — `ListEquipment`(:391-415), `GetEquipment`(:418-423), `CreateEquipment`(:437-452), `UpdateEquipment`(:457-492), `DeleteEquipment`(:498-505, guard D26 `EXISTS task_equipment.equipment_id ∪ subtasks.equipment_id`→`errEquipmentUsedInTasks`) — НЕТ метода истории заказов по единице (нет запроса вида `SELECT tasks WHERE task_equipment.equipment_id=...`).
- `internal/features/catalog/services_dto.go:18-27` — `ServiceResponse.Products []ServiceProductResponse` с денормализованными Name/Unit/Price/Qty — эталон копирования комплекта товаров в заказ (D3/D10): фаза 6 повторяет денормализацию в `subtask_products`.
- `pkg/numeric/amount.go:9-11` — `Amount` для NUMERIC-полей (строка в Go, JSON-число на wire, без float64); `internal/features/catalog/amount.go:8` — алиас `catalog.Amount = numeric.Amount`.

## DB Schema (baseline)
Источник целиком: `migrations/0005_board.up.sql`, `migrations/0003_clients_equipment.up.sql`, `migrations/0004_catalog.up.sql`.

**tasks** (`migrations/0005_board.up.sql:25-44`): id bigserial PK(D37), column_id uuid FK columns RESTRICT, client_id uuid FK clients RESTRICT, title text CHECK(btrim<>''∧len≤200), description_html text DEFAULT '' (:30 — фаза 5 пишет только seed-demo), assignee_id uuid FK users SET NULL, due_date date NULL, is_paid boolean, paid_at timestamptz NULL (CHECK is_paid=(paid_at IS NOT NULL), :40), position numeric, created_by uuid FK users RESTRICT, created_at/updated_at, version bigint DEFAULT 1. Индексы :42-44.

**subtasks** (`migrations/0005_board.up.sql:46-62`, уже создана, Go читает только `OpenSubtasks`): id uuid PK, task_id bigint FK tasks CASCADE, service_id uuid FK services RESTRICT, status text CHECK IN('new','in_progress','done') DEFAULT 'new', assignee_id uuid FK users SET NULL, due_date date NULL, price numeric(12,2) NULL CHECK(≥0), equipment_id uuid FK equipment RESTRICT NULL, position int DEFAULT 0 (НЕ дробный, нет NextPosition-аналога). Индексы :59-62.

**subtask_products** (`migrations/0005_board.up.sql:64-72`, уже создана): id uuid PK, subtask_id uuid FK subtasks CASCADE, product_id uuid FK products RESTRICT, qty numeric(12,3) CHECK(>0) (D33 дробное), price numeric(12,2) NULL CHECK(≥0). Индексы :71-72.

**task_stock** (`migrations/0005_board.up.sql:74-80`, уже использует StockEngine): task_id bigint FK tasks CASCADE, product_id uuid FK products RESTRICT, qty numeric(12,3) CHECK(>0), PK(task_id,product_id).

**task_equipment** (`migrations/0005_board.up.sql:82-89`, создана, Go не читает/не пишет кроме E2E-фикстуры): task_id bigint FK tasks CASCADE, equipment_id uuid FK equipment RESTRICT, completeness text DEFAULT '', condition text DEFAULT '', PK(task_id,equipment_id).

**stock_movements** (создана `migrations/0004_catalog.up.sql:54-63`, task_id мигрирован uuid→bigint в `migrations/0005_board.up.sql:91-96`, `FK task_id→tasks ON DELETE SET NULL`): id uuid PK, product_id uuid FK products RESTRICT, task_id bigint NULL, qty_delta numeric(12,3) CHECK(<>0), reason text CHECK IN('task_consume','task_return','manual'), user_id uuid FK users RESTRICT, created_at.

**equipment** (`migrations/0003_clients_equipment.up.sql:33-47`): id uuid PK, client_id uuid FK clients CASCADE, type_id uuid FK equipment_types RESTRICT, brand/model(CHECK btrim<>'')/sku/serial text, specs jsonb DEFAULT '[]' CHECK(array), note text, created_at/updated_at. Индексы :46-47.

**equipment_types** (`migrations/0003_clients_equipment.up.sql:22-31`): id uuid PK, name text CHECK(btrim<>''), spec_keys text[] DEFAULT '{}', is_active boolean DEFAULT true; UNIQUE lower(btrim(name)) (:31).

**services/products/service_products** (`migrations/0004_catalog.up.sql:17-51`): services(id, category_id FK RESTRICT, name, price numeric(12,2) NULL, is_active) (:34-43); products(id, category_id, name, sku, unit, price numeric(12,2) DEFAULT 0, is_active, stock/min_stock numeric(12,3), box, place) (:17-31); service_products(service_id FK CASCADE, product_id FK RESTRICT, qty numeric(12,3) CHECK>0, position int) (:45-51).

**subtask_logs — ОТСУТСТВУЕТ.** Проверено: полный список миграций `migrations/0001…0005` (5 файлов up/down) не содержит `subtask_logs`. Журнал работ услуги (D13/D27, `docs/plan.md:56,63,138,151`) — НОВАЯ таблица фазы 6; ожидаемые колонки из `docs/plan.md:56`: `subtask_logs(id, subtask_id, user_id, text, created_at, updated_at)`.

## Frontend
Не применимо в терминах Vue — SSR (`Go html/template` + vanilla JS, `docs/plan.md:5`), `frontend/` отсутствует. Фактический "фронт" фичи ниже.

- `web/ssr/board/templates/pages/board.html:1` (133 строки) и `web/ssr/board/templates/pages/stock.html:1` (78 строк) — HTML-шаблоны доски/остатков.
- `web/ssr/_shared/js/board.js:548-667` (815 строк в файле): `openTaskForm(task, defaultColumnId)`(:548-569) — ТОЛЬКО простая форма (title/client/column/assignee/due/paid), без описания/оборудования/услуг/товаров/журнала/итогов; `taskFormPayload()`(:587-602) шлёт `{title,client_id,column_id,assignee_id,due_date,is_paid}` — is_paid ВСЕГДА отправляется даже при disabled-чекбоксе у executor (:590-593, важно сохранить при расширении — иначе тихий сброс оплаты); `bindTaskForm()`(:610-667) — submit→PUT/POST, catch различает `TASK_VERSION_CONFLICT`(:638-643) от `err.fields`(:644-646) от общей(:648); `finishOpenTaskForm`(:571-585) — `data-testid="task-delete"` снимается АТРИБУТОМ, не только `d-none` (:576-579) — конвенция для новых admin-only кнопок.
- `web/ssr/_shared/js/api.js:1` — `KT.api` fetch-обёртка с CSRF, используется всеми хендлерами.
- Общие компоненты: `web/ssr/_shared/js/combobox.js:1`, `web/ssr/_shared/js/ui-dialog.js:1`(`KT.uiConfirm/uiAlert`), `web/ssr/_shared/js/nav-badge.js:1`, `web/ssr/_shared/js/theme.js:1`.
- `e2e/pages/BoardPage.ts:1`, `e2e/pages/EquipmentPage.ts:1` и др. в `e2e/pages/` — POM; `e2e/fixtures/board.ts:1`, `e2e/fixtures/board-types.ts:1` — типы фикстур.

## Contracts
- Защищённые маршруты — все `/api/v1/*` под `a.RequireAPI()` (`internal/features/board/routes.go:22,29,37,40`; `internal/features/clients/routes.go:16,25`); admin-only — `/columns` мутации(`internal/features/board/routes.go:24-26`), `/tasks/:id` DELETE(`:35`), `/stock/:product_id` PUT(`:43`), `/clients` мутации кроме equipment(`internal/features/clients/routes.go:18,20-21`), `/equipment/:id` PUT/DELETE(`:27-28`).
- **D19-исключение:** `POST /clients/:id/equipment` — RequireAPI БЕЗ admin (`internal/features/clients/routes.go:23`, комментарий `:10-13`) — единственный API-эндпоинт справочника, доступный executor на запись; будущие эндпоинты фазы 6 (`POST /tasks/:id/subtasks`, `PUT /subtasks/:id` и т.п.) по D19 (`docs/plan.md:143`) тоже НЕ должны требовать admin (кроме Delete Task и оплаты — уже так, `internal/features/board/routes.go:35`).
- Публичные — нет (весь `/api/v1` за сессией).
- Rate limiting/.env — не проверялись, вне области фазы.
- **Enum coverage (CLAUDE.md §8.1) — НЕ ПРИМЕНИМО в исходном виде:** `UI-scope swagger:no` (`docs/plan.md:11`) — swag-теги `enums:` отсутствуют по архитектуре; источник правды — Go-константы `internal/features/board/enums.go:8-72` + `internal/features/board/enums_test.go:1` (статическая сверка) + live-сьют `internal/features/board/board_http_live_test.go:1` (сверка с `pg_get_constraintdef`). Фаза 6 переиспользует `subtasks.status` (`internal/features/board/enums.go:8-14`) без изменений.
- Error responses: JSON `{code,message,details?}` (`pkg/errorspkg/errors.go:60-64,71-84`) — 400 VALIDATION→`details.fields[]`; коды board — `ErrTaskNotFound`(404), `ErrSubtasksNotDone`(409, `.WithDetail`), `ErrTaskVersionConflict`(409), `ErrPaidAdminOnly`(403) (`internal/features/board/errors.go:15-22`). Ожидаемые НОВЫЕ коды фазы 6 (по аналогии, не в коде): `SUBTASK_NOT_FOUND`, `SUBTASK_PRODUCT_NOT_FOUND`, `SUBTASK_LOG_NOT_FOUND`, `EQUIPMENT_NOT_ACCEPTED`.

## Infrastructure
Вне области (M-tier, `touches.infra=false`) — фаза не меняет docker-compose/CI/Makefile сверх существующего (`docs/plan.md:8`).

## Tests
- `internal/features/board/board_http_live_test.go:1` — LiveSuite(`pkg/testutil.LiveSuite:1`, TD-8): `TestBoardLiveSuite_Update_LastNonFinal_Rejected`, `…StaleVersion_PaidChanged_VersionConflictWins`, `…Update_ToFinal_ColumnHasOpenSubtasks` (номера строк — code review `docs/features/phase-5-board/8-code-review/output.md:18-21`).
- Эталон структуры для новых `subtasks_validate_test.go`/`subtasks_http_live_test.go` (не читаны построчно): `internal/features/board/position_integration_test.go:1`, `internal/features/board/stock_engine_integration_test.go:1`, `internal/features/board/stock_http_live_test.go:1`, `internal/features/board/board_fuzz_test.go:1`, `internal/features/board/tasks_filter_test.go:1`, `internal/features/board/columns_validate_test.go:1`, `internal/features/board/tasks_validate_test.go:1`, `internal/features/board/enums_test.go:1`.
- E2E: `e2e/fixtures/board.ts:5-9` — комментарий: «у доски фазы 6 нет собственного API создания/правки подзадач — единственный способ завести их для E2E — прямая вставка в БД»; `e2e/fixtures/board.ts:24` — `insertSubtask` (INSERT в subtasks+subtask_products, `:45-57`); `e2e/fixtures/board.ts:28` — `insertTaskEquipment` (INSERT в task_equipment, `:65-72`); `setSubtaskStatus` — `e2e/fixtures/board.ts:26,59-63`. Подтверждает отсутствие API subtasks на момент написания фикстуры.
- `e2e/pages/BoardPage.ts:1`, `e2e/pages/EquipmentPage.ts:1`, `e2e/pages/CatalogPage.ts:1`, `e2e/pages/ClientsPage.ts:1`, `e2e/pages/StockPage.ts:1` — POM для переиспользования.
- Coverage — не проверялось.

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

## Чего НЕТ (потребуется создать)
- Таблица `subtask_logs` (журнал работ услуги, D13/D27) — новая миграция следующим номером.
- Go-код для `subtasks`/`subtask_products`/`task_equipment` — таблицы уже есть (`migrations/0005_board.up.sql:46-89`), но ПОЛНОСТЬЮ отсутствуют DTO/handler/service/repo/validate/routes; сейчас единственный доступ — E2E pg-фикстура (`e2e/fixtures/board.ts:1`) и служебное чтение `OpenSubtasks` (`internal/features/board/tasks_repo.go:103-122`).
- Эндпоинты из `docs/plan.md:87-96`, отсутствующие в `internal/features/board/routes.go:1`: `POST /api/v1/tasks/:id/subtasks`, `PUT/DELETE /api/v1/subtasks/:id`, `GET/POST /api/v1/subtasks/:id/logs`, `PUT/DELETE /api/v1/subtask-logs/:id`, `PUT /api/v1/tasks/:id/equipment`, `POST /api/v1/subtasks/:id/products`, `PUT/DELETE /api/v1/subtask-products/:id`, `PUT /api/v1/tasks/:id/payment`, `GET /tasks/:id/print?kind=intake|handover`.
- Расширение `TaskCreateRequest/TaskUpdateRequest/TaskCard/TaskResponse` (`internal/features/board/tasks_dto.go:10-61`) полями description_html/equipment[]/subtasks[] — колонка `tasks.description_html` в БД уже есть (`migrations/0005_board.up.sql:30`), DTO её не сериализует.
- Механизм позиционирования подзадач — `subtasks.position int DEFAULT 0` (`migrations/0005_board.up.sql:55`) без дробной арифметики/NextPosition-аналога (`internal/features/board/position.go:1`); решение — design-стадии.
- Запрос "история заказов по единице оборудования" — в `internal/features/clients/repo.go:1` нет `OrdersByEquipment(equipmentID)`; JOIN через `task_equipment`/`subtasks.equipment_id`→`tasks`.
- Печать актов (D20) — SSR `/tasks/:id/print?kind=intake|handover` отсутствует полностью; `web/ssr/board/templates/pages/board.html:1`/`web/ssr/board/templates/pages/stock.html:1` — единственные существующие страницы фичи.
- Санитайзер HTML (`bluemonday`, `docs/plan.md:76`) — не найден в `go.mod:1`; потребуется добавить, если description_html начнёт приходить с клиента (сейчас пишет только seed-demo, `migrations/0005_board.up.sql:30`).
- Хук "в наличии/останется/не хватает" на товар услуги (D18) — прототип считает на клиенте (`prototype/index.html:1168-1174`); в бэкенде для `/stock` есть аналог `pending/forecast` (`internal/features/board/stock_dto.go:10-21`), для строки `subtask_products` в коде нет.
- SSR equipment-форма (`internal/features/clients/routes.go:33-47`) admin-only — путь для создания оборудования исполнителем ВНЕ API (в SSR-модалке заказа) отсутствует.

## Зависимости (что затронет изменение)
- `internal/features/board/routes.go:1` — новые группы `/subtasks`, `/subtask-products`, `/subtask-logs`, `/tasks/:id/equipment`, `/tasks/:id/payment`, печать.
- `internal/features/board/tasks_dto.go:1`, `internal/features/board/tasks_service.go:1`, `internal/features/board/tasks_repo.go:1` — расширение `Get`/`toTaskResponse` для полной карточки.
- `internal/features/board/stock_engine.go:50-53` — НЕ требует изменений: `reconcileDeltasSQL` уже читает `subtask_products JOIN subtasks WHERE task_id=$1`.
- `internal/features/clients/repo.go:1`/`internal/features/clients/dto.go:1` — переиспользуется для истории оборудования и `CreateEquipment` из карточки.
- `web/ssr/_shared/js/board.js:548-667` — полная переработка модалки заказа.
- `web/ssr/board/templates/pages/board.html:1` — модалка заказа как HTML-фрагмент.
- `e2e/fixtures/board.ts:1` — после появления реального API можно заменить SQL-вставки на вызовы API (решение design/planner).
- `pkg/errorspkg` — новые sentinel-ошибки фазы 6 по паттерну `errorspkg.New`.
- Возможно `go.mod:1` — `microcosm-cc/bluemonday`, если description_html правится клиентом в этой фазе.

## Граничные случаи из кода
- Гонка удаления `client_id`/`assignee_id` — `mapTaskFKError` по constraint (`internal/features/board/tasks_service.go:396-407`) — тот же паттерн нужен для `service_id`/`product_id`/`equipment_id` в новых repo-методах.
- Version-конфликт ДОЛЖЕН проверяться ДО paid-guard (`internal/features/board/tasks_service.go:150-157`, фикс MEDIUM-2 фазы 5) — новые guard'ы (D22) идут в том же порядке: LockTask/version → бизнес-guard'ы.
- D22 (финальная колонка блокирует возврат услуги из done и добавление новой) — в коде проверяется только на перенос заказа (`internal/features/board/tasks_service.go:188-196`) и смену колонки в is_final (`internal/features/board/columns_service.go:143-149`); проверки на уровне подзадачи — НЕТ, целиком фаза 6.
- **Lock Order для смены статуса подзадачи** — прямое замечание code review фазы 5 (`docs/features/phase-5-board/8-code-review/output.md:38`): будущий API смены статуса подзадачи должен брать блокировку в том же порядке columns→tasks, иначе окно гонки со сменой `is_final` колонки; записи в subtasks вне seed нет.
- `nilIfEmpty`/`samePtr` (`internal/features/board/tasks_service.go:362-376`) — nil-безопасное сравнение `*string`, пригодится для `equipment_id`/`assignee_id` подзадачи.
- Прототип-эталон (`prototype/index.html`): смена клиента заказа убирает "чужое" оборудование с тостом (`:1117-1123`); единственная привязанная единица оборудования подставляется в новую услугу автоматически (`:1229`, соответствует D15, `docs/plan.md:140`); убранная из заказа единица обнуляет `equipmentId` у ссылавшихся подзадач (`:1083`); подсказка по остатку считается на каждый товар каждой услуги через `taskDemand(t)` (`:636-639`, `:1168-1174`) — суммарный спрос заказа + уже списанное по ЭТОМУ заказу возвращается в "доступно".

## PostgreSQL 18 специфика
- `gen_random_uuid()` DEFAULT PK во всех таблицах — миграции 0003-0005.
- `trim_scale()` (`internal/features/board/position.go:24`) — контроль scale NUMERIC при делении пополам (ADR-001, потолок scale=16, `internal/features/board/position.go:9-16`).
- `NUMERIC(12,3)`/`NUMERIC(12,2)` — CHECK-constraints в БД (≥0/>0) — единственный источник правды для диапазона (Go дублирует только формат).
- `SELECT ... FOR UPDATE`/`FOR SHARE` — явные блокировки, ретрай на `40001`/`40P01` (`internal/features/board/tx.go:1`).
- `count(*) OVER ()` в `boardListSQL` (`internal/features/board/tasks_repo.go:235`) — total без второго запроса.
