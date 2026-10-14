# Фаза 4 — Каталог: постановка (pipeline-light)

**Tier:** M (light по D31, подтверждено пользователем) | **Совместимость:** MVP (CM-1) | **UI-scope:** swagger:no / adaptive:min / themes:both
**Источник требований:** docs/plan.md §3 (модель), §4 (API «Каталог»), §5 (экран 5), решения D3, D6, D10, D12, D16, D17, D19, D26, D28, D43, D44. Эталон UI/текстов — prototype/index.html: renderCatalog / openService / openProduct (строки ~1444–1595).
**Эталон кода (D43):** internal/features/clients + internal/features/equipmenttypes (плоский пакет: dto/validate/repo/service/handler/routes/ssr + live-тесты), web/ssr/clients|equipmenttypes/templates, web/ssr/_shared/js (row-list.js, ui-dialog.js data-confirm/data-once, combobox.js), e2e/ (POM, fixtures/test.ts, fixtures/auth.ts AdminApi).

## Модель (миграция 0004_catalog)
- `catalog_categories(id uuid, kind text CHECK service|product, name, position int, is_active bool, created_at, updated_at)`; UNIQUE (kind, lower(btrim(name))) — D16.
- `products(id, category_id → catalog_categories RESTRICT, name, sku, unit, price NUMERIC(12,2) ≥0, is_active, stock NUMERIC(12,3) DEFAULT 0 (может быть < 0), min_stock NUMERIC(12,3) ≥0 DEFAULT 0, box, place, created_at, updated_at)`.
- `services(id, category_id → catalog_categories RESTRICT, name, price NUMERIC(12,2) NULL (NULL = «по факту»), is_active, created_at, updated_at)`.
- `service_products(service_id → services CASCADE, product_id → products RESTRICT, qty NUMERIC(12,3) > 0, position, PK(service_id, product_id))` — комплект (D10).
- `stock_movements(id, product_id → products RESTRICT, task_id uuid NULL (FK появится в фазе 5), qty_delta NUMERIC(12,3) ≠ 0, reason text CHECK task_consume|task_return|manual, user_id → users, created_at)` — D44.
- Единицы измерения (unit): список из прототипа (UNITS) — enum-строка, валидируется.

## Поведение
- **Категории:** вид услуги/товары; создать, переименовать (D16: дубль без учёта регистра/пробелов → 409 `CATEGORY_NAME_TAKEN`); удалить неиспользуемую → DELETE; используемую → 409 `CATEGORY_IN_USE` + архив `is_active=false` (D26), вернуть из архива. Архивная категория не предлагается в выборе для новых записей.
- **Товары:** категория (обяз.), название (обяз.), артикул, единица, цена ≥0, остаток (дробный, может быть отрицательным), минимум ≥0, коробка, место. Ручное изменение остатка при сохранении карточки → запись `stock_movements` (reason=manual, delta=новое−старое, user) в той же транзакции; delta=0 → записи нет. Удалить неиспользуемый → DELETE; используемый в комплекте услуги → 409 `PRODUCT_IN_USE` + архив (D26). Архивный товар не предлагается при добавлении в комплект.
- **Услуги:** категория (обяз.), название (обяз.), цена работы (пусто = «по факту», ≥0), комплект товаров (товар + количество >0, дубли товара объединяются, пустой комплект допустим). Удалить → DELETE (проверка «используется в заказах» — фаза 5/6, как D26 у клиентов). Архив `is_active`.
- **Цены** хранятся в каталоге; копирование в заказ — фаза 6 (D3).
- **Права (D19):** исполнитель — только чтение API каталога (активные записи; понадобится форме заказа фазы 5), мутации 403; SSR «Каталог» — admin-only. `version` нет — last-write-wins (D28).
- **API (§API-1, snake_case, конверт {items,total,limit,offset}):** `/api/v1/catalog-categories?kind=&include_archived=` (GET/POST, GET/PUT/DELETE `/:id`), `/api/v1/products?category_id=&q=&include_archived=` (+ `/:id`), `/api/v1/services?category_id=&q=&include_archived=` (+ `/:id`, комплект — массив `products[{product_id, qty}]` в теле, ответ с названием/ед./ценой товара). Лимиты: default 50 / max 500 (справочник для селекторов). Мусор → дефолты, битый uuid → 404, NUL/UTF-8 → 400 `invalid_text` (pkg/sqlutil.HasInvalidText), строгий JSON.

## Экран «Каталог» (SSR, admin)
- `/catalog` → 303 `/catalog/services`. Вкладки: «Услуги» `/catalog/services`, «Товары» `/catalog/products`, «Типы оборудования» `/catalog/equipment-types` (существующий экран фазы 3 получает ту же полосу вкладок). Счётчики на вкладках.
- Слева — категории текущего вида («Все категории» + категории со счётчиками; фильтр `?category=` в URL), «+ Категория» (форма или диалог KT.uiPrompt + POST), переименовать/архив/удалить категорию.
- Поиск `?q=` по названию (у товаров — ещё артикул/коробка/место). «Показать архивные».
- Таблица услуг: услуга, категория, цена работы («по факту» при NULL), товары в комплекте (`Название ×qty`). Таблица товаров: товар, артикул, категория, ед., цена, в наличии (≤ минимума — выделено, < 0 — «долг»), где лежит (место · коробка).
- Формы (страницы `/catalog/services/new`, `/catalog/services/:id`, аналогично products) — PRG, ошибки под полями, удаление POST-формой с `data-confirm`, `data-once`. Комплект услуги — `row-list.js` (строка: товар через combobox + количество), подсказка цены/ед.

## Тесты (ID для судейства)
- Unit: валидаторы категорий/товаров/услуг (границы, trim, эмодзи, числа, отрицательный остаток, NUL).
- Live (`catalog_http_live_test.go` или по пакетам, testutil.LiveSuite/Browser): CRUD, D16 409, D26 IN_USE 409 + архив, комплект (дубли, qty ≤0 → 400, архивный товар → 400), stock_movements manual (delta, автор, delta 0 — без записи), authz (исполнитель чтение 200 / мутации 403 / SSR 403-страница; аноним 401/редирект), IDOR/битый uuid 404, мусор 400, CSRF 403.
- E2E (`e2e/tests/catalog.spec.ts`, POM `e2e/pages/CatalogPage.ts`), заголовки `CT-NNN`:
  - CT-001 вкладки и `/catalog` → услуги; меню «Каталог» подсвечено.
  - CT-002 категории: создать, дубль → сообщение, переименовать, фильтр слева в URL.
  - CT-003 товар: создать/править (все поля), валидация полей (§TD-6.5), остаток ≤ минимума выделен, отрицательный — «долг».
  - CT-004 услуга с ценой «по факту» и комплектом (добавить/удалить строки, количество), показ в таблице.
  - CT-005 удаление: неиспользуемый товар удаляется; товар из комплекта → сообщение + архив; архивный не предлагается в комплект.
  - CT-006 поиск `?q=` и «Показать архивные» переживают reload.
  - CT-007 исполнитель: нет пункта меню, `/catalog/services` → «Нет доступа».
  - CT-008 360/1280 без горизонтального overflow, тёмная тема.
