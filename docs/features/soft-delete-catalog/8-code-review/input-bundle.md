# Review Bundle: soft-delete-catalog — CHECKS-FAILED
**Base:** 7074cd183fd46ef4d7b6ec0aa806479bf7847a50 | **HEAD:** 074886c | **Generated:** 2026-10-08 22:57 | diff: 89 files changed, 7154 insertions(+), 718 deletions(-)

> ℹ️ Go-каталоги фичи выведены из диффа (имя «soft-delete-catalog» не резолвится в пакет): `internal/features/board`, `internal/features/catalog`, `internal/features/clients`, `internal/features/equipmenttypes`, `internal/features/softdelete`, `internal/features/transfers`

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
| e2e_drift | ❌ | 1 | e2e-drift: FAIL (режим: TS (spec-track §ST-1.4), doc: 1 id, code: 0 в 0 файлах, проблем: 1)<br>DRIFT: TS-044 есть в draft-spec.md, теста test('TS-044: …') нет (сценарий без теста) |

## Diff по линзам
| Линза | Файлов | Patch |
|---|---|---|
| backend | 58 | 8-code-review/diff-backend.patch |
| migrations | 2 | 8-code-review/diff-migrations.patch |
| other | 23 | 8-code-review/diff-other.patch |
| docs | 6 | — (артефакты, не ревьюятся построчно) |

### backend
- `M` internal/features/board/card_dto.go
- `M` internal/features/board/card_repo.go
- `M` internal/features/board/errors.go
- `A` internal/features/board/soft_delete_catalog_http_live_test.go
- `M` internal/features/board/stock_dto.go
- `M` internal/features/board/stock_repo.go
- `M` internal/features/board/stock_service.go
- `A` internal/features/board/stock_sql_test.go
- `M` internal/features/board/subtask_products_repo.go
- `M` internal/features/board/subtasks_repo.go
- `M` internal/features/catalog/catalog.go
- `M` internal/features/catalog/catalog_http_live_test.go
- `A` internal/features/catalog/catalog_soft_delete_http_live_test.go
- `M` internal/features/catalog/categories_handler.go
- `M` internal/features/catalog/categories_repo.go
- `M` internal/features/catalog/categories_service.go
- `M` internal/features/catalog/categories_validate.go
- `A` internal/features/catalog/migration_soft_delete_integration_test.go
- `M` internal/features/catalog/products_dto.go
- `M` internal/features/catalog/products_handler.go
- `M` internal/features/catalog/products_repo.go
- `M` internal/features/catalog/products_service.go
- `M` internal/features/catalog/routes.go
- `M` internal/features/catalog/services_dto.go
- `M` internal/features/catalog/services_handler.go
- `M` internal/features/catalog/services_http_live_test.go
- `M` internal/features/catalog/services_repo.go
- `M` internal/features/catalog/services_service.go
- `A` internal/features/catalog/soft_delete.go
- `M` internal/features/catalog/ssr.go
- `M` internal/features/catalog/ssr_categories.go
- `A` internal/features/catalog/ssr_options.go
- `M` internal/features/catalog/ssr_products.go
- `M` internal/features/catalog/ssr_services.go
- `M` internal/features/clients/clients_http_live_test.go
- `M` internal/features/clients/dto.go
- `M` internal/features/clients/repo.go
- `M` internal/features/clients/service.go
- `M` internal/features/clients/ssr.go
- `M` internal/features/equipmenttypes/equipmenttypes_http_live_test.go
- `A` internal/features/equipmenttypes/equipmenttypes_soft_delete_http_live_test.go
- `M` internal/features/equipmenttypes/handler.go
- `M` internal/features/equipmenttypes/repo.go
- `M` internal/features/equipmenttypes/routes.go
- `M` internal/features/equipmenttypes/service.go
- `A` internal/features/equipmenttypes/soft_delete.go
- `M` internal/features/equipmenttypes/ssr.go
- `A` internal/features/softdelete/catalog_source.go
- `A` internal/features/softdelete/catalog_source_test.go
- `A` internal/features/softdelete/delete_request_test.go
- `M` internal/features/softdelete/reason.go
- `M` internal/features/softdelete/softdelete_http_live_test.go
- `M` internal/features/transfers/dto.go
- `M` internal/features/transfers/repo.go
- `A` internal/features/transfers/transfers_soft_delete_catalog_http_live_test.go
- `M` internal/seeddemo/seeddemo.go
- `M` internal/seeddemo/seeddemo_integration_test.go
- `M` internal/server/router.go

### migrations
- `A` migrations/0022_soft_delete_catalog.down.sql
- `A` migrations/0022_soft_delete_catalog.up.sql

### other
- `M` e2e/fixtures/auth.ts
- `M` e2e/fixtures/board-types.ts
- `M` e2e/fixtures/test.ts
- `M` e2e/pages/CatalogPage.ts
- `M` e2e/pages/DeletedPage.ts
- `M` e2e/pages/EquipmentTypesPage.ts
- `M` e2e/pages/StockPage.ts
- `M` e2e/tests/board-guards.spec.ts
- `M` e2e/tests/catalog.spec.ts
- `M` e2e/tests/equipment-types.spec.ts
- `A` e2e/tests/soft-delete-catalog.spec.ts
- `M` web/ssr/_shared/js/deleted.js
- `M` web/ssr/_shared/js/stock.js
- `M` web/ssr/_shared/js/task-card.js
- `M` web/ssr/_shared/js/transfers.js
- `M` web/ssr/catalog/templates/pages/products-form.html
- `M` web/ssr/catalog/templates/pages/products-list.html
- `M` web/ssr/catalog/templates/pages/services-form.html
- `M` web/ssr/catalog/templates/pages/services-list.html
- `M` web/ssr/clients/templates/pages/equipment-form.html
- `M` web/ssr/clients/templates/pages/form.html
- `M` web/ssr/equipmenttypes/templates/pages/form.html
- `M` web/ssr/equipmenttypes/templates/pages/list.html

### docs
- `A` docs/features/soft-delete-catalog/0-setup/draft-spec.slice-light.md
- `A` docs/features/soft-delete-catalog/0-setup/idioms.md
- `A` docs/features/soft-delete-catalog/0-setup/light-progress.md
- `A` docs/features/soft-delete-catalog/0-setup/passport.md
- `A` docs/features/soft-delete-catalog/0-setup/pipeline-state.md
- `A` docs/features/soft-delete-catalog/0-setup/timeline.jsonl

## Экстракты документов (вместо полного чтения)
### draft-spec.md (экстракт: шапка + DS + дельта + Impact + TS)
# Draft Spec: Мягкое удаление — каталог (товары, услуги, категории) и типы оборудования

**Tier:** M · **Совместимость:** MVP · **Режим:** new

**Дата:** 2026-10-08 | **Статус:** draft → final (после подтверждения пользователя) | **Tier:** M
**Совместимость:** MVP (DS-022) — рабочей версии с живыми пользователями нет: таблицы и адреса меняем свободно, без переходного периода и отката; данные на базе сохраняются — все существующие записи каталога после миграции считаются действующими, признак архива не меняется
**UI-scope:** swagger:no | adaptive:min | themes:both (DS-023)
**Источник:** мастер-спека `docs/features/soft-delete/draft-spec.md` (§4.6–§4.9, §6.11–§6.18, §6.26, §8.5) + её журнал `docs/features/soft-delete/draft-spec-decisions.md` + as-is факты кода (`internal/features/catalog`, `internal/features/equipmenttypes`, `internal/features/board/card_dto.go`, `migrations/0003_clients_equipment.up.sql`, `0004_catalog.up.sql`, `0008_warehouses.up.sql`)
**Часть 5 из 6 мастер-спеки `soft-delete`; зависит от: 1 (`soft-delete-core`).** Номера DS и TS — мастер-идентификаторы, не перенумерованы. Номера разделов — свои: §6.1–§6.8 = мастер §6.11–§6.18, §6.9 = строки мастера §6.26 про каталог; §8.1 = часть мастера §8.5.

> После мержа этой части: товар, услуга каталога, категория и тип оборудования удаляются с причиной при любых ссылках — отказы «используется — отправьте в архив» сняты — и восстанавливаются из вкладок «Товары», «Услуги», «Категории», «Типы оборудования» раздела «Удалённые». «В архив» остаётся отдельным действием. Товар с остатком на складе не удаляется. Название удалённой категории или типа свободно.

## §9. Решения (DS-index)
Строки — дословно из мастера, номера не меняются; включены только решения, действующие в части 5.

| DS | Формулировка | Источник | Статус |
|---|---|---|---|
| DS-003 | Удаляет и восстанавливает только администратор; раздел «Удалённые» и его API — только администратор; остальным — 403 «Нет доступа» | батч 1 (В-2) | closed |
| DS-009 | Архив и удаление — два независимых состояния: «В архив» остаётся как есть; удалённая запись возвращается с прежним признаком архива | батч 2 (В-5) | closed |
| DS-011 | Один раздел «Удалённые» с вкладками по видам записей; в строке — что удалено, причина, кто, когда и кнопка «Восстановить»; фильтры — поиск, период, кто удалил | батч 2 (В-8), батч 4 (В-15); фильтры — inferred | closed |
| DS-012 | Удалённое хранится бессрочно; окончательного удаления и автоматической очистки нет | батч 3 (В-9) | closed |
| DS-014 | Удалить можно всегда, ссылки не мешают: в действующих заказах и документах удалённая запись остаётся видна с пометкой «удалён», в новых выбрать её нельзя. Сняты отказы `CLIENT_HAS_TASKS`, `EQUIPMENT_HAS_TRANSFERS`, `EQUIPMENT_USED_IN_TASKS` и отказы «используется» каталога. Заменяет transfers DS-026 в части удаления единицы из справочника | батч 3 (В-10) | closed |
| DS-017 | Название удалённой записи свободно: уникальность — только среди действующих (категории, типы оборудования, склады, поставщики); восстановление при занятом названии — 409 `NAME_TAKEN` «Название занято — переименуйте действующую запись» | батч 3 (В-12) | closed |
| DS-020 | Причина удаления обязательна для любого вида записи: 1–500 символов после обрезки пробелов; без неё удалить нельзя | батч 4 (В-15); границы длины — inferred | closed |
| DS-021 | Не удаляются: товар с ненулевым остатком на любом складе (409 `PRODUCT_HAS_STOCK`); склад с товаром, оборудованием или незавершёнными перемещениями (409 `WAREHOUSE_IN_USE`); последний активный склад (409 `WAREHOUSE_LAST_ACTIVE`) | батч 4 (В-16); последний активный склад — inferred из кода (`internal/features/warehouses`) | closed |
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
| DS-042 | Остаток, появившийся у удалённого товара или на удалённом складе позже (возврат из заказа, приём документа), виден в отчёте «Остатки» с пометкой «удалён»; при нулевом остатке удалённые товар и склад в отчёте не показываются. Ручная корректировка такого остатка разрешена — чтобы его списать | батч 6 (В-23); разрешение корректировки — inferred | closed |
| DS-045 | Заявка перемещения не восстанавливается, пока что-то из её состава в удалённых — единица оборудования, товар, склад или поставщик: 409 `TRANSFER_HAS_DELETED_REFS` «В перемещении есть удалённые записи — сначала восстановите их», `details.kind` и `details.id`. Отменённые документы не проверяются. Строки удалённой заявки не удерживают единицу, товар, склад и поставщика от удаления (следствие DS-014, DS-018, DS-044), поэтому без этой проверки восстановленная заявка привезла бы товар или оборудование на удалённый склад либо заняла удалённую единицу (нарушение DS-041). Проверка каждого вида вводится его частью: единица — часть 4, товар — часть 5, склад и поставщик — часть 6 | inferred (следствие DS-041, DS-044; по аналогии с DS-043) | closed |
| DS-046 | Дополняет DS-045. Отмена проведения документа перемещения отклоняется, если его единица оборудования, товар или склад в удалённых: 409 `TRANSFER_HAS_DELETED_REFS`, `details.kind` и `details.id`. Откат вернул бы документ в «В пути»: удалённая единица оказалась бы в пути (нарушение DS-041), а остаток удалённого товара или на удалённом складе изменился бы. Достижимый случай для единицы — документ «склад → клиент» получен, затем единицу или клиента удалили. Удалённый поставщик отмене проведения не мешает: он не держит ни остатка, ни местоположения. Проверка вводится по частям: единица — часть 4, товар — часть 5, склад — часть 6 | оркестратор по ревью части 4 (единица; пользователь уведомлён); товар, склад и исключение для поставщика — inferred | closed |

## §10. Открытые вопросы (status: open)
Открытых вопросов нет

## §11. Тестовые сценарии (черновик — вход для design-e2e-strategy, TD-6.8)
Оси UI-scope (DS-023): Swagger-сценариев нет; адаптив и темы раздела «Удалённые» и окна удаления покрыты TS-046, TS-047 части 1 (новые вкладки и бейджи проверяются в них же). Номера TS — мастер-идентификаторы; включены только сценарии части 5. В TS-012 и TS-033 в этой части проверяются таблицы `t:products`, `t:services`, `t:catalog_categories`, `t:equipment_types` и адреса §6.1–§6.8; в TS-026 — признаки услуги каталога и товара (клиент и единица — часть 4). В TS-027 негатив «приход на удалённый товар → 400» относится к новым строкам документов и заказов; остаток, пришедший по существующим ссылкам, — TS-054. В TS-059 и TS-060 в этой части проверяется вид `product`.

### Unit / Integration (Go)
- **TS-012.** Миграция: после применения все существующие строки 11 таблиц имеют `deleted_at IS NULL`; CHECK «три поля вместе» отклоняет частичное заполнение (части 1–6, каждая — свои таблицы) — DS-022, DS-030
  - негатив: UPDATE только `deleted_at` без `deleted_by` → ошибка CHECK
- **TS-026.** Пометки в карточке: у действующего заказа с удалённым клиентом, единицей, услугой каталога, товаром в `GET /api/v1/tasks/:id/card` — `f:client_is_deleted`, `f:is_deleted` (единица), `f:service_is_deleted`, `f:product_is_deleted` = true (части 4, 5) — DS-014
  - негатив: у действующих записей все четыре признака = false
- **TS-027.** Удаление товара: нулевой остаток на всех складах → 200; ненулевой хотя бы на одном → 409 `PRODUCT_HAS_STOCK`; товар в заказах удалению не мешает (часть 5) — DS-021, DS-014
  - негатив: приход на удалённый товар → 400 `product_id: not_found`
- **TS-028.** Архив и удаление независимы: архивная запись удаляется и восстанавливается архивной (`f:is_active` = false); активная — активной; список справочника с фильтром «архив» удалённых не содержит (часть 5) — DS-009
  - негатив: в `GET /api/v1/deleted` у архивной `f:is_archived` = true
- **TS-029.** Название свободно: после удаления категории создаётся новая с тем же названием и видом; восстановление старой → 409 `NAME_TAKEN`; после переименования новой — восстановление 200 (часть 5) — DS-017
  - негатив: две действующие категории одного вида с одним названием по-прежнему невозможны
- **TS-030.** Тип оборудования и услуга каталога: удаление при наличии единиц и услуг заказов → 200; выбор удалённого типа в единице → 400 `type_id: not_found`; удалённой услуги в заказе → 400 `service_id: not_found` (часть 5) — DS-014
  - негатив: существующие единицы и услуги заказов не изменились
- **TS-033.** Права на всех 22 адресах удаления и восстановления: executor → 403 «Нет доступа» до поиска записи, включая несуществующий идентификатор (части 1–6) — DS-003
  - негатив: без сессии → 401
- **TS-054.** Остаток на удалённом товаре: товар удалён при нулевом остатке, затем заказ с этим товаром уходит из списывающей колонки → в `GET /api/v1/stock` строка товара с `f:is_deleted` = true и возвращённым остатком; ручная корректировка до нуля → 200, строка исчезает из отчёта (часть 5) — DS-042
  - негатив: удалённый товар с нулевым остатком в отчёте не показывается; добавить его в новый заказ нельзя — 400 `product_id: not_found`
- **TS-059.** Восстановление заявки с удалённым товаром, складом или поставщиком → 409 `TRANSFER_HAS_DELETED_REFS` с соответствующим `details.kind` (`product`, `warehouse`, `supplier`); после восстановления записи заявка восстанавливается (товар — часть 5; склад и поставщик — часть 6) — DS-045
  - негатив: отменённый документ с удалёнными товаром, складом, поставщиком восстанавливается — 200
- **TS-060.** Отмена проведения документа с удалённым товаром или складом: `POST /api/v1/transfers/:id/unreceive` → 409 `TRANSFER_HAS_DELETED_REFS` с `details.kind` = `product` либо `warehouse`, документ остаётся «Получено», остатки не изменились; после восстановления записи отмена проведения проходит; документ удалённого поставщика при действующих товаре и складе откатывается — 200 (товар — часть 5; склад и поставщик — часть 6) — DS-046
  - негатив: приём документа «В пути» с удалённым товаром по-прежнему проходит — остаток виден с пометкой (DS-042)
- **TS-061.** Счётчик единиц типа оборудования: `f:equipment_count` в `GET /api/v1/equipment-types` считает только действующие единицы — тип с единственной мягко удалённой единицей показывает 0; такой тип удаляется (`DELETE /api/v1/equipment-types/:id` → 200) без `EQUIPMENT_TYPE_IN_USE` (часть 5) — DS-014
  - негатив: после восстановления единицы счётчик типа снова 1; удалённый тип у неё показан с `f:type_is_deleted` = true

### E2E (Playwright)
- **TS-044.** Товар и категория — экраны §8.1, §8.2 — DS-009, DS-017, DS-021 (часть 5)
  - шаги: удалить категорию → создать новую с тем же названием → восстановить старую; проверка: «Название занято — переименуйте действующую запись»; архивный товар после удаления и восстановления остаётся в архиве; негатив: товар с остатком — «У товара есть остаток на складе…»

### Фикстуры и сиды (АКЦЕНТ — хронический пропуск)
- **TS-F3.** `e2e/fixtures/auth.ts`: новые помощники `softDelete(kind, id, reason)`, `restore(kind, id)`, `listDeleted(kind, filters)`; payload'ы типизировать wire-типами (§TD-6.2; сырые ключи невидимы гейтам) — DS-028
- **TS-F4.** Существующие спеки с проверками безвозвратного удаления и отказов по ссылкам (`e2e/tests/board.spec.ts`, `board-guards.spec.ts`, `clients.spec.ts`, `equipment.spec.ts`, `catalog.spec.ts`, `equipment-types.spec.ts`, `transfers.spec.ts`, `transfers-revert.spec.ts`, `transfers-partners.spec.ts`, `polish.spec.ts`): ожидания меняются — обязательная причина, отсутствие 409 по ссылкам; каждая часть обновляет свои спеки — DS-006, DS-014, DS-020
- **TS-F6.** Go live-тесты удаления (`internal/features/board/board_http_live_test.go`, `internal/features/transfers/transfers_partners_http_live_test.go`, `internal/features/clients`, `internal/features/catalog`): запросы DELETE получают тело с причиной; проверки «строки нет в базе» заменяются на «строка помечена удалённой» — DS-020, DS-028
- **TS-F10.** Каталог и типы оборудования в автотестах: `e2e/pages/CatalogPage.ts`, `e2e/pages/EquipmentTypesPage.ts`, `e2e/tests/catalog.spec.ts`, `equipment-types.spec.ts` — в шаги удаления добавляется ввод причины; проверки отказов «используется — отправьте в архив» заменяются на успешное удаление; созданные тестами товары, услуги, категории и типы убираются `purge(kind, id)`; обновить в части 5 — DS-027, DS-014

## §Impact — точки касания существующего кода
Источник: `python3 ~/.claude/tools/impact_scan.py` (sample `internal/features/catalog`, `internal/features/equipmenttypes`; термы `products`, `services`, `catalog_categories`, `equipment_types`, `product_stock`, `PRODUCT_IN_USE`, `SERVICE_IN_USE`, `CATEGORY_IN_USE`, `EQUIPMENT_TYPE_IN_USE`) + поиск запросов к таблицам каталога вне пакетов-владельцев (fff) + решения мастера.

| # | Класс | Что задеваем (file:line / объект) | Дельта фичи | Решение | DS |
|---|---|---|---|---|---|
| 1 | SHARED-DEP | `internal/features/catalog/products_service.go:155` (`Delete`), `services_service.go:183`, `categories_service.go:138`; обработчики `products_handler.go:93`, `services_handler.go:95`, `categories_handler.go:75` (ответы 204); `routes.go:22`, `:29`, `:36` | мягкое удаление с причиной; ответ 200 с телом; новые `Restore` и маршруты `POST /:id/restore` | править | DS-014, DS-028 |
| 2 | SHARED-DEP | `internal/features/catalog/products_repo.go:198`, `services_repo.go:376`, `categories_repo.go:152` (`Delete`) | DELETE → UPDATE; разбор нарушений внешних ключей убирается; у товара — блокировка и проверка `product_stock` | править | DS-014, DS-021 |
| 3 | SHARED-DEP | `internal/features/catalog/products_repo.go:74`–`:110` (список, чтение), `categories_repo.go:58`–`:96` (список, `item_count`), `services_repo.go` (список, чтение, состав), `products_stock.go` (`updateStocks`, `checkStockWarehouses`) | условие `deleted_at IS NULL`; `category_is_deleted`, `product_is_deleted` в выборках; `item_count` — только действующие | править | DS-014, DS-009 |
| 4 | SHARED-DEP | `internal/features/catalog/ssr_products.go:275`, `ssr_services.go:339`, `ssr_categories.go:74`; шаблоны `web/ssr/catalog` | поле формы `reason`; повторный показ окна при ошибке; бейджи; страница 404 для удалённых | править | DS-032 |
| 5 | SHARED-DEP | `internal/features/equipmenttypes/service.go:146` (`Delete`), `repo.go:157`, `handler.go:79`, `ssr.go:191`, `routes.go:20`; шаблоны `web/ssr/equipmenttypes` | то же для типов оборудования; `equipment_count` — только действующие единицы: подзапрос `repo.go:43` получает условие `deleted_at IS NULL` | править | DS-014, DS-017 |
| 6 | ENUMERATION | коды ошибок пакетов `catalog`, `equipmenttypes` | новые `PRODUCT_HAS_STOCK`, `NAME_TAKEN` (409); `PRODUCT_IN_USE`, `SERVICE_IN_USE`, `CATEGORY_IN_USE`, `EQUIPMENT_TYPE_IN_USE` перестают возвращаться продуктовыми адресами (остаются в служебной уборке); `CATEGORY_NAME_TAKEN`, `EQUIPMENT_TYPE_NAME_TAKEN`, `EQUIPMENT_TYPE_NAME_ARCHIVED` — без изменений | править | DS-014, DS-017, DS-021 |
| 7 | SHARED-DEP | `internal/features/board/card_repo.go:134` (услуги карточки), `:193` (товары услуг), `card_dto.go:102`, `:124` | новые поля `service_is_deleted`, `product_is_deleted` | править | DS-014 |
| 8 | SHARED-DEP | `internal/features/board/subtask_products_repo.go:41` (проверка товара), `:99` (комплект услуги), проверка услуги при создании услуги заказа | удалённый товар и услуга → `not_found`; комплект — без удалённых товаров | править | DS-014 |
| 9 | SHARED-DEP | `internal/features/board/stock_repo.go:41`, `:70`, `:172`, `:219`, `:246` (отчёт «Остатки», корректировка), `stock_dto.go` (`StockRow`) | отчёт: существующее условие `is_active` дополняется «либо удалён с ненулевым остатком»; новое поле `is_deleted`; счётчики — без удалённых; корректировка разрешена для удалённого товара с остатком | править | DS-042, DS-021 |
| 10 | SHARED-DEP | `internal/features/board/tasks_repo.go:183`, `:347`, `:350`, `:355`, `task_equipment_repo.go:75`, `:119` (соединения с `services`, `products` из заказов) | без изменений: из заказов удалённая запись читается как обычно | не трогать | DS-014 |
| 11 | SHARED-DEP | `internal/features/clients/repo.go:406` (соединение с `equipment_types`), `:444` (`TypeActive`), `:554` (`TypeOptions`), `dto.go:61` | новое поле `type_is_deleted`; выбор типа без удалённых; удалённый тип → `not_found` | править | DS-014 |
| 12 | SHARED-DEP | `internal/features/transfers/dto.go:159`, проверка товаров строк документа в `repo.go`, восстановление документа | новое поле `product_is_deleted`; удалённый товар в новой строке → `not_found`; восстановление заявки и отмена проведения документа с удалённым товаром → `TRANSFER_HAS_DELETED_REFS` | править | DS-014, DS-045, DS-046 |
| 13 | SHARED-DEP | `internal/features/softdelete` (часть 1) | четыре вида в списке, счётчиках и служебной уборке; `is_archived` | править | DS-011, DS-027 |
| 14 | FK | `migrations/0004_catalog.up.sql:15` (`catalog_categories_kind_name_lower_uq`), `migrations/0003_clients_equipment.up.sql:31` (`equipment_types_name_lower_uq`); внешние ключи `:19`, `:36`, `:47`, `:56` | новая миграция: по три поля на четырёх таблицах, CHECK, FK `deleted_by`, частичные индексы; два уникальных индекса названий пересоздаются частичными; внешние ключи не меняются | добавить | DS-017, DS-022 |
| 15 | FRONTEND | `web/ssr/catalog`, `web/ssr/equipmenttypes`, `web/ssr/board`, `web/ssr/clients`, `web/ssr/transfers`, `web/ssr/softdelete` (часть 1) | окно с причиной; бейджи; четыре вкладки | править | DS-020, DS-014 |
| 16 | ENUMERATION | `e2e/pages/CatalogPage.ts`, `e2e/pages/EquipmentTypesPage.ts`, `e2e/tests/catalog.spec.ts`, `equipment-types.spec.ts`, `polish.spec.ts`, `stock.spec.ts` | причина в шагах удаления; ожидания по снятым отказам; уборка служебным адресом | править | DS-027, DS-014 |
| 17 | SHARED-DEP | Go live-тесты пакетов `catalog`, `equipmenttypes` (проверки 204, «используется → 409») | ожидания: 200 с телом, удаление при ссылках проходит | править | DS-014, DS-028 |
| 18 | CONFIG | env-ключи | не задеваются | — | — |
| 19 | MONITORING | дашборды, nginx | не задеваются | — | — |

**Вне каталога фичи:** есть — internal/features/catalog, internal/features/equipmenttypes, internal/features/board, internal/features/clients, internal/features/transfers, internal/features/softdelete, migrations, web/ssr/catalog, web/ssr/equipmenttypes, web/ssr/board, web/ssr/clients, web/ssr/transfers, web/ssr/softdelete, e2e

### prd.md
MISSING

## Инвентарь доков (точечный grep разрешён, полное чтение — нет)
- draft-spec.md — 123KB
- 0-setup/draft-spec.slice-light.md — 109KB
- 8-code-review/input-bundle.md — 37KB
- 8-code-review/output.md — 17KB
- 0-setup/passport.md — 16KB
- 0-setup/idioms.md — 12KB
- 0-setup/light-progress.md — 7KB
- 0-setup/pipeline-state.md — 0KB
- 9-validation/evidence/last-run.json — 0KB

## Паспорт (выжимка; полный — 0-setup/passport.md)
# Passport: soft-delete-catalog (generated — руками не править; stage_bundle.py)
**Tier:** M | **Совместимость:** MVP (pipeline-doctrine.md §CM-1; ? → уточни у оркестратора)
**Трек:** обычный (prefs нет — дефолт) — режимы агентов (`spec-track delta/micro/mini/merged`) включает ТОЛЬКО строка `Режим:` в prompt оркестратора; DS/TS-индексы ниже присутствуют на ЛЮБОМ треке при наличии draft-spec.
**Layout:** автодетект (tools/layout_resolve.py)
**UI-scope (pipeline-doctrine.md §US-1, шапка draft-spec):** swagger:no | adaptive:min | themes:both
