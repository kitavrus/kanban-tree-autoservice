# Phase Bundle: phase-6-order-card / Phase 03 (frontend) — generated 2026-09-27 12:12

> Твой вход — ЭТОТ файл. Upstream-доки (research/prd/design-*) напрямую НЕ читать —
> нужные слайсы дизайна ниже. Код фичи и её тесты — читать/писать можно (твоя фаза).

## Фаза плана (6-plan/code-plan-phase-03-subtasks-products-logs.md)
# Phase 03: Услуги, товары, журнал работ, фикстуры E2E→API

**Layer:** backend
**Commit**: `feat(phase-6): subtasks/subtask_products CRUD + Reconcile, журнал работ, фикстуры API`

## Files to CREATE
- `internal/features/board/subtasks_repo.go` — `SubtaskRepo`: `TaskIDOf`, `Lock`, `Insert` (+копия комплекта, design-sql.md §Insert Subtask Query), `ServiceState`, `Update` (§Update Subtask Query, `equipment_id` проверка из `task_equipment`), `Delete` (§Delete Subtask Query, CASCADE products+logs)
- `internal/features/board/subtasks_service.go` — методы `CardService`: `CreateSubtask` (409 `SUBTASK_FINAL_COLUMN` если `column.is_final`), `UpdateSubtask` (409 `SUBTASK_FINAL_COLUMN` при `done`→не-`done` в финальной; без Reconcile, но под теми же блокировками ADR-100), `DeleteSubtask` (Reconcile после каскадного удаления товаров)
- `internal/features/board/subtasks_validate.go` — `SubtaskCreateRequest`/`SubtaskUpdateRequest` `normalize()/validate()` (design-errors.md §Validation Fields: `status` enum, `assignee_id` активный при смене — SD-007, `due_date` YYYY-MM-DD, `price` ≥0 Scale 10/2, `equipment_id` из `task_equipment` заказа)
- `internal/features/board/subtask_products_repo.go` — `SubtaskProductRepo`: `ParentOf`, `Lock`, `ProductState`, `Insert` (design-sql.md §Subtask Product Queries), `Update` (LWW), `Delete`
- `internal/features/board/subtask_products_service.go` — методы `CardService`: `CreateSubtaskProduct`, `UpdateSubtaskProduct`, `DeleteSubtaskProduct` (каждая — `engine.Reconcile(tx, taskID, col.ConsumesStock, a.ID)` после записи)
- `internal/features/board/subtask_products_validate.go` — `SubtaskProductCreateRequest`/`UpdateRequest` `normalize()/validate()` (`qty` >0 Scale 9/3, `price` ≥0 или null Scale 10/2)
- `internal/features/board/subtask_logs_repo.go` — `SubtaskLogRepo{pool}`: `Insert` (23503 → `errNotFound`), `Owner`, `Update`, `Delete` (design-sql.md §Subtask Log Queries)
- `internal/features/board/subtask_logs_service.go` — методы `CardService`: `CreateLog`, `UpdateLog`, `DeleteLog` (403 `SUBTASK_LOG_FORBIDDEN` — автор ≠ актор и актор не admin, D27; без блокировки заказа)
- `internal/features/board/subtask_logs_validate.go` — `SubtaskLogRequest.normalize()/validate()` (`text` trim 1..4000)

## Files to MODIFY
- `internal/features/board/card_service.go` — довключить поля `subtasks`/`products`/`logs`-репозитории в `CardService`; `lockRowsScope`/`cardMutation` используются всеми новыми методами (design-go-layers.md §Service Interface: «Поток мутации товара/услуги»)
- `internal/features/board/card_handler.go` — добавить методы `CreateSubtask`, `UpdateSubtask`, `DeleteSubtask`, `CreateSubtaskProduct`, `UpdateSubtaskProduct`, `DeleteSubtaskProduct`, `CreateLog`, `UpdateLog`, `DeleteLog`
- `internal/features/board/routes.go` — группы `subtasks`, `subtask-products`, `subtask-logs` (design-go-layers.md §Routes: `t.Post("/:id/subtasks", ...)`, `sub.Put/Delete("/:id", ...)`, `sub.Post("/:id/products"|"/:id/logs", ...)`, `sp.Put/Delete("/:id", ...)`, `sl.Put/Delete("/:id", ...)`)
- `internal/features/board/card_dto.go` — довключить `normalize()/validate()` методы на Row Requests, объявленных в фазе 02 (типы уже есть, поведение — здесь), если не покрыто `*_validate.go` файлами
- `internal/features/board/card_validate_test.go` — MODIFY: добавить `SubtaskUpdateRequest`, `SubtaskProductCreateRequest`/`UpdateRequest`, `SubtaskLogRequest` кейсы (полный UT-002: границы qty/price Scale, text trim/4000, status enum, nil vs value)
- `internal/features/board/card_integration_test.go` — MODIFY: расширить `CardSuite` до полного покрытия: `TestCardSuite_Reconcile_ConsumesStock` (LV-120…123), `TestCardSuite_StockInvariant` (IT-001), `TestCardSuite_Concurrency_UpdateSubtaskProduct` (IT-010), `TestCardSuite_Concurrency_MoveVsUpdate` (IT-011), `TestCardSuite_Concurrency_DeleteVsUpdate` (IT-012), `TestCardSuite_Concurrency_TaskMovedRetry` (IT-013)
- `internal/features/board/order_card_http_live_test.go` — MODIFY: добавить `TestOrderCardLiveSuite_Subtask_CreateWithKit` (LV-103), `TestOrderCardLiveSuite_Subtask_UpdateFields` (LV-104), `TestOrderCardLiveSuite_SubtaskProduct_CRUD` (LV-105), `TestOrderCardLiveSuite_Totals` (LV-106), `TestOrderCardLiveSuite_Logs_CRUD` (LV-107), `TestOrderCardLiveSuite_Version_UnaffectedByRowMutations` (LV-130), `TestOrderCardLiveSuite_Version_ConflictOnHeader` (LV-131), `TestOrderCardLiveSuite_FinalColumn_BlocksCreate` (LV-140), `TestOrderCardLiveSuite_FinalColumn_BlocksUndone` (LV-141), `TestOrderCardLiveSuite_Subtask_EquipmentNotInOrder400` (LV-145), `TestOrderCardLiveSuite_Subtask_InactiveAssignee400` (LV-146), `TestOrderCardLiveSuite_Subtask_ArchivedServiceOrProduct400` (LV-147), `TestOrderCardLiveSuite_DeleteSubtask_CascadesLogsProducts` (LV-148), `TestOrderCardLiveSuite_Access_ExecutorRowEndpoints` (LV-160), `TestOrderCardLiveSuite_Logs_ForbiddenForOthers` (LV-163), `TestOrderCardLiveSuite_NoSession401` (LV-164), `TestOrderCardLiveSuite_InvalidPathIDs404` (LV-170, дополнить subtasks/products/logs), `TestOrderCardLiveSuite_InvalidInput400` (LV-171), `TestOrderCardLiveSuite_GarbageBody400` (LV-172), `TestOrderCardLiveSuite_IdempotentDelete404` (LV-173), `TestOrderCardLiveSuite_LogOnDeletedSubtask404` (LV-174)
- `internal/features/board/board_http_live_test.go` — MODIFY: добавить `TestBoardLiveSuite_Move_ToFinal_WithAPISubtask` (LV-150, регресс фазы 5: услуга создана через новое API, а не `pg`)
- `internal/features/board/board_fuzz_test.go` — MODIFY: добавить `FuzzSubtaskUpdateBody` (FZ-001, `PUT /subtasks/:id`), `FuzzSubtaskProductBody` (FZ-002, `POST/PUT` товара), `FuzzSubtaskLogAndEquipmentBody` (FZ-003, `POST /subtasks/:id/logs`, `POST /tasks/:id/equipment`) — seeds design-tests.md §Fuzzing, `app.Test` допустим (§9.1)
- `e2e/fixtures/board.ts` — MODIFY: заменить `pg`-вставки на API-хелперы (ADR-101): `addSubtask(taskId, {serviceId, status?, dueDate?, products?})` (`POST /tasks/:id/subtasks` → при `products` DELETE скопированных + `POST /subtasks/:id/products`; при `status`/`dueDate` — `PUT /subtasks/:id`), `attachEquipment(taskId, equipmentId)` (`POST /tasks/:id/equipment`); удалить импорт `pg` из `e2e/package.json`, если других использований нет
- `e2e/fixtures/board-types.ts` — MODIFY: добавить типы `OrderCard`, `CardSubtask`, `CardSubtaskProduct`, `CardEquipment`, `CardSubtaskLog` (зеркало DTO фазы 02/03)
- `e2e/tests/board.spec.ts`, `e2e/tests/board-guards.spec.ts`, `e2e/tests/stock.spec.ts` — MODIFY при необходимости под новые сигнатуры фикстур (регресс-гейт ADR-101); без изменения сценариев, если фикстуры совместимы по интерфейсу

## Run after
```bash
make build
make test
make test-integration-scoped
```

## Tests in this phase
- Unit: `card_validate_test.go` полный набор UT-002 (qty/price Scale, text trim, status enum)
- Fuzz (TD-7, класс 9.1): `FuzzSubtaskUpdateBody`, `FuzzSubtaskProductBody`, `FuzzSubtaskLogAndEquipmentBody`
- Integration (`CardSuite`): IT-001, IT-010, IT-011, IT-012, IT-013
- Live (`OrderCardLiveSuite`): LV-103…107, LV-130/131, LV-140/141/145…148, LV-160/163/164, LV-170…174
- Live regression (`BoardLiveSuite`): LV-150
- E2E regression: `board.spec.ts board-guards.spec.ts stock.spec.ts --project=chromium --headed --workers=1` зелёные после перевода фикстур на API

## Definition of Done
- [ ] `make build` — OK
- [ ] `make test` — unit OK
- [ ] `make test-integration-scoped` — OK (скоуп-дефолт §EE-5, BASE=9580596)
- [ ] `make lint` — 0 ошибок
- [ ] Тесты этой фазы зелёные до перехода (TD-1)
- [ ] `order_card_http_live_test.go` — все перечисленные LV-* зелёные (TD-8); `board_http_live_test.go` LV-150 зелёный
- [ ] `go test -run=^$ -fuzz=FuzzSubtaskUpdateBody -fuzztime=10s ./internal/features/board/` и аналогично для `FuzzSubtaskProductBody`, `FuzzSubtaskLogAndEquipmentBody` — зелёные, без паник, только 400/404/409/2xx (TD-7, класс 9.1)
- [ ] Инвариант склада (IT-001: `stock_after − stock_before = Σ qty_delta`, `task_stock.qty = Σ subtask_products.qty` для consumes) — выполнен после LV-120…123 и IT-010…013
- [ ] AC-007, AC-008, AC-009 (подсказка остатка использует `CardRepo.Stock` фазы 02 + реальные товары этой фазы), AC-010, AC-011, AC-012, AC-013 (полный — с новыми услугами/товарами), AC-014 (конкурентность, ADR-100/errTaskMovedRetry) — покрыты
- [ ] AC-017 (права исполнителя на row-эндпоинтах, `SUBTASK_LOG_FORBIDDEN`) — LV-160/163/164 зелёные
- [ ] `e2e/fixtures/board.ts` не импортирует `pg` (ADR-101); `board.spec.ts`/`board-guards.spec.ts`/`stock.spec.ts` зелёные на новых фикстурах (реальный прогон Playwright, не по тексту отчёта — `frontend/test-results/.last-run.json` эквивалент проекта, `e2e/test-results/`)
- [ ] `Reconcile` вызывается ПОСЛЕ commit-эквивалентной точки внутри той же `withTx` для create/delete услуги и create/update/delete товара; `UpdateSubtask` — без Reconcile, но под `lockRowsScope` (helper-binding: НЕ дублировать блокировку вручную, использовать `lockRowsScope` фазы 02)
- [ ] testdata/фикстуры (`e2e/fixtures/board.ts`, seed использующийся в LiveSuite) приведены к новой схеме (`position`, `subtask_logs`) — TD-1/TD-6.2


## Слайс: 3-design/design-frontend-architecture.md [full]
MISSING

## Слайс: 3-design/design-go-layers.md [sections]
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

## Слайс: 3-design/output.md [sections]
## Контракт
Все JSON — `/api/v1`, `RequireAPI` (+CSRF на мутациях), без admin-гарда (D19). «card» = `OrderCardMutationResponse {card, warnings}`. DTO — `design-go-layers.md`; коды — `design-errors.md`.

| Метод | Путь | Запрос | Ответ | Driver |
|---|---|---|---|---|
| GET | `/api/v1/tasks/:id/card` | — | 200 `OrderCardResponse` | AC-001 |
| POST | `/api/v1/tasks` (mod) | `TaskCreateRequest` + `description` | 201 `TaskMutationResponse` (as-is) | AC-001 |
| PUT | `/api/v1/tasks/:id` (mod) | `TaskUpdateRequest` + `description` | 200 `TaskMutationResponse`; 409 `TASK_VERSION_CONFLICT`/`TASK_EQUIPMENT_IN_USE`; 403 `PAID_ADMIN_ONLY` | AC-002, AC-003, AC-004 |
| GET | `/api/v1/tasks` (mod) | as-is | элементы + `total_amount`, `last_log` (то же `total_amount` — в `card.task`, другого имени итога нет) | AC-012, AC-013 |
| POST | `/api/v1/tasks/:id/subtasks` | `SubtaskCreateRequest` | 201 card; 409 `SUBTASK_FINAL_COLUMN` | AC-007, AC-008, AC-011 |
| PUT | `/api/v1/subtasks/:id` | `SubtaskUpdateRequest` | 200 card; 409 `SUBTASK_FINAL_COLUMN` | AC-007, AC-011 |
| DELETE | `/api/v1/subtasks/:id` | — | 200 card | AC-007, AC-010 |
| POST | `/api/v1/subtasks/:id/products` | `SubtaskProductCreateRequest` | 201 card | AC-008, AC-010 |
| PUT | `/api/v1/subtask-products/:id` | `SubtaskProductUpdateRequest` | 200 card | AC-008, AC-010, AC-014 |
| DELETE | `/api/v1/subtask-products/:id` | — | 200 card | AC-008, AC-010 |
| POST | `/api/v1/tasks/:id/equipment` | `TaskEquipmentCreateRequest` | 201 card | AC-004, AC-005 |
| PUT | `/api/v1/tasks/:id/equipment/:equipment_id` | `TaskEquipmentUpdateRequest` | 200 card | AC-004 |
| DELETE | `/api/v1/tasks/:id/equipment/:equipment_id` | — | 200 card; 409 `TASK_EQUIPMENT_IN_USE` | AC-004 |
| POST | `/api/v1/subtasks/:id/logs` | `SubtaskLogRequest` | 201 card | AC-012 |
| PUT | `/api/v1/subtask-logs/:id` | `SubtaskLogRequest` | 200 card; 403 `SUBTASK_LOG_FORBIDDEN` | AC-012 |
| DELETE | `/api/v1/subtask-logs/:id` | — | 200 card; 403 `SUBTASK_LOG_FORBIDDEN` | AC-012 |
| POST | `/api/v1/clients/:id/equipment` (as-is) | `EquipmentRequest` | 201 `EquipmentResponse` | AC-005 |
| GET (SSR) | `/tasks/:id/print/intake`, `/tasks/:id/print/handover` | — | 200 HTML (layout `print`), `RequireSSR`, no-store | AC-015, AC-016 |
| GET (SSR) | `/?task=N` (deep-link, as-is роут `/`) | — | каркас доски, карточку открывает JS | AC-001 |

§API-1: сегменты kebab (`subtask-products`, `subtask-logs`), JSON snake_case, списков нового вида нет (журнал и строки отдаются в составе карточки). NUMERIC — `pkg/numeric.Amount` (число на wire), nullable-цены — `*numeric.Amount` → `null`.

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

## Слайс: 3-design/design-e2e-strategy.md [head]
MISSING

## Слайс: 1-research/output.md [sections]
## Frontend
Не применимо в терминах Vue — SSR (`Go html/template` + vanilla JS, `docs/plan.md:5`), `frontend/` отсутствует. Фактический "фронт" фичи ниже.

- `web/ssr/board/templates/pages/board.html:1` (133 строки) и `web/ssr/board/templates/pages/stock.html:1` (78 строк) — HTML-шаблоны доски/остатков.
- `web/ssr/_shared/js/board.js:548-667` (815 строк в файле): `openTaskForm(task, defaultColumnId)`(:548-569) — ТОЛЬКО простая форма (title/client/column/assignee/due/paid), без описания/оборудования/услуг/товаров/журнала/итогов; `taskFormPayload()`(:587-602) шлёт `{title,client_id,column_id,assignee_id,due_date,is_paid}` — is_paid ВСЕГДА отправляется даже при disabled-чекбоксе у executor (:590-593, важно сохранить при расширении — иначе тихий сброс оплаты); `bindTaskForm()`(:610-667) — submit→PUT/POST, catch различает `TASK_VERSION_CONFLICT`(:638-643) от `err.fields`(:644-646) от общей(:648); `finishOpenTaskForm`(:571-585) — `data-testid="task-delete"` снимается АТРИБУТОМ, не только `d-none` (:576-579) — конвенция для новых admin-only кнопок.
- `web/ssr/_shared/js/api.js:1` — `KT.api` fetch-обёртка с CSRF, используется всеми хендлерами.
- Общие компоненты: `web/ssr/_shared/js/combobox.js:1`, `web/ssr/_shared/js/ui-dialog.js:1`(`KT.uiConfirm/uiAlert`), `web/ssr/_shared/js/nav-badge.js:1`, `web/ssr/_shared/js/theme.js:1`.
- `e2e/pages/BoardPage.ts:1`, `e2e/pages/EquipmentPage.ts:1` и др. в `e2e/pages/` — POM; `e2e/fixtures/board.ts:1`, `e2e/fixtures/board-types.ts:1` — типы фикстур.

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
