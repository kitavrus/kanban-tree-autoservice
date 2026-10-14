# Review Bundle: suppliers — CHECKS-FAILED
**Base:** 18a0137023680a2073f2a197e40d8bbb080f346e | **HEAD:** 0cc63da | **Generated:** 2026-10-07 13:01 | diff: 30 files changed, 3849 insertions(+), 1 deletion(-)

> Линзы ревью: читай ЭТОТ бандл + patch своей линзы. Полное чтение research/draft-spec/PRD
> и перегон чекеров ЗАПРЕЩЕНЫ (вердикты ниже; детали — checks/<name>.txt; точечный grep по DS-NNN разрешён).

## Чекеры (детерминированные — НЕ перегонять)
| Чекер | Вердикт | exit | Нарушения (первые строки) |
|---|---|---|---|
| go_naming | ✅ | 0 | — |
| api_conventions | ✅ | 0 | — |
| contract | ✅ | 0 | — |
| date_bound | ✅ | 0 | — |
| fuzz_verdict | ✅ | 0 | — |
| live_suite | ✅ | 0 | — |
| fixture_sync | ❌ | 1 | fixture-sync: FAIL — схема/DTO изменены, тест-данные НЕ тронуты (TD-1/TD-6.2: та же фаза) |
| e2e_mode | ✅ | 0 | — |
| rule_usage | ✅ | 0 | — |
| e2e_drift | ❌ | 1 | e2e-drift: FAIL (режим: TS (spec-track §ST-1.4), doc: 1 id, code: 0 в 0 файлах, проблем: 1)<br>DRIFT: TS-035 есть в draft-spec.md, теста test('TS-035: …') нет (сценарий без теста) |

## Diff по линзам
| Линза | Файлов | Patch |
|---|---|---|
| backend | 12 | 8-code-review/diff-backend.patch |
| migrations | 2 | 8-code-review/diff-migrations.patch |
| other | 5 | 8-code-review/diff-other.patch |
| docs | 11 | — (артефакты, не ревьюятся построчно) |

### backend
- `A` internal/features/suppliers/dto.go
- `A` internal/features/suppliers/handler.go
- `A` internal/features/suppliers/migration_integration_test.go
- `A` internal/features/suppliers/repo.go
- `A` internal/features/suppliers/routes.go
- `A` internal/features/suppliers/service.go
- `A` internal/features/suppliers/ssr.go
- `A` internal/features/suppliers/suppliers_http_live_test.go
- `A` internal/features/suppliers/validate.go
- `A` internal/features/suppliers/validate_test.go
- `M` internal/server/router.go
- `M` internal/ssr/menu.go

### migrations
- `A` migrations/0012_suppliers.down.sql
- `A` migrations/0012_suppliers.up.sql

### other
- `M` e2e/fixtures/auth.ts
- `A` e2e/pages/SuppliersPage.ts
- `A` e2e/tests/suppliers.spec.ts
- `A` web/ssr/_shared/js/suppliers.js
- `A` web/ssr/suppliers/templates/pages/list.html

### docs
- `A` docs/features/suppliers/0-setup/draft-spec.slice-light.md
- `A` docs/features/suppliers/0-setup/idioms.md
- `A` docs/features/suppliers/0-setup/light-progress.md
- `A` docs/features/suppliers/0-setup/passport.md
- `A` docs/features/suppliers/0-setup/pipeline-state.md
- `A` docs/features/suppliers/0-setup/timeline.jsonl
- `A` docs/features/suppliers/draft-spec-decisions.md
- `A` docs/features/suppliers/draft-spec-flow.svg
- `A` docs/features/suppliers/draft-spec-pmap.txt
- `A` docs/features/suppliers/draft-spec.html
- `A` docs/features/suppliers/draft-spec.md

## Экстракты документов (вместо полного чтения)
### draft-spec.md (экстракт: шапка + DS + дельта + Impact + TS)
# Draft Spec: Справочник «Поставщики»

**Tier:** S · **Совместимость:** MVP · **Режим:** new

**Дата:** 2026-10-07 | **Статус:** draft → final (после подтверждения пользователя) | **Tier:** S
**Совместимость:** MVP (DS-027 — контекст, реализуется в `transfers-core`) — живых данных нет
**UI-scope:** swagger:no | adaptive:min | themes:both (DS-028 — контекст, реализуется в `transfers-core`)
**Источник:** мастер-спека `docs/features/transfers/draft-spec.md` (§4.3, §6.10–§6.12, §8.2) + её журнал `docs/features/transfers/draft-spec-decisions.md`
**Часть 1 из 6 мастер-спеки `transfers`; зависит от: —** (независима). Номера DS и TS — мастер-идентификаторы, не перенумерованы.

> После мержа этой части: справочник «Поставщики» работает (список, создание, правка, архив), но нигде не используется — документы «поставщик → склад» появятся в части 4 `transfers-partners`.

## §9. Решения (DS-index)
| DS | Формулировка | Источник | Статус |
|---|---|---|---|
| DS-007 | Справочник «Поставщики»: название (обязательное, уникальное), контактное лицо, телефон, адрес, примечание; только администратор; без удаления — архивирование | мастер, батч 1 | closed |
| DS-043 | Поставщики: last-write-wins без версии; архивирование свободное | мастер, inferred (по образцу складов) | closed |
| DS-016 | В документе от поставщика цен нет; товары только из каталога | мастер, батч 3 | closed — контекст, реализуется в `transfers-partners` |
| DS-027 | Совместимость — MVP | мастер, батч 6 | closed — контекст, реализуется в `transfers-core` |
| DS-028 | UI-scope: swagger:no, adaptive:min, themes:both | мастер, батч 6 | closed — контекст, реализуется в `transfers-core` |
| DS-031 | Время храним в UTC, показываем по поясу пользователя | мастер, батч 6 | closed — контекст, реализуется в `user-timezone` |
| DS-051 | Отдельной таблицы аудита нет | мастер, zap | closed — контекст, реализуется в `transfers-core` |
| DS-052 | UI-конвенции: ошибка под полем, тост справа вверху 5 с, блокировка кнопки | мастер, inferred | closed — контекст, реализуется в `transfers-core` |

## §10. Открытые вопросы (status: open)
Открытых вопросов нет.

## §11. Тестовые сценарии (черновик — вход для design-e2e-strategy, TD-6.8)
UI-scope: swagger-сценариев нет; адаптив и темы — проверяются общими сценариями TS-045, TS-046 части `transfers-core`; здесь — одна проверка overflow в составе TS-035.

### Unit / Integration (Go)
- **TS-020.** Поставщики: `name` required / 200 символов / уникальность без регистра и с пробелами по краям, включая архивные; `phone` без цифр → 400, 51 символ → 400; длины `contact_name` (200), `address` (500), `note` (1000); `is_active` обязателен в `PUT`; параллельное создание с одним названием → ровно один 201; executor → 403 на все три endpoint'а; фильтры списка `q`, `is_active`, конверт `{items,total,limit,offset}` — DS-007, DS-043
  - негатив: несуществующий id → 404 `SUPPLIER_NOT_FOUND`; `is_active=maybe` → 400
  - не входит в эту часть: «архивный поставщик в новом документе → 400 `supplier_id: archived`» — проверяется в `transfers-partners`

### E2E (Playwright)
- **TS-035.** Справочник «Поставщики» — экран §8.1 — DS-007
  - шаги: admin → «Справочники → Поставщики» → «Добавить» → заполнить → «Сохранить» → открыть → «В архив» → включить «Показать архивные» → «Вернуть из архива»; проверка: строка в таблице, бейдж «в архиве» появляется и исчезает, при вьюпорте 375×812 нет горизонтальной прокрутки; негатив: дубль названия → под полем «Поставщик с таким названием уже есть»; executor: пункта меню нет, прямой URL `/suppliers` → «Недостаточно прав»
  - не входит в эту часть: «архивного поставщика нет в select нового документа» — проверяется в `transfers-partners`

### Фикстуры и сиды (АКЦЕНТ — хронический пропуск)
- Новый page-object `e2e/pages/SuppliersPage.ts`; payload'ы e2e-хелперов типизировать wire-типами из исходников, не сырыми ключами (§TD-6.2).
- seed-demo в этой части не меняется; демо-поставщики — в `transfers-partners` (TS-F1 — контекст, реализуется в `transfers-core`).

## §Impact — точки касания существующего кода
Источник: `impact_scan.py --sample internal/features/suppliers --terms suppliers,supplier_id,/suppliers` → NO-FINDINGS (каталога образца ещё нет, термы в репозитории не встречаются); строки ниже — реестры, куда новая фича обязана вписаться.

| # | Класс | Что задеваем (file:line / объект) | Дельта фичи | Решение | DS |
|---|---|---|---|---|---|
| 1 | ENUMERATION | `internal/server/router.go` | регистрация пакета `suppliers` | добавить | DS-007 |
| 2 | ENUMERATION | `internal/ssr/menu.go:27-31` (группа «Справочники») | пункт «Поставщики», `/suppliers`, `AdminOnly: true` | добавить | DS-007 |
| 3 | FK | `migrations/` | новая миграция с `t:suppliers` (номер — следующий свободный на момент мержа) | добавить | DS-007 |
| 4 | CONFIG | env-ключи | не задеваются | — | — |
| 5 | MONITORING | дашборды/алерты | не задеваются | — | — |

**Вне каталога фичи:** есть — internal/server/router.go, internal/ssr/menu.go, migrations, e2e

### prd.md
MISSING

## Инвентарь доков (точечный grep разрешён, полное чтение — нет)
- draft-spec.md — 31KB
- 0-setup/draft-spec.slice-light.md — 27KB
- 8-code-review/input-bundle.md — 11KB
- 0-setup/idioms.md — 5KB
- 0-setup/passport.md — 3KB
- 0-setup/light-progress.md — 1KB
- draft-spec-decisions.md — 1KB
- 0-setup/pipeline-state.md — 0KB
- 9-validation/evidence/last-run.json — 0KB

## Паспорт (выжимка; полный — 0-setup/passport.md)
# Passport: suppliers (generated — руками не править; stage_bundle.py)
**Tier:** S | **Совместимость:** MVP (pipeline-doctrine.md §CM-1; ? → уточни у оркестратора)
**Трек:** обычный (prefs нет — дефолт) — режимы агентов (`spec-track delta/micro/mini/merged`) включает ТОЛЬКО строка `Режим:` в prompt оркестратора; DS/TS-индексы ниже присутствуют на ЛЮБОМ треке при наличии draft-spec.
**Layout:** автодетект (tools/layout_resolve.py)
**UI-scope (pipeline-doctrine.md §US-1, шапка draft-spec):** swagger:no | adaptive:min | themes:both
