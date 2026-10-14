---
stage: 5-design-review
feature: phase-6-order-card
tier: M
verdict: APPROVED
high: 0
medium: 0
low: 2
---
# Design Review: phase-6-order-card — APPROVED
**Дата:** 2026-09-27 (итерация 2)
**Ревьюер:** design-reviewer agent

## Блокеры (design НЕ готов к stage Plan)
Нет. Все 3 блокера итерации 1 закрыты, проверено поимённо против owner-файлов:

1. **`4-swimlane/design-swimlane.html:350`** — `SubtaskCreateRequest` теперь `(service_id)` — совпадает с `design-go-layers.md:131` (`type SubtaskCreateRequest struct { ServiceID string }`).
2. **`design-swimlane.html:293`** — состав `GET /tasks/:id/card` теперь `task, column, equipment[], subtasks[], stock[]` — совпадает с `OrderCardResponse` (`design-go-layers.md:51-57`); лишнего `warnings`/`task_equipment` нет.
3. **`design-swimlane.html:333`** — `EquipmentRequest (type_id, brand, model, sku, serial, specs, note)` — сверено с `internal/features/clients/dto.go:85-93`, совпадает поле-в-поле (включая `serial`, не `serial_number`).

## Серьёзные замечания (исправить до мержа)
Нет. Оба замечания итерации 1 закрыты:

1. **ADR-102 (акт выдачи, AC-016)** — решение подтверждено пользователем: `2-spec-interview/spec-decisions.md §SD-009` фиксирует «все услуги со статусом, итог = итог карточки». `design-errors.md`/`design-go-layers.md`/swimlane ссылаются на SD-009 согласованно.
2. **ADR-103 (backfill `position`)** — добавлен явный backfill `UPDATE ... row_number() OVER (PARTITION BY subtask_id ORDER BY product_id, id)` в `0006_subtask_logs.up.sql` (`design-sql.md §Subtask Logs Migration`) + интеграционный тест `IT-020` (миграция на БД со старыми `subtask_products` → `position` 1..N без нулей). Риск снят.

## Незначительные замечания
1. **`design-sequence-diagrams.md`** — не обновлён под ADR-105 (клиентская `rowQueue`); диаграмма 2 показывает только серверную часть потока, что формально не противоречит новому клиентскому поведению (документ и раньше не детализировал JS), но явного упоминания одной PUT-в-полёте на строку там нет. Не блокирует Plan — детали клиентской очереди полностью описаны в `design-go-layers.md §Frontend Modules` и ADR-105.
2. **`design-errors.md:10`** — Driver кода `SUBTASK_NOT_FOUND` (16) теперь `AC-007, AC-014` — точнее прежнего (было только AC-014), замечание итерации 1 закрыто.

## Новое: ADR-105 (клиентская очередь запросов на строку)
Добавлен по итогам mobile-review (H-01/M-03: self-race при автосохранении — два PUT одной строки в полёте могли молча откатить поле). Проверено:
- **Корректность решения:** `rowQueue` — не более одного запроса на строку в полёте, накопленные правки уходят одним следующим PUT, тело собирается из последнего `card` ⊕ `pending` (не из DOM на момент `change`) — устраняет race без нового серверного контракта (`version` для строк сознательно не вводится, см. ADR-002).
- **Согласованность с ADR-002/ADR-100:** очередь — чисто клиентский механизм, не меняет LWW-семантику строк и не требует изменений в Lock Order; разные строки продолжают уходить параллельно (межпользовательский LWW, AC-014, что и раньше).
- **Тесты:** `OC-022` (услуга: статус→срок→цена при задержке первого PUT → все три значения сохранены), `OC-023` (товар: qty 1→2→3 → финальный qty=3, остатки согласованы) — обе в `design-tests.md §E2E` и `§Coverage AC` (AC-007, AC-010, AC-014).
- Driver и альтернативы (`client_seq`-guard, PATCH по полю) рассмотрены и обоснованно отклонены в самом ADR-105.

## Покрытие открытых вопросов
| Q-NNN | Формулировка | ADR-NNN | Статус |
|---|---|---|---|
| Q-001 | Пути create и состав ответа мутации | ADR-001 | Принято |
| Q-002 | Мутации строк не инкрементируют `tasks.version`/`updated_at`? | ADR-002 | Принято |
| Q-003 | Sentinel-ы фазы 6 | ADR-003 | Принято |
| Q-004 | Новое оборудование из карточки — 2 запроса или составной эндпоинт | ADR-004 | Принято |
| Q-005 | SQL для итогов/last_log, индексы | ADR-005 | Принято |
| Q-006 | Редактор описания и санитизация | ADR-006 | Принято |
| Q-007 | `subtask_logs.user_id` — RESTRICT или SET NULL | ADR-007 | Принято |

Без изменений с итерации 1 — все 7 закрыты 1:1. ADR-100…105 — вне диапазона Q-NNN, дополнительные архитектурные решения (в т.ч. ADR-105 — реакция на mobile-review), это нормально.

## Полнота файлов (feature, tier M)
- [x] 3-design/output.md (Контракт + Enums + ADR-001…007, ADR-100…105 инлайн — допустимо для M)
- [ ] design-dataflow.md — n/a (`diagrams.dataflow: no`)
- [x] 3-design/design-sequence-diagrams.md
- [x] 4-swimlane/design-swimlane.html — валиден, DTO сверены поимённо с owner-файлами
- [x] 3-design/design-go-layers.md
- [x] 3-design/design-sql.md
- [x] 3-design/design-tests.md
- [ ] design-di.md / design-integrations.md / design-infrastructure.md / design-adr.md — n/a для M (инлайн в output.md/go-layers)
- [x] 3-design/design-errors.md
- [ ] 3-design/design-e2e-strategy.md — n/a (SSR + vanilla JS, не Vue)

## Консистентность
- [x] DTO в go-layers ↔ sequence-diagrams — ОК
- [x] DTO в go-layers ↔ swimlane — ОК (устранены все 3 расхождения итерации 1, сверено поимённо, включая as-is `clients/dto.go`)
- [x] SQL columns (`subtask_products.position` + backfill, `subtask_logs.*`) ↔ миграция 0006 — ОК
- [x] Errors (Codes 16-22 + Reused) ↔ Error Mapping ↔ design-tests (LV-140…182, IT-010…013/020) — ОК
- [x] Lock Order (ADR-100) единообразен в design-sql/go-layers/swimlane; `errTaskMovedRetry` в общем ретрае `withTx`, дедлок-цикла нет
- [x] Reconcile на всех мутациях состава товаров в списывающей колонке; инвариант склада — IT-001
- [x] Итог заказа именован единообразно `total_amount` (`task.total_amount`) во всех файлах — output.md, design-sql.md, design-go-layers.md, design-tests.md, swimlane; отдельного поля `total`/`card.total` нигде не осталось
- [x] ADR-105 (клиентская очередь) согласован с ADR-002/ADR-100, тесты OC-022/023 добавлены

## Итог
APPROVED. Все 3 блокера и оба серьёзных замечания итерации 1 закрыты и точечно перепроверены против owner-файлов (`design-go-layers.md`, `design-sql.md`, `design-errors.md`) и as-is кода (`internal/features/clients/dto.go`). Дополнительно принят и корректно интегрирован ADR-105 (клиентская `rowQueue` на self-race автосохранения, найденный mobile-review) с тестовым покрытием OC-022/023. Открытые вопросы Q-001…Q-007 закрыты 1:1, AC-001…AC-017 покрыты тестами (включая новые IT-020, OC-022/023). Design готов к стадии Plan.
