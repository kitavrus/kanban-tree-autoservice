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
