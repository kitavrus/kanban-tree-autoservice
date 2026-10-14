---
stage: 8-code-review
feature: transfers-core
tier: M
verdict: APPROVED
high: 0
medium: 2
low: 9
---
# Code Review: transfers-core — APPROVED
**Review level:** Standard (tier: M) · Режим: spec-track merged (Sec 17 по DS/TS, Sec 18 API-линза) · совместимость MVP (DS-027)

Вход: `input-bundle.md`, `diff-{backend,migrations,other}.patch`, точечно `draft-spec.md` §4.2–4.4, §6.2, §6.8–6.10, §8.2. Гейты (build/test/lint/integration/E2E 189 passed) — по прогонам оркестратора, не перегонялись.

## Блокеры (обязательно исправить до мержа)
Нет.

## Серьёзные замечания (исправить в этом PR)
1. **[internal/features/board/tasks_service.go:207-217 → task_equipment_repo.go:162 `DeleteForeign`]** Обход DS-026. Смена клиента заказа молча удаляет из заказа единицы прежнего клиента, включая стоящие в незавершённом документе (`active_transfer_id IS NOT NULL`). Проверяется только `ForeignInUse` (услуги). `RemoveEquipment` тот же случай отбивает 409 `EQUIPMENT_IN_TRANSFER`.
   Исправление: до `DeleteForeign` — выборка чужих единиц заказа с `e.active_transfer_id IS NOT NULL` под `FOR NO KEY UPDATE OF e` → `equipmentInTransfer(id)`; live-тест рядом с `TestTS018_EquipmentInOpenTransferGuards`. Альтернатива — явный DS «смена клиента не входит в DS-026» (решение пользователя).
2. **[docs/features/transfers-core/draft-spec.md]** Спека расходится с принятой реализацией — обновить owner:
   - `:35, :80, :208, :255` — `FOR UPDATE` строки `equipment` → фактически `FOR NO KEY UPDATE` (`transfers/repo.go:482-493`, обоснованно, см. оценку 2);
   - `:485, :605, :612` — пример `"qty_sent": "5"` (строка) даёт 400: `numeric.Amount` принимает только JSON-число (as-is проекта);
   - §6.2/§6.3 — нет поля `items[].equipment_client_name` (`transfers/dto.go:112`);
   - `:904` — не описана форма `location` / `transfers` в `GET /equipment/:id/history` (`clients/history.go:454-461`);
   - §6.10 — нет строки `DELETE /api/v1/clients/:id` → 409 `EQUIPMENT_HAS_TRANSFERS` (`clients/repo.go:356-362`);
   - §8.2 `:1063-1064` — «тост», фактически `KT.uiAlert` (`task-card.js:622`).

## Незначительные замечания (можно в следующем PR)
1. **[internal/features/transfers/repo.go:751 `checkRefs`/`activeFlags`; board/task_equipment_service.go:55]** TOCTOU с архивированием склада: `is_active` читается без блокировки, архив (`warehouses/service.go` — `FOR UPDATE` активных + `InUse`) может пройти между проверкой и вставкой → заявка/единица на архивном складе. Окно узкое, только admin. Исправление: читать склады `... FOR KEY SHARE` (конфликтует с `FOR UPDATE` архива, после ожидания перечитывает `is_active`).
2. **[web/ssr/_shared/js/task-card.js:622]** После 409 `EQUIPMENT_IN_TRANSFER` карточка не перечитывается — «Убрать» остаётся видимой до следующего действия. Добавить `loadCard(taskId)` в `.then` диалога (как `:148`, `:166`).
3. **[internal/features/transfers/repo.go:533-536]** Страховочная ветка `claimEquipment` отдаёт `equipmentBusyError` без `transferID` → текст «…в перемещении #0», `details.transfer_id: 0`. Вернуть внутреннюю ошибку (500) либо перечитать фактический документ.
4. **[internal/features/transfers/repo.go:550-553 `settleEquipment`]** Нет сверки `RowsAffected == len(ids)`: рассогласованная единица документа молча пропускается при receive/cancel. Добавить проверку → ошибка транзакции.
5. **[e2e/fixtures/contrast.ts:238]** Комментарий «см. проверку ниже» — проверки нет: неподдерживаемая строка цвета даёт `#000` и ложный контраст. Добавить сверку (второй проход с другим сентинелом) или убрать ссылку.
6. **[e2e/tests/transfers.spec.ts:555-576, equipment-warehouses.spec.ts:331-344]** Порог контраста бейджей 3:1 — норма WCAG для крупного текста; текст бейджа мелкий (AA = 4.5). Поднять порог либо зафиксировать 3:1 решением.
7. **[internal/features/transfers/migration_integration_test.go:21-45]** Устаревшие имена `StockTransfersMigrationSuite` / `TestStockTransfersMigrationSuite` в пакете `transfers`.
8. **[internal/features/transfers/ssr.go:42-58]** SQL в `Service.Warehouses` через `s.repo.pool` — перенести в `Repo`.
9. **[board/enums.go:240-248, clients/equipment_search.go:2078-2085, transfers/dto.go:29-32]** Значения `location_status` заведены трижды; с CHECK сверяется только копия board (`TestEquipmentLocation_EnumMatchesDBCheck`). Добавить сверку для clients либо общий источник.

## Оценка по вопросам оркестратора
1. **Миграция 0014 — допустимо.** Перенос DS-054 реализован буквально (`0014…up.sql:157-169`): последняя колонка `position DESC, id DESC`; `DISTINCT ON … ORDER BY t.id DESC` — самый свежий незавершённый; «в пути» → `transit_from_warehouse_id`; одна колонка/нет колонок → все `at_client`; `columns` глобальны (доска одна). Перенос ДО `DROP`. CHECK `equipment_location_check` = спека §4.3. Транзакционность: golang-migrate шлёт файл одним Exec — неявная транзакция. Переименование ограничений по каталогу покрывает и NOT NULL-констрейнты PG18; `stock_movements_transfer_id_fkey` живёт на `stock_movements`, имя нейтральное, ссылка следует за rename. Down теряет: «в пути», занятие документом, строки-единицы, `at_client` (→ склад по умолчанию), журнал пуст — описано в шапке, MVP. Тест `TestTS026_MigrationLocations` проверяет все случаи (а)–(е), негативы, CHECK'и, down.
2. **Конкурентность — допустимо.** Гарантия DS-019 на уровне БД: одна колонка `equipment.active_transfer_id` + guard-UPDATE `IS NULL` под блокировкой строки (`repo.go:512-538`); частичный уникальный индекс невозможен (статус в другой таблице). Порядок transfers → product_stock → equipment соблюдён во всех ветках; board идёт task → product_stock либо task → equipment — цикла нет. `FOR NO KEY UPDATE` корректен: `FOR UPDATE` при параллельных созданиях дал бы 40P01 (обе tx держат `FOR KEY SHARE` от FK `transfer_items.equipment_id`); все писатели местоположения берут NO KEY UPDATE, удаление единицы отсекает FK RESTRICT. Подтверждено `TestTS005_ParallelCreateSameEquipment`, `TestTS009_ParallelReceiveOnce`.
3. **Переходы статусов — допустимо.** send: `location=at_warehouse ∧ warehouse=from ∧ active=id`, иначе 409 (`repo.go:590-596`); receive → склад назначения, cancel in_transit → склад отправления, cancel requested → освобождение. Любая ошибка — откат всей tx (`inTx`). См. LOW 3, 4.
4. **Мёртвый код D57 — чисто.** fff по `TASK_EQUIPMENT_IN_TRANSIT`, `transit_*`, `task_equipment_movements`, `stock-transfers`, `stocktransfers`, `equipment_status`, `movementText`: в prod-коде 0; остатки — только тесты «удалено → 404/нет ключа» и LOW 7.
5. **Запреты DS-026 — допустимо, кроме MEDIUM 1.** `RemoveEquipment`, `DeleteEquipment`, `WAREHOUSE_IN_USE` реализованы и покрыты (TS-018, TS-019). Удаление клиента → 409 `EQUIPMENT_HAS_TRANSFERS` — разумное следствие FK RESTRICT; внести в спеку (MEDIUM 2).
6. **API (Sec 18).** `GET /api/v1/equipment` — допустимо: исполнитель и так читает `GET /clients`, `/clients/:id/equipment`, `/equipment/:id`; конверт, фильтры, лимиты, сортировка = §6.8; N+1 нет. История: ключи `location`/`transfers` рядом с конвертом — допустимо (§API-1 запрещает массив/`page`/`per_page`/`has_more`, конверт цел); чище — местоположение в `GET /equipment/:id`. `equipment_client_name` — допустимо (аддитивно). Количество JSON-числом — as-is проекта, неверен пример спеки. Строгий разбор (`server/router.go:47 strictJSON`) — as-is, покрыт TS-028.
7. **`KT.uiAlert` вместо тоста — допустимо:** вся карточка заказа as-is на диалогах (`task-card.js:148-1364`); см. LOW 2.
8. **CSS — допустимо, улучшение.** В тёмной теме `--bs-primary #60A5FA`, `--bs-success #34D399`: белый текст ~2.5:1, тёмный `--kt-on-accent` — норма; согласовано с комментарием `app.css:47-49`. `contrast.ts`: альфа-наложение слоёв от корня и приведение `color()` через canvas корректны; ограничения — `opacity` не учитывается, LOW 5.
9. **seed-demo — допустимо.** Правило DS-054 через `lastColumnID`, guard-UPDATE'ы как в сервисе; `assertWarehouses` проверяет инварианты (ровно 1 строка-единица «в пути», `active_transfer_id` только на незавершённый документ, нет рассогласования по колонкам).
10. **Тесты — по существу.** Проверяют состояние БД, остатки, движения, версии, details ошибок. Удалённые `equipment_warehouses_http_live_test.go` / `stock-transfers.spec.ts`: сохранившееся поведение перенесено (`equipment_location_http_live_test.go`, `transfers_http_live_test.go`, `transfers.spec.ts`, `equipment-warehouses.spec.ts`); поведение send/receive/cancel-transit удалено намеренно. `waitForTimeout`/`fixme`/`t.Skip` нет.

## Вынести пользователю (не дефект)
- До части 4 единица, принятая на склад через заказ, не возвращается в `at_client` (нет «склад → клиент»): завершение заказа местоположение не меняет, `WAREHOUSE_IN_USE` блокирует архив такого склада. Соответствует §4.3, но расходится с правилом миграции DS-054 для новых данных.
- MEDIUM 1: чинить сейчас или оформить DS.

## Соответствие кода спеке (spec-track: draft-spec вместо design-*)
- [x] Данные §4.1–§4.4 (DDL, индексы, CHECK, FK) — ДА
- [x] API §6.1–§6.10 (коды, сообщения, конверты) — ДА (расхождения — MEDIUM 2)
- [x] Статусная модель §5, сообщения переходов — ДА
- [x] Ошибки через `errorspkg`, 400 → `details.fields[]` — ДА
- [x] §Impact «Вне каталога фичи» — все позиции в diff — ДА (`equipment-history.js`, `board.js`: ключи ответа прежние, правка не потребовалась)
- [x] as-is DS-инварианты (DS-048, DS-049) не нарушены — ДА
- Swagger — n/a (`swagger:no`); Monitoring/Runbook/Perf/Rollout — n/a (в спеке нет); PRD — n/a (spec-track)

## Чекеры (из bundle)
- 9/10 зелёные. `e2e_drift` FAIL — ложное: ищет `test('TS-NNN: …')`, в проекте `TR-NNN (TS-NNN)` / `EW-NNN (TS-NNN)`; сверено вручную: TS-031 → TR-001, TS-036 → EW-001/002, TS-043 → TR-004, TS-044 → EW-004, TS-045 → TR-010/EW-007, TS-046 → TR-011/EW-007.
- `live_suite` PASS с пометкой «HTTP-слоя нет» — слепота чекера на плоской раскладке; фактически `*_http_live_test.go` на LiveSuite есть для всех новых endpoint'ов.
- Улика E2E: `9-validation/evidence/last-run.json` — `status: passed`, `failedTests: []`.

## Статус фаз
- Носителя статусов плана в `docs/features/transfers-core/` нет (`0-setup/light-progress.md` — прогресс light-трека) — n/a.

## Итог
APPROVED: блокеров 0. MEDIUM 1 — исправить в этом PR либо закрыть решением пользователя; MEDIUM 2 — синхронизация спеки.
