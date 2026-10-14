# Review Bundle: soft-delete-board-columns — CHECKS-FAILED
**Base:** 7ab050367d4da43c538c037e0121b55386bda12a | **HEAD:** 9db8395 | **Generated:** 2026-10-08 15:00 | diff: 35 files changed, 2224 insertions(+), 128 deletions(-)

> ℹ️ Go-каталоги фичи выведены из диффа (имя «soft-delete-board-columns» не резолвится в пакет): `internal/features/board`, `internal/features/softdelete`

> Линзы ревью: читай ЭТОТ бандл + patch своей линзы. Полное чтение research/draft-spec/PRD
> и перегон чекеров ЗАПРЕЩЕНЫ (вердикты ниже; детали — checks/<name>.txt; точечный grep по DS-NNN разрешён).

## Чекеры (детерминированные — НЕ перегонять)
| Чекер | Вердикт | exit | Нарушения (первые строки) |
|---|---|---|---|
| go_naming | ✅ | 0 | — |
| api_conventions | ✅ | 0 | — |
| contract | ⚠️ SKIP-RESOLVE (Go-код в diff есть — НЕ зелёно, §GO-1: перегенерируй bundle с --paths/--go) | 0 | — |
| date_bound | ✅ | 0 | — |
| fuzz_verdict | ✅ | 0 | — |
| live_suite | ✅ | 0 | — |
| fixture_sync | ✅ | 0 | — |
| e2e_mode | ✅ | 0 | — |
| rule_usage | ✅ | 0 | — |
| e2e_drift | ❌ | 1 | e2e-drift: FAIL (режим: TS (spec-track §ST-1.4), doc: 1 id, code: 0 в 0 файлах, проблем: 1)<br>DRIFT: TS-042 есть в draft-spec.md, теста test('TS-042: …') нет (сценарий без теста) |

## Diff по линзам
| Линза | Файлов | Patch |
|---|---|---|
| backend | 18 | 8-code-review/diff-backend.patch |
| migrations | 2 | 8-code-review/diff-migrations.patch |
| other | 9 | 8-code-review/diff-other.patch |
| docs | 6 | — (артефакты, не ревьюятся построчно) |

### backend
- `M` internal/features/board/board_http_live_test.go
- `M` internal/features/board/card_repo.go
- `M` internal/features/board/columns_dto.go
- `M` internal/features/board/columns_handler.go
- `M` internal/features/board/columns_repo.go
- `M` internal/features/board/columns_service.go
- `A` internal/features/board/columns_soft_delete_http_live_test.go
- `M` internal/features/board/errors.go
- `M` internal/features/board/routes.go
- `M` internal/features/board/soft_delete_http_live_test.go
- `M` internal/features/board/tasks_repo.go
- `M` internal/features/board/tasks_service.go
- `A` internal/features/softdelete/columns_source.go
- `A` internal/features/softdelete/columns_source_test.go
- `M` internal/features/softdelete/softdelete_http_live_test.go
- `M` internal/seeddemo/seeddemo.go
- `M` internal/seeddemo/seeddemo_integration_test.go
- `M` internal/server/router.go

### migrations
- `A` migrations/0020_soft_delete_board_columns.down.sql
- `A` migrations/0020_soft_delete_board_columns.up.sql

### other
- `M` e2e/fixtures/auth.ts
- `M` e2e/fixtures/board-types.ts
- `M` e2e/fixtures/test.ts
- `M` e2e/pages/BoardPage.ts
- `M` e2e/tests/board.spec.ts
- `A` e2e/tests/soft-delete-board-columns.spec.ts
- `M` e2e/tests/soft-delete-core.spec.ts
- `M` web/ssr/_shared/js/board.js
- `M` web/ssr/_shared/js/deleted.js

### docs
- `A` docs/features/soft-delete-board-columns/0-setup/draft-spec.slice-light.md
- `A` docs/features/soft-delete-board-columns/0-setup/idioms.md
- `A` docs/features/soft-delete-board-columns/0-setup/light-progress.md
- `A` docs/features/soft-delete-board-columns/0-setup/passport.md
- `A` docs/features/soft-delete-board-columns/0-setup/pipeline-state.md
- `A` docs/features/soft-delete-board-columns/0-setup/timeline.jsonl

## Экстракты документов (вместо полного чтения)
### draft-spec.md (экстракт: шапка + DS + дельта + Impact + TS)
# Draft Spec: Мягкое удаление — колонки доски

**Tier:** S · **Совместимость:** MVP · **Режим:** new

**Дата:** 2026-10-08 | **Статус:** draft → final (после подтверждения пользователя) | **Tier:** S
**Совместимость:** MVP (DS-022) — рабочей версии с живыми пользователями нет: таблицы и адреса меняем свободно, без переходного периода и отката; данные на базе сохраняются — все существующие колонки после миграции считаются действующими
**UI-scope:** swagger:no | adaptive:min | themes:both (DS-023)
**Источник:** мастер-спека `docs/features/soft-delete/draft-spec.md` (§4.3, §6.5, §6.6, §6.26, §8.5) + её журнал `docs/features/soft-delete/draft-spec-decisions.md` + as-is факты кода (`internal/features/board/columns_service.go`, `columns_repo.go`, `columns_dto.go`, `migrations/0005_board.up.sql`)
**Часть 3 из 6 мастер-спеки `soft-delete`; зависит от: 1 (`soft-delete-core`).** Номера DS и TS — мастер-идентификаторы, не перенумерованы. Номера разделов — свои: §6.1 = мастер §6.5, §6.2 = мастер §6.6, §6.3 = строки мастера §6.26 про колонки; §8.1 = часть мастера §8.5 про колонки.

> После мержа этой части: колонка доски удаляется с причиной и восстанавливается из вкладки «Колонки» раздела «Удалённые»; удалённые заказы больше не удерживают колонку от удаления; заказ, чья колонка удалена, при восстановлении встаёт в первую нефинальную колонку с предупреждением. Переходное правило части 1 (DS-035) для колонок перестаёт действовать.

## §9. Решения (DS-index)
Строки — дословно из мастера, номера не меняются; включены только решения, действующие в части 3.

| DS | Формулировка | Источник | Статус |
|---|---|---|---|
| DS-003 | Удаляет и восстанавливает только администратор; раздел «Удалённые» и его API — только администратор; остальным — 403 «Нет доступа» | батч 1 (В-2) | closed |
| DS-011 | Один раздел «Удалённые» с вкладками по видам записей; в строке — что удалено, причина, кто, когда и кнопка «Восстановить»; фильтры — поиск, период, кто удалил | батч 2 (В-8), батч 4 (В-15); фильтры — inferred | closed |
| DS-012 | Удалённое хранится бессрочно; окончательного удаления и автоматической очистки нет | батч 3 (В-9) | closed |
| DS-016 | Заказ восстанавливается всегда: колонка удалена — встаёт в конец первой нефинальной действующей колонки; клиент или оборудование в удалённых — остаются с пометкой; обо всём — предупреждение | батч 3 (В-11); «нефинальной» — inferred (в финальную колонку нельзя с незавершёнными услугами) | closed |
| DS-019 | Колонка доски удаляется мягко, только без действующих заказов и не последняя нефинальная; удалённые заказы колонку не удерживают; восстановленная колонка встаёт в конец доски | батч 4 (В-14); условия и место восстановления — inferred из кода (`internal/features/board/columns_service.go`) | closed |
| DS-020 | Причина удаления обязательна для любого вида записи: 1–500 символов после обрезки пробелов; без неё удалить нельзя | батч 4 (В-15); границы длины — inferred | closed |
| DS-022 | Совместимость — MVP: рабочей версии нет, меняем свободно, без переходного периода; данные на базе сохраняются, существующие записи — действующие | батч 5 (В-17) | closed |
| DS-023 | UI-scope: Swagger не ведём; адаптив минимальный; темы в автотестах — обе | батч 5 (В-18, В-19, В-20) | closed |
| DS-024 | Конкурентность: удаление и восстановление — атомарный UPDATE с условием по `f:deleted_at`; повтор удаления — 404, повтор восстановления — 409 `NOT_DELETED`; заказ — под блокировкой строки; документ — по `f:version` | inferred | closed |
| DS-025 | Время удаления хранится в UTC, показывается и фильтруется по часовому поясу пользователя, формат «ДД.ММ.ГГГГ ЧЧ:ММ» | черновик (правила проекта), `docs/features/user-timezone` | closed |
| DS-026 | Удалённая запись для всех обычных адресов не существует: чтение по идентификатору и любая мутация — 404 своего вида; доступны только восстановление и раздел «Удалённые» | inferred из В-7 | closed |
| DS-027 | Уборка тестовых данных автотестами — служебный адрес физического удаления, включается переменной окружения `TEST_PURGE_ENABLED=true`, по умолчанию выключен | inferred, согласовано оркестратором (батч 4) | closed |
| DS-028 | Единый API: удаление — `DELETE /api/v1/{ресурс}/:id` с JSON-телом `{reason}` (у документа — и `version`, перенесён из query); восстановление — `POST /api/v1/{ресурс}/:id/restore`; список — `GET /api/v1/deleted` в конверте `{items, total, limit, offset}` | inferred, CLAUDE.md §API-1 | closed |
| DS-030 | Аудит: на записи хранится последнее удаление — кто, когда, почему; при восстановлении отметка стирается; история повторных удалений не ведётся; факты удаления и восстановления — в журнал приложения | inferred из В-15, zap | closed |
| DS-031 | Задача Tier L реализуется шестью частями S/M по порядку: `soft-delete-core`, `soft-delete-transfers`, `soft-delete-board-columns`, `soft-delete-clients`, `soft-delete-catalog`, `soft-delete-warehouses-suppliers`; спека части разворачивает полные таблицы полей своих таблиц | оркестратор (бриф), inferred | closed |
| DS-033 | UI-конвенции: ошибка поля — под полем; тосты — правый верхний угол, 5 секунд; причина и подтверждения — `KT.uiDialog`; пометка «удалён» — серый бейдж; предупреждения восстановления — жёлтый блок над таблицей | inferred, transfers DS-052 | closed |
| DS-035 | Переходное правило части 1 (до частей 3 и 4): мягко удалённый заказ по-прежнему удерживает своего клиента, единицы оборудования и колонку от безвозвратного удаления — действующие отказы `CLIENT_HAS_TASKS`, `EQUIPMENT_USED_IN_TASKS`, `COLUMN_NOT_EMPTY` срабатывают и по удалённым заказам | inferred — следствие разбивки DS-031 | closed |
| DS-039 | Заказ с незавершёнными услугами, чья колонка за время удаления стала финальной, при восстановлении ставится в конец первой нефинальной действующей колонки с предупреждением `column_replaced` — в финальной колонке не бывает заказов с незавершёнными услугами. Дополняет DS-016 | inferred из кода (`SUBTASKS_NOT_DONE`, `internal/features/board/columns_service.go`) | closed |

## §10. Открытые вопросы (status: open)
Открытых вопросов нет

## §11. Тестовые сценарии (черновик — вход для design-e2e-strategy, TD-6.8)
Оси UI-scope (DS-023): Swagger-сценариев нет; адаптив и темы раздела «Удалённые» и окна удаления покрыты TS-046, TS-047 части 1 (новая вкладка проверяется в них же). Номера TS — мастер-идентификаторы; включены только сценарии части 3. В TS-012 и TS-033 в этой части проверяются таблица `t:columns` и адреса §6.1, §6.2. Сценарий TS-049 части 1 в части про колонку (`COLUMN_NOT_EMPTY` при одном удалённом заказе) с этой частью перестаёт действовать и заменяется TS-019.

### Unit / Integration (Go)
- **TS-012.** Миграция: после применения все существующие строки 11 таблиц имеют `deleted_at IS NULL`; CHECK «три поля вместе» отклоняет частичное заполнение (части 1–6, каждая — свои таблицы) — DS-022, DS-030
  - негатив: UPDATE только `deleted_at` без `deleted_by` → ошибка CHECK
- **TS-019.** Колонка: удаление пустой → 200; с действующими заказами → 409 `COLUMN_NOT_EMPTY`; только с удалёнными заказами → 200; последняя нефинальная → 409 `LAST_NON_FINAL_COLUMN` (часть 3) — DS-019
  - негатив: перенос заказа в удалённую колонку → 404 `COLUMN_NOT_FOUND`
- **TS-020.** Восстановление заказа с удалённой колонкой: заказ в конце первой нефинальной действующей колонки, `f:notices` содержит `column_replaced` (часть 3) — DS-016
  - негатив: колонка заказа сама не восстановлена
- **TS-021.** Восстановление колонки: `f:position` = максимум среди действующих + 1 (часть 3) — DS-019
  - негатив: повтор → 409 `NOT_DELETED`
- **TS-033.** Права на всех 22 адресах удаления и восстановления: executor → 403 «Нет доступа» до поиска записи, включая несуществующий идентификатор (части 1–6) — DS-003
  - негатив: без сессии → 401

### E2E (Playwright)
- **TS-042.** Колонка доски — экраны §8.1, §8.2 — DS-019, DS-016 (часть 3)
  - шаги: удалить пустую колонку с причиной → восстановить; проверка: колонка в конце доски; негатив: колонка с заказом — «В колонке есть заказы»

### Фикстуры и сиды (АКЦЕНТ — хронический пропуск)
- **TS-F3.** `e2e/fixtures/auth.ts`: новые помощники `softDelete(kind, id, reason)`, `restore(kind, id)`, `listDeleted(kind, filters)`; payload'ы типизировать wire-типами (§TD-6.2; сырые ключи невидимы гейтам) — DS-028
- **TS-F8.** `e2e/fixtures/auth.ts`: `deleteColumn` (строка 635) передаёт `{reason}` и ждёт 200 с телом вместо 204; уборка созданных колонок в `e2e/fixtures/test.ts` (`_boardCleanup`) идёт служебным адресом `DELETE /api/v1/test/purge/columns/:id`; обновить в части 3 — DS-027

## §Impact — точки касания существующего кода
Источник: `python3 ~/.claude/tools/impact_scan.py` (sample `internal/features/board`; термы `columns`, `column_id`, `COLUMN_NOT_EMPTY`, `LAST_NON_FINAL_COLUMN`, `deleteColumn`, `/api/v1/columns`) + чтение запросов к `columns` в репозитории (fff) + решения мастера.

| # | Класс | Что задеваем (file:line / объект) | Дельта фичи | Решение | DS |
|---|---|---|---|---|---|
| 1 | SHARED-DEP | `internal/features/board/columns_service.go:209` (`ColumnService.Delete`), `columns_handler.go:84` (ответ 204), `routes.go:29` | мягкое удаление с причиной; ответ 200 `{id, deleted_at}` вместо 204; новый `Restore` и маршрут `POST /:id/restore` | править | DS-019, DS-028 |
| 2 | SHARED-DEP | `internal/features/board/columns_repo.go:34` (`List`), `:62` (`LockAll`), `:116` (`Renumber`), `:130` (`closeGaps`), `:139` (`HasTasks`), `:194` (`Delete`) | условие `deleted_at IS NULL` в списке, счётчике `total`, перенумерации; `HasTasks` — только действующие заказы; `Delete` → UPDATE; блокировка при восстановлении — все колонки | править | DS-019, DS-026 |
| 3 | SHARED-DEP | `internal/features/board/tasks_repo.go:708` (чтение колонки `FOR SHARE` при создании и переносе), `card_repo.go:411` | условие `deleted_at IS NULL` → `COLUMN_NOT_FOUND` | править | DS-026 |
| 4 | SHARED-DEP | `internal/features/board/tasks_service.go` (`Restore` части 1) | подмена удалённой колонки: первая нефинальная действующая, позиция в конец, `notices` `column_replaced`; общий код с DS-039 части 1 | править | DS-016, DS-039 |
| 5 | SHARED-DEP | `internal/features/board/tasks_repo.go:898`, `:940`, `:959`, `stock_repo.go:95`, `internal/features/users/repo.go:47` (соединения `tasks` с `columns`) | без изменений: действующий заказ всегда в действующей колонке | не трогать | DS-019 |
| 6 | SHARED-DEP | `internal/features/softdelete` (часть 1) | вид `columns` в списке, счётчиках и служебной уборке | править | DS-011, DS-027 |
| 7 | FK | `migrations/0005_board.up.sql:5`–`:15` (`columns`), `:27` (`tasks.column_id` RESTRICT) | новая миграция: три поля, CHECK `columns_deleted_chk`, FK `deleted_by`, индекс `columns_deleted_at_idx`; внешний ключ заказов не меняется | добавить | DS-019, DS-022 |
| 8 | FRONTEND | `web/ssr/board/templates/pages` и скрипт доски (настройка колонок); `web/ssr/softdelete` (часть 1) | «Удалить» через общее окно с причиной; вкладка «Колонки»; строка предупреждения о замене колонки | править | DS-020, DS-016 |
| 9 | ENUMERATION | `e2e/fixtures/auth.ts:635` (`deleteColumn`), `e2e/fixtures/test.ts:120` (`_boardCleanup`), `e2e/tests/board.spec.ts`, `board-guards.spec.ts` | тело запроса и ответ 200; уборка колонок служебным адресом; ожидания по удалению колонки | править | DS-027, DS-020 |
| 10 | SHARED-DEP | Go live-тесты `internal/features/board/board_http_live_test.go` (удаление колонки: 204, «строки нет») | ожидания: 200 с телом, строка помечена | править | DS-019, DS-028 |
| 11 | CONFIG | env-ключи | не задеваются | — | — |
| 12 | MONITORING | дашборды, nginx | не задеваются | — | — |

**Вне каталога фичи:** есть — internal/features/board, internal/features/softdelete, migrations, web/ssr/board, web/ssr/softdelete, e2e

### prd.md
MISSING

## Инвентарь доков (точечный grep разрешён, полное чтение — нет)
- draft-spec.md — 58KB
- 0-setup/draft-spec.slice-light.md — 51KB
- 0-setup/passport.md — 9KB
- 0-setup/idioms.md — 9KB
- 0-setup/light-progress.md — 1KB
- 0-setup/pipeline-state.md — 0KB

## Паспорт (выжимка; полный — 0-setup/passport.md)
# Passport: soft-delete-board-columns (generated — руками не править; stage_bundle.py)
**Tier:** S | **Совместимость:** MVP (pipeline-doctrine.md §CM-1; ? → уточни у оркестратора)
**Трек:** обычный (prefs нет — дефолт) — режимы агентов (`spec-track delta/micro/mini/merged`) включает ТОЛЬКО строка `Режим:` в prompt оркестратора; DS/TS-индексы ниже присутствуют на ЛЮБОМ треке при наличии draft-spec.
**Layout:** автодетект (tools/layout_resolve.py)
**UI-scope (pipeline-doctrine.md §US-1, шапка draft-spec):** swagger:no | adaptive:min | themes:both
