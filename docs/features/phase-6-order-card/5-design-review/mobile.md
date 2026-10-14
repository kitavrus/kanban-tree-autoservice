---
stage: 5-design-review
feature: phase-6-order-card
tier: M
verdict: APPROVED
high: 0
medium: 0
low: 0
---
# Mobile API Review: phase-6-order-card — APPROVED

**Артефакты, которые читал (итерация 2 — точечно, по местам правок):**
- `docs/features/phase-6-order-card/3-design/output.md` — ADR-001 (исключение оборудования), ADR-105 (новый)
- `docs/features/phase-6-order-card/3-design/design-go-layers.md` — `§DTO OrderCard` (`CardSubtaskProduct.Position`, `EquipmentHistory.CreatedAt`, комментарий об унификации `total_amount`)

**Tier:** M. `swagger:no` — swagger-пункты n/a.

## Проверка фиксов (итерация 1 → 2)

| # | Было | Стало | Статус |
|---|---|---|---|
| H-01 | full-replace PUT без version + автосохранение «без кнопки» → self-race, тихий откат поля | ADR-105: клиентская `rowQueue` в `task-card.js` — не более 1 запроса на строку в полёте, правки во время полёта копятся в `pending`, тело PUT собирается в момент отправки из последнего `card` ⊕ `pending` (не из DOM/кэша на `change`); межпользовательский LWW — как и было, ADR-002/AC-014. Тесты OC-022/023. | **Устранено** |
| M-01 | оборудование ломает паттерн «create nested / edit-delete flat» | ADR-001 явно фиксирует исключение: `у task_equipment нет собственного id (PK task_id+equipment_id)` — причина документирована | **Устранено** |
| M-02 | `total` (карточка) vs `total_amount` (список) — разные имена одного значения | `OrderCardResponse.Total` удалено; комментарий в DTO: «Итог заказа — только `task.total_amount` (одно имя в `GET /tasks` и в карточке; отдельного `total` нет)» | **Устранено** |
| M-03 | партиал-PATCH нет, риск потери полей при устаревшем кэше | закрыто тем же ADR-105 — тело всегда собирается из последнего применённого `card`, а не из промежуточного локального состояния | **Устранено** |
| L-01 | `subtask_products.position` (ADR-103) не отдавался в DTO | `CardSubtaskProduct.Position int json:"position"` добавлено, комментарий «массив уже отсортирован» | **Устранено** |
| L-02 | `EquipmentHistory` без даты заказа (AC-006) | `EquipmentHistory.CreatedAt time.Time json:"created_at"` добавлено, комментарий «дата заказа для «Уже был в ремонте»» | **Устранено** |

Побочных регрессий от правок не обнаружено: `CardSubtask.Total` (`json:"total"`) — отдельная сущность (итог строки услуги, не заказа), с `total_amount` не конфликтует, переименования не требовало.

## Findings

Нет открытых находок (HIGH/MEDIUM/LOW) — все 6 пунктов итерации 1 подтверждены закрытыми по факту чтения кода дизайна, не только по тексту ADR.

## Резюме

HIGH=0 → **APPROVED**. Breaking changes отсутствуют (аддитивно к фазе 5). §API-1 соблюдён. `swagger:no` — swagger-пункты не проверялись (n/a).
