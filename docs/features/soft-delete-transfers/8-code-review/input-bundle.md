# Review Bundle: soft-delete-transfers — CHECKS-FAILED
**Base:** ecc023b4d08952660361d2aaea22cb0deafeff93 | **HEAD:** 562e3a3 | **Generated:** 2026-10-08 14:29 | diff: 48 files changed, 3330 insertions(+), 345 deletions(-)

> ℹ️ Go-каталоги фичи выведены из диффа (имя «soft-delete-transfers» не резолвится в пакет): `internal/features/board`, `internal/features/clients`, `internal/features/softdelete`, `internal/features/transfers`

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
| e2e_drift | ❌ | 1 | e2e-drift: FAIL (режим: TS (spec-track §ST-1.4), doc: 2 id, code: 0 в 0 файлах, проблем: 2)<br>DRIFT: TS-040 есть в draft-spec.md, теста test('TS-040: …') нет (сценарий без теста)<br>DRIFT: TS-041 есть в draft-spec.md, теста test('TS-041: …') нет (сценарий без теста) |

## Diff по линзам
| Линза | Файлов | Patch |
|---|---|---|
| backend | 26 | 8-code-review/diff-backend.patch |
| migrations | 2 | 8-code-review/diff-migrations.patch |
| other | 13 | 8-code-review/diff-other.patch |
| docs | 7 | — (артефакты, не ревьюятся построчно) |

### backend
- `M` internal/features/board/board_http_live_test.go
- `M` internal/features/board/card_repo.go
- `M` internal/features/board/order_card_revert_http_live_test.go
- `A` internal/features/board/soft_delete_transfers_http_live_test.go
- `M` internal/features/board/stock_repo.go
- `M` internal/features/board/tasks_handler.go
- `M` internal/features/board/tasks_repo.go
- `M` internal/features/board/tasks_service.go
- `M` internal/features/clients/history.go
- `M` internal/features/softdelete/softdelete_http_live_test.go
- `A` internal/features/softdelete/transfers_source.go
- `A` internal/features/softdelete/transfers_source_test.go
- `M` internal/features/transfers/dto.go
- `M` internal/features/transfers/handler.go
- `A` internal/features/transfers/migration_soft_delete_integration_test.go
- `M` internal/features/transfers/repo.go
- `M` internal/features/transfers/routes.go
- `M` internal/features/transfers/service.go
- `M` internal/features/transfers/transfers_http_live_test.go
- `M` internal/features/transfers/transfers_revert_http_live_test.go
- `A` internal/features/transfers/transfers_soft_delete_http_live_test.go
- `M` internal/features/transfers/validate.go
- `M` internal/features/transfers/validate_revert_test.go
- `M` internal/seeddemo/seeddemo.go
- `M` internal/seeddemo/seeddemo_integration_test.go
- `M` internal/server/router.go

### migrations
- `A` migrations/0019_soft_delete_transfers.down.sql
- `A` migrations/0019_soft_delete_transfers.up.sql

### other
- `M` e2e/fixtures/auth.ts
- `M` e2e/fixtures/board-types.ts
- `M` e2e/fixtures/test.ts
- `M` e2e/pages/OrderCardPage.ts
- `M` e2e/pages/TransfersPage.ts
- `M` e2e/tests/soft-delete-core.spec.ts
- `A` e2e/tests/soft-delete-transfers.spec.ts
- `M` e2e/tests/transfers-revert.spec.ts
- `M` web/ssr/_shared/js/deleted.js
- `M` web/ssr/_shared/js/task-card.js
- `M` web/ssr/_shared/js/transfers.js
- `M` web/ssr/_shared/js/ui-dialog.js
- `M` web/ssr/softdelete/templates/pages/list.html

### docs
- `A` docs/features/soft-delete-transfers/0-setup/draft-spec.slice-light.md
- `A` docs/features/soft-delete-transfers/0-setup/idioms.md
- `A` docs/features/soft-delete-transfers/0-setup/light-progress.md
- `A` docs/features/soft-delete-transfers/0-setup/passport.md
- `A` docs/features/soft-delete-transfers/0-setup/pipeline-state.md
- `A` docs/features/soft-delete-transfers/0-setup/timeline.jsonl
- `M` docs/plan.md

## Экстракты документов (вместо полного чтения)
### draft-spec.md (экстракт: шапка + DS + дельта + Impact + TS)
# Draft Spec: Мягкое удаление — документы перемещения и заказы с документами

**Tier:** M · **Совместимость:** MVP · **Режим:** new

**Дата:** 2026-10-08 | **Статус:** draft → final (после подтверждения пользователя) | **Tier:** M
**Совместимость:** MVP (DS-022) — рабочей версии с живыми пользователями нет: таблицы и адреса меняем свободно, без переходного периода и отката; данные на базе сохраняются — все существующие документы после миграции считаются действующими
**UI-scope:** swagger:no | adaptive:min | themes:both (DS-023)
**Источник:** мастер-спека `docs/features/soft-delete/draft-spec.md` (§4.2, §6.3, §6.4, §6.26, §8.4) + её журнал `docs/features/soft-delete/draft-spec-decisions.md` + as-is факты кода (`internal/features/transfers`, `internal/features/board/tasks_service.go`, `migrations/0009`, `0014`–`0017`)
**Часть 2 из 6 мастер-спеки `soft-delete`; зависит от: 1 (`soft-delete-core`).** Номера DS и TS — мастер-идентификаторы, не перенумерованы. Номера разделов — свои: §6.1 = мастер §6.3, §6.2 = мастер §6.4, §6.3 = строки мастера §6.26 про документы; §8.1 = мастер §8.4.

> После мержа этой части: документ перемещения в статусах «Заявка» и «Отменён» удаляется с причиной и восстанавливается из вкладки «Перемещения» раздела «Удалённые»; заказ удаляется с любыми документами — отказ «Сначала отмените перемещения» снят; документы удалённого заказа видны в «Перемещениях» с пометкой «заказ удалён» и доводятся до конца, новые по такому заказу не создаются. Переходное правило части 1 (DS-034) перестаёт действовать.

## §9. Решения (DS-index)
Строки — дословно из мастера, номера не меняются; включены только решения, действующие в части 2.

| DS | Формулировка | Источник | Статус |
|---|---|---|---|
| DS-003 | Удаляет и восстанавливает только администратор; раздел «Удалённые» и его API — только администратор; остальным — 403 «Нет доступа» | батч 1 (В-2) | closed |
| DS-006 | Заказ удаляется с любыми документами перемещения; отказ `TASK_HAS_TRANSFERS` снят; отменённые документы вместе с заказом больше не удаляются. Заменяет в части удаления заказа transfers DS-025, DS-050 и `docs/plan.md` D75, D78 | батч 1 (В-4) | closed |
| DS-007 | Документы удалённого заказа видны в «Перемещениях» с пометкой «заказ удалён» и доводятся до конца: принять, откатить, отменить; также отправить и править заявку; создать новый документ по удалённому заказу нельзя — 409 `TASK_DELETED` | батч 2 (В-6); «отправить и править заявку» — inferred | closed |
| DS-008 | Остатки и местоположение оборудования по документам перемещения не меняются ни при удалении, ни при восстановлении заказа, документа, клиента, единицы | батч 1 (В-4), батч 2 (В-6), transfers DS-025 | closed |
| DS-010 | Удалённый заказ не виден нигде, кроме раздела «Удалённые» и пометки у своих документов: доска, поиск, карточка, печать, «Остатки», «Выплаты», история оборудования, итоги | батч 2 (В-7) | closed |
| DS-011 | Один раздел «Удалённые» с вкладками по видам записей; в строке — что удалено, причина, кто, когда и кнопка «Восстановить»; фильтры — поиск, период, кто удалил | батч 2 (В-8), батч 4 (В-15); фильтры — inferred | closed |
| DS-012 | Удалённое хранится бессрочно; окончательного удаления и автоматической очистки нет | батч 3 (В-9) | closed |
| DS-018 | Документ перемещения удаляется только в «Заявка» и «Отменён» (теперь мягко); проведённый сначала откатывают и отменяют; восстановление заявки снова занимает её единицы, единица занята другим незавершённым документом — 409 `EQUIPMENT_IN_TRANSFER` | transfers DS-025 (закрыто пользователем); поведение восстановления заявки — inferred | closed |
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
| DS-034 | Переходное правило части 1 (до части 2): заказ с документом в «Заявка», «В пути», «Получено» по-прежнему не удаляется (409 `TASK_HAS_TRANSFERS`); отменённые документы с части 1 вместе с заказом не удаляются и остаются у удалённого заказа. Часть 2 снимает отказ (DS-006) | inferred — следствие разбивки DS-031 и DS-006 | closed |
| DS-036 | Служебная уборка заказа повторяет действующее безвозвратное удаление заказа: неотменённые документы — отказ, отменённые удаляются вместе с заказом; документы автотест убирает сам, как сейчас; работает для действующего и для удалённого заказа. Уточняет DS-027 | inferred | closed |
| DS-038 | Служебная уборка получает вид `transfers`: физически удаляет документ в «Заявка» или «Отменён», действующий или мягко удалённый; уборка заказа удаляет также его мягко удалённые документы. Дополняет DS-027, DS-036 | inferred | closed |
| DS-040 | Заменяет DS-037. Мягко удалённые документы перемещения не мешают сменить клиента заказа: запрет смены клиента (`TASK_CLIENT_LOCKED_BY_TRANSFERS`, `docs/plan.md` D84) считает только действующие документы. Если клиента заказа сменили, удалённый документ с прежним клиентом восстановить нельзя — 409 `TRANSFER_CLIENT_MISMATCH` «Клиент заказа изменён — восстановить перемещение нельзя» | батч 6 (В-21); код и текст ошибки — inferred | closed |

## §10. Открытые вопросы (status: open)
Открытых вопросов нет

## §11. Тестовые сценарии (черновик — вход для design-e2e-strategy, TD-6.8)
Оси UI-scope (DS-023): Swagger-сценариев нет; адаптив и темы раздела «Удалённые» и окна удаления покрыты TS-046, TS-047 части 1 — новая вкладка проверяется в них же расширением (отдельных сценариев нет). Номера TS — мастер-идентификаторы; включены только сценарии части 2. В TS-012 и TS-033 в этой части проверяются таблица `t:transfers` и адреса §6.1, §6.2. Сценарий TS-048 части 1 (отказ `TASK_HAS_TRANSFERS`) с этой частью теряет негатив-ветку и заменяется TS-014.

### Unit / Integration (Go)
- **TS-012.** Миграция: после применения все существующие строки 11 таблиц имеют `deleted_at IS NULL`; CHECK «три поля вместе» отклоняет частичное заполнение (части 1–6, каждая — свои таблицы) — DS-022, DS-030
  - негатив: UPDATE только `deleted_at` без `deleted_by` → ошибка CHECK
- **TS-014.** Удаление заказа с документами в `requested`, `in_transit`, `received`, `cancelled`: 200, документы не изменились, остатки и местоположение оборудования по документам не изменились (часть 2) — DS-006, DS-008
  - негатив: ответа 409 `TASK_HAS_TRANSFERS` больше нет
- **TS-015.** Документ удалённого заказа: `send`, `receive`, `unreceive`, `cancel`, `PUT` → как у обычного; в ответе `f:task_is_deleted` = true (часть 2) — DS-007
  - негатив: `POST /api/v1/transfers` с `task_id` удалённого заказа → 409 `TASK_DELETED`
- **TS-016.** Удаление документа: `requested` и `cancelled` → 200, строки на месте, у заявки `equipment.active_transfer_id` = NULL; `in_transit` и `received` → 409 со своим сообщением (часть 2) — DS-018
  - негатив: без `version` в теле → 400 `required`; устаревшая `version` → 409 `TRANSFER_VERSION_CONFLICT`
- **TS-017.** Восстановление заявки: единицы снова заняты документом; единица в другом незавершённом документе → 409 `EQUIPMENT_IN_TRANSFER`, документ остаётся удалённым (часть 2) — DS-018
  - негатив: остатки и местоположение не изменились
- **TS-018.** Гонка «создание документа ↔ удаление заказа»: исход детерминирован — либо 409 `TASK_DELETED`, либо документ создан у удалённого заказа; взаимной блокировки нет (часть 2) — DS-007, DS-024
  - негатив: частично созданного документа нет
- **TS-033.** Права на всех 22 адресах удаления и восстановления: executor → 403 «Нет доступа» до поиска записи, включая несуществующий идентификатор (части 1–6) — DS-003
  - негатив: без сессии → 401
- **TS-050.** Смена клиента заказа при мягко удалённом документе: `PUT /api/v1/tasks/:id` с другим `client_id` → 200, клиент сменён; последующее восстановление этого документа → 409 `TRANSFER_CLIENT_MISMATCH`, документ остаётся удалённым; без смены клиента документ восстанавливается (часть 2) — DS-040
  - негатив: при действующем документе смена клиента по-прежнему → 409 `TASK_CLIENT_LOCKED_BY_TRANSFERS`
- **TS-051.** Служебная уборка документов: `DELETE /api/v1/test/purge/transfers/:id` физически удаляет заявку и отменённый документ, в том числе мягко удалённые; документ «в пути» → 409; уборка заказа с мягко удалённым документом проходит, строки документа в базе нет (часть 2) — DS-038
  - негатив: без `TEST_PURGE_ENABLED=true` → 404

### E2E (Playwright)
- **TS-040.** Заказ с документами — экран §8.1 — DS-006, DS-007 (часть 2)
  - шаги: заказ с документом «в пути» → удалить заказ → «Перемещения»; проверка: документ в списке с бейджем «заказ удалён», «Принять» работает; негатив: в форме нового документа удалённого заказа нет в выборе
- **TS-041.** Удалить и восстановить документ — экраны §8.1, §8.2 — DS-018 (часть 2)
  - шаги: заявка → «Удалить» с причиной → «Удалённые» → «Перемещения» → «Восстановить»; проверка: документ снова в списке со своими строками; негатив: у документа «в пути» в окне удаления — «Перемещение в пути — сначала отмените его»

### Фикстуры и сиды (АКЦЕНТ — хронический пропуск)
- **TS-F3.** `e2e/fixtures/auth.ts`: новые помощники `softDelete(kind, id, reason)`, `restore(kind, id)`, `listDeleted(kind, filters)`; payload'ы типизировать wire-типами (§TD-6.2; сырые ключи невидимы гейтам) — DS-028
- **TS-F4.** Существующие спеки с проверками безвозвратного удаления и отказов по ссылкам (`e2e/tests/board.spec.ts`, `board-guards.spec.ts`, `clients.spec.ts`, `equipment.spec.ts`, `catalog.spec.ts`, `equipment-types.spec.ts`, `transfers.spec.ts`, `transfers-revert.spec.ts`, `transfers-partners.spec.ts`, `polish.spec.ts`): ожидания меняются — обязательная причина, отсутствие 409 по ссылкам; каждая часть обновляет свои спеки — DS-006, DS-014, DS-020
- **TS-F6.** Go live-тесты удаления (`internal/features/board/board_http_live_test.go`, `internal/features/transfers/transfers_partners_http_live_test.go`, `internal/features/clients`, `internal/features/catalog`): запросы DELETE получают тело с причиной; проверки «строки нет в базе» заменяются на «строка помечена удалённой» — DS-020, DS-028
- **TS-F7.** `e2e/fixtures/auth.ts`: `deleteTransfer` (строка 518) передаёт `{reason, version}` в теле вместо `?version=`; `removeTransferForCleanup` и `purgeTask` после приведения документа к «Отменён» убирают его служебным адресом `DELETE /api/v1/test/purge/transfers/:id` — иначе тестовые документы копятся в «Удалённых» и удерживают заказ от физической уборки; обновить в части 2, в той же фазе, что миграция — DS-038

## §Impact — точки касания существующего кода
Источник: `python3 ~/.claude/tools/impact_scan.py` (sample `internal/features/transfers`; термы `transfers`, `transfer_items`, `active_transfer_id`, `TASK_HAS_TRANSFERS`, `deleteTransfer`, `/api/v1/transfers`) + чтение запросов к `transfers` в пакетах `board` и `clients` (fff) + решения мастера.

| # | Класс | Что задеваем (file:line / объект) | Дельта фичи | Решение | DS |
|---|---|---|---|---|---|
| 1 | SHARED-DEP | `internal/features/transfers/handler.go:141`, `service.go:343` (`Service.Delete`), `repo.go:684` (`Repo.Delete`), `dto.go` (`TransferDeleteResponse`), `routes.go:24` | мягкое удаление; `version` из query в тело, новое поле `reason`; ответ получает `deleted_at`; новый `Restore` и маршрут `POST /:id/restore` | править | DS-018, DS-028 |
| 2 | SHARED-DEP | `internal/features/transfers/repo.go:120` (`List`), `:165` (`Get`), `:296` (`Head`), запросы блокировки документа в `Update`, `Send`, `Receive`, `Cancel`, `Unreceive` | условие `deleted_at IS NULL`; `task_is_deleted` в выборке (`LEFT JOIN tasks` на `:185`) | править | DS-026, DS-007 |
| 3 | SHARED-DEP | `internal/features/transfers/repo.go:1157` (`checkRefs`) | чтение `deleted_at` заказа под `FOR KEY SHARE`; удалён → `TASK_DELETED` | править | DS-007 |
| 4 | SHARED-DEP | `internal/features/transfers/dto.go` (`TransferResponse`, `TransferSummary`) | новое поле `task_is_deleted` рядом с `task_id` | править | DS-007 |
| 5 | ENUMERATION | `internal/features/transfers/errors.go`, `internal/features/board/errors.go` | новые коды `TASK_DELETED`, `TRANSFER_CLIENT_MISMATCH` (409); `NOT_DELETED` — из части 1; `TASK_HAS_TRANSFERS` перестаёт возвращаться при удалении заказа (остаётся в служебной уборке) | править | DS-006, DS-007 |
| 6 | SHARED-DEP | `internal/features/board/tasks_service.go:411`, `:422` (`deleteTaskTransfers`), вызов в `TaskService.Delete` | проверка и удаление документов при удалении заказа убираются; функция остаётся только для служебной уборки | править | DS-006, DS-036 |
| 7 | SHARED-DEP | `internal/features/board/tasks_service.go:380` (`clientLockedByTransfers`) | условие `deleted_at IS NULL`: считает только действующие документы; при восстановлении документа — сверка клиента документа с клиентом заказа (`TRANSFER_CLIENT_MISMATCH`) | править | DS-040 |
| 8 | SHARED-DEP | `internal/features/board` — читатели документов: блок перемещений карточки (`card_repo.go`), итог заказа (`tasks_repo.go`), «в пути» на остатках (`stock_repo.go`), соединение `equipment.active_transfer_id` → `transfers` | условие `transfers.deleted_at IS NULL` | править | DS-026 |
| 9 | SHARED-DEP | `internal/features/clients/history.go` (события истории по документам) | удалённые документы исключаются | править | DS-026 |
| 10 | SHARED-DEP | `internal/features/softdelete` (часть 1) | вид `transfers` в списке и счётчиках; вид `transfers` в служебной уборке; уборка заказа удаляет его мягко удалённые документы | править | DS-011, DS-038 |
| 11 | FK | `migrations/0009_stock_transfers.up.sql`, `0014_transfers_core.up.sql:53` (`equipment.active_transfer_id` SET NULL), `0015_transfers_partners.up.sql:101` (`task_id` RESTRICT) | новая миграция: три колонки, CHECK `transfers_deleted_chk`, FK `deleted_by`, индекс `transfers_deleted_at_idx`; существующие ключи не меняются | добавить | DS-018, DS-022 |
| 12 | FRONTEND | `web/ssr/transfers/templates/pages`, скрипт экрана «Перемещения»; `web/ssr/softdelete` (часть 1); `web/ssr/board` (окно удаления заказа) | «Удалить» через общее окно с причиной; бейдж «заказ удалён»; вкладка «Перемещения»; вторая строка пояснения и удаление ветки 409 | править | DS-007, DS-020 |
| 13 | ENUMERATION | `e2e/fixtures/auth.ts:518` (`deleteTransfer`), `removeTransferForCleanup`, `:590` (`purgeTask`); `e2e/tests/transfers.spec.ts`, `transfers-revert.spec.ts`, `transfers-partners.spec.ts` | тело запроса удаления; уборка через служебный адрес; ожидания по снятому отказу | править | DS-038, DS-006 |
| 14 | SHARED-DEP | Go live-тесты `internal/features/transfers/*_http_live_test.go`, `internal/features/board/order_card_revert_http_live_test.go` (проверки «отменённый документ удалён с заказом», «документов заказа нет») | ожидания меняются: документы остаются | править | DS-006, DS-018 |
| 15 | SHARED-DEP | `docs/plan.md` (D75, D78, D84), `docs/features/transfers/draft-spec.md` (DS-025, DS-050) | запись нового решения в `docs/plan.md` при реализации | добавить запись | DS-006 |
| 16 | CONFIG | env-ключи | не задеваются (`TEST_PURGE_ENABLED` — часть 1) | — | — |
| 17 | MONITORING | дашборды, nginx | не задеваются | — | — |

**Вне каталога фичи:** есть — internal/features/transfers, internal/features/board, internal/features/clients, internal/features/softdelete, migrations, web/ssr/transfers, web/ssr/softdelete, web/ssr/board, e2e, docs/plan.md

### prd.md
MISSING

## Инвентарь доков (точечный grep разрешён, полное чтение — нет)
- draft-spec.md — 87KB
- 0-setup/draft-spec.slice-light.md — 77KB
- 8-code-review/input-bundle.md — 31KB
- 0-setup/passport.md — 13KB
- 8-code-review/output.md — 12KB
- 0-setup/idioms.md — 8KB
- 0-setup/light-progress.md — 3KB
- 0-setup/pipeline-state.md — 0KB
- 9-validation/evidence/last-run.json — 0KB

## Паспорт (выжимка; полный — 0-setup/passport.md)
# Passport: soft-delete-transfers (generated — руками не править; stage_bundle.py)
**Tier:** M | **Совместимость:** MVP (pipeline-doctrine.md §CM-1; ? → уточни у оркестратора)
**Трек:** обычный (prefs нет — дефолт) — режимы агентов (`spec-track delta/micro/mini/merged`) включает ТОЛЬКО строка `Режим:` в prompt оркестратора; DS/TS-индексы ниже присутствуют на ЛЮБОМ треке при наличии draft-spec.
**Layout:** автодетект (tools/layout_resolve.py)
**UI-scope (pipeline-doctrine.md §US-1, шапка draft-spec):** swagger:no | adaptive:min | themes:both
