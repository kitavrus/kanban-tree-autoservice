# light-progress — executor-payouts

> Анти-компакт чеклист. После компакта — перечитать и продолжить с незакрытого пункта.

## Решения (кратко; полные — docs/plan.md D62–D71)
- Отчёт — отдельный пакет `internal/features/executorpayouts` (как stocktransfers, D50); subtasks/фильтр/метка — в `board`.
- PUT /subtasks/:id: `is_executor_paid` (*bool) и `executor_payout` (OptionalAmount): нет поля → не менять; payout null → очистить. Карточка шлёт их только при правке самих полей (fix-loop №2).
- 409 `SUBTASK_EXECUTOR_PAID`; 400 `is_executor_paid: no_assignee`.
- API: GET /api/v1/executor-payouts ({items,total,limit,offset}), GET /api/v1/executor-payouts/summary (итоги по исполнителям, тот же конверт), POST /api/v1/executor-payouts/mark → `{updated}`.
- Доска: `executor_paid=all|paid|unpaid`, поле `executor_unpaid_done`.
- Экран /executor-payouts (admin и executor), JS `executor-payouts.js`, пункт «Работа → Выплаты».

## Работы
- [x] Миграция 0011 up/down
- [x] board: DTO/validate/repo/service subtasks (флаг, сумма, дата, 400/409)
- [x] board: фильтр executor_paid + executor_unpaid_done в GET /tasks и GET /tasks/:id
- [x] executorpayouts: dto/validate/repo/service/handler/routes/ssr + wiring router/menu
- [x] Фронт: карточка (task-card.js), доска (board.html/board.js), экран отчёта (html+js)
- [x] seed-demo
- [x] docs/plan.md D62–D71
- [x] Unit-тесты
- [x] Integration (LiveSuite) + миграционный
- [x] E2E специи + page objects + типы фикстур (только компиляция `--list`; прогон — оркестратор)
- [x] Self-checks: build / vet / unit / integration / lint / tsc / playwright --list

## Тест-сценарии → тесты
- [x] TS-001 admin отмечает выплату (флаг, дата, сумма) — `board/executor_payouts_http_live_test.go` TestExecutorPayouts_TS001_AdminMarksPaid
- [x] TS-002 executor отмечает/правит сумму/снимает — TestExecutorPayouts_TS002_ExecutorAllowed; E2E EP-002
- [x] TS-003 отметка без исполнителя → 400 no_assignee — TestExecutorPayouts_TS003_NoAssignee400
- [x] TS-004 смена/снятие исполнителя у выплаченной → 409 — TestExecutorPayouts_TS004_ChangeAssigneeOfPaid409; E2E EP-001
- [x] TS-005 executor_paid_at ставится/не меняется/обнуляется — TestExecutorPayouts_TS005_PaidAtLifecycle
- [x] TS-006 сумма независима от флага, невалидная → 400, null очищает — TestExecutorPayouts_TS006_PayoutIndependentOfFlag
- [x] TS-007 PUT без is_executor_paid флаг не меняет — TestExecutorPayouts_TS007_FlagOmittedKeepsValue
- [x] TS-008 фильтр доски all/paid/unpaid — TestExecutorPayouts_TS008_BoardFilter
- [x] TS-009 executor_paid=мусор → 400 — TestExecutorPayouts_TS009_BoardFilterInvalid400
- [x] TS-010 метка executor_unpaid_done — TestExecutorPayouts_TS010_BoardBadgeCount
- [x] TS-011 отчёт: список/конверт/поля/порядок/страницы — `executorpayouts/executorpayouts_http_live_test.go` TestTS011_List_EnvelopeFieldsOrder
- [x] TS-012 отчёт: фильтры и дефолт по роли, невалидные → 400 — TestTS012_List_FiltersAndRoleDefault, TestTS012_List_InvalidParams400
- [x] TS-013 отчёт: итоги по исполнителям — TestTS013_Summary_TotalsByExecutor
- [x] TS-014 массовая отметка/снятие, идемпотентность — TestTS014_Mark_SetUnsetIdempotent
- [x] TS-015 массовая отметка атомарна — TestTS015_Mark_AtomicOnInvalidItem; E2E EP-006
- [x] TS-016 валидация тела mark → 400 — TestTS016_Mark_InvalidBody400
- [x] TS-017 401 без сессии; executor допущен — TestTS017_Access_NoSession401_ExecutorAllowed
- [x] TS-018 миграция 0011 up/down — `executorpayouts/migration_integration_test.go` TestTS018_UpAddsPayoutColumns_DownDropsThem
- [x] TS-019 seed-demo: выплаченные и невыплаченные — `seeddemo/seeddemo_integration_test.go` assertExecutorPayouts (в TestSeedDemo_Run_EmptyDB_CreatesFullDemoSet)
- [x] TS-020 unit: validate/разбор фильтров — `board/executor_payouts_test.go` TestTS020_*, `executorpayouts/validate_test.go` TestTS020_*
- [x] TS-021 E2E карточка — `e2e/tests/executor-payouts.spec.ts` EP-001, EP-002 (прогнано оркестратором — 9-validation/evidence)
- [x] TS-022 E2E доска — EP-003 (прогнано оркестратором — 9-validation/evidence)
- [x] TS-023 E2E отчёт — EP-004…EP-007 (прогнано оркестратором — 9-validation/evidence)
- [x] TS-024 SSR /executor-payouts admin/executor/без сессии — TestTS024_SSR_Page

## Self-checks (факт)
- `go build ./...` → exit 0
- `go vet -tags=integration` (board, executorpayouts, seeddemo, server, ssr) → exit 0
- `go test ./... -count=1` → все пакеты ok
- `go test ./internal/features/executorpayouts/... -tags=integration -count=1 -v` → ok, 20 PASS / 0 FAIL
- `go test ./internal/features/board/... -tags=integration -count=1 -v` → ok, 293 PASS / 0 FAIL (10 новых TestExecutorPayouts_TS*)
- `go test ./internal/seeddemo/... ./internal/server/... ./internal/ssr/... -tags=integration -count=1 -v` → ok
- `golangci-lint run ./...` → 0 issues
- `api_conventions_check.py` / `go_naming_check.py` (--paths executorpayouts board) → PASS
- `node --check` (executor-payouts.js, task-card.js, board.js) → OK
- `npx tsc --noEmit -p e2e` → exit 0
- `npx playwright test tests/executor-payouts.spec.ts --project=chromium --list` → 7 тестов

## Не проверено
- E2E не прогонялись (зона оркестратора). Перед прогоном dev-стенд нужно обновить: миграция 0011 (`make migrate-up`) + перезапуск backend.
- Вживую в браузере экраны не открывались.

## Fix-loop №2 (code review: HIGH-1, MEDIUM-1…4, LOW)
- [x] HIGH-1 task-card.js: поля выплаты — только в patch, не в базе тела PUT
- [x] MEDIUM-1 executor_payout: нет поля → не менять / null → очистить (OptionalAmount); D65; фикстура E2E
- [x] MEDIUM-2 отчёт: в mark только строки с отличающейся отметкой; >500 — сообщение, без чанкинга
- [x] MEDIUM-3 тесты конкуренции mark ↔ PUT + детерминированный «устаревшая карточка» + E2E EP-008
- [x] MEDIUM-4 no_assignee/409 по переходу; осиротевшая выплаченная услуга редактируема; D66
- [x] LOW: esc() для executor_unpaid_done; убран неиспользуемый viewerId; TS-021…023 отмечены
- [x] TS-025 устаревшая карточка не затирает отметку/сумму — board live TestExecutorPayouts_TS025_StaleCardKeepsMarkFromReport; unit TestTS020_SubtaskUpdateRequest_ExecutorPayoutPresence; E2E EP-008
- [x] TS-026 конкуренция mark ↔ PUT другого поля (25 итераций) — TestExecutorPayouts_TS026_ConcurrentMarkAndPut
- [x] TS-027 конкуренция mark ↔ PUT снимает исполнителя (20 итераций) — TestExecutorPayouts_TS027_ConcurrentMarkAndUnassign
- [x] TS-028 выплаченная услуга без исполнителя редактируема — TestExecutorPayouts_TS028_OrphanPaidSubtaskEditable
- [ ] E2E EP-008 и новые шаги EP-001/EP-004 — написаны, прогон за оркестратором

## Доработка: правка суммы выплаты на экране «Выплаты» (D72)
- [x] API `PUT /api/v1/executor-payouts/:id` — только сумма; dto/validate/repo/service/handler/routes
- [x] UI: инлайн-поле суммы в колонке «Выплата» (blur/Enter, откат при ошибке, перезапрос итогов)
- [x] D72 в docs/plan.md
- [x] TS-029 правка суммы меняет только сумму (admin/executor), null очищает, итоги отражают — live TestTS029_UpdatePayout_OnlySumChanges
- [x] TS-030 отказы: нет поля/невалидная сумма/мусор/лишнее поле → 400, битый uuid → 400, нет услуги/без исполнителя → 404, без сессии → 401 — live TestTS030_UpdatePayout_Rejections
- [x] TS-031 unit: валидация тела — validate_test.go TestTS031_PayoutUpdateRequestValidate
- [x] TS-032 конкуренция правки суммы с mark и PUT карточки (20 итераций) — live TestTS032_UpdatePayout_ConcurrentWithMarkAndCardPut
- [x] TS-033 E2E: правка суммы в отчёте (admin EP-009, executor EP-010) — прогнано оркестратором
- [x] EP-004: локаторы суммы переведены с текста на значение поля (rowPayoutInput)

## Fix-loop №3 (ревью D72: MEDIUM-1, MEDIUM-2, LOW)
- [x] MEDIUM-1: после сохранения суммы — точечное обновление (поле строки + шапки групп + общие итоги), без пересоздания строк; строка ищется по subtask_id; render() сохраняет фокус/ввод активного поля суммы; load()/mark() ждут незавершённые сохранения
- [x] MEDIUM-2: поле суммы — type="text" inputmode="decimal", разбор как в карточке (запятая → точка), невалидный ввод → сообщение и откат
- [x] LOW: «слишком большая сумма» отдельным сообщением без запроса; TS-033 отмечен
- [ ] TS-034 E2E: ввод в соседнем поле во время сохранения не теряется (EP-011 — клик, EP-012 — Tab) — написано, прогон за оркестратором
- [ ] EP-009: добавлены запятая («1500,5» → 1500.5), мусор → откат, слишком большое число — прогон за оркестратором
