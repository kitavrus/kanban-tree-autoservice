---
stage: 3-design
feature: phase-3-clients
tier: M
---
# Design Tests: phase-3-clients

Доктрина: TD-1 (послойно, fix-before-next; E2E на capability), TD-4 (adversarial), TD-6 (POM, глубина форм), TD-8 (HTTP-integration только `testutil.LiveSuite`/`Browser`, `app.Test` запрещён). US-1: adaptive:min (нет overflow ~360px + 1280px), themes:both. E2E — `e2e/`, `--project=chromium --headed --workers=1` (TD-3), запускает оркестратор.

ID: `UT-NNN` unit · `LV-NNN` live (Go, тег `integration`) · E2E: `CL-NNN` клиенты, `EQ-NNN` оборудование, `ET-NNN` типы (заголовок Playwright-теста = `test('CL-001: …')`).

## Матрица AC → тесты
| AC | Unit | Live | E2E |
|---|---|---|---|
| AC-001 | UT-001, UT-003 | LV-001, LV-021 | CL-001, CL-011 |
| AC-002 | UT-002, UT-004 | LV-002, LV-036 | CL-002, CL-003 |
| AC-003 | UT-003 | LV-001, LV-003 | CL-004 |
| AC-004 | UT-010 | LV-004, LV-035 | CL-005, CL-006 |
| (ADR-006 меню) | UT-011 | — | CL-010 |
| AC-005 | — | LV-005, LV-006, LV-008, LV-020, LV-021 | CL-007, CL-020 |
| AC-006 | — | LV-007, LV-022 | CL-008 |
| AC-007 | — | LV-010 | CL-009 |
| AC-008 | UT-005, UT-006 | LV-011, LV-012, LV-013 | EQ-001, EQ-002, EQ-003 |
| AC-009 | UT-007 | LV-014 | EQ-004, EQ-005 |
| AC-010 | — | LV-015 | EQ-006 |
| AC-011 | UT-008 | LV-030, LV-031, LV-035 | ET-001, ET-002 |
| AC-012 | UT-009 | LV-032, LV-033, LV-034, LV-013 | ET-003, ET-004, EQ-007 |
| AC-013 | — | LV-040, LV-041 | CL-010 |
| AC-014 | — | — | CL-021, EQ-021, ET-021 |

## Unit
`internal/features/clients/validate_test.go`, `internal/features/equipmenttypes/validate_test.go`, `pkg/sqlutil/like_test.go`, `internal/ssr/shell_test.go` (table-driven, эталон `users/validate_test.go`).
| ID | Что |
|---|---|
| UT-001 | ClientInput: trim; `company_name` пусто → required, 200/201 rune (кириллица) → ok/too_long; address 500/501; contact_* 200/201 |
| UT-002 | phones: 0 после отброса → `phones` required; 20 ok / 21 too_many |
| UT-003 | строки телефонов: пустые (phone+note) отбрасываются; note без phone → `phones[i].phone` required; индекс после нормализации |
| UT-004 | phone: 32/33 → too_long; «abc» / «()-» → invalid_format; «+7 (999) 1» ok; note 200/201 |
| UT-005 | EquipmentInput: type_id пусто → required, «xyz» → invalid; model required/200/201; brand/sku/serial 200/201; note 1000/1001 |
| UT-006 | specs `nil` → `[]` (не null в JSON) |
| UT-007 | specs: пустые пары отбрасываются; v без k → `specs[i].k` key_required; k 100/101, v 500/501; 50/51 → too_many; дубли k разрешены |
| UT-008 | types Input: name required/100/101; spec_keys trim, пустые/дубли case-insensitive отброшены, порядок первого вхождения; 50/51 после дедупа |
| UT-009 | SSR-разбор `spec_keys` «a, b,,A » → `["a","b"]` |
| UT-010 | digits-нормализация поиска (ADR-003): «8 (999) 12» → `%899912%`; «ab1» → `''`; «12» (<3 цифр) → `''`; `sqlutil.EscapeLike` `%`/`_`/`\` |
| UT-011 | `Shell.NavActive`: `/` точно; `/clients` на `/clients/abc` true; `/users` на `/users-x` false; `/catalog` на `/catalog/equipment-types` true |

## Live (TD-8)
Файлы: `internal/features/clients/clients_http_live_test.go` (`ClientsLiveSuite{testutil.LiveSuite{BinaryPkg: "./cmd/server"}}`), `internal/features/equipmenttypes/equipmenttypes_http_live_test.go` (`TypesLiveSuite`). `SetupSuite`: `SeedUser` admin + executor, `NewBrowser().Login(...)`; тип/клиенты сидятся через API в тесте (уникальные имена). `make test-integration-scoped`.
| ID | Сценарий | Ожидание |
|---|---|---|
| LV-001 | POST /clients (2 телефона + пустая строка) → GET | 201; `phones` 2 в порядке ввода; `version`=1; `equipment_count`=0 |
| LV-002 | POST мусор: `{}`, `null`, неизвестное поле, `company_name` 201 rune, `phones: []` | 400 `BAD_REQUEST`/`VALIDATION` с полями; не 500 |
| LV-003 | PUT с другим набором телефонов (замена, удаление, перестановка) | 200; набор = присланный, `version`=2 |
| LV-004 | GET /clients?q=: часть компании (регистр), ФИО, телефон «8999…» против «+7 (999)…», `%`/`_` литерально; клиент с 3 совпавшими телефонами | каждый найден 1 раз; `total` без дублей; сортировка `lower(company_name)`; `limit`/`offset`/мусорный `limit` → дефолт |
| LV-005 | PUT без `version` / `"version":"x"` | 400 (`version` required / `BAD_REQUEST`) |
| LV-006 | PUT со старым `version` | 409 `CLIENT_VERSION_CONFLICT`; данные и телефоны не изменены |
| LV-007 | DELETE клиента с 2 единицами → GET клиента, GET /equipment/:id; повторный DELETE | 204; 404/404; повтор 404 |
| LV-008 | битый uuid / несуществующий id: GET/PUT/DELETE /clients/:id, /equipment/:id, /clients/:id/equipment | 404, не 500 |
| LV-010 | GET /clients/:id/equipment: 0 и 2 единицы | конверт `{items,total,limit,offset}`; `type_name` |
| LV-011 | POST /clients/:id/equipment happy → GET /clients/:id | 201; `equipment_count`=1; `clients.version` не изменился |
| LV-012 | POST без type_id / несуществующий type_id / без model | 400 поля `type_id` required/not_found, `model` required |
| LV-013 | архивный тип: POST с ним → 400 `archived`; PUT существующей единицы без смены типа → 200; PUT со сменой на архивный → 400 | как указано; GET единицы `type_is_active`=false |
| LV-014 | specs: пустые пары отброшены, v без k → 400, смена `spec_keys` типа не меняет `specs` единицы | ✓ |
| LV-015 | DELETE /equipment/:id → GET; повтор | 204 → 404; повтор 404 |
| LV-020 | конкурентность (TD-4): 2 Browser admin, одновременно PUT с одним `version` (goroutine + `sync.WaitGroup`) | ровно один 200, один 409; итог = данные победителя, `version`=2, телефоны не смешаны |
| LV-021 | SSR PRG: POST /clients (form, `_csrf`, `phone` ×2) → 303 `/clients/{id}?flash=created`; POST /clients/:id со старым version → 409 HTML содержит текст AC-005 и введённые телефоны | ✓ |
| LV-022 | SSR POST /clients/:id/delete, /equipment/:id/delete → 303; без `_csrf` → 403 | ✓ |
| LV-030 | POST/PUT /equipment-types happy; spec_keys нормализованы | 201/200 |
| LV-031 | дубль имени « Лазер » vs «лазер» (POST и PUT-переименование); дубль архивного | 409 `EQUIPMENT_TYPE_NAME_TAKEN`; для архивного — 409 `EQUIPMENT_TYPE_NAME_ARCHIVED` (ветвление по `code`) |
| LV-032 | DELETE неиспользуемого → 204; используемого → 409 `EQUIPMENT_TYPE_IN_USE`, тип цел | ✓ |
| LV-033 | PUT `is_active:false` → GET list без `include_archived` не содержит, с `include_archived=true` (admin) содержит; PUT без `is_active` → 400 | ✓ |
| LV-035 | пагинация (M-02): `/equipment-types` без `limit` → `limit`=500, 60 типов все в `items`; `limit=9999` → 500; `limit=abc`/`-1` → 500; `offset=-5` → 0. `/clients` без `limit` → 50, `limit=9999` → 200. `/clients/:id/equipment` → 200 | ✓ |
| LV-036 | мусорный body новых хендлеров → 400 `BAD_REQUEST`, message «Некорректное тело запроса» | ✓ |
| LV-034 | SSR POST /catalog/equipment-types/:id/archive|restore → 303; delete используемого → 409 HTML с кнопкой «В архив» | ✓ |
| LV-040 | executor API: GET clients/equipment/types — 200; `include_archived=true` → только активные; POST /clients/:id/equipment → 201; POST/PUT/DELETE clients, PUT/DELETE equipment, POST/PUT/DELETE types → 403 и данные не изменены | ✓ |
| LV-041 | executor SSR: /clients, /clients/:id, /equipment/:id, /catalog/equipment-types → 403 «Нет доступа»; без сессии API → 401, SSR → редирект на /login | ✓ |

## E2E (Playwright)
Файлы: `e2e/tests/clients.spec.ts`, `e2e/tests/equipment.spec.ts`, `e2e/tests/equipment-types.spec.ts`.
POM: `e2e/pages/ClientsPage.ts` (list/form/conflict/delete), `e2e/pages/EquipmentPage.ts` (форма единицы, specs-строки), `e2e/pages/EquipmentTypesPage.ts`; переиспользуют `ShellPage` (dialog `ui-dialog-ok`, combobox `combobox-dropdown`).
Фикстуры: `e2e/fixtures/auth.ts` `AdminApi` + `createClient(c)`, `updateClient(id, c)`, `getClient(id)`, `createEquipmentType(t)`, `updateEquipmentType(id, t)`, `createEquipment(clientId, e)`; типы payload — wire-типы (TD-6.2, `e2e/tsconfig.json`); имена — `uniq()`; executor — `makeUser({role:'executor'})` + `loginAs`.
| ID | Сценарий |
|---|---|
| CL-001 | создание клиента (все поля, 2 телефона + пустая строка) → PRG → карточка с flash; в списке компания и оба телефона с примечаниями |
| CL-002 | exhaustive валидация (TD-6.5): пустая компания; все телефоны пустые; 201 символ компании; телефон «abc»; телефон 33 символа — сообщение у поля, остальной ввод сохранён |
| CL-003 | исправление ошибки после 422 → сохранение проходит |
| CL-004 | добавить/удалить строки телефонов (`phone-add`/`phone-del`), последняя строка не удаляется; порядок сохранения |
| CL-005 | поиск по части компании / ФИО / телефону с другими разделителями; URL `?q=`; reload и «назад» сохраняют; «Клиенты не найдены»; «Сбросить» |
| CL-006 | список: сортировка по компании, колонка «Оборудование» = N (seed через API) |
| CL-007 | правка карточки → flash «Изменения сохранены», значения обновлены |
| CL-008 | удаление: confirm содержит «N единиц оборудования» → отмена ничего не меняет → ok → список без клиента |
| CL-009 | блок «Оборудование клиента»: пусто «Оборудования пока нет»; с единицей — модель, тип, SN; архивный тип с пометкой «(в архиве)» |
| CL-010 | executor: пунктов «Клиенты»/«Каталог» нет в меню; прямые URL `/clients`, `/catalog/equipment-types`, `/equipment/:id` → «Нет доступа» |
| CL-011 | XSS: `<img src=x onerror=…>` в компании/телефоне/характеристике отображается текстом (список, карточка, confirm) — образец фазы 1 |
| CL-020 | конкурентность (TD-4): 2 browser-контекста открывают карточку; A сохраняет; B сохраняет → текст AC-005, ввод B сохранён (вкл. телефоны); «Открыть актуальную версию» → значения A; B сохраняет повторно → успех |
| CL-021 | adaptive:min + themes: `/clients` и карточка на 360×640 и 1280×800 — `scrollWidth <= clientWidth`; тёмная тема — страница рендерится без overflow |
| EQ-001 | новая единица из карточки: тип через combobox-поиск, модель, SN → PRG в карточку, единица в блоке |
| EQ-002 | валидация: без типа / без модели / 201 символ модели — сообщения у полей, ввод сохранён |
| EQ-003 | правка единицы → карточка клиента с flash; значения обновлены |
| EQ-004 | выбор типа подставляет строки шаблона; смена типа: заполненные сохраняются, пустые заменены шаблоном нового |
| EQ-005 | своя характеристика вне шаблона, удаление строки, «v без k» → «Укажите характеристику»; пустые строки не сохраняются |
| EQ-006 | удаление единицы через confirm → исчезает из карточки |
| EQ-007 | архивный тип не предлагается для новой единицы; у существующей показан «(в архиве)» и сохраняется без смены |
| EQ-021 | adaptive:min: форма единицы 360/1280 без overflow; строки характеристик переносятся |
| ET-001 | создание типа (name + keys через запятую с дублями/пробелами) → список: название, ключи, «0» единиц |
| ET-002 | дубль названия (регистр/пробелы) → «Такой тип уже есть»; дубль архивного → «…в архиве — верните его из архива» + ссылка «Показать архивные» |
| ET-003 | удаление неиспользуемого (confirm) → исчез; используемого → сообщение + «В архив» → в архиве |
| ET-004 | «Показать архивные» (URL `?archived=1`, reload сохраняет); «Вернуть» → тип снова активен |
| ET-021 | adaptive:min: `/catalog/equipment-types` 360/1280 без overflow |

## Mock Strategy
Моков нет: repository/service — через живую БД (BaseSuite/LiveSuite, dockertest PG18 + Redis); внешних клиентов нет.

## Fuzz (TD-7)
Класс 9: n/a — tier M, `risk=reversible`, по запросу не заказан. `pw-fuzz` — вердикт frontend-architect (не пересуживается здесь).

## Test Data / Helpers
- Go: хелперы в live-сьюте — `createType(name, keys)`, `createClient(company, phones...)`, `createEquipment(clientID, typeID, model)` через `Browser.Do`.
- Сид миграции/фикстуры: схема 0003 + фикстуры `AdminApi` — в той же фазе (TD-1, `fixture_sync_check.py`).
- Не тестируем: D26 (фаза 5), нагрузку, миграцию `specs` при смене шаблона (по дизайну не мигрирует — проверяется LV-014).
