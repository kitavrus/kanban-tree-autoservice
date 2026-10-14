---
stage: 5-design-review
feature: phase-3-clients
tier: M
verdict: APPROVED
high: 0
medium: 2
low: 1
---
# Design Review: phase-3-clients — APPROVED
**Дата:** 2026-09-25 (дельта-перепроверка после fix-loop #1, mobile-api-reviewer)
**Ревьюер:** design-reviewer agent

## Блокеры (design НЕ готов к stage Plan)
Нет.

## Серьёзные замечания (исправить до мержа)
1. **[3-design/design-go-layers.md:252]** SSR-таблица (`§SSR Routes`, перенесённая из `output.md` этим fix-loop) содержит строку `POST /clients/:id/delete | удалить (confirm, ADR-104)` — `ADR-104` не существует (список ADR в `output.md` — 001…007, 100…103). Дефект унаследован из исходной версии `output.md` (был там же до переноса) и не исправлен при переносе таблицы. Похоже на опечатку: удаление клиента описано в `ADR-103` («Удаление в SSR — POST-форма + делегированный `form[data-confirm]`»). Поправить ссылку на `ADR-103`.
2. **[4-swimlane/design-swimlane.html:365]** Каталог ошибок swimlane не обновлён вслед за fix-loop 1: строка `EQUIPMENT_TYPE_NAME_TAKEN (409) — уникальность имени (вкл. архив)` описывает СТАРУЮ модель (один код на оба случая). После правки `design-errors.md`/`ADR-007` дубль с активным типом → `EQUIPMENT_TYPE_NAME_TAKEN`, дубль с архивным → отдельный `EQUIPMENT_TYPE_NAME_ARCHIVED` (разные коды специально, чтобы клиент строил CTA по `code`, не по тексту). Swimlane — не owner-файл ошибок, но фактическая карта kода в нём теперь противоречит `design-errors.md`; строку стоит обновить (добавить `EQUIPMENT_TYPE_NAME_ARCHIVED`) при следующей правке артефакта, чтобы planner/reviewer не унесли устаревшую формулировку дальше.

## Незначительные замечания
1. **[4-swimlane/design-swimlane.html:373-395]** (без изменений с прошлого прогона) SVG-блок стрелок — декоративная заглушка: комментарий в файле сам говорит, что «на боевом swimlane стрелки отражают реальные потоки между ячейками сетки», т.е. текущие 3 стрелки не привязаны к конкретным ячейкам и не несут `items` с конкретными полями. Не блокер: содержательная нагрузка полностью раскрыта в `flows-section` (per-AC блоки с конкретными именами полей).

## Что проверено в дельте (fix-loop #1)
- **409 `EQUIPMENT_TYPE_NAME_ARCHIVED` (новый sentinel).** `design-errors.md`: `ErrTypeNameArchived`/`errNameArchived` заведены консистентно (было `ErrTypeNameTakenArchived`/`errNameTakenArchived` — переименование сквозное, старых имён не осталось). `design-sql.md` (`## Create Equipment Type Query`) обновлён на `errNameArchived`/`EQUIPMENT_TYPE_NAME_ARCHIVED`. `3-design/output.md` §API-таблица и `ADR-007 §Решение` синхронно ссылаются на новый код. `design-tests.md` `LV-031`/`ET-002` переписаны под раздельные коды + CTA-ссылку «Показать архивные». **Дырок не найдено** — единственный хвост, требующий правки, — swimlane (замечание выше).
- **Блок «Правила контракта» (`output.md` §Контракт).** Пять пунктов (LWW у equipment/types, семантика полной замены PUT, лимиты пагинации по эндпоинту, отказ от идемпотентности создания, язык `message`) не противоречат существующим ADR (ADR-100 — блокировка только у клиентов; ADR-102 — LWW у equipment допустима) и `design-go-layers.md` (`Service.List` сигнатуры комментариями синхронно несут те же default/max: `clients` 50/200, `equipment` 200/200, `equipment-types` 500/500). Числа в трёх местах (output.md таблица, go-layers комментарии, LV-035) совпадают 1:1.
- **Перенос SSR-таблицы в `design-go-layers.md` §SSR Routes.** Таблица перенесена дословно (кроме находки #1 выше), `output.md` §SSR теперь корректно ссылается на новое расположение (`design-go-layers.md §SSR Routes`). Дублирования между файлами не осталось.
- **Удаление Summary/Rejected/Plan hints из `output.md`.** Проверено на потерю фактов при сжатии (§AT-1.5 vibe-check): «Rejected» дублировал текст, уже присутствующий в теле ADR-003/ADR-006 (`Альтернативы`) и в `2-spec-interview/output.md` §«Отклонённые решения» — не потеряно. «Summary ключевых решений» дублировал заголовки самих ADR построчно — не потеряно (при необходимости навигации у каждого ADR есть номер и статус на месте). «Plan hints» (5 пунктов для planner) удалены полностью и нигде не продублированы — фактический контент («миграция+equipmenttypes», «clients API+phones+version», «equipment API», «shared row-list/ui-dialog/NavActive», «SSR+E2E по capability») в явном виде в других секциях design НЕ повторяется. Это не блокер (`Plan hints` — необязательная секция, planner способен вывести порядок фаз из `design-tests.md` §TD-1 + зависимостей DTO/SQL), но при желании можно вернуть короткий список — на усмотрение design-агента.
- **LV-031/LV-035/LV-036, ET-002.** Все присутствуют в `design-tests.md`, привязаны к AC-011 (LV-031, LV-035) и AC-002/AC-004 (LV-036/LV-035) в матрице AC→тесты; ET-002 расширен под раздельные коды. Согласовано с error mapping.

## Покрытие открытых вопросов
_(без изменений с прошлого прогона; источник Q-NNN — `2-spec-interview/output.md` §«Открытые вопросы», вне allowlist §3.6 — см. «Дырка контракта»)_

| Q-NNN | Формулировка | ADR-NNN | Статус |
|---|---|---|---|
| Q-001 | Демо-данные клиентов в `make seed-demo` | ADR-001 | Принято |
| Q-002 | Общий JS-повторитель строк | ADR-002 | Принято |
| Q-003 | Поиск по телефону и индексы | ADR-003 | Принято |
| Q-004 | Видимость архивных типов для executor и в формах | ADR-004 | Принято |
| Q-005 | SSR-маршрутизация мутаций и 409-перерисовка | ADR-005 | Принято |
| Q-006 | Каталог-навигация под фазу 4 | ADR-006 | Принято |
| Q-007 | Уникальность названия типа с учётом архива | ADR-007 | Принято (расширен fix-loop 1: раздельные коды) |

7/7 закрыто. ADR-100…ADR-103 — вне диапазона Q (доп. решения, без «Отвечает на» — ОК).

## Полнота файлов (feature, M-tier, diagrams: dataflow:no / sequence:no / swimlane:yes)
- [x] 3-design/output.md
- [ ] design-dataflow.md — n/a (diagrams.dataflow: no)
- [ ] design-sequence-diagrams.md — n/a (diagrams.sequence: no)
- [x] 4-swimlane/design-swimlane.html — см. замечания (не обновлён под fix-loop 1)
- [x] 3-design/design-go-layers.md (DI/Wiring + SSR Routes теперь тоже здесь)
- [x] 3-design/design-sql.md
- [x] 3-design/design-tests.md
- [ ] design-di.md — n/a, inline
- [ ] design-integrations.md — n/a (touches.external=false)
- [x] 3-design/design-errors.md
- [ ] design-infrastructure.md — n/a (touches.infra=false)
- [ ] design-adr.md — n/a, ADR inline в output.md (M-tier)
- [ ] 3-design/design-e2e-strategy.md — n/a (нет Vue, D30; frontend-architect N/A)

## Консистентность
- [x] DTO ↔ SQL-проекции — ОК, без изменений в затронутых полях
- [x] SQL columns ↔ migrations — ОК
- [x] Errors ↔ handlers ↔ tests — ОК после fix-loop (новый код `EQUIPMENT_TYPE_NAME_ARCHIVED` сквозной в errors/sql/output/tests)
- [ ] Errors ↔ swimlane — расхождение (см. Серьёзные замечания #2)
- [x] Q-NNN ↔ ADR-NNN — 7/7
- [x] ADR-ссылки в теле design — 1 dangling (`ADR-104`, см. Серьёзные замечания #1), остальные валидны
- [x] Дублей контента между design-файлами после сжатия не осталось (SSR-таблица теперь только в go-layers, ADR только в output.md)

## Дырка контракта (оркестратору, без изменений)
`5-design-review/input-bundle.md` не содержит секцию «Открытые вопросы» (`Q-NNN`) из `2-spec-interview/output.md`. Для Блока 2 пришлось точечно прочитать `2-spec-interview/output.md` целиком. Рекомендация бандлеру: включать `## Открытые вопросы` как отдельный `[источник]`.

## Итог
APPROVED — блокеров нет, design готов к стадии Plan. Дельта fix-loop #1 (новый код `EQUIPMENT_TYPE_NAME_ARCHIVED`, блок «Правила контракта», перенос SSR-таблицы, удаление Summary/Rejected/Plan hints, тесты LV-031/035/036 + ET-002) внесена консистентно и без потери фактов при сжатии. Два новых серьёзных замечания — не блокеры: dangling-ссылка `ADR-104` (опечатка, вероятно `ADR-103`) и не обновлённый под новый код swimlane-каталог ошибок; оба — точечные правки на следующей итерации, не требуют возврата на Design.
