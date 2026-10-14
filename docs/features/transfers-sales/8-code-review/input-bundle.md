# Review Bundle: transfers-sales — CHECKS-FAILED
**Base:** 4f032b43634216c1f6350eaf2e9a874f6c9db9d3 | **HEAD:** 06860c7 | **Generated:** 2026-10-07 22:08 | diff: 36 files changed, 2723 insertions(+), 111 deletions(-)

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
| e2e_drift | ❌ | 1 | e2e-drift: FAIL (режим: TS (spec-track §ST-1.4), doc: 1 id, code: 0 в 0 файлах, проблем: 1)<br>DRIFT: TS-030 есть в draft-spec.md, теста test('TS-030: …') нет (сценарий без теста) |

## Diff по линзам
| Линза | Файлов | Patch |
|---|---|---|
| backend | 20 | 8-code-review/diff-backend.patch |
| migrations | 2 | 8-code-review/diff-migrations.patch |
| other | 8 | 8-code-review/diff-other.patch |
| docs | 6 | — (артефакты, не ревьюятся построчно) |

### backend
- `M` internal/features/board/card_dto.go
- `M` internal/features/board/card_repo.go
- `M` internal/features/board/equipment_location_http_live_test.go
- `M` internal/features/board/order_card_http_live_test.go
- `A` internal/features/board/order_card_sales_http_live_test.go
- `M` internal/features/board/print_http_live_test.go
- `M` internal/features/board/tasks_dto.go
- `M` internal/features/board/tasks_repo.go
- `M` internal/features/board/tasks_service.go
- `M` internal/features/transfers/dto.go
- `A` internal/features/transfers/migration_sales_integration_test.go
- `M` internal/features/transfers/repo.go
- `M` internal/features/transfers/transfers_http_live_test.go
- `M` internal/features/transfers/transfers_partners_http_live_test.go
- `A` internal/features/transfers/transfers_sales_http_live_test.go
- `M` internal/features/transfers/validate.go
- `M` internal/features/transfers/validate_partners_test.go
- `A` internal/features/transfers/validate_sales_test.go
- `M` internal/seeddemo/seeddemo.go
- `M` internal/seeddemo/seeddemo_integration_test.go

### migrations
- `A` migrations/0016_transfers_sales.down.sql
- `A` migrations/0016_transfers_sales.up.sql

### other
- `M` e2e/fixtures/auth.ts
- `M` e2e/fixtures/board-types.ts
- `M` e2e/pages/OrderCardPage.ts
- `M` e2e/pages/TransfersPage.ts
- `A` e2e/tests/transfers-sales.spec.ts
- `M` web/ssr/_shared/js/task-card.js
- `M` web/ssr/_shared/js/transfers.js
- `M` web/ssr/board/templates/pages/print-handover.html

### docs
- `A` docs/features/transfers-sales/0-setup/draft-spec.slice-light.md
- `A` docs/features/transfers-sales/0-setup/idioms.md
- `A` docs/features/transfers-sales/0-setup/light-progress.md
- `A` docs/features/transfers-sales/0-setup/passport.md
- `A` docs/features/transfers-sales/0-setup/pipeline-state.md
- `A` docs/features/transfers-sales/0-setup/timeline.jsonl

## Экстракты документов (вместо полного чтения)
### draft-spec.md (экстракт: шапка + DS + дельта + Impact + TS)
# Draft Spec: Перемещения — накладная клиенту (товар с ценой, сумма, итог заказа)

**Tier:** S · **Совместимость:** MVP · **Режим:** new

**Дата:** 2026-10-07 | **Статус:** draft → final (после подтверждения пользователя) | **Tier:** S
**Совместимость:** MVP (DS-027 — контекст, реализуется в `transfers-core`) — живых данных нет
**UI-scope:** swagger:no | adaptive:min | themes:both (DS-028 — контекст, реализуется в `transfers-core`)
**Источник:** мастер-спека `docs/features/transfers/draft-spec.md` (§4.2, §4.6, §6.1, §6.2, §6.14, §8.1, §8.3) + её журнал `docs/features/transfers/draft-spec-decisions.md`
**Часть 5 из 6 мастер-спеки `transfers`; зависит от: `transfers-partners` (4)** (и через неё от частей 1–3). Номера DS и TS — мастер-идентификаторы, не перенумерованы.

> Эта спека — дельта поверх `transfers-partners`: документ «склад → клиент» уже существует, но везёт только оборудование. После мержа этой части: в него можно добавить товар с ценой продажи — документ становится накладной; товар списывается со склада при отправке; сумма накладной видна в документе, в списке и в карточке заказа и входит в итог заказа. Чего ещё нет: отката «Получено» и удаления документа (часть 6) — ошибочно отправленную накладную можно отменить, пока она «в пути»; полученную исправить нельзя до части 6.

## §9. Решения (DS-index)
| DS | Формулировка | Источник | Статус |
|---|---|---|---|
| DS-011 | В документе «склад → клиент» у товарных строк цена продажи, у документа — сумма (накладная); товар списывается со склада при отправке | мастер, батч 2 | closed |
| DS-014 | Сумма накладной входит в итог заказа (услуги + товары услуг + накладные), блок «Продано по накладным», отметка «Оплачен» одна; в сумму идут накладные «В пути» и «Получено» | мастер, батч 3 | closed |
| DS-038 | Для «склад → клиент» факт по товару не вводится; сумма накладной = Σ цена × отправленное количество | мастер, inferred | closed |
| DS-039 | Цена строки по умолчанию — цена товара из каталога, редактируется в документе | мастер, inferred | closed |
| DS-004 | Состав строк по направлениям | мастер, батч 0 | closed — контекст, реализуется в `transfers-partners` |
| DS-016 | В документе от поставщика цен нет | мастер, батч 3 | closed — контекст, реализуется в `transfers-partners` |
| DS-040 | Причины движений остатка (`client_out` добавлен в CHECK частью 4) | мастер, inferred | closed — контекст, реализуется в `transfers-partners` |
| DS-017 | «Отменить проведение» | мастер, батч 3 | closed — контекст, реализуется в `transfers-revert` |
| DS-023 | Откат не блокируется оплатой заказа; итог пересчитывается | мастер, батч 5 | closed — контекст, реализуется в `transfers-revert` |
| DS-027 | Совместимость — MVP | мастер, батч 6 | closed — контекст, реализуется в `transfers-core` |
| DS-028 | UI-scope: swagger:no, adaptive:min, themes:both | мастер, батч 6 | closed — контекст, реализуется в `transfers-core` |
| DS-031 | Показ времени по поясу пользователя | мастер, батч 6 | closed — контекст, реализуется в `user-timezone` |
| DS-048 | Конкурентность документа: `version`, 409 | мастер, as-is | closed — контекст, реализуется в `transfers-core` |
| DS-052 | UI-конвенции | мастер, inferred | closed — контекст, реализуется в `transfers-core` |

## §10. Открытые вопросы (status: open)
Открытых вопросов нет.

## §11. Тестовые сценарии (черновик — вход для design-e2e-strategy, TD-6.8)
UI-scope: swagger-сценариев нет; адаптив и темы — в составе TS-030 (таблица строк с колонками цены при 375×812 без горизонтальной прокрутки страницы; суммы читаемы в обеих темах).

### Unit / Integration (Go)
- **TS-003.** Валидатор цены: только товар в `warehouse_to_client`; отрицательная → `negative`; 3 знака после запятой, не число, больше 9999999999.99 → `invalid`; в `warehouse_to_warehouse`, `supplier_to_warehouse` и в строке-единице → `not_allowed`; не передана либо `null` → цена каталога на момент сохранения — DS-011, DS-039
  - негатив: `price` в `warehouse_to_warehouse` → 400; правка цены в `in_transit` → 409 `TRANSFER_STATUS_CONFLICT`
- **TS-014.** Итог заказа: `total_amount` = услуги + товары услуг + накладные в `in_transit` / `received`; заявка и отменённая не входят; `transfers_amount` в карточке; `total_amount` в `GET /tasks` совпадает с карточкой; `amount` в объекте документа и в списке = Σ `price × qty_sent`; накладная без товаров → `0.00` — DS-014, DS-038
  - негатив: накладная другого заказа не влияет; отмена накладной «в пути» убирает её сумму из `total_amount`; `is_paid` заказа при отправке накладной не меняется

Хвосты сценариев других частей, реализуемые здесь (без новых номеров):
- TS-007 (`transfers-partners`): отправка накладной → `client_out` −`qty_sent` на складе отправления; отмена «в пути» → `transfer_return` +`qty_sent`; приёмка — без движений; остаток `product_stock` сходится с суммой движений; двойная параллельная отправка → списание одно.
- TS-002 (`transfers-partners`): товар в `warehouse_to_client` теперь допустим (201), в `client_to_warehouse` по-прежнему `not_allowed`.

### E2E (Playwright)
- **TS-030.** Возврат клиенту с продажей товара — экран §8.2 → §8.1 — DS-011, DS-014
  - шаги: карточка заказа → «Вернуть клиенту» → «+ Товар», цена подставилась из каталога → изменить цену → «Отправить» → «Принять»; проверка: бейдж единицы «у клиента», «Сумма накладной» в документе, строка «Продано по накладным» и «Итого» в карточке выросли на сумму накладной, остаток товара на «Остатках» уменьшился; при 375×812 нет горизонтальной прокрутки страницы; суммы читаемы в светлой и тёмной теме; негатив: отрицательная цена → ошибка под полем; отмена накладной «в пути» → строка «Продано по накладным» исчезла, остаток вернулся

### Фикстуры и сиды (АКЦЕНТ — хронический пропуск)
- seed-demo (`internal/seeddemo/seeddemo.go`, владелец TS-F1 — `transfers-core`): дополнить в той же фазе, что миграция — одна накладная с товаром и ценой в оплаченном заказе (`received`), одна — «в пути» в неоплаченном; движения `client_out` и остатки правятся теми же дельтами.
- e2e (`e2e/fixtures/board-types.ts`, владелец TS-F2 — `transfers-core`): wire-типы — `amount`, `items[].price`, `items[].line_total` документа, `transfers_amount` и `transfers[].amount` карточки; payload'ы типизировать wire-типами из исходников (§TD-6.2).
- Существующие Go-тесты итога заказа (`internal/features/board/order_card_http_live_test.go:819`, `print_http_live_test.go:231,263`) — ожидания не меняются для заказов без накладных; добавить случай с накладной.

## §Impact — точки касания существующего кода
Источник: поиск `total_amount` по `internal/` (fff) + строки `impact_scan.py` мастера, относящиеся к этой части; отдельный прогон не делался: пакета `internal/features/transfers` на момент написания спеки нет.

| # | Класс | Что задеваем (file:line / объект) | Дельта фичи | Решение | DS |
|---|---|---|---|---|---|
| 1 | SHARED-DEP | `internal/features/board/tasks_repo.go:110-112` (`totalAmountSelectSQL`), использования `:277` (карточка) и `:365` (доска) | в выражение итога добавить накладные заказа в `in_transit` / `received` — одно место на оба экрана | править | DS-014 |
| 2 | SHARED-DEP | `internal/features/board/card_dto.go:12`, `card_repo.go`, `tasks_dto.go:31` | `transfers_amount`, `transfers[].amount` | править | DS-014 |
| 3 | SHARED-DEP | `internal/features/transfers/*` (после частей 3–4) | цена, сумма, товар в накладной, списание `client_out`, возврат при отмене | править | DS-011 |
| 4 | SHARED-DEP | `web/ssr/board/templates/pages/print-handover.html` + `internal/features/board/print_ssr.go` | строка «Продано по накладным» | править | DS-014 |
| 5 | FRONTEND | `web/ssr/_shared/js/transfers.js` (после ядра) | колонки «Цена», «Сумма», «Сумма накладной», «+ Товар» в возврате | править | DS-011 |
| 6 | FRONTEND | `web/ssr/_shared/js/task-card.js` (итоги заказа, блок «Перемещения заказа») | строка «Продано по накладным», суммы | править | DS-014 |
| 7 | FK | `migrations/` | новая миграция: `transfer_items.price`, CHECK цены, обновление CHECK `transfer_items_kind_check` | добавить | DS-011 |
| 8 | ENUMERATION | `internal/features/board/order_card_http_live_test.go:148,819,875`, `print_http_live_test.go:231,263`, `internal/seeddemo/seeddemo.go`, `e2e/fixtures/board-types.ts` | тесты итога, сид, wire-типы | дополнить | DS-014 |
| 9 | SHARED-DEP | `internal/features/executorpayouts/dto.go` | проверено поиском: `total_amount` в `internal/` встречается только в пакете `board` — отчёт «Выплаты» итог заказа не использует | не задевается | — |
| 10 | CONFIG | env-ключи | не задеваются | — | — |
| 11 | MONITORING | дашборды/алерты | не задеваются | — | — |

**Вне каталога фичи:** есть — internal/features/board, internal/features/transfers, internal/seeddemo, web/ssr/board, web/ssr/_shared/js, migrations, e2e

### prd.md
MISSING

## Инвентарь доков (точечный grep разрешён, полное чтение — нет)
- draft-spec.md — 51KB
- 0-setup/draft-spec.slice-light.md — 44KB
- 0-setup/idioms.md — 24KB
- 0-setup/passport.md — 5KB
- 0-setup/light-progress.md — 3KB
- draft-spec-decisions.md — 2KB
- 0-setup/pipeline-state.md — 0KB

## Паспорт (выжимка; полный — 0-setup/passport.md)
# Passport: transfers-sales (generated — руками не править; stage_bundle.py)
**Tier:** S | **Совместимость:** MVP (pipeline-doctrine.md §CM-1; ? → уточни у оркестратора)
**Трек:** обычный (prefs нет — дефолт) — режимы агентов (`spec-track delta/micro/mini/merged`) включает ТОЛЬКО строка `Режим:` в prompt оркестратора; DS/TS-индексы ниже присутствуют на ЛЮБОМ треке при наличии draft-spec.
**Layout:** автодетект (tools/layout_resolve.py)
**UI-scope (pipeline-doctrine.md §US-1, шапка draft-spec):** swagger:no | adaptive:min | themes:both
