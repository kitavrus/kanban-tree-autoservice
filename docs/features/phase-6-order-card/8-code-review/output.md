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
