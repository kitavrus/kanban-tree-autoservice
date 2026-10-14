# Review Bundle: phase-3-clients — READY
**Base:** d3de17e | **HEAD:** 1d0506f | **Generated:** 2026-09-26 01:20 | diff: 111 files changed, 22102 insertions(+), 11 deletions(-)

> Линзы ревью: читай ЭТОТ бандл + patch своей линзы. Полное чтение research/draft-spec/PRD
> и перегон чекеров ЗАПРЕЩЕНЫ (вердикты ниже; детали — checks/<name>.txt; точечный grep по DS-NNN разрешён).

## Чекеры (детерминированные — НЕ перегонять)
| Чекер | Вердикт | exit | Нарушения (первые строки) |
|---|---|---|---|
| go_naming | ✅ | 0 | — |
| api_conventions | ✅ | 0 | — |
| contract | ⚠️ SKIP-RESOLVE (Go-код в diff есть — НЕ зелёно, §GO-1: перегенерируй bundle с --paths/--go) | 0 | — |
| date_bound | ✅ | 0 | — |
| fuzz_verdict | ✅ | 0 | — |
| live_suite | ✅ | 0 | — |
| fixture_sync | ✅ | 0 | — |
| e2e_mode | ✅ | 0 | — |
| rule_usage | ✅ | 0 | — |

## Diff по линзам
| Линза | Файлов | Patch |
|---|---|---|
| backend | 28 | 8-code-review/diff-backend.patch |
| e2e | 8 | 8-code-review/diff-e2e.patch |
| migrations | 2 | 8-code-review/diff-migrations.patch |
| other | 9 | 8-code-review/diff-other.patch |
| docs | 64 | — (артефакты, не ревьюятся построчно) |

### backend
- `A` internal/features/clients/clients_http_live_test.go
- `A` internal/features/clients/dto.go
- `A` internal/features/clients/handler.go
- `A` internal/features/clients/repo.go
- `A` internal/features/clients/routes.go
- `A` internal/features/clients/service.go
- `A` internal/features/clients/ssr.go
- `A` internal/features/clients/validate.go
- `A` internal/features/clients/validate_test.go
- `A` internal/features/equipmenttypes/dto.go
- `A` internal/features/equipmenttypes/equipmenttypes_http_live_test.go
- `A` internal/features/equipmenttypes/handler.go
- `A` internal/features/equipmenttypes/repo.go
- `A` internal/features/equipmenttypes/routes.go
- `A` internal/features/equipmenttypes/service.go
- `A` internal/features/equipmenttypes/ssr.go
- `A` internal/features/equipmenttypes/validate.go
- `A` internal/features/equipmenttypes/validate_test.go
- `M` internal/features/users/repo.go
- `M` internal/server/router.go
- `M` internal/ssr/menu.go
- `M` internal/ssr/render_test.go
- `M` internal/ssr/shell.go
- `A` internal/ssr/shell_test.go
- `A` pkg/sqlutil/like.go
- `A` pkg/sqlutil/like_test.go
- `A` pkg/sqlutil/text.go
- `A` pkg/sqlutil/text_test.go

### e2e
- `M` e2e/fixtures/auth.ts
- `A` e2e/pages/ClientsPage.ts
- `A` e2e/pages/EquipmentPage.ts
- `A` e2e/pages/EquipmentTypesPage.ts
- `A` e2e/tests/clients.spec.ts
- `A` e2e/tests/equipment-types.spec.ts
- `A` e2e/tests/equipment.spec.ts
- `M` e2e/tests/shell.spec.ts

### migrations
- `A` migrations/0003_clients_equipment.down.sql
- `A` migrations/0003_clients_equipment.up.sql

### other
- `M` web/ssr/_layout/templates/layouts/base.html
- `M` web/ssr/_shared/app.css
- `A` web/ssr/_shared/js/row-list.js
- `M` web/ssr/_shared/js/ui-dialog.js
- `A` web/ssr/clients/templates/pages/equipment-form.html
- `A` web/ssr/clients/templates/pages/form.html
- `A` web/ssr/clients/templates/pages/list.html
- `A` web/ssr/equipmenttypes/templates/pages/form.html
- `A` web/ssr/equipmenttypes/templates/pages/list.html

### docs
- `A` docs/features/phase-3-clients/0-setup/context-violations.jsonl
- `A` docs/features/phase-3-clients/0-setup/diagrams-prefs.md
- `A` docs/features/phase-3-clients/0-setup/e2e-scope-prefs.md
- `A` docs/features/phase-3-clients/0-setup/integration-scope-prefs.md
- `A` docs/features/phase-3-clients/0-setup/passport.md
- `A` docs/features/phase-3-clients/0-setup/pipeline-state.md
- `A` docs/features/phase-3-clients/0-setup/timeline.jsonl
- `A` docs/features/phase-3-clients/0-setup/ui-scope-prefs.md
- `A` docs/features/phase-3-clients/0-setup/watch-gate-prefs.md
- `A` docs/features/phase-3-clients/1-research/input-bundle.md
- `A` docs/features/phase-3-clients/1-research/output.md
- `A` docs/features/phase-3-clients/2-spec-interview/input-bundle.md
- `A` docs/features/phase-3-clients/2-spec-interview/output.md
- `A` docs/features/phase-3-clients/2-spec-interview/prd.md
- `A` docs/features/phase-3-clients/2-spec-interview/spec-decisions.md
- `A` docs/features/phase-3-clients/3-design/design-errors.md
- `A` docs/features/phase-3-clients/3-design/design-go-layers.md
- `A` docs/features/phase-3-clients/3-design/design-sql.md
- `A` docs/features/phase-3-clients/3-design/design-tests.md
- `A` docs/features/phase-3-clients/3-design/input-bundle.md
- `A` docs/features/phase-3-clients/3-design/output.md
- `A` docs/features/phase-3-clients/4-swimlane/design-swimlane.html
- `A` docs/features/phase-3-clients/4-swimlane/input-bundle.md
- `A` docs/features/phase-3-clients/5-design-review/input-bundle.md
- `A` docs/features/phase-3-clients/5-design-review/mobile.md
- `A` docs/features/phase-3-clients/5-design-review/output.md
- `A` docs/features/phase-3-clients/6-plan/code-plan-phase-01-shared-guard.md
- `A` docs/features/phase-3-clients/6-plan/code-plan-phase-02-migration.md
- `A` docs/features/phase-3-clients/6-plan/code-plan-phase-03-equipmenttypes-backend.md
- `A` docs/features/phase-3-clients/6-plan/code-plan-phase-04-equipmenttypes-frontend.md
- `A` docs/features/phase-3-clients/6-plan/code-plan-phase-05-clients-backend.md
- `A` docs/features/phase-3-clients/6-plan/code-plan-phase-06-clients-frontend.md
- `A` docs/features/phase-3-clients/6-plan/code-plan-phase-07-equipment-backend.md
- `A` docs/features/phase-3-clients/6-plan/code-plan-phase-08-equipment-frontend.md
- `A` docs/features/phase-3-clients/6-plan/code-plan-phase-09-hardening.md
- `A` docs/features/phase-3-clients/6-plan/code-plan.md
- `A` docs/features/phase-3-clients/6-plan/input-bundle.md
- `A` docs/features/phase-3-clients/6-plan/phase-bundles/phase-01-input.md
- `A` docs/features/phase-3-clients/6-plan/phase-bundles/phase-02-input.md
- `A` docs/features/phase-3-clients/6-plan/phase-bundles/phase-03-input.md
- `A` docs/features/phase-3-clients/6-plan/phase-bundles/phase-04-input.md
- `A` docs/features/phase-3-clients/6-plan/phase-bundles/phase-05-input.md
- `A` docs/features/phase-3-clients/6-plan/phase-bundles/phase-06-input.md
- `A` docs/features/phase-3-clients/6-plan/phase-bundles/phase-07-input.md
- `A` docs/features/phase-3-clients/6-plan/phase-bundles/phase-08-input.md
- `A` docs/features/phase-3-clients/6-plan/phase-bundles/phase-09-input.md
- `A` docs/features/phase-3-clients/8-code-review/checks/api_conventions.txt
- `A` docs/features/phase-3-clients/8-code-review/checks/contract.txt
- `A` docs/features/phase-3-clients/8-code-review/checks/date_bound.txt
- `A` docs/features/phase-3-clients/8-code-review/checks/e2e_mode.txt
- `A` docs/features/phase-3-clients/8-code-review/checks/fixture_sync.txt
- `A` docs/features/phase-3-clients/8-code-review/checks/fuzz_verdict.txt
- `A` docs/features/phase-3-clients/8-code-review/checks/go_naming.txt
- `A` docs/features/phase-3-clients/8-code-review/checks/live_suite.txt
- `A` docs/features/phase-3-clients/8-code-review/checks/rule_usage.txt
- `A` docs/features/phase-3-clients/8-code-review/diff-backend.patch
- `A` docs/features/phase-3-clients/8-code-review/diff-e2e.patch
- `A` docs/features/phase-3-clients/8-code-review/diff-migrations.patch
- `A` docs/features/phase-3-clients/8-code-review/diff-other.patch
- `A` docs/features/phase-3-clients/8-code-review/input-bundle.md
- `A` docs/features/phase-3-clients/8-code-review/mobile.md
- `A` docs/features/phase-3-clients/8-code-review/output.md
- `A` docs/features/phase-3-clients/9-validation/evidence/e2e-run.txt
- `A` docs/features/phase-3-clients/9-validation/evidence/last-run.json

## Экстракты документов (вместо полного чтения)
### draft-spec.md
MISSING

### prd.md (только AC-строки)
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

### design-errors.md
---
stage: 3-design
feature: phase-3-clients
tier: M
---
# Design Errors: phase-3-clients

Формат — `pkg/errorspkg` без изменений: `{code,message,details?}`, 400 `VALIDATION` с `details.fields[{name,message}]` (message — код, SSR переводит в текст). Коды — строковые (как `LOGIN_TAKEN`), числовых supportMessage в проекте нет.

## Sentinel Errors
`internal/features/clients/service.go`:
```go
var (
	ErrClientNotFound        = errorspkg.New("CLIENT_NOT_FOUND", http.StatusNotFound, "Клиент не найден")
	ErrEquipmentNotFound     = errorspkg.New("EQUIPMENT_NOT_FOUND", http.StatusNotFound, "Оборудование не найдено")
	ErrClientVersionConflict = errorspkg.New("CLIENT_VERSION_CONFLICT", http.StatusConflict,
		"Карточку уже изменил другой пользователь — ваши правки не сохранены")
)
```
`internal/features/equipmenttypes/service.go`:
```go
var (
	ErrTypeNotFound          = errorspkg.New("EQUIPMENT_TYPE_NOT_FOUND", http.StatusNotFound, "Тип оборудования не найден")
	ErrTypeNameTaken         = errorspkg.New("EQUIPMENT_TYPE_NAME_TAKEN", http.StatusConflict, "Такой тип уже есть")
	ErrTypeNameArchived = errorspkg.New("EQUIPMENT_TYPE_NAME_ARCHIVED", http.StatusConflict,
		"Такой тип уже есть в архиве — верните его из архива")                 // ADR-007: отдельный код → клиент показывает CTA «Вернуть из архива» по code, не по тексту
	ErrTypeInUse = errorspkg.New("EQUIPMENT_TYPE_IN_USE", http.StatusConflict,
		"Тип используется в оборудовании клиентов — удалить нельзя, отправьте его в архив")
)
```
Repo-уровень (неэкспортируемые, маппинг pg → домен в repo, `errors.Is` в сервисе — паттерн users): `ErrNotFound = pgx.ErrNoRows`, `errVersionConflict`, `errNameTaken`, `errNameArchived`, `errInUse` (23503), `errClientGone` (23503 на `equipment.client_id`).

НЕ вводятся (фаза 5, Tech Spec D26): `CLIENT_HAS_ORDERS`, `EQUIPMENT_IN_ORDERS`.

## Codes Fields
Коды `details.fields[].message` (константы в `validate.go` каждой фичи; `required/too_long/invalid_format/invalid` — как users):
| Код | Смысл | Поля |
|---|---|---|
| `required` | пусто после trim | `company_name`, `phones`, `phones[i].phone`, `model`, `type_id`, `name`, `version`, `is_active` |
| `too_long` | превышение rune-лимита | все строковые, `specs[i].k`, `specs[i].v`, `spec_keys[i]` |
| `too_many` | превышение числа строк | `phones` (>20), `specs` (>50), `spec_keys` (>50 после дедупа) |
| `invalid_format` | телефон без цифр | `phones[i].phone` |
| `invalid` | битый uuid | `type_id` |
| `not_found` | тип не существует | `type_id` |
| `archived` | тип в архиве (создание/смена) | `type_id` |
| `key_required` | значение без названия характеристики | `specs[i].k` |
| `invalid_text` | NUL-байт или невалидный UTF-8 (code review fix-loop #1, §TD-4: мусор → 400, не 500 от Postgres 22021) | все пользовательские строки clients / equipment / equipment-types |

Индекс `i` — позиция в НОРМАЛИЗОВАННОМ списке (пустые строки уже отброшены); SSR перерисовывает форму нормализованным списком → индексы совпадают со строками формы.

## Error Mapping
### API (`errorspkg.FiberErrorHandler`, без изменений)
| Ситуация | HTTP | code |
|---|---|---|
| невалидный JSON / неизвестные поля (`strictJSON`) | 400 | `BAD_REQUEST`, message «Некорректное тело запроса» (новые хендлеры фазы 3; `users` — as-is «invalid body», не переписывается молча) |
| ошибки полей | 400 | `VALIDATION` + `details.fields` |
| битый/несуществующий `:id` | 404 | `CLIENT_NOT_FOUND` / `EQUIPMENT_NOT_FOUND` / `EQUIPMENT_TYPE_NOT_FOUND` |
| POST equipment, клиента нет / удалён параллельно | 404 | `CLIENT_NOT_FOUND` |
| устаревший `version` | 409 | `CLIENT_VERSION_CONFLICT` |
| дубль имени активного типа | 409 | `EQUIPMENT_TYPE_NAME_TAKEN` |
| дубль имени архивного типа (ADR-007) | 409 | `EQUIPMENT_TYPE_NAME_ARCHIVED` — клиент находит тип `GET /equipment-types?include_archived=true&q=<name>` и предлагает `PUT … is_active:true` |
| DELETE используемого типа | 409 | `EQUIPMENT_TYPE_IN_USE` |
| executor на admin-мутации | 403 | `FORBIDDEN` (`RequireAdmin(nil)`) |
| без сессии | 401 | `UNAUTHORIZED` (`RequireAPI`) |

### SSR (`formError` в `clients/ssr.go`, `equipmenttypes/ssr.go` — эталон `users/ssr.go`)
| Ошибка | Статус | Отображение |
|---|---|---|
| `VALIDATION` | 422 | текст у поля (`fieldText`), ввод сохранён |
| `ErrClientVersionConflict` | 409 | `vm.Error` = message + `vm.Conflict` (ссылка «Открыть актуальную версию», ADR-005) |
| `ErrTypeNameTaken` | 409 | текст у поля `name` |
| `ErrTypeNameArchived` | 409 | текст у поля `name` + ссылка «Показать архивные» (`/catalog/equipment-types?archived=1`) |
| `ErrTypeInUse` | 409 | форма типа: `vm.Error` + видимая кнопка «В архив» |
| `*NotFound` | 404 | `status.NotFound` |
| прочее ≥500 | 500 | пробрасывается (`FiberErrorHandler`) |

### fieldText (SSR, тексты)
| Код | Текст |
|---|---|
| `required` | «Обязательное поле»; `phones` — «Укажите хотя бы один телефон»; `type_id` — «Выберите тип» |
| `too_long` | «Слишком длинное значение» |
| `too_many` | `phones` — «Не больше 20 телефонов»; `specs`/`spec_keys` — «Не больше 50 характеристик» |
| `invalid_format` | «Номер должен содержать цифры» |
| `invalid` / `not_found` | «Тип не найден» |
| `archived` | «Тип в архиве — выберите другой» |
| `key_required` | «Укажите характеристику» |
| `invalid_text` | «Недопустимое значение» |

## Инвентарь доков (точечный grep разрешён, полное чтение — нет)
- 6-plan/phase-bundles/phase-05-input.md — 51KB
- 6-plan/phase-bundles/phase-03-input.md — 51KB
- 6-plan/phase-bundles/phase-07-input.md — 51KB
- 6-plan/phase-bundles/phase-02-input.md — 49KB
- 3-design/input-bundle.md — 36KB
- 6-plan/phase-bundles/phase-06-input.md — 35KB
- 6-plan/phase-bundles/phase-08-input.md — 34KB
- 6-plan/phase-bundles/phase-09-input.md — 34KB
- 6-plan/phase-bundles/phase-04-input.md — 34KB
- 6-plan/phase-bundles/phase-01-input.md — 33KB
- 4-swimlane/input-bundle.md — 31KB
- 1-research/output.md — 29KB
- 3-design/output.md — 29KB
- 3-design/design-go-layers.md — 23KB
- 2-spec-interview/output.md — 18KB
- 8-code-review/input-bundle.md — 17KB
- 2-spec-interview/input-bundle.md — 16KB
- 3-design/design-tests.md — 15KB
- 2-spec-interview/prd.md — 13KB
- 5-design-review/output.md — 11KB
- 3-design/design-sql.md — 10KB
- 8-code-review/output.md — 8KB
- 8-code-review/mobile.md — 7KB
- 2-spec-interview/spec-decisions.md — 6KB
- 3-design/design-errors.md — 6KB

## Паспорт (выжимка; полный — 0-setup/passport.md)
# Passport: phase-3-clients (generated — руками не править; stage_bundle.py)
**Tier:** M | **Совместимость:** ? (pipeline-doctrine.md §CM-1; ? → уточни у оркестратора)
**Трек:** обычный (prefs нет — дефолт) — режимы агентов (`spec-track delta/micro/mini/merged`) включает ТОЛЬКО строка `Режим:` в prompt оркестратора; DS/TS-индексы ниже присутствуют на ЛЮБОМ треке при наличии draft-spec.
**Layout:** автодетект (tools/layout_resolve.py)
**UI-scope (pipeline-doctrine.md §US-1, 0-setup/ui-scope-prefs.md):** swagger:no | adaptive:min | themes:both
