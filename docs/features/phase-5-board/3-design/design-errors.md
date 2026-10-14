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
