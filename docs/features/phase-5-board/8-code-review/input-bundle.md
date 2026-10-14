# Review Bundle: phase-5-board — READY
**Base:** be769e5 | **HEAD:** 34bd3af | **Generated:** 2026-09-27 03:51 | diff: 149 files changed, 31225 insertions(+), 187 deletions(-)

> Линзы ревью: читай ЭТОТ бандл + patch своей линзы. Полное чтение research/draft-spec/PRD
> и перегон чекеров ЗАПРЕЩЕНЫ (вердикты ниже; детали — checks/<name>.txt; точечный grep по DS-NNN разрешён).

## Чекеры (детерминированные — НЕ перегонять)
| Чекер | Вердикт | exit | Нарушения (первые строки) |
|---|---|---|---|
| go_naming | ✅ | 0 | — |
| api_conventions | ✅ | 0 | — |
| contract | ✅ | 0 | — |
| date_bound | ✅ | 0 | — |
| fuzz_verdict | ✅ | 0 | — |
| live_suite | ✅ | 0 | — |
| fixture_sync | ✅ | 0 | — |
| e2e_mode | ✅ | 0 | — |
| rule_usage | ✅ | 0 | — |

## Diff по линзам
| Линза | Файлов | Patch |
|---|---|---|
| backend | 59 | 8-code-review/diff-backend.patch |
| e2e | 13 | 8-code-review/diff-e2e.patch |
| migrations | 2 | 8-code-review/diff-migrations.patch |
| other | 11 | 8-code-review/diff-other.patch |
| docs | 64 | — (артефакты, не ревьюятся построчно) |

### backend
- `A` cmd/seed-demo/main.go
- `A` internal/features/board/assignees_handler.go
- `A` internal/features/board/board_fuzz_test.go
- `A` internal/features/board/board_http_live_test.go
- `A` internal/features/board/columns_dto.go
- `A` internal/features/board/columns_handler.go
- `A` internal/features/board/columns_repo.go
- `A` internal/features/board/columns_service.go
- `A` internal/features/board/columns_validate.go
- `A` internal/features/board/columns_validate_test.go
- `A` internal/features/board/enums.go
- `A` internal/features/board/enums_test.go
- `A` internal/features/board/errors.go
- `A` internal/features/board/position.go
- `A` internal/features/board/position_integration_test.go
- `A` internal/features/board/routes.go
- `A` internal/features/board/ssr.go
- `A` internal/features/board/stock_dto.go
- `A` internal/features/board/stock_engine.go
- `A` internal/features/board/stock_engine_integration_test.go
- `A` internal/features/board/stock_handler.go
- `A` internal/features/board/stock_http_live_test.go
- `A` internal/features/board/stock_repo.go
- `A` internal/features/board/stock_service.go
- `A` internal/features/board/tasks_dto.go
- `A` internal/features/board/tasks_filter.go
- `A` internal/features/board/tasks_filter_test.go
- `A` internal/features/board/tasks_handler.go
- `A` internal/features/board/tasks_repo.go
- `A` internal/features/board/tasks_service.go
- `A` internal/features/board/tasks_validate.go
- `A` internal/features/board/tasks_validate_test.go
- `A` internal/features/board/tx.go
- `M` internal/features/catalog/amount.go
- `M` internal/features/catalog/amount_validate.go
- `M` internal/features/catalog/products_service.go
- `M` internal/features/catalog/products_validate.go
- `M` internal/features/catalog/services_http_live_test.go
- `M` internal/features/catalog/services_repo.go
- `M` internal/features/catalog/services_service.go
- `M` internal/features/clients/clients_http_live_test.go
- `M` internal/features/clients/repo.go
- `M` internal/features/clients/service.go
- `M` internal/features/clients/ssr.go
- `D` internal/features/home/handler.go
- `A` internal/seeddemo/seeddemo.go
- `A` internal/seeddemo/seeddemo_integration_test.go
- `M` internal/server/middleware/auth.go
- `M` internal/server/router.go
- `M` internal/server/shell_http_live_test.go
- `M` internal/ssr/menu.go
- `M` internal/ssr/render_test.go
- `M` internal/ssr/shell.go
- `M` internal/ssr/shell_test.go
- `M` pkg/errorspkg/errors.go
- `A` pkg/errorspkg/errors_test.go
- `A` pkg/numeric/amount.go
- `A` pkg/numeric/amount_test.go
- `A` pkg/numeric/amount_validate.go

### e2e
- `M` e2e/fixtures/auth.ts
- `A` e2e/fixtures/board-types.ts
- `A` e2e/fixtures/board.ts
- `M` e2e/package-lock.json
- `M` e2e/package.json
- `A` e2e/pages/BoardPage.ts
- `A` e2e/pages/StockPage.ts
- `A` e2e/tests/board-guards.spec.ts
- `A` e2e/tests/board.spec.ts
- `M` e2e/tests/catalog.spec.ts
- `M` e2e/tests/shell.spec.ts
- `A` e2e/tests/stock.spec.ts
- `M` e2e/tests/users.spec.ts

### migrations
- `A` migrations/0005_board.down.sql
- `A` migrations/0005_board.up.sql

### other
- `M` Makefile
- `M` web/ssr/_layout/templates/layouts/base.html
- `M` web/ssr/_shared/app.css
- `M` web/ssr/_shared/js/api.js
- `A` web/ssr/_shared/js/board.js
- `M` web/ssr/_shared/js/combobox.js
- `A` web/ssr/_shared/js/nav-badge.js
- `A` web/ssr/_shared/js/stock.js
- `A` web/ssr/board/templates/pages/board.html
- `A` web/ssr/board/templates/pages/stock.html
- `D` web/ssr/home/templates/pages/index.html

### docs
- `A` docs/features/phase-5-board/0-setup/context-violations.jsonl
- `A` docs/features/phase-5-board/0-setup/diagrams-prefs.md
- `A` docs/features/phase-5-board/0-setup/e2e-scope-prefs.md
- `A` docs/features/phase-5-board/0-setup/integration-scope-prefs.md
- `A` docs/features/phase-5-board/0-setup/passport.md
- `A` docs/features/phase-5-board/0-setup/pipeline-state.md
- `A` docs/features/phase-5-board/0-setup/timeline.jsonl
- `A` docs/features/phase-5-board/0-setup/ui-scope-prefs.md
- `A` docs/features/phase-5-board/0-setup/watch-gate-prefs.md
- `A` docs/features/phase-5-board/1-research/input-bundle.md
- `A` docs/features/phase-5-board/1-research/output.md
- `A` docs/features/phase-5-board/2-spec-interview/input-bundle.md
- `A` docs/features/phase-5-board/2-spec-interview/output.md
- `A` docs/features/phase-5-board/2-spec-interview/prd.md
- `A` docs/features/phase-5-board/2-spec-interview/spec-decisions.md
- `A` docs/features/phase-5-board/3-design/design-errors.md
- `A` docs/features/phase-5-board/3-design/design-go-layers.md
- `A` docs/features/phase-5-board/3-design/design-sequence-diagrams.md
- `A` docs/features/phase-5-board/3-design/design-sql.md
- `A` docs/features/phase-5-board/3-design/design-tests.md
- `A` docs/features/phase-5-board/3-design/input-bundle.md
- `A` docs/features/phase-5-board/3-design/output.md
- `A` docs/features/phase-5-board/4-swimlane/design-swimlane.html
- `A` docs/features/phase-5-board/4-swimlane/input-bundle.md
- `A` docs/features/phase-5-board/5-design-review/input-bundle.md
- `A` docs/features/phase-5-board/5-design-review/mobile.md
- `A` docs/features/phase-5-board/5-design-review/output.md
- `A` docs/features/phase-5-board/6-plan/code-plan-phase-01-foundation.md
- `A` docs/features/phase-5-board/6-plan/code-plan-phase-02-board-columns-stock-engine.md
- `A` docs/features/phase-5-board/6-plan/code-plan-phase-03-board-tasks-move.md
- `A` docs/features/phase-5-board/6-plan/code-plan-phase-04-stock-api-badge-di.md
- `A` docs/features/phase-5-board/6-plan/code-plan-phase-05-seed-demo.md
- `A` docs/features/phase-5-board/6-plan/code-plan-phase-06-ui-board.md
- `A` docs/features/phase-5-board/6-plan/code-plan-phase-07-ui-stock.md
- `A` docs/features/phase-5-board/6-plan/code-plan-phase-08-guards-polish.md
- `A` docs/features/phase-5-board/6-plan/code-plan-status.md
- `A` docs/features/phase-5-board/6-plan/code-plan.md
- `A` docs/features/phase-5-board/6-plan/input-bundle.md
- `A` docs/features/phase-5-board/6-plan/phase-bundles/phase-01-input.md
- `A` docs/features/phase-5-board/6-plan/phase-bundles/phase-02-input.md
- `A` docs/features/phase-5-board/6-plan/phase-bundles/phase-03-input.md
- `A` docs/features/phase-5-board/6-plan/phase-bundles/phase-04-input.md
- `A` docs/features/phase-5-board/6-plan/phase-bundles/phase-05-input.md
- `A` docs/features/phase-5-board/6-plan/phase-bundles/phase-06-input.md
- `A` docs/features/phase-5-board/6-plan/phase-bundles/phase-07-input.md
- `A` docs/features/phase-5-board/6-plan/phase-bundles/phase-08-input.md
- `A` docs/features/phase-5-board/8-code-review/checks/api_conventions.txt
- `A` docs/features/phase-5-board/8-code-review/checks/contract.txt
- `A` docs/features/phase-5-board/8-code-review/checks/date_bound.txt
- `A` docs/features/phase-5-board/8-code-review/checks/e2e_mode.txt
- `A` docs/features/phase-5-board/8-code-review/checks/fixture_sync.txt
- `A` docs/features/phase-5-board/8-code-review/checks/fuzz_verdict.txt
- `A` docs/features/phase-5-board/8-code-review/checks/go_naming.txt
- `A` docs/features/phase-5-board/8-code-review/checks/live_suite.txt
- `A` docs/features/phase-5-board/8-code-review/checks/rule_usage.txt
- `A` docs/features/phase-5-board/8-code-review/diff-backend.patch
- `A` docs/features/phase-5-board/8-code-review/diff-e2e.patch
- `A` docs/features/phase-5-board/8-code-review/diff-migrations.patch
- `A` docs/features/phase-5-board/8-code-review/diff-other.patch
- `A` docs/features/phase-5-board/8-code-review/input-bundle.md
- `A` docs/features/phase-5-board/8-code-review/mobile.md
- `A` docs/features/phase-5-board/8-code-review/output.md
- `A` docs/features/phase-5-board/9-validation/evidence/e2e-run.txt
- `A` docs/features/phase-5-board/9-validation/evidence/last-run.json

## Экстракты документов (вместо полного чтения)
### draft-spec.md
MISSING

### prd.md (только AC-строки)
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

### design-errors.md (усечён до 6KB)
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
| 03 | `SUBTASKS_NOT_DONE` | 409 | `ErrSubtasksNotDone` (board) + `details.subtasks[]` | «Есть незавершённые услуги — в финальную колонку нельзя» | AC-003; также PUT колонки is_final false→true при заказах с незавершёнными услугами (решение оркестратора, Code Review MEDIUM-3) |
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
    Err

## Инвентарь доков (точечный grep разрешён, полное чтение — нет)
- 6-plan/phase-bundles/phase-03-input.md — 62KB
- 6-plan/phase-bundles/phase-01-input.md — 61KB
- 6-plan/phase-bundles/phase-02-input.md — 60KB
- 6-plan/phase-bundles/phase-04-input.md — 60KB
- 6-plan/phase-bundles/phase-05-input.md — 57KB
- 6-plan/phase-bundles/phase-06-input.md — 37KB
- 6-plan/phase-bundles/phase-08-input.md — 33KB
- 6-plan/phase-bundles/phase-07-input.md — 33KB
- 4-swimlane/input-bundle.md — 31KB
- 1-research/output.md — 29KB
- 3-design/input-bundle.md — 29KB
- 3-design/output.md — 29KB
- 3-design/design-go-layers.md — 20KB
- 8-code-review/input-bundle.md — 19KB
- 3-design/design-sql.md — 19KB
- 2-spec-interview/output.md — 16KB
- 3-design/design-tests.md — 15KB
- 6-plan/code-plan-phase-03-board-tasks-move.md — 13KB
- 2-spec-interview/prd.md — 12KB
- 2-spec-interview/input-bundle.md — 12KB
- 8-code-review/output.md — 11KB
- 6-plan/code-plan-status.md — 11KB
- 3-design/design-errors.md — 9KB
- 5-design-review/mobile.md — 8KB
- 8-code-review/mobile.md — 7KB

## Паспорт (выжимка; полный — 0-setup/passport.md)
# Passport: phase-5-board (generated — руками не править; stage_bundle.py)
**Tier:** M | **Совместимость:** ? (pipeline-doctrine.md §CM-1; ? → уточни у оркестратора)
**Трек:** обычный (prefs нет — дефолт) — режимы агентов (`spec-track delta/micro/mini/merged`) включает ТОЛЬКО строка `Режим:` в prompt оркестратора; DS/TS-индексы ниже присутствуют на ЛЮБОМ треке при наличии draft-spec.
**Layout:** автодетект (tools/layout_resolve.py)
**UI-scope (pipeline-doctrine.md §US-1, 0-setup/ui-scope-prefs.md):** swagger:no | adaptive:min | themes:both
