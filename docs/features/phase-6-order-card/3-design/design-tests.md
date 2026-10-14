# Design Tests: phase-6-order-card

> Owner: тест-матрица. Уровни: unit (без БД) · integration `BaseSuite` (dockertest `postgres:18-alpine`, тег `integration`) · live `testutil.LiveSuite` (реальный бинарь + TCP + реальный login; auth-стабы и `app.Test` запрещены, кроме fuzz §9.1 — §TD-8) · E2E Playwright (`--project=chromium --headed --workers=1`). ID: `UT-NNN` unit, `IT-NNN` integration, `LV-1NN` live (продолжение LV-0NN фазы 5), `FZ-NNN` fuzz, `OC-NNN` E2E. Коды — `design-errors.md`. Каждая строка → AC.

## Test Files
| Файл | Уровень |
|---|---|
| `internal/features/board/card_validate_test.go` (`SubtaskUpdateRequest`, `SubtaskProductCreate/UpdateRequest`, `TaskEquipment*Request`, `SubtaskLogRequest`), `description_test.go`, `enums_test.go` (+PrintKind), `tasks_validate_test.go` (+description) | unit |
| `internal/features/board/board_fuzz_test.go` (+FZ-001…003) | fuzz §9.1 |
| `internal/features/board/card_integration_test.go` (`CardSuite`, BaseSuite) | integration |
| `internal/features/board/order_card_http_live_test.go` (`OrderCardLiveSuite`), `print_http_live_test.go` (`PrintLiveSuite`) | live |
| `internal/features/board/board_http_live_test.go` (регресс + LV-150) | live (дополнение) |
| `e2e/tests/order-card.spec.ts`, `e2e/tests/order-print.spec.ts`; POM `e2e/pages/OrderCardPage.ts` (new), `e2e/pages/BoardPage.ts` (mod) | E2E |
| `e2e/fixtures/board.ts` (mod: pg → API, ADR-101), `e2e/fixtures/board-types.ts` (+типы OrderCard) | E2E fixtures |

## Happy Path
| ID | Сценарий | Уровень | AC |
|---|---|---|---|
| LV-101 | `GET /tasks/:id/card` заказа seed: task (+description_html, total_amount, last_log), column, equipment+history, subtasks+products+logs, stock (task_reserved); итог — только `task.total_amount` (поля `total` нет); пустые — массивы `[]`, не null | live | AC-001 |
| LV-102 | `POST /tasks` с `description` «a<b>\n\nc» → `description_html`=`<p>a&lt;b&gt;</p><p>c</p>`; `PUT` без `description` — поле не меняется; с `""` — очищается | live | AC-002 |
| LV-103 | `POST /tasks/:id/subtasks` в НЕ списывающей колонке → 201; `price`=`services.price`; товары = комплект (qty, `products.price`, порядок `service_products.position`); position = max+1; `warnings: []`; `tasks.version` не изменился | live | AC-007, AC-008 |
| LV-104 | `PUT /subtasks/:id` status/assignee/due/price(null=«по факту»)/equipment_id из заказа → 200; `card.subtasks[i]` обновлена, `subtasks[i].total` и `task.total_amount` пересчитаны | live | AC-007, AC-011 |
| LV-105 | `POST /subtasks/:id/products` (qty `0.5`), `PUT /subtask-products/:id` (qty, price, price=null), `DELETE` → карточка и `line_total` корректны | live | AC-008 |
| LV-106 | Итоги: услуга 1000 + товары 2×150.50 + 0.333×10 (→3.33) + товар price=null; услуга «по факту» с товаром 1×100 → `subtask.total`, `task.total_amount`=Σ, `GET /tasks` `total_amount` = тот же | live | AC-013 |
| LV-107 | Журнал: `POST /subtasks/:id/logs` → author/created_at, `can_edit=true`; `PUT`, `DELETE` своей; `GET /tasks` → `last_log` = последняя по времени среди услуг, обрезка 120 + «…» | live | AC-012 |
| LV-108 | Оборудование: `POST /tasks/:id/equipment` (completeness/condition) → 201; повтор → 201 без дубля; `PUT` → поля; `DELETE` не используемой → 200 | live | AC-004 |
| LV-109 | Новое оборудование executor: `POST /clients/:id/equipment` → 201 → `POST /tasks/:id/equipment` → в карточке; `GET /clients/:id/equipment` содержит единицу | live | AC-005 |
| LV-110 | История: единица в заказах #A, #B → карточка #C: `history` = [{B}, {A}] с `created_at` (без #C), в карточке #A — только #B | live | AC-006 |
| LV-111 | SSR `/?task=N` → 200 (каркас доски, deep-link обрабатывает JS) | live | AC-001 |

## Stock Reconcile
| ID | Сценарий | Уровень | AC |
|---|---|---|---|
| LV-120 | Заказ в «В работе» (consumes): add услуги с комплектом → stock−qty, `task_stock`=demand, `stock_movements` `task_consume` по товару | live | AC-010 |
| LV-121 | consumes: qty 2→3.5 → движение −1.5; 3.5→1 → `task_return` +2.5; delete товара → возврат всего; delete услуги → возврат всех её товаров | live | AC-010 |
| LV-122 | consumes: нехватка → 200, `warnings[{kind:"negative"}]`; stock≤min → `below_min` | live | AC-010 |
| LV-123 | не consumes: любые правки товаров → 0 движений, `task_stock` пуст | live | AC-010 |
| IT-001 | Инвариант склада: сценарий LV-120…121 + move туда-обратно → для каждого товара `stock_after − stock_before = Σ qty_delta движений теста` И `task_stock.qty = Σ subtask_products.qty` (consumes) / нет строк (не consumes) | integration | AC-010, AC-014 |

## Concurrency
| ID | Сценарий | Уровень | AC |
|---|---|---|---|
| IT-010 | 2 горутины × 20 итераций `UpdateSubtaskProduct` одной строки (разные qty) в consumes-колонке → без ошибок (ретраи 40P01 допустимы); итог qty = последний commit; инвариант IT-001 выполняется | integration | AC-014 |
| IT-011 | параллельно: `UpdateSubtaskProduct` товара X в заказе 1 и `Move` заказа 2 с товаром X + `PUT /columns` consumes toggle → нет deadlock-500 (ретраи), инвариант IT-001 | integration | AC-014 |
| IT-012 | `DeleteSubtask` ∥ `UpdateSubtaskProduct` её товара → один успешен, второй 404 `SUBTASK_PRODUCT_NOT_FOUND`/`SUBTASK_NOT_FOUND`; склад согласован | integration | AC-014 |
| IT-020 | Миграция 0006 на БД с `subtask_products` фазы 5 (down до 0005 → вставка 3 товаров услуги → up) → `position` = 1..3, без нулей | AC-008 |
| IT-013 | `Move` заказа между чтением `column_id` и `LockTask` (хук теста) → `errTaskMovedRetry` → повтор, Reconcile по новой колонке | integration | AC-014 |
| LV-130 | Row-мутации не меняют `tasks.version`: после 3 правок строк `PUT /tasks/:id` со старым version → 200 | live | AC-002, AC-014 |
| LV-131 | Два `PUT /tasks/:id` с одним version → 200 + 409 `TASK_VERSION_CONFLICT`; данные первого сохранены | live | AC-002 |

## Business Rules
| ID | Сценарий | Ожидание | AC |
|---|---|---|---|
| LV-140 | финальная колонка: `POST /tasks/:id/subtasks` | 409 `SUBTASK_FINAL_COLUMN`, услуг не прибавилось | AC-011 |
| LV-141 | финальная: `PUT /subtasks/:id` done→in_progress; done→done с правкой цены | 409 `SUBTASK_FINAL_COLUMN`; второй — 200 | AC-011 |
| LV-142 | `DELETE /tasks/:id/equipment/:eq` при выбранной в услуге единице | 409 `TASK_EQUIPMENT_IN_USE`, `details.subtasks[{id,service_name,status}]` | AC-004 |
| LV-143 | `PUT /tasks/:id` смена клиента: чужая единица не в услугах → 200, `task_equipment` очищена; в услуге → 409 `TASK_EQUIPMENT_IN_USE`, клиент не сменён | 409/200 | AC-004 |
| LV-144 | `POST /tasks/:id/equipment` единица другого клиента | 400 `fields[equipment_id]` «оборудование другого клиента» | AC-004 |
| LV-145 | `PUT /subtasks/:id` `equipment_id` не из заказа | 400 `fields[equipment_id]` | AC-007 |
| LV-146 | `PUT /subtasks/:id` новый `assignee_id` заблокированного → 400 `fields[assignee_id]`; ранее назначенный заблокирован, правка цены с тем же assignee → 200, `assignee_is_active=false` | 400/200 | AC-007 |
| LV-147 | `POST /tasks/:id/subtasks` архивной услуги; `POST /subtasks/:id/products` архивного товара | 400 `fields[service_id]`/`fields[product_id]` «в архиве» | AC-007, AC-008 |
| LV-148 | `DELETE /subtasks/:id` → логи и товары удалены каскадом | 200 | AC-007, AC-012 |
| LV-150 | регресс фазы 5: move в финальную при незавершённой услуге, созданной через API | 409 `SUBTASKS_NOT_DONE` | AC-011 |

## Access Control
| ID | Сценарий | Ожидание | AC |
|---|---|---|---|
| LV-160 | executor: все row-эндпоинты (subtasks, products с ценой, equipment, logs), `PUT /tasks` без смены is_paid | 2xx | AC-017, AC-008 |
| LV-161 | executor: `PUT /tasks/:id` is_paid≠текущему → 403 `PAID_ADMIN_ONLY`; `description`+title правка без `is_paid`-изменения → 200, is_paid сохранён | 403/200 | AC-003 |
| LV-162 | executor: `DELETE /tasks/:id` | 403 `FORBIDDEN` | AC-017 |
| LV-163 | executor B: `PUT/DELETE /subtask-logs/:id` записи A → 403 `SUBTASK_LOG_FORBIDDEN`; admin → 200; `can_edit` у B=false, у admin=true | 403/200 | AC-012 |
| LV-164 | без сессии: все новые API → 401; SSR печать → редирект на логин; мутации без CSRF → 403 | 401/302/403 | AC-017 |

## Invalid Input
| ID | Сценарий | Ожидание | AC |
|---|---|---|---|
| LV-170 | `/subtasks/not-uuid`, `/subtasks/<random uuid>`, `/subtask-products/x`, `/subtask-logs/<uuid>`, `/tasks/abc/card`, `/tasks/1/equipment/x` | 404 соотв. код 16-19 / `TASK_NOT_FOUND` | AC-014 |
| LV-171 | qty 0, −1, `1.2345`, `"5"` (строка); price −1, `1.001`; text `"   "`, 4001 символ; completeness 1001; description 10001; status `closed`; due_date `2026-02-30` | 400 fields / 400 `BAD_REQUEST` (`"5"`) | AC-008, AC-012 |
| LV-172 | мусорное тело (не JSON, массив, лишнее поле `task_id` в `PUT /subtasks`) | 400 `BAD_REQUEST`, не 500 | — (TD-4) |
| LV-173 | повтор `DELETE /subtask-products/:id` | 404 `SUBTASK_PRODUCT_NOT_FOUND` (идемпотентность: состояние не меняется) | AC-014 |
| LV-174 | `POST /subtasks/:id/logs` удалённой услуги | 404 `SUBTASK_NOT_FOUND` | AC-014 |

## Print
| ID | Сценарий | Уровень | AC |
|---|---|---|---|
| LV-180 | `GET /tasks/:id/print/intake` executor → 200, HTML: FIRM_* (env LiveSuite), `#N`, дата, клиент+телефоны, оборудование (тип, бренд/модель, серийный, комплектность, состояние), описание, «Сдал»/«Принял»; `Cache-Control: no-store` | live | AC-015 |
| LV-181 | `.../print/handover` → + услуги со статусом и ценой, товары qty×цена, итог = `card.task.total_amount`, оплата, «Выдал»/«Получил» | live | AC-016 |
| LV-182 | FIRM_* пустые → 200, блока реквизитов нет; `print/other` → 404; нет заказа → 404; `description` с `<script>` → экранирован в HTML | live | AC-015, AC-016 |

## Unit
| ID | Сценарий | AC |
|---|---|---|
| UT-001 | `descriptionToHTML`: экранирование `<>&"'`, пустые строки → абзацы, `\n` → `<br>`, `""` → `""` | AC-002 |
| UT-002 | validate row-запросов: границы qty/price (Scale), text trim/4000, status enum, nil vs value | AC-008, AC-012 |
| UT-003 | `PrintKinds` = `{intake, handover}` | AC-015 |

## Fuzzing
Класс 9 (новые bind-парсеры тел). Таргеты в `board_fuzz_test.go` (app.Test допустим §9.1): инварианты — no panic, только 400/404/409/2xx, никогда 5xx; seeds — `{}`, `null`, `[]`, `{"qty":1e309}`, `{"qty":"NaN"}`, `{"price":-0}`, 4001×«я», `\u0000`, вложенный JSON.
- FZ-001 `PUT /subtasks/:id` (`SubtaskUpdateRequest`)
- FZ-002 `POST /subtasks/:id/products`, `PUT /subtask-products/:id`
- FZ-003 `POST /subtasks/:id/logs`, `POST /tasks/:id/equipment`

## E2E
POM `e2e/pages/OrderCardPage.ts` (testid — `design-go-layers.md §Frontend Modules`). Данные — только API-фикстуры (ADR-101), без `pg`.
| ID | Сценарий | AC |
|---|---|---|
| OC-001 | «+ Заказ» → разделы оборудования/услуг неактивны → «Сохранить» → модалка остаётся, заголовок `#N`, разделы активны | AC-001 |
| OC-002 | Клик по карточке → все разделы; deep-link `/?task=N` открывает карточку; закрытие убирает `task` из URL; `/?task=999999` → сообщение «Заказ не найден» | AC-001 |
| OC-003 | Шапка: title/description/срок → «Сохранить» → reload → значения на месте | AC-002 |
| OC-004 | Две вкладки: A сохраняет шапку, B сохраняет свою → B видит «Заказ уже изменён, обновите», данные A на месте | AC-002 |
| OC-005 | executor: чекбокс оплаты disabled; сохранение шапки не сбрасывает is_paid=true | AC-003, AC-017 |
| OC-006 | Добавить оборудование клиента, заполнить комплектность/состояние → reload → сохранено; удалить; смена клиента → confirm → оборудование убрано | AC-004 |
| OC-007 | «Новое оборудование» (тип, бренд/модель, серийный) → в карточке и в списке оборудования клиента | AC-005 |
| OC-008 | Единица из закрытого заказа → «Уже был в ремонте: #N …» | AC-006 |
| OC-009 | Добавить услугу → цена из каталога, комплект товаров; изменить цену на пусто («по факту»), исполнителя, срок, статус, единицу; reload → сохранено без кнопки «Сохранить» | AC-007, AC-008 |
| OC-010 | Товар: qty `0.5`, цена; подсказка «в наличии N» / «останется N — минимум M» / «в наличии только N» при росте qty | AC-008, AC-009 |
| OC-011 | Заказ в «В работе»: +qty товара → предупреждение склада; «Остатки» — остаток уменьшился; удаление услуги → вернулся | AC-010 |
| OC-012 | Финальная колонка: кнопка «Добавить услугу» → сообщение; статус «Выполнена» → «В работе» → сообщение, откат значения | AC-011 |
| OC-013 | Журнал: добавить, исправить, удалить свою; чужая (admin создал) у executor — без кнопок; на доске — последняя запись | AC-012 |
| OC-014 | Итог услуги/заказа в карточке; `task-card-total` на доске; `column-total` = сумма карточек колонки | AC-013 |
| OC-015 | Две вкладки правят qty одного товара → после reload обеих — последнее значение; «Остатки» = стартовый − последний qty | AC-014 |
| OC-016 | Параллельно вкладка B удалила услугу, A правит её товар → «Строка уже удалена», карточка перечитана | AC-014 |
| OC-017 | «Акт приёма» → новая вкладка `/tasks/N/print/intake`: реквизиты, клиент, оборудование, подписи; тёмная тема UI → страница светлая | AC-015 |
| OC-018 | «Акт выдачи» → услуги, товары, итог, оплата, подписи | AC-016 |
| OC-019 | executor: нет кнопки удаления заказа (testid отсутствует), всё остальное редактируется | AC-017 |
| OC-020 | 375 px: модалка во всю ширину, таблица товаров со скроллом внутри, нет горизонтального скролла страницы | AC-001 |
| OC-021 | TD-4 UI: qty `-1`/`abc` → ошибка у поля, значение откатилось, склад не изменился | AC-008 |
| OC-022 | Self-race (ADR-105): `page.route` задерживает ответ первого `PUT /subtasks/:id` на 1.5 с; быстро меняем статус, затем срок, затем цену → в сети ≤1 in-flight PUT этой строки (счётчик в route), второй PUT несёт статус+срок+цену; после reload все три значения сохранены | AC-007, AC-014 |
| OC-023 | То же для товара: qty 1→2→3 при задержке первого ответа → финальный qty=3, «Остатки» = старт − 3 (consumes) | AC-010, AC-014 |

## Fixtures
`e2e/fixtures/board.ts` (ADR-101): pg-хелперы заменяются API-хелперами через `request` с сессией admin — `addSubtask(taskId, {serviceId, status?, dueDate?, products?})` = `POST /tasks/:id/subtasks` → при `products` — `DELETE` скопированных + `POST /subtasks/:id/products` → при `status/dueDate` — `PUT /subtasks/:id`; `attachEquipment(taskId, equipmentId)` = `POST /tasks/:id/equipment`. Порядок: услуги создаются до переноса заказа в финальную колонку. `pg` из `e2e/package.json` удаляется, если других импортов нет. Существующие `board.spec.ts`/`board-guards.spec.ts`/`stock.spec.ts` — регресс-гейт после замены.

## Coverage AC
AC-001 LV-101/111 OC-001/002/020 · AC-002 LV-102/130/131 OC-003/004 · AC-003 LV-161 OC-005 · AC-004 LV-108/142/143/144 OC-006 · AC-005 LV-109 OC-007 · AC-006 LV-110 OC-008 · AC-007 LV-103/104/145/146 OC-009/022 · AC-008 LV-105/171 IT-020 OC-010/021 · AC-009 OC-010 · AC-010 LV-120…123 IT-001 OC-011 · AC-011 LV-140/141/150 OC-012 · AC-012 LV-107/163 OC-013 · AC-013 LV-106 OC-014 · AC-014 IT-010…013 LV-130/170 OC-015/016/022/023 · AC-015 LV-180/182 OC-017 · AC-016 LV-181 OC-018 · AC-017 LV-160/162/164 OC-019.
