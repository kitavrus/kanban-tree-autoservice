# API delta — pagination

Аддитивная дельта к контракту фазы 5 (`docs/features/phase-5-board/3-design/design-sql.md`
§Board List Query, `design-go-layers.md` §DTO Task) — чужие фазы не правятся, новый
контракт фиксируется здесь. Swagger-аннотаций в проекте нет (`make swagger` не
используется хендлерами) — источник правды для wire-формата: этот файл +
`internal/features/board/tasks_dto.go`.

Совместимость (§CM-1): только новые опциональные query-параметры и новое поле ответа;
конверт `{items,total,limit,offset}` (CLAUDE.md §API-1) без новых ключей. Лимиты/дефолты
не меняются (DS-008): `limit` 500 (1..1000), `offset` ≥ 0.

## GET /api/v1/tasks

### Новые query-параметры (DS-002)

| Параметр | Тип | Обяз. | Семантика | Ошибка |
|---|---|---|---|---|
| `column_id` | uuid | нет | только заказы этой колонки | невалидный uuid → 400 `VALIDATION`, `details.fields[].name = "column_id"` |
| `per_column` | int 1..1000 | нет | окно: не более N первых карточек каждой колонки по `(tasks.position, tasks.id)` (`row_number() OVER (PARTITION BY column_id …)`) | 0 / >1000 / не число → 400 `VALIDATION`, поле `per_column` |

`limit`/`offset` применяются ПОСЛЕ окна `per_column`. Порядок `items` — прежний контракт:
`columns.position, columns.id, tasks.position, tasks.id`.

### Новое поле карточки

| Поле | Тип | Семантика |
|---|---|---|
| `column_total` | int | число заказов ЭТОЙ колонки, подходящих под фильтры (`client_id`/`q`/`category_id`/`paid`/`assignee`/`due`/`column_id`), без учёта окна `per_column` и `limit/offset`. Всегда присутствует в элементах списка. |

`TaskResponse` (GET /tasks/:id, mutation-ответы) встраивает `TaskCard`, поэтому ключ
`column_total` там тоже есть, но всегда `0` — не вычисляется, клиент его там не читает.

### Семантика `total`

`total` — число заказов под фильтрами (с `column_id`, если задан; БЕЗ окна `per_column`).
Как и раньше, при `offset` за пределами выдачи (`items` пуст) `total = 0`
(`count(*) OVER ()` — поведение не менялось).

### Примеры

Первичная загрузка доски (DS-001): не более 50 карточек на колонку одним запросом.

```
GET /api/v1/tasks?assignee=all&per_column=50&limit=1000
200
{
  "items": [
    {"id": 101, "column_id": "6f1c…", "title": "…", "position": "1024", "column_total": 120, …},
    … (50 карточек колонки 6f1c…, у каждой column_total=120)
    {"id": 7, "column_id": "a2d9…", "column_total": 3, …}
  ],
  "total": 123,
  "limit": 1000,
  "offset": 0
}
```

«Показать ещё» по колонке:

```
GET /api/v1/tasks?assignee=all&column_id=6f1c…&limit=50&offset=50
200
{"items": [ … 50 карточек, column_total=120 … ], "total": 120, "limit": 50, "offset": 50}
```

Ошибка:

```
GET /api/v1/tasks?per_column=0&column_id=bad
400
{"code": "VALIDATION", "message": "validation failed",
 "details": {"fields": [{"name": "column_id", "message": "invalid"}, {"name": "per_column", "message": "invalid"}]}}
```

## SSR (не API, для полноты)

- `/catalog/products`, `/catalog/services` — параметр URL `offset` (кратен 100; невалидный
  или отрицательный → 0, некратный — вниз до начала страницы), страница по 100 позиций,
  пейджер `catalog-pager` / `catalog-shown` / `catalog-prev` / `catalog-next` (DS-005).
- Панель категорий и select'ы категорий в формах товара/услуги — пометка
  `categories-truncated` «показаны N из M», если категорий больше лимита 2000 (DS-006).

## Известное ограничение и клиентская компенсация (ревью MEDIUM-2)

`limit` применяется ПОСЛЕ окна `per_column`: если сумма `min(per_column, заказов колонки)`
по всем колонкам больше `limit` (1000), хвостовые (правые) колонки в первичной выдаче не
получают ни одной карточки и не имеют `column_total`. Серверный контракт не меняется.
Клиент (`board.js`) при `items.length < total` помечает такие колонки (нет записи
`column_total`) кнопкой «Показать заказы» (`data-testid="column-more-<id>"`), которая
делает обычную догрузку `column_id=<id>&limit=50&offset=0` — её ответ приносит
`column_total`, дальше работает «Показать ещё». Тулбар показывает «Заказов: M · показаны N».
