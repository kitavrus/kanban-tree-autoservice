# light-progress: purchase-orders

## Работы
- [x] Обзор каркаса и соседнего кода (auth middleware, shell, users, softdelete, suppliers, board stock/pdf)
- [x] Миграция 0019 (таблицы, sequence, users.role/supplier_id, CHECK)
- [x] errorspkg: ошибки фичи (из сниппета, без дублей) — pkg/errorspkg/purchase_orders.go
- [x] Backend: repository/service/handler закупок (§6.1–§6.10)
- [x] Backend: кабинет поставщика API (§6.12–§6.15)
- [x] Backend: PDF (§6.11, §6.16) — board.NewPDFDoc/PDFTableRow (pdf_kit.go)
- [x] Middleware: RoleSupplier, запрет supplier в RequireAPI/RequireSSR, RequireSupplier, RequireStaff
- [x] auth: supplier_id в Principal, редирект supplier после входа
- [x] users: роль supplier + supplier_id (validate/service/repo/ssr/шаблон)
- [x] suppliers: SUPPLIER_HAS_OPEN_PURCHASES (архив + soft-delete)
- [x] softdelete: вид purchase-orders (источник + вкладка deleted.js + restore/purge)
- [x] board stock_repo: экспорт PendingCTE/PendingTasksSQL
- [x] shell: пункт «Закупки», сайдбар поставщика
- [x] SSR: /purchases, /purchases/new, /purchases/:id (+purchases.js)
- [x] SSR: /supplier/orders, /supplier/orders/:id (+supplier-orders.js, адаптив)
- [x] Роутинг/подключение в server
- [x] e2e: спека в e2e/tests/purchase-orders.spec.ts, pages/PurchaseOrdersPage.ts, fixtures/purchase-orders.ts

## Тесты (§11)
- [x] TS-001 unit validateCreate/validateUpdate (validators_test.go)
- [x] TS-002 unit validateAnswer (+409 ready в live TS-006)
- [x] TS-003 матрица переходов (unit checks_test + live)
- [x] TS-004 конкурентность (live)
- [x] TS-005 сброс ответа (live)
- [x] TS-006 «Собрано» incomplete (unit + live)
- [x] TS-007 подсказка дефицита (live)
- [x] TS-008 миграция CHECK/sequence/up-down (repository/migration_integration_test.go + live)
- [x] TS-009 live supplier заперт
- [x] TS-010 live executor только свои
- [x] TS-011 live архив/удаление поставщика
- [x] TS-012 live users supplier
- [x] TS-013 «Забрали» не трогает остатки
- [x] TS-014..TS-026 e2e специи (только --list)
- [x] TS-F1..F4 фикстуры/сиды (testutil.SeedSupplierUser, e2e po-фикстура, users validate_test, softdelete тесты 9 видов)

## Self-check
- [x] go build ./...
- [x] go vet пакетов (+ -tags=integration)
- [x] go test ./... unit — 21 ok, 0 fail
- [x] integration: purchase-orders/..., users, auth, suppliers, softdelete, server, board — все ok
- [x] playwright --list — 18 тестов; tsc --noEmit чисто
- [x] ИСПОЛНИТЕЛЬ ЗАВЕРШИЛ
