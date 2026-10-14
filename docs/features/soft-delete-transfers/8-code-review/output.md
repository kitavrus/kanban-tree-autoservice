---
stage: 8-code-review
feature: soft-delete-transfers
tier: M
verdict: APPROVED
high: 0
medium: 0
low: 6
---
# Code Review: soft-delete-transfers — APPROVED
**Review level:** Standard (tier: M) · Режим: spec-track merged (Sec 17 по DS/TS, Sec 18 API-линза) · Диапазон: `ecc023b..c9b2350` (повторное ревью: `7dcd8bc..c9b2350` — коммиты `562e3a3`, `c9b2350`)

Гейты (build, unit, lint, integration -race 22 ok, E2E 216 passed + 1 flaky TR-021, после правки page object — 62 passed с `--repeat-each=2`) — по фактам судейства, не перегонялись. Сверка — против `draft-spec.md` §4/§6/§9/§11/§Impact.

## Блокеры (обязательно исправить до мержа)
Нет.

## Серьёзные замечания (исправить в этом PR)
Нет.

## Закрыто повторным ревью
1. **MEDIUM-1 (restore заявки с единицей вне заказа) — закрыт.** `internal/features/transfers/repo.go:801`.
   - Сверка — та же выборка `task_equipment … e.client_id`, что в `checkRefs`; единица вне заказа → `notInTaskError` → 409 `TRANSFER_EQUIPMENT_NOT_IN_TASK` с `details.equipment_id` (`service.go:61`, `mapErr` `service.go:469`).
   - Порядок блокировок не изменился: документ `FOR UPDATE` → заказ `FOR KEY SHARE` → чтение `task_equipment` (без блокировки) → equipment в `claimEquipment`. Снятие единицы с заказа идёт под `LockTask FOR UPDATE` (`board/task_equipment_service.go:114` → `ActiveTransfer` `:124`): либо уже видно restore'у (409), либо ждёт его и получает `EQUIPMENT_IN_TRANSFER`. Новых циклов нет.
   - `clientID` в запросе не nil: ветка `taskID != nil` выше уже вернула `errClientMismatch` при nil.
   - Отменённые документы сверку не проходят — верно: единиц не занимают, и то же состояние (отменённый документ с единицей, убранной из заказа) достижимо без удаления.
   - Отказ откатывает транзакцию целиком: документ остаётся удалённым, оставшаяся в заказе единица не занята.
2. **Тест `TestTS017_RestoreRequest_UnitRemovedFromTask` — корректен.** Единица убирается боевым API доски; проверены код, текст, `details.equipment_id`, строка документа (`requested|true|version+1`), незанятость обеих единиц, негатив с отменённым документом, успех после возврата единицы в заказ. Маппинг — в unit `validate_revert_test.go`.
3. **`fixture_sync` — PASS.** `seeddemo.go seedDeletedTransfer`: заявка «склад → склад», `requested`, три поля удаления вместе, `version = 2`, товарная строка, единиц не занимает — CHECK `transfers_deleted_chk` соблюдён.
4. **`seeddemo_integration_test.go` — смысл проверки сохранён.** Фильтр `t.status <> 'requested'` в «журнал = qty_sent» семантически точен: заявка движений `transfer_out` не создаёт. Маскировки нет — рядом закреплено, что заявка «склад → склад» ровно одна и именно удалённая (`statuses[...] == 1`, `deletedDocs == 1`), у неё 0 движений, 0 занятых единиц, 1 строка.
5. **SDT-002 — ветка без подмены добавлена** (`e2e/tests/soft-delete-transfers.spec.ts:222`): устаревшая версия → «Перемещение уже изменено, обновите», окно и причина остаются, документ не тронут. Ветка с `page.route` оставлена для статусного отказа — приемлемо.
6. **`e2e/pages/TransfersPage.ts currentId()`** — web-first ожидание `/#\d+/` вместо мгновенного чтения; `waitForTimeout` нет. Корректно.

## Незначительные замечания (можно в следующем PR)
1. **[docs/features/soft-delete-transfers/draft-spec.md §6.2, §9] Код `TRANSFER_EQUIPMENT_NOT_IN_TASK` отсутствует в спеке** — внести обещанный DS (строка ошибок §6.2, сценарий в §11 к TS-017) до Validation, иначе код опережает source of truth. Там же §8.2: текст тоста.
2. **[internal/features/warehouses/repo.go:136 `InUse`] Мягко удалённая заявка мешает архивированию склада.** Вне diff и §Impact; безопасно, но расходится с DS-026 и невидимо админу. Перенести в часть 6 (`soft-delete-warehouses-suppliers`) явным DS.
3. **[internal/features/clients/repo.go:524] Строки удалённой заявки держат единицу в справочнике (409 `EQUIPMENT_HAS_TRANSFERS`).** Приемлемо и необходимо (FK RESTRICT, `Restore` опирается на существование единицы); переписанный `TestTS018_DeleteRequestFreesEquipment` — следствие спеки. Сообщение ссылается на невидимый документ — учесть в части с мягким удалением оборудования.
4. **e2e_drift FAIL — ложный класс.** TS-040/TS-041 в заголовках `SDT-001…004`, шаги и негативы покрыты.
5. **[draft-spec.md §6.3, строка `PUT /api/v1/transfers/:id`] «смена `task_id` на удалённый заказ → 409 `TASK_DELETED`» недостижима** — `task_id` неизменяем (`service.go immutableGuards` → 400 `not_allowed`). Код корректен, поправить строку спеки.
6. **[internal/features/board/tasks_service.go:413 `deleteTaskTransfers`] Служебная уборка заказа: порядок «заказ FOR UPDATE → DELETE transfers» обратен `Restore`/`Update`** — теоретический 40P01, только тестовый адрес, класс существовал раньше. Туда же: подписи направления/статуса продублированы в `softdelete/transfers_source.go:23` и `web/ssr/_shared/js/transfers.js`.

## Что проверено по акцентам (первое ревью, без изменений)
- **Блокировки/гонки.** `SoftDelete`: документ → equipment; `Restore`: документ → заказ (`FOR KEY SHARE`) → equipment; `lockTransfer` с `deleted_at IS NULL` после ожидания блокировки отдаёт 404. Продуктовые пути board строки `transfers` не блокируют. DS-040 и «создание ↔ удаление заказа» детерминированы (`TestTS018_CreateVsDeleteTaskRace`, `TestTS017_RestoreRace_SharedUnit`).
- **Полнота `deleted_at IS NULL`.** `List`, `getTransfer`, `Head`, `lockTransfer`; `board/card_repo.go:109`, `tasks_repo.go:119`, `stock_repo.go:57`, `tasks_service.go:386`; `clients/history.go:139`. Через `active_transfer_id` и в проверке отката условие не нужно. Пропуск один — `warehouses.InUse` (LOW-2).
- **Удаление заказа с документами.** Документы, остатки, движения, местоположение не меняются (`TestTS014_DeleteTaskWithTransfersAnyStatus`); скоуп исполнителя — по направлению, документы удалённого заказа доводятся.
- **Права/валидация/контракт/миграция.** `softdelete.AdminOnly` + повтор в сервисе; причина и `version` → `details.fields[]`; ответы по §6.1/§6.2; 0019 up/down и CHECK — `TestTS012_Migration0019_UpDown`.
- **Существующие тесты** сверх спеки не ослаблены.

## Sec 18 — API-контракт
- `task_is_deleted` — non-null boolean в `TransferResponse` и `TransferSummary`.
- Ошибки — только `pkg/errorspkg`; новый 409 `TRANSFER_EQUIPMENT_NOT_IN_TASK` — тот же формат, `details.equipment_id` (аналог `EQUIPMENT_IN_TRANSFER`); wire-тип в `e2e/fixtures/auth.ts:253` обновлён.
- Breaking `version` query → body — DS-028, режим MVP (§CM-1), не флагается.
- `GET /api/v1/deleted?kind=transfers` — конверт `{items,total,limit,offset}`, N+1 нет. `api_conventions`, `go_naming` — pass; `contract` — неприменим (Vue нет); Sec 15 — n/a (`swagger:no`).

## Соответствие кода спеке (spec-трек)
- [x] §4.1 DDL/CHECK/индекс ↔ `migrations/0019_*` — ДА
- [x] §6.1 DELETE, §6.2 restore, §6.3 дельта — ДА (сверх спеки — код `TRANSFER_EQUIPMENT_NOT_IN_TASK`, LOW-1)
- [x] Ошибки §6 → свои коды и сообщения — ДА
- [x] §Impact «вне каталога фичи» — след в diff есть — ДА
- [x] DS-006, 007, 008, 018, 020, 024, 026, 028, 030, 038, 040 реализованы — ДА
- [x] as-is инварианты (в т.ч. `not_in_task`, DS-035) не нарушены — ДА
- [x] Scaffold-стабов в diff нет — ДА
- [x] TS-012, 014, 015, 016, 017, 018, 033, 050, 051, TS-040, TS-041, TS-F3/F4/F6/F7 покрыты — ДА
- Monitoring / Rollout / Runbook / Perf Budget — N/A

## Статус фаз
- [x] Все фазы закрыты: `0-setup/light-progress.md` — все пункты `- [x]` — ДА

## Итог
APPROVED: блокеров и серьёзных замечаний нет. До Validation — внести DS на `TRANSFER_EQUIPMENT_NOT_IN_TASK` в спеку (LOW-1); LOW-2/LOW-3 — в части 6 и 4/5 мастера.
