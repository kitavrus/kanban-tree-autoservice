---
stage: 5-design-review
feature: phase-3-clients
tier: M
verdict: APPROVED
high: 0
medium: 0
low: 0
---
# Mobile API Review (5-design-review): phase-3-clients — APPROVED

**Артефакты, которые читал (повторное ревью после fix-loop #1):**
- `docs/features/phase-3-clients/3-design/output.md` §Контракт (таблица API + новый блок «Правила контракта», sed -n 34,76p) и ADR-007 (sed -n 127,135p)
- `docs/features/phase-3-clients/3-design/design-errors.md` (Sentinel Errors, Error Mapping — целиком)

**Tier:** M | **Совместимость:** MVP (CM-1) | **swagger:no** (US-1 — swagger.json-пункты n/a)

## Проверка находок предыдущего прогона (все — CONFIRMED FIXED)

#### [H-01] Дублирующийся код `EQUIPMENT_TYPE_NAME_TAKEN` на две причины — ИСПРАВЛЕНО
`design-errors.md`: `ErrTypeNameArchived = errorspkg.New("EQUIPMENT_TYPE_NAME_ARCHIVED", ...)` — отдельный код от `ErrTypeNameTaken` (`EQUIPMENT_TYPE_NAME_TAKEN`). `output.md` ADR-007: клиент строит CTA «Вернуть из архива» по `code`, путь восстановления явно описан (`GET /equipment-types?include_archived=true&q=` → найти id → `PUT .../:id` с `is_active:true`). Таблица `## Контракт` строка POST/PUT `/equipment-types` теперь перечисляет оба 409-кода отдельно.

#### [M-01] Асимметрия optimistic locking Client vs Equipment/Type — ИСПРАВЛЕНО
Новый блок «Правила контракта» в `output.md` §Контракт явно фиксирует: только `PUT /clients/:id` защищён `version`/409 `CLIENT_VERSION_CONFLICT`; equipment/equipment-types — осознанный LWW со ссылкой на AC-008/AC-011 (конфликт не требуется) и D28 (version только у клиентов и заказов). Больше не implicit-пропуск.

#### [M-02] Списочные эндпоинты без default/max `limit`/`offset` — ИСПРАВЛЕНО
Таблица в «Правила контракта»: `clients` 50/200, `clients/:id/equipment` 200/200, `equipment-types` 500/500 (lookup без пагинации по факту — весь список для селектора AC-008/AC-009, клиент сверяет `total ≤ limit`); мусор/≤0 → дефолт, `offset<0` → 0 (паттерн users). Риск усечения дропдауна типов снят.

#### [M-03] Смешение языка в `message` (`BAD_REQUEST`/"invalid body") — ИСПРАВЛЕНО
`design-errors.md` §Error Mapping: новые хендлеры фазы 3 → `message` «Некорректное тело запроса» (русский). Явно оговорено, что `users` as-is «invalid body» не переписывается молча (as-is, вне scope фичи) — не вводит в заблуждение про новые эндпоинты.

#### [L-01] Нет idempotency-key на create — ЗАКРЫТО (осознанное решение)
«Правила контракта»: idempotency-key не вводится — решение Tech Spec §Технические ограничения «Двойной submit» (disable кнопки + PRG на клиенте; серверного естественного ключа нет). Дубли типов и так отсекаются уникальностью имени (409). Зафиксировано явно, не молчаливый пропуск.

#### [L-02] Full-replace семантика PUT только в Go-комментарии — ИСПРАВЛЕНО
Таблица `## Контракт` теперь содержит инлайн-пометку «полная замена, LWW» / «полная замена вкл. `phones[]` (replace-all)» прямо в колонке Request для `PUT /clients/:id` и `PUT /equipment/:id`, плюс отдельный пункт «Семантика PUT» в блоке правил.

## Резюме

HIGH=0, MEDIUM=0, LOW=0 → **APPROVED**. Все находки design-review #1 закрыты явными, self-contained решениями в контракте (не переносом в код без документации). Замечаний к текущей редакции нет.
