# Input Bundle: phase-3-clients / стадия 4-swimlane — generated 2026-09-25 14:46

> Твой ЕДИНСТВЕННЫЙ документный вход (пайп-контракт). Upstream-доки напрямую НЕ читать;
> точечный grep по конкретному ID (DS-NNN/AC-NNN/TS-NNN) — разрешён. Отсутствие нужного
> факта в бандле = сигнал оркестратору (дырка контракта), не повод читать всё.

## Источник: 3-design/output.md [full]
---
stage: 3-design
feature: phase-3-clients
tier: M
---
# Design: Клиенты и оборудование (фаза 3)
**Дата:** 2026-09-25
**Tier:** M | **Совместимость:** MVP (CM-1) | **UI-scope:** swagger:no · adaptive:min · themes:both
**На основе:** `3-design/input-bundle.md` (Tech Spec + AC-001…AC-014 + research-экстракты)

```yaml
diagrams:
  dataflow: no
  sequence: no
  swimlane: yes
```

## Обзор
- Клиенты (телефоны 1..N, `version`-блокировка), типы оборудования (шаблон `spec_keys`, архив), реестр оборудования клиента (`specs` JSONB `[{k,v}]`).
- Две плоские Go-фичи по эталону `internal/features/users`: `internal/features/clients` (clients + client_phones + equipment), `internal/features/equipmenttypes` (equipment_types). z-scaffold не используется (Tech Spec, отступление от D30).
- SSR: Go html/template + vanilla JS + Bootstrap 5.3 (D38), общий layout, PRG; JSON API `/api/v1/...` (API-1).
- Миграция `0003_clients_equipment`; новых env/сервисов/интеграций нет.

**Triage:** `scope: multi-endpoint | tier: M | touches: {db: true, fe: true, infra: false, external: false} | risk: reversible | novelty: standard-crud | decisions: []` (источник: 1-research §Triage).

## Детальные документы (owner-файлы)
- `3-design/design-sql.md` — миграция 0003 (DDL up/down), все SQL-запросы.
- `3-design/design-errors.md` — sentinel-ошибки, коды полей, маппинг API/SSR.
- `3-design/design-go-layers.md` — DTO, Input/нормализация/валидация, Service/Repo, Handler/SSR, routes, VM, шаблоны, JS, wiring.
- `3-design/design-tests.md` — матрица AC → unit/live/E2E, POM/фикстуры, конкурентность.

Инфраструктура: изменений не требуется (новых env vars/сервисов/CI нет; `make migrate-up` применяет 0003).

## Контракт
Конвенции: API-1 (kebab URL, snake_case JSON, списки `{items,total,limit,offset}`); FE-1-wire: `version` в body. Ошибки — `pkg/errorspkg` `{code,message,details?}`. DTO — `design-go-layers.md`.

### API `/api/v1`
| Метод | Путь | Гард | Request | Response | Driver |
|---|---|---|---|---|---|
| GET | `/clients?q=&limit=&offset=` | RequireAPI | — | `ClientListResponse` 200 | AC-004 |
| POST | `/clients` | RequireAPI+RequireAdmin | `ClientCreateRequest` | `ClientResponse` 201 | AC-001, AC-002, AC-003 |
| GET | `/clients/:id` | RequireAPI | — | `ClientResponse` 200 | AC-005 |
| PUT | `/clients/:id` | RequireAPI+RequireAdmin | `ClientUpdateRequest` (+`version`) | `ClientResponse` 200 / 409 `CLIENT_VERSION_CONFLICT` | AC-005 |
| DELETE | `/clients/:id` | RequireAPI+RequireAdmin | — | 204 | AC-006 |
| GET | `/clients/:id/equipment?limit=&offset=` | RequireAPI | — | `EquipmentListResponse` 200 | AC-007 |
| POST | `/clients/:id/equipment` | RequireAPI (admin+executor, D19) | `EquipmentRequest` | `EquipmentResponse` 201 | AC-008, AC-013 |
| GET | `/equipment/:id` | RequireAPI | — | `EquipmentResponse` 200 | AC-008 |
| PUT | `/equipment/:id` | RequireAPI+RequireAdmin | `EquipmentRequest` | `EquipmentResponse` 200 | AC-008, AC-009 |
| DELETE | `/equipment/:id` | RequireAPI+RequireAdmin | — | 204 | AC-010 |
| GET | `/equipment-types?include_archived=&q=&limit=&offset=` | RequireAPI (`include_archived` — только admin, ADR-004) | — | `TypeListResponse` 200 | AC-011, AC-013 |
| POST | `/equipment-types` | RequireAPI+RequireAdmin | `TypeCreateRequest` | `TypeResponse` 201 / 409 `EQUIPMENT_TYPE_NAME_TAKEN` | AC-011 |
| GET | `/equipment-types/:id` | RequireAPI | — | `TypeResponse` 200 | AC-011 |
| PUT | `/equipment-types/:id` | RequireAPI+RequireAdmin | `TypeUpdateRequest` (+`is_active`) | `TypeResponse` 200 | AC-011, AC-012 |
| DELETE | `/equipment-types/:id` | RequireAPI+RequireAdmin | — | 204 / 409 `EQUIPMENT_TYPE_IN_USE` | AC-012 |

### SSR (все: RequireSSR + RequireAdmin(status.Forbidden); AC-013)
| Метод | Путь | Действие | Успех (PRG 303) |
|---|---|---|---|
| GET | `/clients?q=` | список | — |
| GET | `/clients/new` | форма создания | — |
| POST | `/clients` | создать | `/clients/{id}?flash=created` |
| GET | `/clients/:id` | карточка (форма + блок оборудования) | — |
| POST | `/clients/:id` | сохранить (hidden `version`) | `/clients/{id}?flash=saved` |
| POST | `/clients/:id/delete` | удалить (confirm, ADR-104) | `/clients?flash=deleted` |
| GET | `/clients/:id/equipment/new` | форма новой единицы | — |
| POST | `/clients/:id/equipment` | создать единицу | `/clients/{client_id}?flash=eq_created` |
| GET | `/equipment/:id` | форма единицы | — |
| POST | `/equipment/:id` | сохранить | `/clients/{client_id}?flash=eq_saved` |
| POST | `/equipment/:id/delete` | удалить (confirm) | `/clients/{client_id}?flash=eq_deleted` |
| GET | `/catalog` | 303 → `/catalog/equipment-types` | — |
| GET | `/catalog/equipment-types?archived=1` | список типов | — |
| GET | `/catalog/equipment-types/new` | форма | — |
| POST | `/catalog/equipment-types` | создать | `/catalog/equipment-types?flash=created` |
| GET | `/catalog/equipment-types/:id` | форма | — |
| POST | `/catalog/equipment-types/:id` | сохранить | `/catalog/equipment-types?flash=saved` |
| POST | `/catalog/equipment-types/:id/delete` | удалить (confirm) | `…?flash=deleted`; 409 IN_USE → форма типа с ошибкой + кнопка «В архив» |
| POST | `/catalog/equipment-types/:id/archive` | в архив | `…?flash=archived` (фильтр сохраняется: `archived` из формы) |
| POST | `/catalog/equipment-types/:id/restore` | вернуть | `…?archived=1&flash=restored` |

Ошибка формы: тот же POST, форма перерисовывается с вводом (400-валидация → 422, 409 → 409, 404 → `status.NotFound`) — паттерн `users/ssr.go formError`.

## ADR

### ADR-001: Демо-данные клиентов — не в фазе 3
- **Статус:** Принято
- **Driver:** PRD §3 — «100% демо-клиентов в реестре к старту фазы 5» (цитата метрики по Q-001 Tech Spec)
- **Отвечает на:** Q-001 — «расширять ли `seed-demo` клиентами/телефонами/типами/оборудованием из прототипа в этой фазе и как»
- **Контекст:** D36 + plan §6 (фаза 2): `seed-demo` — фаза 5; цели `seed-demo` в Makefile нет.
- **Решение:** в фазе 3 `seed-demo` не создаётся; фаза 5 заводит `cmd/seed-demo` (идемпотентно, `ON CONFLICT`/поиск по natural-ключу) с клиентами, телефонами, типами и оборудованием прототипа — метрика «к старту фазы 5» выполняется её первым шагом. Тесты фазы 3 сидят через API (`AdminApi`), не через сид.
- **Альтернативы:** SQL-сид сейчас — отклонено: фаза 5 всё равно переписывает сид под заказы (двойная работа).
- **Риски:** ручная приёмка фазы 3 на пустом реестре → заводить через UI.

### ADR-002: Общий JS-повторитель строк `row-list.js` + шаблоны типов в `data-spec-keys`
- **Статус:** Принято
- **Driver:** AC-003 — «В карточке клиента можно добавлять и удалять строки телефонов (номер + примечание). В списке клиентов видны все телефоны с примечаниями.»; AC-009 — «При выборе типа в форму подставляются строки характеристик из шаблона этого типа, остаётся заполнить значения. При смене типа заполненные значения сохраняются, пустые строки заменяются шаблоном нового типа. Можно добавить свою характеристику вне шаблона и удалить любую строку. Совсем пустые строки не сохраняются. Если шаблон типа потом изменится, у уже заведённых единиц характеристики не меняются.»
- **Отвечает на:** Q-002 — «общий модуль в `web/ssr/_shared/js/` (переиспользуем фазой 4 — комплект товаров услуги) или код в фиче; как передавать шаблоны типов в форму»
- **Решение:** `web/ssr/_shared/js/row-list.js` → `KT.rowList` (контейнер `[data-row-list]`, строка из `<template data-row-template>`, кнопки `[data-row-add]`/`[data-row-del]`, `KT.rowList.fill(container, rows)`), подключается в `{{ define "scripts" }}` страниц. Поля строк — повторяющиеся имена (`phone`/`phone_note`, `spec_k`/`spec_v`) → сервер читает параллельные массивы по индексу, реиндексация имён не нужна. Сервер рендерит начальные строки (SSR работает и с выключенным JS, кроме add/del). Шаблон типа — атрибут `<option data-spec-keys='["…"]'>` (JSON-строка из Go, экранирует html/template); логика смены типа (заполненные сохраняются, пустые заменяются шаблоном — эталон `openEquipment` прототипа) — inline-скрипт в `scripts`-блоке `equipment-form.html`. Опции: активные типы + текущий архивный тип единицы с пометкой «(в архиве)» (ADR-004).
- **Альтернативы:** `r.Fragment`-AJAX за новой строкой — лишний round-trip; JSON-блок `<script type="application/json">` — отделён от опции, combobox.js работает с `<option>` напрямую.
- **Риски:** `combobox.js` должен генерировать `change` на исходном `<select>` — проверить (E2E EQ-004); иначе — слушать событие комбобокса.

### ADR-003: Поиск клиентов — EXISTS по телефонам + цифровая нормализация, без новых индексов
- **Статус:** Принято
- **Driver:** AC-004 — «Поиск одной строкой находит клиента по части названия компании, ФИО представителя или телефона. Поиск остаётся после обновления страницы и кнопки «назад», ссылку с ним можно переслать. Если ничего не найдено, показывается «Клиенты не найдены».»
- **Отвечает на:** Q-003 — «форма запроса (EXISTS vs DISTINCT), нормализация телефона при поиске, нужны ли индексы»
- **Решение:** `EXISTS (SELECT 1 FROM client_phones …)` — строка клиента одна, `total` честный. Телефон: `phone ILIKE %q%` ИЛИ (если в `q` ≥3 цифр и `q` состоит только из `[0-9+()\- ]`) `regexp_replace(phone,'\D','','g') LIKE %digits%` — «8999123» находит «+7 (999) 123-…» частично по цифрам. Индексы только FK/сортировка; `pg_trgm` не вводится (внутренний инструмент, сотни–тысячи клиентов, seq scan < 10 ms).
- **Альтернативы:** `JOIN … DISTINCT` — дубли в count, сложнее пагинация; хранимая нормализованная колонка — лишняя миграция/синхронизация.
- **Риски:** рост до 10^5 клиентов → добавить trgm-индекс (вне MVP).

### ADR-004: Архивные типы — executor видит только активные; единица показывает свой архивный тип
- **Статус:** Принято
- **Driver:** AC-013 — «Исполнитель может получить данные клиентов, их оборудования и активных типов (для заказов в следующих фазах) и завести новую единицу оборудования клиенту.»; AC-012 — «Архивный тип: не предлагается при заведении новой единицы; у существующих единиц показывается как прежде; в списке виден при включённом «Показать архивные»; возвращается из архива одной кнопкой.»
- **Отвечает на:** Q-004 — «разрешён ли executor `include_archived=true`; как форма оборудования показывает текущий архивный тип существующей единицы»
- **Решение:** `include_archived=true` от не-admin молча игнорируется (только активные; не 403 — фильтр, не мутация; паттерн whitelist-отката). `GET /equipment-types/:id` — любому аутентифицированному (архивный тоже: нужен для отображения). `EquipmentResponse` несёт `type_name`, `type_is_active` → UI не нужен список архивных. Форма единицы: `<select>` = активные типы + текущий тип единицы, если архивный (`… (в архиве)`); сервер: смена `type_id` на архивный / создание с архивным → 400 `type_id: archived`; сохранение без смены архивного типа — ок.
- **Альтернативы:** 403 на `include_archived` для executor — ломает общий клиентский код фильтра без пользы.
- **Риски:** нет.

### ADR-005: SSR-мутации — POST-маршруты `…/delete|archive|restore`; 409 version перерисовывает форму
- **Статус:** Принято
- **Driver:** AC-005 — «Если в это время карточку уже сохранил другой пользователь, изменения не записываются. Форма остаётся с введёнными значениями и сообщением «Карточку уже изменил другой пользователь — ваши правки не сохранены», рядом кнопка «Открыть актуальную версию». Ничьи правки не теряются молча.»
- **Отвечает на:** Q-005 — «схема POST-маршрутов SSR, как 409 `CLIENT_VERSION_CONFLICT` перерисовывает форму через `formError()` без потери ввода телефонов; ссылка «Открыть актуальную версию»»
- **Решение:** маршруты — таблица SSR выше. `clients.formError`: `errors.Is(err, ErrClientVersionConflict)` → `vm.Conflict=true`, `vm.Error=`текст AC-005, статус 409; форма рендерится из POST-значений (вкл. массивы телефонов, нормализованные тем же `Input.normalize()`), hidden `version` = присланный (устаревший) → повторный submit снова 409, пока пользователь не откроет актуальную версию. Кнопка «Открыть актуальную версию» = `<a href="/clients/{id}" data-testid="client-conflict-reload">` (GET перечитывает карточку). Удалённый параллельно клиент → 404 `status.NotFound`.
- **Альтернативы:** merge-диалог — вне MVP; редирект с flash — теряет ввод (нарушает AC-005).
- **Риски:** нет.

### ADR-006: Каталог — без partial вкладок сейчас; пункт меню «Каталог» → `/catalog` (303), подсветка по префиксу
- **Статус:** Принято
- **Driver:** AC-011 — «В разделе «Каталог» → «Типы оборудования» администратор видит список: название, характеристики по умолчанию, число единиц у клиентов.»; AC-013 — «Исполнитель не видит в меню «Клиенты» и «Каталог».»
- **Отвечает на:** Q-006 — «заложить ли сейчас общий partial вкладок каталога (одна вкладка) или добавить в фазе 4»
- **Решение:** partial вкладок — фаза 4 (одна вкладка = мёртвый UI). `internal/ssr/menu.go`: пункт `{Title: "Каталог", URL: "/catalog", AdminOnly: true}` (было «Услуги и товары»); `GET /catalog` → 303 `/catalog/equipment-types` (фаза 4 перенаправит на свою вкладку). `NewShellBuilder("/", "/users", "/clients", "/catalog")`. Подсветка пункта: `Shell.NavActive(url)` — `url=="/"` точное совпадение, иначе `ActivePath==url || HasPrefix(ActivePath, url+"/")`; `base.html` использует его вместо `eq .URL $.Shell.ActivePath` (иначе `/clients/:id` и `/catalog/equipment-types` не подсвечиваются).
- **Альтернативы:** partial сейчас — переделывается фазой 4.
- **Риски:** регресс подсветки `/users`/`/` — покрыт unit-тестом `NavActive` + существующим `shell.spec.ts`.

### ADR-007: Уникальность названия типа — единая, включая архив
- **Статус:** Принято
- **Driver:** AC-011 — «Два типа с одинаковым названием (без учёта регистра и пробелов по краям) завести нельзя, появится сообщение.»
- **Отвечает на:** Q-007 — «распространяется ли на архивные типы и текст сообщения при совпадении с архивным»
- **Решение:** `CREATE UNIQUE INDEX equipment_types_name_lower_uq ON equipment_types (lower(btrim(name)))` — на все строки. Имя нормализуется `strings.TrimSpace` до записи. 23505 → репо дочитывает `is_active` совпавшей строки → `ErrTypeNameTaken` («Такой тип уже есть») или `ErrTypeNameTakenArchived` («Такой тип уже есть в архиве — верните его из архива»); код у обоих `EQUIPMENT_TYPE_NAME_TAKEN`.
- **Альтернативы:** частичный индекс `WHERE is_active` — восстановление из архива ломается о дубль.
- **Риски:** нет.

### ADR-100: Оптимистичная блокировка клиента — `UPDATE … WHERE version=$n` в транзакции
- **Статус:** Принято
- **Driver:** AC-005 — «Ничьи правки не теряются молча.»
- **Решение:** `PUT` требует `version` (нет → 400 `version: required`). Tx: `UPDATE clients SET …, version=version+1, updated_at=now() WHERE id=$1 AND version=$2 RETURNING …`; 0 строк → `SELECT EXISTS` → есть = 409 `CLIENT_VERSION_CONFLICT`, нет = 404; затем replace-all телефонов (ADR-101); commit. Атомарность проверки+инкремента в одном UPDATE — без `FOR UPDATE`. DELETE клиента `version` не проверяет (Tech Spec). Правка оборудования `clients.version` не трогает.
- **Альтернативы:** `If-Match` — отклонено Tech Spec (FE-1: `version` в body); `SELECT … FOR UPDATE` + сравнение — лишний round-trip.
- **Риски:** нет.

### ADR-101: Телефоны — отдельная таблица с `position`, replace-all
- **Статус:** Принято
- **Driver:** AC-001 — «Администратор создаёт клиента: название компании (обязательно), адрес, ФИО и должность представителя, хотя бы один телефон с необязательным примечанием. После сохранения клиент появляется в списке «Клиенты». Пустые строки телефонов не сохраняются.»
- **Решение:** `client_phones(client_id, position, phone, note)`, `UNIQUE(client_id, position)` (порядок ввода сохраняется); запись — `DELETE` + `INSERT … SELECT FROM unnest($2::text[], $3::text[]) WITH ORDINALITY` в транзакции клиента. `id` у телефона наружу не выдаётся (строка — значение, не сущность).
- **Альтернативы:** JSONB в `clients` — не по plan §3 (схема фиксирована), поиск по телефону сложнее; diff-апдейт по id — лишняя сложность без выгоды.
- **Риски:** нет.

### ADR-102: Межфичевые связи — только SQL-чтение, без Go-импорта между фичами
- **Статус:** Принято
- **Driver:** AC-008 — «Пользователь заводит единицу оборудования клиенту: тип (обязательно, выбор с поиском, только не архивные типы), производитель, модель (обязательно), артикул, серийный номер / VIN, характеристики, примечание.»; AC-011 — «…видит список: название, характеристики по умолчанию, число единиц у клиентов.»
- **Решение:** `clients` владеет записью `clients/client_phones/equipment`, `equipmenttypes` — `equipment_types`. `clients.Repo` читает `equipment_types` (опции формы, `is_active` при валидации, `type_name` в ответе); `equipmenttypes.Repo` считает `equipment` (`equipment_count`). DELETE типа — по FK `ON DELETE RESTRICT` (23503 → `ErrTypeInUse`), без pre-count (нет гонки).
- **Альтернативы:** `clients` импортирует `equipmenttypes.Service` — связность пакетов, циклы при фазе 5.
- **Риски:** гонка «тип архивирован между проверкой и INSERT единицы» — допустима (LWW, D28).

### ADR-103: Удаление в SSR — POST-форма + делегированный `form[data-confirm]` → `KT.uiConfirm`
- **Статус:** Принято
- **Driver:** AC-006 — «Администратор удаляет клиента после подтверждения. В подтверждении сказано, сколько единиц оборудования будет удалено вместе с ним. Клиент, его телефоны и оборудование исчезают.»
- **Решение:** кнопки удаления/архива — `<form method="post" action="…/delete" data-confirm="…">` с `_csrf`. `web/ssr/_shared/js/ui-dialog.js` дополняется делегированным `submit`-обработчиком: `form[data-confirm]` → `preventDefault` → `KT.uiConfirm(form.dataset.confirm, form.dataset.confirmOk)` → `form.submit()` (не порождает повторный submit-event). Текст клиента: «Удалить клиента «{company}»? Вместе с ним будет удалено N единиц оборудования.» (N = `equipment_count` из карточки). Результат — PRG + flash; 409 IN_USE типа — перерисовка формы типа с ошибкой (без JS-обработки ответов).
- **Альтернативы:** `fetch` DELETE к API через `KT.api` + `location` — вторая схема ошибок в JS, ломается без JS, дублирует PRG-flash.
- **Риски:** N устаревает, если оборудование добавили параллельно — информационно, каскад всё равно удалит всё.

### Rejected
- DISTINCT-join для поиска (ADR-003); partial вкладок каталога (ADR-006); `version` у equipment/types (D28, LWW); D26-проверки удаления — фаза 5 (Tech Spec).

## Summary ключевых решений
- ADR-001 — seed-demo клиентов в фазе 5 (Принято)
- ADR-002 — `row-list.js` + `data-spec-keys` (Принято)
- ADR-003 — поиск EXISTS + цифровая нормализация, без trgm (Принято)
- ADR-004 — executor: только активные типы; архивный тип виден у своей единицы (Принято)
- ADR-005 — SSR POST-маршруты, 409 → форма с вводом (Принято)
- ADR-006 — «Каталог» → `/catalog` 303, `NavActive` по префиксу (Принято)
- ADR-007 — уникальность имени типа вкл. архив (Принято)
- ADR-100 — `UPDATE … WHERE version` (Принято)
- ADR-101 — `client_phones` с `position`, replace-all (Принято)
- ADR-102 — межфичевые связи только SQL-чтением (Принято)
- ADR-103 — удаление POST-формой + `form[data-confirm]` (Принято)

## Enums и константы
Не применимо — enum-полей нет (text / boolean `is_active` / `text[]` / `jsonb`); swagger:no.

## Security
- Гарды: SSR — `RequireSSR`+`RequireAdmin(status.Forbidden)`; API мутации — `RequireAdmin(nil)` по-маршрутно, кроме `POST /clients/:id/equipment` (D19). CSRF глобальный (без изменений).
- IDOR: несуществующий id / битый uuid → 404 (сервис: `uuid.Validate` → NotFound), не 500. `type_id` битый → 400 поле.
- Вывод — только `html/template`; `data-spec-keys` — JSON из `json.Marshal`, экранируется контекстом атрибута. `data-confirm` выводится через `textContent` (ui-dialog.js).
- В логи значения полей (ФИО/телефоны) не пишутся — только `trace_id`/коды.
- Новых зависимостей (go.mod / vendored JS) нет.

## Monitoring & Alerting Spec
| Span | Log fields | Метрика | Алерт |
|---|---|---|---|
| штатные HTTP-spans `observability.TraceMiddleware("kanban-tree")` | `trace_id`, `method`, `path`, `status` (`RequestLogger`) | штатные HTTP-метрики `/metrics` | deferred — MVP, прода нет (CM-1); алерты заводятся инфраструктурной фазой (plan §6); issue-трекера в проекте нет |
Новых ручных spans/метрик нет (Tech Spec §Операции).

## Test Strategy
Owner — `3-design/design-tests.md` (матрица AC-001…AC-014 → unit/live/E2E, concurrency CL-020 по TD-4, adaptive:min, themes:both). Fuzz (TD-7): n/a — M, `risk=reversible`, не заказан.

## Runbook (post-deploy)
Не требуется — MVP, прода нет (Tech Spec §Операции). Локально: `make migrate-up` → `make run` → смок `/clients`, `/catalog/equipment-types`.

## Performance Budget
Не применимо — не hot-path, `risk=reversible`, MVP (Tech Spec §Performance). Список клиентов: 2 запроса + count, без N+1 (design-sql.md `## List Clients Query`).

## Rollout Strategy
Не применимо — обычный merge, feature flag не нужен (Tech Spec). Down-миграция — простой DROP (CM-1 MVP).

## Plan hints (для planner)
1. Миграция 0003 + `equipmenttypes` (repo/service/handler/validate/ssr/routes) + unit + live.
2. `clients` API (clients + phones + version) + unit + live (вкл. конкурентность).
3. `clients` equipment API + live.
4. Shared: `NavActive`, menu, `row-list.js`, `ui-dialog.js` `form[data-confirm]`.
5. SSR clients/equipment/types + E2E по capability (TD-1).

## Отклонения от оригинальной задачи
- Меню: пункт «Услуги и товары» переименован в «Каталог» (AC-011/AC-013 называют раздел «Каталог»).
- Дырка контракта (оркестратору): бандл содержит только заголовки AC — тексты для цитат Driver взяты точечным grep по AC-NNN из `2-spec-interview/prd.md`; бандлеру стоит включать тела AC.

## Паспорт (выжимка)
# Passport: phase-3-clients (generated — руками не править; stage_bundle.py)
**Tier:** M | **Совместимость:** ? (pipeline-doctrine.md §CM-1; ? → уточни у оркестратора)
**Трек:** обычный (prefs нет — дефолт) — режимы агентов (`spec-track delta/micro/mini/merged`) включает ТОЛЬКО строка `Режим:` в prompt оркестратора; DS/TS-индексы ниже присутствуют на ЛЮБОМ треке при наличии draft-spec.
**Layout:** автодетект (tools/layout_resolve.py)
**UI-scope (pipeline-doctrine.md §US-1, 0-setup/ui-scope-prefs.md):** swagger:no | adaptive:min | themes:both

**Индексы:** AC 14 / DS 0 / TS 0 — полные списки: `0-setup/passport.md` (разрешён контрактом); конкретное решение — точечный grep по `DS-NNN`/`TS-NNN` в draft-spec §9/§11 (разрешён).
