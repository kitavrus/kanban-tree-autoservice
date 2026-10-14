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
