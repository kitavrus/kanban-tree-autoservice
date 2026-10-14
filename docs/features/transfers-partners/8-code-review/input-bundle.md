# Review Bundle: transfers-partners — CHECKS-FAILED
**Base:** 49a637ddccbf45fe82535e7d6b52cd4cf599d4dc | **HEAD:** bbd3c4e | **Generated:** 2026-10-07 21:27 | diff: 67 files changed, 14209 insertions(+), 660 deletions(-)

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
| e2e_drift | ❌ | 1 | e2e-drift: FAIL (режим: TS (spec-track §ST-1.4), doc: 6 id, code: 0 в 0 файлах, проблем: 6)<br>DRIFT: TS-029 есть в draft-spec.md, теста test('TS-029: …') нет (сценарий без теста)<br>DRIFT: TS-032 есть в draft-spec.md, теста test('TS-032: …') нет (сценарий без теста)<br>DRIFT: TS-034 есть в draft-spec.md, теста test('TS-034: …') нет (сценарий без теста)<br>DRIFT: TS-035 есть в draft-spec.md, теста test('TS-035: …') нет (сценарий без теста)<br>DRIFT: TS-036 есть в draft-spec.md, теста test('TS-036: …') нет (сценарий без теста)<br>DRIFT: TS-039 есть в draft-spec.md, теста test('TS-039: …') нет (сценарий без теста) |

## Diff по линзам
| Линза | Файлов | Patch |
|---|---|---|
| backend | 33 | 8-code-review/diff-backend.patch |
| migrations | 2 | 8-code-review/diff-migrations.patch |
| other | 12 | 8-code-review/diff-other.patch |
| docs | 20 | — (артефакты, не ревьюятся построчно) |

### backend
- `M` internal/features/board/card_dto.go
- `M` internal/features/board/card_repo.go
- `M` internal/features/board/card_service.go
- `M` internal/features/board/equipment_location_http_live_test.go
- `M` internal/features/board/equipment_warehouse_validate_test.go
- `M` internal/features/board/order_card_http_live_test.go
- `M` internal/features/board/stock_repo.go
- `M` internal/features/board/task_equipment_repo.go
- `M` internal/features/board/task_equipment_service.go
- `M` internal/features/board/task_equipment_validate.go
- `M` internal/features/board/tasks_service.go
- `M` internal/features/clients/clients_http_live_test.go
- `M` internal/features/clients/history.go
- `M` internal/features/clients/repo.go
- `M` internal/features/transfers/dto.go
- `M` internal/features/transfers/handler.go
- `M` internal/features/transfers/migration_integration_test.go
- `M` internal/features/transfers/repo.go
- `M` internal/features/transfers/routes.go
- `M` internal/features/transfers/service.go
- `M` internal/features/transfers/ssr.go
- `M` internal/features/transfers/transfers_equipment_http_live_test.go
- `M` internal/features/transfers/transfers_http_live_test.go
- `A` internal/features/transfers/transfers_partners_http_live_test.go
- `M` internal/features/transfers/validate.go
- `A` internal/features/transfers/validate_partners_test.go
- `M` internal/features/transfers/validate_test.go
- `M` internal/features/warehouses/warehouses_in_use_http_live_test.go
- `M` internal/seeddemo/seeddemo.go
- `M` internal/seeddemo/seeddemo_integration_test.go
- `M` internal/ssr/menu.go
- `M` pkg/timezone/timezone.go
- `M` pkg/timezone/timezone_test.go

### migrations
- `A` migrations/0015_transfers_partners.down.sql
- `A` migrations/0015_transfers_partners.up.sql

### other
- `M` e2e/fixtures/auth.ts
- `M` e2e/fixtures/board-types.ts
- `M` e2e/pages/OrderCardPage.ts
- `M` e2e/pages/TransfersPage.ts
- `M` e2e/tests/equipment-warehouses.spec.ts
- `M` e2e/tests/order-card.spec.ts
- `A` e2e/tests/transfers-partners.spec.ts
- `M` e2e/tests/transfers.spec.ts
- `M` web/ssr/_shared/js/task-card.js
- `M` web/ssr/_shared/js/transfers.js
- `M` web/ssr/board/templates/pages/board.html
- `M` web/ssr/transfers/templates/pages/list.html

### docs
- `A` docs/features/transfers-partners/0-setup/draft-spec.slice-light.md
- `A` docs/features/transfers-partners/0-setup/idioms.md
- `A` docs/features/transfers-partners/0-setup/light-progress.md
- `A` docs/features/transfers-partners/0-setup/passport.md
- `A` docs/features/transfers-partners/0-setup/pipeline-state.md
- `A` docs/features/transfers-partners/0-setup/timeline.jsonl
- `A` docs/features/transfers-partners/8-code-review/checks/api_conventions.txt
- `A` docs/features/transfers-partners/8-code-review/checks/contract.txt
- `A` docs/features/transfers-partners/8-code-review/checks/date_bound.txt
- `A` docs/features/transfers-partners/8-code-review/checks/e2e_drift.txt
- `A` docs/features/transfers-partners/8-code-review/checks/e2e_mode.txt
- `A` docs/features/transfers-partners/8-code-review/checks/fixture_sync.txt
- `A` docs/features/transfers-partners/8-code-review/checks/fuzz_verdict.txt
- `A` docs/features/transfers-partners/8-code-review/checks/go_naming.txt
- `A` docs/features/transfers-partners/8-code-review/checks/live_suite.txt
- `A` docs/features/transfers-partners/8-code-review/checks/rule_usage.txt
- `A` docs/features/transfers-partners/8-code-review/diff-backend.patch
- `A` docs/features/transfers-partners/8-code-review/diff-migrations.patch
- `A` docs/features/transfers-partners/8-code-review/diff-other.patch
- `A` docs/features/transfers-partners/8-code-review/input-bundle.md

## Экстракты документов (вместо полного чтения)
### draft-spec.md (экстракт: шапка + DS + дельта + Impact + TS)
# Draft Spec: Перемещения — клиент и поставщик (направления, заказ, права исполнителя)

**Tier:** M · **Совместимость:** MVP · **Режим:** new

**Дата:** 2026-10-07 | **Статус:** draft → final (после подтверждения пользователя) | **Tier:** M
**Совместимость:** MVP (DS-027 — контекст, реализуется в `transfers-core`) — живых данных нет
**UI-scope:** swagger:no | adaptive:min | themes:both (DS-028 — контекст, реализуется в `transfers-core`)
**Источник:** мастер-спека `docs/features/transfers/draft-spec.md` (§1, §4.1, §4.2, §4.6, §5, §6.1–§6.8, §6.14, §6.15, §8.1, §8.3) + её журнал `docs/features/transfers/draft-spec-decisions.md`
**Часть 4 из 6 мастер-спеки `transfers`; зависит от: `suppliers` (1), `user-timezone` (2), `transfers-core` (3).** Номера DS и TS — мастер-идентификаторы, не перенумерованы.

> Эта спека — дельта поверх `transfers-core`: документ, строки, занятие единицы, optimistic lock и экран «Перемещения» уже существуют. После мержа этой части: оборудование принимается от клиента и возвращается клиенту документом по заказу; товары заказываются у поставщика и принимаются по факту; исполнитель работает с документами с клиентом; в списке есть фильтр «период». Чего ещё нет: товара с ценой в документе «склад → клиент» (часть 5 — пока клиенту уходит только оборудование, товар идёт через услуги заказа, итог заказа не меняется); отката «Получено» и удаления документа (часть 6); удаление заказа с документами отвечает 409 с общим текстом.

## §9. Решения (DS-index)
| DS | Формулировка | Источник | Статус |
|---|---|---|---|
| DS-002 | Контрагент — точка маршрута; направления: клиент → склад, склад → клиент, склад → склад, поставщик → склад | мастер, батч 0 | closed |
| DS-004 | Состав строк по направлениям: от клиента — только присланное клиентом; клиенту — его оборудование (+ товар — в части 5); между складами — оборудование и товары; от поставщика — товары | мастер, батч 0 | closed |
| DS-012 | Для направлений с клиентом заказ обязателен; клиент документа выводится сервером из заказа; заказ с документами не удалить | мастер, батч 2 | closed |
| DS-013 | Три шага «Заявка → В пути → Получено» для всех направлений; «Принять» у «склад → клиент» нажимает наш сотрудник; «клиент → склад» можно создать и принять одним действием | мастер, батч 2 | closed |
| DS-015 | Приход от поставщика: приёмка по факту; недопоставка закрывает документ, довоз — новым документом; факт больше заказанного разрешён только для этого направления | мастер, батч 3 | closed |
| DS-016 | В документе от поставщика цен нет; товары только из каталога | мастер, батч 3 | closed |
| DS-018 | Исполнитель создаёт и проводит документы с клиентом и видит в списке только их; «склад → склад», «поставщик → склад» — только администратор | мастер, батч 4 | closed |
| DS-024 | «Ждёт поставку» — отдельная задача; связь прихода от поставщика со строками товаров заказа не закладывается | мастер, батч 5 | closed (вне объёма, реализации нет) |
| DS-034 | В списке перемещений фильтр «период» (дата создания, с — по); границы дня — по поясу пользователя, в SQL — в UTC | мастер, батч 6 (уточнение 3) | closed |
| DS-035 | Единица в документе с клиентом должна входить в оборудование заказа документа | мастер, inferred | closed |
| DS-040 | Новые причины движений остатка: `supplier_in` (пишется здесь), `client_out` (часть 5), `transfer_unreceive` (часть 6); CHECK расширяется здесь сразу тремя; от поставщика при отправке движений нет | мастер, inferred | closed |
| DS-042 | Документ чужого направления для исполнителя → 404; удаление документа — только администратор (endpoint удаления — часть 6) | мастер, батч 7 | closed |
| DS-045 | При добавлении единицы в заказ склад не выбирается; единица остаётся там, где числится | мастер, inferred | closed |
| DS-047 | «В пути» на «Остатках» включает приход от поставщика в статусе «В пути» | мастер, inferred | closed |
| DS-007 | Справочник «Поставщики» | мастер, батч 1 | closed — контекст, реализуется в `suppliers` |
| DS-011 | Товар с ценой в документе «склад → клиент» | мастер, батч 2 | closed — контекст, реализуется в `transfers-sales` |
| DS-017 | «Отменить проведение» | мастер, батч 3 | closed — контекст, реализуется в `transfers-revert` |
| DS-025 | Удаление документа; сообщение с номерами при удалении заказа с документами | мастер, батч 5 | closed — контекст, реализуется в `transfers-revert` |
| DS-050 | Отменённые документы удаляются вместе с заказом | мастер, батч 7 | closed — контекст, реализуется в `transfers-revert` |
| DS-027 | Совместимость — MVP | мастер, батч 6 | closed — контекст, реализуется в `transfers-core` |
| DS-028 | UI-scope: swagger:no, adaptive:min, themes:both | мастер, батч 6 | closed — контекст, реализуется в `transfers-core` |
| DS-031 | Показ времени по поясу пользователя | мастер, батч 6 | closed — контекст, реализуется в `user-timezone` |
| DS-048 | Конкурентность документа: `version`, 409 | мастер, as-is | closed — контекст, реализуется в `transfers-core` |
| DS-052 | UI-конвенции | мастер, inferred | closed — контекст, реализуется в `transfers-core` |

## §10. Открытые вопросы (status: open)
Открытых вопросов нет.

## §11. Тестовые сценарии (черновик — вход для design-e2e-strategy, TD-6.8)
UI-scope: swagger-сценариев нет; адаптив и темы — общие сценарии TS-045, TS-046 части `transfers-core` расширяются формой документа с клиентом и блоком «Перемещения заказа» (в составе TS-029).

### Unit / Integration (Go)
- **TS-001.** Валидатор создания документа: `direction` required/enum; набор `from_warehouse_id` / `to_warehouse_id` / `task_id` / `supplier_id` по каждому из четырёх направлений (лишнее поле → `not_allowed`, недостающее → `required`); `same_warehouse`; `direction` и `task_id` в `PUT` → `not_allowed` — DS-002, DS-012
  - негатив: `client_to_warehouse` без `task_id` → 400 `task_id: required`
- **TS-002.** Валидатор строк: матрица «направление × kind» (товар в `client_to_warehouse` и в `warehouse_to_client` → `not_allowed`; единица в `supplier_to_warehouse` → `not_allowed`) — DS-004
  - негатив: `items[0].kind: not_allowed`
- **TS-004.** Единица не из заказа документа → 400 `items[i].equipment_id: not_in_task`; единица другого клиента → 400 — DS-035
  - негатив: `client_id` в теле игнорируется, берётся из заказа (DS-012)
- **TS-007.** Движения остатка (integration): приход от поставщика — при отправке движений нет, при приёмке `supplier_in` +факт на складе назначения, при отмене движений нет; документы с клиентом движений не создают; остаток `product_stock` сходится с суммой движений — DS-040
  - негатив: факт 0 — движение не пишется, документ «Получено»
  - не входит в эту часть: `client_out` — `transfers-sales` (TS-030); `transfer_unreceive` — `transfers-revert` (TS-010)
- **TS-008.** Приёмка: `warehouse_to_warehouse` факт > отправленного → 400 `exceeds_sent`; `supplier_to_warehouse` факт > заказанного → 200, остаток +факт; факт < заказанного → остаток +факт; `items` у `warehouse_to_client` и `client_to_warehouse` → 400 `not_allowed`; `receive: true` вне `client_to_warehouse` → 400 — DS-015, DS-013
  - негатив: отрицательный факт → 400; двойной параллельный `receive` прихода → остаток вырос один раз
- **TS-015.** Права: executor — `POST` `warehouse_to_warehouse` / `supplier_to_warehouse` → 403; `GET`, `PUT`, `send`, `receive`, `cancel` документа чужого направления → 404; список не содержит чужих направлений даже с фильтром `direction` — DS-018, DS-042
  - негатив: executor создаёт и проводит `client_to_warehouse` → 201 / 200
  - не входит в эту часть: `unreceive`, `DELETE` → 403 — `transfers-revert`; `/suppliers` → 403 — `suppliers` (TS-020)
- **TS-023.** Фильтр «период», граница суток: документ создан `2026-10-06T19:30:00Z` (00:30 7 октября по Алматы, 22:30 6 октября по Москве). Пользователь с `Asia/Almaty`: `created_from=2026-10-07&created_to=2026-10-07` — найден, `2026-10-06` — нет. Пользователь с `Europe/Moscow`: `2026-10-06` — найден, `2026-10-07` — нет — DS-034
  - негатив: `created_to` раньше `created_from` → 400; `2026-13-40` → 400
- **TS-025.** «Остатки»: «в пути» включает `supplier_to_warehouse` в `in_transit` для склада назначения — DS-047
  - негатив: заявка от поставщика (`requested`) в «в пути» не входит

Хвосты сценариев других частей, реализуемые здесь (без новых номеров):
- TS-006 (ядро): отправка `client_to_warehouse`, когда единица не «у клиента» → 409 `TRANSFER_EQUIPMENT_NOT_AT_SOURCE`.
- TS-027 (ядро): `POST /tasks/:id/equipment` с `warehouse_id` — поле игнорируется, единица остаётся «у клиента».
- TS-020 (`suppliers`): архивный поставщик в новом документе → 400 `supplier_id: archived`; в уже созданном остаётся.
- Удаление заказа с документом → 409 `CONFLICT`, заказ и документ на месте (DS-012).

### E2E (Playwright)
- **TS-029.** Приём в ремонт из карточки заказа — экран §8.2 → §8.1 — DS-012, DS-013
  - шаги: исполнитель → карточка заказа → «Принять от клиента» → склад → «Принять сразу»; проверка: документ «Получено» в блоке «Перемещения заказа», бейдж единицы «на складе Алматы»; затем «Вернуть клиенту» → «Отправить» → «Принять»: бейдж «у клиента»; блок и форма без горизонтальной прокрутки при 375×812, читаемы в обеих темах; негатив: повторное «Принять от клиента» той же единицы в незавершённом документе → «уже в перемещении #N»
- **TS-032.** Приход от поставщика — экран §8.1 — DS-015
  - шаги: admin → «Приход от поставщика» → поставщик, товар 10 → «Отправить» → «Принять», факт 12; проверка: «Получено», «заказано 10 / получено 12», остаток +12; негатив: кнопки «+ Оборудование» в этом направлении нет; архивного поставщика нет в select (хвост TS-035)
- **TS-034.** Права исполнителя — экран §8.1 — DS-018
  - шаги: executor → «Перемещения»; проверка: в меню «Новое перемещение» два направления, в списке только документы с клиентом, фильтра «Поставщик» нет; негатив: прямой URL `/transfers?id=<склад→склад>` → «Перемещение не найдено»
- **TS-039.** Фильтры списка, включая «период» — экран §8.1 — DS-034
  - шаги: статус + направление + период; проверка: список соответствует, «Сбросить» возвращает всё; негатив: «по» раньше «с» → «Некорректный период»

Хвост TS-036 (ядро), реализуемый здесь: в форме добавления единицы в заказ нет select «Склад приёма».

### Фикстуры и сиды (АКЦЕНТ — хронический пропуск)
- seed-demo (`internal/seeddemo/seeddemo.go`, владелец TS-F1 — `transfers-core`): дополнить в той же фазе, что миграция — два поставщика (один в архиве); документы трёх новых направлений: приём от клиента `received`, возврат клиенту `in_transit`, приход от поставщика `received` с расхождением и один `in_transit`; у существующих документов `direction = warehouse_to_warehouse`; единицы заказов в последней колонке доски — «у клиента» через проведённую пару «приём + возврат».
- e2e (`e2e/fixtures/board-types.ts`, владелец TS-F2 — `transfers-core`): wire-типы — `transfers[]` карточки, `direction`, `task_id`, `client_*`, `supplier_*` документа; хелпер `addTaskEquipment` без `warehouse_id`; payload'ы типизировать wire-типами из исходников (§TD-6.2).
- Фикстура времени TS-F5 (владелец — `user-timezone`) переиспользуется для TS-023.

## §Impact — точки касания существующего кода
Источник: `impact_scan.py` мастера (213 находок) — строки, относящиеся к этой части, в состоянии после `transfers-core`; отдельный прогон не делался: пакет `internal/features/transfers` на момент написания спеки не существует.

| # | Класс | Что задеваем (file:line / объект) | Дельта фичи | Решение | DS |
|---|---|---|---|---|---|
| 1 | SHARED-DEP | `internal/features/transfers/*` (после ядра; сейчас `internal/features/stocktransfers`) | направления, контрагенты, скоуп исполнителя, приёмка от поставщика, фильтры | править | DS-002, DS-018, DS-015 |
| 2 | ENUMERATION | `internal/features/transfers/routes.go` (сейчас `stocktransfers/routes.go:12,24` — `RequireAdmin` на группе API и SSR) | снять admin-guard с группы; права — в сервисе по направлению | править | DS-018 |
| 3 | ENUMERATION | `internal/ssr/menu.go:25` | снять `AdminOnly` у «Перемещения» | править | DS-018 |
| 4 | SHARED-DEP | `internal/features/board/task_equipment_service.go`, `task_equipment_validate.go`, DTO добавления единицы | убрать `warehouse_id` и запись склада | править | DS-045 |
| 5 | SHARED-DEP | `internal/features/board/card_repo.go`, `card_dto.go` | `transfers[]`, `transit_to_name` = «клиент» | править | DS-012 |
| 6 | SHARED-DEP | `internal/features/board/tasks_service.go:332` (`Delete`) | 409 при наличии документов заказа | править | DS-012 |
| 7 | SHARED-DEP | `internal/features/board/stock_repo.go` (CTE «в пути»), `tasks_repo.go` (фильтр доски) | приход от поставщика в «в пути»; единица в пути к клиенту | править | DS-047, DS-013 |
| 8 | SHARED-DEP | `internal/features/suppliers` (часть 1) | чтение поставщика, проверка `is_active` | использовать | DS-002 |
| 9 | SHARED-DEP | `users.timezone` (часть 2), `internal/server/middleware/auth.go` (`Viewer`) | пояс запрашивающего для фильтра «период» | использовать | DS-034 |
| 10 | FRONTEND | `web/ssr/_shared/js/transfers.js` (после ядра), шаблон страницы | меню направлений, поля шапки, фильтры, «Принять сразу» | править | DS-002, DS-034 |
| 11 | FRONTEND | `web/ssr/_shared/js/task-card.js` | блок «Перемещения заказа», кнопки; убрать select «Склад приёма» | править | DS-012, DS-045 |
| 12 | FK | `migrations/` | новая миграция: колонки и CHECK маршрута `t:transfers`, DROP CHECK `qty_received <= qty_sent`, расширение `stock_movements_reason_check` (сейчас `migrations/0008_warehouses.up.sql:70-72`) | добавить | DS-002, DS-015, DS-040 |
| 13 | ENUMERATION | `internal/seeddemo/seeddemo.go`, `e2e/fixtures/board-types.ts`, `e2e/fixtures/board.ts:8` (`addTaskEquipment`), `e2e/pages/OrderCardPage.ts`, `e2e/tests/order-card.spec.ts` | сид, wire-типы, хелпер без `warehouse_id` | править | DS-045 |
| 14 | CONFIG | env-ключи | не задеваются | — | — |
| 15 | MONITORING | дашборды/алерты | не задеваются | — | — |

**Вне каталога фичи:** есть — internal/features/board, internal/features/transfers, internal/ssr/menu.go, internal/seeddemo, web/ssr/_shared/js, migrations, e2e

### prd.md
MISSING

## Инвентарь доков (точечный grep разрешён, полное чтение — нет)
- draft-spec.md — 104KB
- 0-setup/draft-spec.slice-light.md — 91KB
- 8-code-review/input-bundle.md — 26KB
- 0-setup/idioms.md — 15KB
- 0-setup/light-progress.md — 11KB
- 0-setup/passport.md — 11KB
- draft-spec-decisions.md — 2KB
- 0-setup/pipeline-state.md — 0KB
- 9-validation/evidence/last-run.json — 0KB

## Паспорт (выжимка; полный — 0-setup/passport.md)
# Passport: transfers-partners (generated — руками не править; stage_bundle.py)
**Tier:** M | **Совместимость:** MVP (pipeline-doctrine.md §CM-1; ? → уточни у оркестратора)
**Трек:** обычный (prefs нет — дефолт) — режимы агентов (`spec-track delta/micro/mini/merged`) включает ТОЛЬКО строка `Режим:` в prompt оркестратора; DS/TS-индексы ниже присутствуют на ЛЮБОМ треке при наличии draft-spec.
**Layout:** автодетект (tools/layout_resolve.py)
**UI-scope (pipeline-doctrine.md §US-1, шапка draft-spec):** swagger:no | adaptive:min | themes:both
