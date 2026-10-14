---
stage: 2-spec-interview
feature: phase-5-board
tier: M
---
# Spec: Доска заказов и «Остатки» (фаза 5)
**Дата интервью:** 2026-09-26
**На основе:** PRD `2-spec-interview/prd.md` (approved) + input-bundle (research-экстракты) + docs/plan.md D1–D44 + `spec-decisions.md` SD-001…SD-008
**Tier:** M
**Совместимость:** MVP (CM-1)
**UI-scope (US-1):** swagger:no | adaptive:min | themes:both

## Применённые правила (zap-list)
- Ошибки — только `pkg/errorspkg`, единый wire-формат; 400 → `details.fields[]` (CLAUDE.md проекта).
- 404 / 403 / 409 — стандартные статусы; auth — Redis-сессия + CSRF каркаса (D35), гарды `middleware.RequireAdmin(nil)` per-route.
- Синхронная обработка, без очередей; аудит-лог не вводится (slog + `trace_id`); журнал склада — `stock_movements` (D34, D44).
- Миграции — только `make migrate-up`, без auto-apply.
- URL kebab-case + `/api/v1`, JSON snake_case, list-эндпоинты — конверт `{items,total,limit,offset}` (CLAUDE.md §API-1).
- NUMERIC — `type Amount string`-паттерн (`internal/features/catalog/amount.go`), не `float64`.
- `version` в теле (`version *int64`), не If-Match (§FE-1.1); только у clients и tasks (D28).
- Tracer — `otel.Tracer("kanban-tree/{component}")` по паттерну каркаса.
- Тесты — `pkg/testutil.LiveSuite` (§TD-8), auth-стабы запрещены; E2E — POM на экран + fixture API.
- Каталоги Go — §GO-1: новая фича `internal/features/board` (или `board`+`stock` — решает design), файлы snake_case.

## Технические ограничения

### Схема (source: SD-001, SD-002, SD-007)
Одна новая миграция `0005_*` создаёт:
- `columns(id uuid, name, color, is_final, consumes_stock, position)` + сид D32 в самой миграции (driver: AC-008).
- `tasks(id BIGSERIAL, column_id→columns RESTRICT, client_id→clients RESTRICT NOT NULL, title, description_html, assignee_id→users NULL, due_date NULL, is_paid, paid_at NULL, position NUMERIC, created_by→users, created_at, updated_at, version)` (source: D37; driver: AC-009).
- `subtasks`, `subtask_products`, `task_stock(task_id, product_id, qty)`, `task_equipment` — полная схема по docs/plan.md; UI правки нет (D25), наполнение только seed-demo (driver: AC-003, AC-004, AC-013, AC-015, AC-018).
- `ALTER stock_movements.task_id TYPE bigint` (данных с `task_id` нет — все строки `manual`) + FK `→ tasks(id) ON DELETE SET NULL` (source: SD-002, SD-007).
- FK `clients`→`tasks` RESTRICT и `equipment`→`task_equipment` RESTRICT — физическая опора D26-проверок; сервис отвечает 409 до ошибки FK (driver: AC-017, AC-018).
- `ON DELETE` для `subtasks/subtask_products/task_stock/task_equipment` от `tasks` — CASCADE (удаление заказа, AC-012).

### Списание (source: D17, D34)
- Потребность заказа = Σ `subtask_products.qty` по product_id. `task_stock` = фактически списанное по заказу.
- Функция пересчёта `reconcile(task)`: целевое = потребность, если колонка `consumes_stock`, иначе 0; дельта = целевое − `task_stock`; для каждой ненулевой дельты: `products.stock -= дельта`, upsert `task_stock`, строка `stock_movements` (`task_consume` при дельте>0 → `qty_delta<0`; `task_return` → `qty_delta>0`) с `task_id`, `user_id`.
- Единая транзакция: `SELECT … FOR UPDATE` строки(строк) `tasks`, затем `products` в порядке id (D34). Вызывается из: move, PUT со сменой колонки, DELETE заказа (целевое=0 до удаления), смена `consumes_stock` колонки (по всем её заказам в одной транзакции, D23).
- Минус допустим; после коммита ответ несёт `warnings` (товары с `stock < 0` и `stock ≤ min_stock`) (driver: AC-005).
- Инвариант метрики PRD §3: `products.stock` = начальный + Σ `stock_movements.qty_delta` — проверяется integration-тестом.

### Перенос (source: SD-006)
- `POST /api/v1/tasks/:id/move` `{column_id, position}` (или соседи — Q-001), без `version`; last-write-wins; инкремент `version`.
- В `is_final`-колонку при наличии `subtasks.status<>'done'` → 409 `SUBTASKS_NOT_DONE` + список (D11, D22). Та же проверка в `PUT` при смене `column_id`. Проверки D22 «вернуть услугу из done / добавить услугу в финальной» — эндпоинтов услуг в фазе 5 нет, реализуются в фазе 6.
- Бюджет: p95 < 500 мс (driver: PRD §3).

### Внешние зависимости и инфра
- Новых сервисов/env нет; Redis — только сессии каркаса. CI без изменений.
- `make seed-demo` → `cmd/seed-demo` (или флаг `cmd/seed`): грузит демо прототипа только при `SELECT count(*) FROM tasks = 0`, иначе сообщение и exit 0; админа не трогает; списания в сиде — через тот же `reconcile`, чтобы журнал и остаток сходились (source: SD-003, D36; driver: AC-019).

### UI (D30, D43)
- Доска `/` и «Остатки» `/stock` — rich-экраны на vanilla JS поверх JSON API; каркас страницы SSR-layout `base.html`; `/stock` добавить в `enabled` ShellBuilder (`router.go`).
- Формы колонки и заказа — модалки Bootstrap по эталону prototype/index.html; клиент — `combobox.js` (D6); подтверждения — `ui-dialog.js`.

## Enum-поля фичи (CLAUDE.md §8.1)
| Поле (DTO.json) | Допустимые значения | Источник правды |
|---|---|---|
| `subtasks.status` (в фильтрах/ответе доски) | `new, in_progress, done` | миграция 0005 CHECK + Go-константы фичи |
| `stock_movements.reason` | `task_consume, task_return, manual` | `migrations/0004_catalog.up.sql` (без изменений) |
| фильтр `paid` | `all, paid, unpaid` | Go-константы фичи (Q-003) |
| фильтр `due` | см. Q-003 | Go-константы фичи |

## Безопасность и доступ
| Действие | admin | executor | Driver |
|---|---|---|---|
| Просмотр доски, фильтры, «Остатки», счётчик | да | да | AC-001, AC-015, AC-016 |
| Создание/правка заказа, move | да | да | AC-009, AC-002 (D19) |
| Изменение `is_paid` | да | 403, если значение отличается от текущего | AC-011 (SD-008) |
| Удаление заказа | да | 403 | AC-012 (D19) |
| CRUD колонок | да | 403, UI скрыт | AC-006 (SD-005) |
| Правка остатка в «Остатках» | да (manual, D44) | 403, UI только просмотр | AC-015 |
- Все write-эндпоинты — CSRF каркаса. Id: `uuid.Validate` для uuid-сущностей; `tasks.id` — парсинг int64 до SQL (невалидный → 404).
- Чувствительных данных нет; 403 vs 404 — ресурс не скрывается (одна мастерская).

## UI/UX требования
- Drag&drop карточки: оптимистичный перенос; при 409/ошибке — откат карточки + сообщение `ui-dialog`/toast со списком незавершённых услуг (AC-003).
- `warnings` из ответа move/PUT/смены колонки → toast с перечнем и ссылкой на `/stock` (AC-005).
- Фильтр «исполнитель» у executor по умолчанию «мои» (ответственный ИЛИ исполнитель услуги), admin — «все» (AC-014); состояние фильтров — в URL query (Q-003).
- 360 px: горизонтальный скролл только у контейнера колонок, `hasHorizontalOverflow()` страницы = false; обе темы через `--bs-*` токены (AC-020).
- Повторный клик «Сохранить» — кнопка блокируется до ответа.

## Обработка ошибок
| Ошибка | Что видит пользователь | Действие системы | Driver |
|---|---|---|---|
| 409 `SUBTASKS_NOT_DONE` (`details.subtasks[]`) | карточка вернулась, список незавершённых услуг | транзакция откатывается | AC-003 |
| 409 `COLUMN_NOT_EMPTY` | «В колонке есть заказы» | колонка не удаляется | AC-006 |
| 409 `LAST_NON_FINAL_COLUMN` | «Нужна хотя бы одна нефинальная колонка» | удаление/смена `is_final` отклонены | AC-006 |
| 409 `TASK_VERSION_CONFLICT` | «Заказ уже изменён, обновите» | правка не применяется | AC-010 |
| 409 `CLIENT_HAS_TASKS` | «У клиента есть заказы» | клиент не удаляется | AC-017 |
| 409 `EQUIPMENT_USED_IN_TASKS` | «Оборудование было в заказе» | не удаляется | AC-018 |
| 403 | «Недостаточно прав» | — | AC-006, AC-011, AC-012 |
| 404 заказ/колонка/клиент/ответственный | «Не найдено» (в форме — ошибка поля) | — | AC-009 |
| 400 валидация (`title`, `client_id` обязательны; `color` hex; `name` не пустой) | ошибки полей `details.fields[]` | — | AC-009, AC-006 |
| 200 + `warnings[]` (минус / ≤ минимума) | toast-предупреждение | списание выполнено | AC-005 |
- Всё-или-ничего: каждая операция склада — одна транзакция; retry ручной (повтор действия пользователем).

## Операции и наблюдаемость (M)
### Spans
| Имя | Tracer | Log fields | Когда |
|---|---|---|---|
| `tasks.move` | `kanban-tree/board` | `trace_id`, `task_id`, `from_column`, `to_column`, `movements_count` | каждый move |
| `stock.reconcile` | `kanban-tree/board` | `trace_id`, `task_id`, `products_count`, `negative_count` | каждый пересчёт |
| `columns.recalc` | `kanban-tree/board` | `trace_id`, `column_id`, `tasks_count` | смена `consumes_stock` |

### Метрики и алерты
| Метрика | Порог алерта | Действие on-call |
|---|---|---|
| `http_request_duration_seconds{route="/api/v1/tasks/:id/move"}` p95 | > 0.5 с за 10 мин (driver: PRD §3) | Jaeger по `trace_id`, проверить блокировки `FOR UPDATE` |
| `stock_reconcile_errors_total` | > 0 за 5 мин | логи `stock.reconcile`, сверить остаток vs журнал |
- Прод-алертинг проекта ещё не развёрнут (`/infra-bootstrap` не проводился): правила фиксируются в спеке, подключение — при `/infra-bootstrap` (deferred, трекинг — docs/plan.md, инфра-фаза).
- Runbook читает Dev (владелец). После деплоя: `make seed-demo` локально, smoke drag&drop «Согласование → В работе → Готово», сверка остатка с журналом.

## Принятые компромиссы
- Услуги/товары заказа в фазе 5 только из сида (SD-001) — редактирование и D22-запреты на уровне услуг в фазе 6.
- move без `version` (SD-006) — одновременный перенос одного заказа: побеждает последний.
- Физическое удаление заказа, журнал теряет связь с заказом (SD-007).
- Номер `#id` bigserial не переиспользуется, пропуски после удаления допустимы (D37).
- Счётчик «Остатков» считается на каждый рендер layout (объём каталога мастерской мал) — Q-004.

## Отклонённые решения
- uuid PK + отдельный номер у `tasks` — против D37 (SD-002).
- Перенос схемы услуг в фазу 6 — ломает D11/D17/E2E фазы 5 (SD-001).
- Мягкое удаление заказов — фильтрация во всех выборках ради одной связи в журнале (SD-007).
- `version` на move — частые отказы drag&drop при устаревшей доске (SD-006).
- drag&drop колонок — порядок полем «позиция» (SD-005).
- Блок «услуги» в простой форме — расширяет D25 (SD-001).

## Открытые вопросы

### Q-001: Алгоритм дробной позиции карточки
Контракт move: клиент присылает `position` или id соседей (`before_id/after_id`), сервер считает середину. Нужны правило вставки в пустую колонку/край и ребалансировка при исчерпании точности NUMERIC. Design фиксирует контракт и правило.

### Q-002: Формула прогноза «Остатков» и KPI
«Ещё не взятые в работу заказы» = заказы в колонках без `consumes_stock` и не `is_final`; прогноз = `stock − Σ потребности таких заказов`. KPI: на минимуме (`0 < stock ≤ min_stock`), нет в наличии (`stock ≤ 0`), уйдут в минимум (прогноз ≤ `min_stock` при текущем > `min_stock`). Рисковые = объединение трёх множеств; счётчик меню = размер этого множества. Design подтверждает по прототипу и фиксирует SQL.

### Q-003: Контракт фильтров доски
Параметры query (`client_id`, `q`, `category_id` — «вид работ» по категории услуг subtasks, `paid`, `assignee` = `me|all|<user_id>`, `due` — набор значений по прототипу: просрочен/сегодня/неделя/без срока); хранение в URL; загрузка доски — один list-запрос со всеми карточками (limit по умолчанию достаточно большой) vs по колонкам. Design фиксирует с опорой на prototype/index.html.

### Q-004: Доставка счётчика «Остатков» в меню
Расширение `ssr.NavItem` полем бейджа + вычисление в `ShellBuilder.Build` (один SQL на рендер) vs DOM-инъекция JS после `GET /api/v1/stock/summary`. Design выбирает и описывает инвалидацию после move/правки остатка.

### Q-005: Размещение D26-проверок фазы 3
409 `CLIENT_HAS_TASKS` / `EQUIPMENT_USED_IN_TASKS` — в сервисах `clients` (зависимость от таблиц `tasks`/`task_equipment` через SQL EXISTS) без импорта пакета доски. Design подтверждает границу пакетов и маппинг ошибки FK → 409 как страховку.

### Q-006: Формат `warnings` в ответах
Единая структура `warnings: [{product_id, name, stock, min_stock, kind: negative|below_min}]` в ответах move, PUT задачи, PUT колонки, DELETE задачи. Design фиксирует DTO и TS-зеркало (§TD-6.2).
