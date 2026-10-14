# light-progress — pagination (чеклист исполнителя)

- [x] DS-001 доска: ленивая догрузка по колонке (board.js)
- [x] DS-002 API tasks: column_id, per_column, column_total
- [x] DS-003 колонки/исполнители/категории — все страницы
- [x] DS-004 счётчик board-total
- [x] DS-005 каталог SSR: пейджер товаров/услуг
- [x] DS-006 пометка categories-truncated
- [x] DS-007 комбобокс товара в форме услуги — серверный поиск
- [x] DS-008 лимиты не меняются (проверка) — константы tasks 500/1000, catalog 2000/2000, products/services 50 не тронуты
- [x] TS-001 unit ParseTaskFilter
- [x] TS-002 integration per_column
- [x] TS-003 integration column_id+limit/offset
- [x] TS-004 integration фильтр+per_column
- [x] TS-005 integration SSR products offset
- [x] TS-006 integration SSR services offset
- [x] TS-007 E2E доска «Показать ещё»
- [x] TS-008 E2E board-total
- [x] TS-009 E2E пейджер товаров
- [x] TS-010 E2E комбобокс товара
- [x] api-delta.md
- [x] self-checks (go test, integration, vet, gofmt, lint, playwright --list, tsc) — unit ok, integration board+catalog 338 PASS/0 FAIL, vet ok, gofmt ok, golangci-lint 0 issues, playwright --list 4 теста, tsc ok
- [x] ИСПОЛНИТЕЛЬ ЗАВЕРШИЛ

## Fix-loop по ревью (8-code-review/output.md)
- [x] MEDIUM-1 поколение загрузки state.loadGen: устаревший ответ «Показать ещё» игнорируется, moreInFlight сбрасывается в loadTasks
- [x] MEDIUM-2 обрезанная выдача: колонки без column_total → «Показать заказы» (column-more-<id>, offset 0); api-delta.md обновлён
- [x] LOW-1 кнопка снова активна после ошибки; повторный клик при moreInFlight не гасит её
- [x] LOW-2 applyTaskPatch не затирает column_total нулём из ответа мутации
- [x] FIX-LOOP ЗАВЕРШЁН

## Fix-loop 2 (E2E-регресс: постраничный каталог)
- [x] CatalogPage.gotoProductsByName/gotoServicesByName (?q=); board-guards BD-019a и catalog CT-003/CT-005 ищут свою запись через ?q=
- [x] Остальные вызовы без фильтра проверены: CT-002 (панель категорий, не пейджится), CT-006 (поиск через форму), CT-008 (overflow, без поиска строк) — оставлены
- [x] TS-009: 102 товара в своей категории, пейджер по ?category=
- [x] FIX-LOOP 2 ЗАВЕРШЁН
