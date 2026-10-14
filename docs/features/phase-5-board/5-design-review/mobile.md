---
stage: 5-design-review
feature: phase-5-board
tier: M
verdict: APPROVED
high: 0
medium: 2
low: 3
---
# Mobile API Review: phase-5-board — APPROVED

**Артефакты, которые читал:**
- `docs/features/phase-5-board/5-design-review/input-bundle.md` (AC-001…AC-020, Triage, паспорт)
- `docs/features/phase-5-board/3-design/output.md` §Контракт, §Impact, §Enums и константы, §ADR-001…003
- `docs/features/phase-5-board/3-design/design-errors.md` (целиком — owner-файл кодов ошибок)
- `docs/features/phase-5-board/3-design/design-go-layers.md` §DTO Column/Task/TaskFilter/StockWarning/Stock/Assignee
- `docs/features/phase-5-board/3-design/design-sql.md` (точечно, строка 281 — ORDER BY доски)

**Tier:** M — все 4 класса чеклиста, стандартная глубина.

**Контекст клиента:** `swagger:no` — swagger.json/baseline как артефакты не существуют, все swagger-специфичные пункты (аннотации, `$ref`, codegen) — n/a. Клиент — собственный vanilla JS (`board.js`/`stock.js`) того же приложения, без строгой типизации/decoding (Codable-класс проблем неприменим). API-1 (kebab URL, snake JSON, `{items,total,limit,offset}`) соблюдён по всей таблице контракта — нарушений не найдено.

## Findings

### MEDIUM (advisory)

#### [M-01] Один код `FORBIDDEN` на две разные причины отказа
**Класс:** Error handling
**Локация:** `design-errors.md` → `## Codes 01-15`, строки 11 и 12
**Цитата:** `11 | FORBIDDEN | 403 | errorspkg.Forbidden("Оплату меняет только администратор") | — | AC-011` и `12 | FORBIDDEN | 403 | middleware.RequireAdmin(nil) | «Недостаточно прав» | AC-006, AC-012, AC-015`.
**Проблема:** `board.js` не сможет различить по `code` «попытка сменить оплату не-админом» (AC-011, ожидаемо для non-admin UI, нужна точечная реакция — например, подсветить чекбокс оплаты) и «нет прав на действие вообще» (удаление колонки/заказа, `/stock` PUT) — только по тексту `message`, который не предназначен как программный маркер (owner-комментарий design-errors.md:1: «строковые коды … не числовые supportMessage», подразумевает код — стабильный якорь смысла).
**Рекомендация:** завести отдельный код, например `PAID_ADMIN_ONLY`, для AC-011 (аналогично отдельным кодам 07/08/09 для разных 409-причин), либо явно зафиксировать в контракте, что клиент не должен различать 403 по коду (полагаться только на `message`).

#### [M-02] Гарантия сортировки списка задач не зафиксирована в контракте
**Класс:** Контракт / UX
**Локация:** `output.md` §Контракт (`GET /api/v1/tasks`) и `design-go-layers.md` §DTO Task (`TaskListResponse`) — нет пометки о порядке; фактическая сортировка — только `design-sql.md:281`: `ORDER BY c.position, c.id, t.position, t.id`.
**Проблема:** `board.js` рендерит колонки и карточки в порядке, в котором пришли `items` (ADR-003: доска = один `GET /tasks`, без клиентской пересортировки согласно тексту ADR). Если это неявное допущение не закреплено как контракт-гарантия в DTO/§Контракт, будущая правка SQL/пагинации (например, добавление `limit/offset`-курсора) может тихо сломать порядок карточек без затрагивания самого DTO — reviewer.md/design-review это не поймает, т.к. смотрит на структуру полей, не на ORDER BY.
**Рекомендация:** добавить в `design-go-layers.md` §DTO Task или в `output.md` §Контракт явную строку: «`items` отсортированы `(column_position, column_id, task_position, task_id)`, клиент не пересортировывает» — с якорем на `design-sql.md §Board Query`.

### LOW (nice-to-have)

#### [L-01] `GET /api/v1/columns` не документирует `limit/offset`, но `ColumnListResponse` несёт эти поля
**Локация:** `output.md` §Контракт (строка с `GET /api/v1/columns`) vs `design-go-layers.md` §DTO Column (`ColumnListResponse { Items …; Total, Limit, Offset int }`).
**Проблема:** по API-1 конверт обязателен для любого списка, но раз query-параметров пагинации нет, неясно, какими значениями будут `limit`/`offset` в ответе (весь список / `len(items)` / 0) — `board.js` не должен опираться на них для доски, но это неочевидно без пометки.
**Рекомендация:** одна строка в §DTO Column: «пагинация не применяется, `limit=total`, `offset=0`».

#### [L-02] Дефолт пагинации `GET /api/v1/assignees` не зафиксирован
**Локация:** `output.md` §Контракт (`GET /api/v1/assignees | … | limit,offset`) vs `design-sql.md:382` (параметризованный запрос без явных дефолтов в контракте).
**Проблема:** в отличие от `TaskFilter` (`Limit default 500, 1..1000` — явно в DTO), для списка исполнителей (используется в выпадающем списке назначения задачи) дефолт `limit` нигде не зафиксирован в контракте/DTO — при росте штата это может незаметно обрезать список в дропдауне.
**Рекомендация:** добавить дефолт (например, 200) в §DTO Assignee рядом с `AssigneeListResponse`.

#### [L-03] Разный код успеха на DELETE между ресурсами (204 vs 200+body)
**Локация:** `output.md` §Контракт: `DELETE /api/v1/columns/:id → 204`, `DELETE /api/v1/tasks/:id → 200 TaskDeleteResponse`.
**Проблема:** оправдано (у задачи есть `warnings` от возврата остатков на склад, у колонки — нет), но `board.js` должен отдельно обрабатывать 204-без-тела и 200-с-телом для двух похожих операций удаления — не баг, но стоит явно закомментировать в клиентском коде на этапе реализации, чтобы не унифицировали обработчик по ошибке.

## Резюме

HIGH = 0 → **APPROVED**. Контракт соответствует API-1 (kebab URL, snake JSON, list-конверт) без нарушений; nullable-поля (assignee/due_date/paid_at) консистентны между DTO; `warnings`-массивы явно зафиксированы как `не nil: []`; N+1 предотвращён архитектурно (ADR-003 — один `GET /tasks` с денормализованными `client_name`/`assignee_name`, без дополнительных запросов на карточку). Найденные MEDIUM/LOW — точечные документационные дыры контракта (дублирующийся код 403, неявная сортировка, недокументированные дефолты пагинации), не блокируют переход к реализации; рекомендуется закрыть на этапе `design-go-layers.md`/`design-errors.md` правкой в 1-2 строки каждая.
