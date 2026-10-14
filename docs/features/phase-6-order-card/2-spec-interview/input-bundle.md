# Input Bundle: phase-6-order-card / стадия 2-spec-interview — generated 2026-09-27 10:49

> Твой ЕДИНСТВЕННЫЙ документный вход (пайп-контракт). Upstream-доки напрямую НЕ читать;
> точечный grep по конкретному ID (DS-NNN/AC-NNN/TS-NNN) — разрешён. Отсутствие нужного
> факта в бандле = сигнал оркестратору (дырка контракта), не повод читать всё.

## Источник: 1-research/output.md [sections]
## Triage
```yaml
scope: cross-feature
tier: M
touches:
  db: true
  fe: false
  infra: false
  external: false
risk: reversible
novelty: standard-crud
decisions: []
```
Обоснование: фича в `docs/plan.md:6` треком `/pipeline (M)` явно (D31, `docs/plan.md:155`). Не Vue — SSR + vanilla JS (`web/ssr/_shared/js/board.js:1`), `touches.fe=false` формально. `scope=cross-feature`: новая таблица subtask_logs, новые эндпоинты subtasks/subtask-products/subtask-logs/task-equipment/print, расширение tasks/board.js/SSR-печати, переиспользование API оборудования из фичи clients.

## DB Schema (baseline)
Источник целиком: `migrations/0005_board.up.sql`, `migrations/0003_clients_equipment.up.sql`, `migrations/0004_catalog.up.sql`.

**tasks** (`migrations/0005_board.up.sql:25-44`): id bigserial PK(D37), column_id uuid FK columns RESTRICT, client_id uuid FK clients RESTRICT, title text CHECK(btrim<>''∧len≤200), description_html text DEFAULT '' (:30 — фаза 5 пишет только seed-demo), assignee_id uuid FK users SET NULL, due_date date NULL, is_paid boolean, paid_at timestamptz NULL (CHECK is_paid=(paid_at IS NOT NULL), :40), position numeric, created_by uuid FK users RESTRICT, created_at/updated_at, version bigint DEFAULT 1. Индексы :42-44.

**subtasks** (`migrations/0005_board.up.sql:46-62`, уже создана, Go читает только `OpenSubtasks`): id uuid PK, task_id bigint FK tasks CASCADE, service_id uuid FK services RESTRICT, status text CHECK IN('new','in_progress','done') DEFAULT 'new', assignee_id uuid FK users SET NULL, due_date date NULL, price numeric(12,2) NULL CHECK(≥0), equipment_id uuid FK equipment RESTRICT NULL, position int DEFAULT 0 (НЕ дробный, нет NextPosition-аналога). Индексы :59-62.

**subtask_products** (`migrations/0005_board.up.sql:64-72`, уже создана): id uuid PK, subtask_id uuid FK subtasks CASCADE, product_id uuid FK products RESTRICT, qty numeric(12,3) CHECK(>0) (D33 дробное), price numeric(12,2) NULL CHECK(≥0). Индексы :71-72.

**task_stock** (`migrations/0005_board.up.sql:74-80`, уже использует StockEngine): task_id bigint FK tasks CASCADE, product_id uuid FK products RESTRICT, qty numeric(12,3) CHECK(>0), PK(task_id,product_id).

**task_equipment** (`migrations/0005_board.up.sql:82-89`, создана, Go не читает/не пишет кроме E2E-фикстуры): task_id bigint FK tasks CASCADE, equipment_id uuid FK equipment RESTRICT, completeness text DEFAULT '', condition text DEFAULT '', PK(task_id,equipment_id).

**stock_movements** (создана `migrations/0004_catalog.up.sql:54-63`, task_id мигрирован uuid→bigint в `migrations/0005_board.up.sql:91-96`, `FK task_id→tasks ON DELETE SET NULL`): id uuid PK, product_id uuid FK products RESTRICT, task_id bigint NULL, qty_delta numeric(12,3) CHECK(<>0), reason text CHECK IN('task_consume','task_return','manual'), user_id uuid FK users RESTRICT, created_at.

**equipment** (`migrations/0003_clients_equipment.up.sql:33-47`): id uuid PK, client_id uuid FK clients CASCADE, type_id uuid FK equipment_types RESTRICT, brand/model(CHECK btrim<>'')/sku/serial text, specs jsonb DEFAULT '[]' CHECK(array), note text, created_at/updated_at. Индексы :46-47.

**equipment_types** (`migrations/0003_clients_equipment.up.sql:22-31`): id uuid PK, name text CHECK(btrim<>''), spec_keys text[] DEFAULT '{}', is_active boolean DEFAULT true; UNIQUE lower(btrim(name)) (:31).

**services/products/service_products** (`migrations/0004_catalog.up.sql:17-51`): services(id, category_id FK RESTRICT, name, price numeric(12,2) NULL, is_active) (:34-43); products(id, category_id, name, sku, unit, price numeric(12,2) DEFAULT 0, is_active, stock/min_stock numeric(12,3), box, place) (:17-31); service_products(service_id FK CASCADE, product_id FK RESTRICT, qty numeric(12,3) CHECK>0, position int) (:45-51).

**subtask_logs — ОТСУТСТВУЕТ.** Проверено: полный список миграций `migrations/0001…0005` (5 файлов up/down) не содержит `subtask_logs`. Журнал работ услуги (D13/D27, `docs/plan.md:56,63,138,151`) — НОВАЯ таблица фазы 6; ожидаемые колонки из `docs/plan.md:56`: `subtask_logs(id, subtask_id, user_id, text, created_at, updated_at)`.

## Frontend
Не применимо в терминах Vue — SSR (`Go html/template` + vanilla JS, `docs/plan.md:5`), `frontend/` отсутствует. Фактический "фронт" фичи ниже.

- `web/ssr/board/templates/pages/board.html:1` (133 строки) и `web/ssr/board/templates/pages/stock.html:1` (78 строк) — HTML-шаблоны доски/остатков.
- `web/ssr/_shared/js/board.js:548-667` (815 строк в файле): `openTaskForm(task, defaultColumnId)`(:548-569) — ТОЛЬКО простая форма (title/client/column/assignee/due/paid), без описания/оборудования/услуг/товаров/журнала/итогов; `taskFormPayload()`(:587-602) шлёт `{title,client_id,column_id,assignee_id,due_date,is_paid}` — is_paid ВСЕГДА отправляется даже при disabled-чекбоксе у executor (:590-593, важно сохранить при расширении — иначе тихий сброс оплаты); `bindTaskForm()`(:610-667) — submit→PUT/POST, catch различает `TASK_VERSION_CONFLICT`(:638-643) от `err.fields`(:644-646) от общей(:648); `finishOpenTaskForm`(:571-585) — `data-testid="task-delete"` снимается АТРИБУТОМ, не только `d-none` (:576-579) — конвенция для новых admin-only кнопок.
- `web/ssr/_shared/js/api.js:1` — `KT.api` fetch-обёртка с CSRF, используется всеми хендлерами.
- Общие компоненты: `web/ssr/_shared/js/combobox.js:1`, `web/ssr/_shared/js/ui-dialog.js:1`(`KT.uiConfirm/uiAlert`), `web/ssr/_shared/js/nav-badge.js:1`, `web/ssr/_shared/js/theme.js:1`.
- `e2e/pages/BoardPage.ts:1`, `e2e/pages/EquipmentPage.ts:1` и др. в `e2e/pages/` — POM; `e2e/fixtures/board.ts:1`, `e2e/fixtures/board-types.ts:1` — типы фикстур.

## Паттерны в коде
- Многосущностный feature-пакет: файлы-по-сущности (dto/handler/repo/service/validate) в ОДНОМ пакете `board` — эталон `internal/features/board/errors.go:1-4`; фаза 6 добавляет `subtasks`-файлы в ТОТ ЖЕ пакет, не отдельную фичу.
- Lock Order (ADR-104): `columns → tasks → products`, FOR UPDATE/FOR SHARE в этом порядке внутри `withTx`; новые мутации `subtasks`/`subtask_products` ДОЛЖНЫ брать `LockTask` ПЕРЕД правкой — см. LOW code review ниже (Граничные случаи).
- Re-fetch-after-write: ответ mutation-хендлера из `repo.Get()` после commit, не RETURNING (`internal/features/board/tasks_service.go:330-340`).
- Sentinel-ошибки — общие `var Err... = errorspkg.New(...)` на пакете (`internal/features/board/errors.go:15-22`), `.WithDetail` — КОПИЯ (`pkg/errorspkg/errors.go:36-44`), sentinel не мутируется.
- Optimistic locking (`version`) — только `clients`/`tasks` (D28, `docs/plan.md:152`); `subtasks`/`subtask_products` — last-write-wins без version (паттерн уже проверен на `tasks.Move`).
- DTO-валидация в методах запроса `(in Request) normalize()/validate()` (`internal/features/board/tasks_validate.go:30-100`), не в handler/service.
- Денежные/количественные поля — `pkg/numeric.Amount` (строка в Go, число на wire) — обязателен для `subtask_products.qty/price`, `subtasks.price`, итогов заказа.
- `withTx` с ретраем 40P01/40001×3 (`internal/features/board/tx.go:16-29`) — обязателен для мутаций склада/нескольких таблиц.
- Testid снимается атрибутом (не только `d-none`) для элементов, видимых только одной роли (`web/ssr/_shared/js/board.js:576-579`).

## Источник: draft-spec.md [full]
MISSING

## Источник: draft-spec-decisions.md [full]
MISSING

## Паспорт (выжимка)
# Passport: phase-6-order-card (generated — руками не править; stage_bundle.py)
**Tier:** M | **Совместимость:** ? (pipeline-doctrine.md §CM-1; ? → уточни у оркестратора)
**Трек:** обычный (prefs нет — дефолт) — режимы агентов (`spec-track delta/micro/mini/merged`) включает ТОЛЬКО строка `Режим:` в prompt оркестратора; DS/TS-индексы ниже присутствуют на ЛЮБОМ треке при наличии draft-spec.
**Layout:** автодетект (tools/layout_resolve.py)
**UI-scope (pipeline-doctrine.md §US-1, 0-setup/ui-scope-prefs.md):** swagger:no | adaptive:min | themes:both

**Индексы:** AC 0 / DS 0 / TS 0 — полные списки: `0-setup/passport.md` (разрешён контрактом); конкретное решение — точечный grep по `DS-NNN`/`TS-NNN` в draft-spec §9/§11 (разрешён).
