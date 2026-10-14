# Хронометраж: phase-1-skeleton

Обновлено: 2026-09-25T13:19:46+05:00

## Стадии

| Стадия | Начало | Конец | Длит. | Ожидание | Чистое | Рестарты | Переделки |
|---|---|---|---|---|---|---|---|
| light-exec | 25.09 12:49 | 25.09 13:09 | 20м 28с | 3м 14с | 17м 14с | — | — |
| light-judge | 25.09 13:09 | 25.09 13:11 | 2м 03с | — | 2м 03с | — | — |
| light-review | 25.09 13:11 | 25.09 13:19 | 7м 54с | — | 7м 54с | — | — |
| **Итого (union)** | 25.09 12:49 | 25.09 13:19 | **30м 25с** | **3м 14с** | **27м 11с** |  | **0с** |

Wall-clock (первое→последнее событие журнала): **30м 25с**; застаймлено (union стадий): 30м 25с.

## События (fix-loop / повторные гейты / ошибки)

Событий не было.

## Инструменты (тайминги гейтов/чекеров)

| Время | Инструмент | Статус | Итого | Шаги (по убыванию) |
|---|---|---|---|---|
| 25.09 13:09 | e2e_evidence | PASS | 0с | status:passed 0s |
| 25.09 13:10 | review_bundle | PASS | 0с | contract 0.1s · date_bound 0.1s · live_suite 0.1s · fixture_sync 0.1s · go_naming 0.0s · +4 шагов (0.0s) |
| 25.09 13:10 | review_bundle | PASS | 0с | date_bound 0.1s · live_suite 0.1s · fixture_sync 0.1s · go_naming 0.0s · api_conventions 0.0s · +4 шагов (0.0s) |
| 25.09 13:11 | review_bundle | PASS | 0с | contract 0.1s · date_bound 0.1s · live_suite 0.1s · fixture_sync 0.1s · go_naming 0.0s · +4 шагов (0.0s) |
| 25.09 13:18 | e2e_evidence | PASS | 0с | status:passed 0s |
| 25.09 13:18 | review_bundle | PASS | 0с | contract 0.1s · date_bound 0.1s · live_suite 0.1s · fixture_sync 0.1s · rule_usage 0.1s · +4 шагов (0.0s) |

## ⚠ BLIND-GAP — интервалы без записей журнала (>5 мин)

Тяжёлый прогон обязан обрамляться `event-start`/`event-end` (§EE anti-polling):
без них нельзя отличить фоновый прогон от sleep-поллинга.

| Стадия | Начало | Конец | Длит. |
|---|---|---|---|
| light-review | 25.09 13:11 | 25.09 13:18 | 6м 31с |

Суммарно вне наблюдения: **6м 31с** (1 интервал(ов)).

## Пайп-передачи (§3.6: бандлы, контракты)

| Время | Стадия | Вид | Артефакт | Размер | Детали |
|---|---|---|---|---|---|
| 25.09 13:10 | 8-code-review | bundle | 8-code-review/input-bundle.md | 4KB | чекеры 8✅/0❌/1⚠; патчи 479KB |
| 25.09 13:10 | 8-code-review | bundle | 8-code-review/input-bundle.md | 4KB | чекеры 8✅/0❌/1⚠; патчи 479KB |
| 25.09 13:11 | 8-code-review | bundle | 8-code-review/input-bundle.md | 4KB | чекеры 8✅/0❌/1⚠; патчи 480KB |
| 25.09 13:18 | 8-code-review | bundle | 8-code-review/input-bundle.md | 5KB | чекеры 8✅/0❌/1⚠; патчи 484KB |

Бандлов: 4, суммарно 17KB — весь документный контекст, переданный между стадиями.
Нарушений пайп-контракта не зафиксировано (context-violations.jsonl отсутствует).

## Ожидание пользователя

| Стадия | Причина | Начало | Длит. |
|---|---|---|---|
| light-exec | light-exec | 25.09 12:51 | 3м 14с |

## Диаграмма

```mermaid
gantt
    dateFormat YYYY-MM-DDTHH:mm:ss
    axisFormat %H:%M
    title Хронометраж phase-1-skeleton
    section Стадии
    light-exec : 2026-09-25T12:49:21, 2026-09-25T13:09:49
    light-judge : 2026-09-25T13:09:49, 2026-09-25T13:11:52
    light-review : 2026-09-25T13:11:52, 2026-09-25T13:19:46
    section Ожидание
    light-exec : 2026-09-25T12:51:45, 2026-09-25T12:54:59
```

