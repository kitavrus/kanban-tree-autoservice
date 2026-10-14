# Review Bundle: soft-delete-warehouses-suppliers — CHECKS-FAILED
**Base:** 84c3315f0d4a5e04c25b8f09f0cf7f83400443ca | **HEAD:** 6d85d32 | **Generated:** 2026-10-09 00:21 | diff: 64 files changed, 5967 insertions(+), 266 deletions(-)

> ℹ️ Go-каталоги фичи выведены из диффа (имя «soft-delete-warehouses-suppliers» не резолвится в пакет): `internal/features/board`, `internal/features/catalog`, `internal/features/softdelete`, `internal/features/suppliers`, `internal/features/transfers`, `internal/features/warehouses`

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
| e2e_drift | ❌ | 1 | e2e-drift: FAIL (режим: TS (spec-track §ST-1.4), doc: 1 id, code: 0 в 0 файлах, проблем: 1)<br>DRIFT: TS-045 есть в draft-spec.md, теста test('TS-045: …') нет (сценарий без теста) |

## Diff по линзам
| Линза | Файлов | Patch |
|---|---|---|
| backend | 38 | 8-code-review/diff-backend.patch |
| migrations | 2 | 8-code-review/diff-migrations.patch |
| other | 18 | 8-code-review/diff-other.patch |
| docs | 6 | — (артефакты, не ревьюятся построчно) |

### backend
- `M` internal/features/board/card_dto.go
- `M` internal/features/board/card_repo.go
- `M` internal/features/board/errors.go
- `A` internal/features/board/soft_delete_warehouses_http_live_test.go
- `M` internal/features/board/stock_dto.go
- `M` internal/features/board/stock_repo.go
- `M` internal/features/board/stock_service.go
- `M` internal/features/board/stock_sql_test.go
- `M` internal/features/board/subtask_products_repo.go
- `M` internal/features/board/subtask_products_service.go
- `M` internal/features/board/tasks_service.go
- `A` internal/features/catalog/catalog_soft_delete_warehouses_http_live_test.go
- `M` internal/features/catalog/products_service.go
- `M` internal/features/catalog/products_stock.go
- `M` internal/features/softdelete/catalog_source.go
- `M` internal/features/softdelete/catalog_source_test.go
- `M` internal/features/softdelete/softdelete_http_live_test.go
- `M` internal/features/suppliers/repo.go
- `M` internal/features/suppliers/routes.go
- `A` internal/features/suppliers/soft_delete.go
- `M` internal/features/suppliers/suppliers_http_live_test.go
- `A` internal/features/suppliers/suppliers_soft_delete_http_live_test.go
- `M` internal/features/transfers/dto.go
- `M` internal/features/transfers/repo.go
- `M` internal/features/transfers/ssr.go
- `A` internal/features/transfers/transfers_soft_delete_warehouses_http_live_test.go
- `A` internal/features/warehouses/migration_soft_delete_integration_test.go
- `M` internal/features/warehouses/repo.go
- `M` internal/features/warehouses/routes.go
- `M` internal/features/warehouses/service.go
- `A` internal/features/warehouses/soft_delete.go
- `M` internal/features/warehouses/ssr.go
- `M` internal/features/warehouses/validate_test.go
- `M` internal/features/warehouses/warehouses_http_live_test.go
- `A` internal/features/warehouses/warehouses_soft_delete_http_live_test.go
- `M` internal/seeddemo/seeddemo.go
- `M` internal/seeddemo/seeddemo_integration_test.go
- `M` internal/server/router.go

### migrations
- `A` migrations/0023_soft_delete_warehouses_suppliers.down.sql
- `A` migrations/0023_soft_delete_warehouses_suppliers.up.sql

### other
- `M` e2e/fixtures/auth.ts
- `M` e2e/fixtures/board-types.ts
- `M` e2e/fixtures/test.ts
- `M` e2e/pages/OrderCardPage.ts
- `M` e2e/pages/StockPage.ts
- `M` e2e/pages/SuppliersPage.ts
- `M` e2e/pages/TransfersPage.ts
- `A` e2e/pages/WarehousesPage.ts
- `A` e2e/tests/soft-delete-warehouses-suppliers.spec.ts
- `M` e2e/tests/suppliers.spec.ts
- `M` e2e/tests/warehouses.spec.ts
- `M` web/ssr/_shared/js/deleted.js
- `M` web/ssr/_shared/js/stock.js
- `M` web/ssr/_shared/js/suppliers.js
- `M` web/ssr/_shared/js/task-card.js
- `M` web/ssr/_shared/js/transfers.js
- `M` web/ssr/suppliers/templates/pages/list.html
- `M` web/ssr/warehouses/templates/pages/list.html

### docs
- `A` docs/features/soft-delete-warehouses-suppliers/0-setup/draft-spec.slice-light.md
- `A` docs/features/soft-delete-warehouses-suppliers/0-setup/idioms.md
- `A` docs/features/soft-delete-warehouses-suppliers/0-setup/light-progress.md
- `A` docs/features/soft-delete-warehouses-suppliers/0-setup/passport.md
- `A` docs/features/soft-delete-warehouses-suppliers/0-setup/pipeline-state.md
- `A` docs/features/soft-delete-warehouses-suppliers/0-setup/timeline.jsonl

## Экстракты документов (вместо полного чтения)
### draft-spec.md (экстракт: шапка + DS + дельта + Impact + TS)
# Draft Spec: Мягкое удаление — склады и поставщики

**Tier:** S · **Совместимость:** MVP · **Режим:** new

**Дата:** 2026-10-08 | **Статус:** draft → final (после подтверждения пользователя) | **Tier:** S
**Совместимость:** MVP (DS-022) — рабочей версии с живыми пользователями нет: таблицы и адреса меняем свободно, без переходного периода и отката; данные на базе сохраняются — все существующие склады и поставщики после миграции считаются действующими, признак архива не меняется
**UI-scope:** swagger:no | adaptive:min | themes:both (DS-023)
**Источник:** мастер-спека `docs/features/soft-delete/draft-spec.md` (§4.10, §4.11, §6.19–§6.22, §6.26, §8.5) + её журнал `docs/features/soft-delete/draft-spec-decisions.md` + as-is факты кода (`internal/features/warehouses`, `internal/features/suppliers`, `internal/features/transfers/dto.go`, `migrations/0008_warehouses.up.sql`, `migrations/0012_suppliers.up.sql`)
**Часть 6 из 6 мастер-спеки `soft-delete`; зависит от: 1 (`soft-delete-core`).** Номера DS и TS — мастер-идентификаторы, не перенумерованы. Номера разделов — свои: §6.1–§6.4 = мастер §6.19–§6.22, §6.5 = строки мастера §6.26 про склады и поставщиков; §8.1 = часть мастера §8.5.

> После мержа этой части: у складов и поставщиков впервые появляется действие «Удалить» (до этого был только архив) — с причиной и с восстановлением из вкладок «Склады» и «Поставщики» раздела «Удалённые». Склад удаляется только пустой: без товара, без оборудования, без незавершённых перемещений. Поставщик удаляется при любых документах. Название удалённого склада или поставщика свободно.

## §9. Решения (DS-index)
Строки — дословно из мастера, номера не меняются; включены только решения, действующие в части 6.

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
| DS-033 | UI-конвенции: ошибка поля — под полем; тосты — правый верхний угол, 5 секунд; причина и подтверждения — `KT.uiDialog`; пометка «удалён» — серый бейдж; предупреждения восстановления — жёлтый блок над таблицей | inferred, transfers DS-052 | closed |
| DS-041 | Клиента и отдельную единицу оборудования нельзя удалить, пока оборудование на нашем складе или в пути: сначала вернуть его клиенту (409 `CLIENT_EQUIPMENT_AT_WAREHOUSE`, `EQUIPMENT_AT_WAREHOUSE`). Исключение из DS-014, по аналогии со складом с остатком (DS-021). Клиента нельзя удалить и пока его единица стоит в незавершённом документе (409 `CLIENT_EQUIPMENT_IN_TRANSFER`) — иначе документ привёз бы удалённую единицу на склад. Уточняет DS-015 | батч 6 (В-22); запрет при незавершённом документе, коды и тексты ошибок — inferred | closed |
| DS-042 | Остаток, появившийся у удалённого товара или на удалённом складе позже (возврат из заказа, приём документа), виден в отчёте «Остатки» с пометкой «удалён»; при нулевом остатке удалённые товар и склад в отчёте не показываются. Ручная корректировка такого остатка разрешена — чтобы его списать | батч 6 (В-23); разрешение корректировки — inferred | closed |
| DS-044 | Проверка архивирования и удаления склада считает мешающими только действующие документы в «Заявка» и «В пути»; мягко удалённые заявки не мешают — администратору они не видны. Уточняет DS-021 | оркестратор по ревью части 2, inferred | closed |
| DS-045 | Заявка перемещения не восстанавливается, пока что-то из её состава в удалённых — единица оборудования, товар, склад или поставщик: 409 `TRANSFER_HAS_DELETED_REFS` «В перемещении есть удалённые записи — сначала восстановите их», `details.kind` и `details.id`. Отменённые документы не проверяются. Строки удалённой заявки не удерживают единицу, товар, склад и поставщика от удаления (следствие DS-014, DS-018, DS-044), поэтому без этой проверки восстановленная заявка привезла бы товар или оборудование на удалённый склад либо заняла удалённую единицу (нарушение DS-041). Проверка каждого вида вводится его частью: единица — часть 4, товар — часть 5, склад и поставщик — часть 6 | inferred (следствие DS-041, DS-044; по аналогии с DS-043) | closed |
| DS-046 | Дополняет DS-045. Отмена проведения документа перемещения отклоняется, если его единица оборудования, товар или склад в удалённых: 409 `TRANSFER_HAS_DELETED_REFS`, `details.kind` и `details.id`. Откат вернул бы документ в «В пути»: удалённая единица оказалась бы в пути (нарушение DS-041), а остаток удалённого товара или на удалённом складе изменился бы. Достижимый случай для единицы — документ «склад → клиент» получен, затем единицу или клиента удалили. Удалённый поставщик отмене проведения не мешает: он не держит ни остатка, ни местоположения. Проверка вводится по частям: единица — часть 4, товар — часть 5, склад — часть 6 | оркестратор по ревью части 4 (единица; пользователь уведомлён); товар, склад и исключение для поставщика — inferred | closed |
| DS-047 | Дополняет DS-046. Отправка заявки перемещения отклоняется, если её товар в удалённых: 409 `TRANSFER_HAS_DELETED_REFS`, `details.kind` = `product`, `details.id` — для всех направлений. Отмена документа «В пути» с удалённым товаром НЕ запрещается: товар возвращается на склад-источник и виден в «Остатках» с пометкой (DS-042). Отправка заявки с удалённым складом отправления или назначения отклоняется тем же кодом (`details.kind` = `warehouse`) — как страховка: склад с действующей заявкой не удаляется (DS-021), а заявка с удалённым складом не восстанавливается (DS-045). Удалённый поставщик отправке не мешает: он не держит ни остатка, ни местоположения, а отправка — доведение начатого документа (DS-007). Проверка товара — часть 5, склада — часть 6 | оркестратор по ревью части 5 (товар; пользователь уведомлён и не возразил); склад и исключение для поставщика — inferred | closed |
| DS-048 | Уточняет DS-042. Остаток удалённого товара и остаток на удалённом складе можно только списывать. Корректировка допустима только по паре товар × склад с ненулевым остатком (иначе 404 `PRODUCT_NOT_FOUND` для удалённого товара, строка остатка не создаётся); новое значение — между нулём и текущим включительно (отрицательный остаток — долг — так же списывается к нулю); увеличение или смена знака → 409 `PRODUCT_DELETED_WRITE_OFF_ONLY` «Товар удалён — его остаток можно только списать» либо 409 `WAREHOUSE_DELETED_WRITE_OFF_ONLY` «Склад удалён — остаток на нём можно только списать». Корректировка остатка удалённого товара не требует, чтобы склад был активным. Товар — часть 5, склад — часть 6 | оркестратор по ревью части 5 (товар; реализовано); правило и код для склада — inferred | closed |
| DS-049 | Отчёт «Остатки» показывает и архивные, и удалённые склады, на которых есть ненулевой остаток, — колонкой с пометкой «в архиве» либо «удалён»; без остатка такие склады в отчёте не показываются. Иначе удалённый товар с остатком только на архивном складе виден строкой с нулями и его нечем списать. Окно корректировки для удалённого товара предлагает только склады с ненулевым остатком этого товара. Часть 6 | оркестратор по ревью части 5, inferred | closed |

## §10. Открытые вопросы (status: open)
Открытых вопросов нет

## §11. Тестовые сценарии (черновик — вход для design-e2e-strategy, TD-6.8)
Оси UI-scope (DS-023): Swagger-сценариев нет; адаптив и темы раздела «Удалённые» и окна удаления покрыты TS-046, TS-047 части 1 (новые вкладки и бейджи проверяются в них же). Номера TS — мастер-идентификаторы; включены только сценарии части 6. В TS-012 и TS-033 в этой части проверяются таблицы `t:warehouses`, `t:suppliers` и адреса §6.1–§6.4. В TS-031 ветка «с оборудованием» проверяется для действующей единицы: удалённых единиц на складе не бывает (DS-041), отдельного сценария на них нет — запрос проверки просто не фильтрует по удалению. В TS-059 в этой части проверяются виды `warehouse` и `supplier`; в TS-060 — вид `warehouse` и ветка «удалённый поставщик отмене проведения не мешает». TS-064 и TS-065 добавлены по ревью реализации части 5.

### Unit / Integration (Go)
- **TS-012.** Миграция: после применения все существующие строки 11 таблиц имеют `deleted_at IS NULL`; CHECK «три поля вместе» отклоняет частичное заполнение (части 1–6, каждая — свои таблицы) — DS-022, DS-030
  - негатив: UPDATE только `deleted_at` без `deleted_by` → ошибка CHECK
- **TS-031.** Удаление склада: пустой → 200; с товаром → 409; с оборудованием → 409; с документом в `requested` или `in_transit` → 409 — три разных сообщения `WAREHOUSE_IN_USE`; последний активный → 409 `WAREHOUSE_LAST_ACTIVE` (часть 6) — DS-021
  - негатив: склад с завершёнными документами удаляется
- **TS-032.** Поставщик: удаление при наличии документов → 200, в документе `f:supplier_is_deleted` = true; название свободно; восстановление при занятом названии → 409 `NAME_TAKEN` (часть 6) — DS-014, DS-017
  - негатив: выбор удалённого поставщика в новом документе → 400 `supplier_id: not_found`
- **TS-033.** Права на всех 22 адресах удаления и восстановления: executor → 403 «Нет доступа» до поиска записи, включая несуществующий идентификатор (части 1–6) — DS-003
  - негатив: без сессии → 401
- **TS-055.** Остаток на удалённом складе: склад удалён пустым, затем возврат товара из заказа по строке этого склада → в `GET /api/v1/stock` склад в `f:warehouses` с `f:is_deleted` = true; корректировка до нуля → 200, склад исчезает из отчёта (часть 6) — DS-042
  - негатив: удалённый склад без остатка в отчёте не показывается; выбрать его в новом документе нельзя — 400 `not_found`
- **TS-057.** Удалённая заявка не удерживает склад: склад, на который ссылается только мягко удалённая заявка, архивируется (`PUT /api/v1/warehouses/:id` с `is_active` = false → 200) и удаляется (`DELETE /api/v1/warehouses/:id` → 200) (часть 6) — DS-044
  - негатив: действующая заявка с этим складом → 409 `WAREHOUSE_IN_USE` в обоих случаях
- **TS-059.** Восстановление заявки с удалённым товаром, складом или поставщиком → 409 `TRANSFER_HAS_DELETED_REFS` с соответствующим `details.kind` (`product`, `warehouse`, `supplier`); после восстановления записи заявка восстанавливается (товар — часть 5; склад и поставщик — часть 6) — DS-045
  - негатив: отменённый документ с удалёнными товаром, складом, поставщиком восстанавливается — 200
- **TS-060.** Отмена проведения документа с удалённым товаром или складом: `POST /api/v1/transfers/:id/unreceive` → 409 `TRANSFER_HAS_DELETED_REFS` с `details.kind` = `product` либо `warehouse`, документ остаётся «Получено», остатки не изменились; после восстановления записи отмена проведения проходит; документ удалённого поставщика при действующих товаре и складе откатывается — 200 (товар — часть 5; склад и поставщик — часть 6) — DS-046
  - негатив: приём документа «В пути» с удалённым товаром по-прежнему проходит — остаток виден с пометкой (DS-042)
- **TS-064.** Отправка заявки и удалённые склад, поставщик: заявка, склад которой помечен удалённым в обход правил (прямой записью в базу), → `send` 409 `TRANSFER_HAS_DELETED_REFS` с `details.kind` = `warehouse`; заявка «поставщик → склад», поставщик которой удалён после создания, отправляется — 200 (часть 6) — DS-047
  - негатив: удаление склада при действующей заявке по-прежнему → 409 `WAREHOUSE_IN_USE`
- **TS-065.** Остаток на архивном и удалённом складе: в `GET /api/v1/stock` склад с ненулевым остатком присутствует в `f:warehouses` с `f:is_archived` либо `f:is_deleted` = true, без остатка — отсутствует; удалённый товар с остатком только на архивном складе показан с этим остатком, а не нулями; корректировка на удалённом складе — только списание: увеличение → 409 `WAREHOUSE_DELETED_WRITE_OFF_ONLY`; список складов окна корректировки удалённого товара содержит только склады с его ненулевым остатком (часть 6) — DS-048, DS-049
  - негатив: активный склад без остатка по-прежнему показывается колонкой

### E2E (Playwright)
- **TS-045.** Склад и поставщик — экраны §8.1, §8.2 — DS-021, DS-014 (часть 6)
  - шаги: удалить пустой склад и поставщика с документами → восстановить; проверка: кнопка «Удалить» есть рядом с «В архив», записи во вкладках «Склады», «Поставщики»; негатив: склад с товаром — «На складе есть товар…»

### Фикстуры и сиды (АКЦЕНТ — хронический пропуск)
- **TS-F3.** `e2e/fixtures/auth.ts`: новые помощники `softDelete(kind, id, reason)`, `restore(kind, id)`, `listDeleted(kind, filters)`; payload'ы типизировать wire-типами (§TD-6.2; сырые ключи невидимы гейтам) — DS-028
- **TS-F6.** Go live-тесты удаления (`internal/features/board/board_http_live_test.go`, `internal/features/transfers/transfers_partners_http_live_test.go`, `internal/features/clients`, `internal/features/catalog`): запросы DELETE получают тело с причиной; проверки «строки нет в базе» заменяются на «строка помечена удалённой» — DS-020, DS-028
- **TS-F11.** Склады и поставщики в автотестах: `e2e/tests/warehouses.spec.ts`, `suppliers.spec.ts`, `e2e/pages/SuppliersPage.ts` — новые шаги «Удалить» с причиной и «Восстановить»; созданные тестами склады и поставщики убираются `purge(kind, id)` (сейчас они только архивируются и копятся); обновить в части 6 — DS-027

## §Impact — точки касания существующего кода
Источник: `python3 ~/.claude/tools/impact_scan.py` (sample `internal/features/warehouses`, `internal/features/suppliers`; термы `warehouses`, `suppliers`, `warehouse_id`, `supplier_id`, `default_warehouse_id`, `WAREHOUSE_IN_USE`, `WAREHOUSE_LAST_ACTIVE`) + чтение проверок архивирования склада и запросов к складам и поставщикам вне пакетов-владельцев (fff) + решения мастера.

| # | Класс | Что задеваем (file:line / объект) | Дельта фичи | Решение | DS |
|---|---|---|---|---|---|
| 1 | SHARED-DEP | `internal/features/warehouses/routes.go:13`–`:27`, `handler.go`, `service.go` (транзакция архивирования с блокировкой активных складов), `repo.go:136` (`Repo.InUse` — проверка оборудования и документов) | новые `Delete`, `Restore`, маршруты `DELETE /:id`, `POST /:id/restore`, SSR `POST /:id/delete`; проверка остатка товара; общий код проверок с архивированием; в `InUse` — условие `transfers.deleted_at IS NULL` (действует и на архивирование) | добавить, править | DS-021, DS-044 |
| 2 | SHARED-DEP | `internal/features/warehouses/repo.go` (список, чтение, создание, правка), `ssr.go` (список и ошибки над списком) | условие `deleted_at IS NULL`; кнопка «Удалить»; три текста `WAREHOUSE_IN_USE` | править | DS-014, DS-017 |
| 3 | ENUMERATION | коды ошибок `internal/features/warehouses`: `WAREHOUSE_IN_USE` (as-is текст «На складе есть оборудование или незавершённые перемещения»), `WAREHOUSE_LAST_ACTIVE` (as-is текст «Нельзя отправить в архив последний активный склад») | для удаления — отдельные тексты по причине отказа и «Нельзя удалить последний активный склад»; тексты архивирования не меняются; новый код `NAME_TAKEN` | править | DS-021, DS-017 |
| 4 | SHARED-DEP | `internal/features/suppliers/routes.go:13`–`:16`, `handler.go`, `service.go`, `repo.go` | новые `Delete`, `Restore` и маршруты; условие `deleted_at IS NULL` в списке, чтении, проверке названия | добавить, править | DS-014, DS-017 |
| 5 | FK | `migrations/0008_warehouses.up.sql:15` (`warehouses_name_lower_uq`), `:22` (функция `default_warehouse_id()`), `migrations/0012_suppliers.up.sql:16` (`suppliers_name_lower_uq`) | новая миграция: по три поля, CHECK, FK `deleted_by`, частичные индексы; два уникальных индекса пересоздаются частичными; функция `default_warehouse_id()` пересоздаётся с условием `deleted_at IS NULL` | добавить | DS-017, DS-021, DS-022 |
| 6 | SHARED-DEP | `internal/features/transfers/dto.go` (`TransferResponse`, `TransferSummary`), `repo.go:185` (соединения со складами и поставщиком), проверка ссылок документа (`activeFlags`, сообщения `archived` / `not_found`), восстановление документа, список складов экрана | новые поля `from_warehouse_is_deleted`, `to_warehouse_is_deleted`, `supplier_is_deleted`; удалённые склад и поставщик → `not_found`; восстановление заявки с удалённым складом или поставщиком, отправка заявки и отмена проведения документа с удалённым складом → `TRANSFER_HAS_DELETED_REFS`; выбор без удалённых | править | DS-014, DS-045, DS-046, DS-047 |
| 7 | SHARED-DEP | `internal/features/board/card_dto.go` (`CardWarehouse`), `card_repo.go:287` (склады карточки), проверка склада строки товара, фильтр доски по складу, `stock_repo.go` (колонки складов отчёта, корректировка), `stock_dto.go` (`StockWarehouse`) | новые поля `is_deleted` (`CardWarehouse`, `StockWarehouse`); удалённый склад в новых строках → `not_found`; отчёт «Остатки» показывает архивный и удалённый склад с ненулевым остатком с пометкой (`is_archived`, `is_deleted`); корректировка на удалённом складе — только списание (`WAREHOUSE_DELETED_WRITE_OFF_ONLY`); окно корректировки удалённого товара — только склады с его остатком; фильтры — без складов без остатка | править | DS-014, DS-021, DS-042, DS-048, DS-049 |
| 8 | SHARED-DEP | `internal/features/catalog/products_stock.go` (`checkStockWarehouses`, `StocksBatch`, `ActiveWarehouses`) | блок «Склады» товара без удалённых складов; удалённый склад в запросе → `not_found` | править | DS-014 |
| 9 | SHARED-DEP | `internal/features/softdelete` (часть 1) | виды `warehouses`, `suppliers` в списке, счётчиках и служебной уборке; `is_archived` | править | DS-011, DS-027 |
| 10 | FRONTEND | `web/ssr/warehouses`, `web/ssr/suppliers`, `web/ssr/transfers`, `web/ssr/board`, `web/ssr/catalog`, `web/ssr/softdelete` (часть 1) | кнопки «Удалить»; бейджи; две вкладки | править | DS-020, DS-014 |
| 11 | ENUMERATION | `e2e/pages/SuppliersPage.ts`, `e2e/tests/warehouses.spec.ts`, `suppliers.spec.ts`, `transfers-partners.spec.ts`, `stock.spec.ts` | новые шаги удаления и восстановления; уборка служебным адресом | править | DS-027 |
| 12 | SHARED-DEP | Go live-тесты `internal/features/warehouses`, `internal/features/suppliers` (проверка «архивный поставщик тоже занимает название» остаётся верной; добавляется «удалённый — не занимает») | новые сценарии | добавить | DS-017 |
| 13 | SHARED-DEP | `internal/seeddemo/seeddemo.go` | вставки со складом по умолчанию продолжают работать (функция выбирает действующий склад) | проверить | DS-021 |
| 14 | CONFIG | env-ключи | не задеваются | — | — |
| 15 | MONITORING | дашборды, nginx | не задеваются | — | — |

**Вне каталога фичи:** есть — internal/features/warehouses, internal/features/suppliers, internal/features/transfers, internal/features/board, internal/features/catalog, internal/features/softdelete, internal/seeddemo, migrations, web/ssr/warehouses, web/ssr/suppliers, web/ssr/transfers, web/ssr/board, web/ssr/catalog, web/ssr/softdelete, e2e

### prd.md
MISSING

## Инвентарь доков (точечный grep разрешён, полное чтение — нет)
- draft-spec.md — 113KB
- 0-setup/draft-spec.slice-light.md — 101KB
- 8-code-review/input-bundle.md — 39KB
- 0-setup/passport.md — 21KB
- 8-code-review/output.md — 14KB
- 0-setup/idioms.md — 13KB
- 0-setup/light-progress.md — 2KB
- 0-setup/pipeline-state.md — 0KB
- 9-validation/evidence/last-run.json — 0KB

## Паспорт (выжимка; полный — 0-setup/passport.md)
# Passport: soft-delete-warehouses-suppliers (generated — руками не править; stage_bundle.py)
**Tier:** S | **Совместимость:** MVP (pipeline-doctrine.md §CM-1; ? → уточни у оркестратора)
**Трек:** обычный (prefs нет — дефолт) — режимы агентов (`spec-track delta/micro/mini/merged`) включает ТОЛЬКО строка `Режим:` в prompt оркестратора; DS/TS-индексы ниже присутствуют на ЛЮБОМ треке при наличии draft-spec.
**Layout:** автодетект (tools/layout_resolve.py)
**UI-scope (pipeline-doctrine.md §US-1, шапка draft-spec):** swagger:no | adaptive:min | themes:both
