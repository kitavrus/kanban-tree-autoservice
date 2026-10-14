# transfers-core — light progress (анти-компакт чеклист)

> После сжатия истории: перечитать §11 среза (`draft-spec.slice-light.md:924-970`) и этот файл; продолжить с первого `- [ ]`.

## Шаги работ
- [x] 1. Миграция `migrations/0014_transfers_core.{up,down}.sql` (написана, НЕ прогонялась)
- [x] 1t. Миграционный тест TS-026 — `TransfersCoreMigrationSuite` (`TestTS026_MigrationLocations`, `TestTS026_SingleColumnAllAtClient`) в `internal/features/transfers/migration_integration_test.go`; ЗЕЛЁНЫЙ (`-run Migration`), старый тест 0009 тоже зелёный
- [x] 5. seeddemo.go + seeddemo_integration_test.go переписаны (TS-F1), integration ещё НЕ прогонялся
- [x] 2. Пакет `internal/features/transfers` (dto/validate/repo/service/handler/routes/ssr переписаны; `go build` ок). Тесты пакета ещё СТАРЫЕ (validate_test.go, transfers_http_live_test.go — не компилируются/устарели)
- [x] 2e. `GET /api/v1/equipment` — `internal/features/clients/equipment_search.go` + route
- [x] 3. board: card_repo/card_dto (location_status, active_transfer_id, transit_to_name), tasks_repo (фильтр+has_in_transit), task_equipment_* (ReceiveAtWarehouse, ActiveTransfer, снятие send/receive/cancel-transit), subtask_products_repo D58, stock_repo CTE, errors/enums/routes/handler
- [x] 4a. clients: history.go (location + transfers в ответе и SSR-шаблоне), `EQUIPMENT_HAS_TRANSFERS`
- [x] 4b. warehouses: `WAREHOUSE_IN_USE` (service.withLastActiveGuard + repo.InUse + ssr error=in_use)
- [x] 4c. stock CTE «в пути» → transfers, kind=product
- [ ] 5. seeddemo.go + seeddemo_integration_test.go (TS-F1)
- [x] 6a. Экран /transfers: `web/ssr/_shared/js/transfers.js` (KT.transfers, переписан под §8.1, node --check ок), `web/ssr/transfers/templates/pages/list.html`, `transfers/ssr.go` (PageVM.Warehouses, NewSSR(svc,…)), router
- [x] 6b. task-card.js (бейдж местоположения `card-equipment-location[data-location-status]`, `card-equipment-transfer` ссылка/текст, кнопки и журнал убраны, подпись `card-equipment-warehouse-hint`, «Убрать» скрыта при active_transfer_id); board.js менять не пришлось (has_in_transit — то же поле API)
- [x] 7a-board. `go test ./internal/features/board/... -tags=integration` ЗЕЛЁНЫЙ (55s): удалён equipment_warehouses_http_live_test.go; новый equipment_location_http_live_test.go (TestTS018_EquipmentInOpenTransferGuards, TestTS024_BoardWarehouseFilterAndTransitBadge, TestTS024_EquipmentSearchEndpoint, TestTS027_RemovedEndpoints404, TestTS027_AddDoesNotMoveEquipmentAlreadyAtWarehouse, TestEquipmentLocation_*); equipment_warehouse_validate_test.go (TestTS018_EquipmentInTransferError), board_fuzz_test.go, order_card_http_live_test.go (DTO)
- [x] 7a-rest. ЗЕЛЁНЫЕ integration: clients (TestEquipmentHistory_LocationAndTransfers, TestTS018_DeleteEquipmentWithTransfers, TestAPI_EquipmentHistory поправлен), warehouses (warehouses_in_use_http_live_test.go: TestTS019_ArchiveBlockedWhileInUse, TestTS019_LastActiveStillWins), seeddemo; `go build ./...` и `go test ./... -count=1` (unit) зелёные
- [x] 7b. E2E написаны (НЕ запускались — зона оркестратора): `e2e/pages/TransfersPage.ts`, `e2e/tests/transfers.spec.ts` (TR-001…TR-011), `e2e/tests/equipment-warehouses.spec.ts` (EW-001…EW-007, переписана), fixtures auth.ts/board-types.ts, OrderCardPage, order-card.spec (текст бейджа), user-timezone.spec; удалены StockTransfersPage.ts, stock-transfers.spec.ts
- [x] 8. Self-checks зелёные: go build, go vet -tags=integration (5 пакетов), go test ./... (19 ok), integration transfers/board/clients/warehouses/seeddemo, node --check (transfers.js, task-card.js), `npx tsc --noEmit`, `playwright --list` (63 теста в 5 файлах)
- ВСЁ СДЕЛАНО — остался только итоговый отчёт (SubagentHandback).

## TS → тест (итог)
- TS-005: transfers `TestTS005_EquipmentClaimedByOneOpenDocument`, `TestTS005_ParallelCreateSameEquipment`; unit `TestMapErr_EquipmentConflicts`; E2E TR-008
- TS-006: `TestTS006_SendEquipmentNotAtSource`; E2E TR-009
- TS-009: `TestTS009_VersionBeforeStatusAndTransitionMessages`, `TestTS009_ParallelReceiveOnce`, unit `TestTS009_StatusConflictMessages`; E2E TR-003, TR-005
- TS-018: board `TestTS018_EquipmentInOpenTransferGuards`, unit `TestTS018_EquipmentInTransferError`; clients `TestTS018_DeleteEquipmentWithTransfers`; E2E EW-002
- TS-019: warehouses `TestTS019_ArchiveBlockedWhileInUse`, `TestTS019_LastActiveStillWins`
- TS-024: board `TestTS024_BoardWarehouseFilterAndTransitBadge`, `TestTS024_EquipmentSearchEndpoint`, unit `TestParseTaskFilter_WarehouseID`
- TS-026: transfers `TestTS026_MigrationLocations`, `TestTS026_SingleColumnAllAtClient`
- TS-027: board `TestTS027_RemovedEndpoints404`, `TestTS027_AddDoesNotMoveEquipmentAlreadyAtWarehouse`; E2E EW-005, TR-007
- TS-028: transfers `FuzzTS028_CreateBody` (unit), `TestTS028_CreateArbitraryJSON_No5xx` (live)
- TS-031: E2E TR-001; TS-036: EW-001, EW-002; TS-043: TR-004; TS-044: EW-004; TS-045: TR-010, EW-007; TS-046: TR-011, EW-007
- TS-F1: seeddemo `TestSeedDemo_Run_EmptyDB_CreatesFullDemoSet` (assertWarehouses); TS-F2/F3/F4 — см. 7a/7b
- [x] 7a-transfers. `go test ./internal/features/transfers/... -tags=integration` ЗЕЛЁНЫЙ: validate_test.go (unit, FuzzTS028_CreateBody, TestTS009_StatusConflictMessages), transfers_http_live_test.go (TransfersLiveSuite, as-is товары), transfers_equipment_http_live_test.go (TestTS005_*, TestTS006_*, TestTS009_*, TestTS028_*, TestMixedDocument_*)
- [ ] 7a. Go-тесты: transfers validate_test + live (TS-005/006/009/028), board (TS-018/024/027; удалить equipment_warehouses_http_live_test, equipment_warehouse_validate_test; поправить order_card_http_live_test, warehouses_card_http_live_test, board_fuzz_test fakeCardService), warehouses (TS-019), clients live (history, search, delete)
- [ ] 7b. E2E: TransfersPage, transfers.spec.ts (TS-031/043/045/046), equipment-warehouses.spec.ts (TS-036/044), fixtures auth.ts/board-types.ts (TS-F2), OrderCardPage, user-timezone.spec.ts (ссылки на stock-transfers), ExecutorPayoutsPage
- [ ] 8. Self-checks: go build, go vet integration, unit, integration по пакетам, node --check, tsc, playwright --list

## TS → тест
(заполняется по мере написания)

## Решения
- Коды полей по спеке: not_found/archived (склад, товар), not_positive (qty_sent ≤ 0), negative (факт < 0), not_allowed (чужое поле типа строки; факт по строке-единице); bad uuid — `invalid`.
- uuid принимается только в канонической 36-символьной форме (иначе ::uuid → 500 на urn:/{}).
- Блокировка equipment — `FOR NO KEY UPDATE` (FK transfer_items держит KEY SHARE → FOR UPDATE дал бы 40P01 при параллельных POST).
- Create+send: lockPairs до claimEquipment (Lock Order документ → остатки → единицы).
- `TransferItemResponse` + `equipment_client_name` (нужно UI §8.1 п.5; в спеке поля нет).
- История единицы: конверт заказов + доп. ключи `location`, `transfers` в ответе `GET /equipment/:id/history`; per-order `equipment_status`/`location` убраны.
- DELETE /equipment/:id: проверка документов раньше проверки заказов (EQUIPMENT_HAS_TRANSFERS приоритетнее EQUIPMENT_USED_IN_TASKS).
- WAREHOUSE_IN_USE проверяется после WAREHOUSE_LAST_ACTIVE и только при переходе активный → архив.
- AddEquipment: склад назначается единице только при фактической вставке строки заказа (повторный POST — без изменений).
- Карточка: wire-поле `status` единицы заменено на `location_status`; убраны transit_from_*/transit_to_warehouse_*; `transit_to_name`.
- Старые миграционные тесты 0009/0010 оставлены (проверяют исторические миграции и заодно down 0014).
