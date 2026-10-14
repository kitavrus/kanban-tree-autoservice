# Review Bundle: transfers-core — CHECKS-FAILED
**Base:** bf5bc9be8b4e75e0786ef8b860c30c610bdc06c9 | **HEAD:** 49b1375 | **Generated:** 2026-10-07 19:34 | diff: 90 files changed, 19931 insertions(+), 3216 deletions(-)

> Линзы ревью: читай ЭТОТ бандл + patch своей линзы. Полное чтение research/draft-spec/PRD
> и перегон чекеров ЗАПРЕЩЕНЫ (вердикты ниже; детали — checks/<name>.txt; точечный grep по DS-NNN разрешён).

## Чекеры (детерминированные — НЕ перегонять)
| Чекер | Вердикт | exit | Нарушения (первые строки) |
|---|---|---|---|
| go_naming | ✅ | 0 | — |
| api_conventions | ✅ | 0 | — |
| contract | ✅ | 0 | — |
| date_bound | ✅ | 0 | — |
| fuzz_verdict | ✅ | 0 | — |
| live_suite | ✅ | 0 | — |
| fixture_sync | ✅ | 0 | — |
| e2e_mode | ✅ | 0 | — |
| rule_usage | ✅ | 0 | — |
| e2e_drift | ❌ | 1 | e2e-drift: FAIL (режим: TS (spec-track §ST-1.4), doc: 6 id, code: 0 в 0 файлах, проблем: 6)<br>DRIFT: TS-031 есть в draft-spec.md, теста test('TS-031: …') нет (сценарий без теста)<br>DRIFT: TS-036 есть в draft-spec.md, теста test('TS-036: …') нет (сценарий без теста)<br>DRIFT: TS-043 есть в draft-spec.md, теста test('TS-043: …') нет (сценарий без теста)<br>DRIFT: TS-044 есть в draft-spec.md, теста test('TS-044: …') нет (сценарий без теста)<br>DRIFT: TS-045 есть в draft-spec.md, теста test('TS-045: …') нет (сценарий без теста)<br>DRIFT: TS-046 есть в draft-spec.md, теста test('TS-046: …') нет (сценарий без теста) |

## Diff по линзам
| Линза | Файлов | Patch |
|---|---|---|
| backend | 49 | 8-code-review/diff-backend.patch |
| migrations | 2 | 8-code-review/diff-migrations.patch |
| other | 19 | 8-code-review/diff-other.patch |
| docs | 20 | — (артефакты, не ревьюятся построчно) |

### backend
- `M` internal/features/board/board_fuzz_test.go
- `M` internal/features/board/card_dto.go
- `M` internal/features/board/card_handler.go
- `M` internal/features/board/card_repo.go
- `M` internal/features/board/card_service.go
- `M` internal/features/board/enums.go
- `A` internal/features/board/equipment_location_http_live_test.go
- `M` internal/features/board/equipment_warehouse_validate_test.go
- `D` internal/features/board/equipment_warehouses_http_live_test.go
- `M` internal/features/board/errors.go
- `M` internal/features/board/order_card_http_live_test.go
- `M` internal/features/board/routes.go
- `M` internal/features/board/stock_repo.go
- `M` internal/features/board/subtask_products_repo.go
- `M` internal/features/board/task_equipment_repo.go
- `M` internal/features/board/task_equipment_service.go
- `M` internal/features/board/task_equipment_validate.go
- `M` internal/features/board/tasks_repo.go
- `M` internal/features/clients/clients_http_live_test.go
- `A` internal/features/clients/equipment_search.go
- `M` internal/features/clients/history.go
- `M` internal/features/clients/repo.go
- `M` internal/features/clients/routes.go
- `M` internal/features/clients/service.go
- `M` internal/features/executorpayouts/dto.go
- `D` internal/features/stocktransfers/migration_integration_test.go
- `D` internal/features/stocktransfers/repo.go
- `D` internal/features/stocktransfers/service.go
- `D` internal/features/stocktransfers/ssr.go
- `D` internal/features/stocktransfers/validate_test.go
- `R067` internal/features/transfers/dto.go
- `R082` internal/features/transfers/handler.go
- `A` internal/features/transfers/migration_integration_test.go
- `A` internal/features/transfers/repo.go
- `R063` internal/features/transfers/routes.go
- `A` internal/features/transfers/service.go
- `A` internal/features/transfers/ssr.go
- `A` internal/features/transfers/transfers_equipment_http_live_test.go
- `R062` internal/features/transfers/transfers_http_live_test.go
- `R052` internal/features/transfers/validate.go
- `A` internal/features/transfers/validate_test.go
- `M` internal/features/warehouses/repo.go
- `M` internal/features/warehouses/service.go
- `M` internal/features/warehouses/ssr.go
- `A` internal/features/warehouses/warehouses_in_use_http_live_test.go
- `M` internal/seeddemo/seeddemo.go
- `M` internal/seeddemo/seeddemo_integration_test.go
- `M` internal/server/router.go
- `M` internal/ssr/menu.go

### migrations
- `A` migrations/0014_transfers_core.down.sql
- `A` migrations/0014_transfers_core.up.sql

### other
- `M` e2e/fixtures/auth.ts
- `M` e2e/fixtures/board-types.ts
- `A` e2e/fixtures/contrast.ts
- `M` e2e/pages/ExecutorPayoutsPage.ts
- `M` e2e/pages/OrderCardPage.ts
- `D` e2e/pages/StockTransfersPage.ts
- `A` e2e/pages/TransfersPage.ts
- `M` e2e/tests/equipment-warehouses.spec.ts
- `M` e2e/tests/order-card.spec.ts
- `D` e2e/tests/stock-transfers.spec.ts
- `A` e2e/tests/transfers.spec.ts
- `M` e2e/tests/user-timezone.spec.ts
- `M` web/ssr/_shared/app.css
- `D` web/ssr/_shared/js/stock-transfers.js
- `M` web/ssr/_shared/js/task-card.js
- `A` web/ssr/_shared/js/transfers.js
- `M` web/ssr/clients/templates/pages/equipment-history.html
- `D` web/ssr/stocktransfers/templates/pages/list.html
- `A` web/ssr/transfers/templates/pages/list.html

### docs
- `A` docs/features/transfers-core/0-setup/draft-spec.slice-light.md
- `A` docs/features/transfers-core/0-setup/idioms.md
- `A` docs/features/transfers-core/0-setup/light-progress.md
- `A` docs/features/transfers-core/0-setup/passport.md
- `A` docs/features/transfers-core/0-setup/pipeline-state.md
- `A` docs/features/transfers-core/0-setup/timeline.jsonl
- `A` docs/features/transfers-core/8-code-review/checks/api_conventions.txt
- `A` docs/features/transfers-core/8-code-review/checks/contract.txt
- `A` docs/features/transfers-core/8-code-review/checks/date_bound.txt
- `A` docs/features/transfers-core/8-code-review/checks/e2e_drift.txt
- `A` docs/features/transfers-core/8-code-review/checks/e2e_mode.txt
- `A` docs/features/transfers-core/8-code-review/checks/fixture_sync.txt
- `A` docs/features/transfers-core/8-code-review/checks/fuzz_verdict.txt
- `A` docs/features/transfers-core/8-code-review/checks/go_naming.txt
- `A` docs/features/transfers-core/8-code-review/checks/live_suite.txt
- `A` docs/features/transfers-core/8-code-review/checks/rule_usage.txt
- `A` docs/features/transfers-core/8-code-review/diff-backend.patch
- `A` docs/features/transfers-core/8-code-review/diff-migrations.patch
- `A` docs/features/transfers-core/8-code-review/diff-other.patch
- `A` docs/features/transfers-core/8-code-review/input-bundle.md

## Экстракты документов (вместо полного чтения)
### draft-spec.md (экстракт: шапка + DS + дельта + Impact + TS)
# Draft Spec: Перемещения — ядро (документ, оборудование клиентов, местоположение)

**Tier:** M · **Совместимость:** MVP · **Режим:** new

**Дата:** 2026-10-07 | **Статус:** draft → final (после подтверждения пользователя) | **Tier:** M (верхняя граница)
**Совместимость:** MVP (DS-027) — живых данных нет: таблицы и адреса меняем свободно, без переходного периода
**UI-scope:** swagger:no | adaptive:min | themes:both (DS-028)
**Источник:** мастер-спека `docs/features/transfers/draft-spec.md` (§4.1, §4.2, §4.4, §4.7, §5, §6.1–§6.8, §6.14, §6.15, §8.1, §8.3) + её журнал `docs/features/transfers/draft-spec-decisions.md`
**Часть 3 из 6 мастер-спеки `transfers`; зависит от: —** (независима от частей 1 и 2). Номера DS и TS — мастер-идентификаторы, не перенумерованы.

> После мержа этой части: экран `/transfers` (только администратор) перемещает между складами товары И оборудование клиентов одним документом; местоположение — одно на единицу оборудования, в карточке заказа вместо кнопок «Отправить / Принять / Отменить отправку» — бейдж местоположения и ссылка на документ. Чего ещё нет: направлений с клиентом и поставщиком, исполнителя на экране, цен, отката «Получено». Принять оборудование от клиента по-прежнему можно только добавлением единицы в заказ с выбором «Склада приёма» (как сейчас) — документ «клиент → склад» заменит это в части 4.

## §9. Решения (DS-index)
| DS | Формулировка | Источник | Статус |
|---|---|---|---|
| DS-001 | Один документ «Перемещение» со смешанными строками (товар с количеством + единица оборудования); расширяем существующее перемещение, вторую сущность не заводим | мастер, батч 0 | closed |
| DS-003 | Кнопки «Отправить / Принять / Отменить отправку» у оборудования в карточке заказа убираются (отмена D57); оборудование двигается только документом; в карточке — местоположение и ссылка | мастер, батч 0 | closed |
| DS-005 | Закупленное у поставщика («наше оборудование») — товар каталога с количеством; `equipment.client_id` остаётся NOT NULL | мастер, батч 1–2 | closed |
| DS-006 | «Наша» единица при отправке клиенту становится его оборудованием | мастер, батч 1 | superseded by DS-011 (реализации нет) |
| DS-008 | Местоположение — на самой единице оборудования: «у клиента» / «на складе X» / «в пути (документ #N)»; новая единица — «у клиента» | мастер, батч 1 | closed |
| DS-009 | Всё от поставщика учитывается количеством; карточка оборудования клиента заводится вручную | мастер, батч 2 | closed |
| DS-010 | Строки-единицы — только оборудование клиентов | мастер, inferred | closed |
| DS-019 | Единица занимается документом сразу при добавлении в заявку: только один незавершённый документ; иначе «уже в перемещении #N» | мастер, батч 4 | closed |
| DS-026 | Единицу нельзя убрать из заказа и удалить из справочника, пока она в незавершённом документе; единицу с документами удалить из справочника нельзя; склад нельзя архивировать, пока на нём числится оборудование или есть незавершённые документы | мастер, батч 5 | closed |
| DS-027 | Совместимость — MVP | мастер, батч 6 | closed |
| DS-028 | UI-scope: swagger:no, adaptive:min, themes:both | мастер, батч 6 | closed |
| DS-029 | Миграция — сброс: единицы «в пути» считаются лежащими на складе отправления; старый журнал удаляется; демо-данные пересоздаются | мастер, батч 6 | closed |
| DS-030 | Переименование в `transfers`: страница, API, таблицы, код; старые адреса убираются | мастер, батч 6 | closed |
| DS-036 | Отправить единицу можно только оттуда, где она числится | мастер, inferred | closed |
| DS-037 | Строки-единицы принимаются целиком вместе с документом | мастер, батч 7 | closed |
| DS-041 | Занятие единицы хранится колонкой `equipment.active_transfer_id` (guard-UPDATE) | мастер, inferred | closed |
| DS-046 | Фильтр доски по складу, бейдж «в пути», склад по умолчанию строки товара (D58, D59), история оборудования читают местоположение с единицы и документов | мастер, inferred | closed |
| DS-048 | Конкурентность документа — как сейчас: `version` обязателен, сверяется до статуса, 409; Lock Order документ → остатки → единицы | мастер, as-is (D50, D51) | closed |
| DS-049 | Лимиты как сейчас: до 200 строк, примечание до 1000 символов, количество numeric(12,3) > 0, дубль товара/единицы — ошибка | мастер, as-is (D52) | closed |
| DS-051 | Отдельной таблицы аудита и аудита чтения нет | мастер, zap | closed |
| DS-052 | UI-конвенции: ошибка под полем, тост справа вверху 5 с, модальный `KT.uiDialog`, блокировка кнопки | мастер, inferred | closed |
| DS-054 | Миграция местоположения: единица хотя бы в одном незавершённом заказе → «на складе»; только в завершённых → «у клиента»; без заказов → «у клиента». Завершённый заказ = заказ в последней колонке доски (наибольший `columns.position`) | мастер, батч 7 | closed |
| DS-055 | Ключа идемпотентности нет: повтор отсекается версией/статусом; дубль чисто товарной заявки — риск принят | мастер, inferred | closed |
| DS-057 | Поиск единиц для строки документа — новый `e:GET /api/v1/equipment` (фильтры: склад, местоположение, строка поиска); доступен любому авторизованному | нарезка (пробел мастера: у «+ Оборудование» не было endpoint'а), inferred | closed |
| DS-002 | Четыре направления: клиент → склад, склад → клиент, склад → склад, поставщик → склад | мастер, батч 0 | closed — контекст, реализуется в `transfers-partners` |
| DS-018 | Исполнитель работает с документами с клиентом | мастер, батч 4 | closed — контекст, реализуется в `transfers-partners` |
| DS-045 | При добавлении единицы в заказ склад не выбирается | мастер, inferred | closed — контекст, реализуется в `transfers-partners` |
| DS-017 | «Отменить проведение» полученного документа | мастер, батч 3 | closed — контекст, реализуется в `transfers-revert` |
| DS-025 | Удаление документа и запрет удаления заказа с документами | мастер, батч 5 | closed — контекст, реализуется в `transfers-revert` |
| DS-031 | Показ времени по поясу пользователя | мастер, батч 6 | closed — контекст, реализуется в `user-timezone` |

## §10. Открытые вопросы (status: open)
Открытых вопросов нет.

## §11. Тестовые сценарии (черновик — вход для design-e2e-strategy, TD-6.8)
UI-scope (DS-028): swagger-сценариев нет; адаптив — один overflow-смок (TS-045); темы — обе (TS-046).

### Unit / Integration (Go)
- **TS-005.** Занятие единицы: создание второй заявки с той же единицей → 409 `EQUIPMENT_IN_TRANSFER` с `details.transfer_id`; после отмены первой — успех; снятая в `PUT` единица освобождается — DS-019, DS-041
  - негатив: два параллельных `POST` с одной единицей → ровно один 201
- **TS-006.** Отправка: единица не на складе отправления (на другом складе либо «у клиента») → 409 `TRANSFER_EQUIPMENT_NOT_AT_SOURCE` — DS-036
  - негатив: остатки и местоположение не изменились
  - не входит в эту часть: вариант «не у клиента» для направления «клиент → склад» — `transfers-partners`
- **TS-009.** Версия и статус: устаревшая `version` → 409 `TRANSFER_VERSION_CONFLICT` (сверяется до статуса); каждый недопустимый переход из §5 → 409 `TRANSFER_STATUS_CONFLICT` со своим сообщением — DS-048
  - негатив: двойной параллельный `receive` → один 200, один 409, остаток вырос один раз, единица принята один раз
- **TS-018.** Единица в незавершённом документе: `DELETE /tasks/:id/equipment/:equipment_id` → 409 `EQUIPMENT_IN_TRANSFER`; `DELETE /equipment/:id` при наличии строк документов → 409 `EQUIPMENT_HAS_TRANSFERS` — DS-026
  - негатив: после отмены документа единица убирается из заказа
- **TS-019.** Архивирование склада: на складе числится единица → 409 `WAREHOUSE_IN_USE`; есть заявка/в пути с этим складом → 409; иначе — как сейчас (в т.ч. `WAREHOUSE_LAST_ACTIVE`) — DS-026
  - негатив: склад только с завершёнными документами и без оборудования архивируется
- **TS-024.** Доска: `GET /tasks?warehouse_id=X` — заказ с единицей на X и заказ с единицей в пути к X; `has_in_transit` — по `equipment.location_status` — DS-046
  - негатив: единица «у клиента» не попадает ни под один склад
- **TS-026.** Миграция (integration): `x:stock_transfers` → `t:transfers`, `x:stock_transfer_items` → `t:transfer_items` с `kind = product`; местоположение: (а) единица только в завершённом заказе (заказ в последней колонке доски — наибольший `columns.position`) → `at_client`, склад пуст; (б) единица в незавершённом заказе `at_warehouse` → `at_warehouse`, тот же склад; (в) единица в незавершённом заказе `in_transit` → `at_warehouse`, склад отправления; (г) единица и в завершённом, и в незавершённом заказе → `at_warehouse`, склад из незавершённого; (д) единица в двух незавершённых заказах → склад из заказа с большим `id`; (е) единица без заказов → `at_client`; `x:task_equipment_movements` отсутствует; CHECK'и `i:transfer_items_kind_check`, `i:equipment_location_check` отклоняют невалидные строки — DS-029, DS-030, DS-054
  - негатив: вставка `in_transit` без `active_transfer_id` отклонена; заказ в колонке с `is_final = true`, но не последней по `position`, считается незавершённым; при единственной колонке все единицы → `at_client`
- **TS-027.** Удалённые endpoints: `POST /tasks/:id/equipment/:equipment_id/send`, `/receive`, `/cancel-transit`, `/api/v1/stock-transfers` → 404 — DS-003, DS-030
  - негатив: `POST /tasks/:id/equipment` с `warehouse_id` для единицы, уже лежащей на другом складе, — местоположение не меняется
  - не входит в эту часть: «`warehouse_id` игнорируется, единица остаётся у клиента» — `transfers-partners` (DS-045)
- **TS-028.** Fuzz валидатора `POST /api/v1/transfers` (произвольный JSON) — без паники, только 201/400/403/409 — TD-4
  - негатив: 500 недопустим

Дополнительно к мастер-сценариям (в объёме этой части, без новых номеров): валидатор строк `kind` × поля (§6.2) покрывается в составе TS-028 и TS-005; `e:GET /api/v1/equipment` (фильтры, конверт, `location_status: invalid` → 400) — в составе TS-024.

### E2E (Playwright)
- **TS-031.** Перевозка между складами: товар + оборудование клиента — экран §8.1 — DS-001
  - шаги: admin → «Новое перемещение» → склады → «+ Товар», «+ Оборудование» → «Отправить» → «Принять» с фактом меньше отправленного; проверка: расхождение подсвечено, в карточке заказа единица «на складе Москва»; негатив: факт больше отправленного → «Факт больше отправленного»
- **TS-036.** Карточка заказа без кнопок перемещения — экран §8.2 — DS-003
  - шаги: открыть карточку с оборудованием; проверка: кнопок «Отправить», «Принять», «Отменить отправку» нет; есть бейдж местоположения и «перемещение #N»; негатив: «Убрать» скрыта у единицы в незавершённом документе
  - не входит в эту часть: «select „Склад приёма“ отсутствует» — `transfers-partners` (DS-045)
- **TS-043.** Конкурентная правка документа — экран §8.1 — DS-048
  - шаги: два контекста открывают одну заявку, первый отправляет, второй сохраняет; проверка: у второго тост «Перемещение уже изменено, обновите», документ перечитан; негатив: данные первого не затёрты
- **TS-044.** Фильтр доски по складу и бейдж «в пути» — доска — DS-046
  - шаги: документ с единицей заказа отправлен; проверка: заказ виден под фильтром склада назначения, бейдж «в пути»; негатив: после приёмки бейджа нет
- **TS-045.** Overflow-смок (adaptive:min) — экраны §8.1, §8.2 — DS-028
  - шаги: вьюпорт 375×812, открыть каждый экран с документом; проверка: `scrollWidth <= clientWidth` у документа; негатив: нет
- **TS-046.** Темы (themes:both) — экраны §8.1, §8.2 — DS-028
  - шаги: светлая и тёмная тема; проверка: бейджи статуса и местоположения читаемы, жёлтый блок предупреждений контрастен; негатив: нет

### Фикстуры и сиды (АКЦЕНТ — хронический пропуск)
- **TS-F1.** `internal/seeddemo/seeddemo.go` (+ `seeddemo_integration_test.go`): переписать под новую модель в той же фазе, что миграция (§TD-1) — местоположение на `t:equipment` (единицы заказов в последней колонке доски — «у клиента»; единицы незавершённых заказов — «на складе» либо «в пути»; DS-054); документы «склад → склад»: один `received`, один `in_transit`, в одном из них строка-единица; строки `x:task_equipment_movements` убрать — DS-029. Части 4–6 дополняют сид своими документами (направления, накладная, откат).
- **TS-F2.** `e2e/fixtures/auth.ts`, `e2e/fixtures/board-types.ts`: wire-типы карточки (`location_status`, `active_transfer_id`, `transit_to_name`), убрать `equipment_status` / `movements`; payload'ы типизировать wire-типами из исходников, не сырыми ключами (§TD-6.2) — DS-008. Поля `transfers[]`, `transfers_amount` добавляют части 4 и 5.
- **TS-F3.** `e2e/pages/StockTransfersPage.ts` → `TransfersPage.ts`, `e2e/tests/stock-transfers.spec.ts` → `transfers.spec.ts`; `e2e/pages/OrderCardPage.ts` — убрать локаторы кнопок перемещения, добавить бейдж местоположения; `e2e/tests/equipment-warehouses.spec.ts` — переписать под документ — DS-030, DS-003
- **TS-F4.** Go-тесты под удаление/переписывание: `internal/features/board/equipment_warehouses_http_live_test.go`, `equipment_warehouses_migration_integration_test.go`, `equipment_warehouse_validate_test.go`, `order_card_http_live_test.go`, `warehouses_card_http_live_test.go`; `internal/features/stocktransfers/*_test.go` → пакет `transfers` — DS-003, DS-030

## §Impact — точки касания существующего кода
Источник: `impact_scan.py` мастера (sample `internal/features/stocktransfers`, 213 находок) — строки, относящиеся к этой части; отдельный прогон для части не делался (образец и термы те же).

| # | Класс | Что задеваем (file:line / объект) | Дельта фичи | Решение | DS |
|---|---|---|---|---|---|
| 1 | ENUMERATION | `internal/server/router.go` (11 вхождений) | регистрация пакета `transfers` вместо `stocktransfers` | переименовать | DS-030 |
| 2 | ENUMERATION | `internal/ssr/menu.go:25` | URL пункта «Перемещения» → `/transfers` (`AdminOnly` остаётся до части 4) | править | DS-030 |
| 3 | SHARED-DEP | `internal/features/stocktransfers/*` (dto, handler, repo, routes, service, ssr, validate), `web/ssr/stocktransfers` | переименование пакета и шаблонов; строки-единицы, занятие, смена местоположения | переписать в `internal/features/transfers` | DS-001, DS-019, DS-030 |
| 4 | ENUMERATION | `internal/features/board/routes.go:47-49` | удалить маршруты `send` / `receive` / `cancel-transit` | удалить | DS-003 |
| 5 | SHARED-DEP | `internal/features/board/task_equipment_service.go`, `task_equipment_repo.go`, `task_equipment_validate.go` | убрать переходы; склад приёма пишется на единицу; запрет удаления по `active_transfer_id` | править | DS-003, DS-008, DS-026 |
| 6 | SHARED-DEP | `internal/features/board/card_repo.go`, `card_dto.go`, `card_service.go` | местоположение с `t:equipment`, убрать `equipment_status` / `movements` | править | DS-008 |
| 7 | SHARED-DEP | `internal/features/board/tasks_repo.go:334` (фильтр доски), `tasks_dto.go`, `tasks_filter.go` | фильтр `warehouse_id`, `has_in_transit` по `t:equipment` и `t:transfers` | править | DS-046 |
| 8 | SHARED-DEP | `internal/features/board/stock_repo.go` (CTE «в пути») | имена таблиц; только строки `kind = product` | править | DS-030 |
| 9 | SHARED-DEP | пакет `internal/features/board` — склад по умолчанию строки товара (D58; точный файл определить при планировании: читает `task_equipment.warehouse_id`) | читать склад единицы с `t:equipment` | править | DS-046 |
| 10 | ENUMERATION | `internal/features/board/errors.go:756,758` | убрать `TASK_EQUIPMENT_IN_TRANSIT`, `TASK_EQUIPMENT_NOT_IN_TRANSIT`; новый код `EQUIPMENT_IN_TRANSFER` | править | DS-026 |
| 11 | SHARED-DEP | `internal/features/clients` — `history.go`, удаление единицы, новый список `GET /api/v1/equipment` (`routes.go:25-29`), `web/ssr/clients/templates/pages/equipment-history.html` | история по документам; 409 `EQUIPMENT_HAS_TRANSFERS`; новый endpoint | править | DS-046, DS-026, DS-057 |
| 12 | SHARED-DEP | `internal/features/warehouses/service.go` (архивирование) | 409 `WAREHOUSE_IN_USE` | править | DS-026 |
| 13 | FRONTEND | `web/ssr/_shared/js/stock-transfers.js` | переименовать в `transfers.js`; строки-единицы, combobox «+ Оборудование» | переписать | DS-030, DS-001 |
| 14 | FRONTEND | `web/ssr/_shared/js/task-card.js` (кнопки перемещения, журнал `movementText` — вызов `:492`) | убрать кнопки и журнал; бейдж местоположения, ссылка на документ | править | DS-003 |
| 15 | FRONTEND | `web/ssr/_shared/js/equipment-history.js`, `web/ssr/_shared/js/board.js` (бейдж «в пути») | новые поля ответа | править | DS-046 |
| 16 | FK | `migrations/0009` (таблицы, FK `stock_movements.transfer_id`), `0010` (колонки `task_equipment`, журнал) | новая миграция: rename таблиц и индексов, `transfer_items.kind` / `equipment_id`, колонки `equipment`, перенос местоположения (DS-054), DROP колонок `task_equipment` и таблицы `task_equipment_movements`; старые миграции не правим | добавить | DS-029, DS-030, DS-054 |
| 17 | ENUMERATION | `internal/seeddemo/seeddemo.go` (14), `seeddemo_integration_test.go` (5) | пересоздание демо под новую модель | переписать (TS-F1) | DS-029 |
| 18 | ENUMERATION | Go-тесты `internal/features/board/equipment_warehouses_*_test.go`, `order_card_http_live_test.go`, `warehouses_card_http_live_test.go`; `internal/features/stocktransfers/*_test.go` | удалить/переписать | TS-F4 | DS-003, DS-030 |
| 19 | ENUMERATION | `e2e/tests/stock-transfers.spec.ts` (21), `e2e/tests/equipment-warehouses.spec.ts`, `e2e/fixtures/auth.ts` (15), `e2e/fixtures/board-types.ts` (4), `e2e/pages/StockTransfersPage.ts` (6), `e2e/pages/OrderCardPage.ts` (2), `e2e/tests/board.spec.ts` | переименование и новые сценарии | TS-F2, TS-F3 | DS-030 |
| 20 | ENUMERATION | `project/.scaffold-lock.json` | проверить при планировании, нет ли в lock-файле каркаса путей `stocktransfers` | проверить | DS-030 |
| 21 | CONFIG | env-ключи | не задеваются | — | — |
| 22 | MONITORING | дашборды/алерты | не задеваются | — | — |

**Вне каталога фичи:** есть — internal/features/board, internal/features/clients, internal/features/warehouses, internal/features/stocktransfers, internal/server/router.go, internal/ssr/menu.go, internal/seeddemo, web/ssr/_shared/js, web/ssr/stocktransfers, web/ssr/clients, migrations, e2e

### prd.md
MISSING

## Инвентарь доков (точечный grep разрешён, полное чтение — нет)
- draft-spec.md — 117KB
- 0-setup/draft-spec.slice-light.md — 101KB
- 8-code-review/input-bundle.md — 32KB
- 0-setup/passport.md — 12KB
- 0-setup/idioms.md — 11KB
- 0-setup/light-progress.md — 8KB
- draft-spec-decisions.md — 2KB
- 0-setup/pipeline-state.md — 0KB
- 9-validation/evidence/last-run.json — 0KB

## Паспорт (выжимка; полный — 0-setup/passport.md)
# Passport: transfers-core (generated — руками не править; stage_bundle.py)
**Tier:** M | **Совместимость:** MVP (pipeline-doctrine.md §CM-1; ? → уточни у оркестратора)
**Трек:** обычный (prefs нет — дефолт) — режимы агентов (`spec-track delta/micro/mini/merged`) включает ТОЛЬКО строка `Режим:` в prompt оркестратора; DS/TS-индексы ниже присутствуют на ЛЮБОМ треке при наличии draft-spec.
**Layout:** автодетект (tools/layout_resolve.py)
**UI-scope (pipeline-doctrine.md §US-1, шапка draft-spec):** swagger:no | adaptive:min | themes:both
