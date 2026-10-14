# Input Bundle: phase-3-clients / стадия 3-design — generated 2026-09-25 14:37

> Твой ЕДИНСТВЕННЫЙ документный вход (пайп-контракт). Upstream-доки напрямую НЕ читать;
> точечный grep по конкретному ID (DS-NNN/AC-NNN/TS-NNN) — разрешён. Отсутствие нужного
> факта в бандле = сигнал оркестратору (дырка контракта), не повод читать всё.

## Источник: 2-spec-interview/output.md [full]
---
stage: 2-spec-interview
feature: phase-3-clients
tier: M
---
# Spec: Клиенты и оборудование (фаза 3)
**Дата интервью:** 2026-09-25
**На основе:** PRD `2-spec-interview/prd.md` (approved, AC-001…AC-014) + input-bundle (research) + docs/plan.md §1–§7 (D1–D42) + prototype/index.html
**Tier:** M
**Совместимость:** MVP (CM-1)
**UI-scope:** swagger:no | adaptive:min | themes:both (US-1)
**Журнал решений:** `2-spec-interview/spec-decisions.md` (батчи 1–2)

## Применённые правила (zap-list)
- Формат ошибок `{code,message,details?}` (`pkg/errorspkg`); 400 VALIDATION с `details.fields[]`; 404/403/409.
- CSRF double-submit на всех мутациях, вкл. `/api/v1` (D40) — без изменений.
- Rate limit — только login (D41), вне фичи.
- Синхронная обработка; аудит-лог не нужен (slog + trace_id).
- Миграции — только `make migrate-up`; следующая свободная `0003_*`.
- Права — из БД на каждый запрос (D42); гарды `RequireAPI`/`RequireSSR`/`RequireAdmin`.
- ID — `DEFAULT gen_random_uuid()` в БД; Go валидирует `uuid.Validate`.
- Wire JSON snake_case, списки `{items,total,limit,offset}` (API-1); URL kebab-case.
- swagger:no → swag-аннотации не пишутся.
- Новых env vars/сервисов/интеграций нет.
- Tracer/OTel/slog — штатные (`internal/observability`).

## Технические ограничения
- **Код (отклонение от D30, решение батча 2 В-1):** вручную, без `/z-scaffold` → 2 плоских пакета по образцу `internal/features/users` (`dto.go`/`handler.go`/`service.go`/`repo.go`/`ssr.go`/`validate.go`/`routes.go`):
  - `internal/features/clients` — clients + client_phones + equipment (реестр);
  - `internal/features/equipmenttypes` — equipment_types.
  Причина → 7 расхождений z-scaffold (research), JSONB-характеристики и phones 1..N всё равно ручные. D30 в силе для фазы 4. Шаблоны — `web/ssr/{clients,equipmenttypes}/templates`, общий layout, VM с `Shell` (D38).
- **Схема (plan §3, миграция `0003_*`):** `clients(+version bigint)`, `client_phones`, `equipment_types(name, spec_keys text[], is_active)`, `equipment(specs jsonb [{k,v}])`. FK: `client_phones.client_id`, `equipment.client_id` → `ON DELETE CASCADE`; `equipment.type_id` → `ON DELETE RESTRICT` (страховка к `EQUIPMENT_TYPE_IN_USE`). (driver: AC-006, AC-012)
- **Уникальность:** `equipment_types` — unique по `lower(btrim(name))` → 409 `EQUIPMENT_TYPE_NAME_TAKEN` (driver: AC-011; аналог D16). Компания, SN/VIN — без уникальности (driver: батч 1 В-4ж).
- **Телефоны:** хранятся внутри клиента; PUT клиента = replace-all набора телефонов в одной транзакции с проверкой+инкрементом `version` (driver: AC-003, AC-005).
- **Оптимистичная блокировка (D28):** `version` только у clients; API — поле body `version`; SSR — hidden input; mismatch → 409 `CLIENT_VERSION_CONFLICT`. Equipment/types — LWW. Правка оборудования `clients.version` не меняет. DELETE клиента version не проверяет. (driver: AC-005 — «ничьи правки не теряются молча»)
- **Шаблон характеристик:** применяется только в форме (создание/смена типа, заполненные значения сохраняются); сервер хранит `specs` как прислано; смена `spec_keys` существующие `equipment.specs` не трогает. (driver: AC-009)
- **Архивный тип:** создание единицы или смена `type_id` на архивный → 400 поле `type_id`; правка единицы без смены архивного типа — ок. (driver: AC-008, AC-012)
- **D26 в фазе 3 (решение батча 2 В-5 → «всё в фазу 5»):** проверки «клиент с заказами» / «оборудование было в заказе» — НЕ реализуются (ни заглушка, ни коды, ни тесты); `tasks` нет. DELETE клиента/оборудования в фазе 3 — безусловный. Реализация + коды `CLIENT_HAS_ORDERS`/`EQUIPMENT_IN_ORDERS` + тесты — фаза 5. (driver: AC-006, AC-010 — оговорка «с фазы 5»)
- **Список клиентов:** `limit` 50 (дефолт), счётчик «всего N» без номеров страниц (как users); поиск `q` — ILIKE по company_name / contact_name / client_phones.phone с `escapeLike()`; сортировка company_name; число единиц оборудования — в том же запросе (без N+1). Состояние в URL query. (driver: AC-004)
- **Двойной submit:** disable кнопки на время отправки + PRG; серверный дедуп создания не делается (нет естественного ключа); повторный DELETE → 404.

### Лимиты и нормализация (driver: AC-002, AC-008, AC-011)
Все строки — trim; длина в символах (rune); превышение → 400 VALIDATION с полем.
| Сущность | Поле | Правило |
|---|---|---|
| client | company_name | обязательно, 1–200 |
| client | address | ≤500 |
| client | contact_name, contact_position | ≤200 |
| client | phones | 1–20 после отброса пустых строк |
| phone | phone | ≤32, ≥1 цифра, формат свободный |
| phone | note | ≤200 |
| equipment | type_id | обязательно, uuid существующего; активный при создании/смене |
| equipment | model | обязательно, 1–200 |
| equipment | brand, sku, serial | ≤200 |
| equipment | note | ≤1000 |
| equipment | specs | ≤50 пар; k ≤100, v ≤500; пара без k и v отбрасывается; v без k → ошибка «укажите характеристику»; дубли k разрешены |
| equipment_type | name | обязательно, 1–100 |
| equipment_type | spec_keys | ≤50 ключей × ≤100; пустые и дубли (case-insensitive) отбрасываются молча; порядок сохраняется |

### API (plan §4; driver: AC-001…AC-013)
| Метод | URL | admin | executor |
|---|---|---|---|
| GET | `/api/v1/clients` (`q,limit,offset`) | ✅ | ✅ чтение |
| POST | `/api/v1/clients` | ✅ | 403 |
| GET | `/api/v1/clients/:id` (с `phones[]`, `version`) | ✅ | ✅ |
| PUT | `/api/v1/clients/:id` (с `version`, `phones[]`) | ✅ | 403 |
| DELETE | `/api/v1/clients/:id` | ✅ | 403 |
| GET | `/api/v1/clients/:id/equipment` | ✅ | ✅ |
| POST | `/api/v1/clients/:id/equipment` | ✅ | ✅ (D19) |
| GET | `/api/v1/equipment/:id` | ✅ | ✅ |
| PUT/DELETE | `/api/v1/equipment/:id` | ✅ | 403 |
| GET | `/api/v1/equipment-types` (`include_archived=true`) | ✅ | ✅ (только активные — Q-004) |
| POST/PUT/DELETE | `/api/v1/equipment-types[/:id]` (архив/восстановление — PUT `is_active`) | ✅ | 403 |

### SSR-страницы (admin-only, driver: AC-013; батч 1 В-4г/д)
`/clients`, `/clients/new`, `/clients/:id`, `/clients/:id/equipment/new`, `/equipment/:id`, `/catalog/equipment-types` (+ `/new`, `/:id`). Отдельные страницы + PRG вместо вложенных модалок прототипа; confirm/alert — `KT.uiConfirm/uiAlert`; select типа — `combobox.js` (D6). Меню «Каталог» → `/catalog/equipment-types`.

## Enum-поля фичи (CLAUDE.md §8.1)
Не применимо — enum-полей нет (text / boolean `is_active` / `text[]` / `jsonb`); swagger:no.

## Безопасность и доступ
- SSR-экраны фичи — `RequireSSR`+`RequireAdmin`; executor → страница «Нет доступа» (403, `status.html`); меню скрыто (`AdminOnly:true` в `internal/ssr/menu.go`). (driver: AC-013)
- API: чтение — любой аутентифицированный; `POST /clients/:id/equipment` — admin+executor (D19); прочие мутации — `RequireAdmin` → 403. (driver: AC-013)
- Несуществующий/чужой id → 404 (единое пространство данных, IDOR = проверка существования и роли). Битый uuid → 400 или 404 без 500.
- Чувствительные данные: ФИО/телефоны представителей — только за аутентификацией; в логи не пишутся значения полей.
- Вывод в шаблоны — только через экранирование `html/template`; XSS-строки в полях — E2E-проверка (образец фазы 1).

## UI/UX требования
- Эталон полей/текстов — `prototype/index.html` (openClient/openEquipment/openEqType/renderClients/renderEqTypes); вид — Bootstrap + `app.css`.
- Ошибки валидации — у поля, форма сохраняет ввод. (driver: AC-002, AC-008)
- 409 version — форма с вводом + «Карточку уже изменил другой пользователь — ваши правки не сохранены» + «Открыть актуальную версию». (driver: AC-005)
- Удаление клиента — confirm «вместе с ним будет удалено N единиц оборудования». (driver: AC-006)
- Динамические строки телефонов/характеристик — vanilla JS (образец `drawPhones`/`drawSpecs` прототипа); шаблон типа подставляется на клиенте при выборе типа. (driver: AC-003, AC-009)
- Список типов — фильтр «Показать архивные», кнопки «В архив»/«Вернуть». (driver: AC-012)
- adaptive:min — нет horizontal overflow на ~360px и 1280px; themes:both. (driver: AC-014)
- Операций >3 с нет.

## Обработка ошибок
| Ошибка | HTTP / code | Что видит пользователь | Действие системы | Driver |
|---|---|---|---|---|
| Пустая компания / нет телефона / превышение длины / телефон без цифр | 400 `VALIDATION` fields[] | сообщение у поля, ввод сохранён | не сохраняет | AC-002, AC-003 |
| Нет модели/типа, v без k, >50 specs | 400 `VALIDATION` | сообщение у поля | не сохраняет | AC-008, AC-009 |
| Архивный тип на новой единице / смене типа | 400 `VALIDATION` (type_id) | «тип в архиве» у поля | не сохраняет | AC-008, AC-012 |
| Устаревший `version` | 409 `CLIENT_VERSION_CONFLICT` | сообщение AC-005 + «Открыть актуальную версию» | транзакция откатывается | AC-005 |
| Дубль названия типа | 409 `EQUIPMENT_TYPE_NAME_TAKEN` | «Такой тип уже есть» | не сохраняет | AC-011 |
| DELETE используемого типа | 409 `EQUIPMENT_TYPE_IN_USE` | «тип используется — отправьте в архив» | не удаляет | AC-012 |
| Клиент/единица/тип не найдены | 404 | «Не найдено» | — | AC-005, AC-008 |
| Executor: мутация справочника / SSR-экран | 403 | «Нет доступа» | данные не меняются | AC-013 |
| Мусорный body/неизвестные поля/битый uuid | 400 | — | не 500 | TD-4 |
| Клиент с заказами / единица в заказе | — (фаза 5) | — | в фазе 3 не проверяется | AC-006, AC-010 |
Частичное выполнение не допускается: клиент+телефоны — одна транзакция. Retry — ручной (пользователь повторяет).

## Операции и наблюдаемость (M)
### Spans
| Имя | Tracer | Log fields | Когда |
|---|---|---|---|
| штатные HTTP-spans OTel middleware | существующий (`internal/observability/otel.go`) | `trace_id` (slog) | каждый запрос |
Новых ручных spans не требуется.

### Метрики и алерты
| Метрика | Порог | Действие on-call |
|---|---|---|
| — | deferred | Новых метрик/алертов нет: внутренний инструмент, прода нет (CM-1 MVP); штатные HTTP-метрики Prometheus `/metrics` покрывают. Алерты — при появлении прода (инфраструктурная фаза). |
Runbook не нужен (MVP, прода нет).

## Performance & Rollout
Не применимо: `risk=reversible`, не hot-path, MVP (Блок 10 не триггерится). Rollout — обычный merge; feature flag не нужен.

## Принятые компромиссы
- D26-проверки удаления отложены целиком в фазу 5 → в фазе 3 можно удалить клиента/единицу без проверки заказов (заказов физически нет).
- Без серверной защиты от дублей создания клиента (нет естественного ключа).
- Колонка «Заказов» и история заказов единицы не показываются до фаз 5/6.
- Шаблон типа не мигрирует характеристики существующих единиц.
- Технический долг: фаза 5 обязана добавить D26-проверки в `clients`-сервис + коды + тесты.

## Отклонённые решения
- `/z-scaffold` для справочников фазы 3 (D30) → 7 расхождений с паттерном проекта; ручные плоские пакеты.
- Вложенные модалки прототипа (клиент → оборудование) → отдельные страницы + PRG (прямые ссылки, простой E2E, как users).
- Заглушка D26 с unit-тестом сейчас → отклонено пользователем (всё в фазу 5).
- Проверка `version` при DELETE клиента → отклонено (удаление подтверждается confirm).
- If-Match для version → поле body (FE-1-конвенция, plan).
- Уникальность компании / SN/VIN → нет.

## Открытые вопросы

### Q-001: Демо-данные клиентов в `make seed-demo`
Метрика PRD §3 «100% демо-клиентов в реестре к старту фазы 5» и D36 (`seed-demo` — данные прототипа). Design: расширять ли `seed-demo` клиентами/телефонами/типами/оборудованием из прототипа в этой фазе и как (SQL/Go-сид, идемпотентность).

### Q-002: Общий JS-повторитель строк
Телефоны (AC-003) и характеристики (AC-009) — динамические списки; в SSR-стеке паттерна нет. Design: общий модуль в `web/ssr/_shared/js/` (переиспользуем фазой 4 — комплект товаров услуги) или код в фиче; как передавать шаблоны типов в форму (data-атрибуты `<option>` / JSON в странице) с учётом архивных типов у существующей единицы.

### Q-003: Поиск по телефону и индексы
AC-004: ILIKE по company/contact/phone через join `client_phones` → дубли строк клиента / `total`. Design: форма запроса (EXISTS vs DISTINCT), нормализация телефона при поиске (игнор пробелов/скобок/дефисов?), нужны ли индексы (pg_trgm не вводить без необходимости).

### Q-004: Видимость архивных типов для executor и в формах
Executor читает типы для будущей формы заказа (AC-013). Design: разрешён ли executor `include_archived=true`; как форма оборудования показывает текущий архивный тип существующей единицы (AC-012 «у существующих показывается как прежде»).

### Q-005: SSR-маршрутизация мутаций и 409-перерисовка
HTML-формы — только GET/POST. Design: схема POST-маршрутов SSR (`POST /clients/:id`, `POST /clients/:id/delete` и т.п.), как 409 `CLIENT_VERSION_CONFLICT` перерисовывает форму через `formError()` (образец `users/ssr.go`) без потери ввода телефонов; ссылка «Открыть актуальную версию».

### Q-006: Каталог-навигация под фазу 4
Батч 1 В-4д: меню «Каталог» → `/catalog/equipment-types`, вкладки «Услуги»/«Товары» — фаза 4. Design: заложить ли сейчас общий partial вкладок каталога (одна вкладка) или добавить в фазе 4.

### Q-007: Уникальность названия типа с учётом архива
AC-011: уникальность `lower(btrim(name))`. Design: распространяется ли на архивные типы (рекомендация интервью — да, единый unique-индекс) и текст сообщения при совпадении с архивным («тип есть в архиве — восстановите»).

## Источник: AC-строки (2-spec-interview/prd.md) [ac]
#### AC-001: Заведение клиента
#### AC-002: Проверка обязательных полей клиента
#### AC-003: Несколько телефонов
#### AC-004: Список и поиск клиентов
#### AC-005: Правка клиента и конфликт одновременной правки
#### AC-006: Удаление клиента
#### AC-007: Оборудование клиента в карточке
#### AC-008: Заведение и правка единицы оборудования
#### AC-009: Характеристики по шаблону типа
#### AC-010: Удаление единицы оборудования
#### AC-011: Типы оборудования — список и заведение
#### AC-012: Удаление и архив типа оборудования
#### AC-013: Права исполнителя
#### AC-014: Работа на узком экране и в тёмной теме

## Источник: 1-research/output.md [sections]
## Triage
```yaml
scope: multi-endpoint        # clients CRUD + client_phones + equipment_types CRUD + equipment CRUD
tier: M
touches:
  db: true
  fe: true                   # SSR (html/template + vanilla JS) — фронтенд проекта, D30
  infra: false
  external: false
risk: reversible
novelty: standard-crud        # + optimistic locking version+409 — прецедента в коде нет
decisions: []                 # draft-spec отсутствует; решения — D1-D42 docs/plan.md, не DS-NNN
```

## DB Schema (baseline)

### `users` (образец: version-колонка есть, НЕ используется в API — D28 last-write-wins)
```sql
-- migrations/0001_init.up.sql
CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE TABLE users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    login text NOT NULL UNIQUE, password_hash text NOT NULL, name text NOT NULL DEFAULT '',
    is_active boolean NOT NULL DEFAULT true, version bigint NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);
-- migrations/0002_users_roles.up.sql
ALTER TABLE users ADD COLUMN role text NOT NULL DEFAULT 'executor' CHECK (role IN ('admin','executor')), ADD COLUMN phone text NOT NULL DEFAULT '';
UPDATE users SET role = 'admin', login = lower(login);
CREATE UNIQUE INDEX users_login_lower_uq ON users (lower(login));
```
Следующая свободная миграция: `0003_*` (0001/0002 заняты, `migrations/` имеет ровно 2 пары up/down).

### Целевая схема фазы 3 (docs/plan.md:29-34 — ЕЩЁ НЕ РЕАЛИЗОВАНА, факт для Design, не код)
```sql
clients(id, company_name, address, contact_name, contact_position, created_at, updated_at, version)
client_phones(id, client_id → clients, phone, note)                 -- 1..N телефонов
equipment_types(id, name, spec_keys TEXT[], is_active)
equipment(id, client_id, type_id, brand, model, sku, serial, specs JSONB [{k,v}], note)  -- DELETE только если не было в заказе (D26)
```
`task_equipment(task_id, equipment_id, completeness, condition)` — принадлежит фазе 5/6 (`tasks` ещё не существует) — связь «оборудование было в заказе» физически нечем проверить сейчас (см. §Граничные случаи).

`pgcrypto` уже создан миграцией 0001 — `gen_random_uuid()` доступен без повторной установки.

## Frontend (SSR-слой, D30 — Vue отсутствует, это фронтенд проекта)
- Существующие SSR-фичи: `web/ssr/users/templates/pages/list.html`, `web/ssr/users/templates/pages/form.html` (список+PRG-форма), `web/ssr/auth/templates/pages/login.html`, `web/ssr/home/templates/pages/index.html`, общие `web/ssr/_layout/templates/layouts/base.html`, `web/ssr/_layout/templates/layouts/bare.html`, `web/ssr/_layout/templates/pages/status.html`.
- `web/ssr/_layout/templates/layouts/base.html:54-58` — порядок подключения JS: `bootstrap.bundle.min.js`→`theme.js`→`api.js`→`ui-dialog.js`→`combobox.js`.
- `web/ssr/_shared/js/ui-dialog.js:81-84` — `KT.uiDialog/uiPrompt/uiConfirm/uiAlert`, Bootstrap Modal, `data-testid="ui-dialog"`/`ui-dialog-ok`/`ui-dialog-cancel`/`ui-dialog-input` — замена нативных prompt/confirm/alert прототипа (`prototype/index.html:1381-1383`).
- `web/ssr/_shared/js/combobox.js:26-157` — автообогащение ЛЮБОГО `select.form-select` (кроме `[multiple]`/`[data-native]`) в поисковый комбобокс (D6), `data-testid="combobox-dropdown"`, работает внутри открытой Bootstrap-модалки (combobox.js:69-70) — аналог `enhanceSelect` прототипа.
- Нет генерического JS для динамических списков (телефоны 1..N, характеристики N) — прототип использует ручной JS `drawPhones()`/`drawSpecs()` (`prototype/index.html:1352-1361`, `prototype/index.html:1413-1421`); в SSR-стеке проекта такого паттерна ещё нет — писать заново (chistый JS по образцу прототипа или AJAX-фрагмент `r.Fragment`).
- Переиспользуемый образец карточки/PRG-формы — `internal/features/users/ssr.go:1` целиком; общая 403/404-страница — `web/ssr/_layout/templates/pages/status.html:1`.
- Нормативных Vue-компонентов (`components/ui/`, StatusBadge и т.п.) в проекте НЕТ — не тот стек (SSR/Bootstrap, не Vue+Element Plus).
- Стек: vendored Bootstrap 5.3 (`web/ssr/_shared/vendor/bootstrap/bootstrap.min.css:1`, `web/ssr/_shared/vendor/bootstrap/bootstrap.bundle.min.js:1`), без CDN/Vue/React (D38); токены — `web/ssr/_shared/app.css:1` (primary `#2563EB`); тёмная тема — `data-bs-theme` через `web/ssr/_shared/js/theme.js:1` (US-1 `themes:both`).

### Расхождение с `/z-scaffold` (~/.claude/skills/z-scaffold/SKILL.md — вход для решения scaffold vs вручную)
1. z-scaffold штампует СВОЙ `layouts/base.html` на фичу → конфликт с общим `web/ssr/_layout/templates/layouts/base.html` (два `{{define "base"}}`) — обязателен к удалению (рецепт `internal/ssr/render.go:13-19`).
2. Сгенерированный `BaseVM` — БЕЗ поля `Shell` — добавляется руками в `models/dto/view_model.go:1` (z-scaffold-путь), иначе общий layout не отрендерит сайдбар/тему/CSRF.
3. z-scaffold разбивает на пакеты `handler/`(по файлу на действие)+`mappers/helpers.go:1`+`validators/form.go:1` — РАСХОДИТСЯ с фактическим паттерном: `internal/features/users` — один плоский пакет (`dto.go`/`handler.go`/`service.go`/`repo.go`/`ssr.go`/`validate.go`/`routes.go`), без под-пакетов.
4. Путь `web/ssr/{feature}/templates` — СОВПАДАЕТ (`internal/features/users/ssr.go:14`) — единственный несовпадающий с расхождением пункт.
5. z-scaffold заворачивает ошибки сервиса (кроме валидации формы) в `fiber.ErrInternalServerError`/`ErrNotFound` — расходится с `formError()` (users/ssr.go:136-156), которая через `errors.As` вытаскивает произвольные бизнес-коды (409 и т.п.) — доработка руками, если используется scaffold.
6. Пагинация z-scaffold — классическая нумерованная; фактический паттерн — простой счётчик `Total`/`Items` без номеров страниц (`internal/features/users/ssr.go:29-37`, `web/ssr/users/templates/pages/list.html:41`).
7. Динамические JSONB-характеристики оборудования по внешнему шаблону (`equipment_types.spec_keys`) — вне штатных типов полей манифеста z-scaffold — ручная доработка формы в любом случае.

**Для Design:** зафиксировать явный выбор — плоский пакет (как users) или разбитые z-scaffold-пакеты; в обоих случаях 3 шага D38-рецепта обязательны, а JSONB-характеристики дорабатываются руками.

### Contracts
- Защищённые: весь `/api/v1/users*` (RequireAPI+RequireAdmin), SSR `/users*` (RequireSSR+RequireAdmin) — по аналогии `/clients`/`/catalog` тоже `AdminOnly:true` в `internal/ssr/menu.go:25-26`.
- Публичные: `/health`, `/ready`, `/metrics`, `/login`, `/assets/ssr-shared/*`.
- CSRF — на все мутации всех маршрутов включая `/api/v1` (`internal/server/middleware/csrf.go:30-63` — глобально).
- `.env` — новых переменных фаза 3 не требует (DATABASE_URL/REDIS_ADDR/PORT уже покрывают).
- Rate limiting — только login (D41), вне зоны фичи.
- Не-REST протоколы — отсутствуют.

**Enum coverage:** US-1 `swagger:no` → §API-1/§8.1 n/a, swag-аннотации не пишутся. DB CHECK-образец: `users.role CHECK (role IN ('admin','executor'))` (migrations/0002_users_roles.up.sql:4). Целевая схема фазы 3 (clients/client_phones/equipment_types/equipment) enum-полей НЕ содержит — только text/boolean `is_active`/`TEXT[]`/`JSONB`.

### Error responses (образец формата)
```json
{"code":"VALIDATION","message":"validation failed","details":{"fields":[{"name":"login","message":"required"}]}}
{"code":"LOGIN_TAKEN","message":"Логин уже занят"}
```
Источник: `pkg/errorspkg/errors.go:46-76`. 401/403/404/500 — тот же `{code,message}` без `details`.

### Infrastructure
- Docker Compose — `postgres:18`+`redis:8` уже подняты фазой 1 (не читалось построчно — `touches.infra=false`); фаза 3 новых сервисов не добавляет.
- CI/CD — не обнаружено ни одного файла CI в `git ls-files` (нет `.gitlab-ci.yml`/`.github/workflows`).
- Dockerfile — не обнаружен в `git ls-files`.
- Внешние сервисы — Redis (сессии, `internal/session/store.go:9`), Postgres 18; новых не вводит.
- Secrets — `.env.example`, локальная разработка (CM-1=MVP, прода нет).
- Мониторинг/логирование — `internal/observability/logger.go:9` (slog `NewLogger`), `internal/observability/metrics.go:25` (Prometheus `/metrics`, `internal/server/router.go:58`), `internal/observability/otel.go:18` (`InitTracer`, `cmd/server/main.go:38`) — фаза 3 новых метрик не требует.

### Tests
- Unit: `internal/features/users/validate_test.go:1`, `internal/server/middleware/csrf_test.go:1`, `internal/ssr/render_test.go:1` — обычный `go test`.
- Integration (§TD-8, тег `integration`): `pkg/testutil/base_suite.go:22` — `BaseSuite` (dockertest PG18+Redis, миграции применены, `SeedUser`); `pkg/testutil/live_suite.go:20` — `LiveSuite` (боевой бинарь `go build ./cmd/server`, реальный TCP, `/health`-опрос); `pkg/testutil/login.go:30` — `Browser` (свой cookie jar, редиректы не следуются, `CSRFToken()`/`Request()`/`Do()`/`Login()` через реальный `/api/v1/auth/login`, auth-стабы запрещены).
  - Образец: `internal/features/users/users_http_live_test.go:20-27` — `UsersLiveSuite{LiveSuite{BinaryPkg:"./cmd/server"}}`, покрывает happy-path/authz/валидацию-мусор→400/дубликат→409/конкурентность(`LastAdminSuite`, гонка на `FOR UPDATE`)/SSR PRG.
  - `internal/features/auth/auth_http_live_test.go:1`, `internal/server/shell_http_live_test.go:1` — тот же паттерн.
  - `app.Test` НЕ используется нигде в существующих тестах — соответствует запрету §TD-8 на новые `app.Test`-сьюты.
- Vue-тестов нет (Vue отсутствует). E2E — Playwright:
  - `e2e/playwright.config.ts:7-22` — `projects:[chromium, devices['Desktop Chrome']]`, `workers:1`, `fullyParallel:false`, `PWSLOWMO` env (§TD-3/§TD-5 соблюдены).
  - `e2e/fixtures/test.ts:24-92` — `allowHttpErrors` (auto, ловит console.error/pageerror/HTTP≥400, §TD-4), `uniq`, `makeUser`, `loginAs`, `adminPage`, `runAdmin`/`adminApi` (worker-scope).
  - `e2e/fixtures/auth.ts:32-82` — `loginContext()` (реальный вход, csrf из cookie `kt_csrf`), `AdminApi` (сидирование данных), `SEED_ADMIN={login:'admin',password:'admin1234'}`, `RUN` — метка прогона (не литерал в ассертах, §TD-6.2).
  - `e2e/pages/ShellPage.ts:5-75` — POM общего layout (sidebar/theme/dialog/combobox, §TD-6.1); `e2e/pages/UsersPage.ts:4-77` — образец экрана: полноценные страницы `/users/new`,`/users/:id` (PRG), НЕ модалки — расходится с прототипом, где карточка клиента/оборудования — Bootstrap-модалка.
  - `e2e/tests/users.spec.ts:8-233` — образец глубины TD-6: happy path (US-001), exhaustive-валидация (US-002, TD-6.5), занятый логин (US-003), правка (US-004/005), блокировка+фильтры-в-URL (US-010..013), конкурентность last-write-wins (US-020, D28-для-users), адаптив 360/1280px без overflow (US-021, `adaptive:min`).
- Coverage — `coverage.out`/threshold не обнаружены.

## Паттерны в коде
- Один плоский Go-пакет на фичу (`internal/features/{feature}`), НЕ разбит на handler/mappers/validators (расходится с z-scaffold SSR).
- Sentinel `*errorspkg.AppError` пакетными переменными, маппинг pg-ошибок→доменные в репозитории, матчинг в сервисе `errors.Is`.
- Ручная field-валидация кодами сообщений (`msgRequired` и т.п.), не текстом — SSR транслирует код в текст.
- SSR VM обязательно несёт `Shell ssr.Shell`, заполняется `shell.Build(c,title)` перед рендером (D38).
- PRG для форм: `POST /{feature}` → `303 See Other` → `?flash=created`; ошибка формы — тот же метод, соответствующий HTTP-код, форма перерисовывается с `vm.Errors`.
- Whitelist для пользовательского ввода, идущего в SQL как идентификатор (сортировка/фильтр-роль) — тихий откат к дефолту, не 500.
- `escapeLike()` — экранирование ILIKE-паттернов при поиске по подстроке.
- Права читаются из БД на каждый запрос, не кэшируются в сессии.
- Все HTTP-интеграционные тесты — через `testutil.LiveSuite`, НЕ `app.Test` (§TD-8).
- E2E — POM (`e2e/pages/*Page.ts`) + `data-testid` на все интерактивные элементы; `ShellPage` переиспользуется каждой фичей.
- UUID/ID: единой Go-точки генерации (`pkg/ids`) НЕТ — генерация только в БД `DEFAULT gen_random_uuid()`; Go лишь валидирует уже присвоенный uuid (`uuid.Validate`, users/service.go:94,137). Новые таблицы следуют тому же паттерну.

## Паспорт (выжимка)
# Passport: phase-3-clients (generated — руками не править; stage_bundle.py)
**Tier:** M | **Совместимость:** ? (pipeline-doctrine.md §CM-1; ? → уточни у оркестратора)
**Трек:** обычный (prefs нет — дефолт) — режимы агентов (`spec-track delta/micro/mini/merged`) включает ТОЛЬКО строка `Режим:` в prompt оркестратора; DS/TS-индексы ниже присутствуют на ЛЮБОМ треке при наличии draft-spec.
**Layout:** автодетект (tools/layout_resolve.py)
**UI-scope (pipeline-doctrine.md §US-1, 0-setup/ui-scope-prefs.md):** swagger:no | adaptive:min | themes:both

**Индексы:** AC 14 / DS 0 / TS 0 — полные списки: `0-setup/passport.md` (разрешён контрактом); конкретное решение — точечный grep по `DS-NNN`/`TS-NNN` в draft-spec §9/§11 (разрешён).
