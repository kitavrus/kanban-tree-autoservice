---
stage: 3-design
feature: phase-5-board
tier: M
---
# Design: phase-5-board — доска заказов и «Остатки»
**Дата:** 2026-09-26
**Tier:** M | **Совместимость:** MVP (CM-1) | **UI-scope:** swagger:no · adaptive:min · themes:both
**На основе:** 3-design/input-bundle.md (Tech Spec 2-spec-interview/output.md + AC PRD + research-экстракты)

```yaml
diagrams:
  dataflow: no
  sequence: yes
  swimlane: yes
```

## Обзор
Фаза 5: доска (колонки D23/D32, заказы `#N` bigserial D37, простая форма D25), перенос = смена статуса с проверкой D11/D22 и транзакционным списанием/возвратом D17/D34, «Остатки» с прогнозом и бейджем D18, фильтры и роли D19, D26-гарды clients/equipment, `make seed-demo` D36. Схема услуг/товаров заказа создаётся полностью, наполняется только сидом (SD-001).

**Triage:** `scope: cross-feature, tier: M, touches: {db: true, fe: true, infra: false, external: false}, risk: reversible, novelty: standard-crud, decisions: []` (источник: 1-research/output.md §Triage).

## Детальные документы (owner-файлы)
`design-sql.md` · `design-errors.md` · `design-go-layers.md` · `design-tests.md` · `design-sequence-diagrams.md` (все в `3-design/`).

## Контракт
Все под `/api/v1`, сессия + CSRF каркаса; JSON snake_case; списки `{items,total,limit,offset}` (CLAUDE.md §API-1). DTO — design-go-layers.
| Метод | Путь | Auth | Request | Response | Driver |
|---|---|---|---|---|---|
| GET | `/api/v1/columns` | любой | — | `ColumnListResponse` | AC-001 |
| POST | `/api/v1/columns` | admin | `ColumnInput` | 201 `ColumnMutationResponse` | AC-006 |
| PUT | `/api/v1/columns/:id` | admin | `ColumnInput` | 200 `ColumnMutationResponse` | AC-006, AC-007 |
| DELETE | `/api/v1/columns/:id` | admin | — | 204 | AC-006 |
| GET | `/api/v1/tasks` | любой | query `TaskFilter` | `TaskListResponse` | AC-001, AC-013, AC-014 |
| POST | `/api/v1/tasks` | любой (`is_paid=true` — admin, иначе 403 `PAID_ADMIN_ONLY`) | `TaskCreateRequest` | 201 `TaskMutationResponse` | AC-009 |
| GET | `/api/v1/tasks/:id` | любой | — | `TaskResponse` | AC-009 |
| PUT | `/api/v1/tasks/:id` | любой (смена `is_paid` — admin, иначе 403 `PAID_ADMIN_ONLY`) | `TaskUpdateRequest` (+`version`) | `TaskMutationResponse` | AC-009, AC-010, AC-011 |
| POST | `/api/v1/tasks/:id/move` | любой | `TaskMoveRequest` | `TaskMutationResponse` | AC-002…AC-005 |
| DELETE | `/api/v1/tasks/:id` | admin | — | 200 `TaskDeleteResponse` | AC-012 |
| GET | `/api/v1/stock` | любой | `scope,q,limit,offset` | `StockListResponse` | AC-015 |
| GET | `/api/v1/stock/summary` | любой | — | `StockSummaryResponse` | AC-015, AC-016 |
| PUT | `/api/v1/stock/:product_id` | admin | `StockAdjustRequest` | `StockRow` | AC-015 |
| GET | `/api/v1/assignees` | любой | `limit,offset` | `AssigneeListResponse` | AC-009 |
| GET | `/` , `/stock` | SSR любой | — | HTML (каркас; данные — JS) | AC-001, AC-015 |
- Порядок и пагинация списков (контракт): `GET /columns` — `position, id`, limit 100 (1..1000); `GET /tasks` — `columns.position, columns.id, tasks.position, tasks.id` (слева направо, сверху вниз, стабилен), limit 500 (1..1000); `GET /stock` — `stock−min_stock, name, id`, limit 200 (1..1000); `GET /assignees` — `name, id`, limit 200 (1..1000); offset по умолчанию 0.
- DELETE: `/columns/:id` → 204 без тела (склад не затрагивается); `/tasks/:id` → 200 `{warnings}` (возврат товаров, ADR-006) — различие намеренное.
- 403: `FORBIDDEN` — маршрут закрыт ролью; `PAID_ADMIN_ONLY` — запрещено только изменение `is_paid` (design-errors §Codes 01-15).

Изменённые as-is: `DELETE /api/v1/clients/:id`, `DELETE /api/v1/equipment/:id` (+SSR `POST .../delete`) → 409 `CLIENT_HAS_TASKS`/`EQUIPMENT_USED_IN_TASKS`; `DELETE /api/v1/services/:id` → 409 `SERVICE_IN_USE`.

## Impact
Перечень затронутых файлов — design-go-layers §Package Layout («Прочие изменения»): удаление `home`, `ssr` бейдж, `router.go`, `errorspkg` (ADR-102), `pkg/numeric` (ADR-101), `clients`/`catalog` (ADR-005), `0005_board`, `seeddemo` (ADR-105), `e2e/package.json` `pg` (ADR-103).

## Enums и константы
| Поле (DTO.json) | Значения | Источник правды | Sync-тест |
|---|---|---|---|
| `subtasks.status` / `details.subtasks[].status` | `new, in_progress, done` | `internal/features/board/enums.go::SubtaskStatuses` = CHECK 0005 | `board/enums_test.go` + live-сверка CHECK |
| query `paid` | `all, paid, unpaid` | `board/enums.go::PaidFilters` | `tasks_filter_test.go` |
| query `due` | `all, soon, overdue` | `board/enums.go::DueFilters` | `tasks_filter_test.go` |
| query `assignee` | `me, all, <uuid>` | `board/enums.go` | `tasks_filter_test.go` |
| query `scope` (stock) | `risky, all` | `board/enums.go::StockScopes` | live LV-037 |
| `StockRow.state` | `out, low, will_low, ok` | SQL CASE + `board/enums.go::StockStates` | live LV-012 |
| `StockWarning.kind` | `negative, below_min` | `board/enums.go::WarningKinds` | live LV-007 |
| `stock_movements.reason` | `task_consume, task_return, manual` | `migrations/0004_catalog.up.sql` (as-is) | — |

## ADR

### ADR-001: Дробная позиция карточки через соседей
- **Статус:** Принято
- **Driver:** AC-002 — «Перенос карточки мышью в другую колонку меняет статус заказа. Перенос внутри колонки меняет порядок карточек. После перезагрузки страницы положение сохраняется (source: D7).»
- **Отвечает на:** Q-001 — «Алгоритм дробной позиции карточки»
- **Контекст:** клиент не знает актуальных позиций соседей при устаревшей доске; LWW без version (SD-006).
- **Решение:** `POST /tasks/:id/move {column_id, prev_id, next_id}` (prev — карточка сверху, next — снизу, оба nullable). Сервер под блокировкой: оба соседа → `trim_scale((prev+next)/2)`; только prev → prev+1024; только next → next−1024; ни одного → max(колонки)+1024 (пустая → 1024). Сосед не из целевой колонки / не найден / = сам заказ — игнорируется (не 400: доска могла устареть). Ребалансировка колонки (`rn*1024`, без инкремента version других заказов) при `prev>=next` или `scale>20`. PUT/POST со сменой колонки — в конец. design-sql §Position Query.
- **Альтернативы:** клиент присылает `position` (доверие устаревшим данным, коллизии); целочисленный индекс со сдвигом всех (N UPDATE на каждый перенос).
- **Риски:** рост точности NUMERIC — ограничен ребалансировкой; одинаковые позиции при гонке — детерминированный порядок `(position, id)`.

### ADR-002: Прогноз «Остатков», KPI и счётчик
- **Статус:** Принято
- **Driver:** AC-015 — «Все сотрудники видят экран «Остатки». Показатели: сколько товаров на минимуме, сколько нет в наличии, сколько уйдут в минимум после заказов. Таблица рисковых товаров с прогнозом, который учитывает ещё не взятые в работу заказы. Править остаток может только администратор (source: D18, D19).»
- **Отвечает на:** Q-002 — «Формула прогноза «Остатков» и KPI»
- **Контекст:** прототип (`prototype/index.html:1637-1651`) считает pending по заказам не в списывающих и не в финальных колонках.
- **Решение:** pending = Σ `subtask_products.qty` заказов в колонках `NOT consumes_stock AND NOT is_final`; forecast = stock − pending. Состояние (взаимоисключающее): `out` stock≤0; `low` 0<stock≤min_stock; `will_low` stock>min_stock И forecast≤min_stock; иначе `ok`. KPI = count по состояниям; рисковые = `state<>ok`; бейдж = `risk_count`. Только `products.is_active`. Сортировка `stock−min_stock, name`. design-sql §Stock Pending CTE.
- **Альтернативы:** бейдж только «на минимуме» (как прототип `refreshLowBadge`) — расходится со spec «рисковые»; учёт архивных товаров — шум.
- **Риски:** архивный товар в минусе не виден в «Остатках» — приемлемо для MVP (архивируют неиспользуемое).

### ADR-003: Контракт фильтров доски и загрузка одним запросом
- **Статус:** Принято
- **Driver:** AC-013 — «Доску можно отфильтровать по клиенту, тексту (номер или название), виду работ, оплате, исполнителю и сроку. Фильтры сочетаются между собой, и их можно сбросить (source: D19).»; AC-014 — «Исполнитель при открытии доски видит только свои заказы: те, где он ответственный или исполнитель хотя бы одной услуги. Фильтр снимается, и тогда видна вся доска. Администратор по умолчанию видит всё (source: D19).»
- **Отвечает на:** Q-003 — «Контракт фильтров доски»
- **Контекст:** прототип фильтрует на клиенте (`prototype/index.html:778-796`); у нас данные из API.
- **Решение:** `GET /api/v1/tasks?client_id&q&category_id&paid=all|paid|unpaid&assignee=me|all|<uuid>&due=all|soon|overdue&limit&offset` — серверная фильтрация, AND-комбинация. `q`: ILIKE по названию и клиенту + точный `#N`/`N`. `due` по прототипу: `soon` = срок ≤ завтра (включая просрочку) ИЛИ просроченная услуга; `overdue` = срок < сегодня ИЛИ просроченная услуга; финальные колонки исключены. `assignee` отсутствует → дефолт по роли на сервере (executor=`me`). Доска = `GET /columns` + ОДИН `GET /tasks` (limit по умолчанию 500, max 1000; при `total>limit` — баннер «показаны первые N»). Состояние — в `location.search` (reload сохраняет, «Сбросить» → дефолты роли).
- **Альтернативы:** загрузка по колонкам (N запросов, сложный DnD между загруженными страницами); клиентская фильтрация всего набора (рост объёма, расхождение с API).
- **Риски:** `current_date` — TZ БД; для одной мастерской совпадает с локальной (MVP).

### ADR-004: Бейдж «Остатков» через ShellBuilder
- **Статус:** Принято
- **Driver:** AC-016 — «У пункта меню «Остатки» есть счётчик рисковых товаров, видимый всем сотрудникам (source: D18).»
- **Отвечает на:** Q-004 — «Доставка счётчика «Остатков» в меню»
- **Контекст:** `ssr.NavItem` без бейджа (`internal/ssr/menu.go:4-9`); `ShellBuilder` — value-тип без БД.
- **Решение:** `NavItem.Badge int`; `ssr.BadgeSource{NavBadges(ctx) map[string]int}` + `ShellBuilder.WithBadges(src)`; `board.StockService` реализует (Stock Summary Query, 1 SQL на SSR-рендер; ошибка → бейдж скрыт + `slog.Warn`). В `base.html` span `data-nav-badge="/stock"` рендерится всегда (hidden при 0). Инвалидация без перезагрузки: `KT.refreshNavBadge()` (`nav-badge.js`) → `GET /api/v1/stock/summary` после move/PUT/POST/DELETE заказа, PUT колонки, PUT остатка.
- **Альтернативы:** только DOM-инъекция JS (мигание, лишний запрос на каждой странице); кеш в Redis (инвалидация сложнее выгоды).
- **Риски:** +1 запрос на каждый SSR-рендер — каталог мастерской мал (spec «Принятые компромиссы»).

### ADR-005: D26-гарды в clients/catalog без импорта board
- **Статус:** Принято
- **Driver:** AC-017 — «При попытке удалить клиента, у которого есть заказы, пользователь видит объяснение, и клиент не удаляется (source: D26).»; AC-018 — «При попытке удалить оборудование, которое хотя бы раз было в заказе, пользователь видит объяснение, и оборудование не удаляется (source: D26).»
- **Отвечает на:** Q-005 — «Размещение D26-проверок фазы 3»
- **Контекст:** FK RESTRICT (0005) дали бы 23503 → 500 в `clients`/`catalog.services`.
- **Решение:** `clients.Repo.DeleteClient`/`DeleteEquipment` — SQL EXISTS (`tasks.client_id`; `task_equipment` ∪ `subtasks.equipment_id`) → 409 `CLIENT_HAS_TASKS`/`EQUIPMENT_USED_IN_TASKS`; страховка от гонки — 23503 по `ConstraintName` → тот же 409. Пакеты зависят только от таблиц, не от Go-пакета `board`. Дополнительно: `catalog.services` 23503 → 409 `SERVICE_IN_USE`; текст `PRODUCT_IN_USE` обобщён. SSR-удаление показывает `Message` через as-is flash.
- **Альтернативы:** вызов `board`-сервиса из `clients` (циклическая связность фич); только FK-маппинг (текст ошибки зависит от порядка constraint'ов).
- **Риски:** SQL-знание о чужих таблицах в clients — фиксируется live-тестами LV-056…LV-059.

### ADR-006: Единый формат warnings
- **Статус:** Принято
- **Driver:** AC-005 — «Если для списания товара не хватает, заказ всё равно переносится, остаток уходит в минус, а пользователь видит предупреждение с перечнем таких товаров. Если после списания товар оказался на минимуме или ниже, пользователь видит предупреждение со ссылкой на «Остатки» (source: D17, D18).»
- **Отвечает на:** Q-006 — «Формат `warnings` в ответах»
- **Контекст:** предупреждения нужны в move, PUT задачи, POST задачи, PUT колонки, DELETE задачи.
- **Решение:** `warnings: [{product_id, name, stock, min_stock, kind}]`, `kind = negative` (stock<0) | `below_min` (0≤stock≤min_stock); только по товарам с ненулевой дельтой в этой операции; значения после изменения (в той же транзакции); всегда массив (`[]`, не null). Обёртки ответов: `{task, warnings}`, `{column, warnings}`, `{warnings}`. TS-зеркало — `e2e/fixtures/board-types.ts` (§TD-6.2).
- **Альтернативы:** заголовок `X-Warnings`; отдельный GET после действия (лишний запрос, гонка).
- **Риски:** — .

### ADR-100: Один пакет `board`, доска заменяет `home`
- **Статус:** Принято
- **Driver:** AC-001 — «После входа пользователь видит доску: колонки в заданном порядке, в каждой карточки заказов. На карточке видны номер `#N`, название, клиент, ответственный, срок и признак оплаты. Цвет карточки совпадает с цветом колонки (source: D7, D37).»
- **Отвечает на:** — (spec §Применённые правила: «`board` или `board`+`stock` — решает design»)
- **Контекст:** склад (Reconcile) неразрывен с move/колонками; «Остатки» читают те же таблицы.
- **Решение:** один пакет `internal/features/board` с префиксами файлов (эталон `catalog`); `StockEngine` экспортирован для `seeddemo`. `home` удаляется, `GET /` регистрирует board. Код вручную (D43).
- **Альтернативы:** `board`+`stock` — межпакетная зависимость ради одного Reconcile.
- **Риски:** крупный пакет — смягчается префиксами файлов.

### ADR-101: Перенос `Amount` в `pkg/numeric`
- **Статус:** Принято
- **Driver:** AC-015 — «Все сотрудники видят экран «Остатки». … Таблица рисковых товаров с прогнозом … Править остаток может только администратор (source: D18, D19).»
- **Отвечает на:** —
- **Контекст:** NUMERIC-паттерн `catalog/amount.go` нужен и board (stock/forecast/warnings); импорт фичи из фичи запрещён архитектурой.
- **Решение:** `pkg/numeric.Amount` + валидатор диапазона; `catalog.Amount = numeric.Amount` (alias — код catalog не меняется).
- **Альтернативы:** копия в board (дубль логики точности); импорт `catalog` из `board`.
- **Риски:** регресс catalog — покрыт существующими live-тестами фазы 4.

### ADR-102: `details` в errorspkg для не-валидационных ошибок
- **Статус:** Принято
- **Driver:** AC-003 — «Если у заказа есть незавершённые услуги, перенести его в финальную колонку нельзя: карточка возвращается на место, пользователь видит сообщение со списком незавершённых услуг. Заказ, у которого все услуги выполнены, переносится свободно (source: D11, D22).»
- **Отвечает на:** —
- **Контекст:** `errDetails` умеет только `fields`.
- **Решение:** `AppError.Extra` + `WithDetail(key, v)` (возвращает копию; sentinel'ы не мутируются); handler сливает `fields` и `Extra` в `details`. design-errors §errorspkg Extension.
- **Альтернативы:** 200 с флагом ошибки; отдельный тип ответа только для move.
- **Риски:** гонка при мутации общего sentinel — исключена копированием (unit-тест).

### ADR-103: Услуги заказа в E2E — через `pg` в фикстуре
- **Статус:** Принято
- **Driver:** AC-003 — «Если у заказа есть незавершённые услуги, перенести его в финальную колонку нельзя … (source: D11, D22).»; AC-018 — «При попытке удалить оборудование, которое хотя бы раз было в заказе, пользователь видит объяснение … (source: D26).»
- **Отвечает на:** —
- **Контекст:** API услуг заказа нет до фазы 6 (SD-001); seed-demo — только на пустой базе и общий стенд.
- **Решение:** `e2e/fixtures/board.ts` вставляет `subtasks`/`subtask_products`/`task_equipment` для заказа, созданного через API, по `E2E_DATABASE_URL` (дефолт — DSN из `.env` локального стенда). Новая devDependency `pg` + `@types/pg` — только e2e, в бинарь/образ не попадает. Удалится/заменится API в фазе 6.
- **Альтернативы:** тестовый HTTP-эндпоинт в сервере (поверхность атаки в проде); `go run` вспомогательной cmd из теста (медленно, хрупко).
- **Риски:** E2E знает схему БД — локализовано одной фикстурой.

### ADR-104: Транзакция склада и порядок блокировок
- **Статус:** Принято
- **Driver:** AC-004 — «Когда заказ попадает в колонку «Списывать товары», товары заказа автоматически списываются со склада. При возврате заказа в колонку без списания они возвращаются на склад. Повторный перенос между списывающими колонками ничего не списывает повторно (source: D17, D34).»; AC-007 — «Если у колонки с заказами включить или выключить «списывать товары», склад сразу пересчитывается для всех её заказов: товары списываются или возвращаются (source: D23, D17).»
- **Отвечает на:** —
- **Контекст:** move ‖ toggle колонки ‖ move соседнего заказа с общим товаром.
- **Решение:** `StockEngine.Reconcile` идемпотентен (цель − `task_stock`). Глобальный порядок: `columns` (target FOR SHARE / вся таблица FOR UPDATE при CRUD колонок) → `tasks` (FOR UPDATE, по id) → `products` (FOR UPDATE, по id). `withTx` повторяет 40P01/40001 до 3 раз. design-sql §Lock Order.
- **Альтернативы:** SERIALIZABLE (частые повторы на горячем move); advisory lock на весь склад (сериализация всей доски).
- **Риски:** долгий toggle колонки с сотнями заказов держит блокировку колонки — масштаб мастерской мал.

### ADR-105: `make seed-demo` через пакет `seeddemo`
- **Статус:** Принято
- **Driver:** AC-019 — «Команда `make seed-demo` на локальной базе без заказов наполняет систему демо-данными прототипа: клиенты, оборудование, каталог, заказы во всех колонках с услугами и товарами. Если заказы уже есть, команда ничего не меняет и сообщает об этом (source: D36, SD-003).»
- **Отвечает на:** —
- **Решение:** `internal/seeddemo.Run(ctx, pool)` (тестируемо, IT-001) + тонкий `cmd/seed-demo`; guard `count(tasks)=0`, иначе сообщение и exit 0; нужен активный admin (created_by/user_id журнала), иначе exit 1 «make local-up-seed». Демо-исполнители — `ON CONFLICT (login) DO NOTHING`, админ не трогается. Позиции товаров создаются с начальным stock + журнал `manual`, заказы — через `StockEngine.Reconcile` (инвариант склада сходится). Всё в одной транзакции. Цель Makefile — вне блока `project-bootstrap`.
- **Альтернативы:** флаг `cmd/seed -demo` (смешение идемпотентного сида админа и демо).
- **Риски:** дрейф демо от прототипа — данные фиксированы в Go-литералах пакета.

### ADR-106: Перенос на 360 px — меню карточки
- **Статус:** Принято
- **Driver:** AC-020 — «Доска, форма заказа и «Остатки» работают на экране шириной 360 px без горизонтальной прокрутки страницы (доска прокручивается по колонкам внутри себя) и корректно выглядят в светлой и тёмной теме (source: US-1 adaptive:min, themes:both).»
- **Отвечает на:** —
- **Контекст:** нативный HTML5 DnD не работает на touch.
- **Решение:** десктоп — нативный HTML5 DnD (без библиотек); на всех ширинах у карточки кнопка `⋯` (Bootstrap dropdown) «Переместить в → <колонка>» (в конец колонки, тот же `move` с `prev_id/next_id=null`) — также клавиатурная альтернатива. `.board-columns` — единственный горизонтальный скролл; токены `--bs-*`.
- **Альтернативы:** touch-polyfill/SortableJS (новая зависимость); long-press drag вручную.
- **Риски:** на телефоне нет перестановки внутри колонки — не требуется AC-020.

## Security
- Роли: гарды `middleware.RequireAdmin(nil)` per-route (columns write, DELETE tasks, PUT stock) + проверка `is_paid` в сервисе (AC-011). CSRF каркаса на всех write.
- Id: `tasks.id` — `ParseInt` до SQL; uuid — `uuid.Validate`. SQL — только параметры; `q` экранирован для ILIKE.
- Новая зависимость: `pg`/`@types/pg` — devDependency `e2e/` (ADR-103). Go-зависимостей не добавляется.
- JS рендерит пользовательский текст через `textContent`/экранирование (без `innerHTML` с данными); `description_html` в фазе 5 не выводится.

## Monitoring & Alerting Spec
| Span (Tracer `kanban-tree/board`) | Log fields | Когда |
|---|---|---|
| `tasks.move` | `trace_id`, `task_id`, `from_column`, `to_column`, `movements_count` | каждый move |
| `stock.reconcile` | `trace_id`, `task_id`, `products_count`, `negative_count` | каждый Reconcile |
| `columns.recalc` | `trace_id`, `column_id`, `tasks_count` | смена `consumes_stock` |
- Метрика `stock_reconcile_errors_total` (counter, `internal/observability`); латентность move — as-is `http_request_duration_seconds`.
- Алерты (move p95 > 0.5 с/10 мин; reconcile errors > 0/5 мин): deferred — прод-мониторинг не развёрнут, подключение при `/infra-bootstrap` (трекинг docs/plan.md, инфра-фаза).

## Test Strategy
`3-design/design-tests.md`: AC-001…AC-020 1:1 (E2E BD-NNN; AC-019 — IT-001), TD-4, гонки, инвариант склада, fuzz.

## Performance Budget
| Endpoint | p95 | Driver |
|---|---|---|
| `POST /api/v1/tasks/:id/move` | < 500 мс | PRD §3 — «Перенос карточки между колонками, p95 · < 500 мс · сразу» |
| `GET /api/v1/tasks` (≤500 карточек) | < 500 мс | AC-001 |
- Load-test: deferred — прода нет (CM-1 MVP); контроль — метрика `http_request_duration_seconds` после `/infra-bootstrap` (docs/plan.md, инфра-фаза).

## Runbook (post-deploy)
1. `make migrate-up` (0005) → `make seed-demo` на локальной БД.
2. Smoke: DnD «Согласование → В работе → Готово» (заказ с незавершённой услугой — отказ со списком); «Остатки» — KPI/бейдж.
3. Сверка склада по заказам (ожидается 0 строк): `SELECT ts.task_id, ts.product_id FROM task_stock ts LEFT JOIN (SELECT task_id, product_id, -sum(qty_delta) q FROM stock_movements WHERE task_id IS NOT NULL GROUP BY 1,2) m USING (task_id, product_id) WHERE ts.qty <> coalesce(m.q, 0)`.
4. Читатель — Dev (владелец). Дашборды/алерты — deferred (см. Monitoring).

## Rollout Strategy
Не применимо (CM-1 MVP).

## Verification
- `make test` · `make test-integration-scoped` · `go test -fuzz=Fuzz... -fuzztime=30s ./internal/features/board/`
- `npx playwright test tests/board.spec.ts tests/stock.spec.ts tests/board-guards.spec.ts --project=chromium --headed --workers=1` (оркестратор)
