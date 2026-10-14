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
