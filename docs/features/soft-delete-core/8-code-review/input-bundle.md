# Review Bundle: soft-delete-core — CHECKS-FAILED
**Base:** 022d8d6c8726e00888ceb6f325c97bbb0fd844e0 | **HEAD:** c3195c3 | **Generated:** 2026-10-08 13:40 | diff: 71 files changed, 7970 insertions(+), 257 deletions(-)

> ℹ️ Go-каталоги фичи выведены из диффа (имя «soft-delete-core» не резолвится в пакет): `internal/features/board`, `internal/features/clients`, `internal/features/executorpayouts`, `internal/features/softdelete`, `internal/features/users`

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
| e2e_drift | ❌ | 1 | e2e-drift: FAIL (режим: TS (spec-track §ST-1.4), doc: 8 id, code: 0 в 0 файлах, проблем: 8)<br>DRIFT: TS-034 есть в draft-spec.md, теста test('TS-034: …') нет (сценарий без теста)<br>DRIFT: TS-035 есть в draft-spec.md, теста test('TS-035: …') нет (сценарий без теста)<br>DRIFT: TS-036 есть в draft-spec.md, теста test('TS-036: …') нет (сценарий без теста)<br>DRIFT: TS-037 есть в draft-spec.md, теста test('TS-037: …') нет (сценарий без теста)<br>DRIFT: TS-038 есть в draft-spec.md, теста test('TS-038: …') нет (сценарий без теста)<br>DRIFT: TS-039 есть в draft-spec.md, теста test('TS-039: …') нет (сценарий без теста)<br>DRIFT: TS-046 есть в draft-spec.md, теста test('TS-046: …') нет (сценарий без теста) |

## Diff по линзам
| Линза | Файлов | Patch |
|---|---|---|
| backend | 43 | 8-code-review/diff-backend.patch |
| e2e | 1 | 8-code-review/diff-e2e.patch |
| migrations | 2 | 8-code-review/diff-migrations.patch |
| other | 15 | 8-code-review/diff-other.patch |
| docs | 10 | — (артефакты, не ревьюятся построчно) |

### backend
- `M` internal/config/config.go
- `M` internal/config/config_test.go
- `M` internal/features/board/attachments_gc_http_live_test.go
- `M` internal/features/board/attachments_http_live_test.go
- `M` internal/features/board/attachments_repo.go
- `M` internal/features/board/attachments_service.go
- `M` internal/features/board/board_fuzz_test.go
- `M` internal/features/board/board_http_live_test.go
- `M` internal/features/board/card_repo.go
- `M` internal/features/board/columns_repo.go
- `M` internal/features/board/errors.go
- `M` internal/features/board/order_card_http_live_test.go
- `M` internal/features/board/order_card_revert_http_live_test.go
- `M` internal/features/board/position.go
- `M` internal/features/board/routes.go
- `A` internal/features/board/soft_delete_http_live_test.go
- `A` internal/features/board/soft_delete_race_http_live_test.go
- `M` internal/features/board/stock_engine_integration_test.go
- `M` internal/features/board/stock_repo.go
- `M` internal/features/board/subtask_logs_repo.go
- `M` internal/features/board/subtask_logs_service.go
- `M` internal/features/board/tasks_dto.go
- `M` internal/features/board/tasks_handler.go
- `M` internal/features/board/tasks_repo.go
- `M` internal/features/board/tasks_service.go
- `M` internal/features/clients/history.go
- `M` internal/features/executorpayouts/repo.go
- `A` internal/features/softdelete/dto.go
- `A` internal/features/softdelete/handler.go
- `A` internal/features/softdelete/kinds.go
- `A` internal/features/softdelete/reason.go
- `A` internal/features/softdelete/reason_test.go
- `A` internal/features/softdelete/repo.go
- `A` internal/features/softdelete/routes.go
- `A` internal/features/softdelete/service.go
- `A` internal/features/softdelete/softdelete_http_live_test.go
- `A` internal/features/softdelete/ssr.go
- `A` internal/features/softdelete/validate.go
- `M` internal/features/users/repo.go
- `M` internal/seeddemo/seeddemo.go
- `M` internal/seeddemo/seeddemo_integration_test.go
- `M` internal/server/router.go
- `M` internal/ssr/menu.go

### e2e
- `M` scripts/e2e/run.sh

### migrations
- `A` migrations/0018_soft_delete_core.down.sql
- `A` migrations/0018_soft_delete_core.up.sql

### other
- `M` .env.example
- `M` docker-compose.e2e.yml
- `M` e2e/fixtures/auth.ts
- `M` e2e/fixtures/board-types.ts
- `M` e2e/fixtures/test.ts
- `M` e2e/pages/BoardPage.ts
- `A` e2e/pages/DeletedPage.ts
- `M` e2e/pages/OrderCardPage.ts
- `A` e2e/tests/soft-delete-core.spec.ts
- `M` e2e/tests/transfers-revert.spec.ts
- `M` web/ssr/_shared/js/api.js
- `A` web/ssr/_shared/js/deleted.js
- `M` web/ssr/_shared/js/task-card.js
- `M` web/ssr/_shared/js/ui-dialog.js
- `A` web/ssr/softdelete/templates/pages/list.html

### docs
- `A` docs/features/soft-delete-core/0-setup/draft-spec.slice-light.md
- `A` docs/features/soft-delete-core/0-setup/idioms.md
- `A` docs/features/soft-delete-core/0-setup/light-progress.md
- `A` docs/features/soft-delete-core/0-setup/passport.md
- `A` docs/features/soft-delete-core/0-setup/pipeline-state.md
- `A` docs/features/soft-delete-core/0-setup/timeline.jsonl
- `A` docs/features/soft-delete-core/draft-spec-flow.svg
- `A` docs/features/soft-delete-core/draft-spec-pmap.txt
- `A` docs/features/soft-delete-core/draft-spec.html
- `A` docs/features/soft-delete-core/draft-spec.md

## Экстракты документов (вместо полного чтения)
### draft-spec.md (экстракт: шапка + DS + дельта + Impact + TS)
# Draft Spec: Мягкое удаление — ядро (заказы, раздел «Удалённые», причина удаления)

**Tier:** M · **Совместимость:** MVP · **Режим:** new

**Дата:** 2026-10-08 | **Статус:** draft → final (после подтверждения пользователя) | **Tier:** M (верхняя граница)
**Совместимость:** MVP (DS-022) — рабочей версии с живыми пользователями нет: таблицы и адреса меняем свободно, без переходного периода и отката; данные на базе сохраняются — все существующие заказы после миграции считаются действующими
**UI-scope:** swagger:no | adaptive:min | themes:both (DS-023)
**Источник:** мастер-спека `docs/features/soft-delete/draft-spec.md` (§4.1, §5, §6.1, §6.2, §6.23–§6.26, §8.1–§8.3) + её журнал `docs/features/soft-delete/draft-spec-decisions.md` + as-is факты кода (`migrations/0005_board.up.sql`, `internal/features/board/tasks_service.go`)
**Часть 1 из 6 мастер-спеки `soft-delete`; зависит от: —.** Номера DS и TS — мастер-идентификаторы, не перенумерованы. Номера разделов §6 — свои: §6.1, §6.2 совпадают с мастером; §6.3 = мастер §6.23, §6.4 = мастер §6.24, §6.5 = мастер §6.25, §6.6 = мастер §6.26 (строки про заказы).

> После мержа этой части: администратор удаляет заказ только с причиной, заказ уходит в новый раздел «Удалённые» (`/deleted`, вкладка «Заказы») и возвращается оттуда кнопкой «Восстановить»; удалённый заказ не виден на доске, в поиске, в «Остатках», «Выплатах», истории оборудования. Остальные виды записей (документы перемещения, колонки, клиенты, оборудование, каталог, склады, поставщики) удаляются по-старому, безвозвратно — до своих частей 2–6.

## §9. Решения (DS-index)
Строки — дословно из мастера, номера не меняются; включены только решения, действующие в части 1.

| DS | Формулировка | Источник | Статус |
|---|---|---|---|
| DS-001 | Охват: мягко удаляются 11 видов записей — заказы, документы перемещения, колонки доски, клиенты, единицы оборудования, товары, услуги каталога, категории, типы оборудования, склады, поставщики | батч 1 (В-1), батч 4 (В-14) | closed |
| DS-002 | Вне охвата: сотрудники (только «неактивен») и части заказа — услуга, товар в услуге, запись о работе, вложение, оборудование в заказе (удаляются как сейчас); восстановим только заказ целиком со всеми частями | батч 4 (В-13, В-14) | closed |
| DS-003 | Удаляет и восстанавливает только администратор; раздел «Удалённые» и его API — только администратор; остальным — 403 «Нет доступа» | батч 1 (В-2) | closed |
| DS-004 | При удалении заказа все его товары возвращаются на склад (как сейчас) | батч 1 (В-3), код (`internal/features/board/tasks_service.go`, `TaskService.Delete`) | closed |
| DS-005 | При восстановлении заказа товары списываются снова по правилу его колонки; при нехватке заказ восстанавливается с предупреждением, не отказом | батч 1 (В-3) | closed |
| DS-010 | Удалённый заказ не виден нигде, кроме раздела «Удалённые» и пометки у своих документов: доска, поиск, карточка, печать, «Остатки», «Выплаты», история оборудования, итоги | батч 2 (В-7) | closed |
| DS-011 | Один раздел «Удалённые» с вкладками по видам записей; в строке — что удалено, причина, кто, когда и кнопка «Восстановить»; фильтры — поиск, период, кто удалил | батч 2 (В-8), батч 4 (В-15); фильтры — inferred | closed |
| DS-012 | Удалённое хранится бессрочно; окончательного удаления и автоматической очистки нет | батч 3 (В-9) | closed |
| DS-013 | Вложения удалённого заказа: строки и файлы остаются; файл по прямой ссылке не отдаётся (404); фоновая уборка файлов-сирот их не трогает — она сверяет файлы со строками `t:task_attachments`, а строки остаются | inferred из В-9, код (`internal/features/board/attachments_repo.go`, `ExistingKeys`) | closed |
| DS-020 | Причина удаления обязательна для любого вида записи: 1–500 символов после обрезки пробелов; без неё удалить нельзя | батч 4 (В-15); границы длины — inferred | closed |
| DS-022 | Совместимость — MVP: рабочей версии нет, меняем свободно, без переходного периода; данные на базе сохраняются, существующие записи — действующие | батч 5 (В-17) | closed |
| DS-023 | UI-scope: Swagger не ведём; адаптив минимальный; темы в автотестах — обе | батч 5 (В-18, В-19, В-20) | closed |
| DS-024 | Конкурентность: удаление и восстановление — атомарный UPDATE с условием по `f:deleted_at`; повтор удаления — 404, повтор восстановления — 409 `NOT_DELETED`; заказ — под блокировкой строки; документ — по `f:version` | inferred | closed |
| DS-025 | Время удаления хранится в UTC, показывается и фильтруется по часовому поясу пользователя, формат «ДД.ММ.ГГГГ ЧЧ:ММ» | черновик (правила проекта), `docs/features/user-timezone` | closed |
| DS-026 | Удалённая запись для всех обычных адресов не существует: чтение по идентификатору и любая мутация — 404 своего вида; доступны только восстановление и раздел «Удалённые» | inferred из В-7 | closed |
| DS-027 | Уборка тестовых данных автотестами — служебный адрес физического удаления, включается переменной окружения `TEST_PURGE_ENABLED=true`, по умолчанию выключен | inferred, согласовано оркестратором (батч 4) | closed |
| DS-028 | Единый API: удаление — `DELETE /api/v1/{ресурс}/:id` с JSON-телом `{reason}` (у документа — и `version`, перенесён из query); восстановление — `POST /api/v1/{ресурс}/:id/restore`; список — `GET /api/v1/deleted` в конверте `{items, total, limit, offset}` | inferred, CLAUDE.md §API-1 | closed |
| DS-029 | Следствие DS-010: суммы отчёта «Выплаты» за прошлые периоды уменьшаются после удаления заказа с уже выплаченными услугами и возвращаются после его восстановления | inferred из В-7 | closed |
| DS-030 | Аудит: на записи хранится последнее удаление — кто, когда, почему; при восстановлении отметка стирается; история повторных удалений не ведётся; факты удаления и восстановления — в журнал приложения | inferred из В-15, zap | closed |
| DS-031 | Задача Tier L реализуется шестью частями S/M по порядку: `soft-delete-core`, `soft-delete-transfers`, `soft-delete-board-columns`, `soft-delete-clients`, `soft-delete-catalog`, `soft-delete-warehouses-suppliers`; спека части разворачивает полные таблицы полей своих таблиц | оркестратор (бриф), inferred | closed |
| DS-033 | UI-конвенции: ошибка поля — под полем; тосты — правый верхний угол, 5 секунд; причина и подтверждения — `KT.uiDialog`; пометка «удалён» — серый бейдж; предупреждения восстановления — жёлтый блок над таблицей | inferred, transfers DS-052 | closed |
| DS-034 | Переходное правило части 1 (до части 2): заказ с документом в «Заявка», «В пути», «Получено» по-прежнему не удаляется (409 `TASK_HAS_TRANSFERS`); отменённые документы с части 1 вместе с заказом не удаляются и остаются у удалённого заказа. Часть 2 снимает отказ (DS-006) | inferred — следствие разбивки DS-031 и DS-006 | closed |
| DS-035 | Переходное правило части 1 (до частей 3 и 4): мягко удалённый заказ по-прежнему удерживает своего клиента, единицы оборудования и колонку от безвозвратного удаления — действующие отказы `CLIENT_HAS_TASKS`, `EQUIPMENT_USED_IN_TASKS`, `COLUMN_NOT_EMPTY` срабатывают и по удалённым заказам | inferred — следствие разбивки DS-031 | closed |
| DS-036 | Служебная уборка заказа повторяет действующее безвозвратное удаление заказа: неотменённые документы — отказ, отменённые удаляются вместе с заказом; документы автотест убирает сам, как сейчас; работает для действующего и для удалённого заказа. Уточняет DS-027 | inferred | closed |
| DS-039 | Заказ с незавершёнными услугами, чья колонка за время удаления стала финальной, при восстановлении ставится в конец первой нефинальной действующей колонки с предупреждением `column_replaced` — в финальной колонке не бывает заказов с незавершёнными услугами. Дополняет DS-016 | inferred из кода (`SUBTASKS_NOT_DONE`, `internal/features/board/columns_service.go`) | closed |

## §10. Открытые вопросы (status: open)
Открытых вопросов нет

## §11. Тестовые сценарии (черновик — вход для design-e2e-strategy, TD-6.8)
Оси UI-scope (DS-023): Swagger-сценариев нет; адаптив — один смок на отсутствие горизонтальной прокрутки (TS-046); темы — обе (TS-047). Номера TS — мастер-идентификаторы; включены только сценарии части 1. В TS-012 и TS-033 в этой части проверяются только таблица `t:tasks` и адреса §6.1–§6.5.

### Unit / Integration (Go)
- **TS-001.** Валидатор причины удаления: пусто и одни пробелы → 400 `required`; 501 символ → 400 `too_long`; NUL → 400 `invalid_text`; 1 и 500 символов → принято, крайние пробелы обрезаны (часть 1) — DS-020
  - негатив: тело без `reason` → 400 `required`, запись не удалена
- **TS-002.** Удаление заказа: 200, `f:deleted_at`, `f:deleted_by`, `f:delete_reason` заполнены, `t:task_stock` заказа пуст, остаток `t:product_stock` вырос на количество заказа, в `t:stock_movements` — возвраты (часть 1) — DS-004
  - негатив: executor → 403 «Нет доступа», остатки не изменились
- **TS-003.** Повтор удаления заказа → 404 `TASK_NOT_FOUND`; два параллельных удаления — ровно один 200, второй 404, возврат товара выполнен один раз (часть 1) — DS-024
  - негатив: причина второго запроса не записана
- **TS-004.** Восстановление заказа в колонке, списывающей товар: 200, поля удаления очищены, `t:task_stock` и `t:product_stock` как до удаления (часть 1) — DS-005
  - негатив: повтор восстановления → 409 `NOT_DELETED`, остатки не изменились
- **TS-005.** Восстановление заказа при нехватке товара: 200, `f:warnings` содержит пару товар × склад с `kind` = `negative` (часть 1) — DS-005
  - негатив: восстановление не отклоняется
- **TS-006.** Читатели заказов: удалённый заказ отсутствует в `GET /api/v1/tasks`, поиске, `GET /api/v1/stock`, `GET /api/v1/executor-payouts` и его `summary`, `GET /api/v1/equipment/:id/history`; после восстановления — снова присутствует (часть 1) — DS-010, DS-029
  - негатив: `GET /api/v1/tasks/:id` и `/card` удалённого заказа → 404
- **TS-007.** Мутации удалённого заказа: `PUT /api/v1/tasks/:id`, добавление услуги, товара, записи, оборудования, вложения → 404 `TASK_NOT_FOUND` (часть 1) — DS-026
  - негатив: данные заказа не изменились
- **TS-008.** Вложения: после удаления заказа строки `t:task_attachments` и файлы на диске на месте; `GET /api/v1/attachments/:id/file` → 404; проход уборки сирот файлы не удаляет; после восстановления файл отдаётся (часть 1) — DS-013
  - негатив: файл без строки по-прежнему удаляется уборкой
- **TS-009.** `GET /api/v1/deleted`: `kind` обязателен и из списка; фильтры `q`, `deleted_from`, `deleted_to`, `deleted_by` сочетаются по «И»; конверт `{items, total, limit, offset}`; сортировка по `f:deleted_at` по убыванию (часть 1) — DS-011, DS-028
  - негатив: `kind=orders` → 400 `invalid`; `limit=201` → 400 `invalid`; executor → 403
- **TS-010.** Период по поясу пользователя: запись, удалённая в 23:30 UTC, попадает в «следующий день» у пользователя с поясом +03:00 и в «этот день» у пользователя с поясом UTC (часть 1) — DS-025
  - негатив: `deleted_to` раньше `deleted_from` → 400 `invalid`
- **TS-011.** `GET /api/v1/deleted/summary`: счётчики по 11 видам совпадают с `total` соответствующих списков (часть 1) — DS-011
  - негатив: executor → 403
- **TS-012.** Миграция: после применения все существующие строки 11 таблиц имеют `deleted_at IS NULL`; CHECK «три поля вместе» отклоняет частичное заполнение (части 1–6, каждая — свои таблицы) — DS-022, DS-030
  - негатив: UPDATE только `deleted_at` без `deleted_by` → ошибка CHECK
- **TS-013.** Служебная уборка: при `TEST_PURGE_ENABLED=true` `DELETE /api/v1/test/purge/tasks/:id` физически удаляет заказ с частями, документами и файлами; без переменной → 404 (часть 1) — DS-027
  - негатив: executor → 403
- **TS-033.** Права на всех 22 адресах удаления и восстановления: executor → 403 «Нет доступа» до поиска записи, включая несуществующий идентификатор (части 1–6) — DS-003
  - негатив: без сессии → 401
- **TS-048.** Переходное правило части 1: удаление заказа с отменённым документом → 200, документ остался в базе и ссылается на удалённый заказ; после восстановления заказа документ на месте (часть 1) — DS-034
  - негатив: удаление заказа с документом в `requested`, `in_transit`, `received` → 409 `TASK_HAS_TRANSFERS` с `details.transfer_ids`, заказ не удалён
- **TS-049.** Переходное правило части 1: клиент, у которого есть только удалённый заказ, → `DELETE /api/v1/clients/:id` 409 `CLIENT_HAS_TASKS`; колонка, в которой только удалённый заказ, → `DELETE /api/v1/columns/:id` 409 `COLUMN_NOT_EMPTY` (часть 1) — DS-035
  - негатив: строки заказа, клиента и колонки физически на месте
- **TS-052.** Восстановление заказа с незавершённой услугой после того, как его колонку сделали финальной: заказ в конце первой нефинальной колонки, `f:notices` содержит `column_replaced`; смена `consumes_stock` колонки, пока заказ удалён, товар по нему не списывает (часть 1) — DS-039, DS-004
  - негатив: заказ без незавершённых услуг восстанавливается в свою финальную колонку, `f:notices` пуст

### E2E (Playwright)
- **TS-034.** Удалить заказ с причиной — экран §8.3, §8.2 — DS-004, DS-020 (часть 1)
  - шаги: открыть карточку заказа → «Удалить заказ» → ввести причину → «Удалить»; проверка: карточка закрылась, заказа нет на доске, тост «Удалено…»; негатив: пустая причина → под полем «Укажите причину удаления», заказ на месте
- **TS-035.** Раздел «Удалённые», вкладка «Заказы» — экран §8.1 — DS-011 (часть 1)
  - шаги: меню «Удалённые» → вкладка «Заказы»; проверка: строка с номером, клиентом, причиной, именем удалившего и временем в формате «ДД.ММ.ГГГГ ЧЧ:ММ», счётчик вкладки; негатив: executor по прямой ссылке `/deleted` → «Нет доступа», пункта меню нет
- **TS-036.** Восстановить заказ — экран §8.1 — DS-005 (часть 1)
  - шаги: «Восстановить» → подтвердить; проверка: строка исчезла, заказ снова на доске в своей колонке, остатки как до удаления; негатив: повторное восстановление во второй вкладке браузера → тост «Запись уже восстановлена»
- **TS-037.** Восстановление с нехваткой товара — экран §8.1 — DS-005 (часть 1)
  - шаги: удалить заказ → списать товар со склада → восстановить; проверка: жёлтый блок с названием товара и склада, ссылка «Открыть заказ»; негатив: заказ всё равно восстановлен
- **TS-038.** Фильтры раздела «Удалённые» — экран §8.1 — DS-011, DS-025 (часть 1)
  - шаги: поиск по причине → период → «Кто удалил» → «Сбросить»; проверка: список сужается и возвращается, фильтры в адресной строке и переживают перезагрузку; негатив: период «по» раньше «с» → «Проверьте период»
- **TS-039.** Прямые ссылки на удалённый заказ — экраны §8.3 — DS-010, DS-013 (часть 1)
  - шаги: удалить заказ → открыть `/?task={номер}`, печатную форму, ссылку на вложение; проверка: «Заказ не найден» / 404; негатив: после восстановления все три открываются
- **TS-046.** Адаптив (минимальный): `/deleted` на ширине 360 без горизонтальной прокрутки страницы, окно §8.2 помещается — экран §8.1, §8.2 — DS-023 (часть 1)
  - шаги: открыть страницу на 360×640 → открыть окно удаления; проверка: `scrollWidth <= clientWidth`; негатив: —
- **TS-047.** Темы: `/deleted` и окно §8.2 в светлой и тёмной теме — бейджи, жёлтый блок, текст ошибки читаемы — экран §8.1, §8.2 — DS-023 (часть 1)
  - шаги: переключить тему → открыть экран; проверка: контраст элементов; негатив: —

### Фикстуры и сиды (АКЦЕНТ — хронический пропуск)
- **TS-F1.** `e2e/fixtures/auth.ts` (`purgeTask`, строка 590) и `e2e/fixtures/test.ts` (`_boardCleanup`): уборка переводится на служебный адрес §6.5 — иначе тестовые заказы копятся в «Удалённых»; обновить в части 1, в той же фазе, что миграция — DS-027
- **TS-F2.** `docker-compose.e2e.yml` и локальный запуск: добавить `TEST_PURGE_ENABLED=true`; рабочие конфигурации — без переменной — DS-027
- **TS-F3.** `e2e/fixtures/auth.ts`: новые помощники `softDelete(kind, id, reason)`, `restore(kind, id)`, `listDeleted(kind, filters)`; payload'ы типизировать wire-типами (§TD-6.2; сырые ключи невидимы гейтам) — DS-028
- **TS-F5.** `internal/seeddemo/seeddemo.go`: демо-данные создаются действующими; добавить по одной удалённой записи вида «заказ» и «клиент» с причиной — чтобы раздел «Удалённые» был непустым на демо-стенде — DS-011
- **TS-F6.** Go live-тесты удаления (`internal/features/board/board_http_live_test.go`, `internal/features/transfers/transfers_partners_http_live_test.go`, `internal/features/clients`, `internal/features/catalog`): запросы DELETE получают тело с причиной; проверки «строки нет в базе» заменяются на «строка помечена удалённой» — DS-020, DS-028

## §Impact — точки касания существующего кода
Источник: `python3 ~/.claude/tools/impact_scan.py` (sample `internal/features/board`; термы `tasks`, `task_attachments`, `purgeTask`, `TASK_NOT_FOUND`, `deleted_at`, `/api/v1/tasks`) + чтение всех запросов к `tasks` вне пакета `board` (fff) + решения мастера.

| # | Класс | Что задеваем (file:line / объект) | Дельта фичи | Решение | DS |
|---|---|---|---|---|---|
| 1 | ENUMERATION | `internal/server/router.go:198`–`:211` | регистрация нового пакета `internal/features/softdelete` (API + страница `/deleted`); условная регистрация служебной уборки по `TEST_PURGE_ENABLED` | добавить | DS-011, DS-027 |
| 2 | ENUMERATION | `internal/ssr/menu.go:34` | пункт «Удалённые» в группе «Администрирование», `AdminOnly` | добавить | DS-011 |
| 3 | SHARED-DEP | `internal/features/board/tasks_service.go` (`TaskService.Delete`, `deleteTaskTransfers`), `tasks_handler.go:112`, `tasks_dto.go:119` (`TaskDeleteResponse`), `routes.go:38` | мягкое удаление с причиной; отменённые документы не удаляются; новый `Restore`; ответ удаления получает `id`, `deleted_at` (поле `warnings` остаётся) | править | DS-004, DS-005, DS-020, DS-034 |
| 4 | SHARED-DEP | `internal/features/board/tasks_repo.go` (9 запросов к `tasks`), `card_repo.go` (2), `position.go` (5), `stock_repo.go` (1), `columns_repo.go` (5) | условие `deleted_at IS NULL` в чтении доски, карточки, позиций, остатков; `ColumnRepo.TaskIDsForUpdate` и `OpenSubtasksForColumn` (правка колонки) — только действующие заказы, иначе смена `consumes_stock` колонки спишет товар удалённого заказа; `ColumnRepo.HasTasks` (запрет удаления колонки) — БЕЗ условия (DS-035); `tasks_count` в `ColumnResponse` — только действующие | править | DS-010, DS-004, DS-035, DS-039 |
| 5 | SHARED-DEP | `internal/features/board/attachments_repo.go` (`Get`, `List`, `TaskExists`; `ExistingKeys:128` — не трогать), `attachments_service.go` | вложение удалённого заказа → 404; уборка сирот без изменений | править | DS-013 |
| 6 | SHARED-DEP | `internal/features/board` — сервисы частей заказа (`subtasks_service.go`, `subtask_products_service.go`, `subtask_logs_service.go`, `task_equipment_service.go`) и печать (`routes.go:88`) | мутации частей удалённого заказа → 404; общей точки нет: `LockTask` вызывают только `tasks_service.go` и `task_equipment_service.go`, остальные сервисы блокируют заказ своими запросами — условие `deleted_at IS NULL` добавляется в каждый путь | править каждый путь | DS-026 |
| 7 | SHARED-DEP | `internal/features/executorpayouts/repo.go:74`, `:108`, `:192` (и запросы итогов `:43`, `:143`, `:154`) | услуги удалённых заказов исключаются из списка, итогов и отметок выплаты | править | DS-010, DS-029 |
| 8 | SHARED-DEP | `internal/features/users/repo.go:47` (`activeTasksSQL`) | счётчик «Активных заказов» без удалённых | править | DS-010 |
| 9 | SHARED-DEP | `internal/features/clients/history.go:206` (и запросы `:240`, `:274`, `:302`) | история единицы без удалённых заказов | править | DS-010 |
| 10 | SHARED-DEP | `internal/features/clients/repo.go:341`, `:523` | проверки `CLIENT_HAS_TASKS`, `EQUIPMENT_USED_IN_TASKS` — без изменений, учитывают и удалённые заказы | не трогать | DS-035 |
| 11 | SHARED-DEP | `internal/features/transfers/repo.go:185`, `:1157` | без изменений в части 1 (пометка и проверка «заказ не удалён» — часть 2) | не трогать | DS-034 |
| 12 | ENUMERATION | `internal/features/board/errors.go` | новый код `NOT_DELETED` (409 «Запись уже восстановлена») — общий для частей 2–6; `TASK_HAS_TRANSFERS` остаётся | добавить | DS-024, DS-034 |
| 13 | FK | `migrations/0005_board.up.sql:25`–`:44` (`tasks`) | новая миграция `0018`: три колонки, CHECK `tasks_deleted_chk`, FK `deleted_by`, частичный индекс `tasks_deleted_at_idx`; down-миграция — по конвенции проекта | добавить | DS-001, DS-022 |
| 14 | CONFIG | `internal/config/config.go:75`, место запуска сервера для E2E | новый ключ окружения `TEST_PURGE_ENABLED` (по умолчанию выключен) | добавить | DS-027 |
| 15 | SHARED-DEP | `internal/seeddemo/seeddemo.go`, `seeddemo_integration_test.go` | один удалённый заказ с причиной в демо-данных; очистка демо-базы не ломается на новых колонках | править | DS-011 |
| 16 | FRONTEND | `web/ssr/_shared/js/ui-dialog.js` (`KT.uiDialog`), `web/ssr/board`; новый каталог `web/ssr/softdelete` | режим диалога с обязательным текстовым полем и счётчиком; «Удалить заказ» через него; страница «Удалённые» | править, добавить | DS-020, DS-033 |
| 17 | ENUMERATION | `e2e/fixtures/auth.ts:590` (`purgeTask`, `deleteTask`), `e2e/fixtures/test.ts:85` (`_boardCleanup`), `e2e/tests/board.spec.ts`, `board-guards.spec.ts`, `order-card.spec.ts`, `order-attachments.spec.ts`, `polish.spec.ts` | уборка через §6.5; `deleteTask` передаёт причину; ожидания «заказ удалён» | править | DS-027, DS-020 |
| 18 | SHARED-DEP | Go live-тесты `internal/features/board/board_http_live_test.go`, `attachments_gc_integration_test.go`, `internal/features/executorpayouts/executorpayouts_http_live_test.go` | DELETE с телом причины; «строка помечена», а не «строки нет» | править | DS-020, DS-028 |
| 19 | MONITORING | `deploy/nginx/kanban-tree.conf.example` | не задевается | — | — |

**Вне каталога фичи:** есть — internal/features/board, internal/features/executorpayouts, internal/features/users, internal/features/clients, internal/server/router.go, internal/ssr/menu.go, internal/config/config.go, internal/seeddemo, migrations, web/ssr/_shared, web/ssr/board, e2e

### prd.md
MISSING

## Инвентарь доков (точечный grep разрешён, полное чтение — нет)
- draft-spec.md — 103KB
- 0-setup/draft-spec.slice-light.md — 92KB
- 8-code-review/input-bundle.md — 38KB
- 0-setup/idioms.md — 31KB
- 0-setup/passport.md — 17KB
- 8-code-review/output.md — 12KB
- 0-setup/light-progress.md — 3KB
- 0-setup/pipeline-state.md — 0KB
- 9-validation/evidence/last-run.json — 0KB

## Паспорт (выжимка; полный — 0-setup/passport.md)
# Passport: soft-delete-core (generated — руками не править; stage_bundle.py)
**Tier:** M | **Совместимость:** MVP (pipeline-doctrine.md §CM-1; ? → уточни у оркестратора)
**Трек:** обычный (prefs нет — дефолт) — режимы агентов (`spec-track delta/micro/mini/merged`) включает ТОЛЬКО строка `Режим:` в prompt оркестратора; DS/TS-индексы ниже присутствуют на ЛЮБОМ треке при наличии draft-spec.
**Layout:** автодетект (tools/layout_resolve.py)
**UI-scope (pipeline-doctrine.md §US-1, шапка draft-spec):** swagger:no | adaptive:min | themes:both
