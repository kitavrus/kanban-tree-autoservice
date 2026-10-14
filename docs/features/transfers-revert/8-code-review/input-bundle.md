# Review Bundle: transfers-revert — CHECKS-FAILED
**Base:** c374f1b5492a16136e0ca6f37e66367fa3071eec | **HEAD:** e1fb45f | **Generated:** 2026-10-07 22:54 | diff: 36 files changed, 3290 insertions(+), 112 deletions(-)

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
| e2e_drift | ❌ | 1 | e2e-drift: FAIL (режим: TS (spec-track §ST-1.4), doc: 3 id, code: 0 в 0 файлах, проблем: 3)<br>DRIFT: TS-033 есть в draft-spec.md, теста test('TS-033: …') нет (сценарий без теста)<br>DRIFT: TS-037 есть в draft-spec.md, теста test('TS-037: …') нет (сценарий без теста)<br>DRIFT: TS-038 есть в draft-spec.md, теста test('TS-038: …') нет (сценарий без теста) |

## Diff по линзам
| Линза | Файлов | Patch |
|---|---|---|
| backend | 17 | 8-code-review/diff-backend.patch |
| migrations | 2 | 8-code-review/diff-migrations.patch |
| other | 11 | 8-code-review/diff-other.patch |
| docs | 6 | — (артефакты, не ревьюятся построчно) |

### backend
- `M` internal/features/board/equipment_location_http_live_test.go
- `A` internal/features/board/order_card_revert_http_live_test.go
- `M` internal/features/board/tasks_service.go
- `M` internal/features/transfers/dto.go
- `M` internal/features/transfers/handler.go
- `A` internal/features/transfers/migration_revert_integration_test.go
- `M` internal/features/transfers/repo.go
- `M` internal/features/transfers/routes.go
- `M` internal/features/transfers/service.go
- `M` internal/features/transfers/transfers_equipment_http_live_test.go
- `M` internal/features/transfers/transfers_http_live_test.go
- `A` internal/features/transfers/transfers_revert_http_live_test.go
- `M` internal/features/transfers/validate.go
- `A` internal/features/transfers/validate_revert_test.go
- `M` internal/features/transfers/validate_test.go
- `M` internal/seeddemo/seeddemo.go
- `M` internal/seeddemo/seeddemo_integration_test.go

### migrations
- `A` migrations/0017_transfers_revert.down.sql
- `A` migrations/0017_transfers_revert.up.sql

### other
- `M` e2e/fixtures/auth.ts
- `M` e2e/fixtures/board-types.ts
- `M` e2e/fixtures/test.ts
- `M` e2e/pages/OrderCardPage.ts
- `M` e2e/pages/TransfersPage.ts
- `M` e2e/tests/transfers-partners.spec.ts
- `A` e2e/tests/transfers-revert.spec.ts
- `M` e2e/tests/transfers-sales.spec.ts
- `M` web/ssr/_shared/js/task-card.js
- `M` web/ssr/_shared/js/transfers.js
- `M` web/ssr/transfers/templates/pages/list.html

### docs
- `A` docs/features/transfers-revert/0-setup/draft-spec.slice-light.md
- `A` docs/features/transfers-revert/0-setup/idioms.md
- `A` docs/features/transfers-revert/0-setup/light-progress.md
- `A` docs/features/transfers-revert/0-setup/passport.md
- `A` docs/features/transfers-revert/0-setup/pipeline-state.md
- `A` docs/features/transfers-revert/0-setup/timeline.jsonl

## Экстракты документов (вместо полного чтения)
### draft-spec.md (экстракт: шапка + DS + дельта + Impact + TS)
# Draft Spec: Перемещения — отмена проведения и удаление

**Tier:** M · **Совместимость:** MVP · **Режим:** new

**Дата:** 2026-10-07 | **Статус:** draft → final (после подтверждения пользователя) | **Tier:** M
**Совместимость:** MVP (DS-027 — контекст, реализуется в `transfers-core`) — живых данных нет
**UI-scope:** swagger:no | adaptive:min | themes:both (DS-028 — контекст, реализуется в `transfers-core`)
**Источник:** мастер-спека `docs/features/transfers/draft-spec.md` (§4.1, §4.6, §5, §6.5, §6.9, §6.15, §8.1, §8.3) + её журнал `docs/features/transfers/draft-spec-decisions.md`
**Часть 6 из 6 мастер-спеки `transfers`; зависит от: `transfers-partners` (4), `transfers-sales` (5)** (и через них от частей 1–3). Номера DS и TS — мастер-идентификаторы, не перенумерованы.

> Эта спека — дельта поверх частей 3–5. После мержа этой части мастер-спека реализована полностью: администратор может «Отменить проведение» полученного документа любого направления (документ возвращается в «В пути», остатки и местоположение откатываются), удалить документ в статусе «Заявка» или «Отменён»; заказ с неоткатанными документами не удаляется, и сообщение называет мешающие документы, а отменённые документы удаляются вместе с заказом. Вне объёма остаются мягкое удаление с восстановлением и «ждёт поставку» — отдельные следующие задачи.

## §9. Решения (DS-index)
| DS | Формулировка | Источник | Статус |
|---|---|---|---|
| DS-017 | «Отменить проведение»: администратор возвращает полученный документ в «В пути» с обязательной причиной; остатки и местоположение откатываются (пересмотр финальности «Получено», D51) | мастер, батч 3 | closed |
| DS-020 | Принцип «всё можно менять»: откат не блокируется учётными причинами | мастер, батч 4 | closed |
| DS-021 | Принцип: мягкое удаление с восстановлением + отчёт «Удалённые заказы» | мастер, батч 4 | closed (вне объёма — DS-022, реализации нет) |
| DS-022 | Мягкое удаление и отчёт «Удалённые заказы» — отдельная следующая задача, вне объёма `transfers` | мастер, батч 4 (уточнение) | closed (вне объёма, реализации нет) |
| DS-023 | Откат в обратном порядке: документ не отменить, пока его единицу увёз более поздний документ (система называет его); оплата заказа и уход остатка в минус откат не блокируют | мастер, батч 5 | closed |
| DS-025 | До мягкого удаления: заказ с неотменёнными/неоткатанными документами не удаляется (сообщение перечисляет их); документ удаляется только в «Заявка» или «Отменён» | мастер, батч 5 | closed |
| DS-044 | Откат фиксируется на документе: кто, когда, причина (последний откат) + slog | мастер, inferred | closed |
| DS-050 | При удалении заказа его отменённые документы удаляются вместе с ним | мастер, батч 7 | closed |
| DS-053 | Причина отката — 1–500 символов | мастер, inferred | closed |
| DS-004 | Состав строк по направлениям | мастер, батч 0 | closed — контекст, реализуется в `transfers-partners` |
| DS-012 | Заказ обязателен для документов с клиентом; временный общий 409 при удалении заказа | мастер, батч 2 | closed — контекст, реализуется в `transfers-partners` |
| DS-018 | Права: «Отменить проведение» — только администратор | мастер, батч 4 | closed — контекст, реализуется в `transfers-partners` |
| DS-040 | Причины движений остатка (`transfer_unreceive` добавлен в CHECK частью 4) | мастер, inferred | closed — контекст, реализуется в `transfers-partners` |
| DS-042 | Удаление документа — только администратор | мастер, батч 7 | closed — контекст, реализуется в `transfers-partners` |
| DS-014 | Сумма накладной в итоге заказа | мастер, батч 3 | closed — контекст, реализуется в `transfers-sales` |
| DS-027 | Совместимость — MVP | мастер, батч 6 | closed — контекст, реализуется в `transfers-core` |
| DS-028 | UI-scope: swagger:no, adaptive:min, themes:both | мастер, батч 6 | closed — контекст, реализуется в `transfers-core` |
| DS-031 | Показ времени по поясу пользователя | мастер, батч 6 | closed — контекст, реализуется в `user-timezone` |
| DS-048 | Конкурентность документа: `version`, 409 | мастер, as-is | closed — контекст, реализуется в `transfers-core` |
| DS-051 | Отдельной таблицы аудита нет | мастер, zap | closed — контекст, реализуется в `transfers-core` |
| DS-052 | UI-конвенции | мастер, inferred | closed — контекст, реализуется в `transfers-core` |

## §10. Открытые вопросы (status: open)
Открытых вопросов нет.

## §11. Тестовые сценарии (черновик — вход для design-e2e-strategy, TD-6.8)
UI-scope: swagger-сценариев нет; адаптив и темы — в составе TS-033 (диалог причины при 375×812 без горизонтальной прокрутки; красная кнопка «Удалить» и строка об откате читаемы в обеих темах).

### Unit / Integration (Go)
- **TS-010.** Отмена проведения: `received` → `in_transit` для каждого из четырёх направлений; компенсирующие движения `transfer_unreceive` на факт (склад → склад и приход от поставщика), для документов с клиентом движений нет; `qty_received`, `received_by`, `received_at` обнулены; единицы снова «в пути» и заняты документом; `unreceived_by`, `unreceived_at`, `unreceive_reason` заполнены; причина обязательна (пусто и пробелы → 400 `required`, 501 символ → 400 `too_long`); после отката проходит повторный `receive` с другим фактом и `cancel` — DS-017, DS-044, DS-053
  - негатив: executor → 403; откат из `requested`, `in_transit`, `cancelled` → 409 со своим сообщением; двойной параллельный откат → один 200, одно компенсирующее движение
- **TS-011.** Отмена проведения, товар израсходован: остаток склада назначения уходит в минус, ответ 200 с `warnings[].kind = negative` — DS-020, DS-023
  - негатив: откат не блокируется
- **TS-012.** Отмена проведения, единицу увёз более поздний документ → 409 `TRANSFER_UNRECEIVE_BLOCKED` с `details.transfer_id`; после отката более позднего — успех (обратный порядок); единица, которую более поздние документы увезли и вернули на тот же склад, — тоже 409 — DS-023
  - негатив: единица в другой незавершённой заявке → 409 с её номером; при отказе остатки, строки и местоположение не изменились
- **TS-013.** Отмена проведения накладной при оплаченном заказе: 200, `is_paid` заказа не изменился, `total_amount` по-прежнему включает накладную («В пути») — DS-023
  - негатив: последующая отмена накладной (`cancel`) убирает её сумму из `total_amount` и возвращает товар на склад
- **TS-016.** Удаление документа: `requested` и `cancelled` → 200, строки удалены, `stock_movements.transfer_id` = NULL, единицы заявки свободны; `in_transit` и `received` → 409 со своим сообщением — DS-025
  - негатив: без `version` → 400; устаревшая версия → 409; executor → 403; повтор → 404
- **TS-017.** Удаление заказа: документ в `requested` / `in_transit` / `received` → 409 `TASK_HAS_TRANSFERS` с `details.transfer_ids` (по возрастанию); только отменённые → заказ и они удалены одной транзакцией, движения остатков остались с `transfer_id` = NULL — DS-025, DS-050
  - негатив: при отказе остатки, документы и заказ не изменились; заказ без документов удаляется как раньше

Хвосты сценариев других частей, реализуемые здесь (без новых номеров):
- TS-015 (`transfers-partners`): executor → 403 на `unreceive` и `DELETE /transfers/:id`, включая документы с клиентом.
- TS-018 (`transfers-core`): после удаления документа-заявки единица убирается из заказа и удаляется из справочника, если других строк документов у неё нет.
- TS-009 (`transfers-core`): текст 409 при `cancel` полученного документа — «Перемещение получено — сначала отмените проведение».

### E2E (Playwright)
- **TS-033.** Отменить проведение — экран §8.1 — DS-017, DS-023
  - шаги: admin → полученный приход от поставщика → «Отменить проведение» → причина → подтвердить → «Принять» с другим фактом; проверка: после отката бейдж «В пути», строка «Проведение отменено: …» с причиной, остаток откатился; после повторной приёмки остаток соответствует новому факту, строка об откате осталась; диалог при 375×812 без горизонтальной прокрутки, читаем в обеих темах; негатив: пустая причина → «Укажите причину»; единицу увёз другой документ → в диалоге «Сначала отмените проведение перемещения #N» со ссылкой; исполнитель кнопки не видит
- **TS-037.** Запрет удаления заказа с документами — экран §8.2 — DS-025
  - шаги: admin → удалить заказ с проведённым документом; проверка: тост «Сначала отмените перемещения: #N» со ссылкой, заказ на месте; негатив: после отката, отмены документа заказ удаляется, отменённый документ исчез из списка перемещений
- **TS-038.** Удаление документа — экран §8.1 — DS-025
  - шаги: admin → заявка → «Удалить» → подтвердить; проверка: строка исчезла, тост «Удалено», единица заявки снова свободна (её можно добавить в новый документ); негатив: у документа «В пути» и «Получено» кнопки «Удалить» нет; исполнитель кнопки не видит

### Фикстуры и сиды (АКЦЕНТ — хронический пропуск)
- seed-demo (`internal/seeddemo/seeddemo.go`, владелец TS-F1 — `transfers-core`): дополнить в той же фазе, что миграция — один документ, откатанный и принятый заново (с причиной и компенсирующим движением `transfer_unreceive`), один отменённый документ у заказа (для проверки удаления заказа).
- e2e (`e2e/fixtures/board-types.ts`, владелец TS-F2 — `transfers-core`): wire-тип документа получает `unreceived_by`, `unreceived_by_name`, `unreceived_at`, `unreceive_reason`; тип ошибки `TASK_HAS_TRANSFERS` с `details.transfer_ids`; payload'ы типизировать wire-типами из исходников (§TD-6.2).
- Go-тест части 4 на общий 409 `CONFLICT` при удалении заказа — заменить на `TASK_HAS_TRANSFERS`.

## §Impact — точки касания существующего кода
Источник: строки `impact_scan.py` мастера, относящиеся к этой части, в состоянии после частей 3–5; отдельный прогон не делался: пакета `internal/features/transfers` на момент написания спеки нет.

| # | Класс | Что задеваем (file:line / объект) | Дельта фичи | Решение | DS |
|---|---|---|---|---|---|
| 1 | SHARED-DEP | `internal/features/transfers/*` (после частей 3–5; сейчас `internal/features/stocktransfers`: `routes.go:13-19`, `service.go`, `repo.go:335` `lockTransfer`) | новые операции `unreceive` и `delete`; поля отката в DTO; проверка порядка документов по единице | править | DS-017, DS-023, DS-025 |
| 2 | SHARED-DEP | `internal/features/board/tasks_service.go:332` (`Delete`), `errors.go` | `TASK_HAS_TRANSFERS` с номерами вместо общего 409 части 4; удаление отменённых документов в транзакции удаления заказа | править | DS-025, DS-050 |
| 3 | SHARED-DEP | зависимость `board` → `transfers` при удалении заказа | заказ удаляет пакет `board`, документы принадлежат пакету `transfers` — при планировании решить, кто выполняет SQL удаления отменённых документов (общий репозиторий либо вызов сервиса `transfers` в той же транзакции) | решение за планированием | DS-050 |
| 4 | FRONTEND | `web/ssr/_shared/js/transfers.js` (после ядра) | кнопки «Отменить проведение», «Удалить», диалог причины, строка об откате, значок «↩» в списке | править | DS-017, DS-025 |
| 5 | FRONTEND | `web/ssr/_shared/js/task-card.js` (удаление заказа), `web/ssr/_shared/js/board.js` (если удаление заказа вызывается с доски) | тост со ссылками на документы; строка про отменённые документы в диалоге | править | DS-025 |
| 6 | FK | `migrations/` | новая миграция: три колонки отката `t:transfers`, CHECK длины причины | добавить | DS-044 |
| 7 | ENUMERATION | `internal/seeddemo/seeddemo.go`, `e2e/fixtures/board-types.ts`, Go-тест части 4 на общий 409 | сид, wire-типы, замена ожидания | править | DS-025 |
| 8 | ENUMERATION | `docs/plan.md` (D51 — финальность «Получено») | дописать решение-пересмотр после реализации | после реализации | DS-017 |
| 9 | CONFIG | env-ключи | не задеваются | — | — |
| 10 | MONITORING | дашборды/алерты | не задеваются | — | — |

**Вне каталога фичи:** есть — internal/features/board, internal/features/transfers, internal/seeddemo, web/ssr/_shared/js, migrations, e2e, docs/plan.md

### prd.md
MISSING

## Инвентарь доков (точечный grep разрешён, полное чтение — нет)
- draft-spec.md — 68KB
- 0-setup/draft-spec.slice-light.md — 61KB
- 0-setup/idioms.md — 25KB
- 0-setup/passport.md — 9KB
- 0-setup/light-progress.md — 6KB
- draft-spec-decisions.md — 2KB
- 0-setup/pipeline-state.md — 0KB

## Паспорт (выжимка; полный — 0-setup/passport.md)
# Passport: transfers-revert (generated — руками не править; stage_bundle.py)
**Tier:** M | **Совместимость:** MVP (pipeline-doctrine.md §CM-1; ? → уточни у оркестратора)
**Трек:** обычный (prefs нет — дефолт) — режимы агентов (`spec-track delta/micro/mini/merged`) включает ТОЛЬКО строка `Режим:` в prompt оркестратора; DS/TS-индексы ниже присутствуют на ЛЮБОМ треке при наличии draft-spec.
**Layout:** автодетект (tools/layout_resolve.py)
**UI-scope (pipeline-doctrine.md §US-1, шапка draft-spec):** swagger:no | adaptive:min | themes:both
