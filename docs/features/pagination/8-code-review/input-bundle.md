# Review Bundle: pagination — CHECKS-FAILED
**Base:** bb4a7825b9260e0ae6252367637a4536f4e20896 | **HEAD:** 5372d21 | **Generated:** 2026-09-27 19:02 | diff: 50 files changed, 3528 insertions(+), 87 deletions(-)

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
| fixture_sync | ✅ | 0 | — |
| e2e_mode | ✅ | 0 | — |
| rule_usage | ✅ | 0 | — |
| e2e_drift | ❌ | 1 | e2e-drift: FAIL (режим: TS (spec-track §ST-1.4), doc: 10 id, code: 4 в 1 файлах, проблем: 6)<br>DRIFT: TS-001 есть в draft-spec.md, теста test('TS-001: …') нет (сценарий без теста)<br>DRIFT: TS-002 есть в draft-spec.md, теста test('TS-002: …') нет (сценарий без теста)<br>DRIFT: TS-003 есть в draft-spec.md, теста test('TS-003: …') нет (сценарий без теста)<br>DRIFT: TS-004 есть в draft-spec.md, теста test('TS-004: …') нет (сценарий без теста)<br>DRIFT: TS-005 есть в draft-spec.md, теста test('TS-005: …') нет (сценарий без теста)<br>DRIFT: TS-006 есть в draft-spec.md, теста test('TS-006: …') нет (сценарий без теста) |

## Diff по линзам
| Линза | Файлов | Patch |
|---|---|---|
| backend | 11 | 8-code-review/diff-backend.patch |
| e2e | 6 | 8-code-review/diff-e2e.patch |
| other | 8 | 8-code-review/diff-other.patch |
| docs | 25 | — (артефакты, не ревьюятся построчно) |

### backend
- `A` internal/features/board/pagination_http_live_test.go
- `M` internal/features/board/tasks_dto.go
- `M` internal/features/board/tasks_filter.go
- `M` internal/features/board/tasks_filter_test.go
- `M` internal/features/board/tasks_repo.go
- `A` internal/features/catalog/pagination_http_live_test.go
- `M` internal/features/catalog/ssr_categories.go
- `A` internal/features/catalog/ssr_pager.go
- `A` internal/features/catalog/ssr_pager_test.go
- `M` internal/features/catalog/ssr_products.go
- `M` internal/features/catalog/ssr_services.go

### e2e
- `M` e2e/fixtures/board-types.ts
- `M` e2e/pages/BoardPage.ts
- `M` e2e/pages/CatalogPage.ts
- `M` e2e/tests/board-guards.spec.ts
- `M` e2e/tests/catalog.spec.ts
- `A` e2e/tests/pagination.spec.ts

### other
- `M` web/ssr/_shared/app.css
- `M` web/ssr/_shared/js/board.js
- `M` web/ssr/board/templates/pages/board.html
- `M` web/ssr/catalog/templates/pages/products-form.html
- `M` web/ssr/catalog/templates/pages/products-list.html
- `M` web/ssr/catalog/templates/pages/services-form.html
- `M` web/ssr/catalog/templates/pages/services-list.html
- `A` web/ssr/catalog/templates/partials/catalog-pager.html

### docs
- `A` docs/features/pagination/0-setup/draft-spec.slice-light.md
- `A` docs/features/pagination/0-setup/idioms.md
- `A` docs/features/pagination/0-setup/light-progress.md
- `A` docs/features/pagination/0-setup/passport.md
- `A` docs/features/pagination/0-setup/pipeline-state.md
- `A` docs/features/pagination/0-setup/timeline.jsonl
- `A` docs/features/pagination/8-code-review/checks/api_conventions.txt
- `A` docs/features/pagination/8-code-review/checks/contract.txt
- `A` docs/features/pagination/8-code-review/checks/date_bound.txt
- `A` docs/features/pagination/8-code-review/checks/e2e_drift.txt
- `A` docs/features/pagination/8-code-review/checks/e2e_mode.txt
- `A` docs/features/pagination/8-code-review/checks/fixture_sync.txt
- `A` docs/features/pagination/8-code-review/checks/fuzz_verdict.txt
- `A` docs/features/pagination/8-code-review/checks/go_naming.txt
- `A` docs/features/pagination/8-code-review/checks/live_suite.txt
- `A` docs/features/pagination/8-code-review/checks/rule_usage.txt
- `A` docs/features/pagination/8-code-review/diff-backend.patch
- `A` docs/features/pagination/8-code-review/diff-e2e.patch
- `A` docs/features/pagination/8-code-review/diff-other.patch
- `A` docs/features/pagination/8-code-review/input-bundle.md
- `A` docs/features/pagination/8-code-review/output.md
- `A` docs/features/pagination/9-validation/evidence/e2e-run.txt
- `A` docs/features/pagination/9-validation/evidence/last-run.json
- `A` docs/features/pagination/api-delta.md
- `A` docs/features/pagination/draft-spec.md

## Экстракты документов (вместо полного чтения)
### draft-spec.md (экстракт: шапка + DS + дельта + Impact + TS)
# Draft Spec — pagination (доска + справочники каталога)

**Tier:** S
**Трек:** /pipeline-light · **Ветка:** `feat/pagination` (от `origin/feat/phase-8-polish`, bb4a782)
**UI-scope:** затронутые экраны (доска, каталог: услуги/товары/категории, форма услуги)
**Совместимость:** API — только аддитивные изменения (новые опциональные query-параметры и поле), конверт `{items,total,limit,offset}` (CLAUDE.md §API-1) сохраняется.

## 2. Решения (DS)

- **DS-001 Доска — ленивая догрузка по колонке.** Первичная загрузка — ОДИН запрос
  `GET /api/v1/tasks?<фильтры>&per_column=50&limit=1000`: сервер отдаёт не более 50 первых
  (по `t.position, t.id`) карточек каждой колонки. Внизу колонки, где показано меньше, чем
  есть: строка `показаны N из M` (`data-testid="column-shown-<column_id>"`) и кнопка
  «Показать ещё» (`data-testid="column-more-<column_id>"`) → `GET /api/v1/tasks?<фильтры>&
  column_id=<id>&limit=50&offset=<N>` — карточки дописываются в конец колонки.
  Смена фильтров / reloadAndRender сбрасывает догрузку (снова первые 50).
- **DS-002 API GET /api/v1/tasks (аддитивно):**
  - `column_id` (uuid/тип id колонки, опц.) — фильтр по колонке; невалидный → 400 VALIDATION
    с полем `column_id` (как `client_id`).
  - `per_column` (int 1..1000, опц.) — окно «не более N карточек на колонку» (row_number()
    OVER (PARTITION BY t.column_id ORDER BY t.position, t.id)); невалидный → 400 поле `per_column`.
    `limit/offset` применяются ПОСЛЕ окна; порядок items — прежний контракт.
  - Новое поле карточки `column_total` (int) — число заказов этой колонки, подходящих под
    фильтры (без учёта окна/limit/offset). Всегда присутствует.
  - `total` конверта — число заказов, подходящих под фильтры (без окна `per_column`, но с
    `column_id`, если он задан). Конверт `{items,total,limit,offset}` без новых ключей.
- **DS-003 Колонки / исполнители / категории фильтра доски** — загружаются ВСЕМИ страницами
  (цикл `limit=<max сервера>&offset=` до `offset >= total`, страховочный потолок 20 страниц —
  при его достижении `console.warn` и пометка в тулбаре). Лимиты сервера не меняются.
- **DS-004 Счётчик на доске:** в тулбаре/над доской строка `Заказов: M` (`data-testid=
  "board-total"`), а если на доске видно меньше M карточек — `Заказов: M · показаны N`.
  При перемещении карточки между колонками `column_total`/N в колонках пересчитываются
  локально (без полной перезагрузки) — либо после ответа сервера через существующий
  механизм, как сейчас устроено.
- **DS-005 Каталог SSR — постраничная выдача товаров и услуг.** `/catalog/products` и
  `/catalog/services`: страница по 100 позиций, параметр URL `offset` (не `page` — §API-1),
  кратный 100, невалидный/отрицательный → 0. Под таблицей — пейджер-partial (Bootstrap
  pagination): «Показаны 101–200 из 2345» (`data-testid="catalog-shown"`), ссылки «‹ Назад»/
  «Вперёд ›» (`data-testid="catalog-prev"`/`"catalog-next"`), сохраняющие `q`,`category`,
  `archived`. Смена фильтра/поиска сбрасывает offset в 0. offset за пределами total →
  пустая таблица + пейджер с «Назад» (не 404).
- **DS-006 Категории (боковая панель, страница категорий, select'ы форм)** — остаются
  lookup'ом с лимитом 2000, но при `total > len(items)` рендерится явная пометка
  `показаны N из M` (`data-testid="categories-truncated"`). Постраничность категорий не
  делается (осознанно: сотни, не тысячи).
- **DS-007 Комбобокс товара в форме услуги** (строки комплекта) — серверный поиск через
  `sel._cbFetchOptions = q => GET /api/v1/products?q=&limit=50` (эталон — task-card.js /
  board.js `wireClientCombobox`), чтобы товар за пределами первых 2000 находился вводом.
  Предзагруженный список опций остаётся (первые 2000 активных).
- **DS-008 Лимиты/дефолты API не меняются** (tasks 500/1000, columns 100/1000, catalog
  2000/2000, products/services default 50). Существующий код не переписывается сверх нужного.

## 11. Тестовые сценарии

| ID | Уровень | Сценарий |
|---|---|---|
| TS-001 | unit | ParseTaskFilter: `column_id` невалидный → VALIDATION поле `column_id`; `per_column` 0/1001/abc → поле `per_column`; валидные — попадают в фильтр |
| TS-002 | integration (LiveSuite) | `per_column=2` на колонке с 5 заказами → 2 карточки этой колонки, `column_total=5` у каждой, `total` = все подходящие под фильтр |
| TS-003 | integration (LiveSuite) | `column_id=X&limit=2&offset=2` → карточки 3–4 колонки X в порядке position,id; `total`=5 |
| TS-004 | integration (LiveSuite) | фильтр (например `q`) + `per_column` → `column_total` считается с учётом фильтра |
| TS-005 | integration (LiveSuite) | SSR `/catalog/products?offset=100` при >100 товарах → 200, в HTML пейджер «Показаны 101–…», ссылка назад; `offset=-5`/`abc` → как 0 |
| TS-006 | integration (LiveSuite) | SSR `/catalog/services?offset=…` — то же для услуг, фильтры `q/category/archived` сохраняются в ссылках пейджера |
| TS-007 | E2E | Доска: колонка с 51+ заказом (создать через API в фикстуре) → видно 50 карточек и «показаны 50 из M», «Показать ещё» догружает остальные, пометка исчезает |
| TS-008 | E2E | Доска: `board-total` показывает число заказов |
| TS-009 | E2E | Каталог товаров: >100 товаров → пейджер, переход «Вперёд» меняет строки, «Назад» возвращает; поиск сбрасывает на первую страницу |
| TS-010 | E2E | Форма услуги: товар находится вводом в комбобокс строки комплекта через серверный поиск |

### prd.md
MISSING

## Инвентарь доков (точечный grep разрешён, полное чтение — нет)
- 8-code-review/input-bundle.md — 13KB
- 0-setup/idioms.md — 12KB
- 8-code-review/output.md — 10KB
- draft-spec.md — 8KB
- api-delta.md — 5KB
- 0-setup/passport.md — 3KB
- 0-setup/draft-spec.slice-light.md — 2KB
- 0-setup/light-progress.md — 2KB
- 0-setup/pipeline-state.md — 0KB
- 9-validation/evidence/last-run.json — 0KB

## Паспорт (выжимка; полный — 0-setup/passport.md)
# Passport: pagination (generated — руками не править; stage_bundle.py)
**Tier:** S | **Совместимость:** ? (pipeline-doctrine.md §CM-1; ? → уточни у оркестратора)
**Трек:** обычный (prefs нет — дефолт) — режимы агентов (`spec-track delta/micro/mini/merged`) включает ТОЛЬКО строка `Режим:` в prompt оркестратора; DS/TS-индексы ниже присутствуют на ЛЮБОМ треке при наличии draft-spec.
**Layout:** автодетект (tools/layout_resolve.py)
**UI-scope (pipeline-doctrine.md §US-1, шапка draft-spec):** swagger:yes | adaptive:full | themes:both
