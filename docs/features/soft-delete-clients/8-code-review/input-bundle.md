# Review Bundle: soft-delete-clients — CHECKS-FAILED
**Base:** 801b3a59850eb4c738d5930f8c4aa244dfc13963 | **HEAD:** ad99362 | **Generated:** 2026-10-08 21:35 | diff: 58 files changed, 4989 insertions(+), 561 deletions(-)

> ℹ️ Go-каталоги фичи выведены из диффа (имя «soft-delete-clients» не резолвится в пакет): `internal/features/board`, `internal/features/clients`, `internal/features/softdelete`, `internal/features/transfers`

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
| e2e_drift | ❌ | 1 | e2e-drift: FAIL (режим: TS (spec-track §ST-1.4), doc: 1 id, code: 0 в 0 файлах, проблем: 1)<br>DRIFT: TS-043 есть в draft-spec.md, теста test('TS-043: …') нет (сценарий без теста) |

## Diff по линзам
| Линза | Файлов | Patch |
|---|---|---|
| backend | 31 | 8-code-review/diff-backend.patch |
| migrations | 2 | 8-code-review/diff-migrations.patch |
| other | 19 | 8-code-review/diff-other.patch |
| docs | 6 | — (артефакты, не ревьюятся построчно) |

### backend
- `M` internal/features/board/board_http_live_test.go
- `M` internal/features/board/card_dto.go
- `M` internal/features/board/card_repo.go
- `M` internal/features/board/equipment_location_http_live_test.go
- `M` internal/features/board/order_card_revert_http_live_test.go
- `M` internal/features/board/soft_delete_http_live_test.go
- `M` internal/features/board/subtasks_repo.go
- `M` internal/features/board/subtasks_service.go
- `M` internal/features/board/task_equipment_repo.go
- `M` internal/features/board/tasks_dto.go
- `M` internal/features/board/tasks_repo.go
- `M` internal/features/board/tasks_service.go
- `M` internal/features/clients/clients_http_live_test.go
- `A` internal/features/clients/clients_soft_delete_http_live_test.go
- `M` internal/features/clients/equipment_search.go
- `M` internal/features/clients/handler.go
- `M` internal/features/clients/history.go
- `A` internal/features/clients/migration_soft_delete_integration_test.go
- `M` internal/features/clients/repo.go
- `M` internal/features/clients/routes.go
- `M` internal/features/clients/service.go
- `A` internal/features/clients/soft_delete.go
- `M` internal/features/clients/ssr.go
- `A` internal/features/softdelete/clients_source.go
- `A` internal/features/softdelete/clients_source_test.go
- `M` internal/features/transfers/dto.go
- `M` internal/features/transfers/repo.go
- `M` internal/features/transfers/service.go
- `M` internal/seeddemo/seeddemo.go
- `M` internal/seeddemo/seeddemo_integration_test.go
- `M` internal/server/router.go

### migrations
- `A` migrations/0021_soft_delete_clients.down.sql
- `A` migrations/0021_soft_delete_clients.up.sql

### other
- `M` e2e/fixtures/auth.ts
- `M` e2e/fixtures/board-types.ts
- `M` e2e/pages/ClientsPage.ts
- `M` e2e/pages/DeletedPage.ts
- `M` e2e/pages/EquipmentPage.ts
- `M` e2e/tests/board-guards.spec.ts
- `M` e2e/tests/clients.spec.ts
- `M` e2e/tests/equipment.spec.ts
- `A` e2e/tests/soft-delete-clients.spec.ts
- `M` e2e/tests/transfers-partners.spec.ts
- `M` web/ssr/_shared/js/board.js
- `M` web/ssr/_shared/js/deleted.js
- `M` web/ssr/_shared/js/task-card.js
- `M` web/ssr/_shared/js/transfers.js
- `M` web/ssr/_shared/js/ui-dialog.js
- `M` web/ssr/board/templates/pages/board.html
- `M` web/ssr/clients/templates/pages/equipment-form.html
- `M` web/ssr/clients/templates/pages/form.html
- `M` web/ssr/clients/templates/pages/list.html

### docs
- `A` docs/features/soft-delete-clients/0-setup/draft-spec.slice-light.md
- `A` docs/features/soft-delete-clients/0-setup/idioms.md
- `A` docs/features/soft-delete-clients/0-setup/light-progress.md
- `A` docs/features/soft-delete-clients/0-setup/passport.md
- `A` docs/features/soft-delete-clients/0-setup/pipeline-state.md
- `A` docs/features/soft-delete-clients/0-setup/timeline.jsonl

## Экстракты документов (вместо полного чтения)
### draft-spec.md (экстракт: шапка + DS + дельта + Impact + TS)
# Draft Spec: Мягкое удаление — клиенты и оборудование клиентов

**Tier:** M · **Совместимость:** MVP · **Режим:** new

**Дата:** 2026-10-08 | **Статус:** draft → final (после подтверждения пользователя) | **Tier:** M
**Совместимость:** MVP (DS-022) — рабочей версии с живыми пользователями нет: таблицы и адреса меняем свободно, без переходного периода и отката; данные на базе сохраняются — все существующие клиенты и единицы после миграции считаются действующими
**UI-scope:** swagger:no | adaptive:min | themes:both (DS-023)
**Источник:** мастер-спека `docs/features/soft-delete/draft-spec.md` (§4.4, §4.5, §6.7–§6.10, §6.26, §8.3, §8.5) + её журнал `docs/features/soft-delete/draft-spec-decisions.md` + as-is факты кода (`internal/features/clients`, `internal/features/board/tasks_dto.go`, `internal/features/transfers/dto.go`, `migrations/0003_clients_equipment.up.sql`, `migrations/0014_transfers_core.up.sql`)
**Часть 4 из 6 мастер-спеки `soft-delete`; зависит от: 1 (`soft-delete-core`), 2 (`soft-delete-transfers`).** Номера DS и TS — мастер-идентификаторы, не перенумерованы. Номера разделов — свои: §6.1–§6.4 = мастер §6.7–§6.10, §6.5 = строки мастера §6.26 про клиентов и оборудование; §8.1 = часть мастера §8.5, §8.3 = часть мастера §8.3.

> После мержа этой части: клиент и единица оборудования удаляются с причиной при любых ссылках (заказы, завершённые документы) — кроме случая, когда оборудование на нашем складе, в пути или в незавершённом перемещении: сначала вернуть его клиенту — и восстанавливаются из вкладок «Клиенты» и «Оборудование» раздела «Удалённые»; оборудование уходит и возвращается вместе с клиентом; в действующих заказах и документах удалённые клиент и единицы показываются с бейджем «удалён», в новых — не выбираются. Переходное правило части 1 (DS-035) для клиентов и оборудования перестаёт действовать.

## §9. Решения (DS-index)
Строки — дословно из мастера, номера не меняются; включены только решения, действующие в части 4.

| DS | Формулировка | Источник | Статус |
|---|---|---|---|
| DS-003 | Удаляет и восстанавливает только администратор; раздел «Удалённые» и его API — только администратор; остальным — 403 «Нет доступа» | батч 1 (В-2) | closed |
| DS-008 | Остатки и местоположение оборудования по документам перемещения не меняются ни при удалении, ни при восстановлении заказа, документа, клиента, единицы | батч 1 (В-4), батч 2 (В-6), transfers DS-025 | closed |
| DS-010 | Удалённый заказ не виден нигде, кроме раздела «Удалённые» и пометки у своих документов: доска, поиск, карточка, печать, «Остатки», «Выплаты», история оборудования, итоги | батч 2 (В-7) | closed |
| DS-011 | Один раздел «Удалённые» с вкладками по видам записей; в строке — что удалено, причина, кто, когда и кнопка «Восстановить»; фильтры — поиск, период, кто удалил | батч 2 (В-8), батч 4 (В-15); фильтры — inferred | closed |
| DS-012 | Удалённое хранится бессрочно; окончательного удаления и автоматической очистки нет | батч 3 (В-9) | closed |
| DS-014 | Удалить можно всегда, ссылки не мешают: в действующих заказах и документах удалённая запись остаётся видна с пометкой «удалён», в новых выбрать её нельзя. Сняты отказы `CLIENT_HAS_TASKS`, `EQUIPMENT_HAS_TRANSFERS`, `EQUIPMENT_USED_IN_TASKS` и отказы «используется» каталога. Заменяет transfers DS-026 в части удаления единицы из справочника | батч 3 (В-10) | closed |
| DS-015 | Оборудование клиента уходит в удалённые и возвращается вместе с клиентом; причина наследуется от клиента, в списке — пометка «вместе с клиентом»; единицу удалённого клиента отдельно восстановить нельзя (409 `CLIENT_DELETED`) | батч 3 (В-10); наследование причины и запрет отдельного восстановления — inferred | closed |
| DS-016 | Заказ восстанавливается всегда: колонка удалена — встаёт в конец первой нефинальной действующей колонки; клиент или оборудование в удалённых — остаются с пометкой; обо всём — предупреждение | батч 3 (В-11); «нефинальной» — inferred (в финальную колонку нельзя с незавершёнными услугами) | closed |
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
| DS-032 | Существующие SSR-формы удаления (клиенты, оборудование, каталог, типы оборудования) принимают обязательное поле «Причина» и удаляют мягко | inferred из В-15 | closed |
| DS-033 | UI-конвенции: ошибка поля — под полем; тосты — правый верхний угол, 5 секунд; причина и подтверждения — `KT.uiDialog`; пометка «удалён» — серый бейдж; предупреждения восстановления — жёлтый блок над таблицей | inferred, transfers DS-052 | closed |
| DS-035 | Переходное правило части 1 (до частей 3 и 4): мягко удалённый заказ по-прежнему удерживает своего клиента, единицы оборудования и колонку от безвозвратного удаления — действующие отказы `CLIENT_HAS_TASKS`, `EQUIPMENT_USED_IN_TASKS`, `COLUMN_NOT_EMPTY` срабатывают и по удалённым заказам | inferred — следствие разбивки DS-031 | closed |
| DS-041 | Клиента и отдельную единицу оборудования нельзя удалить, пока оборудование на нашем складе или в пути: сначала вернуть его клиенту (409 `CLIENT_EQUIPMENT_AT_WAREHOUSE`, `EQUIPMENT_AT_WAREHOUSE`). Исключение из DS-014, по аналогии со складом с остатком (DS-021). Клиента нельзя удалить и пока его единица стоит в незавершённом документе (409 `CLIENT_EQUIPMENT_IN_TRANSFER`) — иначе документ привёз бы удалённую единицу на склад. Уточняет DS-015 | батч 6 (В-22); запрет при незавершённом документе, коды и тексты ошибок — inferred | closed |
| DS-045 | Заявка перемещения не восстанавливается, пока что-то из её состава в удалённых — единица оборудования, товар, склад или поставщик: 409 `TRANSFER_HAS_DELETED_REFS` «В перемещении есть удалённые записи — сначала восстановите их», `details.kind` и `details.id`. Отменённые документы не проверяются. Строки удалённой заявки не удерживают единицу, товар, склад и поставщика от удаления (следствие DS-014, DS-018, DS-044), поэтому без этой проверки восстановленная заявка привезла бы товар или оборудование на удалённый склад либо заняла удалённую единицу (нарушение DS-041). Проверка каждого вида вводится его частью: единица — часть 4, товар — часть 5, склад и поставщик — часть 6 | inferred (следствие DS-041, DS-044; по аналогии с DS-043) | closed |

## §10. Открытые вопросы (status: open)
Открытых вопросов нет

## §11. Тестовые сценарии (черновик — вход для design-e2e-strategy, TD-6.8)
Оси UI-scope (DS-023): Swagger-сценариев нет; адаптив и темы раздела «Удалённые» и окна удаления покрыты TS-046, TS-047 части 1 (новые вкладки и бейдж «удалён» проверяются в них же). Номера TS — мастер-идентификаторы; включены только сценарии части 4. В TS-012 и TS-033 в этой части проверяются таблицы `t:clients`, `t:equipment` и адреса §6.1–§6.4; в TS-026 — узлы клиента и единицы (услуга каталога и товар — часть 5). В TS-022 и TS-043 оборудование удаляемого клиента находится у клиента; случаи «на складе / в пути / в заявке» — TS-053; удалённая заявка и удаление её единицы — TS-058. Сценарий TS-049 части 1 в части про клиента (`CLIENT_HAS_TASKS`) с этой частью перестаёт действовать и заменяется TS-022.

### Unit / Integration (Go)
- **TS-012.** Миграция: после применения все существующие строки 11 таблиц имеют `deleted_at IS NULL`; CHECK «три поля вместе» отклоняет частичное заполнение (части 1–6, каждая — свои таблицы) — DS-022, DS-030
  - негатив: UPDATE только `deleted_at` без `deleted_by` → ошибка CHECK
- **TS-022.** Удаление клиента с заказами и документами: 200, клиент и все его действующие единицы удалены с одной причиной, `f:deleted_with_client` = true, `f:equipment_deleted` верен; заказы клиента на доске с `f:client_is_deleted` = true (часть 4) — DS-014, DS-015
  - негатив: ответов 409 `CLIENT_HAS_TASKS`, `EQUIPMENT_HAS_TRANSFERS` больше нет
- **TS-023.** Восстановление клиента: возвращаются он и единицы с `f:deleted_with_client` = true; единица, удалённая раньше отдельно, остаётся удалённой (часть 4) — DS-015
  - негатив: восстановление единицы удалённого клиента → 409 `CLIENT_DELETED`
- **TS-024.** Удаление единицы: с завершёнными документами и заказами → 200; в незавершённом документе → 409 `EQUIPMENT_IN_TRANSFER`; `f:location` и `f:warehouse_id` не изменились (часть 4) — DS-014, DS-008
  - негатив: добавление удалённой единицы в заказ → 400 `equipment_id: not_found`
- **TS-025.** Выбор удалённых: `POST /api/v1/tasks` с удалённым клиентом → 400 `client_id: not_found`; списки `GET /api/v1/clients`, `GET /api/v1/equipment` удалённых не содержат (часть 4) — DS-014
  - негатив: `GET /api/v1/clients/:id` удалённого → 404
- **TS-026.** Пометки в карточке: у действующего заказа с удалённым клиентом, единицей, услугой каталога, товаром в `GET /api/v1/tasks/:id/card` — `f:client_is_deleted`, `f:is_deleted` (единица), `f:service_is_deleted`, `f:product_is_deleted` = true (части 4, 5) — DS-014
  - негатив: у действующих записей все четыре признака = false
- **TS-033.** Права на всех 22 адресах удаления и восстановления: executor → 403 «Нет доступа» до поиска записи, включая несуществующий идентификатор (части 1–6) — DS-003
  - негатив: без сессии → 401
- **TS-053.** Оборудование не у клиента: удаление клиента с единицей «на складе» → 409 `CLIENT_EQUIPMENT_AT_WAREHOUSE` с `details.equipment_ids`; с единицей «в пути» → тот же отказ; с единицей в заявке → 409 `CLIENT_EQUIPMENT_IN_TRANSFER` с `details.transfer_ids`; удаление отдельной единицы «на складе» → 409 `EQUIPMENT_AT_WAREHOUSE`; после возврата оборудования клиенту документом удаление проходит (часть 4) — DS-041
  - негатив: при отказе ни клиент, ни одна его единица не помечены удалёнными
- **TS-058.** Удалённая заявка не удерживает единицу, а удалённая единица не даёт восстановить заявку: заявка удалена → её единица (у клиента) удаляется из справочника → 200; `POST /api/v1/transfers/:id/restore` → 409 `TRANSFER_HAS_DELETED_REFS` с `details.kind` = `equipment`; после восстановления единицы заявка восстанавливается (часть 4) — DS-045, DS-041
  - негатив: отменённый документ с удалённой единицей восстанавливается — 200

### E2E (Playwright)
- **TS-043.** Клиент с оборудованием — экраны §8.1, §8.2, §8.3 — DS-014, DS-015 (часть 4)
  - шаги: удалить клиента с заказом и двумя единицами → открыть заказ → «Удалённые»; проверка: в заказе бейдж «удалён» у клиента и единиц; во вкладке «Оборудование» единицы с бейджем «вместе с клиентом»; восстановление клиента возвращает всё; негатив: «Восстановить» у единицы удалённого клиента → «Сначала восстановите клиента»

### Фикстуры и сиды (АКЦЕНТ — хронический пропуск)
- **TS-F3.** `e2e/fixtures/auth.ts`: новые помощники `softDelete(kind, id, reason)`, `restore(kind, id)`, `listDeleted(kind, filters)`; payload'ы типизировать wire-типами (§TD-6.2; сырые ключи невидимы гейтам) — DS-028
- **TS-F4.** Существующие спеки с проверками безвозвратного удаления и отказов по ссылкам (`e2e/tests/board.spec.ts`, `board-guards.spec.ts`, `clients.spec.ts`, `equipment.spec.ts`, `catalog.spec.ts`, `equipment-types.spec.ts`, `transfers.spec.ts`, `transfers-revert.spec.ts`, `transfers-partners.spec.ts`, `polish.spec.ts`): ожидания меняются — обязательная причина, отсутствие 409 по ссылкам; каждая часть обновляет свои спеки — DS-006, DS-014, DS-020
- **TS-F6.** Go live-тесты удаления (`internal/features/board/board_http_live_test.go`, `internal/features/transfers/transfers_partners_http_live_test.go`, `internal/features/clients`, `internal/features/catalog`): запросы DELETE получают тело с причиной; проверки «строки нет в базе» заменяются на «строка помечена удалённой» — DS-020, DS-028
- **TS-F9.** Клиенты и оборудование в автотестах: удаление идёт через страницы (`e2e/tests/clients.spec.ts`, `equipment.spec.ts`, `e2e/pages/EquipmentPage.ts`) — в шаги добавляется ввод причины в окне; в `e2e/fixtures/auth.ts` помощников удаления клиента и единицы нет — добавляется общий `purge(kind, id)` на служебный адрес, им убираются созданные тестами клиенты и единицы; обновить в части 4 — DS-027, DS-020

## §Impact — точки касания существующего кода
Источник: `python3 ~/.claude/tools/impact_scan.py` (sample `internal/features/clients`; термы `clients`, `equipment`, `client_phones`, `CLIENT_HAS_TASKS`, `EQUIPMENT_HAS_TRANSFERS`, `EQUIPMENT_USED_IN_TASKS`, `/api/v1/clients`, `/api/v1/equipment`) + поиск запросов к `clients` и `equipment` вне пакета `clients` (fff) + решения мастера.

| # | Класс | Что задеваем (file:line / объект) | Дельта фичи | Решение | DS |
|---|---|---|---|---|---|
| 1 | SHARED-DEP | `internal/features/clients/service.go:174` (`Delete`), `:336` (`DeleteEquipment`), `handler.go:73`, `:128` (ответы 204), `routes.go:21`, `:30` | мягкое удаление с причиной; ответ 200 с телом вместо 204; новые `Restore`, `RestoreEquipment` и маршруты `POST /:id/restore` | править | DS-014, DS-015, DS-028 |
| 2 | SHARED-DEP | `internal/features/clients/repo.go:339` (`DeleteClient`), `:519` (`DeleteEquipment`), `:65` (`mapEquipmentFKViolation`) | DELETE → UPDATE; проверки заказов и документов и разбор нарушений внешних ключей убираются; удаление клиента — транзакция «клиент + единицы» | править | DS-014, DS-015 |
| 3 | SHARED-DEP | `internal/features/clients/repo.go:148` (`ListClients`), `:238` (`GetClient`), `:380` (`ClientExists`), `:408` (`ListEquipment`), `:435` (`GetEquipment`), поиск единиц (`SearchEquipment`), `history.go` | условие `deleted_at IS NULL`; `equipment_count` — только действующие | править | DS-014, DS-026 |
| 4 | SHARED-DEP | `internal/features/clients/ssr.go:178` (и обработчик удаления единицы), шаблоны `web/ssr/clients` | поле формы `reason`; повторный показ окна при ошибке причины; страница 404 для удалённых | править | DS-032 |
| 5 | SHARED-DEP | `internal/features/board/tasks_repo.go:267` (проверка клиента), `:307`, `:342`, `:403` (соединения с `clients`), `card_repo.go:330` | проверка «клиент не удалён» + `FOR KEY SHARE`; признак `client_is_deleted` в выборках | править | DS-014 |
| 6 | SHARED-DEP | `internal/features/board/tasks_dto.go:14` (`TaskCard`), DTO `CardEquipment` карточки заказа; `card_repo.go:66`; `task_equipment_repo.go:20`, `:49`, `:143` | новые поля `client_is_deleted`, `is_deleted`; добавление удалённой единицы в заказ → `not_found` | править | DS-014 |
| 7 | SHARED-DEP | `internal/features/board/subtask_products_repo.go:61`, `task_equipment_repo.go:119` (соединения услуг с `equipment`), проверка `equipment_id` при создании и правке услуги | назначение удалённой единицы услуге → `not_found`; чтение уже назначенной — без условия | править | DS-014 |
| 8 | SHARED-DEP | `internal/features/board/tasks_service.go` (`Restore` части 1) | `notices`: `client_deleted`, `equipment_deleted` | править | DS-016 |
| 9 | SHARED-DEP | `internal/features/transfers/dto.go` (`TransferResponse`, `TransferSummary`, `TransferItemResponse:159`), `repo.go:120`, `:165`, проверка единиц строк документа и занятие единицы | новые поля `client_is_deleted`, `equipment_is_deleted`; удалённая единица в новой строке → `not_found`; занятие единицы проверяет `deleted_at IS NULL` | править | DS-014 |
| 10 | ENUMERATION | `internal/features/clients/errors.go`, `internal/features/board/errors.go` | новые коды `CLIENT_DELETED`, `CLIENT_EQUIPMENT_AT_WAREHOUSE`, `CLIENT_EQUIPMENT_IN_TRANSFER`, `EQUIPMENT_AT_WAREHOUSE` (409); в пакете `transfers` — `TRANSFER_HAS_DELETED_REFS` (409, восстановление заявки с удалённой единицей); `CLIENT_HAS_TASKS`, `EQUIPMENT_HAS_TRANSFERS`, `EQUIPMENT_USED_IN_TASKS` перестают возвращаться продуктовыми адресами (остаются в служебной уборке); `EQUIPMENT_IN_TRANSFER` начинает возвращаться удалением единицы | править | DS-014, DS-015 |
| 11 | SHARED-DEP | `internal/features/softdelete` (часть 1) | виды `clients`, `equipment` в списке, счётчиках и служебной уборке; `deleted_with` | править | DS-011, DS-027 |
| 12 | FK | `migrations/0003_clients_equipment.up.sql` (`clients`, `equipment`), `0014_transfers_core.up.sql:50` | новая миграция: три поля у `clients`, четыре у `equipment`, CHECK, FK `deleted_by`, частичные индексы; существующие ключи не меняются | добавить | DS-014, DS-015, DS-022 |
| 13 | FRONTEND | `web/ssr/clients`, `web/ssr/board`, `web/ssr/transfers`, `web/ssr/softdelete` (часть 1) | окно с причиной в формах клиента и единицы; бейджи «удалён»; вкладки «Клиенты», «Оборудование» | править | DS-020, DS-014 |
| 14 | ENUMERATION | `e2e/fixtures/auth.ts` (помощников удаления клиента и единицы сейчас нет — добавляется общий `purge(kind, id)`), `e2e/pages/EquipmentPage.ts`, `e2e/tests/clients.spec.ts`, `equipment.spec.ts`, `equipment-warehouses.spec.ts` | причина в удалении; уборка служебным адресом; ожидания по снятым отказам | править | DS-027, DS-014 |
| 15 | SHARED-DEP | Go live-тесты `internal/features/clients/clients_http_live_test.go` (`:484` «единиц не осталось», `:930` прямой DELETE), `internal/features/board/order_card_revert_http_live_test.go:262` | ожидания: строки помечены, отказы по ссылкам сняты | править | DS-014 |
| 16 | CONFIG | env-ключи | не задеваются | — | — |
| 17 | MONITORING | дашборды, nginx | не задеваются | — | — |

**Вне каталога фичи:** есть — internal/features/clients, internal/features/board, internal/features/transfers, internal/features/softdelete, migrations, web/ssr/clients, web/ssr/board, web/ssr/transfers, web/ssr/softdelete, e2e

### prd.md
MISSING

## Инвентарь доков (точечный grep разрешён, полное чтение — нет)
- draft-spec.md — 102KB
- 0-setup/draft-spec.slice-light.md — 91KB
- 8-code-review/input-bundle.md — 33KB
- 0-setup/passport.md — 15KB
- 8-code-review/output.md — 13KB
- 0-setup/idioms.md — 10KB
- 0-setup/light-progress.md — 3KB
- 0-setup/pipeline-state.md — 0KB
- 9-validation/evidence/last-run.json — 0KB

## Паспорт (выжимка; полный — 0-setup/passport.md)
# Passport: soft-delete-clients (generated — руками не править; stage_bundle.py)
**Tier:** M | **Совместимость:** MVP (pipeline-doctrine.md §CM-1; ? → уточни у оркестратора)
**Трек:** обычный (prefs нет — дефолт) — режимы агентов (`spec-track delta/micro/mini/merged`) включает ТОЛЬКО строка `Режим:` в prompt оркестратора; DS/TS-индексы ниже присутствуют на ЛЮБОМ треке при наличии draft-spec.
**Layout:** автодетект (tools/layout_resolve.py)
**UI-scope (pipeline-doctrine.md §US-1, шапка draft-spec):** swagger:no | adaptive:min | themes:both
