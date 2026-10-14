---
stage: 5-design-review
feature: phase-5-board
tier: M
verdict: APPROVED
high: 0
medium: 0
low: 1
---
# Design Review: phase-5-board — APPROVED
**Дата:** 2026-09-26
**Ревьюер:** design-reviewer agent

## Блокеры (design НЕ готов к stage Plan)
Нет.

## Серьёзные замечания (исправить до мержа)
Нет.

## Незначительные замечания
1. **[5-design-review/input-bundle.md §Паспорт]** Паспорт не содержит индекс `Q-NNN` (только AC/DS/TS) — вопрос тулинга, по решению оркестратора идёт в ретро, не блокирует эту фичу.

## Повторное ревью — что проверено (итерация 1)
- **Swimlane (`4-swimlane/design-swimlane.html`) перегенерирован и подтверждён:** код `INCOMPLETE_SUBTASKS` заменён на `SUBTASKS_NOT_DONE` во всех 3 местах (flow AC-002/004/005, flow AC-003, «Sentinel-коды») с `details.subtasks[{id, service_name, status}]` — совпадает с `design-errors.md` (код #03) и `design-sql.md §Open Subtasks Query`. Поля FLOWS приведены к DTO `design-go-layers.md`: `title`/`id` (`TaskCard`), `assignee_id`, `client_name` (плоское), `prev_id`/`next_id` явно типизированы `int64`. Убраны фиктивные `is_default`, `can_move_to_final`, `DECIMAL(20,10)`, `CHECK stock≥−999.99`, `INSUFFICIENT_STOCK`. Новый код `PAID_ADMIN_ONLY` (403) добавлен в раздел ошибок и в лейн «Обработка ошибок» — соответствует `design-errors.md`.
- **`design-errors.md` (M-01):** код #11 переименован `FORBIDDEN`→`PAID_ADMIN_ONLY` с отдельным sentinel `ErrPaidAdminOnly`; код #12 `FORBIDDEN` сужен явно до «маршрут закрыт ролью»; добавлено пояснение различимости 403 по `code`. `design-go-layers.md §Service Interface` (`Update`) и `design-tests.md` (LV-042) синхронно используют `ErrPaidAdminOnly`/`PAID_ADMIN_ONLY` — расхождений не найдено.
- **`3-design/output.md §Контракт` (M-02):** добавлена явная строка с порядком сортировки и лимитами пагинации для всех 4 list-эндпоинтов (`columns`, `tasks`, `stock`, `assignees`) — совпадает 1:1 с `ORDER BY`/`LIMIT` в `design-sql.md` (Board List Query, Stock List Query, Assignees Query) и с комментариями в DTO `design-go-layers.md`. Коды DELETE задокументированы явно: `/columns/:id` → 204 без тела, `/tasks/:id` → 200 `{warnings}` — совпадает с таблицей контракта (строки DELETE) и `design-go-layers.md §Handler Columns/Tasks`.
- Point-check на `2-spec-interview/output.md §Открытые вопросы`: состав Q-001…Q-006 не менялся, покрытие ADR-001…006 подтверждено в предыдущей итерации, повторно не пересчитывалось (не затронуто правками).

## Покрытие открытых вопросов
| Q-NNN | Формулировка | ADR-NNN | Статус |
|---|---|---|---|
| Q-001 | Алгоритм дробной позиции карточки | ADR-001 | Принято |
| Q-002 | Формула прогноза «Остатков» и KPI | ADR-002 | Принято |
| Q-003 | Контракт фильтров доски | ADR-003 | Принято |
| Q-004 | Доставка счётчика «Остатков» в меню | ADR-004 | Принято |
| Q-005 | Размещение D26-проверок фазы 3 | ADR-005 | Принято |
| Q-006 | Формат `warnings` в ответах | ADR-006 | Принято |

## Полнота файлов (feature, tier M)
- [x] 3-design/output.md
- [ ] design-dataflow.md — n/a (diagrams-prefs: dataflow: no)
- [x] 3-design/design-sequence-diagrams.md
- [x] 4-swimlane/design-swimlane.html
- [x] 3-design/design-go-layers.md
- [x] 3-design/design-sql.md
- [x] 3-design/design-tests.md
- [ ] design-di.md — n/a для M
- [ ] design-integrations.md — n/a (Triage: external: false)
- [x] 3-design/design-errors.md
- [ ] design-infrastructure.md — n/a (Triage: infra: false)
- [ ] design-adr.md — ADR-секция инлайн в `3-design/output.md` (M-tier)
- [ ] 3-design/design-e2e-strategy.md — n/a (SSR, не Vue)

## Консистентность
- [x] DTO в go-layers ↔ sequence-diagrams — ОК
- [x] SQL columns ↔ migrations — ОК; `stock_movements.task_id uuid→bigint USING NULL` применим (baseline подтверждён напрямую в `migrations/0004_catalog.up.sql`)
- [x] Errors ↔ handlers ↔ tests — ОК, включая новый `PAID_ADMIN_ONLY` (design-errors ↔ go-layers §Service Interface ↔ design-tests LV-042)
- [x] Swimlane роли ↔ слои go-layers — ОК (устранены расхождения по кодам ошибок и полям DTO)
- [x] Reconcile-инвариант (D17/D34) — формулы дельт/qty_delta телескопируются корректно, покрыт IT-010/assertStockInvariant
- [x] AC-001…AC-020 — 1:1 покрытие E2E (BD-001…018, BD-020) + IT-001 (AC-019)
- [x] Контракт: сортировка/пагинация/DELETE-коды задокументированы и совпадают с SQL/DTO
- [x] Отклонения от spec (400 для ссылок из тела вместо 404; `SERVICE_IN_USE` в catalog) — обоснованы, спеке не противоречат

## Итог
APPROVED. Все замечания из итерации 0 устранены: swimlane перегенерирован без выдуманных кодов/полей, добавлен корректный код `PAID_ADMIN_ONLY`, контракт дополнен порядком сортировки, дефолтами пагинации и явными кодами DELETE. Design-артефакты внутренне консистентны, миграция 0005 применима, инвариант склада формально проверен, AC-001…AC-020 покрыты тестами, Q-001…Q-006 закрыты ADR с цитатами Driver. Дизайн готов к стадии Plan.
