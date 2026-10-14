# Фаза 4 — Каталог: прогресс (подфаза A)

## Миграция
- [x] migrations/0004_catalog.up.sql — catalog_categories, products, services, service_products, stock_movements
- [x] migrations/0004_catalog.down.sql

## Backend Go (internal/features/catalog)
- [x] categories: dto/validate/repo/service/handler
- [x] products: dto/validate/repo/service/handler (+ stock_movements manual в транзакции Update)
- [x] routes.go — API + SSR wiring, /catalog → 303 /catalog/services
- [x] catalog.go — TabCounts (счётчики вкладок), Root
- [x] ssr.go/ssr_products.go/ssr_categories.go/ssr_services.go — категории (левая панель), товары список/форма, услуги-заглушка
- [x] обновить internal/features/equipmenttypes/ssr.go — добавить вкладки (catalog.TabsVM, без обратного импорта)
- [x] обновить internal/server/router.go — регистрация catalog.Register, убрать /catalog redirect из equipmenttypes

## Тесты backend
- [x] categories_validate_test.go — зелёный (go test ./internal/features/catalog/...)
- [x] products_validate_test.go — зелёный
- [x] catalog_http_live_test.go — зелёный (go test ./internal/features/catalog/... -tags=integration -count=1, dockertest PG18)
- [x] equipmenttypes integration — прогнан повторно после правки ssr.go, зелёный (не сломан)

## SSR templates
- [x] web/ssr/_layout/templates/partials/catalog-tabs.html
- [x] web/ssr/catalog/templates/pages/products-list.html (категории — левая панель, встроенные формы create/rename/archive/delete)
- [x] web/ssr/catalog/templates/pages/products-form.html
- [x] web/ssr/catalog/templates/pages/services-placeholder.html
- [x] web/ssr/equipmenttypes/templates/pages/list.html — добавлена полоса вкладок
- [x] app.css — .tabs/.tab-link/.two-pane/.cat-panel/.cat-item/.stock-low/.stock-out

## E2E
- [x] e2e/pages/CatalogPage.ts
- [x] e2e/tests/catalog.spec.ts (CT-001, CT-002, CT-003, CT-006 частично — товары, CT-007, CT-008 частично — товары/viewport/тема; CT-004/005/остаток CT-006/008 — подфаза B)
- [x] e2e/fixtures/auth.ts — AdminApi.createCategory/createProduct + типы Category/Product

## Self-check (все прогнаны локально, все зелёные)
- [x] go build ./...
- [x] go vet ./... (и -tags=integration)
- [x] go test ./... -count=1 — все пакеты ok
- [x] go test ./internal/features/catalog/... -tags=integration -count=1 — ok (dockertest PG18, реальный бинарь)
- [x] go test ./internal/features/equipmenttypes/... -tags=integration -count=1 — ok (повторно, после правки вкладок)
- [x] cd e2e && npx tsc --noEmit -p . — без ошибок
- [x] npx playwright test e2e/tests/catalog.spec.ts --list — 6/6 тестов найдены (CT-001,002,003,006,007,008)

## FIX-LOOP итерация 1 (headed E2E оркестратора: 2/6 passed)
- [x] CT-003: `product-category`/`product-unit` — combobox.js переносит data-testid на
  кнопку (D6); `.selectOption()` заменён на клик по кнопке + выбор `.cb-opt` в
  `combobox-dropdown` (эталон UsersPage.chooseRole). CatalogPage: `chooseCategory`/
  `chooseUnit`, `fill({categoryName,...})` вместо `categoryId`.
- [x] CT-006: GET-форма кодирует пробел как `+`, тест сравнивал с `%20` — заменено на
  `expect.poll(() => new URL(page.url()).searchParams.get('q')).toBe(name)`.
- [x] CT-007: error-context.md подтвердил единственную сетевую проблему —
  `HTTP 403 /catalog/products` (ожидаемый 403, ничего постороннего) — добавлен
  `allowHttpErrors((url,status)=>url.endsWith('/catalog/products')&&status===403)`.
- [x] CT-008: реальный дефект вёрстки — рендер (test-failed-1.png, 360px) показал
  обрезанную иконку ✎ у поля переименования категории: `<input class="form-control">`
  внутри `.d-flex`-строки не сжимается ниже своей content-ширины (браузерный дефолт
  `min-width:auto` для textbox), раздвигая `.cat-panel` шире вьюпорта. Исправлено в
  app.css: `.cat-panel input.form-control{min-width:0;flex:1 1 auto}`,
  `.cat-panel form.d-flex{min-width:0}`, `.cat-item{overflow:hidden}` (сброс
  автоматического min-content для длинных названий категорий), `.two-pane{min-width:0}`.
  `.table-responsive` у таблицы товаров уже был — не источник бага.
  Тест НЕ трогал — правка только продуктового CSS/шаблона.

Self-check после фикса: `go build`, `go vet` (+`-tags=integration`), `go test ./... -count=1`
(все ok), `go test ./internal/features/catalog/... -tags=integration -run TestCatalogLiveSuite`
(ok — шаблон products-list.html парсится/рендерится штатно после правки), `npx tsc --noEmit`
(без ошибок), `npx playwright test tests/catalog.spec.ts --list` (6/6 найдены). Реальный
E2E-прогон не запускал — перепрогон за оркестратором.

## Известные ограничения подфазы A (для передачи подфазе B / оркестратору)
- Услуги (/catalog/services) — заглушка-empty-state, без CRUD/API.
- CT-004, CT-005, остаток CT-006/CT-008 (услуги) — не реализованы, за скобками подфазы A.
- E2E-прогон (реальный playwright run) НЕ выполнялся этим агентом — только tsc + --list
  (владение прогонами — оркестратор, §EE).
- Категория при рассинхроне (архивная категория товара) — форма показывает её первой
  строкой опций с пометкой «(архив)», чтобы не терять выбор при редактировании.

## Подфаза B — услуги целиком

### Backend Go (internal/features/catalog)
- [x] services_dto.go — ServiceResponse/ServiceProductResponse (re-fetch-after-write),
  ServiceCreateRequest/ServiceUpdateRequest, Price *Amount (nil/null = «по факту»).
- [x] services_validate.go — ServiceInput.normalize() (trim + дедуп комплекта суммой qty
  через math/big.Rat — без потери точности), validate() (qty ≤0 → 400, product_id
  формат → 400, products[i].* индексация по нормализованному списку).
- [x] services_repo.go — CRUD в транзакции: категория kind=service, комплект — все товары
  активны (validActiveProductIDs одним ANY($1::uuid[]) запросом, errServiceProductInvalid{Index}
  указывает конкретную строку), комплект — DELETE+INSERT (проще диффа, RESTRICT product_id
  не мешает удалению join-строк).
- [x] services_service.go — List/Get/Create/Update/SetActive/Delete, mapServiceWriteErr
  (категория/строка комплекта → 400 по полю), toResponse дозагружает Products отдельным
  запросом (re-fetch-after-write).
- [x] services_handler.go — JSON API /api/v1/services (GET список+:id, POST, PUT, DELETE).
- [x] routes.go — /api/v1/services (RequireAPI+admin на мутациях), SSR /catalog/services
  (list/new/:id/archive/restore/delete) вместо заглушки.
- [x] ssr.go — SSR.svcSvc, NewSSR(+svcSvc), flashes service_created/saved/archived/restored/deleted.
- [x] ssr_services.go — список услуг (категории слева, поиск/архив в URL), форма
  создания/правки: комплект через row-list.js (PostArgs().PeekMulti, эталон clients
  phonesFrom/specsFrom), товар в строке — combobox с data-hint «артикул · цена/ед.»,
  serviceProductOptions/serviceCategoryOptions — активные + уже выбранные (даже архивные,
  чтобы форма не теряла отображение — эталон productCategoryOptions).
- [x] internal/server/router.go — catalog.NewServiceService(pool), catalog.NewServiceHandler,
  catalog.NewSSR(+svcSvc), catalog.Register(+svcH).

### Дизайн-решение (не в task.md буквально, задокументировано)
- «Архивный/несуществующий товар → 400» (task.md §API) энфорсится ОДИНАКОВО на create И
  update — даже если товар был активен на момент добавления в комплект, а потом
  архивирован: пересохранение услуги с этой строкой вернёт 400 по
  `products[i].product_id`, пока оператор не уберёт строку. SSR-форма при этом ПОКАЗЫВАЕТ
  архивный товар в комбобоксе существующей строки (не теряет отображение) — только не
  предлагает его для НОВОГО выбора. Это буквально соответствует task.md («архивные товары
  не предлагаются, но у существующей строки показываются» + «архивный → 400»), а не
  ослабление до поведения категорий (там is_active не проверяется вовсе — as-is, не
  трогалось).

### Тесты backend
- [x] services_validate_test.go — normalize (дедуп/сумма qty big.Rat, пустая строка
  отбрасывается), validate (границы имени, цена, qty≤0, мусор в product_id, индексация
  по нормализованному списку), sumAmount (точность 0.1+0.2).
- [x] services_http_live_test.go (ServicesLiveSuite, testutil.LiveSuite) — CRUD+комплект,
  «по факту» (null оба направления), дубли объединяются, qty≤0/архивный/несуществующий
  товар → 400, PRODUCT_IN_USE сквозным путём через API услуг + архивирование вместо
  удаления, невалидная категория → 400, is_active обязателен на PUT, IDOR/битый uuid →
  404, мусор → 400, D19 (исполнитель read-only активные / мутации 403 / аноним 401),
  CSRF 403, SSR список+форма с комплектом (product_id[]/qty[] через url.Values).
- [x] catalog_http_live_test.go — поправлена стухшая проверка `services-empty` в
  TestSSR_RootRedirect_ProductFormFlow (подфаза B заменила заглушку реальным списком;
  другой тест той же сьюты пишет строку в services напрямую в БД) — заменено на
  `services-filters` (присутствует в обоих состояниях).

### SSR templates
- [x] web/ssr/catalog/templates/pages/services-list.html — таблица (услуга, категория
  слева, цена работы/«по факту», товары в комплекте), поиск/архив в URL, категории.
- [x] web/ssr/catalog/templates/pages/services-form.html — комплект row-list.js + combobox
  на select товара (глобальный combobox.js уже подключён в base.html), архив/удаление
  data-confirm/data-once (KT.uiConfirm), PRG.
- [x] web/ssr/catalog/templates/pages/services-placeholder.html — удалён (заменён).

### E2E
- [x] e2e/fixtures/auth.ts — NewService/Service/ServiceProduct типы, AdminApi.createService.
- [x] e2e/pages/CatalogPage.ts — локаторы формы услуги, productRow(i)/chooseProductInRow/
  fillProductRowQty/removeProductRow (комплект), serviceRow/serviceRowToggle, confirmServiceDelete.
- [x] e2e/tests/catalog.spec.ts — CT-004 (по факту + комплект: добавить 2 строки, добавить
  и сразу удалить третью, отображение «Название ×qty» в таблице), CT-005 (свободный товар
  удаляется; товар в комплекте → 409 + архив вместо удаления; архивный не предлагается в
  комплекте нового), CT-006 (услуги) — ?q=/archived переживают reload, CT-008 расширен на
  /catalog/services и форму услуги (оба вьюпорта + тёмная тема).

### Self-check (все прогнаны локально агентом, все зелёные)
- [x] go build ./...
- [x] go vet ./... и go vet -tags=integration ./...
- [x] go test ./... -count=1 — все пакеты ok (включая новые services_validate_test.go)
- [x] go test ./internal/features/catalog/... -tags=integration -count=1 — ok
  (CatalogLiveSuite + ServicesLiveSuite, 12/12 тестов ServicesLiveSuite зелёные,
  прогнано отдельно с -v для подтверждения)
- [x] gofmt -l — чисто на всех новых/изменённых файлах пакета catalog
- [x] golangci-lint run ./internal/features/catalog/... — 0 issues
- [x] cd e2e && npx tsc --noEmit -p . — без ошибок
- [x] npx playwright test tests/catalog.spec.ts --list — 9/9 тестов найдены
  (CT-001..CT-008, включая новый CT-006 услуги)
- Реальный E2E-прогон (headed chromium) НЕ выполнялся этим агентом — владение
  прогонами/watch-gate — оркестратор (§EE, §SS-1). Особое внимание при прогоне:
  - CT-005 использует allowHttpErrors для ожидаемого 409 на
    `/catalog/products/{id}/delete`.

## FIX-LOOP итерация 1 (оркестратор: 2 прогона headed chromium, run1 8/9, run2 7/9)

1) **CT-008 — горизонтальный overflow на `/catalog/services/{id}` при 360px, когда у
   услуги ЕСТЬ строка комплекта** (пустая форма /new и списки — без overflow). Причина —
   тот же класс бага, что уже чинили в подфазе A для `.cat-panel input.form-control`:
   flex-обёртка строки комплекта (`<div class="flex-grow-1">` вокруг combobox товара)
   не имела `min-width:0` → её «automatic minimum size» как flex-item равен min-content
   содержимого (текст названия товара в `.cb-btn`/`.cb-label`), а `overflow:hidden` на
   `.cb-label` (потомке) НЕ сбрасывает min-content родительской flex-обёртки — раздвигало
   `.row-list-item` и всю страницу шире 360px. Фикс — `app.css`:
   `.row-list-item > div { min-width: 0; }` (общее правило для всех прямых div-детей
   строки row-list — товарная строка услуг, телефоны клиента, характеристики
   оборудования — везде безопасно и корректно). Шаблоны/JS/тест не трогал — только CSS.
2) **CT-001 — нарушение независимости тестов (§TD-6.2)**: тест полагался на пустой
   список услуг, падал на постоянной БД стенда вторым прогоном. Переписан:
   `e2e/tests/catalog.spec.ts` CT-001 теперь заводит свою категорию+услугу через
   `adminApi`/`uniq()` и проверяет её видимость через собственный `?q=`-фильтр
   (`catalog.gotoServices(query)` — добавлен параметр query по эталону `gotoProducts`),
   а не глобальную пустоту списка; редирект/вкладки/подсветка меню — без изменений (они
   и так не зависели от данных). Убрано требование «порядок тестов важен» — тест
   независим, `--shard`/другая сортировка не проблема.

Self-check после фикса: `go build ./...`, `go test ./internal/features/catalog/...
-tags=integration -count=1` (ok — CSS не задел Go), `cd e2e && npx tsc --noEmit -p .`
(без ошибок), `npx playwright test tests/catalog.spec.ts --list` (9/9). Реальный
headed-прогон не гонял — за оркестратором.

## FIX-LOOP итерация 1 (Code Review, вся фаза 4) — CHANGES REQUESTED: 1 HIGH, 3 MEDIUM, 5 LOW

Полный отчёт — `docs/features/phase-4-catalog/8-code-review/output.md`. Все пункты
исправлены с тестами (unit + live, HTTP-границы).

### HIGH — числовые границы NUMERIC → 500/молчаливое округление (Блокер 1)
Новый файл `internal/features/catalog/amount_validate.go`: `amountScale{IntDigits,
FracDigits}` (`priceScale`=NUMERIC(12,2): ≤10/≤2, `qtyScale`=NUMERIC(12,3): ≤9/≤3),
`validateAmount()` — decimalRe (мусор/экспонента/NaN/Inf → `invalid_format`) → лишняя
точность → `too_precise` (НЕ округляет молча) → переполнение целой части → `too_large`;
`errAmountOutOfRange`/`ErrAmountOutOfRange` + `mapAmountOverflow()` — страховка от 500 на
22003, если что-то всё же проскочит валидатор.
- `products_validate.go` — `addAmount` использует `validateAmount` (price=priceScale,
  stock/min_stock=qtyScale).
- `services_validate.go` — price/qty комплекта через `validateAmount`.
- `products_repo.go`: `mapProductWriteErr` — убрана НЕВЕРНАЯ ветка `22003→category_id`
  (сервис возвращал 400 под чужим полем); дельта `stock_movements` — новый сентинел
  `errStockDeltaOutOfRange`, отдельно от карточки товара (переполнение ДЕЛЬТЫ между двумя
  индивидуально валидными значениями остатка — реальный кейс, не гипотетический: старое
  -600000000 → новое 600000000 → дельта 1.2e9, 10 целых цифр > 9).
- `products_service.go` — `errAmountOutOfRange`→`ErrAmountOutOfRange` (400, без чужого
  поля), `errStockDeltaOutOfRange`→`Field{"stock", too_large}`.
- `services_repo.go` — `mapAmountOverflow` на INSERT/UPDATE `services.price` и на INSERT
  `service_products.qty` (страховка).
- Тесты: `products_validate_test.go`/`services_validate_test.go` — границы (1e10/
  12345678901/1.239/0.0001/1000000000 и т.д.) на `TestProductInput_Validate_Amounts`/
  `TestServiceInput_Validate_Price`/`TestServiceInput_Validate_ProductsRows`. Live:
  `catalog_http_live_test.go` `TestProduct_NumericBoundaries400` +
  `TestProduct_StockDeltaOverflow400` (дельта -600000000→600000000, экспонента — СЫРЫМ
  JSON-телом, т.к. `map[string]any{"price":"1e10"}` после `json.Marshal` стало бы JSON-
  строкой, которую `Amount.UnmarshalJSON` отклоняет ДО валидатора); `services_http_live_
  test.go` `TestService_NumericBoundaries400`. Все — 400 с правильным полем, ни одного 500.

### MEDIUM
1. **N+1 в ServiceService.List** — новый `ServiceRepo.ProductsBatch` (один запрос `WHERE
   service_id = ANY($1::uuid[])`, группировка в Go, эталон clients phonesBatch);
   `ServiceService.List` вызывает его один раз вместо `toResponse`/`repo.Products` на
   каждую услугу страницы; `toServiceResponse` вынесен в чистую функцию (переиспользуют
   List и одиночный `toResponse`).
2. **Архивный товар в комплекте при Update** — `services_repo.go`:
   `existingServiceProductIDs()` читает product_id, УЖЕ бывшие в комплекте этой услуги (в
   той же транзакции, до проверки), `checkProducts()` получил параметр `grandfathered
   map[string]bool` — для них `is_active` не проверяется (существование гарантировано FK),
   Create передаёт `nil` (у новой услуги прошлого комплекта нет). Тест:
   `services_http_live_test.go` `TestService_Update_ArchivedProductInComplect_Grandfathered`
   — ровно поток CT-005 (PRODUCT_IN_USE→архив): пересохранение услуги с той же архивной
   строкой проходит (даже с изменением qty), НОВАЯ строка с ДРУГИМ архивным товаром — 400.
3. **Дедуп ДО валидации** — `services_validate.go` переписан: `normalize()` больше не
   мержит (только trim+drop пустых), `validate()` проверяет КАЖДУЮ исходную строку
   (индексы — позиция ДО дедупа, форма их и перерисует), новая функция `mergeProducts()`
   вызывается ОТДЕЛЬНО и ТОЛЬКО после успешного `validate()` — `sumAmount()` больше не
   получает мусор/отрицательные/экспоненту на вход. Сумма дублей у верхней границы тоже
   проверяется (`mergeProducts` зовёт `validateAmount` на смерженные qty) — ошибка на
   общее поле `"products"` (не `products[i]` — после мержа индекс не соответствует ни
   одной исходной строке, обоснование — комментарий в коде). `services_service.go`:
   новая `prepareProducts()` — validate()→mergeProducts()→репозиторий. Тесты:
   `TestServiceInput_Validate_RejectsBeforeMerge` (регрессии из ревью: `[{p,"abc"},{p,"2"}]`,
   `[{p,"-5"},{p,"6"}]`, `[{p,"1e5"},{p,"1"}]` — все теперь 400 на первой строке, не
   принимаются суммой), `TestMergeProducts_SumsDuplicatesAfterValidation`,
   `TestMergeProducts_OverflowOnSum`.

### LOW (все закрыты)
1. **cat_error как код, не текст** — `ssr_categories.go`: `categoryErrCode(err)` (было
   `categoryErrText`) возвращает КОД (required/too_long/invalid_text/name_taken/in_use/
   not_found/unknown), новая `catErrorText(code)` — код→текст на сервере (эталон
   `flashes`); `ssr_products.go`/`ssr_services.go` — `CatError: catErrorText(c.Query(...))`
   вместо сырого значения query.
2. **LoadTabCounts не логировался** — `ssr.go` (catalog) и `equipmenttypes/ssr.go`:
   `tabs()` теперь `slog.ErrorContext(..., "error", err, "request_id",
   requestid.FromContext(c))` перед возвратом нулевых счётчиков.
3. Закрыт пунктом 1 выше (`catErrorText` даёт `required`/`too_long`/`invalid_text` вместо
   общей фразы; «Не удалось выполнить действие» — только на непредвиденный код).
4. **Комплект услуги — combobox+data-hint (уже было) + data-once** —
   `services-form.html`: добавлен `data-once` на форму удаления + инлайн-скрипт
   подтверждения теперь сам блокирует кнопку (`btn.disabled = true`) перед
   `form.submit()` — задокументировано, ПОЧЕМУ глобальный делегированный `data-once`-
   обработчик (`ui-dialog.js`) здесь не сработает сам (`HTMLFormElement.submit()` не
   генерирует событие `submit`, на чём и построен обход зацикливания confirm-формы).
   Combobox/data-hint у select товара уже были в разметке подфазы B — проверено, что
   `combobox.js` (`select.form-select` + `data-hint` на option) действительно
   энхансится (MutationObserver подхватывает клонированные row-list-строки).
5. **Исполнитель читает архивную запись по id** — доккомментарии добавлены в
   `products_service.go`/`services_service.go` `Get()`: as-is по прецеденту
   `equipmenttypes.Service.Get` (D19 «активные записи» — про List/`include_archived`, не
   про Get по конкретному id) — сверено построчно, поведение идентично.

### Гейты (все прогнаны мной синхронно, все зелёные)
- `go build ./...`, `go vet ./...`, `go vet -tags=integration ./...`
- `go test ./... -count=1` — все пакеты ok
- `golangci-lint run ./...` — 0 issues
- `go test ./internal/features/catalog/... -tags=integration -count=1 -race` — ok,
  24/24 live-теста (CatalogLiveSuite 14 + ServicesLiveSuite 14, включая все новые)
  подтверждены `-v` поимённо
- `cd e2e && npx tsc --noEmit -p .` — без ошибок
- `npx playwright test tests/catalog.spec.ts --list` — 9/9 (набор тестов не менялся)

## FIX-LOOP итерация 2 (Code Review, повторный APPROVED с 2 регрессиями) — обе исправлены

1. **MEDIUM: индекс errServiceProductInvalid считался по СМЕРЖЕННОМУ списку.**
   `[A,A,B(архив)]` → после дедупа `[A(сумма),B]`, ошибка `products[1]` в СМЕРЖЕННОМ
   списке — но SSR-форма/API-клиент рисуют ИСХОДНЫЕ строки, где индекс 1 — вторая A
   (валидная), а не B (индекс 2). Фикс: `services_service.go` `prepareProducts()` больше
   не заменяет `in.Products` смерженным списком — возвращает его ОТДЕЛЬНО
   (`(ServiceInput, []ServiceProductInput, error)`); `services_repo.go`
   `Create`/`Update` получили параметр `merged []ServiceProductInput` — `checkProducts`
   (существование/активность/грандфазер) теперь проверяет ИСХОДНЫЙ `in.Products`
   (индекс = позиция в исходном списке), а `replaceServiceProducts` пишет `merged`.
   Тест: `services_http_live_test.go`
   `TestService_DuplicateThenArchived_ErrorIndexPointsAtArchivedRow` — ровно кейс из
   ревью `[A,A,B(архив)]`, проверяет `products[2]` (не `products[1]`).
2. **LOW: переполнение суммы дублей → поле `"products"`, но форма его не выводила.**
   `services-form.html` — добавлен блок `{{ with index .Errors "products" }}<div
   class="alert alert-danger py-2" data-testid="error-products">{{ . }}</div>{{ end }}`
   над блоком комплекта (Go-код `serviceFormError` уже клал `fieldText(msgTooLarge)` в
   `vm.Errors["products"]` — просто некуда было рендерить). Тест:
   `TestSSR_MergeOverflow_ShowsVisibleError` — POST через SSR-форму с двумя дублями по
   9 цифр (сумма 10 цифр > qtyScale.IntDigits), проверяет 422 + `data-testid="error-
   products"` + текст «Слишком большое число» в HTML-ответе.

Гейты после фикса — все зелёные: `go build`, `go vet` (+`-tags=integration`),
`go test ./... -count=1`, `golangci-lint run ./...` (0 issues), `go test
./internal/features/catalog/... -tags=integration -count=1 -race` (26/26 живых теста,
включая оба новых, подтверждены `-v` поимённо), `cd e2e && npx tsc --noEmit -p .`,
`npx playwright test tests/catalog.spec.ts --list` (9/9, набор не менялся).

E2E реальный прогон не гонял — владение прогонами у оркестратора.

### Итог
Услуги реализованы целиком: backend (API+SSR+транзакционный комплект), unit+integration
(LiveSuite)+E2E тесты по CLAUDE.md §Testing doctrine (TD-1/TD-3/TD-4/TD-8), TD-4
adversarial покрыт (authz/IDOR/валидация/CSRF в services_http_live_test.go). Не
реализовано намеренно (вне объёма подфазы, по task.md): проверка «услуга используется в
заказах» при удалении (фаза 5/6 — заказов ещё нет).
