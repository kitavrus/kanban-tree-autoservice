---
stage: 6-plan
feature: phase-6-order-card
tier: M
---
# Code Plan: phase-6-order-card — полная карточка заказа
**Дата:** 2026-09-27
**На основе:** 3-design/output.md, design-sql.md, design-errors.md, design-go-layers.md, design-tests.md, design-sequence-diagrams.md
**Совместимость:** CM-1 MVP — без dual-write/dual-read, down-миграция 0006 = прямой DROP/удаление колонки.
**Трек:** обычный (draft-spec MISSING в бандле — DS/TS-индексы не применяются).
**Статусы:** [6-plan/code-plan-status.md](code-plan-status.md) — единственный source of truth.

## Обзор фаз
| # | Фаза | Layer | Файл | AC |
|---|------|-------|------|----|
| 01 | Миграция 0006 (subtask_logs, subtask_products.position+backfill) + errors 16-22 + ADR-100 lock-retry | backend | [phase-01](code-plan-phase-01-migration-errors-lock.md) | AC-008 (position), — |
| 02 | API карточки: GET /tasks/:id/card, описание (ADR-006), итоги/last_log (ADR-005), оборудование заказа + история | backend | [phase-02](code-plan-phase-02-card-api-equipment.md) | AC-001, AC-002, AC-004, AC-005, AC-006, AC-013 (частично) |
| 03 | Услуги и товары (CRUD, комплект, Reconcile, статус, SUBTASK_FINAL_COLUMN) + журнал работ + фикстуры E2E→API | backend | [phase-03](code-plan-phase-03-subtasks-products-logs.md) | AC-007, AC-008, AC-009, AC-010, AC-011, AC-012, AC-013, AC-014, AC-017 |
| 04 | SSR печать актов (ADR-102, FIRM_*) | backend | [phase-04](code-plan-phase-04-print-ssr.md) | AC-015, AC-016 |
| 05 | UI карточки `task-card.js` (шапка, оборудование, услуги/товары rowQueue, журнал, итог, печать) + E2E | frontend | [phase-05](code-plan-phase-05-ui-task-card.md) | AC-001…AC-012, AC-014, AC-015…AC-017 (UI) |
| 06 | Доска: итог/последняя запись на карточке, сумма колонки, deep-link `/?task=N` + E2E + 360px/темы | frontend | [phase-06](code-plan-phase-06-board-integration-polish.md) | AC-001, AC-013 (UI), UI-scope gate |

## Зависимости между фазами
- 01 → 02 (Card API читает `subtask_logs`/`position` только для формы ответа; коды 16-22 нужны хендлерам 02/03; `errTaskMovedRetry`/`withTx`-ретрай нужен `lockRowsScope` в 02/03).
- 02 → 03 (`CardService`/`CardRepo`/`card_handler.go`/`lockRowsScope`/`cardMutation` заведены в 02, 03 расширяет теми же файлами методами услуг/товаров/журнала).
- 02 → 04 (`PrintHeader` в `card_repo.go`, `Card()` — источник данных печати; FirmInfo независим, но `print_ssr.go` вызывает `CardService.PrintData`, добавляемый в 04 поверх 02).
- 03 → 04 (акт выдачи печатает услуги/товары — нужны реальные данные API 03, хотя бы для LiveSuite-фикстур печати).
- 02+03 → 05 (UI карточки дергает весь набор row-эндпоинтов).
- 04 → 05 (кнопки печати в карточке ведут на SSR-роуты 04).
- 05 → 06 (доска показывает `total_amount`/`last_log` карточек и deep-link поверх готового `task-card.js`).

## Риски первыми
Миграция `subtask_products.position` (backfill без порядка исходных данных — IT-020) и порядок блокировок `columns→tasks→subtasks→subtask_products→products` с ретраем `errTaskMovedRetry` (ADR-100) — фаза 01, до всего остального API.

## Общие правила
- Каждая фаза = один атомарный коммит; `make build` проходит после каждой.
- Тесты — в той же фазе, что код (TD-1, fix-before-next); фикстуры/testdata обновляются в фазе, меняющей схему/DTO (не откладывать).
- `//go:build integration` для integration-тестов (`BaseSuite`, dockertest `postgres:18-alpine`); `testutil.LiveSuite` для HTTP live (TD-8) — `app.Test` запрещён, кроме fuzz-таргетов (`board_fuzz_test.go`, §9.1).
- UUID — `gen_random_uuid()` в БД (as-is фазы 5); Go-код id для `subtasks`/`subtask_products`/`subtask_logs`/`task_equipment` не генерирует — новых UUID-либ не вводить.
- `pkg/numeric.Amount` — ЕДИНСТВЕННЫЙ тип для NUMERIC-полей (`qty`, `price`, `total_amount`, `line_total`, `stock`/`min_stock`/`task_reserved`); nullable-цены — `*numeric.Amount`.
- `pkg/errorspkg.WithDetail` — ЕДИНСТВЕННЫЙ способ приложить `details.subtasks[]` к `ErrTaskEquipmentInUse`; ad-hoc тип ответа — запрещён.
- Фронтенд — SSR html/template + vanilla JS (Bootstrap 5.3), Vue/Element Plus неприменимо (D30/D38); UI-фазы (05, 06) исполняет `backend-dev`, `frontend_phase_gate.sh` не применяется.
- Scaffold не используется (D43): все файлы фазы — `MODIFY (расширение as-is пакета board)` там, где явно указано; новые файлы пакета создаются вручную по эталону фазы 5.
- Фазы, регистрирующие/меняющие handler или router (02, 03, 04) — DoD включает live-сьюту (TD-8): `order_card_http_live_test.go` (02, расширяется 03), `print_http_live_test.go` (04), регресс `board_http_live_test.go` (03, LV-150).
- Fuzz (TD-7, класс 9 — новые bind-парсеры тел): таргеты `board_fuzz_test.go` FZ-001…003 — фаза 03 (design-tests.md §Fuzzing), `app.Test` — разрешённое исключение §9.1. pw-fuzz: явного вердикта `**pw-fuzz:**` в дизайне нет → класс 9.2: n/a (design).
- Условные фазы 11.5 (Observability)/11.6 (Runbook/Perf docs) — НЕ создаются: `## Monitoring & Alerting Spec`/`## Runbook`/`## Performance Budget` в design помечены `deferred` (CM-1 MVP); span'ы `board`-трейсера — часть DoD фаз 02/03/04 (перечень в design-go-layers.md §Service Interface), отдельной observability-фазы нет.
- `## Rollout Strategy` в design — «без feature-flag» → фазы без переключателя.
- UI-scope (0-setup/ui-scope-prefs.md): `swagger:no` (Swagger regen — нет условной фазы, но если правится `swag`-аннотация существующего `POST/PUT /tasks` — regen в той же фазе, где меняется DTO, т.е. 02); `adaptive:min` + `themes:both` — гейт в фазе 06 (финальный E2E-прогон на 375px и обеих темах).
- Фикстуры E2E: `e2e/fixtures/board.ts` (pg → API, ADR-101) переводятся в фазе 03 (там же, где появляются API строк услуг/оборудования); `board.spec.ts`/`board-guards.spec.ts`/`stock.spec.ts` — регресс-гейт сразу после замены (в той же фазе 03), не откладывается на 06.
