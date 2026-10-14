# Phase 01: Foundation — миграция 0005, `pkg/numeric`, `errorspkg.WithDetail`, D26/SERVICE_IN_USE гарды

**Layer:** backend
**Commit**: `feat(phase-5): фундамент — миграция 0005, pkg/numeric, errorspkg.WithDetail, D26-гарды`

## Files to CREATE
- `migrations/0005_board.up.sql` — DDL: `columns`(+сид D32) → `tasks` → `subtasks` → `subtask_products` → `task_stock` → `task_equipment` → alter `stock_movements.task_id uuid→bigint`+FK (design-sql.md §Columns/Tasks/Subtasks/Subtask Products/Task Stock/Task Equipment/Stock Movements Migration)
- `migrations/0005_board.down.sql` — обратный порядок DROP (design-sql.md §Board Migration, CM-1 MVP)
- `pkg/numeric/amount.go` — `type Amount string` + `MarshalJSON`/`UnmarshalJSON`/`String()` (перенос из `internal/features/catalog/amount.go`, ADR-101)
- `pkg/numeric/amount_validate.go` — валидатор диапазона NUMERIC(12,3) (перенос из `internal/features/catalog/amount_validate.go`, ADR-101)
- `pkg/numeric/amount_test.go` — `TestAmount_MarshalJSON_EmptyToZero`, `TestAmount_UnmarshalJSON_RejectsNonNumber`, `TestAmount_UnmarshalJSON_Null`, `TestAmount_Validate_TooLarge`, `TestAmount_Validate_TooPrecise` (перенос существующих кейсов catalog)

## Files to MODIFY
- `internal/features/catalog/amount.go` — заменить телом на `type Amount = numeric.Amount` (alias, ADR-101); код-потребитель catalog не меняется
- `internal/features/catalog/amount_validate.go` — делегировать в `numeric` (валидатор вызывает `numeric`-реализацию) либо удалить, если алиас покрывает; итог — 0 дублирующей логики диапазона
- `internal/features/catalog/services_repo.go` — 23503 на `subtasks_service_id_fkey` → `ErrServiceInUse` (design-errors.md §Error Mapping)
- `internal/features/catalog/services_service.go` — добавить `ErrServiceInUse = errorspkg.New("SERVICE_IN_USE", http.StatusConflict, "Услуга используется в заказах — удалить нельзя, отправьте её в архив")` (design-errors.md §Sentinel Errors)
- `internal/features/catalog/products_service.go` — расширить текст `ErrProductInUse` до «Товар используется в услугах, заказах или журнале склада — удалить нельзя, отправьте его в архив» (код `PRODUCT_IN_USE`, design-errors.md #15)
- `internal/features/clients/repo.go` — `DeleteClient`: SQL EXISTS `tasks.client_id` перед DELETE (design-sql.md §Client Delete Guard Query); `DeleteEquipment`: SQL EXISTS `task_equipment.equipment_id` ∪ `subtasks.equipment_id` (design-sql.md §Equipment Delete Guard Query); 23503-страховка от гонки на тех же FK → тот же код
- `internal/features/clients/service.go` — добавить `ErrClientHasTasks = errorspkg.New("CLIENT_HAS_TASKS", http.StatusConflict, "У клиента есть заказы — удалить нельзя")`, `ErrEquipmentUsedInTasks = errorspkg.New("EQUIPMENT_USED_IN_TASKS", http.StatusConflict, "Оборудование было в заказе — удалить нельзя")`; SSR-удаление — as-is flash с `Message` (без изменений механизма)
- `pkg/errorspkg/errors.go` — добавить `Extra map[string]any` в `AppError` (json:"-"); метод `func (e *AppError) WithDetail(key string, v any) *AppError` — возвращает КОПИЮ `*AppError` с добавленным `Extra[key]=v` (sentinel не мутируется); `errDetails` — добавить произвольные ключи; `FiberErrorHandler` — `details = {"fields": Fields (если есть)} ∪ Extra`, `Details` опускается если оба пусты (design-errors.md §errorspkg Extension)
- `pkg/errorspkg/errors_test.go` — `TestAppError_WithDetail_ReturnsCopy_DoesNotMutateSentinel`, `TestFiberErrorHandler_MergesFieldsAndExtraIntoDetails`, `TestFiberErrorHandler_OmitsDetailsWhenEmpty`
- `internal/features/clients/clients_http_live_test.go` — добавить `TestClientsLiveSuite_DeleteClient_HasTasks_Returns409` (LV-056, вставка минимальной строки `tasks` напрямую SQL — `board`-сервиса ещё нет), `TestClientsLiveSuite_DeleteEquipment_UsedInTaskEquipment_Returns409` и `TestClientsLiveSuite_DeleteEquipment_UsedInSubtaskEquipment_Returns409` (LV-057, аналогично через прямой SQL insert в `task_equipment`/`subtasks`)
- `internal/features/catalog/services_http_live_test.go` — добавить `TestServicesLiveSuite_Delete_InUseBySubtask_Returns409` (LV-058, прямой SQL insert в `subtasks`)

## Run after
```bash
make build
make migrate-up   # 0005 на чистой БД
make migrate-down # обратный откат — без ошибок
make migrate-up
```

## Tests in this phase
- Unit: `TestAmount_MarshalJSON_EmptyToZero`, `TestAmount_UnmarshalJSON_RejectsNonNumber`, `TestAmount_Validate_TooLarge`, `TestAppError_WithDetail_ReturnsCopy_DoesNotMutateSentinel`, `TestFiberErrorHandler_MergesFieldsAndExtraIntoDetails`
- Live (TD-8, `testutil.LiveSuite`, дополнение существующих сьют): `TestClientsLiveSuite_DeleteClient_HasTasks_Returns409` (LV-056), `TestClientsLiveSuite_DeleteEquipment_UsedInTaskEquipment_Returns409` (LV-057), `TestServicesLiveSuite_Delete_InUseBySubtask_Returns409` (LV-058)

## Definition of Done
- [x] `make build` — OK
- [x] `make test` — unit OK (`pkg/numeric`, `pkg/errorspkg`)
- [x] `make test-integration-scoped` — OK (clients/catalog live-сьюты, скоуп §EE-5)
- [x] `make lint` — 0 ошибок
- [x] Тесты этой фазы зелёные до перехода (fix-before-next, TD-1)
- [x] `make migrate-up` → `make migrate-down` → `make migrate-up` на чистой БД без ошибок (0005 up/down симметричны, CM-1 MVP)
- [x] AC-008 покрыт (сид 5 колонок D32 в самой миграции); AC-017, AC-018 закрыты (`CLIENT_HAS_TASKS`/`EQUIPMENT_USED_IN_TASKS`, LV-056/057 зелёные)
- [x] ADR-005 (SERVICE_IN_USE/PRODUCT_IN_USE тексты), ADR-101 (алиас `catalog.Amount = numeric.Amount`, дублирующей логики диапазона не осталось), ADR-102 (`WithDetail` — копия, не мутирует sentinel) закрыты
- [x] Прямых импортов `internal/features/catalog` из `internal/features/clients`/`board`-кода НЕТ (D26-гарды — только SQL по своим таблицам, ADR-005)
- [x] Классификация 23503 — по `pgErr.ConstraintName`, не по тексту (design-errors.md §Error Mapping)
