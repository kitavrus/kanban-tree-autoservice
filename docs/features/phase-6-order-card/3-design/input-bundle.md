# Input Bundle: phase-6-order-card / стадия 3-design — generated 2026-09-27 11:01

> Твой ЕДИНСТВЕННЫЙ документный вход (пайп-контракт). Upstream-доки напрямую НЕ читать;
> точечный grep по конкретному ID (DS-NNN/AC-NNN/TS-NNN) — разрешён. Отсутствие нужного
> факта в бандле = сигнал оркестратору (дырка контракта), не повод читать всё.

## Источник: 2-spec-interview/output.md [full]
---
stage: 2-spec-interview
feature: phase-6-order-card
tier: M
---
# Spec: Полная карточка заказа (фаза 6)
**Дата интервью:** 2026-09-27
**На основе:** PRD `2-spec-interview/prd.md` (approved) + input-bundle (research-экстракты) + docs/plan.md D1–D44 + phase-5 SD-001…SD-008 + `spec-decisions.md` SD-001…SD-008
**Tier:** M
**Совместимость:** MVP (CM-1)
**UI-scope (US-1):** swagger:no | adaptive:min | themes:both

## Применённые правила (zap-list)
- Ошибки — только `pkg/errorspkg`, 400 → `details.fields[]`; 404/403/409 стандартные; sentinel-ы в `internal/features/board/errors.go`, `.WithDetail` — копия.
- Auth — Redis-сессия + CSRF (D35, D40), права из БД на каждом запросе (D42); admin-гард `middleware.RequireAdmin(nil)` per-route.
- Синхронно, без очередей; аудит-лога нет (slog + `trace_id`); журнал склада — `stock_movements` (D34, D44).
- Миграции — только `make migrate-up`.
- §API-1: URL kebab + `/api/v1`, JSON snake_case, list → `{items,total,limit,offset}`.
- NUMERIC (`qty`, `price`, итоги) — `pkg/numeric.Amount` (строка в Go, число на wire), не float64.
- `version` — только `tasks` (и `clients`), в теле запроса; услуги/товары/оборудование/журнал — LWW (D28).
- Tracer: `otel.Tracer("board")` — как в существующем пакете (`tasks.move`, `columns.recalc`).
- Код — в пакете `internal/features/board` (файлы-по-сущности: `subtasks_*`, `subtask_products_*`, `subtask_logs_*`, `task_equipment_*`, `print_*`), не новая фича.
- Lock Order ADR-104: `columns → tasks → products`; любая мутация состава заказа — `withTx` (ретрай 40P01/40001×3) + `LockTask` ПЕРВЫМ, затем правка, затем `StockEngine.Reconcile` при списывающей колонке.
- Re-fetch-after-write: ответ мутации — из `repo.Get()` после commit.
- Testid admin-only элементов снимается атрибутом (board.js:576-579).

## Технические ограничения
- **Модель сохранения (SD-003, driver: AC-002, AC-007, AC-014):**
  - Шапка: существующие `POST /api/v1/tasks`, `PUT /api/v1/tasks/:id` с `version`, расширяются полем `description_html` (санитизация — Q-006). `is_paid` по-прежнему отправляется всегда (board.js:590-593), иначе тихий сброс оплаты (AC-003).
  - Строки: отдельные эндпоинты, LWW, немедленно по `change` (SD-008). Ориентировочный набор (форма — Q-001):
    - `POST /api/v1/tasks/:id/subtasks`, `PUT|DELETE /api/v1/subtasks/:id`
    - `POST /api/v1/subtasks/:id/products`, `PUT|DELETE /api/v1/subtask-products/:id`
    - `POST /api/v1/tasks/:id/equipment`, `PUT|DELETE /api/v1/tasks/:id/equipment/:equipment_id`
    - `GET|POST /api/v1/subtasks/:id/logs`, `PUT|DELETE /api/v1/subtask-logs/:id` (D27)
  - Каждая мутация строк, меняющая потребность в товарах (добавить/удалить услугу, добавить/удалить/изменить qty товара), в списывающей колонке → `Reconcile(taskID, true)` в той же транзакции (driver: AC-010). Мутации строк НЕ инкрементируют `tasks.version` (иначе открытая шапка ловит 409 от собственных правок строк) — подтвердить в design (Q-002).
  - Ответ мутации строки возвращает обновлённое агрегатное состояние, нужное UI: строку + итоги заказа + `stock_warnings` (формат Reconcile фазы 5) — чтобы не делать второй GET (Q-001).
- **Новый заказ (SD-006, driver: AC-001):** разделы оборудования/услуг неактивны до первого `POST /tasks`; после ответа модалка переходит в режим правки `#N` без закрытия.
- **Добавление услуги (driver: AC-007, AC-008):** сервер копирует `services.price` → `subtasks.price` и `service_products` → `subtask_products` (qty, `products.price` на момент добавления) в одной транзакции; клиент цены не передаёт при создании.
- **Финальная колонка (D22, driver: AC-011):** в финальной колонке `POST subtasks` → 409, `PUT subtasks` со сменой статуса из `done` → 409 (код — существующий паттерн фазы 5; новый sentinel `SUBTASK_FINAL_COLUMN` — Q-003).
- **Оборудование (driver: AC-004, AC-005):** `equipment.client_id` должен совпадать с `tasks.client_id`, иначе 400 (`details.fields[equipment_id]`). При смене клиента в `PUT /tasks/:id` сервер удаляет `task_equipment` чужого клиента в той же транзакции; если такая единица привязана к услуге — 409 `TASK_EQUIPMENT_IN_USE` (SD-005). Новое оборудование из карточки — через API оборудования фичи `clients` + `POST .../equipment` (два запроса) или составной вызов — Q-004.
- **История единицы (AC-006):** список других заказов с этой единицей (`task_equipment` по `equipment_id`, кроме текущего) — отдаётся вместе с оборудованием заказа, без N+1.
- **Подсказка остатка (AC-009):** считает клиент по формуле прототипа (prototype:1169-1173): `avail = products.stock + task_stock.qty(этот заказ)`, `need = Σ qty товара по всем услугам заказа`; данные (`stock`, `min_stock`, `unit`, списанное этим заказом) приходят в GET карточки. Сервер подсказку не валидирует (минус допустим, D17).
- **Итоги (AC-013):** `Σ subtasks.price (NULL не входит) + Σ qty×price (NULL price товара → 0)`; округление — 2 знака на сервере (NUMERIC). Итог отдаётся в GET карточки, в элементах `GET /tasks` (карточка доски) и сумма по колонке — в ответе доски; считается SQL-агрегатом, без N+1 (Q-005).
- **Последняя запись журнала на доске (AC-012):** элемент `GET /tasks` получает `last_log {text (обрезка), author, created_at}` — последняя по времени запись среди всех услуг заказа (Q-005).
- **Позиция услуг:** `subtasks.position int` без дробной схемы; новая услуга — `max+1` внутри заказа; перестановки нет (non-goal DnD).
- **Исполнитель услуги (SD-007):** новый/изменённый `assignee_id` — только активный пользователь; ранее назначенный заблокированный сохраняется и отображается «неактивен».
- **Печать (SD-004, driver: AC-015, AC-016):** SSR `GET /tasks/:id/print/intake`, `GET /tasks/:id/print/handover` (под `RequireSSR`, `no-store` как у доски фазы 5), отдельный print-layout без сайдбара, `@media print`; env `FIRM_NAME`, `FIRM_ADDRESS`, `FIRM_PHONE` (пустые → блок реквизитов опускается, старт не падает). В акт выдачи попадают все услуги заказа с ценами (статус показывается), итог — как AC-013.
- **Новая таблица:** `subtask_logs(id uuid PK, subtask_id FK subtasks ON DELETE CASCADE, user_id FK users RESTRICT, text CHECK(btrim<>'' ∧ len≤4000), created_at, updated_at)`, индекс `(subtask_id, created_at)`. Каскад удаления услуги (SD-005) — FK CASCADE на `subtask_products` (уже есть) и `subtask_logs`. Удаление пользователя с записями — RESTRICT (Q-007).
- **Производительность:** сохранение шапки/строки p95 < 500 мс (PRD §3); ожидаемые объёмы — единицы услуг и десятки товаров на заказ, сотни заказов на доске.
- Внешних зависимостей и новой инфраструктуры нет; новые env — только `FIRM_*`.

## Enum-поля фичи (CLAUDE.md §8.1)
| Поле (DTO.json) | Допустимые значения | Источник правды |
|---|---|---|
| `subtasks.status` | `new, in_progress, done` | `migrations/0005_board.up.sql` CHECK + `internal/features/board/enums.go` |
| `stock_movements.reason` | `task_consume, task_return, manual` (без изменений) | `migrations/0004_catalog.up.sql` CHECK |
| вид акта (path) | `intake, handover` | роут печати |

## Безопасность и доступ
- Все эндпоинты — `RequireAPI` (сессия + CSRF на мутациях); печать — `RequireSSR` (driver: AC-017).
- Executor: всё в карточке, кроме оплаты (`PAID_ADMIN_ONLY` 403, phase-5 SD-008) и удаления заказа (driver: AC-003, AC-017).
- Журнал: `PUT/DELETE /subtask-logs/:id` — автор записи или admin, иначе 403 (новый sentinel, D27, driver: AC-012).
- IDOR: `/subtasks/:id`, `/subtask-products/:id`, `/subtask-logs/:id` проверяют принадлежность к существующему заказу; при несовпадении родителя в пути — 404. Видимость заказов не ограничена ролью (D19 — вся доска видна всем).
- Чувствительные данные: телефон клиента на акте — только авторизованным. `description_html` — XSS-вектор, обязательная серверная санитизация allowlist-тегов (Q-006).

## UI/UX требования
- Модалка поверх доски (SD-002), расширение `openTaskForm`; deep-link `/board?task=N` (фактический путь доски — `/`, т. е. `/?task=N`, уточнить в design).
- Строки сохраняются по `change` (SD-008): строка блокируется на время запроса, ошибка → откат значения + `KT.uiAlert`/inline-сообщение; предупреждения склада (минус/минимум) — как при переносе в фазе 5.
- Удаление услуги — `KT.uiConfirm` с упоминанием, что журнал и товары удалятся (SD-005).
- Итоги по услуге и заказу пересчитываются из ответа сервера (не только локально).
- adaptive:min — модалка во всю ширину на узком экране, таблица товаров со скроллом внутри; themes:both — печатные страницы всегда светлые (`@media print`).
- Кнопки «Акт приёма» / «Акт выдачи» открывают новую вкладку; доступны только у сохранённого заказа.

## Обработка ошибок
| Ошибка | Что видит пользователь | Действие системы | Driver |
|---|---|---|---|
| `TASK_VERSION_CONFLICT` 409 (шапка) | «Заказ уже изменён, обновите» | данные не пишутся | AC-002 |
| `PAID_ADMIN_ONLY` 403 | «Оплату меняет только администратор» | запрос отклонён | AC-003 |
| Оборудование другого клиента 400 `details.fields[equipment_id]` | сообщение у поля | не добавляется | AC-004 |
| `TASK_EQUIPMENT_IN_USE` 409 (удаление единицы / смена клиента) | список услуг, где единица выбрана | ничего не меняется | AC-004, SD-005 |
| Добавление услуги / возврат из `done` в финальной колонке 409 | объяснение про финальную колонку | ничего не меняется | AC-011 |
| Неактивный исполнитель в новом назначении 400 | сообщение у поля | не сохраняется | AC-007, SD-007 |
| Чужая запись журнала 403 | «Можно менять только свои записи» | отклонено | AC-012 |
| Услуга/товар/запись/заказ удалены параллельно 404 | «Строка уже удалена», карточка перечитывается | — | AC-014 |
| Невалидные qty (≤0) / price (<0) / пустой текст записи 400 | сообщение у поля, откат значения | — | AC-008, AC-012 |
| Минус склада после Reconcile | предупреждение со списком товаров (200) | изменение применено | AC-010 |
| Deadlock/serialization | прозрачно | `withTx` ретрай ×3, далее 500 | AC-014 |
- Всё-или-ничего: каждая мутация — одна транзакция (строка + Reconcile + `stock_movements`); автоматический retry на клиенте не делается.

## Операции и наблюдаемость (M)
### Spans
| Имя | Tracer | Log fields | Когда добавляется |
|---|---|---|---|
| `subtasks.mutate` | `board` | `trace_id`, `task_id`, `subtask_id`, `op` | create/update/delete услуги |
| `subtask_products.mutate` | `board` | `trace_id`, `task_id`, `product_id`, `op` | create/update/delete товара |
| `stock.reconcile` (существующий путь) | `board` | `trace_id`, `task_id`, `deltas` | при списывающей колонке |
| `tasks.print` | `board` | `trace_id`, `task_id`, `kind` | рендер акта |

### Метрики и алерты
| Метрика | Порог алерта | Действие on-call |
|---|---|---|
| p95 латентность мутаций `/api/v1/subtasks*`, `/subtask-products*` | > 500 мс за 10 мин | Jaeger по `trace_id`, проверить блокировки products |
| 5xx на эндпоинтах карточки | > 1% за 5 мин | логи slog по `trace_id` |
- Алерты: **deferred** — прод-мониторинг ставит `/infra-bootstrap` (здесь только docker-compose.dev); issue заводится при инфра-фазе. Инвариант «остаток = Σ движений» проверяется интеграционным тестом (PRD §3), не алертом.
- Runbook читает Dev: после деплоя — `make migrate-up` (subtask_logs), задать `FIRM_*`, smoke: открыть карточку seed-demo, добавить услугу в списывающей колонке, сверить «Остатки».

## Принятые компромиссы
- LWW для строк заказа: одновременная правка одной строки молча побеждает последней (D28); склад консистентен за счёт блокировки заказа.
- Подсказка остатка — клиентская, без учёта параллельных заказов в реальном времени.
- Реквизиты фирмы — env, без UI (SD-004); PDF — только браузером.
- Нет перестановки услуг, нет вложений (D21 — позже).

## Отклонённые решения
- Один PUT всего агрегата под `version` заказа — противоречит D28 (SD-003 b).
- Отдельная страница `/tasks/:id` — дальше от эталона и кода фазы 5 (SD-002 b).
- Живое сохранение по debounce — засоряет `stock_movements` (SD-008 b).
- Обнуление `equipment_id` у услуг при удалении единицы — молчаливая потеря данных (SD-005 b).
- Исполнитель услуги только с ролью executor — admin тоже работает руками (SD-007 b).
- Реквизиты в БД с экраном настроек — избыточно для MVP (SD-004 b).

## Открытые вопросы

### Q-001: Форма API строк и агрегатного ответа
Финализировать пути (вложенные `/tasks/:id/subtasks` vs плоские `/subtasks/:id`) по §API-1 и состав ответа мутации: строка + итоги заказа + `stock_warnings`, либо полная карточка. Нужен контракт GET карточки (одним запросом: шапка, оборудование с историей, услуги с товарами и журналом, остатки товаров, `task_stock` заказа, итоги).

### Q-002: Взаимодействие мутаций строк с `tasks.version`
Подтвердить, что мутации услуг/товаров/оборудования/журнала не инкрементируют `tasks.version` (иначе открытая шапка получает 409 после собственных правок строк), а `updated_at` заказа — обновляют или нет. Учесть SD-006 фазы 5 (move инкрементирует version).

### Q-003: Коды ошибок фазы 6
Определить sentinel-ы: `TASK_EQUIPMENT_IN_USE`, финальная колонка для услуг (переиспользовать `SUBTASKS_NOT_DONE`-семейство или новый `SUBTASK_FINAL_COLUMN`), `SUBTASK_LOG_FORBIDDEN`, `SUBTASK_NOT_FOUND`/`SUBTASK_PRODUCT_NOT_FOUND`/`SUBTASK_LOG_NOT_FOUND`, неактивный исполнитель — поле 400.

### Q-004: Создание оборудования из карточки
Проверить, разрешает ли API оборудования фичи `clients` запись исполнителю (D19); выбрать: два запроса (создать в clients + привязать к заказу) или составной `POST /tasks/:id/equipment` с вложенным новым оборудованием в одной транзакции.

### Q-005: Агрегаты доски без N+1
Спроектировать SQL для `GET /tasks` (итог заказа, `last_log`) и суммы по колонке — подзапросы/LATERAL, индексы (`subtask_logs(subtask_id, created_at)`, `subtasks(task_id)`); оценить влияние на латентность доски.

### Q-006: Санитизация `description_html`
Редактор описания (plain textarea → `<p>`/`<br>` или простой rich-text как в прототипе) и серверная allowlist-санитизация; нужна ли новая зависимость (bluemonday) или хватит экранирования plain-текста.

### Q-007: Удаление пользователя с записями журнала
`subtask_logs.user_id` — RESTRICT (как `stock_movements.user_id`) или SET NULL; сверить с поведением удаления пользователя в фиче `users` (фаза 2).

## Источник: AC-строки (2-spec-interview/prd.md) [ac]
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

## Источник: 1-research/output.md [sections]
## Triage
```yaml
scope: cross-feature
tier: M
touches:
  db: true
  fe: false
  infra: false
  external: false
risk: reversible
novelty: standard-crud
decisions: []
```
Обоснование: фича в `docs/plan.md:6` треком `/pipeline (M)` явно (D31, `docs/plan.md:155`). Не Vue — SSR + vanilla JS (`web/ssr/_shared/js/board.js:1`), `touches.fe=false` формально. `scope=cross-feature`: новая таблица subtask_logs, новые эндпоинты subtasks/subtask-products/subtask-logs/task-equipment/print, расширение tasks/board.js/SSR-печати, переиспользование API оборудования из фичи clients.

## DB Schema (baseline)
Источник целиком: `migrations/0005_board.up.sql`, `migrations/0003_clients_equipment.up.sql`, `migrations/0004_catalog.up.sql`.

**tasks** (`migrations/0005_board.up.sql:25-44`): id bigserial PK(D37), column_id uuid FK columns RESTRICT, client_id uuid FK clients RESTRICT, title text CHECK(btrim<>''∧len≤200), description_html text DEFAULT '' (:30 — фаза 5 пишет только seed-demo), assignee_id uuid FK users SET NULL, due_date date NULL, is_paid boolean, paid_at timestamptz NULL (CHECK is_paid=(paid_at IS NOT NULL), :40), position numeric, created_by uuid FK users RESTRICT, created_at/updated_at, version bigint DEFAULT 1. Индексы :42-44.

**subtasks** (`migrations/0005_board.up.sql:46-62`, уже создана, Go читает только `OpenSubtasks`): id uuid PK, task_id bigint FK tasks CASCADE, service_id uuid FK services RESTRICT, status text CHECK IN('new','in_progress','done') DEFAULT 'new', assignee_id uuid FK users SET NULL, due_date date NULL, price numeric(12,2) NULL CHECK(≥0), equipment_id uuid FK equipment RESTRICT NULL, position int DEFAULT 0 (НЕ дробный, нет NextPosition-аналога). Индексы :59-62.

**subtask_products** (`migrations/0005_board.up.sql:64-72`, уже создана): id uuid PK, subtask_id uuid FK subtasks CASCADE, product_id uuid FK products RESTRICT, qty numeric(12,3) CHECK(>0) (D33 дробное), price numeric(12,2) NULL CHECK(≥0). Индексы :71-72.

**task_stock** (`migrations/0005_board.up.sql:74-80`, уже использует StockEngine): task_id bigint FK tasks CASCADE, product_id uuid FK products RESTRICT, qty numeric(12,3) CHECK(>0), PK(task_id,product_id).

**task_equipment** (`migrations/0005_board.up.sql:82-89`, создана, Go не читает/не пишет кроме E2E-фикстуры): task_id bigint FK tasks CASCADE, equipment_id uuid FK equipment RESTRICT, completeness text DEFAULT '', condition text DEFAULT '', PK(task_id,equipment_id).

**stock_movements** (создана `migrations/0004_catalog.up.sql:54-63`, task_id мигрирован uuid→bigint в `migrations/0005_board.up.sql:91-96`, `FK task_id→tasks ON DELETE SET NULL`): id uuid PK, product_id uuid FK products RESTRICT, task_id bigint NULL, qty_delta numeric(12,3) CHECK(<>0), reason text CHECK IN('task_consume','task_return','manual'), user_id uuid FK users RESTRICT, created_at.

**equipment** (`migrations/0003_clients_equipment.up.sql:33-47`): id uuid PK, client_id uuid FK clients CASCADE, type_id uuid FK equipment_types RESTRICT, brand/model(CHECK btrim<>'')/sku/serial text, specs jsonb DEFAULT '[]' CHECK(array), note text, created_at/updated_at. Индексы :46-47.

**equipment_types** (`migrations/0003_clients_equipment.up.sql:22-31`): id uuid PK, name text CHECK(btrim<>''), spec_keys text[] DEFAULT '{}', is_active boolean DEFAULT true; UNIQUE lower(btrim(name)) (:31).

**services/products/service_products** (`migrations/0004_catalog.up.sql:17-51`): services(id, category_id FK RESTRICT, name, price numeric(12,2) NULL, is_active) (:34-43); products(id, category_id, name, sku, unit, price numeric(12,2) DEFAULT 0, is_active, stock/min_stock numeric(12,3), box, place) (:17-31); service_products(service_id FK CASCADE, product_id FK RESTRICT, qty numeric(12,3) CHECK>0, position int) (:45-51).

**subtask_logs — ОТСУТСТВУЕТ.** Проверено: полный список миграций `migrations/0001…0005` (5 файлов up/down) не содержит `subtask_logs`. Журнал работ услуги (D13/D27, `docs/plan.md:56,63,138,151`) — НОВАЯ таблица фазы 6; ожидаемые колонки из `docs/plan.md:56`: `subtask_logs(id, subtask_id, user_id, text, created_at, updated_at)`.

## Frontend
Не применимо в терминах Vue — SSR (`Go html/template` + vanilla JS, `docs/plan.md:5`), `frontend/` отсутствует. Фактический "фронт" фичи ниже.

- `web/ssr/board/templates/pages/board.html:1` (133 строки) и `web/ssr/board/templates/pages/stock.html:1` (78 строк) — HTML-шаблоны доски/остатков.
- `web/ssr/_shared/js/board.js:548-667` (815 строк в файле): `openTaskForm(task, defaultColumnId)`(:548-569) — ТОЛЬКО простая форма (title/client/column/assignee/due/paid), без описания/оборудования/услуг/товаров/журнала/итогов; `taskFormPayload()`(:587-602) шлёт `{title,client_id,column_id,assignee_id,due_date,is_paid}` — is_paid ВСЕГДА отправляется даже при disabled-чекбоксе у executor (:590-593, важно сохранить при расширении — иначе тихий сброс оплаты); `bindTaskForm()`(:610-667) — submit→PUT/POST, catch различает `TASK_VERSION_CONFLICT`(:638-643) от `err.fields`(:644-646) от общей(:648); `finishOpenTaskForm`(:571-585) — `data-testid="task-delete"` снимается АТРИБУТОМ, не только `d-none` (:576-579) — конвенция для новых admin-only кнопок.
- `web/ssr/_shared/js/api.js:1` — `KT.api` fetch-обёртка с CSRF, используется всеми хендлерами.
- Общие компоненты: `web/ssr/_shared/js/combobox.js:1`, `web/ssr/_shared/js/ui-dialog.js:1`(`KT.uiConfirm/uiAlert`), `web/ssr/_shared/js/nav-badge.js:1`, `web/ssr/_shared/js/theme.js:1`.
- `e2e/pages/BoardPage.ts:1`, `e2e/pages/EquipmentPage.ts:1` и др. в `e2e/pages/` — POM; `e2e/fixtures/board.ts:1`, `e2e/fixtures/board-types.ts:1` — типы фикстур.

## Паттерны в коде
- Многосущностный feature-пакет: файлы-по-сущности (dto/handler/repo/service/validate) в ОДНОМ пакете `board` — эталон `internal/features/board/errors.go:1-4`; фаза 6 добавляет `subtasks`-файлы в ТОТ ЖЕ пакет, не отдельную фичу.
- Lock Order (ADR-104): `columns → tasks → products`, FOR UPDATE/FOR SHARE в этом порядке внутри `withTx`; новые мутации `subtasks`/`subtask_products` ДОЛЖНЫ брать `LockTask` ПЕРЕД правкой — см. LOW code review ниже (Граничные случаи).
- Re-fetch-after-write: ответ mutation-хендлера из `repo.Get()` после commit, не RETURNING (`internal/features/board/tasks_service.go:330-340`).
- Sentinel-ошибки — общие `var Err... = errorspkg.New(...)` на пакете (`internal/features/board/errors.go:15-22`), `.WithDetail` — КОПИЯ (`pkg/errorspkg/errors.go:36-44`), sentinel не мутируется.
- Optimistic locking (`version`) — только `clients`/`tasks` (D28, `docs/plan.md:152`); `subtasks`/`subtask_products` — last-write-wins без version (паттерн уже проверен на `tasks.Move`).
- DTO-валидация в методах запроса `(in Request) normalize()/validate()` (`internal/features/board/tasks_validate.go:30-100`), не в handler/service.
- Денежные/количественные поля — `pkg/numeric.Amount` (строка в Go, число на wire) — обязателен для `subtask_products.qty/price`, `subtasks.price`, итогов заказа.
- `withTx` с ретраем 40P01/40001×3 (`internal/features/board/tx.go:16-29`) — обязателен для мутаций склада/нескольких таблиц.
- Testid снимается атрибутом (не только `d-none`) для элементов, видимых только одной роли (`web/ssr/_shared/js/board.js:576-579`).

## Паспорт (выжимка)
# Passport: phase-6-order-card (generated — руками не править; stage_bundle.py)
**Tier:** M | **Совместимость:** ? (pipeline-doctrine.md §CM-1; ? → уточни у оркестратора)
**Трек:** обычный (prefs нет — дефолт) — режимы агентов (`spec-track delta/micro/mini/merged`) включает ТОЛЬКО строка `Режим:` в prompt оркестратора; DS/TS-индексы ниже присутствуют на ЛЮБОМ треке при наличии draft-spec.
**Layout:** автодетект (tools/layout_resolve.py)
**UI-scope (pipeline-doctrine.md §US-1, 0-setup/ui-scope-prefs.md):** swagger:no | adaptive:min | themes:both

**Индексы:** AC 17 / DS 0 / TS 0 — полные списки: `0-setup/passport.md` (разрешён контрактом); конкретное решение — точечный grep по `DS-NNN`/`TS-NNN` в draft-spec §9/§11 (разрешён).
