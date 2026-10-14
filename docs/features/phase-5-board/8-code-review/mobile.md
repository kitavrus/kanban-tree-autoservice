---
stage: 8-code-review
feature: phase-5-board
tier: M
verdict: APPROVED
high: 0
medium: 1
low: 1
---
# Mobile API Review: phase-5-board — APPROVED

**Артефакты, которые читал:**
- `docs/features/phase-5-board/8-code-review/input-bundle.md`
- `docs/features/phase-5-board/8-code-review/diff-backend.patch` (DTO/handler/service/repo: columns, tasks, stock, assignees, errorspkg, numeric.Amount, clients/catalog D26-гарды)
- `docs/features/phase-5-board/8-code-review/diff-other.patch` (`web/ssr/_shared/js/board.js`, `stock.js`, `api.js`)
- `docs/features/phase-5-board/3-design/output.md` §Контракт, §Enums; `design-go-layers.md` §DTO *; `design-errors.md` §Codes 01-15, §Validation Fields
- `docs/features/phase-5-board/5-design-review/mobile.md` (design-стадия: APPROVED, high=0, medium=2, low=3) — сверка, что M-01/M-02/L-01..03 закрыты в коде

**Tier:** M

**swagger:no** — пункты по swagger.json/`make swagger`/`$ref` n/a; клиент — собственный vanilla JS того же приложения (`board.js`, `stock.js`).

## Сверка design-review findings → код

- **M-01 (design)** «один `FORBIDDEN` на AC-006/012/015 и AC-011» — закрыт: реализован отдельный код `PAID_ADMIN_ONLY` (`errors.go`, `ErrPaidAdminOnly`), `board.js` различает `err.code === 'TASK_VERSION_CONFLICT'` и `'SUBTASKS_NOT_DONE'` по коду; `FORBIDDEN` остаётся только для роут-гарда.
- **M-02 (design)** порядок `items` не был зафиксирован в контракте — закрыт: `output.md` §Контракт теперь явно фиксирует `columns.position, columns.id, tasks.position, tasks.id`.
- **L-01/L-02 (design)** дефолты пагинации `/columns`, `/assignees` не были зафиксированы — закрыты: `output.md` §Контракт даёт `limit=100`/`limit=200` явно.
- **L-03 (design)** 204 vs 200+body на DELETE — намеренно, реализовано как задокументировано (`columns_handler.go` → `SendStatus(204)`; `tasks_handler.go` → `c.JSON(resp)` 200).

## Findings

### MEDIUM (advisory)

#### [M-01] Валидация `limit`/`offset` непоследовательна между списочными эндпоинтами одной фичи
**Класс:** Error handling / Контракт
**Локация:** `internal/features/board/columns_handler.go` (`List`, `strconv.Atoi(c.Query("limit"))` — ошибка отброшена `_`); `internal/features/board/assignees_handler.go` (`List`, `if err != nil || limit <= 0 { limit = default }`) — против `internal/features/board/tasks_filter.go` и `internal/features/board/stock_handler.go`, где тот же `limit`/`offset` при мусоре/выходе за диапазон возвращает `400 VALIDATION` с `details.fields:[{name:"limit"}]`.
**Цитата:** `columns_handler.go`: `limit, _ := strconv.Atoi(c.Query("limit")); offset, _ := strconv.Atoi(c.Query("offset"))` — против `tasks_filter.go`: `if err != nil || n < 1 || n > tasksMaxLimit { add("limit") }`.
**Проблема:** `design-errors.md` §Validation Fields фиксирует единое правило `query limit/offset: int, limit 1..1000, offset ≥0` без пометки «кроме columns/assignees». На практике `GET /api/v1/columns?limit=abc` или `?limit=-5` НЕ возвращает 400 — тихо подставляется дефолт (100/200) и клиент получает `200` с неожиданным `limit` в ответе вместо ошибки поля, тогда как та же опечатка на `GET /api/v1/tasks`/`GET /api/v1/stock` даёт `400 {code:"VALIDATION", details:{fields:[{name:"limit"}]}}`. Клиентский код (`board.js`), который единообразно обрабатывает `err.code` на всех списках, не получит сигнала об ошибке ввода на этих двух эндпоинтах — баг в UI (например, опечатка в query, собранной вручную) будет замаскирован тихим fallback вместо видимой 400.
**Рекомендация:** привести `columns_handler.go`/`assignees_handler.go` к тому же паттерну явной 400-валидации `limit`/`offset`, что и `tasks_filter.go`/`stock_handler.go`, либо явно задокументировать в `design-errors.md` §Validation Fields, что `limit`/`offset` на `/columns` и `/assignees` — permissive-clamp (as-is отклонение от общего правила таблицы).

### LOW (nice-to-have)

#### [L-01] `TaskCard.Position` — NUMERIC строкой, а не `numeric.Amount`
**Класс:** Контракт
**Локация:** `internal/features/board/tasks_dto.go` → `TaskCard.Position string` (комментарий: «NUMERIC строкой — клиенту только для отладки/сортировки», отдельно от `numeric.Amount`, используемого в `StockRow`/`StockWarning`).
**Проблема:** тот же приём (NUMERIC-хранение как строка) реализован ДВУМЯ разными Go-типами в одном пакете (`string` для позиции задачи vs `numeric.Amount` для остатков) — оба сериализуются по-разному: `Position` останется JSON-строкой (`"1024.500"`), `Stock`/`MinStock` — JSON-числом без кавычек. Разработчику клиента, незнакомому с намеренным разделением (комментарий в коде это объясняет — «не Amount, это не денежная величина»), легко перепутать при добавлении нового NUMERIC-поля в будущем.
**Рекомендация:** решение обоснованное и задокументированное инлайн-комментарием — оставить как есть; для будущих полей стоит явно фиксировать в `design-go-layers.md` §DTO правило «NUMERIC как строка (сортировка/отладка) → `string`, NUMERIC как величина (клиент отображает/считает) → `numeric.Amount`».

## Резюме

HIGH=0 → **APPROVED**. Основные точки контракта (nullable, `numeric.Amount` как JSON-число, коды ошибок `PAID_ADMIN_ONLY`/`FORBIDDEN`/`SUBTASKS_NOT_DONE`+`details.subtasks[]`, порядок сортировки и дефолты пагинации, 204 vs 200 на DELETE, N+1 — все списки одним JOIN-запросом без построчных подзапросов) реализованы 1:1 с design-контрактом и design-review findings закрыты. Единственная новая находка (M-01) — расхождение строгости валидации `limit`/`offset` между эндпоинтами одной фичи — advisory, не блокирует.
