# Input Bundle: phase-5-board / стадия 10-qa — generated 2026-09-27 09:54

> Твой ЕДИНСТВЕННЫЙ документный вход (пайп-контракт). Upstream-доки напрямую НЕ читать;
> точечный grep по конкретному ID (DS-NNN/AC-NNN/TS-NNN) — разрешён. Отсутствие нужного
> факта в бандле = сигнал оркестратору (дырка контракта), не повод читать всё.

## Источник: AC-строки (2-spec-interview/prd.md) [ac]
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

## Источник: 9-validation/output.md [full]
---
stage: 9-validation
feature: phase-5-board
tier: M
---
# Validation: phase-5-board — PASS

## Платформы
- Backend (Go): ✅
- Web (SSR html/template + vanilla JS на Bootstrap, Vue нет — D30): ✅

## Гейты
- `go build ./...`, `go vet ./...` ✅; `go test ./... -race -count=1` — 11 пакетов ok ✅; `golangci-lint run ./...` — 0 issues ✅
- `BASE=be769e5 make test-integration-scoped` ✅ — FULL-триггер (ALTER TABLE stock_movements в 0005) → 14 пакетов ok (board, catalog, clients, seeddemo, server, …)
- Чекеры review_bundle: 9 pass / 0 fail (go-naming, api-conventions, live-suite, …); fuzz-verdict SKIP (tier M, risk reversible)
- E2E полный прогон №2 (скоуп `full`, 0-setup/e2e-scope-prefs.md): `npx playwright test --project=chromium --headed --workers=1 --retries=1 --reporter=list`
- **Счётчики: passed 87 / hard-fail 0 / flaky 0 / skipped 0**
- Артефакт §TD-2: `9-validation/evidence/last-run.json` (status=passed) + `9-validation/evidence/e2e-run.txt`
- Watch-gate §TD-5 (when=validation): `PWSLOWMO=600` board/board-guards/stock — 21/21; пользователь: «Продолжаем к QA».
- полный integration-регресс не гонялся (скоуп: task) — но FULL-триггер тула расширил прогон до `./...`.
- `make seed-demo` на dev-БД: создано (исполнители 3, клиенты 8, оборудование 8, товары 11, услуги 8, заказы 6); повторный вызов — no-op ✅

## Сценарий (AC trace)
- [x] AC-001 доска с колонками и карточками — BD-001, LV-001…
- [x] AC-002 статус = колонка, перенос/порядок, reload — BD-002, LV-060/064 (ребалансировка, 70 вставок)
- [x] AC-003 финальная колонка только без незавершённых услуг — BD-003, LV-050/051; + PUT колонки is_final с открытыми услугами → 409 (TestBoardLiveSuite_Update_ToFinal_ColumnHasOpenSubtasks)
- [x] AC-004 списание/возврат — BD-004, IT-010, TestStockEngine_*
- [x] AC-005 минус и минимум — предупреждения — BD-005
- [x] AC-006 управление колонками (admin) — BD-006, LV-054, TestBoardLiveSuite_Update_LastNonFinal_Rejected
- [x] AC-007 смена «списывать товары» пересчитывает склад — BD-007, LV-062
- [x] AC-008 колонки D32 — BD-008
- [x] AC-009 простая форма заказа — BD-009
- [x] AC-010 конфликт одновременной правки — BD-010, …StaleVersion_PaidChanged_VersionConflictWins
- [x] AC-011 оплата — только admin — BD-011, LV-042
- [x] AC-012 удаление заказа admin, товары на склад — BD-012
- [x] AC-013 фильтры в URL — BD-013
- [x] AC-014 «Мои» по умолчанию у исполнителя — BD-014
- [x] AC-015 экран «Остатки» — BD-015, StockLiveSuite
- [x] AC-016 счётчик в меню — BD-016, SH-001
- [x] AC-017 клиент с заказами не удаляется — BD-017, LV-056
- [x] AC-018 оборудование из заказа не удаляется — BD-018, LV-057
- [x] AC-019 make seed-demo — IT-001 (SeedDemoSuite) + ручная проверка на dev-БД
- [x] AC-020 360px и обе темы — BD-020

## Observability
- Monitoring & Alerting Spec — deferred в дизайне (MVP, прода нет) → проверка спанов не выполнялась.

## Mobile API live check
- validation skipped: no live API (dev-стенд не задеплоен; проверено локально E2E + live-сьютами)

## Источник: 8-code-review/output.md [full]
---
stage: 8-code-review
feature: phase-5-board
tier: M
verdict: APPROVED
high: 0
medium: 0
low: 2
---
# Code Review: phase-5-board — APPROVED
**Review level:** Standard (tier: M) | итерация 1 (fix `34bd3af`) | CM-1 MVP | UI-scope swagger:no / adaptive:min / themes:both | фронт SSR+vanilla JS (§11 Vue/Element Plus — n/a)

Чекеры bundle после перегенерации (9/9 ✅): go_naming, api_conventions, contract, date_bound, fuzz_verdict, live_suite, fixture_sync, e2e_mode, rule_usage. Не перегонялись. Прогон оркестратора (итерация 1): unit 11/11, lint 0, integration 14/14, E2E board/board-guards/stock/auth 31/31.

## Верификация findings итерации 0 (по diff `34bd3af`)
| # | Finding | Статус | Где закрыто |
|---|---|---|---|
| HIGH-1 | PUT `is_final=true` на последней нефинальной колонке | закрыт | `columns_service.go` `becomesFinal` под `LockAll` → `ErrLastNonFinalColumn` до `repo.Update`; тест `TestBoardLiveSuite_Update_LastNonFinal_Rejected` (`board_http_live_test.go:1001`) |
| MEDIUM-1 | Guard ребалансировки `scale(pos) > 20` недостижим | закрыт | `position.go:33-34`: `pos <= prev_pos OR pos >= next_pos OR scale(pos) >= 16`. Порог 16 = потолок scale при делении NUMERIC (проверено на dev-PG в итерации 0). LV-060 поднят до 70 вставок (:1279) — при старом коде на ~64-й вставке давал бы дубль, так что тест различает старое и новое поведение. После renumber позиции целые, повторная ребалансировка не зацикливается |
| MEDIUM-2 | `PAID_ADMIN_ONLY` раньше version-конфликта | закрыт | `tasks_service.go:150-157`: `*in.Version != current.Version` → `ErrTaskVersionConflict` сразу после `LockTask` (FOR UPDATE — проверка авторитетна). Тест `…StaleVersion_PaidChanged_VersionConflictWins` (:1114); BD-011 шлёт актуальную версию |
| MEDIUM-3 | `is_final` на колонке с открытыми услугами обходит AC-003 | закрыт (решение оркестратора) | `columns_repo.go` `OpenSubtasksForColumn` + `ErrSubtasksNotDone.WithDetail("subtasks", …)` при `becomesFinal`, под тем же `LockAll`. Owner обновлён: `design-errors.md` #03. Тест `TestBoardLiveSuite_Update_ToFinal_ColumnHasOpenSubtasks` (:1059) |
| Mobile M-01 | мусорные `limit`/`offset` у `/columns`, `/assignees` | закрыт | `columns_handler.go`, `assignees_handler.go`: 400 VALIDATION (`msgInvalid`), паттерн как в `stock_handler.go`; тест :806 |
| LOW-2 | ложный комментарий об экранировании q | закрыт | `stock_repo.go:43-46` |
| LOW-3 | TOCTOU в seed-demo guard | закрыт | `seeddemo.go`: `pg_advisory_xact_lock` + `count(tasks)` и lookup admin внутри транзакции записи |
| LOW-4 | снапшот отката `lastKnownTasks` затирается | закрыт | `board.js` `performMove`: снапшот снимается перед своей оптимистичной правкой, на успехе сбрасывается |
| LOW-1 | NavBadges — Summary на каждом SSR-рендере | as-is (known issue) | см. Незначительные |

Попутная правка `board.js` (гард «Удалить колонку» теперь смотрит на `col.tasks_count > 0 || state.tasks…`): ошибка возможна только в безопасную сторону — после выноса заказов кнопка может остаться выключенной до reload. Серверный `COLUMN_NOT_EMPTY` остаётся авторитетным.

## Блокеры (обязательно исправить до мержа)
Нет.

## Серьёзные замечания (исправить в этом PR)
Нет.

## Незначительные замечания (можно в следующем PR)
1. **[internal/ssr/shell.go `Build` + internal/features/board/stock_service.go `NavBadges`]** Бейдж выполняет полный `stockPendingCTE` Summary на каждом SSR-рендере. Принято оркестратором как known issue (MVP); при росте каталога нужен кэш/TTL.
2. **[internal/features/board/columns_service.go `becomesFinal` → `OpenSubtasksForColumn`]** Проверка открытых услуг блокирует колонки, но не строки `subtasks`. Будущий (фаза 6) API смены статуса подзадачи должен брать блокировку в том же порядке columns→tasks, иначе сохраняется окно гонки со сменой флага. Сейчас записи в `subtasks` вне seed нет — только отметка для фазы 6.

## Проверено без замечаний (итерация 0, diff фикса не затрагивает)
- **StockEngine.Reconcile:**
  - дельта target−cur применяется относительно (`stock - delta`), повторный вызов — no-op, двойного списания нет;
  - покрыты все пять триггеров (move / PUT со сменой колонки / delete / toggle consumes_stock / seed-demo);
  - Lock Order columns→tasks→products соблюдён, retry 40P01/40001 ×3;
  - движение пишется в журнал при каждом изменении stock (Reconcile, Adjust, seed).
- **Move без version (SD-006, LWW)** + `version+1`.
- **Authz:** admin-маршруты, 400 (ссылка из тела) vs 404 (id в пути), `PAID_ADMIN_ONLY`.
- **Миграция 0005** совпадает с `design-sql.md`.
- **D26 / SERVICE_IN_USE:** классификация по имени constraint, SSR-перерисовка на 409.
- **Общие компоненты:** `errorspkg.WithDetail` (копия, без мутации sentinel), `RequireSSR no-store`, бейдж, combobox — без регресса фаз 1–4.
- **XSS / CSRF:** user-строки в JS идут через `esc()`/`textContent`, `X-Csrf-Token` на мутациях.
- **Тесты:** LiveSuite TD-8 + adversarial и race-сценарии, E2E BD-001…BD-020 без `waitForTimeout`, 360px и обе темы.

## Соответствие коду дизайну (верификация реализации)
- [x] Dataflow соответствует design-sequence-diagrams — ДА
- [x] Все SQL из 3-design/design-sql.md реализованы — ДА (LV-054 теперь и на PUT)
- [x] Go-слои по 3-design/design-go-layers.md — ДА
- [x] Error handling по 3-design/design-errors.md — ДА (#03 расширен в owner, код совпадает)
- [x] Infrastructure (Makefile seed-demo) — ДА
- [ ] Swagger — N/A (swagger:no)
- [ ] Vue-структура — N/A (SSR + vanilla JS)
- [ ] Monitoring Spec / Алерты / Rollout / Runbook / Perf Budget — N/A (deferred, MVP, M-tier). Span'ы `tasks.move` и `columns.recalc` есть

## PRD соответствие
- [x] Каждое ADR имеет Driver — ДА (13/13)
- [x] AC-001…AC-020 покрыты тестами — ДА (LV/UT/IT + BD-001…BD-020)
- [x] Success Metric наблюдаема — ДА (span `tasks.move` movements_count, бейдж risk_count)
- [x] Non-Goals не реализованы — ДА

## Статус фаз
- [x] Все фазы закрыты: `6-plan/code-plan-status.md` — 8/8 `completed` — ДА

## Итог
APPROVED: HIGH-1, MEDIUM-1…3, M-01 и LOW-2…4 закрыты и покрыты тестами. Остались 2 LOW: NavBadges (known issue) и отметка о порядке блокировок для фазы 6.

## Источник: 8-code-review/mobile.md [verdict]
---
stage: 8-code-review
feature: phase-5-board
tier: M
verdict: APPROVED
high: 0
medium: 1
low: 1
---
# Mobile API Review: phase-5-board — APPROVED

## Источник: 8-code-review/frontend-arch.md [verdict]
MISSING

## Источник: 3-design/output.md [sections]
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

### ADR-003: Контракт фильтров доски и загрузка одним запросом
- **Статус:** Принято
- **Driver:** AC-013 — «Доску можно отфильтровать по клиенту, тексту (номер или название), виду работ, оплате, исполнителю и сроку. Фильтры сочетаются между собой, и их можно сбросить (source: D19).»; AC-014 — «Исполнитель при открытии доски видит только свои заказы: те, где он ответственный или исполнитель хотя бы одной услуги. Фильтр снимается, и тогда видна вся доска. Администратор по умолчанию видит всё (source: D19).»
- **Отвечает на:** Q-003 — «Контракт фильтров доски»
- **Контекст:** прототип фильтрует на клиенте (`prototype/index.html:778-796`); у нас данные из API.
- **Решение:** `GET /api/v1/tasks?client_id&q&category_id&paid=all|paid|unpaid&assignee=me|all|<uuid>&due=all|soon|overdue&limit&offset` — серверная фильтрация, AND-комбинация. `q`: ILIKE по названию и клиенту + точный `#N`/`N`. `due` по прототипу: `soon` = срок ≤ завтра (включая просрочку) ИЛИ просроченная услуга; `overdue` = срок < сегодня ИЛИ просроченная услуга; финальные колонки исключены. `assignee` отсутствует → дефолт по роли на сервере (executor=`me`). Доска = `GET /columns` + ОДИН `GET /tasks` (limit по умолчанию 500, max 1000; при `total>limit` — баннер «показаны первые N»). Состояние — в `location.search` (reload сохраняет, «Сбросить» → дефолты роли).
- **Альтернативы:** загрузка по колонкам (N запросов, сложный DnD между загруженными страницами); клиентская фильтрация всего набора (рост объёма, расхождение с API).
- **Риски:** `current_date` — TZ БД; для одной мастерской совпадает с локальной (MVP).

### ADR-102: `details` в errorspkg для не-валидационных ошибок
- **Статус:** Принято
- **Driver:** AC-003 — «Если у заказа есть незавершённые услуги, перенести его в финальную колонку нельзя: карточка возвращается на место, пользователь видит сообщение со списком незавершённых услуг. Заказ, у которого все услуги выполнены, переносится свободно (source: D11, D22).»
- **Отвечает на:** —
- **Контекст:** `errDetails` умеет только `fields`.
- **Решение:** `AppError.Extra` + `WithDetail(key, v)` (возвращает копию; sentinel'ы не мутируются); handler сливает `fields` и `Extra` в `details`. design-errors §errorspkg Extension.
- **Альтернативы:** 200 с флагом ошибки; отдельный тип ответа только для move.
- **Риски:** гонка при мутации общего sentinel — исключена копированием (unit-тест).

## Источник: 3-design/design-errors.md [full]
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

## Источник: 3-design/design-go-layers.md [sections]
## Enum SubtaskStatus
`new`, `in_progress`, `done` — источник: `board/enums.go::SubtaskStatuses` = CHECK `subtasks.status` (0005); sync-тест `enums_test.go` (сверка с `pg_get_constraintdef` в live-сьюте).

## Enum PaidFilter
`all` (дефолт), `paid`, `unpaid` — `board/enums.go::PaidFilters`.

## Enum DueFilter
`all` (дефолт), `soon` (просрочено ИЛИ срок сегодня/завтра ИЛИ есть просроченная услуга), `overdue` (просрочено ИЛИ просроченная услуга); финальные колонки не входят — `board/enums.go::DueFilters` (ADR-003, по `prototype/index.html:589-596,784-785`).

## Enum AssigneeFilter
`me`, `all`, `<uuid пользователя>`; отсутствует → по роли: executor=`me`, admin=`all` (AC-014) — `board/enums.go::AssigneeMe/AssigneeAll`.

## Enum StockScope
`risky` (дефолт), `all` — `board/enums.go::StockScopes`.

## Enum StockState
`out`, `low`, `will_low`, `ok` — вычисляется SQL (design-sql §Stock Pending CTE); `board/enums.go::StockStates`.

## Enum WarningKind
`negative`, `below_min` — `board/enums.go::WarningKinds`.

## Enum StockMovementReason
`task_consume`, `task_return`, `manual` — as-is CHECK `0004_catalog.up.sql:59`; board пишет `task_consume`/`task_return`.

## DTO Column
```go
type ColumnResponse struct {
    ID            string `json:"id"`
    Name          string `json:"name"`
    Color         string `json:"color"`
    IsFinal       bool   `json:"is_final"`
    ConsumesStock bool   `json:"consumes_stock"`
    Position      int    `json:"position"`
    TasksCount    int    `json:"tasks_count"`
}
type ColumnListResponse struct { Items []ColumnResponse `json:"items"`; Total, Limit, Offset int /* json total/limit/offset */ }
// Порядок: position, id. Дефолты: limit=100 (1..1000), offset=0.
type ColumnInput struct {           // POST и PUT /api/v1/columns[/:id]
    Name          string `json:"name"`
    Color         string `json:"color"`
    IsFinal       bool   `json:"is_final"`
    ConsumesStock bool   `json:"consumes_stock"`
    Position      *int   `json:"position"` // POST: nil → в конец; PUT: nil → без перемещения
}
type ColumnMutationResponse struct {
    Column   ColumnResponse `json:"column"`
    Warnings []StockWarning `json:"warnings"` // не nil: [] (TS без null)
}
```

## DTO Task
```go
type TaskCard struct {              // элемент списка доски
    ID            int64   `json:"id"`
    ColumnID      string  `json:"column_id"`
    ClientID      string  `json:"client_id"`
    ClientName    string  `json:"client_name"`
    Title         string  `json:"title"`
    AssigneeID    *string `json:"assignee_id"`
    AssigneeName  *string `json:"assignee_name"`
    DueDate       *string `json:"due_date"`       // YYYY-MM-DD
    IsPaid        bool    `json:"is_paid"`
    Position      string  `json:"position"`       // NUMERIC строкой — клиенту только для отладки/сортировки
    Version       int64   `json:"version"`
    SubtasksTotal int     `json:"subtasks_total"`
    SubtasksDone  int     `json:"subtasks_done"`
    SubtasksLate  int     `json:"subtasks_late"`
}
type TaskListResponse struct { Items []TaskCard `json:"items"`; Total, Limit, Offset int }
// Гарантия порядка items (контракт): columns.position, columns.id, tasks.position, tasks.id —
// т.е. по колонкам слева направо, внутри колонки сверху вниз; стабилен между запросами.
// Дефолты: limit=500 (1..1000), offset=0.
type TaskResponse struct {          // GET /tasks/:id и в mutation-ответах
    TaskCard
    PaidAt    *time.Time `json:"paid_at"`
    CreatedAt time.Time  `json:"created_at"`
    UpdatedAt time.Time  `json:"updated_at"`
}
type TaskCreateRequest struct {
    Title      string  `json:"title"`
    ClientID   string  `json:"client_id"`
    ColumnID   string  `json:"column_id"`
    AssigneeID *string `json:"assignee_id"`
    DueDate    *string `json:"due_date"`
    IsPaid     *bool   `json:"is_paid"`   // executor: nil|false, иначе 403
}
type TaskUpdateRequest struct {
    TaskCreateRequest
    Version *int64 `json:"version"`       // обязателен (§FE-1.1)
}
type TaskMoveRequest struct {
    ColumnID string `json:"column_id"`
    PrevID   *int64 `json:"prev_id"`       // карточка СВЕРХУ после вставки (ADR-001)
    NextID   *int64 `json:"next_id"`       // карточка СНИЗУ
}
type TaskMutationResponse struct {
    Task     TaskResponse   `json:"task"`
    Warnings []StockWarning `json:"warnings"`
}
type TaskDeleteResponse struct {
    Warnings []StockWarning `json:"warnings"`
}
```
- `description_html` в фазе 5 не входит в DTO (D25, 5 полей формы) — PUT его не перезаписывает.

## DTO TaskFilter
```go
type TaskFilter struct {            // tasks_filter.go: ParseTaskFilter(c fiber.Ctx, a Actor) (TaskFilter, error)
    ClientID   *string  // client_id (uuid; невалидный → 400 field)
    Q          string   // q, trim, ≤100
    TaskNo     *int64   // из q ~ ^#?\d{1,18}$
    CategoryID *string  // category_id (uuid)
    Paid       *bool    // paid: all→nil
    AssigneeID *string  // assignee: me→a.ID, all→nil, uuid
    Due        string   // "" | soon | overdue
    Limit      int      // default 500, 1..1000
    Offset     int      // default 0
}
```

## DTO StockWarning
Q-006 (ADR-006):
```go
type StockWarning struct {
    ProductID string         `json:"product_id"`
    Name      string         `json:"name"`
    Stock     numeric.Amount `json:"stock"`
    MinStock  numeric.Amount `json:"min_stock"`
    Kind      string         `json:"kind"` // negative | below_min
}
```

## DTO Stock
```go
type StockRow struct {
    ProductID string         `json:"product_id"`
    Name      string         `json:"name"`
    Unit      string         `json:"unit"`
    Box       string         `json:"box"`
    Place     string         `json:"place"`
    Stock     numeric.Amount `json:"stock"`
    MinStock  numeric.Amount `json:"min_stock"`
    Pending   numeric.Amount `json:"pending"`    // потребность не взятых в работу заказов
    Forecast  numeric.Amount `json:"forecast"`
    State     string         `json:"state"`      // out|low|will_low|ok
}
type StockListResponse struct { Items []StockRow `json:"items"`; Total, Limit, Offset int }
type StockSummaryResponse struct {
    ProductsTotal int `json:"products_total"`
    LowCount      int `json:"low_count"`
    OutCount      int `json:"out_count"`
    WillLowCount  int `json:"will_low_count"`
    RiskCount     int `json:"risk_count"`
}
type StockAdjustRequest struct { Stock *numeric.Amount `json:"stock"` }   // обязателен
// ответ adjust — StockRow
```

## DTO Assignee
```go
type AssigneeResponse struct { ID string `json:"id"`; Name string `json:"name"`; Role string `json:"role"` }
type AssigneeListResponse struct { Items []AssigneeResponse `json:"items"`; Total, Limit, Offset int }
// Порядок: name, id. Дефолты: limit=200 (1..1000), offset=0; только is_active.
```

## Источник: 3-design/design-tests.md [full]
# Design Tests: phase-5-board

> Owner: тест-матрица. Уровни: unit (без БД) · integration `BaseSuite` (dockertest, тег `integration`) · live `testutil.LiveSuite` (реальный бинарь + TCP + реальный login, §TD-8; auth-стабы и `app.Test` запрещены, кроме fuzz §9.1) · E2E Playwright (`e2e/tests/board.spec.ts`, `e2e/tests/stock.spec.ts`, `e2e/tests/board-guards.spec.ts`; POM `e2e/pages/BoardPage.ts`, `e2e/pages/StockPage.ts`; ID `BD-NNN`). Каждая строка → AC.

## Test Files
| Файл | Уровень |
|---|---|
| `internal/features/board/columns_validate_test.go`, `tasks_validate_test.go`, `tasks_filter_test.go`, `enums_test.go` | unit |
| `internal/features/board/board_fuzz_test.go` | fuzz (§9.1) |
| `internal/features/board/stock_engine_integration_test.go` | integration (BaseSuite) |
| `internal/features/board/board_http_live_test.go` (`BoardLiveSuite`), `stock_http_live_test.go` (`StockLiveSuite`) | live |
| `internal/features/clients/clients_http_live_test.go` (+кейсы D26), `internal/features/catalog/services_http_live_test.go` (+SERVICE_IN_USE) | live (дополнение) |
| `internal/seeddemo/seeddemo_integration_test.go` | integration |
| `pkg/errorspkg/errors_test.go` (+`WithDetail` не мутирует sentinel, details merge) | unit |
| `internal/ssr/shell_test.go` (+badge) | unit |
| `e2e/fixtures/board.ts` (API-хелперы + `pg`-хелпер услуг, ADR-103), `e2e/fixtures/board-types.ts` (§TD-6.2) | E2E fixtures |

## Happy Path
| ID | Сценарий | Уровень | Driver |
|---|---|---|---|
| LV-001 | GET /columns после миграции — 5 колонок D32 с флагами и порядком | live | AC-008 |
| LV-002 | POST /tasks → 201, id bigint растёт, позиция в конец; GET /tasks/:id | live | AC-009 |
| LV-003 | PUT /tasks/:id с version → 200, version+1; поля обновлены | live | AC-009 |
| LV-004 | move между колонками → column_id/position сохранены; GET /tasks порядок по position | live | AC-002 |
| LV-005 | move внутри колонки prev/next → между соседями; пустая колонка; верх (только next); низ (только prev) | live | AC-002 |
| LV-006 | move в «В работе» заказа с товарами → stock−, task_stock=demand, журнал `task_consume`; назад в «Согласование» → возврат `task_return`; «В работе»→«Ждёт запчасти» — 0 новых движений | live | AC-004 |
| LV-007 | списание при нехватке → 200, stock<0, warnings kind=negative; stock≤min → below_min | live | AC-005 |
| LV-008 | PUT колонки consumes_stock true/false при 2 заказах → пересчёт всех, warnings объединены | live | AC-007 |
| LV-009 | CRUD колонок: create (в конец), rename/color/position (перенумерация 1..N), delete пустой | live | AC-006 |
| LV-010 | DELETE /tasks/:id admin → товары вернулись, stock_movements.task_id = NULL, subtasks удалены | live | AC-012 |
| LV-011 | admin меняет is_paid → paid_at выставлен/снят | live | AC-011 |
| LV-012 | GET /stock?scope=risky|all, KPI summary = множества ADR-002 (out/low/will_low); pending только по не-consumes/не-final колонкам | live | AC-015 |
| LV-013 | PUT /stock/:product_id admin → stock, журнал `manual` дельтой | live | AC-015 |
| LV-014 | SSR `/` и `/stock` 200 для executor и admin; пункт «Остатки» с бейджем = risk_count | live | AC-001, AC-016 |
| LV-015 | GET /assignees — executor 200, только активные | live | AC-009 |
| IT-001 | seeddemo.Run на пустой БД: заказы во всех колонках, услуги, товары; повтор → no-op «заказы уже есть»; без админа → ошибка | integration | AC-019 |

## Filters
| ID | Сценарий | Уровень | Driver |
|---|---|---|---|
| LV-020 | client_id, q (название/клиент/`#N`/`N`), category_id (по услугам), paid, due=soon/overdue (финальные исключены, просроченная услуга учитывается) — каждый отдельно и комбинация | live | AC-013 |
| LV-021 | executor без `assignee` → только свои (ответственный ИЛИ исполнитель услуги); `assignee=all` → все; admin без параметра → все | live | AC-014 |
| LV-022 | limit/offset + total; limit>1000 → 400; дефолты limit (tasks 500, columns 100, assignees 200); порядок items по контракту (колонка.position → task.position → id) | live | AC-013 |
| UT-001 | ParseTaskFilter: enum'ы, uuid, `#12`/`12`/`12a`, пустые → дефолты | unit | AC-013 |

## Invalid Input
TD-4 adversarial (live):
| ID | Сценарий | Ожидание | Driver |
|---|---|---|---|
| LV-030 | все эндпоинты без сессии | 401 | — |
| LV-031 | write без CSRF-заголовка | 403 CSRF каркаса | — |
| LV-032 | мусорное тело (не JSON, лишнее поле, `prev_id:"x"`, `stock:"5"`, массив) | 400, не 500 | — |
| LV-033 | POST/PUT /tasks: пустой title, title 201 символ, нет client_id, несуществующие client_id/column_id/assignee_id, due_date `2026-13-40` | 400 `details.fields[]` | AC-009 |
| LV-034 | PUT колонки: пустое name, color `red`, position 0 | 400 fields | AC-006 |
| LV-035 | `/tasks/abc`, `/tasks/-1`, `/tasks/99999999999999999999`, `/columns/not-uuid`, `/stock/not-uuid` | 404 | — |
| LV-036 | move: prev_id из другой колонки / несуществующий / = самому заказу | 200, позиция по правилу «сосед игнорируется» | AC-002 |
| LV-037 | query: `paid=maybe`, `due=x`, `assignee=bad`, `limit=-1`, `scope=zzz` | 400 field | AC-013 |
| UT-002 | columns_validate / tasks_validate — таблица граничных значений | unit | AC-006, AC-009 |

## Access Control
| ID | Сценарий | Ожидание | Driver |
|---|---|---|---|
| LV-040 | executor: POST/PUT/DELETE /columns | 403 `FORBIDDEN` | AC-006 |
| LV-041 | executor: DELETE /tasks/:id | 403, заказ жив | AC-012 |
| LV-042 | executor: PUT /tasks is_paid ≠ текущему → 403 `PAID_ADMIN_ONLY`; = текущему → 200; POST с is_paid=true → 403 `PAID_ADMIN_ONLY` | AC-011 |
| LV-043 | executor: PUT /stock/:id → 403; GET /stock, /stock/summary → 200 | AC-015 |
| LV-044 | executor: POST/PUT/move заказа → 200 (D19) | AC-009, AC-002 |

## Business Rules
| ID | Сценарий | Ожидание | Driver |
|---|---|---|---|
| LV-050 | move в «Готово» при subtask status<>done | 409 SUBTASKS_NOT_DONE, details.subtasks[{id,service_name,status}], колонка/склад без изменений | AC-003 |
| LV-051 | та же проверка через PUT со сменой column_id | 409 | AC-003 |
| LV-052 | все услуги done → move в «Готово» 200 | AC-003 |
| LV-053 | DELETE колонки с заказом | 409 COLUMN_NOT_EMPTY | AC-006 |
| LV-054 | удалить/сделать финальной последнюю нефинальную | 409 LAST_NON_FINAL_COLUMN | AC-006 |
| LV-055 | PUT с устаревшей version | 409 TASK_VERSION_CONFLICT, данные не затёрты | AC-010 |
| LV-056 | DELETE клиента с заказом (API + SSR POST /clients/:id/delete) | 409 CLIENT_HAS_TASKS / flash | AC-017 |
| LV-057 | DELETE оборудования из task_equipment и из subtasks.equipment_id | 409 EQUIPMENT_USED_IN_TASKS | AC-018 |
| LV-058 | DELETE услуги из subtasks | 409 SERVICE_IN_USE (не 500) | ADR-005 |
| LV-059 | DELETE товара из subtask_products/task_stock | 409 PRODUCT_IN_USE (не 500) | ADR-005 |

## Race Condition
| ID | Сценарий | Ожидание | Driver |
|---|---|---|---|
| LV-060 | два клиента (2 `Browser`) параллельно move одной карточки в разные колонки (N=20 горутин, `-race`) | все 200 (LWW), итог = одна из целей, `task_stock` = target итоговой колонки, инвариант склада | AC-002, AC-004 |
| LV-061 | форма (PUT со старой version) vs move | move 200, PUT 409 TASK_VERSION_CONFLICT | AC-010 |
| LV-062 | параллельно: toggle consumes_stock колонки ‖ move заказа в/из неё | нет deadlock-500 (retry), инвариант склада, `task_stock` согласован с итоговой колонкой | AC-007 |
| LV-063 | параллельные move двух заказов с общим товаром | stock = начальный − Σ списаний, без потерянных обновлений | AC-004 |
| LV-064 | 50 вставок между одними соседями → ребалансировка, порядок сохранён, позиции уникальны | AC-002 |
| LV-065 | DELETE клиента ‖ POST заказа на этого клиента | либо 409 CLIENT_HAS_TASKS, либо 400 field client_id — не 500 | AC-017 |

## Stock Invariant
PRD §3 «Расхождения остаток ↔ сумма движений = 0».
- IT-010 (`stock_engine_integration_test.go`): таблично — последовательность (consume, return, повтор consume, toggle колонки, delete заказа, manual adjust) → после КАЖДОГО шага `products.stock = начальный + Σ stock_movements.qty_delta` для всех товаров и `task_stock(task) = demand × consumes(column)`.
- Хелпер `assertStockInvariant(t, pool)` вызывается в конце LV-006…LV-010, LV-013, LV-060…LV-063 и в IT-001 (seed-demo).
- «Начальный» = stock в момент создания товара (фиксируется тестом; журнал до создания пуст — D44).

## Fuzz Targets
Класс 9 (qa.md) — триггер: парсинг query/JSON от клиента.
| Target | Функция | Инварианты | Seeds |
|---|---|---|---|
| `FuzzParseTaskFilter` | `ParseTaskFilter` (url.Values-вход) | no panic; ошибка — только `*errorspkg.AppError` 400 | `#`, `#0`, `#999999999999999999999`, `%`, `_\`, `assignee=me&assignee=all`, UTF-8 мусор |
| `FuzzTaskInputValidate` | `TaskCreateRequest.normalize().validate()` | no panic; fields ⊂ {title, client_id, column_id, assignee_id, due_date} | 200/201 символ, `\u0000`, `2026-02-30` |
| `FuzzColumnInputValidate` | `ColumnInput.validate()` | no panic | `#GGGGGG`, `#fff`, position MinInt |
| `FuzzMoveBody` | `app.Test` POST move (исключение §9.1) | статус ∈ {200,400,404,409}, никогда 5xx | `{}`, `{"prev_id":1e99}`, `[]`, `null` |

## E2E Scenarios
Стенд `make local-up` + `make migrate-up`; изоляция — уникальный клиент/товар/колонки `uniq()` на тест, доска открывается с `?client_id=<uniq>`; §TD-4 чистота консоли/сети (ожидаемые 409 — `allowHttpErrors`). `--project=chromium --headed --workers=1`.
| ID | Сценарий | Driver |
|---|---|---|
| BD-001 | доска: колонки в порядке, карточка с `#N`, названием, клиентом, ответственным, сроком, оплатой; цвет полосы = цвет колонки | AC-001 |
| BD-002 | HTML5 DnD (`locator.dragTo`) «Новые»→«Согласование», перестановка внутри колонки; reload — положение сохранено | AC-002 |
| BD-003 | заказ с открытой услугой (pg-фикстура) → DnD в «Готово»: карточка вернулась, ui-dialog со списком услуг; услуга done → перенос проходит | AC-003 |
| BD-004 | товар stock=5; заказ с товаром 2 → «В работе»: /stock показывает 3; обратно — 5; «В работе»→«Ждёт запчасти» — 3 | AC-004 |
| BD-005 | товар stock=1, потребность 3 → перенос проходит, toast warnings со ссылкой «Остатки» | AC-005 |
| BD-006 | admin: создать колонку, переименовать, цвет, позиция, удалить; удаление непустой — сообщение; executor не видит «+Колонка»/правки | AC-006 |
| BD-007 | admin включает «списывать» у своей колонки с заказом → остаток уменьшился; выключает → вернулся | AC-007 |
| BD-008 | дефолтные колонки D32 присутствуют (по имени/флагам через UI-модалку) | AC-008 |
| BD-009 | создание заказа формой (combobox клиента, колонка, ответственный, срок) → карточка с новым `#N`; правка той же формой; двойной клик «Сохранить» — один запрос | AC-009 |
| BD-010 | два контекста открыли форму; первый сохранил, второй видит «Заказ уже изменён»; форма vs move — то же | AC-010 |
| BD-011 | admin отмечает оплату; executor: переключатель оплаты disabled, прямой PUT → 403 `PAID_ADMIN_ONLY` | AC-011 |
| BD-012 | admin удаляет заказ с подтверждением ui-dialog — товары вернулись; у executor кнопки нет | AC-012 |
| BD-013 | фильтры клиент+текст+вид работ+оплата+срок сочетаются, отражены в URL, reload сохраняет, «Сбросить» | AC-013 |
| BD-014 | executor при входе видит только свои (ответственный / исполнитель услуги), «Все» — видна чужая карточка | AC-014 |
| BD-015 | «Остатки»: KPI, таблица рисковых с прогнозом; admin правит остаток; executor — без кнопки правки | AC-015 |
| BD-016 | бейдж «Остатки» виден executor и admin; после move с уходом в минимум — +1 без перезагрузки | AC-016 |
| BD-017 | удаление клиента с заказом в /clients — сообщение, клиент на месте | AC-017 |
| BD-018 | удаление оборудования из task_equipment (pg-фикстура) — сообщение | AC-018 |
| BD-020 | 360×800, обе темы: `/`, форма заказа, `/stock` — `hasHorizontalOverflow()` = false; доска скроллится внутри `.board-columns`; перенос через меню «Переместить в…» | AC-020 |
- AC-019 (seed-demo) — IT-001 (не E2E: команда + БД; E2E-стенд общий). Проверка оркестратором — ручной smoke Runbook.
- LAST_NON_FINAL_COLUMN — только LV-054 (на общем E2E-стенде нельзя финализировать все колонки).

## Источник: 3-design/design-sql.md [full]
# Design SQL: phase-5-board

> Owner: DDL, миграция `0005_board`, ключевые запросы. Пакет-потребитель — `internal/features/board` (SQL inline-константами в `*_repo.go`, эталон `internal/features/catalog/*_repo.go`; go:embed не используется в проекте). CM-1 = MVP: down = простой DROP.

## Baseline (as-is, затрагиваемое)
- `stock_movements` (`migrations/0004_catalog.up.sql:54-63`): `task_id uuid NULL` без FK; `reason CHECK IN (task_consume, task_return, manual)`; все строки — `manual`, `task_id IS NULL`.
- `clients` / `equipment` / `services` / `products` / `users` — `id uuid DEFAULT gen_random_uuid()`; без изменений схемы.

## Columns Table
```sql
CREATE TABLE columns (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name           text NOT NULL CHECK (btrim(name) <> '' AND char_length(name) <= 60),
    color          text NOT NULL CHECK (color ~ '^#[0-9A-Fa-f]{6}$'),
    is_final       boolean NOT NULL DEFAULT false,
    consumes_stock boolean NOT NULL DEFAULT false,
    position       int NOT NULL CHECK (position >= 1),
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX columns_position_idx ON columns (position, id);

-- Сид D32 (AC-008) — в самой миграции.
INSERT INTO columns (name, color, is_final, consumes_stock, position) VALUES
    ('Новые',          '#94A3B8', false, false, 1),
    ('Согласование',   '#7C3AED', false, false, 2),
    ('В работе',       '#2563EB', false, true,  3),
    ('Ждёт запчасти',  '#D97706', false, true,  4),
    ('Готово',         '#059669', true,  true,  5);
```
- Инвариант «≥1 нефинальная колонка» — сервисом под `FOR UPDATE` всей таблицы (ADR-104), не CHECK.
- `position` не UNIQUE: перенумерация 1..N в транзакции при create/update/delete (ADR-100 §колонки).

## Tasks Table
```sql
CREATE TABLE tasks (
    id               bigserial PRIMARY KEY,                       -- D37: сквозной #N, без переиспользования
    column_id        uuid NOT NULL REFERENCES columns(id) ON DELETE RESTRICT,
    client_id        uuid NOT NULL REFERENCES clients(id) ON DELETE RESTRICT,   -- опора D26 (AC-017)
    title            text NOT NULL CHECK (btrim(title) <> '' AND char_length(title) <= 200),
    description_html text NOT NULL DEFAULT '',                    -- фаза 6; в фазе 5 пишет только seed-demo
    assignee_id      uuid NULL REFERENCES users(id) ON DELETE SET NULL,
    due_date         date NULL,
    is_paid          boolean NOT NULL DEFAULT false,
    paid_at          timestamptz NULL,
    position         numeric NOT NULL,                            -- дробная позиция (ADR-001)
    created_by       uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    version          bigint NOT NULL DEFAULT 1,
    CONSTRAINT tasks_paid_at_chk CHECK (is_paid = (paid_at IS NOT NULL))
);
CREATE INDEX tasks_column_position_idx ON tasks (column_id, position, id);
CREATE INDEX tasks_client_id_idx       ON tasks (client_id);
CREATE INDEX tasks_assignee_id_idx     ON tasks (assignee_id);
```
- Имя FK `tasks_client_id_fkey` (дефолт PG) — ключ маппинга 23503 → `CLIENT_HAS_TASKS` (ADR-005).

## Subtasks Table
```sql
CREATE TABLE subtasks (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id      bigint NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    service_id   uuid NOT NULL REFERENCES services(id) ON DELETE RESTRICT,
    status       text NOT NULL DEFAULT 'new' CHECK (status IN ('new', 'in_progress', 'done')),
    assignee_id  uuid NULL REFERENCES users(id) ON DELETE SET NULL,
    due_date     date NULL,
    price        numeric(12,2) NULL CHECK (price IS NULL OR price >= 0),
    equipment_id uuid NULL REFERENCES equipment(id) ON DELETE RESTRICT,  -- опора D26 (AC-018)
    position     int NOT NULL DEFAULT 0,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX subtasks_task_id_idx      ON subtasks (task_id);
CREATE INDEX subtasks_service_id_idx   ON subtasks (service_id);
CREATE INDEX subtasks_equipment_id_idx ON subtasks (equipment_id);
CREATE INDEX subtasks_assignee_id_idx  ON subtasks (assignee_id);
```

## Subtask Products Table
```sql
CREATE TABLE subtask_products (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    subtask_id uuid NOT NULL REFERENCES subtasks(id) ON DELETE CASCADE,
    product_id uuid NOT NULL REFERENCES products(id) ON DELETE RESTRICT,
    qty        numeric(12,3) NOT NULL CHECK (qty > 0),
    price      numeric(12,2) NULL CHECK (price IS NULL OR price >= 0)
);
CREATE INDEX subtask_products_subtask_id_idx ON subtask_products (subtask_id);
CREATE INDEX subtask_products_product_id_idx ON subtask_products (product_id);
```

## Task Stock Table
```sql
CREATE TABLE task_stock (
    task_id    bigint NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    product_id uuid NOT NULL REFERENCES products(id) ON DELETE RESTRICT,
    qty        numeric(12,3) NOT NULL CHECK (qty > 0),   -- 0 → строка удаляется
    PRIMARY KEY (task_id, product_id)
);
CREATE INDEX task_stock_product_id_idx ON task_stock (product_id);
```

## Task Equipment Table
```sql
CREATE TABLE task_equipment (
    task_id      bigint NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    equipment_id uuid NOT NULL REFERENCES equipment(id) ON DELETE RESTRICT,
    completeness text NOT NULL DEFAULT '',
    condition    text NOT NULL DEFAULT '',
    PRIMARY KEY (task_id, equipment_id)
);
CREATE INDEX task_equipment_equipment_id_idx ON task_equipment (equipment_id);
```

## Stock Movements Migration
```sql
-- task_id: uuid → bigint. Данных с task_id нет (все строки manual, D44) — USING NULL.
ALTER TABLE stock_movements ALTER COLUMN task_id TYPE bigint USING NULL;
ALTER TABLE stock_movements
    ADD CONSTRAINT stock_movements_task_id_fkey
    FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE SET NULL;   -- SD-007
CREATE INDEX stock_movements_task_id_idx ON stock_movements (task_id);
```

## Board Migration
Файлы: `migrations/0005_board.up.sql`, `migrations/0005_board.down.sql`. Порядок up: Columns (+сид) → Tasks → Subtasks → Subtask Products → Task Stock → Task Equipment → Stock Movements Migration.
```sql
-- 0005_board.down.sql (CM-1 = MVP)
DROP INDEX IF EXISTS stock_movements_task_id_idx;
ALTER TABLE stock_movements DROP CONSTRAINT IF EXISTS stock_movements_task_id_fkey;
ALTER TABLE stock_movements ALTER COLUMN task_id TYPE uuid USING NULL;
DROP TABLE IF EXISTS task_equipment;
DROP TABLE IF EXISTS task_stock;
DROP TABLE IF EXISTS subtask_products;
DROP TABLE IF EXISTS subtasks;
DROP TABLE IF EXISTS tasks;
DROP TABLE IF EXISTS columns;
```

## Lock Order
Глобальный порядок блокировок (ADR-104), все операции склада — одна транзакция `READ COMMITTED`:
1. `columns` — целевая колонка `FOR SHARE` (move/PUT/create); вся таблица `ORDER BY id FOR UPDATE` (CRUD колонок, смена флагов).
2. `tasks` — `FOR UPDATE`, несколько — `ORDER BY id`.
3. `products` — `FOR UPDATE ORDER BY id`.
SQLSTATE `40P01`/`40001` → повтор транзакции до 3 раз (`withTx`), затем 500.

## Lock Target Column Query
```sql
SELECT id, is_final, consumes_stock FROM columns WHERE id = $1::uuid FOR SHARE;   -- 0 строк → 400 field column_id
```

## Lock Task Query
```sql
SELECT id, column_id, client_id, title, assignee_id, due_date, is_paid, paid_at, position, version
FROM tasks WHERE id = $1 FOR UPDATE;    -- 0 строк → TASK_NOT_FOUND
```

## Open Subtasks Query
```sql
-- D11/D22: проверка перед переносом в is_final (move и PUT со сменой column_id).
SELECT s.id, sv.name AS service_name, s.status
FROM subtasks s JOIN services sv ON sv.id = s.service_id
WHERE s.task_id = $1 AND s.status <> 'done'
ORDER BY s.position, s.id;              -- ≥1 строка → 409 SUBTASKS_NOT_DONE + details.subtasks[]
```

## Position Query
ADR-001. Соседи читаются только из целевой колонки и не равные самому заказу; «чужой»/несуществующий сосед игнорируется.
```sql
-- $1 target column, $2 task id, $3 prev_id (NULL), $4 next_id (NULL)
WITH prev AS (SELECT position FROM tasks WHERE id = $3 AND column_id = $1 AND id <> $2),
     next AS (SELECT position FROM tasks WHERE id = $4 AND column_id = $1 AND id <> $2),
     last AS (SELECT max(position) AS p FROM tasks WHERE column_id = $1 AND id <> $2)
SELECT CASE
    WHEN (SELECT position FROM prev) IS NOT NULL AND (SELECT position FROM next) IS NOT NULL
         THEN trim_scale(((SELECT position FROM prev) + (SELECT position FROM next)) / 2)
    WHEN (SELECT position FROM prev) IS NOT NULL THEN (SELECT position FROM prev) + 1024
    WHEN (SELECT position FROM next) IS NOT NULL THEN (SELECT position FROM next) - 1024
    ELSE COALESCE((SELECT p FROM last), 0) + 1024
END AS pos,
(SELECT position FROM prev) AS prev_pos, (SELECT position FROM next) AS next_pos;
```
- Ребалансировка, если `prev_pos >= next_pos` ИЛИ `scale(pos) > 20`: перенумеровать колонку и пересчитать pos повторно (одна итерация):
```sql
UPDATE tasks t SET position = r.rn * 1024
FROM (SELECT id, row_number() OVER (ORDER BY position, id) AS rn
      FROM tasks WHERE column_id = $1 AND id <> $2) r
WHERE t.id = r.id;                      -- version НЕ инкрементится (порядок ≠ правка заказа)
```

## Move Task Query
```sql
UPDATE tasks SET column_id = $2::uuid, position = $3::numeric,
       version = version + 1, updated_at = now()
WHERE id = $1
RETURNING id, column_id, client_id, title, assignee_id, due_date, is_paid, paid_at, position, version, created_at, updated_at;
```

## Update Task Query
```sql
-- column_id сменилась → position = append (Position Query с prev/next = NULL); иначе прежняя.
UPDATE tasks SET title = $3, client_id = $4::uuid, column_id = $5::uuid, assignee_id = $6::uuid,
       due_date = $7::date, is_paid = $8,
       paid_at = CASE WHEN $8 AND NOT is_paid THEN now() WHEN NOT $8 THEN NULL ELSE paid_at END,
       position = $9::numeric, version = version + 1, updated_at = now()
WHERE id = $1 AND version = $2
RETURNING ...;                          -- 0 строк при существующем id → TASK_VERSION_CONFLICT
```

## Insert Task Query
```sql
INSERT INTO tasks (column_id, client_id, title, assignee_id, due_date, is_paid, paid_at, position, created_by)
VALUES ($1::uuid, $2::uuid, $3, $4::uuid, $5::date, $6, CASE WHEN $6 THEN now() END, $7::numeric, $8::uuid)
RETURNING id, ..., version;
```

## Reconcile Query
Функция `StockEngine.Reconcile(ctx, tx, taskID int64, targetConsumes bool, userID string)` (ADR-104). Вызывается после блокировки заказа.
```sql
-- 1. Дельты (потребность vs фактически списанное), $2 = targetConsumes
WITH demand AS (
    SELECT sp.product_id, sum(sp.qty) AS qty
    FROM subtask_products sp JOIN subtasks s ON s.id = sp.subtask_id
    WHERE s.task_id = $1 GROUP BY sp.product_id),
target AS (SELECT product_id, qty FROM demand WHERE $2),
cur AS (SELECT product_id, qty FROM task_stock WHERE task_id = $1)
SELECT coalesce(t.product_id, c.product_id) AS product_id,
       coalesce(t.qty, 0) - coalesce(c.qty, 0) AS delta
FROM target t FULL JOIN cur c ON c.product_id = t.product_id
WHERE coalesce(t.qty, 0) <> coalesce(c.qty, 0)
ORDER BY 1;
-- 2. Блокировка товаров (порядок id, D34)
SELECT id FROM products WHERE id = ANY($1::uuid[]) ORDER BY id FOR UPDATE;
-- 3. На каждую дельту d (product p), target = целевое количество (0 при targetConsumes=false):
UPDATE products SET stock = stock - $2::numeric, updated_at = now() WHERE id = $1::uuid;   -- $2 = d
-- target > 0:
INSERT INTO task_stock (task_id, product_id, qty) VALUES ($1, $2::uuid, $3::numeric)        -- $3 = target
    ON CONFLICT (task_id, product_id) DO UPDATE SET qty = EXCLUDED.qty;
-- target = 0:
DELETE FROM task_stock WHERE task_id = $1 AND product_id = $2::uuid;
-- журнал ($3 = d):
INSERT INTO stock_movements (product_id, task_id, qty_delta, reason, user_id)
VALUES ($1::uuid, $2, -$3::numeric,
        CASE WHEN $3 > 0 THEN 'task_consume' ELSE 'task_return' END, $4::uuid);
-- 4. Warnings (Q-006) — по затронутым товарам, значения после UPDATE
SELECT id, name, stock::text, min_stock::text,
       CASE WHEN stock < 0 THEN 'negative' ELSE 'below_min' END AS kind
FROM products WHERE id = ANY($1::uuid[]) AND stock <= min_stock ORDER BY name, id;
```
- Инвариант после Reconcile: `task_stock(task) = target` (строк с qty=0 нет); Σ `stock_movements.qty_delta` по заказу = −Σ `task_stock.qty`.
- `stock` NUMERIC(12,3): переполнение 22003 → 500 + `stock_reconcile_errors_total` (MVP; объёмы мастерской малы).

## Board List Query
Q-003 (ADR-003). Один запрос карточек + `count(*) OVER ()` для `total`.
```sql
SELECT t.id, t.column_id, t.client_id, cl.company_name, t.title, t.assignee_id, u.name,
       t.due_date, t.is_paid, t.position, t.version,
       (SELECT count(*) FROM subtasks s WHERE s.task_id = t.id)                       AS subtasks_total,
       (SELECT count(*) FROM subtasks s WHERE s.task_id = t.id AND s.status = 'done') AS subtasks_done,
       (SELECT count(*) FROM subtasks s WHERE s.task_id = t.id AND s.status <> 'done'
                                           AND s.due_date < current_date)             AS subtasks_late,
       count(*) OVER () AS total
FROM tasks t
JOIN columns c  ON c.id = t.column_id
JOIN clients cl ON cl.id = t.client_id
LEFT JOIN users u ON u.id = t.assignee_id
WHERE ($1::uuid IS NULL OR t.client_id = $1)                                               -- client_id
  AND ($2::text IS NULL OR t.title ILIKE '%' || $2 || '%' ESCAPE '\' OR cl.company_name ILIKE '%' || $2 || '%' ESCAPE '\'
       OR t.id = $3::bigint)                                                                -- q (+#N)
  AND ($4::uuid IS NULL OR EXISTS (SELECT 1 FROM subtasks s JOIN services sv ON sv.id = s.service_id
                                   WHERE s.task_id = t.id AND sv.category_id = $4))        -- category_id
  AND ($5::boolean IS NULL OR t.is_paid = $5)                                              -- paid
  AND ($6::uuid IS NULL OR t.assignee_id = $6
       OR EXISTS (SELECT 1 FROM subtasks s WHERE s.task_id = t.id AND s.assignee_id = $6)) -- assignee (AC-014)
  AND ($7::text IS NULL OR (NOT c.is_final AND (
         ($7 = 'overdue' AND t.due_date < current_date)
      OR ($7 = 'soon'    AND t.due_date <= current_date + 1)
      OR EXISTS (SELECT 1 FROM subtasks s WHERE s.task_id = t.id AND s.status <> 'done'
                 AND s.due_date < current_date))))                                          -- due
ORDER BY c.position, c.id, t.position, t.id
LIMIT $8 OFFSET $9;
```
- `q` экранируется (`\`, `%`, `_`) эталоном `pkg/sqlutil`; `$3` = число, если `q` ~ `^#?\d{1,18}$`, иначе NULL.

## Stock Pending CTE
Q-002 (ADR-002). Общий CTE для списка, KPI и бейджа.
```sql
WITH pending AS (
    SELECT sp.product_id, sum(sp.qty) AS qty
    FROM subtask_products sp
    JOIN subtasks s ON s.id = sp.subtask_id
    JOIN tasks t    ON t.id = s.task_id
    JOIN columns c  ON c.id = t.column_id
    WHERE NOT c.consumes_stock AND NOT c.is_final
    GROUP BY sp.product_id),
rows AS (
    SELECT p.id, p.name, p.unit, p.box, p.place, p.stock, p.min_stock,
           coalesce(pd.qty, 0) AS pending, p.stock - coalesce(pd.qty, 0) AS forecast,
           CASE WHEN p.stock <= 0 THEN 'out'
                WHEN p.stock <= p.min_stock THEN 'low'
                WHEN p.stock - coalesce(pd.qty, 0) <= p.min_stock THEN 'will_low'
                ELSE 'ok' END AS state
    FROM products p LEFT JOIN pending pd ON pd.product_id = p.id
    WHERE p.is_active)
```

## Stock List Query
```sql
<Stock Pending CTE>
SELECT *, count(*) OVER () AS total FROM rows
WHERE ($1 = 'all' OR state <> 'ok')
  AND ($2::text IS NULL OR name ILIKE '%' || $2 || '%' ESCAPE '\')
ORDER BY stock - min_stock, name, id
LIMIT $3 OFFSET $4;
```

## Stock Summary Query
```sql
<Stock Pending CTE>
SELECT count(*)                                  AS products_total,
       count(*) FILTER (WHERE state = 'low')      AS low_count,
       count(*) FILTER (WHERE state = 'out')      AS out_count,
       count(*) FILTER (WHERE state = 'will_low') AS will_low_count,
       count(*) FILTER (WHERE state <> 'ok')      AS risk_count     -- бейдж меню (AC-016)
FROM rows;
```

## Stock Adjust Query
D44, эталон `catalog.ProductRepo.Update` (`products_repo.go:141-195`).
```sql
SELECT stock::text FROM products WHERE id = $1::uuid FOR UPDATE;          -- 0 строк → PRODUCT_NOT_FOUND
UPDATE products SET stock = $2::numeric, updated_at = now() WHERE id = $1::uuid;
INSERT INTO stock_movements (product_id, qty_delta, reason, user_id)
VALUES ($1::uuid, $2::numeric - $3::numeric, 'manual', $4::uuid);        -- только если дельта ≠ 0
```

## Column Recalc Query
AC-007: смена `consumes_stock` (после `SELECT ... FROM columns ORDER BY id FOR UPDATE`).
```sql
SELECT id FROM tasks WHERE column_id = $1::uuid ORDER BY id FOR UPDATE;
-- для каждого task: Reconcile(tx, id, newConsumes, userID); warnings — объединение по product_id
```

## Column Renumber Query
```sql
-- после insert/update(position)/delete; $1 = id двигаемой колонки, $2 = целевая позиция (1..N, clamp)
WITH ordered AS (
    SELECT id, row_number() OVER (ORDER BY position, (id = $1) DESC, id) AS rn FROM columns WHERE id <> $1)
UPDATE columns c SET position = CASE WHEN o.rn >= $2 THEN o.rn + 1 ELSE o.rn END
FROM ordered o WHERE c.id = o.id;
UPDATE columns SET position = $2 WHERE id = $1;
```

## Delete Column Query
```sql
SELECT EXISTS (SELECT 1 FROM tasks WHERE column_id = $1::uuid);   -- true → 409 COLUMN_NOT_EMPTY
SELECT count(*) FROM columns WHERE NOT is_final AND id <> $1::uuid;  -- 0 и удаляемая нефинальная → 409 LAST_NON_FINAL_COLUMN
DELETE FROM columns WHERE id = $1::uuid;                           -- 23503 (гонка) → COLUMN_NOT_EMPTY
```

## Delete Task Query
```sql
-- под FOR UPDATE заказа: Reconcile(target=false) → возврат всех task_stock
DELETE FROM tasks WHERE id = $1;        -- CASCADE subtasks/subtask_products/task_stock/task_equipment; stock_movements.task_id → NULL
```

## Client Delete Guard Query
Пакет `clients` (ADR-005), без импорта board.
```sql
SELECT EXISTS (SELECT 1 FROM tasks WHERE client_id = $1::uuid);    -- true → 409 CLIENT_HAS_TASKS
```

## Equipment Delete Guard Query
```sql
SELECT EXISTS (SELECT 1 FROM task_equipment WHERE equipment_id = $1::uuid)
    OR EXISTS (SELECT 1 FROM subtasks WHERE equipment_id = $1::uuid);   -- true → 409 EQUIPMENT_USED_IN_TASKS
```

## Assignees Query
```sql
SELECT id, name, role, count(*) OVER () FROM users WHERE is_active ORDER BY name, id LIMIT $1 OFFSET $2;
```

## Seed Demo Guard Query
```sql
SELECT count(*) FROM tasks;             -- > 0 → «заказы уже есть», exit 0 (AC-019)
SELECT id FROM users WHERE role = 'admin' AND is_active ORDER BY created_at LIMIT 1;  -- нет → exit 1 «make local-up-seed»
```

## Паспорт (выжимка)
# Passport: phase-5-board (generated — руками не править; stage_bundle.py)
**Tier:** M | **Совместимость:** ? (pipeline-doctrine.md §CM-1; ? → уточни у оркестратора)
**Трек:** обычный (prefs нет — дефолт) — режимы агентов (`spec-track delta/micro/mini/merged`) включает ТОЛЬКО строка `Режим:` в prompt оркестратора; DS/TS-индексы ниже присутствуют на ЛЮБОМ треке при наличии draft-spec.
**Layout:** автодетект (tools/layout_resolve.py)
**UI-scope (pipeline-doctrine.md §US-1, 0-setup/ui-scope-prefs.md):** swagger:no | adaptive:min | themes:both

**Индексы:** AC 20 / DS 0 / TS 0 — полные списки: `0-setup/passport.md` (разрешён контрактом); конкретное решение — точечный grep по `DS-NNN`/`TS-NNN` в draft-spec §9/§11 (разрешён).
