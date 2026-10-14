# Input Bundle: phase-6-order-card / стадия 4-swimlane — generated 2026-09-27 11:13

> Твой ЕДИНСТВЕННЫЙ документный вход (пайп-контракт). Upstream-доки напрямую НЕ читать;
> точечный grep по конкретному ID (DS-NNN/AC-NNN/TS-NNN) — разрешён. Отсутствие нужного
> факта в бандле = сигнал оркестратору (дырка контракта), не повод читать всё.

## Источник: 3-design/output.md [full]
---
stage: 3-design
feature: phase-6-order-card
tier: M
---
# Design: phase-6-order-card — полная карточка заказа
**Дата:** 2026-09-27
**Tier:** M | **Совместимость:** CM-1 MVP | **UI-scope:** swagger:no · adaptive:min · themes:both
**На основе:** `3-design/input-bundle.md` (Tech Spec, AC-001…AC-017, research-экстракты); база — дизайн фазы 5 (`docs/features/phase-5-board/3-design/`), расширяется, не переопределяется.

```yaml
diagrams:
  dataflow: no
  sequence: yes
  swimlane: yes
```

## Обзор
Простая форма заказа (board.js `openTaskForm`) → rich-модалка карточки: шапка с `version` и описанием; оборудование заказа с историей; услуги-подзадачи с товарами и журналом; итоги; подсказка остатка; списание через `StockEngine.Reconcile` при каждой правке состава в списывающей колонке; SSR-акты приёма/выдачи. Код — пакет `internal/features/board` (новые файлы-по-сущности), одна миграция `0006_subtask_logs`. Фронт — SSR html/template + vanilla JS + Bootstrap 5.3 (D30/D38), новый модуль `web/ssr/_shared/js/task-card.js`.

**Triage:** `scope: cross-feature, tier: M, touches: {db: true, fe: false, infra: false, external: false}, risk: reversible, novelty: standard-crud, decisions: []` (источник: 1-research §Triage). Открытые вопросы Q-001…Q-007 → ADR-001…ADR-007.

## Детальные документы (owner-файлы)
`design-errors.md` (единственный источник кодов) · `design-sql.md` · `design-go-layers.md` · `design-tests.md` · `design-sequence-diagrams.md`.

## Контракт
Все JSON-эндпоинты — `/api/v1`, `RequireAPI` (сессия + CSRF на мутациях), без admin-гарда (D19). DTO — `design-go-layers.md`; коды — `design-errors.md`.

| Метод | Путь | Запрос | Ответ | Driver |
|---|---|---|---|---|
| GET | `/api/v1/tasks/:id/card` | — | 200 `OrderCardResponse` | AC-001 |
| POST | `/api/v1/tasks` (mod) | `TaskCreateRequest` + `description` | 201 `TaskMutationResponse` (as-is) | AC-001 |
| PUT | `/api/v1/tasks/:id` (mod) | `TaskUpdateRequest` + `description` | 200 `TaskMutationResponse`; 409 `TASK_VERSION_CONFLICT`/`TASK_EQUIPMENT_IN_USE`; 403 `PAID_ADMIN_ONLY` | AC-002, AC-003, AC-004 |
| GET | `/api/v1/tasks` (mod) | as-is | элементы + `total_amount`, `last_log` | AC-012, AC-013 |
| POST | `/api/v1/tasks/:id/subtasks` | `SubtaskCreateRequest` | 201 `OrderCardMutationResponse`; 409 `SUBTASK_FINAL_COLUMN` | AC-007, AC-008, AC-011 |
| PUT | `/api/v1/subtasks/:id` | `SubtaskUpdateRequest` | 200 `OrderCardMutationResponse`; 409 `SUBTASK_FINAL_COLUMN` | AC-007, AC-011 |
| DELETE | `/api/v1/subtasks/:id` | — | 200 `OrderCardMutationResponse` | AC-007, AC-010 |
| POST | `/api/v1/subtasks/:id/products` | `SubtaskProductCreateRequest` | 201 `OrderCardMutationResponse` | AC-008, AC-010 |
| PUT | `/api/v1/subtask-products/:id` | `SubtaskProductUpdateRequest` | 200 `OrderCardMutationResponse` | AC-008, AC-010, AC-014 |
| DELETE | `/api/v1/subtask-products/:id` | — | 200 `OrderCardMutationResponse` | AC-008, AC-010 |
| POST | `/api/v1/tasks/:id/equipment` | `TaskEquipmentCreateRequest` | 201 `OrderCardMutationResponse` | AC-004, AC-005 |
| PUT | `/api/v1/tasks/:id/equipment/:equipment_id` | `TaskEquipmentUpdateRequest` | 200 `OrderCardMutationResponse` | AC-004 |
| DELETE | `/api/v1/tasks/:id/equipment/:equipment_id` | — | 200 `OrderCardMutationResponse`; 409 `TASK_EQUIPMENT_IN_USE` | AC-004 |
| POST | `/api/v1/subtasks/:id/logs` | `SubtaskLogRequest` | 201 `OrderCardMutationResponse` | AC-012 |
| PUT | `/api/v1/subtask-logs/:id` | `SubtaskLogRequest` | 200 `OrderCardMutationResponse`; 403 `SUBTASK_LOG_FORBIDDEN` | AC-012 |
| DELETE | `/api/v1/subtask-logs/:id` | — | 200 `OrderCardMutationResponse`; 403 `SUBTASK_LOG_FORBIDDEN` | AC-012 |
| POST | `/api/v1/clients/:id/equipment` (as-is) | `EquipmentRequest` | 201 `EquipmentResponse` | AC-005 |
| GET (SSR) | `/tasks/:id/print/intake`, `/tasks/:id/print/handover` | — | 200 HTML (layout `print`), `RequireSSR`, no-store | AC-015, AC-016 |
| GET (SSR) | `/?task=N` (deep-link, as-is роут `/`) | — | каркас доски, карточку открывает JS | AC-001 |

§API-1: сегменты kebab (`subtask-products`, `subtask-logs`), JSON snake_case, списков нового вида нет (журнал и строки отдаются в составе карточки). NUMERIC — `pkg/numeric.Amount` (число на wire), nullable-цены — `*numeric.Amount` → `null`.

## Enums и константы
| Поле (DTO.json) | Значения | Источник правды | Sync-тест |
|---|---|---|---|
| `subtasks[].status` | `new, in_progress, done` | `internal/features/board/enums.go::SubtaskStatuses` (as-is) ↔ CHECK `0005_board.up.sql` | `internal/features/board/enums_test.go` (as-is) |
| path `:kind` печати | `intake, handover` | `enums.go::PrintKinds` (new) | `enums_test.go` (+UT-003) |
| `warnings[].kind` | `negative, below_min` | as-is `WarningKind` (ADR-006 фазы 5) | as-is |
| `stock_movements.reason` | `task_consume, task_return, manual` | as-is | as-is |

Детали — `design-go-layers.md §Enum SubtaskStatus`, `§Enum PrintKind`.

## ADR

### ADR-001: Карточка одним GET и полная карточка в ответе row-мутаций
- **Статус:** Принято
- **Driver:** AC-007 — «Пользователь добавляет в заказ услугу из каталога. Цена работы копируется из каталога, её можно изменить или оставить пустой («по факту»). У услуги задаются исполнитель, срок, статус и единица оборудования из заказа. Услугу можно удалить. Изменения сохраняются сразу, без кнопки «Сохранить» (source: D1, D3, SD-003).»
- **Отвечает на:** Q-001 — «Финализировать пути (вложенные `/tasks/:id/subtasks` vs плоские `/subtasks/:id`) по §API-1 и состав ответа мутации: строка + итоги заказа + `stock_warnings`, либо полная карточка.»
- **Контекст:** правка одной строки меняет итоги услуги/заказа, подсказки остатка по всем строкам с тем же товаром (Reconcile меняет `stock`/`task_reserved`) — частичный ответ требует дублирования формул в JS.
- **Решение:** создание — вложенный путь от родителя (`/tasks/:id/subtasks`, `/subtasks/:id/products`, `/subtasks/:id/logs`, `/tasks/:id/equipment`); правка/удаление — плоский по собственному id (`/subtasks/:id`, `/subtask-products/:id`, `/subtask-logs/:id`), оборудование — составной ключ `/tasks/:id/equipment/:equipment_id`. Чтение — `GET /tasks/:id/card` (7 запросов без N+1). Ответ любой row-мутации — `{card: OrderCardResponse, warnings}` из `Card()` после commit (re-fetch-after-write). Отдельного `GET /subtasks/:id/logs` нет — журнал в карточке. `GET /tasks/:id` — as-is.
- **Альтернативы:** строка+итоги (дублирует серверную арифметику и подсказку на клиенте); расширить `GET /tasks/:id` (ломает форму `TaskResponse` в mutation-ответах шапки).
- **Риски:** ответ тяжелее (в пределах p95 < 500 мс); перерисовка не трогает элемент с фокусом (`applyCard`).

### ADR-002: Мутации строк не трогают `tasks.version` и `tasks.updated_at`
- **Статус:** Принято
- **Driver:** AC-002 — «Название, описание, клиент, колонку, ответственного, срок и оплату пользователь меняет и сохраняет кнопкой «Сохранить». Если заказ за это время изменил кто-то другой, пользователь видит сообщение, что заказ уже изменён, и чужие данные не затираются (source: D28, SD-003).»
- **Отвечает на:** Q-002 — «Подтвердить, что мутации услуг/товаров/оборудования/журнала не инкрементируют `tasks.version` (иначе открытая шапка получает 409 после собственных правок строк), а `updated_at` заказа — обновляют или нет.»
- **Контекст:** `version` защищает только поля шапки (D28); строки — LWW.
- **Решение:** row-мутации НЕ меняют `tasks.version` и `tasks.updated_at` (`updated_at` = «изменена шапка»; строки имеют свой `subtasks.updated_at`/`subtask_logs.updated_at`). Сериализация со шапкой — через `LockTask FOR UPDATE`. SD-006 фазы 5 (move/PUT инкрементируют version) — as-is: шапка, открытая до чужого move, получит 409 — ожидаемо.
- **Альтернативы:** инкремент version (ложные 409); bump `updated_at` (запись без потребителя).
- **Риски:** «когда последний раз трогали заказ» не видно по `tasks.updated_at` — потребителя нет (MVP).

### ADR-003: Коды ошибок фазы 6 — отдельный `SUBTASK_FINAL_COLUMN`
- **Статус:** Принято
- **Driver:** AC-011 — «Статус услуги (новая / в работе / выполнена) меняется в карточке. Для заказа в финальной колонке нельзя вернуть услугу из «выполнена» и нельзя добавить новую услугу — пользователь видит объяснение (source: D11, D22).»
- **Отвечает на:** Q-003 — «Определить sentinel-ы: `TASK_EQUIPMENT_IN_USE`, финальная колонка для услуг (переиспользовать `SUBTASKS_NOT_DONE`-семейство или новый `SUBTASK_FINAL_COLUMN`), `SUBTASK_LOG_FORBIDDEN`, `SUBTASK_NOT_FOUND`/`SUBTASK_PRODUCT_NOT_FOUND`/`SUBTASK_LOG_NOT_FOUND`, неактивный исполнитель — поле 400.»
- **Контекст:** `SUBTASKS_NOT_DONE` означает «заказ нельзя перенести» и несёт `details.subtasks` — семантика другая, клиент ветвится по `code`.
- **Решение:** Codes 16-22 (`design-errors.md`): `SUBTASK_NOT_FOUND`, `SUBTASK_PRODUCT_NOT_FOUND`, `SUBTASK_LOG_NOT_FOUND`, `TASK_EQUIPMENT_NOT_FOUND` (404), `TASK_EQUIPMENT_IN_USE` (409 + `details.subtasks`), `SUBTASK_FINAL_COLUMN` (409), `SUBTASK_LOG_FORBIDDEN` (403). Неактивный исполнитель — 400 `fields[assignee_id]` (as-is сообщение «не найден»).
- **Альтернативы:** переиспользовать `SUBTASKS_NOT_DONE` (вводит в заблуждение UI); общий `FORBIDDEN` для журнала (неразличим с ролевым гардом).
- **Риски:** нет.

### ADR-004: Новое оборудование из карточки — двумя запросами
- **Статус:** Принято
- **Driver:** AC-005 — «Не выходя из карточки, исполнитель или администратор заводит новую единицу оборудования клиента (тип, бренд/модель, серийный номер). Она сразу добавляется в заказ и появляется в списке оборудования клиента (source: D19).»
- **Отвечает на:** Q-004 — «Проверить, разрешает ли API оборудования фичи `clients` запись исполнителю (D19); выбрать: два запроса (создать в clients + привязать к заказу) или составной `POST /tasks/:id/equipment` с вложенным новым оборудованием в одной транзакции.»
- **Контекст:** `POST /api/v1/clients/:id/equipment` уже без admin-гарда (`internal/features/clients/routes.go`, D19), валидирует тип (ADR-004 фазы 3).
- **Решение:** мини-форма в карточке → `POST /clients/{task.client_id}/equipment` → `POST /tasks/:id/equipment {equipment_id}`. Импорта `clients` в `board` нет.
- **Альтернативы:** составной эндпоинт (дублирует валидацию оборудования в `board` либо межпакетная зависимость).
- **Риски:** сбой второго запроса оставляет единицу только у клиента — не потеря данных; UI показывает ошибку и единицу в списке «Добавить».

### ADR-005: Итоги SQL-агрегатом, сумма колонки — на клиенте
- **Статус:** Принято
- **Driver:** AC-013 — «В карточке видны итог по каждой услуге и итог заказа: сумма цен работ плюс количество × цена по всем товарам. Услуга «по факту» в сумму не входит. Итог заказа виден и на карточке на доске, а сумма по заказам — в заголовке колонки (source: D3).»
- **Отвечает на:** Q-005 — «Спроектировать SQL для `GET /tasks` (итог заказа, `last_log`) и суммы по колонке — подзапросы/LATERAL, индексы (`subtask_logs(subtask_id, created_at)`, `subtasks(task_id)`); оценить влияние на латентность доски.»
- **Контекст:** доска — один `GET /tasks` (limit 500 as-is), фильтры применяются к карточкам.
- **Решение:** `design-sql.md §Totals Expression` (аддитивная формула: `line_total=round(qty×price,2)`, «по факту» → работа 0, товары входят — по Tech Spec) и `§Last Log Expression` (LATERAL LIMIT 1, индекс `subtask_logs_subtask_created_idx`) — коррелированные скаляры в `boardListSQL`/`Get`. Сумма колонки — сумма `total_amount` видимых карточек в JS (копейки, целые) — совпадает с отфильтрованным видом и не расширяет конверт `{items,total,limit,offset}`.
- **Альтернативы:** `column_totals` в ответе доски (поле вне конверта §API-1, расходится с фильтром); материализованный `tasks.total` (триггеры, риск рассинхрона).
- **Риски:** +2 коррелированных подзапроса на карточку доски; при сотнях заказов — единицы мс по индексам; замер — Performance Budget deferred.

### ADR-006: Описание — plain text, серверное экранирование, без зависимости
- **Статус:** Принято
- **Driver:** AC-002 — «Название, описание, клиент, колонку, ответственного, срок и оплату пользователь меняет и сохраняет кнопкой «Сохранить». Если заказ за это время изменил кто-то другой, пользователь видит сообщение, что заказ уже изменён, и чужие данные не затираются (source: D28, SD-003).»
- **Отвечает на:** Q-006 — «Редактор описания (plain textarea → `<p>`/`<br>` или простой rich-text как в прототипе) и серверная allowlist-санитизация; нужна ли новая зависимость (bluemonday) или хватит экранирования plain-текста.»
- **Контекст:** `description_html` — XSS-вектор на печатной странице и в карточке.
- **Решение:** UI — `<textarea>`; wire — `description` (plain text, ≤10000). Сервер `descriptionToHTML`: `html.EscapeString`, блоки по пустой строке → `<p>…</p>`, одиночный `\n` → `<br>`. В БД — только этот HTML (и seed-demo). Клиент получает текст для textarea из `description_html` через `DOMParser` (инертный документ, `<br>`→`\n`, `</p>`→`\n\n`). Печать — `template.HTML(description_html)`. PUT без `description` — не менять (защита от тихого сброса, как `is_paid`). Новой зависимости нет.
- **Альтернативы:** contenteditable + bluemonday (новая зависимость, сложный редактор без требования в AC).
- **Риски:** форматирование seed-demo (жирный) теряется при первом сохранении — приемлемо; в LV-182 проверяется, что `<script>` экранирован.

### ADR-007: `subtask_logs.user_id` — ON DELETE RESTRICT
- **Статус:** Принято
- **Driver:** AC-012 — «В каждой услуге пользователь добавляет запись «что сделано / что осталось». У записи видны автор и время. Свою запись он может исправить или удалить, администратор — любую. Последняя запись журнала видна на карточке заказа на доске (source: D13, D27).»
- **Отвечает на:** Q-007 — «`subtask_logs.user_id` — RESTRICT (как `stock_movements.user_id`) или SET NULL; сверить с поведением удаления пользователя в фиче `users` (фаза 2).»
- **Контекст:** фича `users` не удаляет пользователей (`internal/features/users/routes.go`: только GET/POST/PUT, блокировка статусом).
- **Решение:** `RESTRICT` (как `stock_movements.user_id`), `user_id NOT NULL` — автор всегда отображается, в т.ч. заблокированный.
- **Альтернативы:** `SET NULL` (анонимные записи, nullable-автор в DTO без сценария).
- **Риски:** будущее удаление пользователей потребует решения — вне MVP.

### ADR-100: Порядок блокировок для row-мутаций и ретрай при переносе
- **Статус:** Принято
- **Driver:** AC-014 — «Если двое одновременно меняют одну услугу или товар, сохраняется последнее изменение. Склад при этом остаётся согласован с журналом движений (source: D28, D34).»
- **Контекст:** ADR-104 фазы 5: `columns → tasks → products`; строка заказа знает только свой id, колонку заказа — нет; LOW code review фазы 5 — смена статуса услуги тоже под блокировкой.
- **Решение:** `design-sql.md §Lock Order`: `columns (FOR SHARE) → tasks (FOR UPDATE) → subtasks → subtask_products → products`. `lockRowsScope`: читать `tasks.column_id` без блокировки → `LockTargetColumn` → `LockTask` → при несовпадении колонки `errTaskMovedRetry`, который `withTx` ретраит наравне с 40P01/40001 (×3). Все row-мутации услуг/товаров (включая `UpdateSubtask` без Reconcile) идут этим путём; оборудование — `LockTask`; журнал — без блокировки заказа.
- **Альтернативы:** `LockTask` затем колонка (нарушает порядок → deadlock с `PUT /columns`, который ретраится, но шумит).
- **Риски:** 3 неудачных ретрая при непрерывном move → 500; вероятность пренебрежима.

### ADR-101: E2E-фикстуры услуг через API вместо `pg`
- **Статус:** Принято (заменяет ADR-103 фазы 5)
- **Driver:** AC-007 — «Пользователь добавляет в заказ услугу из каталога. Цена работы копируется из каталога, её можно изменить или оставить пустой («по факту»). У услуги задаются исполнитель, срок, статус и единица оборудования из заказа. Услугу можно удалить. Изменения сохраняются сразу, без кнопки «Сохранить» (source: D1, D3, SD-003).»
- **Контекст:** в фазе 5 API услуг не было, `e2e/fixtures/board.ts` писал в `subtasks`/`subtask_products`/`task_equipment` через `pg`, минуя Reconcile.
- **Решение:** API-хелперы `addSubtask`/`attachEquipment` (`design-tests.md §Fixtures`); `pg` удаляется из фикстур.
- **Альтернативы:** оставить `pg` (обходит инварианты склада, тесты не ловят регрессы Reconcile).
- **Риски:** фикстуры медленнее — единицы запросов на тест.

### ADR-102: Акты — SSR-страницы с отдельным layout `print`, реквизиты из env
- **Статус:** Принято
- **Driver:** AC-016 — «Из карточки открывается печатная страница акта выдачи. На ней: то же, что в акте приёма, плюс выполненные услуги с ценами, товары (количество × цена), итог, статус оплаты и подписи «Выдал / Получил» (source: D20, SD-004).»
- **Контекст:** SD-004: env `FIRM_NAME`, `FIRM_ADDRESS`, `FIRM_PHONE`; `ssr.Renderer.Render(w, page, layout, vm)` уже есть.
- **Решение:** `GET /tasks/:id/print/:kind` (`RequireSSR`, no-store), layout `web/ssr/board/templates/layouts/print.html` без сайдбара, всегда светлый, `@media print`. Данные — `CardService.PrintData` (Card + клиент/телефоны). Пустые `FIRM_*` → блок опускается, старт не падает. Акт выдачи перечисляет все услуги со статусом (Tech Spec §Печать); итог = `card.total` (AC-013) — одна цифра на карточке и акте.
- **Альтернативы:** только выполненные услуги (итог акта расходится с итогом карточки); реквизиты в БД (SD-004 b, отклонено).
- **Риски:** телефон клиента на странице — только авторизованным (`RequireSSR`).

### ADR-103: `subtask_products.position` для стабильного порядка
- **Статус:** Принято
- **Driver:** AC-008 — «При добавлении услуги в неё автоматически попадают товары её типового комплекта из каталога, с количеством и текущей ценой. Товары можно добавлять, удалять, менять количество (в том числе дробное) и цену. Цены в заказе может менять и исполнитель (source: D10, D3, D19, D33).»
- **Контекст:** `subtask_products` без порядковой колонки, id — `gen_random_uuid()` → порядок строк случаен.
- **Решение:** `ALTER TABLE subtask_products ADD COLUMN position int NOT NULL DEFAULT 0` в `0006`; комплект — порядок `service_products.position`, новый товар — max+1. Перестановки нет (non-goal).
- **Альтернативы:** `created_at` (комплект вставляется одним `now()` → ничья).
- **Риски:** нет (MVP, строк в проде нет).

### ADR-104: Модуль `task-card.js`, deep-link `/?task=N`
- **Статус:** Принято
- **Driver:** AC-001 — «Клик по карточке на доске или «+ Заказ» открывает большое окно поверх доски. В нём: название, описание, клиент, колонка, ответственный, срок, признак оплаты, оборудование, услуги с товарами и журналом, итог. По ссылке вида `/board?task=N` доска открывается сразу с карточкой заказа `#N` (source: D25, SD-002).»
- **Контекст:** доска живёт на `/` (ADR-100 фазы 5), `/board` не существует; `board.js` — 815 строк.
- **Решение:** deep-link — `/?task=N` (отклонение от буквы AC: путь доски, смысл тот же). Карточка — отдельный модуль `web/ssr/_shared/js/task-card.js` (`KT.taskCard`), `board.js` делегирует; модалка `modal-xl` + `modal-fullscreen-lg-down` (adaptive:min). `parseFilters` игнорирует `task`; открытие/закрытие — `history.replaceState` с сохранением фильтров.
- **Альтернативы:** расширять `board.js` (файл >1500 строк); отдельная страница `/tasks/:id` (SD-002 b, отклонено).
- **Риски:** справочники — только серверный поиск (`/services?q=`, `/products?q=`), без фоновой выгрузки (урок фазы 5).

## Summary ключевых решений
ADR-001 card-ответ · ADR-002 без version · ADR-003 Codes 16-22 · ADR-004 оборудование 2 запроса · ADR-005 итоги SQL · ADR-006 plain-описание · ADR-007 RESTRICT · ADR-100 Lock Order · ADR-101 фикстуры API · ADR-102 акты SSR · ADR-103 position · ADR-104 task-card.js — все Принято.

## Security
- Новых зависимостей нет (ADR-006).
- XSS: `description_html` пишет только сервер (`descriptionToHTML`) и seed-demo; все прочие строки (журнал, комплектность, имена) — через `esc()` в JS и автоэкранирование `html/template`.
- IDOR: row-эндпоинты резолвят родителя и блокируют строку с `task_id`/`subtask_id` в WHERE (404 при несовпадении); видимость заказов не ограничена ролью (D19).
- Права: журнал — автор или admin (`SUBTASK_LOG_FORBIDDEN`); оплата — `PAID_ADMIN_ONLY` as-is; удаление заказа — admin as-is. Печать — `RequireSSR` (телефон клиента только авторизованным).
- Оборудование чужого клиента — 400 (`equipment.client_id = tasks.client_id`, проверка под `LockTask`).

## Monitoring & Alerting Spec
Spans `board`: `tasks.card`, `subtasks.mutate`, `subtask_products.mutate`, `task_equipment.mutate`, `subtask_logs.mutate`, `tasks.print` (+as-is `stock.reconcile`); log fields `trace_id, task_id, subtask_id|product_id, op`. Алерты — **deferred** (MVP: прод-мониторинг ставит `/infra-bootstrap`, issue при инфра-фазе); инвариант склада — IT-001.

## Test Strategy
`design-tests.md`: каждый AC-001…AC-017 → ≥1 тест (§Coverage AC); concurrency IT-010…013, OC-015/016; инвариант склада IT-001; TD-4 LV-170…174, OC-021; fuzz FZ-001…003; фикстуры через API (ADR-101).

## Performance Budget
Deferred (MVP): цель PRD p95 < 500 мс на сохранение шапки/строки; load-test не пишется, issue — при инфра-фазе.

## Runbook (post-deploy)
Deferred (MVP): `make migrate-up` (0006), задать `FIRM_*`, smoke — карточка seed-demo, добавить услугу в «В работе», сверить «Остатки».

## Rollout Strategy
Deferred (MVP): без feature-flag, выкатывается целиком.

## Verification
- `go test ./internal/features/board/... -tags integration` (unit + IT + live), `go test -run Fuzz -fuzz=FuzzSubtask -fuzztime=30s ./internal/features/board`.
- `python3 ~/.claude/tools/api_conventions_check.py`, `tools/go_naming_check.py`.
- E2E: `npx playwright test e2e/tests/order-card.spec.ts e2e/tests/order-print.spec.ts e2e/tests/board.spec.ts e2e/tests/board-guards.spec.ts e2e/tests/stock.spec.ts --project=chromium --headed --workers=1` (владелец прогона — оркестратор).

## Паспорт (выжимка)
# Passport: phase-6-order-card (generated — руками не править; stage_bundle.py)
**Tier:** M | **Совместимость:** ? (pipeline-doctrine.md §CM-1; ? → уточни у оркестратора)
**Трек:** обычный (prefs нет — дефолт) — режимы агентов (`spec-track delta/micro/mini/merged`) включает ТОЛЬКО строка `Режим:` в prompt оркестратора; DS/TS-индексы ниже присутствуют на ЛЮБОМ треке при наличии draft-spec.
**Layout:** автодетект (tools/layout_resolve.py)
**UI-scope (pipeline-doctrine.md §US-1, 0-setup/ui-scope-prefs.md):** swagger:no | adaptive:min | themes:both

**Индексы:** AC 17 / DS 0 / TS 0 — полные списки: `0-setup/passport.md` (разрешён контрактом); конкретное решение — точечный grep по `DS-NNN`/`TS-NNN` в draft-spec §9/§11 (разрешён).
