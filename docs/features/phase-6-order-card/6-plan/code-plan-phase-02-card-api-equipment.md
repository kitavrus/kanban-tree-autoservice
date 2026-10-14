# Phase 02: Card API (GET), описание, итоги/last_log, оборудование заказа

**Layer:** backend
**Commit**: `feat(phase-6): GET /tasks/:id/card, описание, итоги/last_log, оборудование заказа`

## Files to CREATE
- `internal/features/board/card_dto.go` — `OrderCardResponse`, `CardColumn`, `CardEquipment`, `EquipmentHistory`, `CardSubtask`, `CardSubtaskProduct`, `CardSubtaskLog`, `CardStock`, `OrderCardMutationResponse` (design-go-layers.md §DTO OrderCard) + ВСЕ Row Requests (`SubtaskCreateRequest`, `SubtaskUpdateRequest`, `SubtaskProductCreateRequest`, `SubtaskProductUpdateRequest`, `TaskEquipmentCreateRequest`, `TaskEquipmentUpdateRequest`, `SubtaskLogRequest` — design-go-layers.md §DTO Row Requests, поля объявляются одним файлом по Package Layout, реализация normalize/validate — по мере готовности сервисов в фазах 02/03)
- `internal/features/board/card_repo.go` — `CardRepo`: `Column`, `Equipment` (+история json_agg, AC-006), `Subtasks`, `Products`, `Logs`, `Stock`, `PrintHeader` (design-sql.md §Card Queries, §Print Header Query — 7 запросов без N+1)
- `internal/features/board/card_service.go` — `CardService{pool, tasks, card, subtasks, products, logs, equipment, engine}`; `Card(ctx, a, taskID)`; `lockRowsScope` (design-sql.md §Lock Order: `TaskColumnID` → `LockTargetColumn` FOR SHARE → `LockTask` FOR UPDATE → несовпадение колонки → `errTaskMovedRetry`); `cardMutation` (re-fetch-after-write, `Card()` после commit)
- `internal/features/board/card_handler.go` — `CardHandler{svc}`: метод `Card` (GET, 200) + `AddEquipment`/`UpdateEquipment`/`RemoveEquipment` (POST 201/PUT 200/DELETE 200) — прочие row-методы добавляются в фазе 03
- `internal/features/board/task_equipment_repo.go` — `TaskEquipmentRepo`: `EquipmentClient`, `Insert` (идемпотентно, ON CONFLICT DO NOTHING), `Update`, `UsedBy`, `Delete`, `Attached`, `ForeignInUse`, `DeleteForeign` (design-sql.md §Task Equipment Queries, §Foreign Equipment Query)
- `internal/features/board/task_equipment_service.go` — методы `CardService`: `AddEquipment`, `UpdateEquipment`, `RemoveEquipment` (`LockTask`, без блокировки колонки; 409 `TASK_EQUIPMENT_IN_USE` + `details.subtasks` через `errorspkg.WithDetail`)
- `internal/features/board/task_equipment_validate.go` — `TaskEquipmentCreateRequest`/`UpdateRequest` `normalize()/validate()` (design-errors.md §Validation Fields: `equipment_id` принадлежность клиенту, `completeness`/`condition` ≤1000)
- `internal/features/board/description.go` — `descriptionToHTML(text string) string` (ADR-006: `html.EscapeString`, абзацы по пустой строке → `<p>…</p>`, `\n` → `<br>`)
- `internal/features/board/description_test.go` — `TestDescriptionToHTML_Escaping`, `TestDescriptionToHTML_Paragraphs`, `TestDescriptionToHTML_Empty` (UT-001)
- `internal/features/board/card_validate_test.go` — `TestTaskEquipmentCreateRequestValidate_MissingEquipmentID`, `TestTaskEquipmentCreateRequestValidate_TooLongFields` (часть UT-002; остальное — фаза 03)
- `internal/features/board/card_integration_test.go` — `CardSuite` (`BaseSuite`): `TestCardSuite_Card_ReturnsFullTree`, `TestCardSuite_Card_EmptyArraysNotNull` (часть покрытия LV-101/106 на уровне integration; полное покрытие мутаций — фаза 03)
- `internal/features/board/order_card_http_live_test.go` — `OrderCardLiveSuite` (`testutil.LiveSuite`, TD-8): `TestOrderCardLiveSuite_GetCard_FullTree` (LV-101), `TestOrderCardLiveSuite_Description_EscapeAndUpdate` (LV-102), `TestOrderCardLiveSuite_DeepLink_SSR` (LV-111), `TestOrderCardLiveSuite_Equipment_AddUpdateRemove` (LV-108), `TestOrderCardLiveSuite_Equipment_NewFromCard` (LV-109), `TestOrderCardLiveSuite_Equipment_History` (LV-110), `TestOrderCardLiveSuite_Equipment_InUseConflict` (LV-142), `TestOrderCardLiveSuite_Equipment_ClientChange` (LV-143), `TestOrderCardLiveSuite_Equipment_ForeignClient400` (LV-144), `TestOrderCardLiveSuite_InvalidPathIDs404` (LV-170, часть — equipment/card пути)

## Files to MODIFY
- `internal/features/board/tasks_dto.go` — `TaskCard`: `TotalAmount numeric.Amount`, `LastLog *TaskLastLog` (+тип `TaskLastLog`); `TaskResponse`: `DescriptionHTML string`; `TaskCreateRequest`/`TaskUpdateRequest`: `Description *string` (design-go-layers.md §DTO Task Changes)
- `internal/features/board/tasks_repo.go` — `Get`/`List` (`boardListSQL`): + Totals Expression (design-sql.md §Totals Expression), Last Log Expression (LATERAL, §Last Log Expression), `description_html`; новый метод `TaskColumnID(ctx, tx, id) (string, error)` (design-sql.md §Task Column Id Query); `LockTask` — дополнительно читает `description_html`
- `internal/features/board/tasks_service.go` — `Update`: после paid-гарда — если `client_id` сменился, `ForeignInUse`/`DeleteForeign` (409 `TASK_EQUIPMENT_IN_USE` до `repo.Update`, design-sequence-diagrams.md §4); `description`: `nil` → не менять, иначе `descriptionToHTML`
- `internal/features/board/tasks_validate.go` — `description` ≤10000 символов (design-errors.md §Validation Fields)
- `internal/features/board/tasks_validate_test.go` — `TestTaskCreateRequestValidate_DescriptionTooLong`, `TestTaskUpdateRequestValidate_DescriptionNilKeepsValue`
- `internal/features/board/routes.go` — `t.Get("/:id/card", h.Card.Card)`, `t.Post("/:id/equipment", ...)`, `t.Put("/:id/equipment/:equipment_id", ...)`, `t.Delete("/:id/equipment/:equipment_id", ...)` (design-go-layers.md §Routes); `Handlers` — добавить поле `Card *CardHandler`

## Run after
```bash
make build
make test
make test-integration-scoped
```

## Tests in this phase
- Unit: `TestDescriptionToHTML_Escaping`, `TestDescriptionToHTML_Paragraphs`, `TestTaskEquipmentCreateRequestValidate_MissingEquipmentID`, `TestTaskCreateRequestValidate_DescriptionTooLong` (UT-001, часть UT-002)
- Integration: `TestCardSuite_Card_ReturnsFullTree`, `TestCardSuite_Card_EmptyArraysNotNull`
- Live (`OrderCardLiveSuite`, TD-8): LV-101, LV-102, LV-108, LV-109, LV-110, LV-111, LV-142, LV-143, LV-144, часть LV-170

## Definition of Done
- [x] `make build` — OK
- [x] `make test` — unit OK
- [x] `make test-integration-scoped` — OK (BASE=9580596 → FULL-триггер по миграции 0006, полный `./...`, все пакеты зелёные)
- [x] `make lint` — 0 ошибок
- [x] Тесты этой фазы зелёные до перехода (TD-1)
- [x] `order_card_http_live_test.go` (`testutil.LiveSuite`, реальный бинарь+TCP, реальный login) — все перечисленные LV-* зелёные (TD-8)
- [x] AC-001 (карточка одним GET, пустые массивы `[]` не `null`), AC-002 (description escape/keep-on-nil), AC-004 (оборудование CRUD + `TASK_EQUIPMENT_IN_USE` + очистка при смене клиента), AC-005 (`POST /clients/:id/equipment` as-is → `POST /tasks/:id/equipment`, два запроса ADR-004), AC-006 (история других заказов) — покрыты
- [x] AC-013 частично: `task.total_amount` пересчитывается по существующим (фазы 5) `subtasks`/`subtask_products` без товаров фазы 6 — полный сценарий с новыми услугами/товарами добавляется в DoD фазы 03
- [x] `ErrTaskEquipmentInUse.WithDetail("subtasks", list)` — `pkg/errorspkg.WithDetail` (фаза 5), НЕ ad-hoc тип ответа (helper-binding)
- [x] `cardMutation`/`lockRowsScope` — реализованы в `card_service.go` (lockRowsScope покрыт прямым вызовом в `CardSuite` — `TestCardSuite_LockRowsScope_ReturnsTaskAndColumn`/`_TaskMovedConcurrently`); использование row-мутациями услуг/товаров/журнала — фаза 03 (оборудование в этой фазе использует `LockTask` без блокировки колонки, design-sql.md §Lock Order)
- [x] testdata/фикстуры интеграционных тестов, затронутые `description_html`/`total_amount`/`last_log`, приведены к новой схеме (TD-1/TD-6.2) — новые колонки аддитивны (DEFAULT), существующие фикстуры фазы 5 (`BoardLiveSuite`, `StockEngineSuite`) остались зелёными без изменений
