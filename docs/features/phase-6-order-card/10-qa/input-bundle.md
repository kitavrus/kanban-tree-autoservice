# Input Bundle: phase-6-order-card / стадия 10-qa — generated 2026-09-27 16:11

> Твой ЕДИНСТВЕННЫЙ документный вход (пайп-контракт). Upstream-доки напрямую НЕ читать;
> точечный grep по конкретному ID (DS-NNN/AC-NNN/TS-NNN) — разрешён. Отсутствие нужного
> факта в бандле = сигнал оркестратору (дырка контракта), не повод читать всё.

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

## Источник: 9-validation/output.md [full]
---
stage: 9-validation
feature: phase-6-order-card
tier: M
---
# Validation: phase-6-order-card — PASS

## Платформы
- Backend (Go): ✅
- Web (SSR html/template + vanilla JS на Bootstrap, Vue нет — D30): ✅

## Гейты
- `go build ./...` ✅; `go test ./... -race -count=1` — 11 пакетов ok ✅; `golangci-lint run ./...` — 0 issues ✅
- `BASE=9580596 make test-integration-scoped` ✅ — FULL-триггер (миграция 0006) → 14 пакетов ok (board: OrderCardLiveSuite, PrintLiveSuite, CardSuite, BoardLiveSuite, StockLiveSuite)
- Чекеры (`--paths internal/features/board`): go-naming PASS, api-conventions PASS, live-suite PASS; review_bundle 8 pass / 1 skip (contract n/a — клиент vanilla JS)
- E2E полный прогон №2 (скоуп `full`, 0-setup/e2e-scope-prefs.md): `npx playwright test --project=chromium --headed --workers=1 --retries=1 --reporter=list`
- **Счётчики: passed 113 / hard-fail 0 / flaky 0 / skipped 0**
- Артефакт §TD-2: `9-validation/evidence/last-run.json` (status=passed) + `9-validation/evidence/e2e-run.txt`
- Watch-gate §TD-5: пропущен — per-run override пользователя «автоматический режим, без вопросов» (0-setup/watch-gate-prefs.md enabled: no).
- полный integration-регресс не гонялся (скоуп: task) — FULL-триггер тула всё равно расширил прогон до `./...`.

## Сценарий (AC trace, design-tests.md §Coverage AC)
- [x] AC-001 полная карточка вместо простой формы, deep-link /?task=N — LV-101/111, OC-001/002/020
- [x] AC-002 шапка кнопкой, конфликт version — LV-102/130/131, OC-003/004, OC-025
- [x] AC-003 оплата только admin — LV-161, OC-005
- [x] AC-004 принятое оборудование, TASK_EQUIPMENT_IN_USE — LV-108/142/143/144, OC-006
- [x] AC-005 новое оборудование из карточки — LV-109, OC-007
- [x] AC-006 история ремонтов единицы — LV-110, OC-008
- [x] AC-007 услуги из каталога — LV-103/104/145/146, OC-009/022
- [x] AC-008 комплект товаров услуги, position — LV-105/171, IT-020, OC-010/021
- [x] AC-009 подсказка наличия — OC-010
- [x] AC-010 списание при правке товаров в работе — LV-120…123, IT-001, OC-011
- [x] AC-011 статус услуги и финальная колонка — LV-140/141/150, OC-012
- [x] AC-012 журнал работ (своя/чужая) — LV-107/163, OC-013
- [x] AC-013 итог заказа, итог/журнал на доске, сумма колонки — LV-106, OC-014, BD-021
- [x] AC-014 одновременная правка строк (LWW, rowQueue) — IT-010…013, LV-130/170, OC-015/016/022/023/024/026
- [x] AC-015 акт приёма — LV-180/182, OC-017
- [x] AC-016 акт выдачи (все услуги со статусом, SD-009) — LV-181, OC-018
- [x] AC-017 права исполнителя — LV-160/162/164, OC-019

## Observability
- Спаны `tasks.card`, `tasks.print`, `subtasks.mutate`, `subtask_products.mutate`, `subtask_logs.mutate`, `task_equipment.mutate` — в коде (Code Review HIGH закрыт); экспорт в Jaeger не проверялся — алерты/стенд мониторинга deferred (MVP, прода нет).

## Mobile API live check
- validation skipped: no live API (dev-стенд не задеплоен; проверено локально E2E + live-сьютами)

## Источник: 8-code-review/output.md [full]
---
stage: 8-code-review
feature: phase-6-order-card
tier: M
verdict: APPROVED
high: 0
medium: 0
low: 3
---
# Code Review: phase-6-order-card — APPROVED
**Review level:** Standard (tier: M) · CM-1 MVP · UI-scope swagger:no / adaptive:min / themes:both · фронт SSR + vanilla JS (Vue/Element Plus-пункты n/a)
**Итерация:** 1 (re-review фикса `793fd75` поверх `446a259`; base 9580596)

## Блокеры (обязательно исправить до мержа)
Нет.

## Серьёзные замечания (исправить в этом PR)
Нет.

## Незначительные замечания (можно в следующем PR)
1. **[web/ssr/_shared/js/task-card.js:applyCardIfCurrent/flushRow]** Остаточная гонка seq-guard'а: `seq` присваивается в порядке ОТПРАВКИ, а не порядке commit на сервере. Если строка X (seq N) коммитится ПОЗЖЕ строки Y (seq N+1) (сервер взял FOR UPDATE заказа в обратном порядке), ответ X отбрасывается, `card` хранит старое значение X, и следующий pending-flush X собирает `baseGetter()` из устаревшей карточки → LWW-PUT откатывает первое изменение X. Окно намного уже прежнего (нужна переупорядоченная сервером пара запросов одного клиента в пределах мс). Рекомендация: база тела PUT строки = последний отправленный body этой строки (`q.lastBody`), а не `card`.
2. **[web/ssr/_shared/js/task-card.js:loadCard]** `appliedSeq = ticket.seq` без проверки `ticket.seq > appliedSeq`: `loadCard` из `onRowError`, отправленный раньше более свежего row-ответа, может прийти позже и перезаписать карточку более старым снимком (и уменьшить `appliedSeq`). Самоисправляется следующей мутацией; добавить ту же проверку, что в `applyCardIfCurrent`.
3. **[web/ssr/_shared/js/board.js:114,139; internal/features/catalog/categories_service.go:30-31]** Лимиты 1000 (tasks/columns) и 2000 (каталог) — as-is известное ограничение, молчаливое усечение без «показаны N из M»; завести issue (пагинация доски/lookup или очистка dev-стенда).

## Закрытие findings итерации 0
| # | Было | Статус | Пруф |
|---|---|---|---|
| HIGH-1 | span'ы Monitoring Spec | ✅ закрыт | `tasks.card` (card_service.go Card), `tasks.print` (PrintData), `subtasks.mutate` (create/update/delete), `subtask_products.mutate` (create/update/delete), `subtask_logs.mutate` (create/update/delete), `task_equipment.mutate`; атрибуты `task_id`/`subtask_id`/`product_id`/`op`; delete-пути переведены на `spanCtx` |
| MEDIUM-1 | ответы без guard'а | ✅ закрыт (остаток — LOW-1/2) | `openGeneration` (++ в `open()` и `hidden.bs.modal`), `beginRequest()` ДО запроса во всех 12 мутационных обработчиках + `flushRow`, `applyCardIfCurrent`; ошибки чужого поколения не показываются; E2E OC-024, OC-026 |
| MEDIUM-2 | затирание шапки | ✅ закрыт | `headerDirty` (input/change на полях шапки, bindStatic один раз), `renderAll` не трогает шапку при dirty, сброс в `loadCard`/`open`/`hidden`; `combobox._cbSync` change не диспатчит (combobox.js:144 — только выбор пользователя) → ложного dirty нет; E2E OC-025 |
| MEDIUM-3 | drift 404 в row-путях | ✅ закрыт | `lockRowsScopeFor(…, ErrSubtaskNotFound|ErrSubtaskProductNotFound)` в Update/DeleteSubtask, Create(`/subtasks/:id`→SUBTASK_NOT_FOUND)/Update/DeleteSubtaskProduct; CreateSubtask (`/tasks/:id`) оставлен на TASK_NOT_FOUND — верно по design-errors.md |
| LOW | цена услуги num()→0 | ✅ | валидация `≥0`, откат значения, слот `card-subtask-warning` |
| LOW | ошибка услуги в слоте товара | ✅ | `onRowError`: для `subtask:*` — `card-subtask-warning` |
| LOW | aria-busy в цепочке | ✅ | `setRowBusy(key,true)` в `flushRow` |
| LOW | reload доски на каждое автосохранение | ✅ | `cardTouchedBoard` → один `reloadAndRender` на `hidden.bs.modal` |
| LOW | `_boardCleanup` глотает всё | ✅ | 404/409 ожидаемы, прочее → `console.warn`; `registerTaskForCleanup(id)` для UI-созданных |
| LOW | лимиты 1000/2000 | as-is | → LOW-3 выше |
| LOW | contract SKIP-RESOLVE | n/a | клиент vanilla JS без TS-контракта; bundle помечает `contract n/a` |

## Проверено без замечаний (итерация 0, повторно не менялось)
- Lock Order ADR-100 во всех row-мутациях; `errTaskMovedRetry` ограничен `txMaxRetries=3`, tx откатывается `defer Rollback` — петли/утечек нет; статус услуги vs becomesFinal сериализуется FOR UPDATE заказа.
- Reconcile на всех изменениях состава товаров (включая каскад удаления услуги), инвариант склада — IT.
- Authz D19 (per-order ACL нет by design), журнал 404→403, оборудование — только клиента заказа, смена клиента 409/DeleteForeign.
- SQL: карточка 7 запросов без N+1, единая формула `round` итогов, backfill `position` 0006 детерминирован.
- XSS: `esc()` в JS, `html.EscapeString` описания, html/template печати (+ live-тест `<script>`).
- Тесты: live TD-8/TD-4, E2E без `waitForTimeout`; новые OC-024…026 используют `route.fetch()`+удержание ответа — детерминированно воспроизводят гонки. Прогон оркестратора: unit 11/11, lint 0, integration 14/14, E2E 47/47 (свежий артефакт — сводный гейт Validation, EE-1).

## Соответствие коду дизайну (верификация реализации)
- [x] Dataflow соответствует design-sequence-diagrams.md — ДА
- [x] SQL из design-sql.md реализованы — ДА
- [x] Go слои по design-go-layers.md — ДА
- [x] Error handling по design-errors.md — ДА
- [x] Infrastructure (`FIRM_*` в `.env.example`) — ДА
- [x] Swagger — N/A (swagger:no)
- [x] Frontend-структура (task-card.js, testid-контракт) — ДА
- [x] Monitoring Spec реализован — ДА
- [x] Алерты — N/A (explicit deferred в design)
- [x] Rollout flag / Runbook / Perf Budget — N/A (M, CM-1 MVP)

## PRD соответствие
- [x] Каждое ADR имеет Driver (ADR-001…007, ADR-100…105) — ДА
- [x] AC-001…AC-017 покрыты (OC-001…026, BD-021, OrderCardLiveSuite/PrintLiveSuite, CardSuite) — ДА
- [x] Success Metric наблюдаема (итоги/журнал на доске — BD-021) — ДА
- [x] Non-Goals не реализованы — ДА

## Статус фаз
- [x] `6-plan/code-plan-status.md` — фазы 1–6 `completed` — ДА

## Итог
APPROVED: все HIGH/MEDIUM итерации 0 закрыты с тестами; остаются 3 LOW (остаточное окно seq-guard, порядок `loadCard`, as-is лимиты).

## Источник: 8-code-review/mobile.md [verdict]
---
stage: 8-code-review
feature: phase-6-order-card
tier: M
verdict: APPROVED
high: 0
medium: 0
low: 1
---
# Mobile API Review (8-code-review): phase-6-order-card — APPROVED

## Источник: 8-code-review/frontend-arch.md [verdict]
MISSING

## Источник: 3-design/output.md [sections]
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

## Источник: 3-design/design-errors.md [full]
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

## Источник: 3-design/design-go-layers.md [sections]
## Enum SubtaskStatus
As-is `enums.go`: `SubtaskStatusNew="new"`, `SubtaskStatusInProgress="in_progress"`, `SubtaskStatusDone="done"`, `SubtaskStatuses`. Sync-тест — as-is `enums_test.go` (против CHECK `0005_board.up.sql`).

## Enum PrintKind
Новый, `enums.go`: `PrintKindIntake="intake"`, `PrintKindHandover="handover"`, `PrintKinds`. Источник — path `/tasks/:id/print/:kind`; иное → 404. Тест — `enums_test.go` (+кейс).

## DTO Task Changes
```go
// TaskCard (элемент GET /tasks, встроен в TaskResponse) — добавить:
TotalAmount numeric.Amount `json:"total_amount"` // design-sql.md §Totals Expression, AC-013
LastLog     *TaskLastLog   `json:"last_log"`     // null — записей нет, AC-012

type TaskLastLog struct {
    Text       string    `json:"text"`        // ≤120 символов + «…» при обрезке
    AuthorName string    `json:"author_name"`
    CreatedAt  time.Time `json:"created_at"`
}

// TaskResponse — добавить:
DescriptionHTML string `json:"description_html"` // хранимый HTML (ADR-006), клиент берёт текст через DOMParser

// TaskCreateRequest (и TaskUpdateRequest через embed) — добавить:
Description *string `json:"description"` // plain text ≤10000; POST nil→""; PUT nil→не менять
```
`TaskMutationResponse` / `TaskDeleteResponse` — as-is.

## DTO OrderCard
```go
// GET /api/v1/tasks/:id/card и card в ответах row-мутаций (ADR-001).
type OrderCardResponse struct {
    Task      TaskResponse    `json:"task"`
    Column    CardColumn      `json:"column"`
    Equipment []CardEquipment `json:"equipment"` // всегда массив
    Subtasks  []CardSubtask   `json:"subtasks"`
    Stock     []CardStock     `json:"stock"`     // по каждому товару карточки, для подсказки AC-009
}
// Итог заказа — только task.total_amount (одно имя в GET /tasks и в карточке; отдельного `total` нет).
type CardColumn struct {
    ID            string `json:"id"`
    Name          string `json:"name"`
    IsFinal       bool   `json:"is_final"`
    ConsumesStock bool   `json:"consumes_stock"`
}
type CardEquipment struct {
    EquipmentID  string             `json:"equipment_id"`
    TypeName     string             `json:"type_name"`
    Brand        string             `json:"brand"`
    Model        string             `json:"model"`
    Serial       string             `json:"serial"`
    Completeness string             `json:"completeness"`
    Condition    string             `json:"condition"`
    History      []EquipmentHistory `json:"history"` // другие заказы с единицей, AC-006
}
type EquipmentHistory struct {
    TaskID    int64     `json:"task_id"`
    Title     string    `json:"title"`
    CreatedAt time.Time `json:"created_at"` // дата заказа для «Уже был в ремонте» (AC-006)
}
type CardSubtask struct {
    ID               string               `json:"id"`
    ServiceID        string               `json:"service_id"`
    ServiceName      string               `json:"service_name"`
    Status           string               `json:"status"`             // Enum SubtaskStatus
    AssigneeID       *string              `json:"assignee_id"`
    AssigneeName     *string              `json:"assignee_name"`
    AssigneeIsActive *bool                `json:"assignee_is_active"` // null без исполнителя; false → «неактивен» (SD-007)
    DueDate          *string              `json:"due_date"`           // YYYY-MM-DD
    Price            *numeric.Amount      `json:"price"`              // null = «по факту»
    EquipmentID      *string              `json:"equipment_id"`
    Position         int                  `json:"position"`
    Total            numeric.Amount       `json:"total"`              // subtask_total
    Products         []CardSubtaskProduct `json:"products"`
    Logs             []CardSubtaskLog     `json:"logs"`
}
type CardSubtaskProduct struct {
    ID        string          `json:"id"`
    ProductID string          `json:"product_id"`
    Name      string          `json:"name"`
    Unit      string          `json:"unit"`
    Position  int             `json:"position"` // ADR-103; массив уже отсортирован
    Qty       numeric.Amount  `json:"qty"`
    Price     *numeric.Amount `json:"price"`
    LineTotal numeric.Amount  `json:"line_total"`
}
type CardSubtaskLog struct {
    ID         string    `json:"id"`
    Text       string    `json:"text"`
    UserID     string    `json:"user_id"`
    AuthorName string    `json:"author_name"`
    CreatedAt  time.Time `json:"created_at"`
    UpdatedAt  time.Time `json:"updated_at"`
    CanEdit    bool      `json:"can_edit"` // автор == актор || актор admin (D27)
}
type CardStock struct {
    ProductID    string         `json:"product_id"`
    Stock        numeric.Amount `json:"stock"`
    MinStock     numeric.Amount `json:"min_stock"`
    TaskReserved numeric.Amount `json:"task_reserved"` // task_stock.qty этого заказа, 0 если нет
}
// Ответ всех row-мутаций (POST 201 / PUT 200 / DELETE 200).
type OrderCardMutationResponse struct {
    Card     OrderCardResponse `json:"card"`
    Warnings []StockWarning    `json:"warnings"` // as-is формат ADR-006 фазы 5; [] без Reconcile
}
```

## DTO Row Requests
Все — strictJSON (лишнее поле → 400 `BAD_REQUEST`), `normalize()/validate() []errorspkg.Field` на типе запроса (эталон `tasks_validate.go`). Правила — `design-errors.md §Validation Fields`.
```go
type SubtaskCreateRequest struct { ServiceID string `json:"service_id"` }
// PUT /subtasks/:id — полная замена изменяемых полей строки (LWW); клиент шлёт всю строку.
type SubtaskUpdateRequest struct {
    Status      string          `json:"status"`
    AssigneeID  *string         `json:"assignee_id"`
    DueDate     *string         `json:"due_date"`
    Price       *numeric.Amount `json:"price"`
    EquipmentID *string         `json:"equipment_id"`
}
type SubtaskProductCreateRequest struct {
    ProductID string          `json:"product_id"`
    Qty       *numeric.Amount `json:"qty"` // цена копируется из products.price (AC-008)
}
type SubtaskProductUpdateRequest struct {
    Qty   *numeric.Amount `json:"qty"`
    Price *numeric.Amount `json:"price"` // null = без цены (в итог 0)
}
type TaskEquipmentCreateRequest struct {
    EquipmentID  string `json:"equipment_id"`
    Completeness string `json:"completeness"`
    Condition    string `json:"condition"`
}
type TaskEquipmentUpdateRequest struct {
    Completeness string `json:"completeness"`
    Condition    string `json:"condition"`
}
type SubtaskLogRequest struct { Text string `json:"text"` }
```

## Источник: 3-design/design-tests.md [full]
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

## Источник: 3-design/design-sql.md [full]
# Design SQL: phase-6-order-card

> Owner: DDL, миграция, запросы. SQL — Go-константы в `*_repo.go` пакета `board` (as-is стиль фазы 5, без go:embed). Lock Order — §Lock Order (расширение ADR-104 фазы 5).

## Subtask Logs Table
Новая. DDL — §Subtask Logs Migration. `user_id` RESTRICT (ADR-007): пользователи не удаляются (фича `users` — только блокировка).

## Subtasks Table
As-is (`migrations/0005_board.up.sql:46-62`), без изменений DDL:
```sql
subtasks(id uuid PK DEFAULT gen_random_uuid(), task_id bigint FK tasks CASCADE, service_id uuid FK services RESTRICT,
  status text DEFAULT 'new' CHECK IN ('new','in_progress','done'), assignee_id uuid FK users SET NULL, due_date date NULL,
  price numeric(12,2) NULL CHECK(price IS NULL OR price>=0), equipment_id uuid FK equipment RESTRICT NULL,
  position int DEFAULT 0, created_at, updated_at)
-- индексы: subtasks_task_id_idx, subtasks_service_id_idx, subtasks_equipment_id_idx, subtasks_assignee_id_idx
```

## Subtask Products Table
As-is (`0005_board.up.sql:64-72`) + колонка `position` (ADR-103):
```sql
subtask_products(id uuid PK DEFAULT gen_random_uuid(), subtask_id uuid FK subtasks CASCADE, product_id uuid FK products RESTRICT,
  qty numeric(12,3) CHECK(qty>0), price numeric(12,2) NULL CHECK(price IS NULL OR price>=0))
-- индексы: subtask_products_subtask_id_idx, subtask_products_product_id_idx
```

## Task Equipment Table
As-is (`0005_board.up.sql:82-89`): `task_equipment(task_id bigint FK tasks CASCADE, equipment_id uuid FK equipment RESTRICT, completeness text DEFAULT '', condition text DEFAULT '', PK(task_id, equipment_id))`, индекс `task_equipment_equipment_id_idx`. Без изменений.

## Tasks Table
As-is. `description_html text DEFAULT ''` (`0005_board.up.sql:30`) начинает писаться (ADR-006). `version` строками НЕ инкрементируется (ADR-002).

## Subtask Logs Migration
`migrations/0006_subtask_logs.up.sql` (применение — только `make migrate-up`):
```sql
CREATE TABLE subtask_logs (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    subtask_id uuid NOT NULL REFERENCES subtasks(id) ON DELETE CASCADE,
    user_id    uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    text       text NOT NULL CHECK (btrim(text) <> '' AND char_length(text) <= 4000),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX subtask_logs_subtask_created_idx ON subtask_logs (subtask_id, created_at);
CREATE INDEX subtask_logs_user_id_idx ON subtask_logs (user_id);

ALTER TABLE subtask_products ADD COLUMN position int NOT NULL DEFAULT 0;
-- backfill (ADR-103): существующие строки (seed-demo, dev-БД фазы 5) — 1..N внутри услуги
UPDATE subtask_products sp SET position = o.rn
FROM (SELECT id, row_number() OVER (PARTITION BY subtask_id ORDER BY product_id, id) AS rn FROM subtask_products) o
WHERE o.id = sp.id;
```
`migrations/0006_subtask_logs.down.sql` (CM-1 MVP — простой DROP):
```sql
ALTER TABLE subtask_products DROP COLUMN IF EXISTS position;
DROP TABLE IF EXISTS subtask_logs;
```

## Lock Order
ADR-104 фазы 5 расширен (ADR-100): **columns (FOR SHARE) → tasks (FOR UPDATE) → subtasks (FOR UPDATE) → subtask_products (FOR UPDATE) → products (FOR UPDATE ORDER BY id, внутри Reconcile)**.
- Мутации услуг/товаров (вкл. смену статуса услуги — LOW фазы 5): Task Column Id → Lock Target Column (as-is) → Lock Task (as-is) → сверка `column_id` (иначе `errTaskMovedRetry`) → Lock Subtask / Lock Subtask Product → правка → `Reconcile(taskID, consumes_stock)` при изменении состава товаров.
- Оборудование: Lock Task (без колонки: колонку не читаем) → правка.
- Журнал: без блокировки заказа (склад не трогает); FK-проверка берёт KEY SHARE на `subtasks` — цикла с DELETE услуги (tasks→subtasks) нет.

## Task Column Id Query
```sql
SELECT column_id FROM tasks WHERE id = $1   -- 0 строк → TASK_NOT_FOUND
```

## Subtask Parent Query
```sql
SELECT task_id FROM subtasks WHERE id = $1                                  -- 0 → SUBTASK_NOT_FOUND
SELECT s.task_id, sp.subtask_id FROM subtask_products sp JOIN subtasks s ON s.id = sp.subtask_id
WHERE sp.id = $1                                                            -- 0 → SUBTASK_PRODUCT_NOT_FOUND
```

## Lock Subtask Query
```sql
SELECT id, status, assignee_id::text FROM subtasks WHERE id = $1 AND task_id = $2 FOR UPDATE
-- 0 строк (удалена/перенесена параллельно) → SUBTASK_NOT_FOUND
SELECT id FROM subtask_products WHERE id = $1 AND subtask_id = $2 FOR UPDATE -- 0 → SUBTASK_PRODUCT_NOT_FOUND
```

## Insert Subtask Query
Одна tx (AC-007, AC-008): услуга + копия комплекта.
```sql
-- $1 task_id, $2 service_id; 0 строк → 400 service_id (сервис заранее различает «не найдена» / «в архиве»)
INSERT INTO subtasks (task_id, service_id, price, position)
SELECT $1, sv.id, sv.price, coalesce((SELECT max(position) FROM subtasks WHERE task_id = $1), 0) + 1
FROM services sv WHERE sv.id = $2 AND sv.is_active
RETURNING id;

-- $1 subtask_id, $2 service_id: комплект с текущей ценой товара
INSERT INTO subtask_products (subtask_id, product_id, qty, price, position)
SELECT $1, spr.product_id, spr.qty, p.price, row_number() OVER (ORDER BY spr.position, spr.product_id)
FROM service_products spr JOIN products p ON p.id = spr.product_id
WHERE spr.service_id = $2;
```

## Update Subtask Query
```sql
UPDATE subtasks SET status = $2, assignee_id = $3, due_date = $4, price = $5::numeric, equipment_id = $6, updated_at = now()
WHERE id = $1
```
`equipment_id` проверяется до UPDATE: `SELECT 1 FROM task_equipment WHERE task_id = $1 AND equipment_id = $2`.

## Delete Subtask Query
```sql
DELETE FROM subtasks WHERE id = $1   -- CASCADE: subtask_products, subtask_logs
```

## Subtask Product Queries
```sql
-- Insert ($1 subtask_id, $2 product_id, $3 qty); 0 строк → 400 product_id
INSERT INTO subtask_products (subtask_id, product_id, qty, price, position)
SELECT $1, p.id, $3::numeric, p.price, coalesce((SELECT max(position) FROM subtask_products WHERE subtask_id = $1), 0) + 1
FROM products p WHERE p.id = $2 AND p.is_active
RETURNING id;
-- Update (LWW, D28)
UPDATE subtask_products SET qty = $2::numeric, price = $3::numeric WHERE id = $1;
-- Delete
DELETE FROM subtask_products WHERE id = $1;
```
После каждой — `StockEngine.Reconcile(tx, taskID, column.consumes_stock, actorID)` (as-is, `stock_engine.go`).

## Task Equipment Queries
```sql
-- Owner check ($1 equipment_id): client_id для сравнения с tasks.client_id (из LockTask)
SELECT client_id FROM equipment WHERE id = $1;
-- Insert (идемпотентно)
INSERT INTO task_equipment (task_id, equipment_id, completeness, condition) VALUES ($1, $2, $3, $4)
ON CONFLICT (task_id, equipment_id) DO NOTHING;
-- Update; 0 строк → TASK_EQUIPMENT_NOT_FOUND
UPDATE task_equipment SET completeness = $3, condition = $4 WHERE task_id = $1 AND equipment_id = $2;
-- In-use (удаление единицы): непустой → 409 TASK_EQUIPMENT_IN_USE details.subtasks
SELECT s.id, sv.name, s.status FROM subtasks s JOIN services sv ON sv.id = s.service_id
WHERE s.task_id = $1 AND s.equipment_id = $2 ORDER BY s.position, s.id;
-- Delete; 0 строк → TASK_EQUIPMENT_NOT_FOUND
DELETE FROM task_equipment WHERE task_id = $1 AND equipment_id = $2;
```

## Foreign Equipment Query
`PUT /tasks/:id` со сменой клиента, после LockTask, до UPDATE tasks (AC-004):
```sql
-- $1 task_id, $2 new client_id. Непустой → 409 TASK_EQUIPMENT_IN_USE details.subtasks
SELECT s.id, sv.name, s.status
FROM subtasks s JOIN services sv ON sv.id = s.service_id JOIN equipment e ON e.id = s.equipment_id
WHERE s.task_id = $1 AND e.client_id <> $2 ORDER BY s.position, s.id;
-- иначе
DELETE FROM task_equipment te USING equipment e
WHERE te.task_id = $1 AND e.id = te.equipment_id AND e.client_id <> $2;
```

## Subtask Log Queries
```sql
-- Insert ($1 subtask_id, $2 user_id, $3 text); 23503 subtask_id → SUBTASK_NOT_FOUND
INSERT INTO subtask_logs (subtask_id, user_id, text) VALUES ($1, $2, $3) RETURNING id;
-- Author/parent: 0 → SUBTASK_LOG_NOT_FOUND
SELECT l.user_id, s.task_id FROM subtask_logs l JOIN subtasks s ON s.id = l.subtask_id WHERE l.id = $1;
-- Update / Delete (после проверки автора); 0 строк → SUBTASK_LOG_NOT_FOUND
UPDATE subtask_logs SET text = $2, updated_at = now() WHERE id = $1;
DELETE FROM subtask_logs WHERE id = $1;
```

## Totals Expression
Каноничная формула (ADR-005, AC-013), аддитивна на всех уровнях — сумма отображённых строк = итог:
- `line_total = round(sp.qty * coalesce(sp.price, 0), 2)`
- `subtask_total = coalesce(s.price, 0) + Σ line_total` (услуга «по факту» — работа 0, товары входят)
- `task_total = Σ subtask_total`
```sql
-- фрагмент-скаляр, коррелирован с t.id (Board List / Get / Card)
(SELECT coalesce(sum(coalesce(s.price, 0) +
        coalesce((SELECT sum(round(sp.qty * coalesce(sp.price, 0), 2))
                  FROM subtask_products sp WHERE sp.subtask_id = s.id), 0)), 0)
 FROM subtasks s WHERE s.task_id = t.id)::text AS total_amount
```

## Last Log Expression
```sql
LEFT JOIN LATERAL (
  SELECT left(l.text, 120) AS text, char_length(l.text) > 120 AS cut, u.name AS author_name, l.created_at
  FROM subtasks s JOIN subtask_logs l ON l.subtask_id = s.id JOIN users u ON u.id = l.user_id
  WHERE s.task_id = t.id ORDER BY l.created_at DESC, l.id DESC LIMIT 1) ll ON true
```
Go: `cut` → добавить «…». Индексы: `subtasks_task_id_idx` + `subtask_logs_subtask_created_idx`.

## Board List Query
As-is `boardListSQL` (`tasks_repo.go`) + в SELECT: Totals Expression, `ll.text, ll.cut, ll.author_name, ll.created_at` (Last Log Expression — JOIN после `LEFT JOIN users u`). Фильтры/порядок/`LIMIT 500` — as-is. `TaskRepo.Get` — те же добавки + `t.description_html`. Сумма колонки — клиент (ADR-005).

## Card Queries
`GET /tasks/:id/card` — 7 запросов на пуле (без tx, read committed), без N+1:
```sql
-- 1. шапка: TaskRepo.Get (as-is + description_html/total_amount/last_log)
-- 2. колонка
SELECT id, name, is_final, consumes_stock FROM columns WHERE id = $1;
-- 3. оборудование + история (AC-006)
SELECT te.equipment_id, et.name, e.brand, e.model, e.serial, te.completeness, te.condition,
       coalesce((SELECT json_agg(json_build_object('task_id', o.id, 'title', o.title, 'created_at', o.created_at) ORDER BY o.id DESC)
                 FROM task_equipment x JOIN tasks o ON o.id = x.task_id
                 WHERE x.equipment_id = te.equipment_id AND x.task_id <> te.task_id), '[]'::json)
FROM task_equipment te JOIN equipment e ON e.id = te.equipment_id JOIN equipment_types et ON et.id = e.type_id
WHERE te.task_id = $1 ORDER BY et.name, e.brand, e.model, e.id;
-- 4. услуги
SELECT s.id, s.service_id, sv.name, s.status, s.assignee_id::text, u.name, u.is_active, s.due_date::text,
       s.price::text, s.equipment_id::text, s.position
FROM subtasks s JOIN services sv ON sv.id = s.service_id LEFT JOIN users u ON u.id = s.assignee_id
WHERE s.task_id = $1 ORDER BY s.position, s.id;
-- 5. товары всех услуг
SELECT sp.id, sp.subtask_id, sp.product_id, p.name, p.unit, sp.position, sp.qty::text, sp.price::text,
       round(sp.qty * coalesce(sp.price, 0), 2)::text
FROM subtask_products sp JOIN subtasks s ON s.id = sp.subtask_id JOIN products p ON p.id = sp.product_id
WHERE s.task_id = $1 ORDER BY sp.subtask_id, sp.position, sp.id;
-- 6. журнал всех услуг
SELECT l.id, l.subtask_id, l.text, l.user_id, u.name, l.created_at, l.updated_at
FROM subtask_logs l JOIN subtasks s ON s.id = l.subtask_id JOIN users u ON u.id = l.user_id
WHERE s.task_id = $1 ORDER BY l.subtask_id, l.created_at, l.id;
-- 7. остатки товаров карточки (AC-009)
SELECT p.id, p.stock::text, p.min_stock::text, coalesce(ts.qty, 0)::text
FROM products p LEFT JOIN task_stock ts ON ts.task_id = $1 AND ts.product_id = p.id
WHERE p.id IN (SELECT sp.product_id FROM subtask_products sp JOIN subtasks s ON s.id = sp.subtask_id WHERE s.task_id = $1)
ORDER BY p.id;
```
Go десятичной арифметики не делает: `subtask_total` — доп. скаляр в запросе 4 (`coalesce(s.price,0) + coalesce((SELECT sum(round(sp.qty*coalesce(sp.price,0),2)) FROM subtask_products sp WHERE sp.subtask_id = s.id),0)`), итог карточки — `task.total_amount` из запроса 1 (Totals Expression), отдельного поля нет.

## Print Header Query
```sql
SELECT cl.company_name, cl.contact_name,
       coalesce((SELECT string_agg(ph.phone, ', ' ORDER BY ph.position) FROM client_phones ph WHERE ph.client_id = cl.id), '')
FROM tasks t JOIN clients cl ON cl.id = t.client_id WHERE t.id = $1;
```
`client_phones.position` — as-is (`0003_clients_equipment.up.sql:16`). Остальное — Card Queries.

## Паспорт (выжимка)
# Passport: phase-6-order-card (generated — руками не править; stage_bundle.py)
**Tier:** M | **Совместимость:** ? (pipeline-doctrine.md §CM-1; ? → уточни у оркестратора)
**Трек:** обычный (prefs нет — дефолт) — режимы агентов (`spec-track delta/micro/mini/merged`) включает ТОЛЬКО строка `Режим:` в prompt оркестратора; DS/TS-индексы ниже присутствуют на ЛЮБОМ треке при наличии draft-spec.
**Layout:** автодетект (tools/layout_resolve.py)
**UI-scope (pipeline-doctrine.md §US-1, 0-setup/ui-scope-prefs.md):** swagger:no | adaptive:min | themes:both

**Индексы:** AC 17 / DS 0 / TS 0 — полные списки: `0-setup/passport.md` (разрешён контрактом); конкретное решение — точечный grep по `DS-NNN`/`TS-NNN` в draft-spec §9/§11 (разрешён).
