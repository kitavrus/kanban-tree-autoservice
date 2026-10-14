# Review Bundle: phase-6-order-card — READY
**Base:** 9580596 | **HEAD:** 793fd75 | **Generated:** 2026-09-27 15:59 | diff: 127 files changed, 24828 insertions(+), 453 deletions(-)

> Линзы ревью: читай ЭТОТ бандл + patch своей линзы. Полное чтение research/draft-spec/PRD
> и перегон чекеров ЗАПРЕЩЕНЫ (вердикты ниже; детали — checks/<name>.txt; точечный grep по DS-NNN разрешён).

## Чекеры (детерминированные — НЕ перегонять)
| Чекер | Вердикт | exit | Нарушения (первые строки) |
|---|---|---|---|
| go_naming | ✅ | 0 | — |
| api_conventions | ✅ | 0 | — |
| contract | ⚠️ SKIP-RESOLVE (Go-код в diff есть — НЕ зелёно, §GO-1: перегенерируй bundle с --paths/--go) | 0 | — |
| date_bound | ✅ | 0 | — |
| fuzz_verdict | ✅ | 0 | — |
| live_suite | ✅ | 0 | — |
| fixture_sync | ✅ | 0 | — |
| e2e_mode | ✅ | 0 | — |
| rule_usage | ✅ | 0 | — |

## Diff по линзам
| Линза | Файлов | Patch |
|---|---|---|
| backend | 42 | 8-code-review/diff-backend.patch |
| e2e | 14 | 8-code-review/diff-e2e.patch |
| migrations | 2 | 8-code-review/diff-migrations.patch |
| other | 9 | 8-code-review/diff-other.patch |
| docs | 60 | — (артефакты, не ревьюятся построчно) |

### backend
- `M` internal/config/config.go
- `M` internal/features/board/board_fuzz_test.go
- `M` internal/features/board/board_http_live_test.go
- `A` internal/features/board/card_dto.go
- `A` internal/features/board/card_handler.go
- `A` internal/features/board/card_integration_test.go
- `A` internal/features/board/card_repo.go
- `A` internal/features/board/card_service.go
- `A` internal/features/board/card_validate_test.go
- `A` internal/features/board/description.go
- `A` internal/features/board/description_test.go
- `M` internal/features/board/enums.go
- `M` internal/features/board/enums_test.go
- `M` internal/features/board/errors.go
- `A` internal/features/board/order_card_http_live_test.go
- `A` internal/features/board/print_http_live_test.go
- `A` internal/features/board/print_ssr.go
- `M` internal/features/board/routes.go
- `M` internal/features/board/ssr.go
- `M` internal/features/board/stock_engine_integration_test.go
- `A` internal/features/board/subtask_logs_repo.go
- `A` internal/features/board/subtask_logs_service.go
- `A` internal/features/board/subtask_logs_validate.go
- `A` internal/features/board/subtask_products_position_integration_test.go
- `A` internal/features/board/subtask_products_repo.go
- `A` internal/features/board/subtask_products_service.go
- `A` internal/features/board/subtask_products_validate.go
- `A` internal/features/board/subtasks_repo.go
- `A` internal/features/board/subtasks_service.go
- `A` internal/features/board/subtasks_validate.go
- `A` internal/features/board/task_equipment_repo.go
- `A` internal/features/board/task_equipment_service.go
- `A` internal/features/board/task_equipment_validate.go
- `M` internal/features/board/tasks_dto.go
- `M` internal/features/board/tasks_repo.go
- `M` internal/features/board/tasks_service.go
- `M` internal/features/board/tasks_validate.go
- `M` internal/features/board/tasks_validate_test.go
- `M` internal/features/board/tx.go
- `M` internal/features/catalog/categories_service.go
- `M` internal/server/router.go
- `M` internal/ssr/render_test.go

### e2e
- `M` e2e/fixtures/auth.ts
- `M` e2e/fixtures/board-types.ts
- `M` e2e/fixtures/board.ts
- `M` e2e/fixtures/test.ts
- `M` e2e/package-lock.json
- `M` e2e/package.json
- `A` e2e/pages/OrderCardPage.ts
- `M` e2e/tests/board-guards.spec.ts
- `M` e2e/tests/board.spec.ts
- `M` e2e/tests/catalog.spec.ts
- `M` e2e/tests/equipment-types.spec.ts
- `A` e2e/tests/order-card.spec.ts
- `A` e2e/tests/order-print.spec.ts
- `M` e2e/tests/users.spec.ts

### migrations
- `A` migrations/0006_subtask_logs.down.sql
- `A` migrations/0006_subtask_logs.up.sql

### other
- `M` .env.example
- `M` web/ssr/_shared/app.css
- `M` web/ssr/_shared/js/board.js
- `A` web/ssr/_shared/js/task-card.js
- `A` web/ssr/_shared/print.css
- `A` web/ssr/board/templates/layouts/print.html
- `M` web/ssr/board/templates/pages/board.html
- `A` web/ssr/board/templates/pages/print-handover.html
- `A` web/ssr/board/templates/pages/print-intake.html

### docs
- `A` docs/features/phase-6-order-card/0-setup/context-violations.jsonl
- `A` docs/features/phase-6-order-card/0-setup/diagrams-prefs.md
- `A` docs/features/phase-6-order-card/0-setup/e2e-scope-prefs.md
- `A` docs/features/phase-6-order-card/0-setup/integration-scope-prefs.md
- `A` docs/features/phase-6-order-card/0-setup/passport.md
- `A` docs/features/phase-6-order-card/0-setup/pipeline-state.md
- `A` docs/features/phase-6-order-card/0-setup/timeline.jsonl
- `A` docs/features/phase-6-order-card/0-setup/ui-scope-prefs.md
- `A` docs/features/phase-6-order-card/0-setup/watch-gate-prefs.md
- `A` docs/features/phase-6-order-card/1-research/input-bundle.md
- `A` docs/features/phase-6-order-card/1-research/output.md
- `A` docs/features/phase-6-order-card/2-spec-interview/input-bundle.md
- `A` docs/features/phase-6-order-card/2-spec-interview/output.md
- `A` docs/features/phase-6-order-card/2-spec-interview/prd.md
- `A` docs/features/phase-6-order-card/2-spec-interview/spec-decisions.md
- `A` docs/features/phase-6-order-card/3-design/design-errors.md
- `A` docs/features/phase-6-order-card/3-design/design-go-layers.md
- `A` docs/features/phase-6-order-card/3-design/design-sequence-diagrams.md
- `A` docs/features/phase-6-order-card/3-design/design-sql.md
- `A` docs/features/phase-6-order-card/3-design/design-tests.md
- `A` docs/features/phase-6-order-card/3-design/input-bundle.md
- `A` docs/features/phase-6-order-card/3-design/output.md
- `A` docs/features/phase-6-order-card/4-swimlane/design-swimlane.html
- `A` docs/features/phase-6-order-card/4-swimlane/input-bundle.md
- `A` docs/features/phase-6-order-card/5-design-review/input-bundle.md
- `A` docs/features/phase-6-order-card/5-design-review/mobile.md
- `A` docs/features/phase-6-order-card/5-design-review/output.md
- `A` docs/features/phase-6-order-card/6-plan/code-plan-phase-01-migration-errors-lock.md
- `A` docs/features/phase-6-order-card/6-plan/code-plan-phase-02-card-api-equipment.md
- `A` docs/features/phase-6-order-card/6-plan/code-plan-phase-03-subtasks-products-logs.md
- `A` docs/features/phase-6-order-card/6-plan/code-plan-phase-04-print-ssr.md
- `A` docs/features/phase-6-order-card/6-plan/code-plan-phase-05-ui-task-card.md
- `A` docs/features/phase-6-order-card/6-plan/code-plan-phase-06-board-integration-polish.md
- `A` docs/features/phase-6-order-card/6-plan/code-plan-status.md
- `A` docs/features/phase-6-order-card/6-plan/code-plan.md
- `A` docs/features/phase-6-order-card/6-plan/input-bundle.md
- `A` docs/features/phase-6-order-card/6-plan/phase-bundles/phase-01-input.md
- `A` docs/features/phase-6-order-card/6-plan/phase-bundles/phase-02-input.md
- `A` docs/features/phase-6-order-card/6-plan/phase-bundles/phase-03-input.md
- `A` docs/features/phase-6-order-card/6-plan/phase-bundles/phase-04-input.md
- `A` docs/features/phase-6-order-card/6-plan/phase-bundles/phase-05-input.md
- `A` docs/features/phase-6-order-card/6-plan/phase-bundles/phase-06-input.md
- `A` docs/features/phase-6-order-card/8-code-review/checks/api_conventions.txt
- `A` docs/features/phase-6-order-card/8-code-review/checks/contract.txt
- `A` docs/features/phase-6-order-card/8-code-review/checks/date_bound.txt
- `A` docs/features/phase-6-order-card/8-code-review/checks/e2e_mode.txt
- `A` docs/features/phase-6-order-card/8-code-review/checks/fixture_sync.txt
- `A` docs/features/phase-6-order-card/8-code-review/checks/fuzz_verdict.txt
- `A` docs/features/phase-6-order-card/8-code-review/checks/go_naming.txt
- `A` docs/features/phase-6-order-card/8-code-review/checks/live_suite.txt
- `A` docs/features/phase-6-order-card/8-code-review/checks/rule_usage.txt
- `A` docs/features/phase-6-order-card/8-code-review/diff-backend.patch
- `A` docs/features/phase-6-order-card/8-code-review/diff-e2e.patch
- `A` docs/features/phase-6-order-card/8-code-review/diff-migrations.patch
- `A` docs/features/phase-6-order-card/8-code-review/diff-other.patch
- `A` docs/features/phase-6-order-card/8-code-review/input-bundle.md
- `A` docs/features/phase-6-order-card/8-code-review/mobile.md
- `A` docs/features/phase-6-order-card/8-code-review/output.md
- `A` docs/features/phase-6-order-card/9-validation/evidence/e2e-run.txt
- `A` docs/features/phase-6-order-card/9-validation/evidence/last-run.json

## Экстракты документов (вместо полного чтения)
### draft-spec.md
MISSING

### prd.md (только AC-строки)
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

### design-errors.md (усечён до 6KB)
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
| | `assignee_id` | NULL или активный пользователь (пр

## Инвентарь доков (точечный grep разрешён, полное чтение — нет)
- 6-plan/phase-bundles/phase-02-input.md — 59KB
- 6-plan/phase-bundles/phase-04-input.md — 55KB
- 6-plan/phase-bundles/phase-01-input.md — 54KB
- 6-plan/phase-bundles/phase-03-input.md — 35KB
- 4-swimlane/input-bundle.md — 31KB
- 3-design/input-bundle.md — 31KB
- 6-plan/phase-bundles/phase-05-input.md — 30KB
- 3-design/output.md — 29KB
- 6-plan/phase-bundles/phase-06-input.md — 28KB
- 1-research/output.md — 28KB
- 3-design/design-go-layers.md — 23KB
- 6-plan/code-plan-status.md — 22KB
- 2-spec-interview/output.md — 19KB
- 3-design/design-tests.md — 18KB
- 8-code-review/input-bundle.md — 17KB
- 3-design/design-sql.md — 13KB
- 2-spec-interview/prd.md — 12KB
- 8-code-review/output.md — 11KB
- 6-plan/code-plan-phase-03-subtasks-products-logs.md — 11KB
- 2-spec-interview/input-bundle.md — 9KB
- 3-design/design-errors.md — 8KB
- 6-plan/code-plan-phase-02-card-api-equipment.md — 8KB
- 5-design-review/output.md — 8KB
- 6-plan/code-plan.md — 7KB
- 2-spec-interview/spec-decisions.md — 6KB

## Паспорт (выжимка; полный — 0-setup/passport.md)
# Passport: phase-6-order-card (generated — руками не править; stage_bundle.py)
**Tier:** M | **Совместимость:** ? (pipeline-doctrine.md §CM-1; ? → уточни у оркестратора)
**Трек:** обычный (prefs нет — дефолт) — режимы агентов (`spec-track delta/micro/mini/merged`) включает ТОЛЬКО строка `Режим:` в prompt оркестратора; DS/TS-индексы ниже присутствуют на ЛЮБОМ треке при наличии draft-spec.
**Layout:** автодетект (tools/layout_resolve.py)
**UI-scope (pipeline-doctrine.md §US-1, 0-setup/ui-scope-prefs.md):** swagger:no | adaptive:min | themes:both
