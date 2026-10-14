---
stage: 8-code-review
feature: soft-delete-clients
tier: M
verdict: APPROVED
high: 0
medium: 1
low: 7
---
# Code Review: soft-delete-clients — APPROVED
**Review level:** Standard (tier: M) · Режим: spec-track merged (Sec 17 по DS/TS, Sec 18 API-линза) · Диапазон: 801b3a5..ad99362 (повторное ревью: 2dd507e..ad99362)
**Метод:** bundle + diff + точечное чтение кода; гейты, чекеры и тесты не перегонялись (факты прогонов — оркестратора).

## Блокеры (обязательно исправить до мержа)
Нет.

## Закрыто повторным ревью (ad99362)
1. **Блокер «отмена проведения возвращает удалённую единицу в путь» — закрыт.** `internal/features/transfers/repo.go` `unsettleEquipment`: `states[eq].deleted` → `deletedRefError` → 409 `TRANSFER_HAS_DELETED_REFS` (`details.kind = equipment`, `details.id`); признак читается под `lockEquipment` (`FOR NO KEY UPDATE`), UPDATE с `deleted_at IS NULL AND active_transfer_id IS NULL` + сверка числа строк. Страховка в `doSend`. Комментарии об инварианте (`clients/soft_delete.go`, `checkRefs`) исправлены.
   - Иные пути «удалённая единица не у клиента» не найдены: `equipment.location_status` / `warehouse_id` / `active_transfer_id` пишут только `transfers/repo.go` (claim, send, settle, unsettle, отмена, удаление и уборка заявки) и `seeddemo`; `board`, `clients`, `warehouses` местоположение не пишут, ручной правки местоположения нет. Приёмка, отмена и отправка требуют единицу, занятую этим документом, — такую удалить нельзя.
   - Тесты: `TestTS053_UnreceiveRefusesDeletedUnit` (единица отдельно / вместе с клиентом; документ остаётся `received`, местоположение не изменилось; после восстановления отмена проходит), `TestRace_UnreceiveWaitsForUnitDelete` — корректны.
2. **TR-014 — закрыт.** `web/ssr/_shared/js/transfers.js`: у прихода от поставщика после загрузки списка обновляются только опции `#st-supplier` (`refreshSupplierOptions`, выбор сохраняется, `_cbSync`); полная перерисовка осталась только у документа заказа. E2E TR-028 закрепляет. Полный E2E: 222 passed, flaky 0.
3. **Гонки `FOR SHARE` — покрыты:** `TestRace_TaskEquipmentAddWaitsForUnitDelete`, `TestRace_UnitDeleteWaitsForTaskEquipmentAdd`, `TestRace_SubtaskAssignWaitsForUnitDelete`, `TestRace_ClientRestoreVsUnitOperations`.
   - Поведение «добавление в заказ и создание документа не ждут восстановление клиента, сразу 400 `not_found`» — приемлемо: условие `deleted_at IS NULL` отсекает строку по снимку до блокировки, исход равен порядку «операция раньше восстановления»; инварианты не нарушаются, повтор после фиксации проходит.
4. **Комментарии `ErrTaskDeleted` / `ErrHasDeletedRefs`** — расставлены.

## Серьёзные замечания (исправить в этом PR)
1. **[docs/features/soft-delete-clients/draft-spec.md §6.5, §9 DS-045; мастер `docs/features/soft-delete/draft-spec.md`]** Синхронизировать спеку с кодом: DS-045 распространён на отмену проведения (`POST /api/v1/transfers/:id/unreceive` → 409 `TRANSFER_HAS_DELETED_REFS`), добавить строку в §6.5 и сценарий в TS-053; строка `e:POST /api/v1/subtasks` — `equipment_id` при создании услуги не принимается (`board/card_dto.go:184`), проверка только в PUT.

## Незначительные замечания (можно в следующем PR)
1. **[web/ssr/_shared/js/transfers.js `newDocument`, ветка `withTask`]** Документ заказа по-прежнему перерисовывается целиком после `loadTaskCard` — тот же класс гонки (вероятная причина нестабильного TR-021). Вынести отдельной задачей: фикс другой (карточка заказа приносит оборудование и предвыбор — нужно сохранять введённое, а не просто обновить опции), к этой части не относится, полный E2E зелёный без flaky.
2. **[internal/features/equipmenttypes/repo.go:43, :174]** Счётчик единиц типа считает мягко удалённые; тип с удалённой единицей — `EQUIPMENT_TYPE_IN_USE`. Адресат — часть 5.
3. **[web/ssr/_shared/js/board.js `ensureClientOption`]** `?client_id=` удалённого клиента без заказов → `GET /api/v1/clients/:id` 404 в консоли. Приемлемо: только устаревшая ссылка.
4. **[internal/features/clients/soft_delete.go:163-173 vs :269-274]** Разный приоритет отказов «не у клиента» / «в документе» у клиента и единицы; спека порядок не задаёт.
5. **[internal/features/transfers/repo.go `deletedAsField`]** При гонке называется одна строка (первая удалённая по id). Приемлемо.
6. **[internal/features/clients/clients_soft_delete_http_live_test.go `holdUnitDelete`, `TestRace_UnitDeleteWaitsForTaskEquipmentAdd`, `TestRace_ClientRestoreVsUnitOperations`]** Удерживающая сторона гонки повторяет SQL продукта в тесте — при смене запросов продукта тест молча разойдётся с ним. Приемлемо, как и в прежних гонках пакета.
7. **[checks/e2e_drift.txt]** Ложный FAIL (чекер смотрит `scripts/e2e`, канон имени `SDCL-001 (TS-043): …`); `contract` SKIP-RESOLVE — n/a (Vue нет). Порядок вкладок по реестру видов — приемлемо.

## Проверено без замечаний
- **Блокировки:** порядок заказ → клиент → equipment един для board, transfers, clients; циклов нет. `FOR SHARE` на единице обязателен и корректен; board строку equipment не пишет. `SELECT … FOR NO KEY UPDATE` + guard-UPDATE при удалении — корректно.
- **Каскад:** единица, удалённая отдельно до клиента, при восстановлении клиента остаётся удалённой и восстанавливается отдельно после него (TS-023).
- **DS-041:** `location_status`, `active_transfer_id` под блокировкой строк; заявка занимает единицу при создании → `CLIENT_EQUIPMENT_IN_TRANSFER`; новая единица ↔ удаление клиента (`CreateEquipment` `FOR KEY SHARE`); отмена проведения — см. выше.
- **DS-045:** `Restore` и `Unreceive` → 409 `TRANSFER_HAS_DELETED_REFS` с `kind` / `id`; отменённый документ не проверяется.
- **«Не удалён» в чтениях:** clients (списки, карточка, поиск, история), board (`ClientExists`, `EquipmentClient`, `Deleted`), transfers (`checkRefs`, `claimEquipment`). Пометки `client_is_deleted`, `is_deleted`, `equipment_is_deleted` — тем же запросом, без N+1.
- **Миграция 0021:** CHECK-и, FK `deleted_by` RESTRICT, частичные индексы; down симметричен.
- **SSR:** причина — 422 с окном, 409 — красный блок, успех — PRG + тост; права — `RequireAdmin` + проверка роли в сервисе до поиска записи.
- **Тесты:** BD-017 / BD-018 и снятые отказы — замена по TS-F4 / TS-F6; `t.Skip` / `test.fixme` / `waitForTimeout` в диффе нет.
- **Фикс 2dd507e:** корректен, BD-009 не затронут.
- **Security / Sec 18:** параметры pgx, ошибки через `errorspkg`, причина в журнал не пишется; коды и тексты §6.1–§6.5 совпадают; `deleted_with` — null либо `client`.

## Соответствие коду дизайну (верификация реализации)
- [x] Dataflow соответствует draft-spec §3/§7 — ДА
- [x] Инварианты §4 / DS реализованы — ДА
- [x] Go слои — as-is структура пакетов, новых отклонений нет — ДА
- [x] Error handling соответствует §6 — ДА (расширение DS-045 на отмену проведения — внести в спеку)
- [x] Infrastructure / Swagger / Vue / Monitoring / rollout / runbook / perf budget — N/A

## Соответствие спеке (Sec 17 по DS/TS)
- [x] Каждый TS-NNN части имеет тест — ДА
- [x] §Impact «Вне каталога фичи» — все позиции имеют правку в diff — ДА
- [x] Scope creep нет — ДА (отказ отмены проведения — решение оркестратора, DS-045)
- [x] DS-инварианты кодом не нарушены — ДА

## Статус фаз
- [x] `0-setup/light-progress.md` — все пункты `[x]` — ДА

## Итог
APPROVED. До закрытия части — синхронизировать спеку (DS-045 на отмену проведения, строка `POST /subtasks`). Гонку перерисовки документа заказа (TR-021) вынести отдельной задачей.
