---
stage: 6-plan
feature: phase-5-board
tier: M
---
# Code Plan: phase-5-board — доска заказов и «Остатки»
**Дата:** 2026-09-26
**На основе:** 3-design/output.md, design-sql.md, design-errors.md, design-go-layers.md, design-tests.md, design-sequence-diagrams.md
**Совместимость:** CM-1 MVP — без dual-write/dual-read, down-миграция = прямой DROP.
**Трек:** обычный (draft-spec MISSING в бандле — DS/TS-индексы не применяются).
**Статусы:** [6-plan/code-plan-status.md](code-plan-status.md) — единственный source of truth.

## Обзор фаз
| # | Фаза | Layer | Файл | AC |
|---|------|-------|------|----|
| 01 | Foundation: миграция 0005, pkg/numeric, errorspkg.WithDetail, D26/SERVICE_IN_USE гарды | backend | [phase-01](code-plan-phase-01-foundation.md) | AC-008, AC-017, AC-018 |
| 02 | Board: колонки + StockEngine.Reconcile | backend | [phase-02](code-plan-phase-02-board-columns-stock-engine.md) | AC-006, AC-007 (ядро) |
| 03 | Board: заказы + move + live-сьюта | backend | [phase-03](code-plan-phase-03-board-tasks-move.md) | AC-002…AC-014 (backend) |
| 04 | Stock API + бейдж меню + DI (роутер, удаление home) | backend | [phase-04](code-plan-phase-04-stock-api-badge-di.md) | AC-015, AC-016 |
| 05 | `make seed-demo` (`internal/seeddemo`) | backend | [phase-05](code-plan-phase-05-seed-demo.md) | AC-019 |
| 06 | UI доски (board.js DnD, формы) + E2E доски | frontend | [phase-06](code-plan-phase-06-ui-board.md) | AC-001…AC-014 (UI) |
| 07 | UI «Остатки» + E2E | frontend | [phase-07](code-plan-phase-07-ui-stock.md) | AC-015, AC-016 (UI) |
| 08 | D26 E2E-гарды + 360px/тёмная тема полировка | frontend | [phase-08](code-plan-phase-08-guards-polish.md) | AC-017, AC-018, AC-020 |

## Зависимости между фазами
- 01 → 02 (таблицы `columns`/`tasks`/... нужны Reconcile) → 03 (нужен StockEngine + columns_repo) → 04 (нужны tasks+stock для DI/роутера, home удаляется здесь — конфликт пути `/`) → 05 (нужен `board.StockEngine` экспортирован в 02/03).
- 06 зависит от 04 (SSR-стабы `/`, `/stock` регистрированы, testid-контракт из design-go-layers) и от 03 (API заказов/move).
- 07 зависит от 04 (Stock API) и 06 (общий `app.css`/nav-badge.js).
- 08 зависит от 01 (гарды), 06/07 (UI + `e2e/fixtures/board.ts` из ADR-103).

## Риски первыми
Миграция `task_id uuid→bigint` (0005) и Reconcile под блокировками (ADR-104) — в фазах 01–03, до UI.

## Общие правила
- Каждая фаза = один атомарный коммит; `make build` проходит после каждой.
- Тесты — в той же фазе, что код (TD-1, fix-before-next).
- `//go:build integration` для integration-тестов; `testutil.LiveSuite` для HTTP live (TD-8) — `app.Test` запрещён кроме `FuzzMoveBody` (§9.1).
- UUID — только `pkg/ids`? — В проекте `gen_random_uuid()` в БД, Go id не генерирует (см. research-паттерны) — новых UUID-либ не вводить.
- `pkg/numeric.Amount` (фаза 01) — ЕДИНСТВЕННЫЙ тип для NUMERIC-полей board (stock/min_stock/pending/forecast/qty в warnings); прямой `float64`/сырая строка в новых DTO — запрещены.
- `errorspkg.AppError.WithDetail` (фаза 01) — ЕДИНСТВЕННЫЙ способ приложить `details.subtasks[]` к `ErrSubtasksNotDone`; отдельный ad-hoc тип ответа для move — запрещён.
- Фронтенд — SSR html/template + vanilla JS (Bootstrap 5.3), Vue/Element Plus НЕ применимо (D30/D38) — DoD-пункт «Frontend-стек Element Plus» из шаблона неприменим к этому проекту.
- Фаза, регистрирующая/меняющая handler или router (03, 04) — DoD включает `board_http_live_test.go`/`stock_http_live_test.go` (`testutil.LiveSuite`, TD-8).
- Fuzz (TD-7, класс 9 — парсинг query/JSON от клиента): backend fuzz-таргеты — фазы 02/03 (design-tests §Fuzz Targets). pw-fuzz: явного вердикта `**pw-fuzz:**` в дизайне нет → класс 9.2: n/a (design).
- Условные фазы 11.5 (Observability)/11.6 (Runbook/Perf docs) — НЕ создаются: `## Monitoring & Alerting Spec` и `## Runbook`/`## Performance Budget` в design помечены `deferred` (прод-мониторинг не развёрнут, CM-1 MVP).
- `## Rollout Strategy` в design — «Не применимо» → фазы без флага-переключателя фичи.
