# light-progress: suppliers

- [x] Чтение эталона warehouses (backend, SSR, e2e) + stocktransfers (JS-экран поверх API)
- [x] Миграция 0012_suppliers (up/down)
- [x] internal/features/suppliers: repo.go, dto.go, validate.go, service.go, handler.go, routes.go, ssr.go
- [x] Регистрация в internal/server/router.go
- [x] Пункт меню internal/ssr/menu.go
- [x] SSR-экран web/ssr/suppliers/templates/pages/list.html + web/ssr/_shared/js/suppliers.js
- [x] Unit: validate_test.go (TS-020)
- [x] Integration LiveSuite: suppliers_http_live_test.go (TS-020)
- [x] Миграционный тест migration_integration_test.go (TS-020, индекс уникальности)
- [x] E2E: e2e/pages/SuppliersPage.ts + e2e/tests/suppliers.spec.ts (TS-035: SUP-001…SUP-006, overflow 375x812) + wire-типы/хелперы в e2e/fixtures/auth.ts
- [x] Self-checks: go build OK, go vet OK, go test ./... OK, go test -tags=integration suppliers OK, playwright --list OK (6 тестов), tsc --noEmit OK
- [x] Фактический прогон E2E — оркестратор: 28 passed (улика `9-validation/evidence/last-run.json`)
- [x] Fix-loop 1: SUP-005 — allowHttpErrors на ожидаемый POST 400
- [x] Fix-loop 2: постраничный поиск ?id= (findById), guard busy в openForm, сужен предикат SUP-002
