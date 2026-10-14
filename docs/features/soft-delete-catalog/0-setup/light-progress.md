# light-progress: soft-delete-catalog (часть 5)

- [x] 1. Миграция 0022 (4 таблицы, частичные индексы) + down
- [x] 2. catalog: products (delete/restore/purge, deleted_at IS NULL, category_is_deleted, HAS_STOCK)
- [x] 3. catalog: services (delete/restore/purge, product_is_deleted, category_is_deleted)
- [x] 4. catalog: categories (delete/restore/purge, item_count, NAME_TAKEN)
- [x] 5. equipmenttypes (delete/restore/purge, equipment_count TS-061, NAME_TAKEN)
- [x] 6. clients: type_is_deleted, type_id not_found
- [x] 7. board: service_is_deleted/product_is_deleted, выбор без удалённых, stock is_deleted/summary/adjust
- [x] 8. transfers: product_is_deleted, product not_found в новых строках, unreceive/restore TRANSFER_HAS_DELETED_REFS(product)
- [x] 9. softdelete: 4 Source + регистрация + purge в router.go
- [x] 10. SSR: формы удаления каталога/типов (data-delete-dialog), бейджи, deleted.js вкладки
- [x] 11. seeddemo + тест
- [x] 12. Go-тесты: unit, migration integration, live (TS-012,026..030,033,054,059,060,061) + гонки
- [x] 13. Правка существующих тестов (204→200, *_IN_USE)
- [x] 14. E2E: spec TS-044, pages, fixtures, старые спеки (TS-F4/F10)
- [x] 15. Self-checks: go build, go vet, go test ./..., integration по пакетам, tsc, playwright --list

## Заметки
- catalog: soft_delete.go (repo/service/handlers), ssr_options.go (опции с удалёнными), шаблоны форм → data-delete-dialog; SSR каталога+типов сделан (п.10 частично: остались deleted.js, бейджи board/clients/transfers/stock).
- Порядок блокировок товара: товар (NO KEY UPDATE) → категория (SHARE) → product_stock (UPDATE). Новые ссылки на товар — товар FOR SHARE c deleted_at IS NULL.
- softdelete: ErrNameTaken, ParseDeleteRequest, ReasonError, ToastDeleted добавлены в reason.go.
- category_id/product_id удалённой записи → not_found; несуществующей → invalid (as-is).
- [x] 6,7,8,9 сделаны (clients type_is_deleted+lockLiveType; board ServiceState/ProductState FOR SHARE, CopyKit, card flags, stock CTE is_deleted/Adjust; transfers checkLiveProducts/productFlags/product_is_deleted; softdelete/catalog_source.go + router.go). go build/vet зелёные.
- Осталось: JS (deleted.js вкладки, task-card.js/transfers.js/stock.js бейджи), seeddemo, тесты Go, правка старых тестов, E2E.
- Решения к отчёту: Send/Cancel документа с удалённым товаром — as-is (не запрещены спекой); PUT subtask-products не меняет product_id (N/A); CopyKit as-is копирует архивные (спека утверждает обратное) — добавлен только фильтр удалённых; stock list: удалённый с остатком виден при любом scope.
- [x] 10 (JS: deleted.js 4 вкладки+бейдж «в архиве», task-card.js/transfers.js/stock.js бейджи), 11 (seeddemo.seedDeletedCatalog + assertDeletedCatalog) сделаны.
- [x] 13 частично: поправлены catalog_http_live_test.go, services_http_live_test.go, equipmenttypes_http_live_test.go, clients_http_live_test.go (TypeInUse). softdelete_http_live_test.go: добавлен TestTS009_CatalogKinds (чистит фикстуры SQL).
- SSR категорий: rename/archive/restore удалённой → страница 404 (ssrCategories.status).
- TODO тесты: catalog/catalog_soft_delete_http_live_test.go (TS-027/028/029/033, флаги, SSR, purge, гонки), catalog/migration_soft_delete_integration_test.go (TS-012), equipmenttypes soft delete test (TS-028/030/033/061), clients type test (type_id not_found, type_is_deleted), board (TS-026/030/054 + гонки), transfers (TS-059/060 + гонки), затем E2E.
- [x] catalog тесты ЗЕЛЁНЫЕ (go test ./internal/features/catalog/... -p 1 -tags=integration: ok 22s): migration_soft_delete_integration_test.go (TS-012), catalog_soft_delete_http_live_test.go (TS-027/028/029/033, причина, флаги, SSR, purge, 6 гонок). CatalogLiveSuite теперь с TEST_PURGE_ENABLED=true.
- TODO: equipmenttypes soft delete test (TS-028/029/030/033/061+гонки), board (TS-026/030/054), transfers (TS-059/060), прогон softdelete/clients/seeddemo/board/transfers пакетов, E2E.
- [x] equipmenttypes тесты ЗЕЛЁНЫЕ (ok 27s): equipmenttypes_soft_delete_http_live_test.go (TS-028/029/030/033/061, причина, purge, 4 гонки). TypesLiveSuite с TEST_PURGE_ENABLED=true.
- TODO: board (TS-026/030/054 + гонки) — новый файл на OrderCardLiveSuite/StockLiveSuite; transfers (TS-059/060); прогон softdelete/clients/seeddemo/board/transfers; E2E.
- [x] board тесты ЗЕЛЁНЫЕ (весь пакет ok 64s): board/soft_delete_catalog_http_live_test.go (TS-026, TS-030 услуга, TS-027 негатив, TS-054, архивный удалённый на 2-м складе, 4 гонки).
- TODO: transfers (TS-059/060 товар + гонки), прогон softdelete/clients/seeddemo/transfers, go test ./..., E2E (spec TS-044, pages, fixtures, старые спеки).
- [x] transfers тесты ЗЕЛЁНЫЕ (ok 25s): transfers_soft_delete_catalog_http_live_test.go (TS-059, TS-060, TS-027 негатив, 4 гонки). TODO: прогон softdelete/clients/seeddemo, go test ./..., E2E.
- [x] unit: softdelete/delete_request_test.go, softdelete/catalog_source_test.go, board/stock_sql_test.go. Все integration по тронутым пакетам зелёные (catalog, equipmenttypes, board, transfers, softdelete, clients, seeddemo). TODO: internal/server integration, go test ./..., E2E.
- [x] E2E: tests/soft-delete-catalog.spec.ts (SDCT-001..003), правки catalog.spec/equipment-types.spec/board-guards.spec, pages, fixtures (catalogCleanup autouse). tsc чистый, --list 21 тест. Осталось: финальный integration-прогон тронутых пакетов + отчёт.
- [x] 15. Финальные self-check'и зелёные: go build, go vet (+integration), go test ./... , integration 8 тронутых пакетов -p 1, tsc, playwright --list. ГОТОВО; E2E не прогонялись (по условию).

## Fix-loop 1 (по ревью, поверх WIP ca6251e)
- [x] transfers.Send: checkLiveProducts после lockTransfer → 409 TRANSFER_HAS_DELETED_REFS (product); отмена «В пути» без запрета. Тесты: TestSendRefusedForDeletedProduct, TestRace_SendWaitsForProductDelete.
- [x] board StockRepo.Adjust: удалённый товар — только списание по паре с ненулевым остатком (под FOR UPDATE строки), в сторону нуля; иначе 404 / 409 PRODUCT_DELETED_WRITE_OFF_ONLY. Тесты: TestStock_DeletedProductAdjustOnlyWriteOff, TestRace_DeletedProductAdjustWaitsForWriteOff.
- [x] e2e catalogCleanup: статус purge читается, 409 — одним итоговым предупреждением.
