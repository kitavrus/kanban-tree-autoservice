<!-- AUTO-GENERATED spec_slice.py (profile: light) из draft-spec.md — НЕ редактировать; полная спека — источник истины, гейты работают только с ней -->
# Draft Spec: Перемещения — ядро (документ, оборудование клиентов, местоположение)

**Tier:** M · **Совместимость:** MVP · **Режим:** new

**Дата:** 2026-10-07 | **Статус:** draft → final (после подтверждения пользователя) | **Tier:** M (верхняя граница)
**Совместимость:** MVP (DS-027) — живых данных нет: таблицы и адреса меняем свободно, без переходного периода
**UI-scope:** swagger:no | adaptive:min | themes:both (DS-028)
**Источник:** мастер-спека `docs/features/transfers/draft-spec.md` (§4.1, §4.2, §4.4, §4.7, §5, §6.1–§6.8, §6.14, §6.15, §8.1, §8.3) + её журнал `docs/features/transfers/draft-spec-decisions.md`
**Часть 3 из 6 мастер-спеки `transfers`; зависит от: —** (независима от частей 1 и 2). Номера DS и TS — мастер-идентификаторы, не перенумерованы.

> После мержа этой части: экран `/transfers` (только администратор) перемещает между складами товары И оборудование клиентов одним документом; местоположение — одно на единицу оборудования, в карточке заказа вместо кнопок «Отправить / Принять / Отменить отправку» — бейдж местоположения и ссылка на документ. Чего ещё нет: направлений с клиентом и поставщиком, исполнителя на экране, цен, отката «Получено». Принять оборудование от клиента по-прежнему можно только добавлением единицы в заказ с выбором «Склада приёма» (как сейчас) — документ «клиент → склад» заменит это в части 4.


## §1. Суть задачи
Перемещение между складами становится общим документом для товаров и оборудования клиентов; местоположение оборудования переезжает со строки заказа на саму единицу. Для администратора; исполнитель видит результат в карточке заказа и на доске.

**Бизнес (что делает пользователь/процесс):**
1. Администратор создаёт документ «склад → склад» со строками двух видов: товар (количество) и единица оборудования клиента (§8.1) — DS-001, DS-010.
2. Отправляет, принимает (товар — с фактом), отменяет документ — три шага, как у перемещения товаров сейчас (§5).
3. В карточке заказа любой пользователь видит, где единица: «у клиента» / «на складе X» / «в пути → Y», и ссылку на документ (§8.2) — DS-003, DS-008.
4. Доска фильтруется по складу и показывает бейдж «в пути» по новому местоположению — DS-046.

**Логика (правила системы):**
1. Один документ со смешанными строками; вторая сущность не заводится — DS-001.
2. Строка-единица — только оборудование клиента (`equipment.client_id` NOT NULL); «наше оборудование» — товар каталога с количеством — DS-005, DS-009, DS-010.
3. Местоположение — одно на единицу: `s:at_client`, `s:at_warehouse`, `s:in_transit`; меняется только документом (и добавлением единицы «у клиента» в заказ с выбором склада приёма — временно, до части 4) — DS-008, DS-003.
4. Единица — только в одном незавершённом документе (заявка или в пути); занимается при добавлении в заявку — DS-019, DS-041.
5. Отправить единицу можно только со склада, где она числится — DS-036.
6. Единицы принимаются целиком вместе с документом; факт вводится только по товарам — DS-037.
7. Единицу в незавершённом документе нельзя убрать из заказа; единицу со строками документов нельзя удалить из справочника; склад с оборудованием или незавершёнными документами нельзя архивировать — DS-026.
8. Время: хранение UTC (`timestamptz`); показ — как на остальных экранах проекта на момент мержа (после части 2 `user-timezone` — по поясу пользователя; DS-031 — контекст, реализуется в `user-timezone`). Фильтров по датам в этой части нет.
9. Конкурентный доступ (краткий дубль §3):
   - правка и переходы документа → optimistic lock `f:version` + 409 — DS-048;
   - остатки товара → `FOR UPDATE` строк `t:product_stock` в порядке (product_id, warehouse_id) — DS-048;
   - занятие единицы документом → guard-UPDATE `f:active_transfer_id IS NULL` — DS-041;
   - смена местоположения → guard под `FOR UPDATE` строки `t:equipment` — DS-036.

**Техника (API/данные/интеграции):**
1. Переименование: `x:stock_transfers` → `t:transfers`, `x:stock_transfer_items` → `t:transfer_items`, пакет `internal/features/stocktransfers` → `internal/features/transfers`, API `x:/api/v1/stock-transfers` → `e:/api/v1/transfers`, страница `x:/stock-transfers` → `/transfers`; старые адреса убираются — DS-030.
2. `t:transfer_items`: тип строки `f:kind`, `f:equipment_id` — DS-001.
3. `t:equipment`: `f:location_status`, `f:warehouse_id`, `f:active_transfer_id`; с `t:task_equipment` уходят `x:warehouse_id`, `x:status`, `x:transit_from_warehouse_id`, `x:transit_to_warehouse_id`; таблица `x:task_equipment_movements` удаляется — DS-008, DS-029.
4. Новый `e:GET /api/v1/equipment` — поиск единиц для строки документа — DS-057.
5. Удаляются `x:POST /api/v1/tasks/:id/equipment/:equipment_id/send`, `x:…/receive`, `x:…/cancel-transit` — DS-003.
6. Одна миграция с переносом местоположения (DS-054) и пересоздание seed-demo (DS-029).

**UI (графическая часть):**
1. «Работа → Перемещения» (`/transfers`), только администратор: список + документ, строки «+ Товар» и «+ Оборудование» (§8.1).
2. Карточка заказа, блок «Оборудование»: бейдж местоположения и ссылка на документ вместо трёх кнопок (§8.2).
3. Доска: фильтр «Склад» и бейдж «в пути» — без изменения вида, новый источник данных.

Out-of-scope:
1. Направления «клиент → склад», «склад → клиент», «поставщик → склад», обязательный заказ, колонка `f:direction` — часть 4 `transfers-partners` (DS-002, DS-004, DS-012).
2. Исполнитель на экране «Перемещения» — часть 4 (DS-018); в этой части экран и API — только администратор, как сейчас.
3. Снятие выбора «Склад приёма» при добавлении единицы в заказ — часть 4 (DS-045): до появления документа «клиент → склад» это единственный способ принять оборудование.
4. Цены и сумма накладной — часть 5 `transfers-sales` (DS-011, DS-014).
5. «Отменить проведение», удаление документа, запрет удаления заказа с документами — часть 6 `transfers-revert` (DS-017, DS-025); в этой части «Получено» и «Отменён» окончательные, как сейчас.
6. Фильтр «период» — часть 4 (DS-034).
7. Мягкое удаление — отдельная задача вне мастер-спеки (DS-022).


## §2. Интеграции и master-data
Внешних интеграций нет.

Справочники (master-data, таблицей):
| Справочник | Источник правды | Синхронизация | Где используется |
|---|---|---|---|
| Склады (`t:warehouses`) | фича `warehouses` (D45) | нет | `f:from_warehouse_id`, `f:to_warehouse_id`, `f:warehouse_id` единицы |
| Товары (`t:products`) | фича `catalog` | нет | товарные строки |
| Оборудование клиентов (`t:equipment`) | фича `clients` (заводится вручную, DS-009) | нет | строки-единицы, местоположение |


## §3. Нефункциональные требования

**Конкурентность (К-3 — по операциям):**
1. **Правка и переходы документа (`e:PUT`, send, receive, cancel):**
   - optimistic lock: обязательный `f:version`; строка `t:transfers` берётся `FOR UPDATE` — DS-048
   - устаревшая версия → 409 `TRANSFER_VERSION_CONFLICT`; версия сверяется ДО статуса; недопустимый переход → 409 `TRANSFER_STATUS_CONFLICT` — DS-048
2. **Остатки товара (send, receive, cancel):**
   - Lock Order: `t:transfers` FOR UPDATE → строки `t:product_stock` FOR UPDATE в порядке (product_id, warehouse_id) — as-is D50, DS-048
   - отправка в минус разрешена, ответ несёт `f:warnings` (as-is D53)
3. **Занятие единицы оборудования документом (create, update):**
   - guard-UPDATE `UPDATE equipment SET active_transfer_id = :id WHERE id = :eq AND active_transfer_id IS NULL`; 0 строк → 409 `EQUIPMENT_IN_TRANSFER` с номером мешающего документа — DS-019, DS-041
   - Lock Order: после `t:product_stock` — строки `t:equipment` FOR UPDATE в порядке id
4. **Смена местоположения единицы (send, receive, cancel):**
   - guard по фактическому местоположению под блокировкой строки `t:equipment`; не совпало → 409 `TRANSFER_EQUIPMENT_NOT_AT_SOURCE` — DS-036
5. **Добавление единицы в заказ с приёмом на склад (`e:POST /api/v1/tasks/:id/equipment`) одновременно с документом:**
   - склад назначается guard-UPDATE'ом только единице `s:at_client` без `f:active_transfer_id`; иначе местоположение не трогается — DS-008

**Аудит:**
- кто и когда: `f:created_by`, `f:sent_by`, `f:received_by`, `f:cancelled_by` + `*_at` на документе — as-is
- движения остатков — `t:stock_movements` с `f:transfer_id` — as-is
- журнал `x:task_equipment_movements` удаляется: историю единицы ведут документы — DS-029
- отдельной таблицы аудита и аудита чтения нет (zap) — DS-051

**Время/таймзоны (К-13):**
- хранение: UTC, `timestamptz`
- показ: общим форматтером проекта (после части 2 — по поясу пользователя; DS-031 — контекст, реализуется в `user-timezone`)
- источник времени: сервер БД (`now()`); порядок событий — по `f:id` документа
- диапазонов дат в фильтрах и отчётах в этой части нет

**Метрики/таймстемпы:**
- `f:created_at`, `f:sent_at`, `f:received_at`, `f:cancelled_at`; отчётов и алертов нет

**Offline / деградация / восстановление:**
- offline-режима нет; после reload документ читается с сервера (`?id=N`), несохранённые правки заявки теряются
- повтор действия после сбоя сети безопасен: вторая попытка получит 409 по версии или статусу; ключа идемпотентности нет, дубль чисто товарной заявки при двойном создании — риск принят (кнопка блокируется) — DS-055

**Логирование и чувствительные данные:**
- slog: мутации документа (id, действие, пользователь); чувствительных данных нет

**Формат ошибок:**
- только `pkg/errorspkg`: `{code, message, details}`; 400 → `details.fields[]` (`name`, `message`)

**Abuse cases (вход для adversarial E2E TD-4):**
1. Исполнитель вызывает любой `e:/api/v1/transfers` или открывает `/transfers` → 403 (as-is admin-guard).
2. Одна единица подставлена в две заявки параллельными запросами → ровно один успех, второй 409 `EQUIPMENT_IN_TRANSFER`.
3. В строке `kind: equipment` передан `qty_sent`, в строке `kind: product` — `equipment_id` → 400, документ не создан.
4. Двойной клик «Принять» → второй запрос 409, остаток не удваивается, единица не «принимается» дважды.
5. Отправка документа с единицей, которую уже перевезли другим документом → 409, остатки не изменились.

**Применённые zap-дефолты** (zap-list — стандарты проекта из CLAUDE.md, применяются автоматически, без вопросов; в §9 — источник `zap`):
- 404 / 403 / 409 — стандартные статусы; ошибки только `pkg/errorspkg` (zap)
- синхронная обработка, без очередей (zap)
- отдельный аудит-лог не заводится (zap) — DS-051
- миграции — только `make migrate-up` (zap)


## §4. Данные
Легенда колонки «Обяз.»: `да` — клиент обязан передать при создании/правке; `нет` — опционально; `авто` — заполняет система (PK, дефолты, таймстемпы, значения из URL/сессии); `условно` — зависит от правила, оно указано в Описании. NULL — DDL-факт, Обяз. — семантика заполнения.

Миграция — одна новая, режим MVP: переименование и ALTER без сохранения совместимости (DS-027, DS-029, DS-030).

### §4.1. `t:transfers`
Документ «Перемещение» (шапка). Переименование `x:stock_transfers`; состав колонок в этой части не меняется (колонки направления и контрагентов — часть 4, отката — часть 6).

| Поле | Тип | NULL | Default | Обяз. | Описание | DS |
|---|---|---|---|---|---|---|
| `f:id` | bigserial | нет | sequence | авто | PK, сквозной номер #N (as-is) | DS-001 |
| `f:from_warehouse_id` | uuid | нет | — | да | FK → `t:warehouses(id)` `ON DELETE RESTRICT`; активный склад (as-is) | DS-030 |
| `f:to_warehouse_id` | uuid | нет | — | да | FK → `t:warehouses(id)` `ON DELETE RESTRICT`; активный склад, не равен `f:from_warehouse_id` (as-is) | DS-030 |
| `f:status` | text | нет | `requested` | авто | `s:requested`, `s:in_transit`, `s:received`, `s:cancelled` (as-is) | DS-048 |
| `f:note` | text | нет | `''` | нет | Примечание, 0–1000 символов (as-is) | DS-049 |
| `f:created_by` | uuid | нет | — | авто | FK → `t:users(id)` `ON DELETE RESTRICT`, из сессии (as-is) | DS-048 |
| `f:sent_by` | uuid | да | — | авто | FK → `t:users(id)` `ON DELETE RESTRICT` (as-is) | DS-048 |
| `f:sent_at` | timestamptz | да | — | авто | Момент отправки (as-is) | DS-048 |
| `f:received_by` | uuid | да | — | авто | FK → `t:users(id)` `ON DELETE RESTRICT` (as-is) | DS-048 |
| `f:received_at` | timestamptz | да | — | авто | Момент приёмки (as-is) | DS-048 |
| `f:cancelled_by` | uuid | да | — | авто | FK → `t:users(id)` `ON DELETE RESTRICT` (as-is) | DS-048 |
| `f:cancelled_at` | timestamptz | да | — | авто | Момент отмены (as-is) | DS-048 |
| `f:created_at` | timestamptz | нет | `now()` | авто | (as-is) | DS-048 |
| `f:updated_at` | timestamptz | нет | `now()` | авто | (as-is) | DS-048 |
| `f:version` | int | нет | `1` | авто | Optimistic lock, +1 на каждую мутацию (as-is) | DS-048 |

Ключи и индексы:
- PK: `f:id`
- UNIQUE: нет
- `i:transfers_status_idx` (`f:status`) — фильтр списка (переименование `x:stock_transfers_status_idx`)
- `i:transfers_from_warehouse_id_idx`, `i:transfers_to_warehouse_id_idx` — фильтр «склад», «в пути» на остатках, запрет архивирования склада (переименование)
- CHECK `i:transfers_distinct_warehouses` — `from_warehouse_id <> to_warehouse_id` (переименование `x:stock_transfers_distinct_warehouses`)

Связи (одна связь = одна строка):
- 1→N `t:transfer_items` — CASCADE — строки документа
- 1→N `t:stock_movements` (`f:transfer_id`) — SET NULL — FK переезжает на переименованную таблицу, набор причин движений не меняется
- 1→N `t:equipment` (`f:active_transfer_id`) — SET NULL — занятие единицы
- N→1 `t:warehouses`, `t:users` — RESTRICT

Поведение при действиях (одно действие = одна строка):
- Создание: только admin; статус `s:requested`; единицы строк занимаются (`f:active_transfer_id`) — DS-019
- Изменение: только в `s:requested`; строки заменяются целиком; снятые единицы освобождаются, новые занимаются — DS-048
- Отправка (`s:requested` → `s:in_transit`): товар списывается с источника (`s:transfer_out`), единицы → `s:in_transit` — одна транзакция — DS-036
- Приёмка (`s:in_transit` → `s:received`): остаток получателя растёт на факт (`s:transfer_in`, при недостаче `s:transfer_loss`), единицы → `s:at_warehouse` на складе назначения и освобождаются — одна транзакция — DS-037
- Отмена (`s:requested` или `s:in_transit` → `s:cancelled`): из «В пути» товар возвращается (`s:transfer_return`), единицы → склад отправления; единицы освобождаются — DS-019
- Удаление: endpoint'а нет (появится в части 6, DS-025)

Инварианты (правило → чем держится → DS):
- склады различаются → CHECK `i:transfers_distinct_warehouses` + валидатор (400 `same_warehouse`) → DS-049
- версия растёт на каждую мутацию → `UPDATE … SET version = version + 1` под `FOR UPDATE` → DS-048

### §4.2. `t:transfer_items`
Строки документа: товар с количеством либо единица оборудования клиента. Переименование `x:stock_transfer_items` + тип строки и единица. Колонка цены — часть 5.

| Поле | Тип | NULL | Default | Обяз. | Описание | DS |
|---|---|---|---|---|---|---|
| `f:id` | uuid | нет | `gen_random_uuid()` | авто | PK (as-is) | DS-001 |
| `f:transfer_id` | bigint | нет | — | авто | FK → `t:transfers(id)` `ON DELETE CASCADE`, из URL (as-is) | DS-001 |
| `f:position` | int | нет | — | авто | Порядок строки = порядок в запросе, с 1 (as-is) | DS-049 |
| **`f:kind`** | text | нет | — | да | НОВОЕ. Тип строки: `s:product`, `s:equipment`; существующие строки при миграции → `s:product` | DS-001 |
| `f:product_id` | uuid | да | — | условно | FK → `t:products(id)` `ON DELETE RESTRICT`. ИЗМЕНЕНО: было NOT NULL. Обязателен при `s:product`; иначе NULL | DS-001 |
| **`f:equipment_id`** | uuid | да | — | условно | НОВОЕ. FK → `t:equipment(id)` `ON DELETE RESTRICT`. Обязателен при `s:equipment`; иначе NULL | DS-010 |
| `f:qty_sent` | numeric(12,3) | да | — | условно | ИЗМЕНЕНО: было NOT NULL. Количество к отправке, > 0, до 3 знаков после запятой, максимум 999999999.999. Обязательно при `s:product`; при `s:equipment` — NULL | DS-049 |
| `f:qty_received` | numeric(12,3) | да | — | условно | Факт приёмки товарной строки, 0 ≤ факт ≤ `f:qty_sent` (as-is); при `s:equipment` — NULL | DS-037 |

Ключи и индексы:
- PK: `f:id`
- UNIQUE (`f:transfer_id`, `f:product_id`) — дубль товара в документе запрещён (as-is)
- UNIQUE (`f:transfer_id`, `f:equipment_id`) — НОВОЕ: дубль единицы в документе запрещён
- `i:transfer_items_product_id_idx` (`f:product_id`) — RESTRICT-проверка товара, «в пути» на остатках (переименование)
- `i:transfer_items_equipment_id_idx` (`f:equipment_id`) — НОВОЕ: история единицы, запрет удаления единицы
- CHECK `i:transfer_items_kind_check` — НОВОЕ: при `s:product` — `f:product_id` и `f:qty_sent` заполнены, `f:equipment_id` NULL; при `s:equipment` — `f:equipment_id` заполнен, `f:product_id`, `f:qty_sent`, `f:qty_received` NULL

Связи (одна связь = одна строка):
- N→1 `t:transfers` — CASCADE — строки удаляются с документом
- N→1 `t:products` — RESTRICT
- N→1 `t:equipment` — RESTRICT — единицу со строками документов не удалить из справочника (DS-026)

Поведение при действиях (одно действие = одна строка):
- Создание/замена: вместе с документом, только в `s:requested`; не более 200 строк суммарно — DS-049
- Приёмка: `f:qty_received` по товарным строкам; строка без записи в запросе — факт = `f:qty_sent`; строки-единицы факта не имеют — DS-037

Инварианты (правило → чем держится → DS):
- у строки ровно один предмет: товар либо единица → CHECK `i:transfer_items_kind_check` → DS-001
- единица отправляется только со склада отправления документа → guard сервиса под `FOR UPDATE` (409 `TRANSFER_EQUIPMENT_NOT_AT_SOURCE`) → DS-036
- факт ≤ отправленного → валидатор сервиса (400 `exceeds_sent`) + CHECK (as-is) → DS-049

### §4.3. `t:equipment` (общая таблица, чужая — фича использует, не владеет)
Единицы оборудования клиентов; владелец — фича `clients`. Дельта: местоположение переезжает сюда с `t:task_equipment`.

| Поле | Тип | NULL | Default | Обяз. | Описание | DS |
|---|---|---|---|---|---|---|
| `f:id` | uuid | нет | `gen_random_uuid()` | авто | PK (as-is) | DS-008 |
| `f:client_id` | uuid | нет | — | да | FK → `t:clients(id)` `ON DELETE CASCADE` (as-is); остаётся NOT NULL — единиц без клиента нет | DS-005 |
| `f:type_id` | uuid | нет | — | да | FK → `t:equipment_types(id)` `ON DELETE RESTRICT` (as-is) | DS-008 |
| `f:brand` | text | нет | `''` | нет | Марка (as-is) | DS-008 |
| `f:model` | text | нет | — | да | Модель, непустая (as-is) | DS-008 |
| `f:sku` | text | нет | `''` | нет | Артикул — строка (as-is) | DS-008 |
| `f:serial` | text | нет | `''` | нет | Серийный номер — строка (as-is) | DS-008 |
| `f:specs` | jsonb | нет | `'[]'` | нет | Характеристики, массив (as-is) | DS-008 |
| `f:note` | text | нет | `''` | нет | Примечание (as-is) | DS-008 |
| `f:created_at` | timestamptz | нет | `now()` | авто | (as-is) | DS-008 |
| `f:updated_at` | timestamptz | нет | `now()` | авто | (as-is) | DS-008 |
| **`f:location_status`** | text | нет | `at_client` | авто | НОВОЕ. Местоположение: `s:at_client`, `s:at_warehouse`, `s:in_transit`; меняют только переходы документа | DS-008 |
| **`f:warehouse_id`** | uuid | да | — | авто | НОВОЕ. FK → `t:warehouses(id)` `ON DELETE RESTRICT`. Заполнен только при `s:at_warehouse` | DS-008 |
| **`f:active_transfer_id`** | bigint | да | — | авто | НОВОЕ. FK → `t:transfers(id)` `ON DELETE SET NULL`. Незавершённый документ (`s:requested` или `s:in_transit`), в котором стоит единица; NULL — свободна | DS-019, DS-041 |

Ключи и индексы:
- PK: `f:id`
- `i:equipment_client_id_idx`, `i:equipment_type_id_idx` — as-is
- `i:equipment_warehouse_id_idx` (`f:warehouse_id`) — фильтр доски по складу, запрет архивирования склада
- `i:equipment_active_transfer_id_idx` (`f:active_transfer_id`) — освобождение единиц документа
- CHECK `i:equipment_location_check` — `s:at_warehouse` ⇔ `f:warehouse_id` заполнен; `s:in_transit` ⇒ `f:active_transfer_id` заполнен

Связи (одна связь = одна строка):
- N→1 `t:warehouses` — RESTRICT
- N→1 `t:transfers` (`f:active_transfer_id`) — SET NULL
- 1→N `t:transfer_items` — RESTRICT

Поведение при действиях (одно действие = одна строка):
- Создание единицы: `s:at_client`, склад пуст — DS-008
- Добавление единицы в заказ (`e:POST /api/v1/tasks/:id/equipment`, временно до части 4): единица `s:at_client` без `f:active_transfer_id` → `s:at_warehouse`, склад — переданный `warehouse_id` либо первый активный (as-is D56); единица уже `s:at_warehouse` или `s:in_transit` — местоположение не меняется — DS-008
- Отправка документа: `s:at_warehouse` (склад отправления) → `s:in_transit`, `f:warehouse_id` = NULL — DS-036
- Приёмка: → `s:at_warehouse` (склад назначения); `f:active_transfer_id` = NULL — DS-037
- Отмена документа «в пути»: возврат на склад отправления (`s:at_warehouse`); `f:active_transfer_id` = NULL — DS-019
- Удаление единицы из справочника: 409 `EQUIPMENT_HAS_TRANSFERS`, если есть хоть одна строка документа — DS-026
- Миграция (DS-029, DS-054): завершённый заказ = заказ в ПОСЛЕДНЕЙ колонке доски — колонка с наибольшим `columns.position` (`migrations/0005_board.up.sql:11`; порядок колонок — индекс `columns_position_idx (position, id)`, там же `:15`); SQL-условие миграции: `tasks.column_id = (SELECT id FROM columns ORDER BY position DESC, id DESC LIMIT 1)` (при равных `position` — колонка с большим `id`). Единица, стоящая хотя бы в одном НЕзавершённом заказе, → `s:at_warehouse`, склад — из её строки `t:task_equipment` самого свежего незавершённого заказа (наибольший `tasks.id`): `x:warehouse_id`, а для «в пути» — `x:transit_from_warehouse_id`. Единица, стоящая только в завершённых заказах, → `s:at_client`. Единица без заказов → `s:at_client`. Края: колонка на доске одна — она же последняя, все заказы завершённые, все единицы → `s:at_client`; заказа без колонки не бывает (`tasks.column_id` NOT NULL, `migrations/0005_board.up.sql:27`). Признак `columns.is_final` в миграции не используется

Инварианты (правило → чем держится → DS):
- одно местоположение на единицу → колонки на `t:equipment` + `i:equipment_location_check` → DS-008
- единица только в одном незавершённом документе → guard-UPDATE `f:active_transfer_id IS NULL` → DS-019
- отправить можно только оттуда, где единица числится → guard под `FOR UPDATE` (409 `TRANSFER_EQUIPMENT_NOT_AT_SOURCE`) → DS-036

Используется в модулях (used by):
| Модуль | Ключ-дискриминатор (если есть) | Роль (writer/reader) | Что делает | Путь |
|---|---|---|---|---|
| transfers (эта часть) | — | writer | пишет `f:location_status`, `f:warehouse_id`, `f:active_transfer_id` | `internal/features/transfers` |
| clients | — | writer | CRUD единицы, история | `internal/features/clients` |
| board | — | reader + writer | карточка заказа, фильтр доски по складу, склад по умолчанию строки товара; пишет склад при добавлении единицы «у клиента» в заказ (временно, до части 4) | `internal/features/board` |

### §4.4. `t:task_equipment` (общая таблица, чужая — фича использует, не владеет)
Оборудование заказа; владелец — фича `board`. Дельта: местоположение уезжает на `t:equipment`.

| Поле | Тип | NULL | Default | Обяз. | Описание | DS |
|---|---|---|---|---|---|---|
| `f:task_id` | bigint | нет | — | авто | FK → `t:tasks(id)` `ON DELETE CASCADE`, часть PK (as-is) | DS-008 |
| `f:equipment_id` | uuid | нет | — | да | FK → `t:equipment(id)` `ON DELETE RESTRICT`, часть PK (as-is) | DS-008 |
| `f:completeness` | text | нет | `''` | нет | Комплектность (as-is) | DS-008 |
| `f:condition` | text | нет | `''` | нет | Состояние (as-is) | DS-008 |
| `x:warehouse_id` | uuid | — | — | — | УДАЛЯЕТСЯ: переезжает на `t:equipment` | DS-008 |
| `x:status` | text | — | — | — | УДАЛЯЕТСЯ (`s:at_warehouse`, `s:in_transit`): заменён `equipment.location_status` | DS-008 |
| `x:transit_from_warehouse_id` | uuid | — | — | — | УДАЛЯЕТСЯ: маршрут «в пути» берётся из документа | DS-008 |
| `x:transit_to_warehouse_id` | uuid | — | — | — | УДАЛЯЕТСЯ: маршрут «в пути» берётся из документа | DS-008 |

Ключи и индексы:
- PK (`f:task_id`, `f:equipment_id`) — as-is
- удаляются `x:task_equipment_warehouse_id_idx`, `x:task_equipment_transit_to_warehouse_id_idx`, CHECK `x:task_equipment_status_check`, CHECK `x:task_equipment_location_check`

Поведение при действиях (одно действие = одна строка):
- Добавление единицы в заказ: склад приёма по-прежнему выбирается, но записывается на единицу (`t:equipment`), а не на строку заказа; правило — §4.3 (снятие выбора склада — DS-045, контекст, реализуется в `transfers-partners`)
- Удаление единицы из заказа: 409 `EQUIPMENT_IN_TRANSFER`, пока единица стоит в незавершённом документе — DS-026
- Таблица `x:task_equipment_movements` (журнал `s:received`, `s:sent`, `s:arrived`, `s:cancelled`) удаляется целиком; историю ведут документы — DS-029

Используется в модулях (used by):
| Модуль | Ключ-дискриминатор (если есть) | Роль (writer/reader) | Что делает | Путь |
|---|---|---|---|---|
| transfers (эта часть) | — | — | в этой части не читает (проверка «единица входит в оборудование заказа» — часть 4) | `internal/features/transfers` |
| board | — | writer | оборудование карточки заказа | `internal/features/board/task_equipment_repo.go`, `task_equipment_service.go` |
| clients | — | reader | история единицы по заказам | `internal/features/clients/history.go` |
| seeddemo | — | writer | демо-заказы | `internal/seeddemo/seeddemo.go` |


## §5. Статусная модель
![Статусная модель](draft-spec-status.svg)

Статусы документа `t:transfers.status` — те же четыре, что у перемещения товаров сейчас. Что можно в каждом статусе (всё остальное → 409 `TRANSFER_STATUS_CONFLICT`):
- `s:requested` («Заявка»): править, отправить, отменить
- `s:in_transit` («В пути»): принять, отменить
- `s:received` («Получено», терминальный в этой части; откат — часть 6)
- `s:cancelled` («Отменён», терминальный)

Детали переходов (guard = атомарное условие; что при гонке; каждый запрет = 409 + негативный тест):
| Из | В | Триггер | Кто | Guard / конкурентный исход | Side effects |
|---|---|---|---|---|---|
| — | `s:requested` | `e:POST /api/v1/transfers` | admin | единицы строк свободны (`f:active_transfer_id IS NULL`), иначе 409 `EQUIPMENT_IN_TRANSFER` | единицы занимаются документом |
| `s:requested` | `s:requested` | `e:PUT /api/v1/transfers/:id` | admin | `f:version` совпал, статус `s:requested`; проигравший — 409 `TRANSFER_VERSION_CONFLICT` | строки заменены; снятые единицы освобождены, новые заняты |
| `s:requested` | `s:in_transit` | `e:POST /api/v1/transfers/:id/send` либо `send: true` при создании | admin | версия + статус; каждая единица числится на складе отправления, иначе 409 `TRANSFER_EQUIPMENT_NOT_AT_SOURCE` | `s:transfer_out` −`f:qty_sent` на складе отправления; единицы → `s:in_transit`; `f:sent_by`, `f:sent_at` |
| `s:in_transit` | `s:received` | `e:POST /api/v1/transfers/:id/receive` | admin | версия + статус; второй одновременный запрос — 409 | `s:transfer_in` +`f:qty_sent`, при недостаче `s:transfer_loss`; единицы → `s:at_warehouse` на складе назначения и освобождаются; `f:received_by`, `f:received_at` |
| `s:requested` | `s:cancelled` | `e:POST /api/v1/transfers/:id/cancel` | admin | версия + статус | единицы освобождаются; движений нет; `f:cancelled_by`, `f:cancelled_at` |
| `s:in_transit` | `s:cancelled` | `e:POST /api/v1/transfers/:id/cancel` | admin | версия + статус | `s:transfer_return` +`f:qty_sent` на склад отправления; единицы → склад отправления и освобождаются |

Местоположение единицы `t:equipment.location_status` (DS-008) — производное: `s:at_client` → `s:at_warehouse` (добавление в заказ с приёмом на склад, временно до части 4) ⇄ `s:in_transit` (документ). Переход `s:at_warehouse` → `s:at_client` (возврат клиенту) появится в части 4; до неё единица, однажды принятая на склад, остаётся «на складе».


## §6. API

### §6.1. `e:GET /api/v1/transfers`
- Назначение: список документов для экрана «Перемещения».
- Authz: только admin; executor → 403 (расширение на исполнителя — часть 4).
- Идемпотентность/повтор: чтение, безопасно.
- Side effects: нет.

Request:

| Параметр | Где | Тип | Обязателен | Валидация |
|---|---|---|---|---|
| `f:status` | query | string | нет | `requested`, `in_transit`, `received`, `cancelled`; иное → 400 |
| `f:warehouse_id` | query | uuid | нет | склад отправления ИЛИ назначения; не uuid → 400 |
| `f:limit` | query | integer | нет | 1–500, по умолчанию 50; больше → 500 |
| `f:offset` | query | integer | нет | ≥ 0, по умолчанию 0 |

Фильтры комбинируются по «И». Сортировка: `f:id` по убыванию.

Response 200:

| Поле | Тип | Описание |
|---|---|---|
| `f:items` | array | элементы списка (поля ниже) |
| `f:items[].id` | integer | номер документа |
| `f:items[].status` | string | статус |
| `f:items[].from_warehouse_id` | uuid | склад отправления |
| `f:items[].from_warehouse_name` | string | его название |
| `f:items[].to_warehouse_id` | uuid | склад назначения |
| `f:items[].to_warehouse_name` | string | его название |
| `f:items[].note` | string | примечание |
| `f:items[].items_count` | integer | число строк (товары + единицы) |
| `f:items[].created_by_name` | string | автор |
| `f:items[].created_at` | datetime (UTC, RFC 3339) | создан |
| `f:items[].updated_at` | datetime (UTC, RFC 3339) | изменён |
| `f:items[].version` | integer | версия |
| `f:total` | integer | всего под фильтрами |
| `f:limit` | integer | применённый лимит |
| `f:offset` | integer | применённое смещение |

Ошибки:

| Код | Условие | Сообщение пользователю |
|---|---|---|
| 400 `VALIDATION` | `status: invalid` | «Некорректный статус» |
| 400 `VALIDATION` | `warehouse_id: invalid` | «Некорректное значение фильтра» |
| 401 `UNAUTHORIZED` | нет сессии | переход на вход |
| 403 `FORBIDDEN` | executor | «Недостаточно прав» |

Успех:
```bash
```
Ошибка (VALIDATION → 400):
```bash
```
DS: DS-030, DS-048.

### §6.2. `e:POST /api/v1/transfers`
- Назначение: создать документ «склад → склад»; опционально сразу отправить.
- Authz: только admin; executor → 403.
- Спец-механика: `send: true` — создать и отправить одним действием (as-is D51).
- Идемпотентность/повтор: ключа дедупликации нет; повтор создаст второй документ, но единицы уже заняты первым → 409 `EQUIPMENT_IN_TRANSFER`; для чисто товарных документов риск дубля принят (кнопка блокируется) — DS-055.
- Side effects: одна транзакция — `t:transfers`, `t:transfer_items`, `t:equipment.active_transfer_id`; при `send` — дополнительно эффекты §6.5.

Request:

| Параметр | Где | Тип | Обязателен | Валидация |
|---|---|---|---|---|
| `f:from_warehouse_id` | body | uuid | да | существующий активный склад; иначе 400 `required` / `invalid` / `not_found` / `archived` |
| `f:to_warehouse_id` | body | uuid | да | существующий активный склад; равен `f:from_warehouse_id` → 400 `same_warehouse` |
| `f:note` | body | string | нет | 0–1000 символов |
| `f:items` | body | array | да | 1–200 строк; пустой → 400 `required`; больше 200 → 400 `too_many` |
| `f:items[].kind` | body | string | да | `product`, `equipment`; иное → 400 `invalid` |
| `f:items[].product_id` | body | uuid | условно | обязателен при `product`; активный товар каталога; дубль в документе → 400 `duplicate`; при `equipment` передан → 400 `not_allowed` |
| `f:items[].equipment_id` | body | uuid | условно | обязателен при `equipment`; существующая единица; дубль → 400 `duplicate`; при `product` передан → 400 `not_allowed` |
| `f:items[].qty_sent` | body | decimal | условно | обязателен при `product`; > 0, до 3 знаков после запятой, ≤ 999999999.999; при `equipment` передан → 400 `not_allowed` |
| `f:send` | body | boolean | нет | по умолчанию `false` |

Response 201 — объект документа:

| Поле | Тип | Описание |
|---|---|---|
| `f:id` | integer | номер документа |
| `f:status` | string | статус |
| `f:from_warehouse_id` | uuid \| null | склад отправления |
| `f:from_warehouse_name` | string \| null | его название |
| `f:to_warehouse_id` | uuid \| null | склад назначения |
| `f:to_warehouse_name` | string \| null | его название |
| `f:note` | string | примечание |
| `f:created_by` | uuid | автор |
| `f:created_by_name` | string | имя автора |
| `f:sent_by` | uuid \| null | кто отправил |
| `f:sent_by_name` | string \| null | имя |
| `f:sent_at` | datetime \| null | когда отправлен |
| `f:received_by` | uuid \| null | кто принял |
| `f:received_by_name` | string \| null | имя |
| `f:received_at` | datetime \| null | когда принят |
| `f:cancelled_by` | uuid \| null | кто отменил |
| `f:cancelled_by_name` | string \| null | имя |
| `f:cancelled_at` | datetime \| null | когда отменён |
| `f:created_at` | datetime | создан |
| `f:updated_at` | datetime | изменён |
| `f:version` | integer | версия |
| `f:items` | array | строки (поля ниже) |
| `f:items[].id` | uuid | id строки |
| `f:items[].kind` | string | `product` \| `equipment` |
| `f:items[].product_id` | uuid \| null | товар |
| `f:items[].product_name` | string \| null | название товара |
| `f:items[].unit` | string \| null | единица измерения |
| `f:items[].equipment_id` | uuid \| null | единица оборудования |
| `f:items[].equipment_name` | string \| null | «тип марка модель» |
| `f:items[].equipment_serial` | string \| null | серийный номер |
| `f:items[].qty_sent` | string \| null | количество |
| `f:items[].qty_received` | string \| null | факт |
| `f:warnings` | array | предупреждения по остатку (всегда массив) |
| `f:warnings[].product_id` | uuid | товар |
| `f:warnings[].warehouse_id` | uuid | склад |
| `f:warnings[].warehouse_name` | string | название склада |
| `f:warnings[].name` | string | название товара |
| `f:warnings[].stock` | string | остаток после операции |
| `f:warnings[].min_stock` | string | минимум |
| `f:warnings[].kind` | string | `negative` \| `below_min` |

Ошибки:

| Код | Условие | Сообщение пользователю |
|---|---|---|
| 400 `VALIDATION` | `from_warehouse_id` / `to_warehouse_id`: `required`, `invalid`, `not_found`, `archived` | «Проверьте склад» |
| 400 `VALIDATION` | `to_warehouse_id: same_warehouse` | «Склады должны различаться» |
| 400 `VALIDATION` | `note: too_long` | «Слишком длинное примечание» |
| 400 `VALIDATION` | `items: required` / `too_many` | «Добавьте строки (не более 200)» |
| 400 `VALIDATION` | `items[i].kind`: `required`, `invalid` | «Некорректный тип строки» |
| 400 `VALIDATION` | `items[i].product_id`: `required`, `not_found`, `archived`, `duplicate`, `not_allowed` | «Проверьте товар» |
| 400 `VALIDATION` | `items[i].equipment_id`: `required`, `not_found`, `duplicate`, `not_allowed` | «Проверьте оборудование» |
| 400 `VALIDATION` | `items[i].qty_sent`: `required`, `invalid`, `not_positive`, `not_allowed` | «Количество должно быть больше нуля» |
| 403 `FORBIDDEN` | executor | «Недостаточно прав» |
| 409 `EQUIPMENT_IN_TRANSFER` | единица уже в незавершённом документе (`details.transfer_id`, `details.equipment_id`) | «Оборудование уже в перемещении #N» |
| 409 `TRANSFER_EQUIPMENT_NOT_AT_SOURCE` | при `send`: единица числится не на складе отправления | «Оборудование находится не на складе отправления» |

Успех:
```bash
  "from_warehouse_id": "5b0c1c0e-6a55-4c0e-9c37-3d2f6f1f7a11",
  "to_warehouse_id": "9e8d7c6b-5a49-4382-b170-6f5e4d3c2b1a",
  "note": "Принтер на диагностику в Москву + картриджи",
  "items": [
    {"kind": "equipment", "equipment_id": "0d9a4c1b-7e2f-4b63-9a51-2c8e5f7d3a90"},
    {"kind": "product", "product_id": "c7e2a9f4-1b3d-4e6a-8c2f-9d0b5a4e3f12", "qty_sent": "5"}
  ],
  "send": true
}'
```
Ошибка (EQUIPMENT_IN_TRANSFER → 409):
```bash
```
DS: DS-001, DS-010, DS-019, DS-041, DS-049, DS-055.

### §6.3. `e:GET /api/v1/transfers/:id`
- Назначение: один документ со строками.
- Authz: только admin; executor → 403.
- Идемпотентность/повтор: чтение, безопасно.
- Side effects: нет.

Request:

| Параметр | Где | Тип | Обязателен | Валидация |
|---|---|---|---|---|
| `f:id` | path | integer | да | 1–9223372036854775807; не число → 404 |

Response 200 — объект документа: полный перечень полей — таблица «Response 201» в §6.2 (тот же объект).

Ошибки:

| Код | Условие | Сообщение пользователю |
|---|---|---|
| 403 `FORBIDDEN` | executor | «Недостаточно прав» |
| 404 `TRANSFER_NOT_FOUND` | нет документа с таким id / id не число | «Перемещение не найдено» |

Успех:
```bash
```
Ошибка (TRANSFER_NOT_FOUND → 404):
```bash
```
DS: DS-030.

### §6.4. `e:PUT /api/v1/transfers/:id`
- Назначение: изменить заявку (шапка + строки целиком).
- Authz: только admin.
- Идемпотентность/повтор: повтор с той же `f:version` после успеха → 409 `TRANSFER_VERSION_CONFLICT`.
- Side effects: одна транзакция — `t:transfers`, полная замена `t:transfer_items`, освобождение снятых и занятие добавленных единиц (`t:equipment.active_transfer_id`).

Request:

| Параметр | Где | Тип | Обязателен | Валидация |
|---|---|---|---|---|
| `f:id` | path | integer | да | существующий документ |
| `f:version` | body | integer | да | ≥ 1; отсутствует → 400 `required`; не совпала → 409 |
| `f:from_warehouse_id` | body | uuid | да | существующий активный склад |
| `f:to_warehouse_id` | body | uuid | да | существующий активный склад; равен `f:from_warehouse_id` → 400 `same_warehouse` |
| `f:note` | body | string | нет | 0–1000 символов |
| `f:items` | body | array | да | 1–200 строк |
| `f:items[].kind` | body | string | да | `product`, `equipment` |
| `f:items[].product_id` | body | uuid | условно | при `product`; архивный товар, уже бывший в заявке, допустим (as-is D52); дубль → 400 `duplicate` |
| `f:items[].equipment_id` | body | uuid | условно | при `equipment`; дубль → 400 `duplicate` |
| `f:items[].qty_sent` | body | decimal | условно | при `product`; > 0, до 3 знаков, ≤ 999999999.999 |

Response 200 — объект документа (поля — таблица «Response 201» §6.2).

Ошибки:

| Код | Условие | Сообщение пользователю |
|---|---|---|
| 400 `VALIDATION` | `version: required` | «Обновите страницу» |
| 400 `VALIDATION` | `from_warehouse_id` / `to_warehouse_id`: `required`, `invalid`, `not_found`, `archived`, `same_warehouse` | «Проверьте склад» |
| 400 `VALIDATION` | `items: required` / `too_many` | «Добавьте строки (не более 200)» |
| 400 `VALIDATION` | `items[i].kind`, `items[i].product_id`, `items[i].equipment_id`, `items[i].qty_sent` — те же сообщения, что при создании (`required`, `invalid`, `not_found`, `duplicate`, `not_allowed`, `not_positive`) | по полю |
| 404 `TRANSFER_NOT_FOUND` | нет документа | «Перемещение не найдено» |
| 409 `TRANSFER_VERSION_CONFLICT` | версия устарела | «Перемещение уже изменено, обновите» |
| 409 `TRANSFER_STATUS_CONFLICT` | статус `in_transit` | «Перемещение уже отправлено — править нельзя» |
| 409 `TRANSFER_STATUS_CONFLICT` | статус `received` | «Перемещение уже получено — править нельзя» |
| 409 `TRANSFER_STATUS_CONFLICT` | статус `cancelled` | «Перемещение отменено — править нельзя» |
| 409 `EQUIPMENT_IN_TRANSFER` | добавленная единица занята другим документом | «Оборудование уже в перемещении #N» |

Успех:
```bash
```
Ошибка (TRANSFER_VERSION_CONFLICT → 409):
```bash
```
DS: DS-048, DS-019.

### §6.5. `e:POST /api/v1/transfers/:id/send`
- Назначение: отправить заявку («Заявка» → «В пути»).
- Authz: только admin.
- Идемпотентность/повтор: повтор → 409 (версия либо статус); остаток дважды не списывается.
- Side effects: одна транзакция — `t:transfers` (статус, `f:sent_by`, `f:sent_at`, версия), `t:product_stock` и `t:stock_movements` (`s:transfer_out`), `t:equipment` (`f:location_status` = `in_transit`, `f:warehouse_id` = NULL).

Request:

| Параметр | Где | Тип | Обязателен | Валидация |
|---|---|---|---|---|
| `f:id` | path | integer | да | существующий документ |
| `f:version` | body | integer | да | ≥ 1; отсутствует → 400 `required` |

Response 200 — объект документа (поля — таблица «Response 201» §6.2); `f:warnings` — товары, ушедшие в минус или ниже минимума на складе отправления.

Ошибки:

| Код | Условие | Сообщение пользователю |
|---|---|---|
| 400 `VALIDATION` | `version: required` | «Обновите страницу» |
| 404 `TRANSFER_NOT_FOUND` | нет документа | «Перемещение не найдено» |
| 409 `TRANSFER_VERSION_CONFLICT` | версия устарела | «Перемещение уже изменено, обновите» |
| 409 `TRANSFER_STATUS_CONFLICT` | статус `in_transit` | «Перемещение уже отправлено» |
| 409 `TRANSFER_STATUS_CONFLICT` | статус `received` | «Перемещение уже получено» |
| 409 `TRANSFER_STATUS_CONFLICT` | статус `cancelled` | «Перемещение отменено» |
| 409 `TRANSFER_EQUIPMENT_NOT_AT_SOURCE` | единица «у клиента» либо на другом складе (`details.equipment_id`) | «Оборудование находится не на складе отправления» |

Успех:
```bash
```
Ошибка (TRANSFER_EQUIPMENT_NOT_AT_SOURCE → 409):
```bash
```
DS: DS-036.

### §6.6. `e:POST /api/v1/transfers/:id/receive`
- Назначение: принять документ («В пути» → «Получено»); по товарам — с фактом.
- Authz: только admin.
- Спец-механика: строки-единицы принимаются целиком, факт по ним не вводится (DS-037).
- Идемпотентность/повтор: повтор → 409; остаток дважды не растёт.
- Side effects: одна транзакция — `t:transfers` (статус, `f:received_by`, `f:received_at`, версия), `t:transfer_items.qty_received`, `t:product_stock` и `t:stock_movements` (`s:transfer_in`, `s:transfer_loss`), `t:equipment` (`f:location_status` = `at_warehouse`, `f:warehouse_id` = склад назначения, `f:active_transfer_id` = NULL).

Request:

| Параметр | Где | Тип | Обязателен | Валидация |
|---|---|---|---|---|
| `f:id` | path | integer | да | существующий документ |
| `f:version` | body | integer | да | ≥ 1 |
| `f:items` | body | array | нет | факт по товарным строкам; строка без записи — факт = `qty_sent` |
| `f:items[].id` | body | uuid | да | id товарной строки этого документа; чужой → 400 `not_found`; строка-единица → 400 `not_allowed`; дубль → 400 `duplicate` |
| `f:items[].qty_received` | body | decimal | да | 0 ≤ значение ≤ `qty_sent`, до 3 знаков; больше → 400 `exceeds_sent` |

Response 200 — объект документа (поля — таблица «Response 201» §6.2).

Ошибки:

| Код | Условие | Сообщение пользователю |
|---|---|---|
| 400 `VALIDATION` | `version: required` | «Обновите страницу» |
| 400 `VALIDATION` | `items[i].id`: `not_found`, `not_allowed`, `duplicate` | «Проверьте строки приёмки» |
| 400 `VALIDATION` | `items[i].qty_received`: `invalid`, `negative` | «Факт не может быть отрицательным» |
| 400 `VALIDATION` | `items[i].qty_received: exceeds_sent` | «Факт больше отправленного» |
| 404 `TRANSFER_NOT_FOUND` | нет документа | «Перемещение не найдено» |
| 409 `TRANSFER_VERSION_CONFLICT` | версия устарела | «Перемещение уже изменено, обновите» |
| 409 `TRANSFER_STATUS_CONFLICT` | статус `requested` | «Перемещение ещё не отправлено» |
| 409 `TRANSFER_STATUS_CONFLICT` | статус `received` | «Перемещение уже получено» |
| 409 `TRANSFER_STATUS_CONFLICT` | статус `cancelled` | «Перемещение отменено» |
| 409 `STOCK_OVERFLOW` | остаток получателя вышел бы за numeric(12,3) (as-is) | «Слишком большое количество на складе» |

Успех:
```bash
```
Ошибка (VALIDATION → 400):
```bash
```
DS: DS-037, DS-049.

### §6.7. `e:POST /api/v1/transfers/:id/cancel`
- Назначение: отменить заявку или документ «в пути».
- Authz: только admin.
- Идемпотентность/повтор: повтор → 409.
- Side effects: одна транзакция — `t:transfers` (статус, `f:cancelled_by`, `f:cancelled_at`, версия); из «В пути» — `s:transfer_return` на склад отправления и возврат единиц туда же; единицы освобождаются.

Request:

| Параметр | Где | Тип | Обязателен | Валидация |
|---|---|---|---|---|
| `f:id` | path | integer | да | существующий документ |
| `f:version` | body | integer | да | ≥ 1 |

Response 200 — объект документа (поля — таблица «Response 201» §6.2).

Ошибки:

| Код | Условие | Сообщение пользователю |
|---|---|---|
| 400 `VALIDATION` | `version: required` | «Обновите страницу» |
| 404 `TRANSFER_NOT_FOUND` | нет документа | «Перемещение не найдено» |
| 409 `TRANSFER_VERSION_CONFLICT` | версия устарела | «Перемещение уже изменено, обновите» |
| 409 `TRANSFER_STATUS_CONFLICT` | статус `received` | «Перемещение уже получено — отменить нельзя» |
| 409 `TRANSFER_STATUS_CONFLICT` | статус `cancelled` | «Перемещение уже отменено» |

Успех:
```bash
```
Ошибка (TRANSFER_STATUS_CONFLICT → 409):
```bash
```
DS: DS-019, DS-048.

### §6.8. `e:GET /api/v1/equipment`
- Назначение: поиск единиц оборудования для строки документа (кнопка «+ Оборудование»). НОВЫЙ endpoint: списка единиц по всем клиентам в проекте нет (есть только `e:GET /api/v1/clients/:id/equipment` и `e:GET /api/v1/equipment/:id`) — DS-057.
- Authz: любой авторизованный (как существующее чтение единицы); данные оборудования клиентов и сейчас видны обеим ролям.
- Идемпотентность/повтор: чтение, безопасно.
- Side effects: нет.

Request:

| Параметр | Где | Тип | Обязателен | Валидация |
|---|---|---|---|---|
| `f:warehouse_id` | query | uuid | нет | единицы, числящиеся на складе; не uuid → 400 |
| `f:location_status` | query | string | нет | `at_client`, `at_warehouse`, `in_transit`; иное → 400 |
| `f:q` | query | string | нет | подстрока без учёта регистра по модели, марке, серийному номеру, названию клиента; 0–100 символов, длиннее → 400 |
| `f:limit` | query | integer | нет | 1–100, по умолчанию 20; больше → 100 |
| `f:offset` | query | integer | нет | ≥ 0, по умолчанию 0 |

Фильтры комбинируются по «И». Сортировка: название клиента, затем модель.

Response 200:

| Поле | Тип | Описание |
|---|---|---|
| `f:items` | array | единицы |
| `f:items[].id` | uuid | id единицы |
| `f:items[].client_id` | uuid | клиент |
| `f:items[].client_name` | string | название клиента |
| `f:items[].type_name` | string | тип оборудования |
| `f:items[].brand` | string | марка |
| `f:items[].model` | string | модель |
| `f:items[].serial` | string | серийный номер |
| `f:items[].location_status` | string | `at_client` \| `at_warehouse` \| `in_transit` |
| `f:items[].warehouse_id` | uuid \| null | склад единицы |
| `f:items[].warehouse_name` | string \| null | его название |
| `f:items[].active_transfer_id` | integer \| null | незавершённый документ единицы |
| `f:total` | integer | всего под фильтрами |
| `f:limit` | integer | применённый лимит |
| `f:offset` | integer | применённое смещение |

Ошибки:

| Код | Условие | Сообщение пользователю |
|---|---|---|
| 400 `VALIDATION` | `warehouse_id: invalid` | «Некорректный склад» |
| 400 `VALIDATION` | `location_status: invalid` | «Некорректное местоположение» |
| 400 `VALIDATION` | `q: too_long` | «Слишком длинный запрос» |
| 401 `UNAUTHORIZED` | нет сессии | переход на вход |

Успех:
```bash
```
Ошибка (VALIDATION → 400):
```bash
```
DS: DS-057, DS-008.

### §6.9. `e:GET /api/v1/tasks/:id/card` (существующий, дельта)
- Назначение: карточка заказа; дельта — местоположение единиц читается с `t:equipment`.
- Authz: без изменений — любой авторизованный (D19).
- Идемпотентность/повтор: чтение, безопасно.
- Side effects: нет.

Request:

| Параметр | Где | Тип | Обязателен | Валидация |
|---|---|---|---|---|
| `f:id` | path | integer | да | существующий заказ; иначе 404 |

Response 200 — только изменяемые и новые поля (остальные поля карточки без изменений):

| Поле | Тип | Описание |
|---|---|---|
| `f:task.has_in_transit` | boolean | ИЗМЕНЕНО (источник): есть единица заказа с `location_status = in_transit` |
| `f:equipment[].location_status` | string | НОВОЕ: `at_client`, `at_warehouse`, `in_transit` |
| `f:equipment[].warehouse_id` | uuid \| null | ИЗМЕНЕНО (источник): склад единицы с `t:equipment` |
| `f:equipment[].warehouse_name` | string \| null | название склада |
| `f:equipment[].active_transfer_id` | integer \| null | НОВОЕ: незавершённый документ единицы (ссылка «перемещение #N») |
| `f:equipment[].transit_to_name` | string \| null | НОВОЕ: название склада назначения документа, если единица в пути |
| `x:equipment[].equipment_status` | — | УДАЛЕНО: заменено `f:equipment[].location_status` |
| `x:equipment[].movements` | — | УДАЛЕНО: журнал перемещений единицы в заказе |

Ошибки:

| Код | Условие | Сообщение пользователю |
|---|---|---|
| 404 `TASK_NOT_FOUND` | нет заказа | «Заказ не найден» |

Успех:
```bash
```
Ошибка (TASK_NOT_FOUND → 404):
```bash
```
DS: DS-003, DS-008.

### §6.10. Прочие изменения существующих endpoints (дельта)
Формат каждой строки: endpoint — что меняется — ошибка — DS.

| Endpoint | Изменение | Новая ошибка | DS |
|---|---|---|---|
| `x:POST /api/v1/tasks/:id/equipment/:equipment_id/send` | УДАЛЯЕТСЯ | — | DS-003 |
| `x:POST /api/v1/tasks/:id/equipment/:equipment_id/receive` | УДАЛЯЕТСЯ | — | DS-003 |
| `x:POST /api/v1/tasks/:id/equipment/:equipment_id/cancel-transit` | УДАЛЯЕТСЯ | — | DS-003 |
| `x:/api/v1/stock-transfers` (7 endpoints), страница `x:/stock-transfers` | УДАЛЯЮТСЯ, заменены `e:/api/v1/transfers` и `/transfers` | — | DS-030 |
| `e:POST /api/v1/tasks/:id/equipment` | параметр `f:warehouse_id` остаётся (снимается в части 4); склад пишется на единицу и только если она `at_client` и свободна; журнал `received` не пишется | — | DS-008 |
| `e:DELETE /api/v1/tasks/:id/equipment/:equipment_id` | запрет, пока единица в незавершённом документе; код `x:TASK_EQUIPMENT_IN_TRANSIT` заменён | 409 `EQUIPMENT_IN_TRANSFER` «Оборудование в перемещении #N — сначала отмените его» | DS-026 |
| `e:DELETE /api/v1/equipment/:id` | запрет, если у единицы есть строки документов | 409 `EQUIPMENT_HAS_TRANSFERS` «У оборудования есть перемещения — удалить нельзя» | DS-026 |
| `e:PUT /api/v1/warehouses/:id` и SSR `POST /warehouses/:id/archive` | запрет архивирования, пока на складе числится оборудование либо есть документ в `requested`, `in_transit` с этим складом | 409 `WAREHOUSE_IN_USE` «На складе есть оборудование или незавершённые перемещения» | DS-026 |
| `e:GET /api/v1/tasks` | фильтр `f:warehouse_id` и поле `f:has_in_transit` считаются по `t:equipment` и `t:transfers`: единица на складе X либо в пути с назначением X | — | DS-046 |
| `e:GET /api/v1/equipment/:id/history` и SSR-страница истории | вместо склада единицы «в каждом заказе» — текущее местоположение единицы и список её документов (#N, маршрут, статус, дата) | — | DS-046 |
| `e:GET /api/v1/stock` («Остатки») | CTE «в пути» читает переименованные таблицы; учитываются только строки `kind = product` | — | DS-030 |
| Склад по умолчанию строки товара услуги (D58) | читает склад единицы с `t:equipment` (`at_warehouse` и склад активен; иначе первый активный) | — | DS-046 |

Ошибка (EQUIPMENT_HAS_TRANSFERS → 409):


## §8. UI-иерархия
- Работа → **Перемещения** (`/transfers`) → список + документ (master-detail) → модалки «Приёмка», «Отмена» — §8.1
- Доска → **Карточка заказа** → блок «Оборудование» — §8.2

UI-конвенции (DS-052): ошибка поля — красный текст под полем + красная рамка (`is-invalid` Bootstrap), текст из `details.fields[]`; глобальная ошибка и успех — тост в правом верхнем углу, 5 секунд; подтверждения — модальный `KT.uiDialog` по центру, закрытие «Отмена» / Esc; на время запроса кнопка действия заблокирована; предупреждения по остатку (`f:warnings`) — жёлтый блок над строками документа. Адаптив — минимальный: нет горизонтальной прокрутки страницы (DS-028).

### §8.1. Экран/форма «Перемещения»
- Маршрут: `/transfers` (список), `/transfers?id=N` (открыт документ)

**Как попадаем:**
- **Путь:** меню «Работа → Перемещения»; ссылка «перемещение #N» в карточке заказа (§8.2, для администратора)
- **URL:** `/transfers`, параметр `id`
- **Роли:** admin (executor — пункта меню нет, прямой URL → страница «Недостаточно прав») · **Устройство:** десктоп; телефон — без горизонтальной прокрутки
- **При открытии:** `GET §6.1` → `/api/v1/transfers?limit=50&offset=0`; при `?id=N` — дополнительно `GET §6.3` → `/api/v1/transfers/N`
- **200:** слева список, справа документ либо подсказка «Выберите перемещение» · **Пусто:** «Перемещений нет» + кнопка «Новое перемещение» · **4xx:** 403 — страница «Недостаточно прав»; 404 по `?id` — тост «Перемещение не найдено», список остаётся

**Что на странице (сверху вниз):**
1. Заголовок «Перемещения» и кнопка «Новое перемещение» (основная, справа).
2. Панель фильтров (над списком); комбинация — «И»; кнопка «Сбросить» справа:

   | Фильтр | Контрол | Источник значений | API-параметр §6.1 | Пример |
   |---|---|---|---|---|
   | Статус | select | enum §5: Заявка / В пути / Получено / Отменён | `status` | `in_transit` |
   | Склад | select | SSR-список складов (все, архивные с пометкой) | `warehouse_id` | uuid |
3. Список (слева): строка = «#N», маршрут «Алматы → Москва», бейдж статуса, число строк, дата создания; под списком — «Показать ещё» (следующие 50).
4. Документ (справа), шапка:
   1. Склад отправления (`from_warehouse_id`) — select активных складов.
   2. Склад назначения (`to_warehouse_id`) — select активных складов.
   3. Примечание (`note`) — textarea.
5. Строки документа: таблица с колонками «Тип» (товар / оборудование), «Наименование» (товар — название и единица измерения; оборудование — «тип марка модель», серийный номер, клиент), «Кол-во» (только товар), «Факт» (после приёмки, только товар), кнопка удаления строки (в строке справа, только в «Заявке»).
6. Под таблицей (только «Заявка»): «+ Товар» (combobox по каталогу) и «+ Оборудование» (combobox с поиском единиц, числящихся на складе отправления; занятые другим документом показаны неактивными с пометкой «в перемещении #N»).
7. Кнопки действий (внизу справа) по статусу: «Заявка» — «Сохранить» (второстепенная), «Отправить» (основная), «Отменить» (второстепенная); «В пути» — «Принять» (основная), «Отменить отправку» (второстепенная); «Получено», «Отменён» — кнопок нет.
8. Сведения: создан / отправлен / получен / отменён — кто и когда.

**Интерактив:**
1. **Смена фильтра**
   - **Элемент:** select в панели фильтров
   - **UI:** список затеняется на время запроса
   - **Вызов:** `GET §6.1` → `/api/v1/transfers?status=…&warehouse_id=…&limit=50&offset=0`
   - **200:** `{ items, total, limit, offset }` → список перерисован; счётчик «N из total» под списком
   - **400:** тост «Некорректное значение фильтра»
2. **Выбор документа (строка списка)**
   - **Элемент:** строка списка
   - **UI:** строка подсвечена, URL → `?id=N`
   - **Вызов:** `GET §6.3` → `/api/v1/transfers/N`
   - **200:** `{ id, status, items, version, … }` → документ справа
   - **404:** тост «Перемещение не найдено», список перезапрашивается
3. **«+ Оборудование» (поиск единицы)**
   - **Элемент:** combobox под таблицей строк, слева
   - **UI:** debounce 300 мс; недоступен, пока не выбран склад отправления (подсказка «Сначала выберите склад отправления»)
   - **Вызов:** `GET §6.8` → `/api/v1/equipment?warehouse_id=<склад отправления>&location_status=at_warehouse&q=…&limit=20`
   - **200:** `{ items, total, limit, offset }` → выпадающий список «клиент · тип марка модель · серийный номер»; выбор добавляет строку-единицу в таблицу
   - **400 (q: too_long):** под combobox — «Слишком длинный запрос»
4. **«Сохранить» (заявка)**
   - **Элемент:** кнопка второстепенная, внизу справа
   - **UI:** кнопки заблокированы
   - **Вызов:** новый — `POST §6.2` → `/api/v1/transfers`, body: `{ from_warehouse_id, to_warehouse_id, note, items }`; существующий — `PUT §6.4` → `/api/v1/transfers/N`, body: `{ version, from_warehouse_id, to_warehouse_id, note, items }`
   - **200/201:** `{ id, version, … }` → тост «Сохранено», URL → `?id=N`, строка списка обновлена
   - **400 (склады):** под соответствующим select — «Проверьте склад» / «Склады должны различаться»
   - **400 (items: required / too_many):** над таблицей — «Добавьте строки (не более 200)»
   - **400 (items[i].product_id / equipment_id / qty_sent):** красный текст в соответствующей строке таблицы
   - **409 (версия):** тост «Перемещение уже изменено, обновите», документ перечитывается
   - **409 (статус: отправлено):** тост «Перемещение уже отправлено — править нельзя», документ перечитывается
   - **409 (статус: получено):** тост «Перемещение уже получено — править нельзя», документ перечитывается
   - **409 (статус: отменено):** тост «Перемещение отменено — править нельзя», документ перечитывается
   - **409 (EQUIPMENT_IN_TRANSFER):** красный текст в строке единицы «уже в перемещении #N» (ссылка)
5. **«Отправить»**
   - **Элемент:** кнопка основная, внизу справа
   - **UI:** несохранённые правки сначала сохраняются (вызов п.4), затем отправка; кнопки заблокированы
   - **Вызов:** `POST §6.5` → `/api/v1/transfers/N/send`, body: `{ version }`; для нового документа — `POST §6.2` с `send: true`
   - **200:** `{ status: "in_transit", warnings, … }` → бейдж «В пути»; `warnings` — жёлтый блок над строками
   - **409 (версия):** тост «Перемещение уже изменено, обновите»
   - **409 (статус: уже отправлено):** тост «Перемещение уже отправлено»
   - **409 (статус: получено):** тост «Перемещение уже получено»
   - **409 (статус: отменено):** тост «Перемещение отменено»
   - **409 (TRANSFER_EQUIPMENT_NOT_AT_SOURCE):** красный текст в строке единицы «Оборудование находится не на складе отправления»
6. **«Принять»**
   - **Элемент:** кнопка основная → модалка «Приёмка» с полем «Факт» по каждой товарной строке (предзаполнено количеством); строки-единицы перечислены без поля, с пометкой «принимается целиком»
   - **UI:** кнопка модалки заблокирована на время запроса
   - **Вызов:** `POST §6.6` → `/api/v1/transfers/N/receive`, body: `{ version, items: [{ id, qty_received }] }`
   - **200:** `{ status: "received", items, … }` → бейдж «Получено», колонка «Факт» в таблице, расхождение подсвечено
   - **400 (exceeds_sent):** под полем «Факт» — «Факт больше отправленного»
   - **400 (negative / invalid):** под полем «Факт» — «Факт не может быть отрицательным»
   - **409 (версия):** тост, документ перечитывается
   - **409 (статус: не отправлено):** тост «Перемещение ещё не отправлено»
   - **409 (статус: уже получено):** тост «Перемещение уже получено»
   - **409 (статус: отменено):** тост «Перемещение отменено»
   - **409 (STOCK_OVERFLOW):** тост «Слишком большое количество на складе»
7. **«Отменить» / «Отменить отправку»**
   - **Элемент:** кнопка второстепенная → `KT.uiDialog` «Отменить перемещение #N?»
   - **UI:** кнопка диалога заблокирована
   - **Вызов:** `POST §6.7` → `/api/v1/transfers/N/cancel`, body: `{ version }`
   - **200:** `{ status: "cancelled", … }` → бейдж «Отменён»
   - **409 (версия):** тост, документ перечитывается
   - **409 (получено):** тост «Перемещение уже получено — отменить нельзя»
   - **409 (уже отменено):** тост «Перемещение уже отменено»

**Состояния:** потеря связи — тост «Нет связи», кнопки разблокируются, правки формы остаются в полях; reload — документ и список читаются с сервера, несохранённые правки заявки теряются; документ изменён другим администратором — при следующем действии 409 по версии, документ перечитывается.

### §8.2. Экран/форма «Карточка заказа»
- Маршрут: карточка заказа на доске (существующая), дельта блока «Оборудование»

**Как попадаем:**
- **Путь:** доска → клик по заказу
- **URL:** существующий URL карточки заказа
- **Роли:** admin, executor · **Устройство:** десктоп; телефон — без горизонтальной прокрутки
- **При открытии:** `GET §6.9` → `/api/v1/tasks/37/card`
- **200:** карточка с бейджами местоположения · **Пусто:** оборудования в заказе нет — блок как сейчас · **4xx:** 404 — «Заказ не найден»

**Что на странице (дельта, сверху вниз):**
1. Блок «Оборудование», строка единицы:
   1. Убраны кнопки `x:«Отправить»`, `x:«Принять»`, `x:«Отменить отправку»` и панель журнала перемещений единицы (DS-003).
   2. Бейдж местоположения под названием единицы: «у клиента» / «на складе Алматы» / «в пути → Москва».
   3. Ссылка «перемещение #N» рядом с бейджем, если единица в незавершённом документе (`active_transfer_id`); для исполнителя — текст без ссылки (экран `/transfers` ему недоступен до части 4).
   4. Кнопка «Убрать» скрыта, пока единица в незавершённом документе.
2. Форма добавления единицы: select «Склад приёма» (`warehouse_id`) остаётся; подпись под ним — «Если оборудование уже на складе или в пути, местоположение не изменится».

**Интерактив:**
1. **Ссылка «перемещение #N» (admin)**
   - **Элемент:** ссылка в строке единицы, рядом с бейджем
   - **UI:** переход на `/transfers?id=N`
   - **Вызов:** `GET §6.3` → `/api/v1/transfers/N`
   - **200:** документ открыт на экране §8.1
   - **404:** тост «Перемещение не найдено»
2. **«Убрать» единицу из заказа**
   - **Элемент:** существующая кнопка в строке единицы
   - **UI:** без изменений
   - **Вызов:** `DELETE` → `/api/v1/tasks/37/equipment/<uuid>` (§6.10)
   - **200:** без изменений (карточка перечитывается)
   - **409 (EQUIPMENT_IN_TRANSFER):** тост «Оборудование в перемещении #N — сначала отмените его»
   - **409 (TASK_EQUIPMENT_IN_USE):** тост «Оборудование выбрано в услугах заказа — сначала смените его в услугах» (as-is)
3. **Добавление единицы в заказ**
   - **Элемент:** существующая форма, select «Склад приёма» + кнопка «Добавить»
   - **UI:** без изменений
   - **Вызов:** `POST` → `/api/v1/tasks/37/equipment`, body: `{ equipment_id, warehouse_id, completeness, condition }` (§6.10)
   - **200:** единица в блоке; бейдж «на складе …» — выбранный склад, если единица была «у клиента», иначе её прежнее местоположение
   - **400 (warehouse_id):** под select — «Проверьте склад» (as-is)

**Состояния:** документ проведён администратором, пока карточка открыта — местоположение обновится при следующем открытии карточки либо после любого действия в ней (re-fetch).


## §10. Открытые вопросы (status: open)
Открытых вопросов нет.


## §11. Тестовые сценарии (черновик — вход для design-e2e-strategy, TD-6.8)
UI-scope (DS-028): swagger-сценариев нет; адаптив — один overflow-смок (TS-045); темы — обе (TS-046).

### Unit / Integration (Go)
- **TS-005.** Занятие единицы: создание второй заявки с той же единицей → 409 `EQUIPMENT_IN_TRANSFER` с `details.transfer_id`; после отмены первой — успех; снятая в `PUT` единица освобождается — DS-019, DS-041
  - негатив: два параллельных `POST` с одной единицей → ровно один 201
- **TS-006.** Отправка: единица не на складе отправления (на другом складе либо «у клиента») → 409 `TRANSFER_EQUIPMENT_NOT_AT_SOURCE` — DS-036
  - негатив: остатки и местоположение не изменились
  - не входит в эту часть: вариант «не у клиента» для направления «клиент → склад» — `transfers-partners`
- **TS-009.** Версия и статус: устаревшая `version` → 409 `TRANSFER_VERSION_CONFLICT` (сверяется до статуса); каждый недопустимый переход из §5 → 409 `TRANSFER_STATUS_CONFLICT` со своим сообщением — DS-048
  - негатив: двойной параллельный `receive` → один 200, один 409, остаток вырос один раз, единица принята один раз
- **TS-018.** Единица в незавершённом документе: `DELETE /tasks/:id/equipment/:equipment_id` → 409 `EQUIPMENT_IN_TRANSFER`; `DELETE /equipment/:id` при наличии строк документов → 409 `EQUIPMENT_HAS_TRANSFERS` — DS-026
  - негатив: после отмены документа единица убирается из заказа
- **TS-019.** Архивирование склада: на складе числится единица → 409 `WAREHOUSE_IN_USE`; есть заявка/в пути с этим складом → 409; иначе — как сейчас (в т.ч. `WAREHOUSE_LAST_ACTIVE`) — DS-026
  - негатив: склад только с завершёнными документами и без оборудования архивируется
- **TS-024.** Доска: `GET /tasks?warehouse_id=X` — заказ с единицей на X и заказ с единицей в пути к X; `has_in_transit` — по `equipment.location_status` — DS-046
  - негатив: единица «у клиента» не попадает ни под один склад
- **TS-026.** Миграция (integration): `x:stock_transfers` → `t:transfers`, `x:stock_transfer_items` → `t:transfer_items` с `kind = product`; местоположение: (а) единица только в завершённом заказе (заказ в последней колонке доски — наибольший `columns.position`) → `at_client`, склад пуст; (б) единица в незавершённом заказе `at_warehouse` → `at_warehouse`, тот же склад; (в) единица в незавершённом заказе `in_transit` → `at_warehouse`, склад отправления; (г) единица и в завершённом, и в незавершённом заказе → `at_warehouse`, склад из незавершённого; (д) единица в двух незавершённых заказах → склад из заказа с большим `id`; (е) единица без заказов → `at_client`; `x:task_equipment_movements` отсутствует; CHECK'и `i:transfer_items_kind_check`, `i:equipment_location_check` отклоняют невалидные строки — DS-029, DS-030, DS-054
  - негатив: вставка `in_transit` без `active_transfer_id` отклонена; заказ в колонке с `is_final = true`, но не последней по `position`, считается незавершённым; при единственной колонке все единицы → `at_client`
- **TS-027.** Удалённые endpoints: `POST /tasks/:id/equipment/:equipment_id/send`, `/receive`, `/cancel-transit`, `/api/v1/stock-transfers` → 404 — DS-003, DS-030
  - негатив: `POST /tasks/:id/equipment` с `warehouse_id` для единицы, уже лежащей на другом складе, — местоположение не меняется
  - не входит в эту часть: «`warehouse_id` игнорируется, единица остаётся у клиента» — `transfers-partners` (DS-045)
- **TS-028.** Fuzz валидатора `POST /api/v1/transfers` (произвольный JSON) — без паники, только 201/400/403/409 — TD-4
  - негатив: 500 недопустим

Дополнительно к мастер-сценариям (в объёме этой части, без новых номеров): валидатор строк `kind` × поля (§6.2) покрывается в составе TS-028 и TS-005; `e:GET /api/v1/equipment` (фильтры, конверт, `location_status: invalid` → 400) — в составе TS-024.

### E2E (Playwright)
- **TS-031.** Перевозка между складами: товар + оборудование клиента — экран §8.1 — DS-001
  - шаги: admin → «Новое перемещение» → склады → «+ Товар», «+ Оборудование» → «Отправить» → «Принять» с фактом меньше отправленного; проверка: расхождение подсвечено, в карточке заказа единица «на складе Москва»; негатив: факт больше отправленного → «Факт больше отправленного»
- **TS-036.** Карточка заказа без кнопок перемещения — экран §8.2 — DS-003
  - шаги: открыть карточку с оборудованием; проверка: кнопок «Отправить», «Принять», «Отменить отправку» нет; есть бейдж местоположения и «перемещение #N»; негатив: «Убрать» скрыта у единицы в незавершённом документе
  - не входит в эту часть: «select „Склад приёма“ отсутствует» — `transfers-partners` (DS-045)
- **TS-043.** Конкурентная правка документа — экран §8.1 — DS-048
  - шаги: два контекста открывают одну заявку, первый отправляет, второй сохраняет; проверка: у второго тост «Перемещение уже изменено, обновите», документ перечитан; негатив: данные первого не затёрты
- **TS-044.** Фильтр доски по складу и бейдж «в пути» — доска — DS-046
  - шаги: документ с единицей заказа отправлен; проверка: заказ виден под фильтром склада назначения, бейдж «в пути»; негатив: после приёмки бейджа нет
- **TS-045.** Overflow-смок (adaptive:min) — экраны §8.1, §8.2 — DS-028
  - шаги: вьюпорт 375×812, открыть каждый экран с документом; проверка: `scrollWidth <= clientWidth` у документа; негатив: нет
- **TS-046.** Темы (themes:both) — экраны §8.1, §8.2 — DS-028
  - шаги: светлая и тёмная тема; проверка: бейджи статуса и местоположения читаемы, жёлтый блок предупреждений контрастен; негатив: нет

### Фикстуры и сиды (АКЦЕНТ — хронический пропуск)
- **TS-F1.** `internal/seeddemo/seeddemo.go` (+ `seeddemo_integration_test.go`): переписать под новую модель в той же фазе, что миграция (§TD-1) — местоположение на `t:equipment` (единицы заказов в последней колонке доски — «у клиента»; единицы незавершённых заказов — «на складе» либо «в пути»; DS-054); документы «склад → склад»: один `received`, один `in_transit`, в одном из них строка-единица; строки `x:task_equipment_movements` убрать — DS-029. Части 4–6 дополняют сид своими документами (направления, накладная, откат).
- **TS-F2.** `e2e/fixtures/auth.ts`, `e2e/fixtures/board-types.ts`: wire-типы карточки (`location_status`, `active_transfer_id`, `transit_to_name`), убрать `equipment_status` / `movements`; payload'ы типизировать wire-типами из исходников, не сырыми ключами (§TD-6.2) — DS-008. Поля `transfers[]`, `transfers_amount` добавляют части 4 и 5.
- **TS-F3.** `e2e/pages/StockTransfersPage.ts` → `TransfersPage.ts`, `e2e/tests/stock-transfers.spec.ts` → `transfers.spec.ts`; `e2e/pages/OrderCardPage.ts` — убрать локаторы кнопок перемещения, добавить бейдж местоположения; `e2e/tests/equipment-warehouses.spec.ts` — переписать под документ — DS-030, DS-003
- **TS-F4.** Go-тесты под удаление/переписывание: `internal/features/board/equipment_warehouses_http_live_test.go`, `equipment_warehouses_migration_integration_test.go`, `equipment_warehouse_validate_test.go`, `order_card_http_live_test.go`, `warehouses_card_http_live_test.go`; `internal/features/stocktransfers/*_test.go` → пакет `transfers` — DS-003, DS-030


## §9. Решения (DS-index)
| DS | Формулировка | Источник | Статус |
|---|---|---|---|
| DS-001 | Один документ «Перемещение» со смешанными строками (товар с количеством + единица оборудования); расширяем существующее перемещение, вторую сущность не заводим | мастер, батч 0 | closed |
| DS-003 | Кнопки «Отправить / Принять / Отменить отправку» у оборудования в карточке заказа убираются (отмена D57); оборудование двигается только документом; в карточке — местоположение и ссылка | мастер, батч 0 | closed |
| DS-005 | Закупленное у поставщика («наше оборудование») — товар каталога с количеством; `equipment.client_id` остаётся NOT NULL | мастер, батч 1–2 | closed |
| DS-008 | Местоположение — на самой единице оборудования: «у клиента» / «на складе X» / «в пути (документ #N)»; новая единица — «у клиента» | мастер, батч 1 | closed |
| DS-009 | Всё от поставщика учитывается количеством; карточка оборудования клиента заводится вручную | мастер, батч 2 | closed |
| DS-010 | Строки-единицы — только оборудование клиентов | мастер, inferred | closed |
| DS-019 | Единица занимается документом сразу при добавлении в заявку: только один незавершённый документ; иначе «уже в перемещении #N» | мастер, батч 4 | closed |
| DS-026 | Единицу нельзя убрать из заказа и удалить из справочника, пока она в незавершённом документе; единицу с документами удалить из справочника нельзя; склад нельзя архивировать, пока на нём числится оборудование или есть незавершённые документы | мастер, батч 5 | closed |
| DS-027 | Совместимость — MVP | мастер, батч 6 | closed |
| DS-028 | UI-scope: swagger:no, adaptive:min, themes:both | мастер, батч 6 | closed |
| DS-029 | Миграция — сброс: единицы «в пути» считаются лежащими на складе отправления; старый журнал удаляется; демо-данные пересоздаются | мастер, батч 6 | closed |
| DS-030 | Переименование в `transfers`: страница, API, таблицы, код; старые адреса убираются | мастер, батч 6 | closed |
| DS-036 | Отправить единицу можно только оттуда, где она числится | мастер, inferred | closed |
| DS-037 | Строки-единицы принимаются целиком вместе с документом | мастер, батч 7 | closed |
| DS-041 | Занятие единицы хранится колонкой `equipment.active_transfer_id` (guard-UPDATE) | мастер, inferred | closed |
| DS-046 | Фильтр доски по складу, бейдж «в пути», склад по умолчанию строки товара (D58, D59), история оборудования читают местоположение с единицы и документов | мастер, inferred | closed |
| DS-048 | Конкурентность документа — как сейчас: `version` обязателен, сверяется до статуса, 409; Lock Order документ → остатки → единицы | мастер, as-is (D50, D51) | closed |
| DS-049 | Лимиты как сейчас: до 200 строк, примечание до 1000 символов, количество numeric(12,3) > 0, дубль товара/единицы — ошибка | мастер, as-is (D52) | closed |
| DS-051 | Отдельной таблицы аудита и аудита чтения нет | мастер, zap | closed |
| DS-052 | UI-конвенции: ошибка под полем, тост справа вверху 5 с, модальный `KT.uiDialog`, блокировка кнопки | мастер, inferred | closed |
| DS-054 | Миграция местоположения: единица хотя бы в одном незавершённом заказе → «на складе»; только в завершённых → «у клиента»; без заказов → «у клиента». Завершённый заказ = заказ в последней колонке доски (наибольший `columns.position`) | мастер, батч 7 | closed |
| DS-055 | Ключа идемпотентности нет: повтор отсекается версией/статусом; дубль чисто товарной заявки — риск принят | мастер, inferred | closed |
| DS-057 | Поиск единиц для строки документа — новый `e:GET /api/v1/equipment` (фильтры: склад, местоположение, строка поиска); доступен любому авторизованному | нарезка (пробел мастера: у «+ Оборудование» не было endpoint'а), inferred | closed |
| DS-002 | Четыре направления: клиент → склад, склад → клиент, склад → склад, поставщик → склад | мастер, батч 0 | closed — контекст, реализуется в `transfers-partners` |
| DS-018 | Исполнитель работает с документами с клиентом | мастер, батч 4 | closed — контекст, реализуется в `transfers-partners` |
| DS-045 | При добавлении единицы в заказ склад не выбирается | мастер, inferred | closed — контекст, реализуется в `transfers-partners` |
| DS-017 | «Отменить проведение» полученного документа | мастер, батч 3 | closed — контекст, реализуется в `transfers-revert` |
| DS-025 | Удаление документа и запрет удаления заказа с документами | мастер, батч 5 | closed — контекст, реализуется в `transfers-revert` |
| DS-031 | Показ времени по поясу пользователя | мастер, батч 6 | closed — контекст, реализуется в `user-timezone` |

