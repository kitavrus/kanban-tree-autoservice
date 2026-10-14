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
Форма заказа → rich-модалка: шапка (`version`, описание), оборудование с историей, услуги с товарами и журналом, итоги, подсказка остатка, `StockEngine.Reconcile` при правке состава в списывающей колонке, SSR-акты. Пакет `internal/features/board`, миграция `0006_subtask_logs`, фронт — SSR + vanilla JS + Bootstrap 5.3 (`task-card.js`).

**Triage:** `scope: cross-feature, tier: M, touches: {db: true, fe: false, infra: false, external: false}, risk: reversible, novelty: standard-crud, decisions: []` (источник: 1-research §Triage). Открытые вопросы Q-001…Q-007 → ADR-001…ADR-007.

## Детальные документы (owner-файлы)
`design-errors.md` (единственный источник кодов) · `design-sql.md` · `design-go-layers.md` · `design-tests.md` · `design-sequence-diagrams.md`.

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

## Enums и константы
- `subtasks[].status`: `new, in_progress, done` — as-is `enums.go::SubtaskStatuses` ↔ CHECK `0005_board.up.sql`, тест `enums_test.go`.
- path `:kind` печати: `intake, handover` — `enums.go::PrintKinds` (new), тест `enums_test.go` (UT-003).
- `warnings[].kind` (`negative, below_min`), `stock_movements.reason` — as-is фазы 5.

## ADR

### ADR-001: Карточка одним GET и полная карточка в ответе row-мутаций
- **Статус:** Принято
- **Driver:** AC-007 — «Пользователь добавляет в заказ услугу из каталога. Цена работы копируется из каталога, её можно изменить или оставить пустой («по факту»). У услуги задаются исполнитель, срок, статус и единица оборудования из заказа. Услугу можно удалить. Изменения сохраняются сразу, без кнопки «Сохранить» (source: D1, D3, SD-003).»
- **Отвечает на:** Q-001 — «Финализировать пути (вложенные `/tasks/:id/subtasks` vs плоские `/subtasks/:id`) по §API-1 и состав ответа мутации: строка + итоги заказа + `stock_warnings`, либо полная карточка.»
- **Контекст:** правка строки меняет итоги и подсказки остатка по всем строкам с тем же товаром.
- **Решение:** создание — вложенный путь от родителя (`/tasks/:id/subtasks`, `/subtasks/:id/products`, `/subtasks/:id/logs`, `/tasks/:id/equipment`); правка/удаление — плоский по собственному id (`/subtasks/:id`, `/subtask-products/:id`, `/subtask-logs/:id`), оборудование — составной ключ `/tasks/:id/equipment/:equipment_id`. Чтение — `GET /tasks/:id/card` (7 запросов без N+1). Ответ любой row-мутации — `{card: OrderCardResponse, warnings}` из `Card()` после commit (re-fetch-after-write). Исключение — оборудование: у `task_equipment` нет собственного id (PK `task_id+equipment_id`), поэтому правка/удаление по составному ключу `/tasks/:id/equipment/:equipment_id`. Отдельного `GET /subtasks/:id/logs` нет — журнал в карточке. `GET /tasks/:id` — as-is.
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
- **Контекст:** `SUBTASKS_NOT_DONE` = «заказ нельзя перенести» — другая семантика.
- **Решение:** Codes 16-22 (`design-errors.md`): `SUBTASK_NOT_FOUND`, `SUBTASK_PRODUCT_NOT_FOUND`, `SUBTASK_LOG_NOT_FOUND`, `TASK_EQUIPMENT_NOT_FOUND` (404), `TASK_EQUIPMENT_IN_USE` (409 + `details.subtasks`), `SUBTASK_FINAL_COLUMN` (409), `SUBTASK_LOG_FORBIDDEN` (403). Неактивный исполнитель — 400 `fields[assignee_id]` (as-is сообщение «не найден»).
- **Альтернативы:** переиспользовать `SUBTASKS_NOT_DONE` (вводит в заблуждение UI); общий `FORBIDDEN` для журнала (неразличим с ролевым гардом).
- **Риски:** нет.

### ADR-004: Новое оборудование из карточки — двумя запросами
- **Статус:** Принято
- **Driver:** AC-005 — «Не выходя из карточки, исполнитель или администратор заводит новую единицу оборудования клиента (тип, бренд/модель, серийный номер). Она сразу добавляется в заказ и появляется в списке оборудования клиента (source: D19).»
- **Отвечает на:** Q-004 — «Проверить, разрешает ли API оборудования фичи `clients` запись исполнителю (D19); выбрать: два запроса (создать в clients + привязать к заказу) или составной `POST /tasks/:id/equipment` с вложенным новым оборудованием в одной транзакции.»
- **Контекст:** `POST /api/v1/clients/:id/equipment` уже без admin-гарда (`internal/features/clients/routes.go`, D19), валидирует тип (ADR-004 фазы 3).
- **Решение:** мини-форма в карточке → `POST /clients/{task.client_id}/equipment` → `POST /tasks/:id/equipment {equipment_id}`. Импорта `clients` в `board` нет.
- **Альтернативы:** составной эндпоинт (дубль валидации / зависимость пакетов).
- **Риски:** сбой второго запроса оставляет единицу только у клиента — не потеря данных; UI показывает ошибку и единицу в списке «Добавить».

### ADR-005: Итоги SQL-агрегатом, сумма колонки — на клиенте
- **Статус:** Принято
- **Driver:** AC-013 — «В карточке видны итог по каждой услуге и итог заказа: сумма цен работ плюс количество × цена по всем товарам. Услуга «по факту» в сумму не входит. Итог заказа виден и на карточке на доске, а сумма по заказам — в заголовке колонки (source: D3).»
- **Отвечает на:** Q-005 — «Спроектировать SQL для `GET /tasks` (итог заказа, `last_log`) и суммы по колонке — подзапросы/LATERAL, индексы (`subtask_logs(subtask_id, created_at)`, `subtasks(task_id)`); оценить влияние на латентность доски.»
- **Контекст:** доска — один `GET /tasks` (limit 500 as-is), фильтры применяются к карточкам.
- **Решение:** `design-sql.md §Totals Expression` (аддитивная формула: `line_total=round(qty×price,2)`, «по факту» → работа 0, товары входят — по Tech Spec) и `§Last Log Expression` (LATERAL LIMIT 1, индекс `subtask_logs_subtask_created_idx`) — коррелированные скаляры в `boardListSQL`/`Get`. Сумма колонки — сумма `total_amount` видимых карточек в JS (копейки, целые) — совпадает с отфильтрованным видом и не расширяет конверт `{items,total,limit,offset}`.
- **Альтернативы:** `column_totals` в ответе доски (вне конверта §API-1); материализованный итог (триггеры).
- **Риски:** +2 коррелированных подзапроса на карточку доски; при сотнях заказов — единицы мс по индексам; замер — Performance Budget deferred.

### ADR-006: Описание — plain text, серверное экранирование, без зависимости
- **Статус:** Принято
- **Driver:** AC-002 — «Название, описание, клиент, колонку, ответственного, срок и оплату пользователь меняет и сохраняет кнопкой «Сохранить». Если заказ за это время изменил кто-то другой, пользователь видит сообщение, что заказ уже изменён, и чужие данные не затираются (source: D28, SD-003).»
- **Отвечает на:** Q-006 — «Редактор описания (plain textarea → `<p>`/`<br>` или простой rich-text как в прототипе) и серверная allowlist-санитизация; нужна ли новая зависимость (bluemonday) или хватит экранирования plain-текста.»
- **Контекст:** `description_html` — XSS-вектор на печатной странице и в карточке.
- **Решение:** UI — `<textarea>`; wire — `description` (plain text, ≤10000). Сервер `descriptionToHTML`: `html.EscapeString`, блоки по пустой строке → `<p>…</p>`, одиночный `\n` → `<br>`. В БД — только этот HTML (и seed-demo). Клиент получает текст для textarea из `description_html` через `DOMParser` (инертный документ, `<br>`→`\n`, `</p>`→`\n\n`). Печать — `template.HTML(description_html)`. PUT без `description` — не менять (защита от тихого сброса, как `is_paid`). Новой зависимости нет.
- **Альтернативы:** contenteditable + bluemonday (новая зависимость).
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
- **Альтернативы:** `LockTask` затем колонка (нарушает порядок → deadlock с `PUT /columns`).
- **Риски:** 3 неудачных ретрая → 500; пренебрежимо.

### ADR-101: E2E-фикстуры услуг через API вместо `pg`
- **Статус:** Принято (заменяет ADR-103 фазы 5)
- **Driver:** AC-007 — «Пользователь добавляет в заказ услугу из каталога. Цена работы копируется из каталога, её можно изменить или оставить пустой («по факту»). У услуги задаются исполнитель, срок, статус и единица оборудования из заказа. Услугу можно удалить. Изменения сохраняются сразу, без кнопки «Сохранить» (source: D1, D3, SD-003).»
- **Контекст:** `e2e/fixtures/board.ts` пишет строки заказа через `pg`, минуя Reconcile.
- **Решение:** API-хелперы `addSubtask`/`attachEquipment` (`design-tests.md §Fixtures`); `pg` удаляется из фикстур.
- **Альтернативы:** оставить `pg` (обходит инварианты склада, тесты не ловят регрессы Reconcile).
- **Риски:** фикстуры медленнее — единицы запросов на тест.

### ADR-102: Акты — SSR-страницы с отдельным layout `print`, реквизиты из env
- **Статус:** Принято
- **Driver:** AC-016 — «Из карточки открывается печатная страница акта выдачи. На ней: то же, что в акте приёма, плюс выполненные услуги с ценами, товары (количество × цена), итог, статус оплаты и подписи «Выдал / Получил» (source: D20, SD-004).»
- **Контекст:** SD-004: env `FIRM_NAME`, `FIRM_ADDRESS`, `FIRM_PHONE`; `ssr.Renderer.Render(w, page, layout, vm)` уже есть.
- **Решение:** `GET /tasks/:id/print/:kind` (`RequireSSR`, no-store), layout `web/ssr/board/templates/layouts/print.html` без сайдбара, всегда светлый, `@media print`. Данные — `CardService.PrintData` (Card + клиент/телефоны). Пустые `FIRM_*` → блок опускается, старт не падает. Акт выдачи перечисляет все услуги со статусом — решение пользователя SD-009 (`2-spec-interview/spec-decisions.md`), уточняет AC-016; итог = `task.total_amount` (AC-013) — одна цифра на карточке и акте.
- **Альтернативы:** только выполненные услуги (отклонено пользователем, SD-009); реквизиты в БД (SD-004 b).
- **Риски:** телефон клиента на странице — только авторизованным (`RequireSSR`).

### ADR-103: `subtask_products.position` для стабильного порядка
- **Статус:** Принято
- **Driver:** AC-008 — «При добавлении услуги в неё автоматически попадают товары её типового комплекта из каталога, с количеством и текущей ценой. Товары можно добавлять, удалять, менять количество (в том числе дробное) и цену. Цены в заказе может менять и исполнитель (source: D10, D3, D19, D33).»
- **Контекст:** `subtask_products` без порядковой колонки, id — `gen_random_uuid()` → порядок строк случаен.
- **Решение:** `ADD COLUMN position int NOT NULL DEFAULT 0` в `0006` + backfill существующих строк `row_number() OVER (PARTITION BY subtask_id ORDER BY product_id, id)` (`design-sql.md §Subtask Logs Migration`); комплект — порядок `service_products.position`, новый товар — max+1. Поле `position` отдаётся в `CardSubtaskProduct`; перестановки нет (non-goal).
- **Альтернативы:** `created_at` (комплект вставляется одним `now()` → ничья).
- **Риски:** в dev/seed-БД порядок уже существующих товаров после backfill — по `product_id` (исходного порядка нет); проверка — IT-020.

### ADR-104: Модуль `task-card.js`, deep-link `/?task=N`
- **Статус:** Принято
- **Driver:** AC-001 — «Клик по карточке на доске или «+ Заказ» открывает большое окно поверх доски. В нём: название, описание, клиент, колонка, ответственный, срок, признак оплаты, оборудование, услуги с товарами и журналом, итог. По ссылке вида `/board?task=N` доска открывается сразу с карточкой заказа `#N` (source: D25, SD-002).»
- **Контекст:** доска на `/` (ADR-100 фазы 5), `/board` нет; `board.js` — 815 строк.
- **Решение:** deep-link — `/?task=N` (отклонение от буквы AC: путь доски, смысл тот же). Карточка — отдельный модуль `web/ssr/_shared/js/task-card.js` (`KT.taskCard`), `board.js` делегирует; модалка `modal-xl` + `modal-fullscreen-lg-down` (adaptive:min). `parseFilters` игнорирует `task`; открытие/закрытие — `history.replaceState` с сохранением фильтров.
- **Альтернативы:** расширять `board.js`; страница `/tasks/:id` (SD-002 b).
- **Риски:** справочники — только серверный поиск (`/services?q=`, `/products?q=`), без фоновой выгрузки (урок фазы 5).

### ADR-105: Клиентская очередь запросов на строку (self-race при автосохранении)
- **Статус:** Принято
- **Driver:** AC-014 — «Если двое одновременно меняют одну услугу или товар, сохраняется последнее изменение. Склад при этом остаётся согласован с журналом движений (source: D28, D34).»
- **Контекст:** mobile-review H-01/M-03: PUT строки — полная замена без `version`, сохранение по `change` (SD-008); два PUT одной строки в полёте могут молча откатить поле.
- **Решение:** `task-card.js` — `rowQueue`: не более одного запроса на строку в полёте; правки во время полёта копятся в `pending` и уходят одним следующим PUT; тело собирается в момент отправки из последнего применённого `card` ⊕ `pending` (не из DOM/кэша на момент `change`). Ошибка → `pending` сброс, значения из `card`. Разные строки — параллельно (межпользовательский LWW — ADR-002). Проверки — OC-022, OC-023.
- **Альтернативы:** серверный guard (`client_seq` в теле, stale → skip) — новый контракт и колонка, от устаревшего тела не спасает; PATCH по полю — дробит `stock_movements`.
- **Риски:** две вкладки одного пользователя не сериализуются — это межпользовательский LWW (AC-014), принято.

## Summary ключевых решений
ADR-001 card-ответ · ADR-002 без version · ADR-003 Codes 16-22 · ADR-004 оборудование 2 запроса · ADR-005 итоги SQL · ADR-006 plain-описание · ADR-007 RESTRICT · ADR-100 Lock Order · ADR-101 фикстуры API · ADR-102 акты SSR · ADR-103 position · ADR-104 task-card.js · ADR-105 очередь PUT на строку — все Принято.

## Security
Новых зависимостей нет (ADR-006). XSS: `description_html` пишет только сервер (`descriptionToHTML`) и seed-demo; прочие строки — `esc()` в JS / `html/template`. IDOR: строка блокируется с `task_id`/`subtask_id` в WHERE (404). Журнал — автор или admin; оплата/удаление заказа — as-is. Печать — `RequireSSR`. Чужое оборудование — 400 под `LockTask`.

## Monitoring & Alerting Spec
Spans `board`: `tasks.card`, `subtasks.mutate`, `subtask_products.mutate`, `task_equipment.mutate`, `subtask_logs.mutate`, `tasks.print` (+as-is `stock.reconcile`); log fields `trace_id, task_id, subtask_id|product_id, op`. Алерты — **deferred** (MVP: прод-мониторинг ставит `/infra-bootstrap`, issue при инфра-фазе); инвариант склада — IT-001.

## Test Strategy
`design-tests.md §Coverage AC`: каждый AC → ≥1 тест; concurrency IT-010…013, OC-015/016/022/023; склад IT-001; TD-4 LV-170…174.

## Performance Budget
Deferred (MVP): цель PRD p95 < 500 мс на сохранение шапки/строки; load-test не пишется, issue — при инфра-фазе.

## Runbook (post-deploy)
Deferred (MVP): `make migrate-up` (0006), задать `FIRM_*`, smoke — карточка seed-demo, добавить услугу в «В работе», сверить «Остатки».

## Rollout Strategy
Deferred (MVP): без feature-flag, выкатывается целиком.

## Verification
`go test ./internal/features/board/... -tags integration`; fuzz FZ-001…003; `api_conventions_check.py`, `go_naming_check.py`; E2E `order-card.spec.ts order-print.spec.ts board.spec.ts board-guards.spec.ts stock.spec.ts --project=chromium --headed --workers=1` (прогон — оркестратор).
