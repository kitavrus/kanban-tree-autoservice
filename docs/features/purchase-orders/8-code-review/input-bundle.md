# Review Bundle: purchase-orders — CHECKS-FAILED
**Base:** 831b32c834c6228bf8f60c409cf3243d48aaef13 | **HEAD:** cd54f1d | **Generated:** 2026-10-22 01:58 | diff: 122 files changed, 22844 insertions(+), 66 deletions(-)

> Линзы ревью: читай ЭТОТ бандл + patch своей линзы. Полное чтение research/draft-spec/PRD
> и перегон чекеров ЗАПРЕЩЕНЫ (вердикты ниже; детали — checks/<name>.txt; точечный grep по DS-NNN разрешён).

## Чекеры (детерминированные — НЕ перегонять)
| Чекер | Вердикт | exit | Нарушения (первые строки) |
|---|---|---|---|
| go_naming | ✅ | 0 | — |
| api_conventions | ✅ | 0 | — |
| contract | ✅ | 0 | — |
| date_bound | ✅ | 0 | — |
| fuzz_verdict | ❌ | 1 | FUZZ-VERDICT FAIL SPEC-MISSING: вердикт `да` (3-design/design-e2e-strategy.md:8), а frontend/e2e/fuzz/purchase-orders_fuzz.spec.ts отсутствует — owner frontend-dev (шаблон templates/qa-fuzz-playwright.md), `--list` в гейте фазы |
| live_suite | ✅ | 0 | — |
| fixture_sync | ✅ | 0 | — |
| e2e_mode | ✅ | 0 | — |
| rule_usage | ✅ | 0 | — |
| e2e_drift | ❌ | 1 | e2e-drift: FAIL (режим: TS (spec-track §ST-1.4), doc: 2 id, code: 12 в 2 файлах, проблем: 10)<br>DRIFT: test('TS-015…') есть в e2e/, сценария в design-e2e-strategy.md нет (тест-сирота)<br>DRIFT: test('TS-016…') есть в e2e/, сценария в design-e2e-strategy.md нет (тест-сирота)<br>DRIFT: test('TS-017…') есть в e2e/, сценария в design-e2e-strategy.md нет (тест-сирота)<br>DRIFT: test('TS-018…') есть в e2e/, сценария в design-e2e-strategy.md нет (тест-сирота)<br>DRIFT: test('TS-019…') есть в e2e/, сценария в design-e2e-strategy.md нет (тест-сирота)<br>DRIFT: test('TS-020…') есть в e2e/, сценария в design-e2e-strategy.md нет (тест-сирота)<br>DRIFT: test('TS-022…') есть в e2e/, сценария в design-e2e-strategy.md нет (тест-сирота) |

## Diff по линзам
| Линза | Файлов | Patch |
|---|---|---|
| backend | 51 | 8-code-review/diff-backend.patch |
| e2e | 7 | 8-code-review/diff-e2e.patch |
| migrations | 2 | 8-code-review/diff-migrations.patch |
| other | 13 | 8-code-review/diff-other.patch |
| docs | 49 | — (артефакты, не ревьюятся построчно) |

### backend
- `M` cmd/server/main.go
- `M` go.mod
- `M` go.sum
- `M` internal/features/auth/handler.go
- `M` internal/features/auth/repo.go
- `M` internal/features/auth/service.go
- `A` internal/features/board/pdf_kit.go
- `M` internal/features/board/stock_repo.go
- `A` internal/features/purchase-orders/constants/constants.go
- `A` internal/features/purchase-orders/handler/handler.go
- `A` internal/features/purchase-orders/handler/orders.go
- `A` internal/features/purchase-orders/handler/print_pdf.go
- `A` internal/features/purchase-orders/handler/purchase_orders_http_live_test.go
- `A` internal/features/purchase-orders/handler/render_test.go
- `A` internal/features/purchase-orders/handler/ssr.go
- `A` internal/features/purchase-orders/mappers/mappers.go
- `A` internal/features/purchase-orders/models/dto/purchase_orders_dto.go
- `A` internal/features/purchase-orders/models/dto/supplier_dto.go
- `A` internal/features/purchase-orders/models/purchase_order_items.go
- `A` internal/features/purchase-orders/models/purchase_orders.go
- `A` internal/features/purchase-orders/repository/migration_integration_test.go
- `A` internal/features/purchase-orders/repository/queries.go
- `A` internal/features/purchase-orders/repository/repository.go
- `A` internal/features/purchase-orders/service/checks_test.go
- `A` internal/features/purchase-orders/service/service.go
- `A` internal/features/purchase-orders/validators/helpers_test.go
- `A` internal/features/purchase-orders/validators/validators.go
- `A` internal/features/purchase-orders/validators/validators_fuzz_test.go
- `A` internal/features/purchase-orders/validators/validators_test.go
- `M` internal/features/softdelete/kinds.go
- `A` internal/features/softdelete/purchase_orders_source.go
- `M` internal/features/softdelete/reason_test.go
- `M` internal/features/softdelete/softdelete_http_live_test.go
- `A` internal/features/suppliers/open_purchases.go
- `M` internal/features/suppliers/service.go
- `M` internal/features/suppliers/soft_delete.go
- `M` internal/features/users/dto.go
- `M` internal/features/users/handler.go
- `M` internal/features/users/repo.go
- `M` internal/features/users/service.go
- `M` internal/features/users/ssr.go
- `M` internal/features/users/validate.go
- `M` internal/features/users/validate_test.go
- `M` internal/server/middleware/auth.go
- `A` internal/server/middleware/supplier_test.go
- `M` internal/server/router.go
- `M` internal/ssr/menu.go
- `M` internal/ssr/shell.go
- `M` internal/ssr/shell_test.go
- `A` pkg/errorspkg/purchase_orders.go
- `M` pkg/testutil/login.go

### e2e
- `M` e2e/fixtures/auth.ts
- `M` e2e/fixtures/board-types.ts
- `A` e2e/fixtures/purchase-orders.ts
- `A` e2e/pages/PurchaseOrdersPage.ts
- `M` e2e/pages/UsersPage.ts
- `A` e2e/tests/fuzz/purchase-orders_fuzz.spec.ts
- `A` e2e/tests/purchase-orders.spec.ts

### migrations
- `A` migrations/0019_add_purchase_orders.down.sql
- `A` migrations/0019_add_purchase_orders.up.sql

### other
- `A` .claude/scaffold-layout.json
- `M` web/ssr/_layout/templates/layouts/base.html
- `M` web/ssr/_shared/app.css
- `M` web/ssr/_shared/js/deleted.js
- `A` web/ssr/_shared/js/purchases.js
- `A` web/ssr/_shared/js/supplier-orders.js
- `A` web/ssr/purchase-orders/templates/pages/list.html
- `A` web/ssr/purchase-orders/templates/pages/new.html
- `A` web/ssr/purchase-orders/templates/pages/supplier-list.html
- `A` web/ssr/purchase-orders/templates/pages/supplier-view.html
- `A` web/ssr/purchase-orders/templates/pages/view.html
- `M` web/ssr/users/templates/pages/form.html
- `M` web/ssr/users/templates/pages/list.html

### docs
- `A` docs/features/purchase-orders/0-setup/diagrams-prefs.md
- `A` docs/features/purchase-orders/0-setup/draft-spec.slice-light.md
- `A` docs/features/purchase-orders/0-setup/idioms.md
- `A` docs/features/purchase-orders/0-setup/light-progress.md
- `A` docs/features/purchase-orders/0-setup/passport.md
- `A` docs/features/purchase-orders/0-setup/pipeline-state.md
- `A` docs/features/purchase-orders/0-setup/timeline.jsonl
- `A` docs/features/purchase-orders/0-setup/watch-gate-prefs.md
- `A` docs/features/purchase-orders/1-research/output.md
- `A` docs/features/purchase-orders/10-qa/evidence/.gitkeep
- `A` docs/features/purchase-orders/10-qa/output.md
- `A` docs/features/purchase-orders/11-report/output.md
- `A` docs/features/purchase-orders/2-spec-interview/prd.md
- `A` docs/features/purchase-orders/2-spec-interview/spec-decisions.md
- `A` docs/features/purchase-orders/3-design/design-e2e-strategy.md
- `A` docs/features/purchase-orders/3-design/design-errors.md
- `A` docs/features/purchase-orders/3-design/design-frontend-architecture.md
- `A` docs/features/purchase-orders/3-design/design-go-layers.md
- `A` docs/features/purchase-orders/3-design/design-sql.md
- `A` docs/features/purchase-orders/3-design/design-tests.md
- `A` docs/features/purchase-orders/3-design/output.md
- `A` docs/features/purchase-orders/6-plan/code-plan-status.md
- `A` docs/features/purchase-orders/6-plan/code-plan.md
- `A` docs/features/purchase-orders/7-executing/scaffold-errors-snippet.md
- `A` docs/features/purchase-orders/7-executing/scaffold-manifest.json
- `A` docs/features/purchase-orders/8-code-review/checks/api_conventions.txt
- `A` docs/features/purchase-orders/8-code-review/checks/contract.txt
- `A` docs/features/purchase-orders/8-code-review/checks/date_bound.txt
- `A` docs/features/purchase-orders/8-code-review/checks/e2e_drift.txt
- `A` docs/features/purchase-orders/8-code-review/checks/e2e_mode.txt
- `A` docs/features/purchase-orders/8-code-review/checks/fixture_sync.txt
- `A` docs/features/purchase-orders/8-code-review/checks/fuzz_verdict.txt
- `A` docs/features/purchase-orders/8-code-review/checks/go_naming.txt
- `A` docs/features/purchase-orders/8-code-review/checks/live_suite.txt
- `A` docs/features/purchase-orders/8-code-review/checks/rule_usage.txt
- `A` docs/features/purchase-orders/8-code-review/diff-backend.patch
- `A` docs/features/purchase-orders/8-code-review/diff-e2e.patch
- `A` docs/features/purchase-orders/8-code-review/diff-migrations.patch
- `A` docs/features/purchase-orders/8-code-review/diff-other.patch
- `A` docs/features/purchase-orders/8-code-review/input-bundle.md
- `A` docs/features/purchase-orders/8-code-review/output.md
- `A` docs/features/purchase-orders/9-validation/evidence/.gitkeep
- `A` docs/features/purchase-orders/9-validation/output.md
- `A` docs/features/purchase-orders/draft-spec-decisions.md
- `A` docs/features/purchase-orders/draft-spec-flow.svg
- `A` docs/features/purchase-orders/draft-spec-pmap.txt
- `A` docs/features/purchase-orders/draft-spec-status.svg
- `A` docs/features/purchase-orders/draft-spec.html
- `A` docs/features/purchase-orders/draft-spec.md

## Экстракты документов (вместо полного чтения)
### draft-spec.md (экстракт: шапка + DS + дельта + Impact + TS)
# Draft Spec: Закупки по дефициту

**Дата:** 2026-10-22 | **Статус:** draft | **Tier:** L | **Режим:** new
**Совместимость:** MVP — живых пользователей нет, таблицы и адреса меняем свободно, без переходного периода и отката (DS-030, pipeline-doctrine.md §CM-1)
**UI-scope:** swagger:yes | adaptive:full | themes:both — adaptive:full относится к кабинету поставщика (§8.6–§8.7, с телефона); экраны «Закупки» (§8.1–§8.4) — один overflow-смок ~360px (DS-031, §US-1)
**Источник:** план «Заказ по дефициту» (shortage-order-plan.md, согласован 2026-10-22) + интервью 4 батча (draft-spec-decisions.md)

## §9. Решения (DS-index)
| DS | Формулировка | Источник | Статус |
|---|---|---|---|
| DS-001 | Проект `kanban-tree-autoservice`, один склад: остаток и минимум в `t:products` (`f:stock`, `f:min_stock`) | план, согласован 2026-10-22 | closed |
| DS-002 | Строки заказа из двух источников с галочками (`s:for_orders`, `s:below_min`) + ручные, всё правится | план, согласован 2026-10-22 | closed |
| DS-003 | К заказу = max(0, pending + `f:min_stock` − `f:stock`); `s:for_orders`, если pending > `f:stock`, иначе `s:below_min` | план, согласован 2026-10-22 | closed |
| DS-004 | Подсказка «уже в открытом заказе З-0012 — 5 шт.» на экране формирования, галочка снята | план, согласован 2026-10-22 | closed |
| DS-005 | Страницу «Остатки» `/stock` не трогаем | план, согласован 2026-10-22 | closed |
| DS-006 | Один заказ — один получатель: поставщик или «Базар» (`f:supplier_id` NULL) | план, согласован 2026-10-22 | closed |
| DS-007 | Кабинет поставщика: роль `s:supplier` + `f:supplier_id` в `t:users`, CHECK роль supplier ⇔ supplier_id задан | план, согласован 2026-10-22 | closed |
| DS-008 | Поставщик по строке: «Соберу» (0 = нет), цена за ед., комментарий; на заказ — «Собрано» | план, согласован 2026-10-22 | closed |
| DS-009 | Сумму считает система: `f:supplier_qty` × `f:supplier_price` | план, согласован 2026-10-22 | closed |
| DS-010 | «Забрали» только закрывает заказ, остатки не меняет | план, согласован 2026-10-22 | closed |
| DS-011 | Статусы draft → sent → ready → picked_up; cancelled из draft/sent/ready; Базар draft → picked_up | план, согласован 2026-10-22 | closed |
| DS-012 | «Отозвать» sent → draft, пока не «Собрано» | план, согласован 2026-10-22 | closed |
| DS-013 | Закупки создают и отправляют admin и executor; раздел виден обеим ролям | план, согласован 2026-10-22 | closed |
| DS-014 | Печать — PDF, тем же способом, что акт выдачи (`internal/features/board/print_pdf.go`) | план, согласован 2026-10-22 | closed |
| DS-015 | Номер сквозной, вид «З-0001» | план, согласован 2026-10-22 | closed |
| DS-016 | Один этап MVP; порядок: таблицы+API → роль supplier и кабинет → наши экраны → PDF; демо после наших экранов | план, согласован 2026-10-22 | closed |
| DS-017 | Роль supplier явно запрещена на всех существующих страницах и API; свой guard; чужой id → 404; live-тесты на каждый раздел | план, согласован 2026-10-22 | closed |
| DS-018 | После входа поставщика — редирект на `/supplier/orders`, свой упрощённый сайдбар | план, согласован 2026-10-22 | closed |
| DS-019 | Поставщик видит заказы своего поставщика в sent/ready/picked_up | план, согласован 2026-10-22 | closed |
| DS-020 | Поставщик правит строки только в `s:sent`; после ready и recall — только чтение | план, согласован 2026-10-22 | closed |
| DS-021 | Снимок name/sku/unit в строке | план, согласован 2026-10-22 | closed |
| DS-022 | `f:reason` строки: `s:for_orders`/`s:below_min`/`s:manual` | план, согласован 2026-10-22 | closed |
| DS-023 | Наши экраны: `/purchases`, `/purchases/new`, `/purchases/:id`, печать | план, согласован 2026-10-22 | closed |
| DS-024 | Кабинет: `/supplier/orders`, `/supplier/orders/:id`, удобный с телефона | план, согласован 2026-10-22 | closed |
| DS-025 | «Пользователи»: роль «Поставщик» + привязка к записи справочника | план, согласован 2026-10-22 | closed |
| DS-026 | Перечень API закупок и кабинета (§6), CLAUDE.md §API-1 | план, согласован 2026-10-22 | closed |
| DS-027 | Список: номер, получатель, статус, строк, сумма, даты; фильтр по статусу | план, согласован 2026-10-22 | closed |
| DS-028 | Потом: «Дозаказать недостающее», уведомления, приход по «Забрали», основной поставщик и история цен | план, согласован 2026-10-22 | closed |
| DS-029 | `f:qty` numeric(12,3), `f:supplier_price` numeric(12,2) | план, согласован 2026-10-22 | closed |
| DS-030 | Совместимость MVP: живых пользователей нет, без переходного периода и отката | батч 1 | closed |
| DS-031 | UI-scope: swagger:yes; adaptive:full для кабинета, экраны «Закупки» — overflow-смок; themes:both | батч 1 | closed |
| DS-032 | Конкурентность: `f:version` заказа + 409; своя `f:version` у строки поставщика; «Собрано» против «Отозвать» — выигрывает первый | батч 1 | closed |
| DS-033 | Время: UTC (`timestamptz`), показ в поясе смотрящего (по умолчанию Asia/Almaty, у поставщика тоже); фильтра по дате нет | батч 1 | closed |
| DS-034 | Править строки можно и в `s:sent`; поставщик видит изменения сразу, ответ по изменённым строкам сбрасывается | батч 2 | closed |
| DS-035 | При «Отозвать» и повторной отправке ответы сохраняются, у изменённых и добавленных строк сбрасываются; отозванный заказ скрыт из кабинета | батч 2 | closed |
| DS-036 | Отмена — обязательная причина 1–500 символов; отменённый заказ скрыт из кабинета | батч 2 | closed |
| DS-037 | Удалять можно только черновик, мягко, в «Удалённые» с восстановлением | батч 2 | closed |
| DS-038 | «Собрано» — только когда у всех строк задано «Соберу» (0 тоже ответ), цена обязательна при количестве > 0; отменить «Собрано» поставщик не может; «Забрали» — после «Собрано» | батч 2 | closed |
| DS-039 | После `s:ready` строки только для чтения | батч 3 | closed |
| DS-040 | «Изменённая строка» = любая правка, включая примечание | батч 3 | superseded by DS-048 |
| DS-041 | Причину отмены видим только мы | батч 3 | closed |
| DS-042 | Отправить поставщику без входа можно; если у поставщика нет активного входа, «Забрали» разрешено прямо из `s:sent` | батч 3 | closed |
| DS-043 | Архив и удаление поставщика с незакрытыми закупками (draft/sent/ready) запрещены, ошибка перечисляет номера | батч 3 | closed |
| DS-044 | Несколько входов на поставщика; создаёт только admin на «Пользователи» | батч 3 | closed |
| DS-045 | Блокировка входа поставщика сразу обрывает сессию (as-is `internal/server/middleware/auth.go` resolve) | батч 3 | closed |
| DS-046 | Поставщик видит номер, дату отправки, примечание, имя и телефон мастерской (`FIRM_NAME`/`FIRM_PHONE`); `f:reason` и наши цены не видит | батч 3 | closed |
| DS-047 | Валидации: qty > 0…99999.999 (3 знака); supplier_qty 0…99999.999, больше просимого можно; цена 0…9999999.99; комментарий ≤ 500; примечание ≤ 1000; дубль запчасти — сложение; ≤ 200 строк | батч 3 | closed |
| DS-048 | Правка строки сбрасывает ответ только этой строки + метка «Изменено заказчиком»; правка примечания ответы не сбрасывает, заказ у поставщика «Изменён», примечание подсвечено (заменяет DS-040) | батч 4 | closed |
| DS-049 | «Открытые» для подсказки — только `s:sent` и `s:ready`; количество без вычета, галочка снята | батч 4 | closed |
| DS-050 | Отменить могут admin и executor; удаление черновика, восстановление и окончательное удаление — только admin | батч 4 | closed |
| DS-051 | Executor правит, отправляет, отзывает, отменяет и закрывает только свои заказы (`f:created_by`) | батч 4 | closed |
| DS-052 | Аудит: у заказа кто/когда создал, отправил, собрал, забрал, отменил, удалил; у строки — кто из поставщика ответил последним; журнала нет | батч 4 | closed |
| DS-053 | Безопасность: роль supplier запрещена по умолчанию в `RequireAPI`/`RequireSSR` (API 403, SSR редирект на `/supplier/orders`); разрешены `/supplier/*`, `/api/v1/supplier/*`, выход, `PUT /api/v1/users/me/timezone`; поставщик берётся из сессии, не из body | inferred | closed |
| DS-054 | Снимок name/sku/unit берётся при добавлении строки и обновляется при каждой отправке | inferred | closed |
| DS-055 | Добавить в заказ можно только активную, не удалённую запчасть; ставшая архивной после добавления остаётся в заказе | inferred | closed |
| DS-056 | `f:number` — bigint из sequence; показ «З-» + номер, дополненный нулями до 4 знаков | inferred | closed |
| DS-057 | Список: сортировка `f:created_at` DESC; пагинация limit 1–100 (по умолчанию 50) / offset | inferred | closed |
| DS-058 | Базар: только «Забрали» из `s:draft` или «Отменить»; полей поставщика нет; в PDF колонка ☐ | inferred | closed |
| DS-059 | «Нет активного входа» проверяется в момент «Забрали»: нет пользователя с этим `f:supplier_id`, `f:role` = supplier и `f:is_active` | inferred | closed |
| DS-060 | Admin видит все заказы и делает с ними всё; executor видит в списке, открывает, печатает и меняет ТОЛЬКО свои (`f:created_by`), чужой id → 404 | батч 5 (update) | closed |
| DS-062 | Защита от двойного создания — блокировка кнопки; лишний черновик удаляет admin; предупреждение о несохранённых правках | inferred | closed |
| DS-063 | Ответ поставщика сохраняется построчно (blur поля) | inferred | closed |
| DS-064 | Подсказка переиспользует CTE pending из `internal/features/board/stock_repo.go`; в ответе только строки с `to_order` > 0 | inferred | closed |
| DS-065 | zap: ошибки `pkg/errorspkg`; 404/403/409; синхронно; миграции через `make migrate-up` | zap | closed |
| DS-066 | PDF для поставщика — отдельный маршрут без `f:reason` и наших данных; наш PDF доступен в любом статусе | inferred | closed |
| DS-067 | Кабинет: `s:sent` первыми, затем `f:sent_at` DESC | inferred | closed |
| DS-068 | `f:supplier_id` заказа задаётся при создании и не меняется | inferred | closed |
| DS-069 | Роль пользователя можно сменить на supplier и обратно при соблюдении CHECK | inferred | closed |
| DS-070 | Строки → заказ `ON DELETE CASCADE` (окончательное удаление черновика удаляет строки) | inferred | closed |
| DS-071 | Суммы вычисляются, не хранятся; итог — по строкам с ценой | inferred | closed |
| DS-072 | Новый вид `purchase-orders` («Закупки») в разделе «Удалённые» (`internal/features/softdelete/kinds.go`), причина удаления 1–500 | inferred | closed |
| DS-073 | Пункт «Закупки» в сайдбаре сотрудника после «Остатки»; UI-конвенции фидбека — как в проекте | inferred | closed |
| DS-075 | `s:picked_up` и `s:cancelled` терминальные: только чтение и печать | inferred | closed |
| DS-076 | Отправить и забрать можно только непустой заказ | inferred | closed |
| DS-077 | Заказ «Базар» нельзя отправить (409) | inferred | closed |
| DS-078 | Подсказка «уже в открытом заказе» учитывает открытые заказы (`s:sent`/`s:ready`): admin — все, executor — только свои (`f:created_by`); номера чужих заказов исполнителю не показываются. Принятый риск — возможны дубли закупок одной запчасти у разных исполнителей | батч 6 (update, пользователь) | closed |

Номера DS-061 и DS-074 не использованы (пропуск нумерации, не удаление).

## §10. Открытые вопросы (status: open)
Открытых вопросов нет.

## §11. Тестовые сценарии (черновик — вход для design-e2e-strategy, TD-6.8)

### Unit / Integration (Go)
- **TS-001.** `validateCreate`/`validateUpdate` заказа: `items` пуст → 400; 201 строка → 400; `qty` 0, −1, 100000, 1.0001 → 400, 0.001 и 99999.999 → ok; `note` 1001 символ → 400; `reason` вне enum → 400; неактивная или удалённая запчасть → 400; повтор product_id → количества сложены — DS-047, DS-055
- **TS-002.** `validateSupplierItem`: `supplier_qty` −0.001 / 100000 / 4 знака → 400, 0 и больше `qty` → ok; `supplier_price` −1 / 10000000 / 3 знака → 400; `supplier_note` 501 → 400 — DS-047
  - негатив: заказ в `s:ready` → 409 `PURCHASE_ORDER_READ_ONLY`
- **TS-003.** Матрица переходов §5 (repo-integration): каждый разрешённый переход → ok; каждый запрещённый (send из sent/ready/picked_up/cancelled, recall из ready, pick-up поставщику из draft, pick-up из sent при активном входе, cancel из picked_up/cancelled, delete не-draft) → 409 с нужным кодом — DS-011, DS-012, DS-042, DS-075
  - негатив: send у Базара → 409 `PURCHASE_ORDER_NO_SUPPLIER`; send или pick-up пустого → 409 `PURCHASE_ORDER_EMPTY`
- **TS-004.** Конкурентность: два PUT §6.4 с одной `version` → один 200, второй 409; одновременные «Собрано» и «Отозвать» → ровно один успех; два PUT строки §6.14 с одной версией → второй 409 — DS-032
  - негатив: устаревшая версия на каждом переходе §6.6–§6.9 → 409 `PURCHASE_ORDER_VERSION_CONFLICT`
- **TS-005.** Сброс ответа: в `s:sent` смена `qty` строки → `f:supplier_qty`/`f:supplier_price`/`f:supplier_note` очищены, `f:customer_changed_at` задан, `f:version` строки + 1; смена `note` → ответы целы, `f:note_changed_at` задан; recall + send без правок → ответы целы — DS-034, DS-035, DS-048
  - негатив: правка в `s:ready` → 409, ответы не тронуты
- **TS-006.** «Собрано»: строка без `f:supplier_qty` → 409 `PURCHASE_ORDER_ITEMS_INCOMPLETE` с `details.item_ids`; строка qty > 0 без цены → 409; все строки с ответом (включая 0 без цены) → ready — DS-038
- **TS-007.** Подсказка §6.10 на сидах: stock 1, min 2, pending 3 → to_order 4, `s:for_orders`; stock 3, min 5, pending 1 → 3, `s:below_min`; запчасть в заказе `s:sent` → `in_open_orders` заполнен, `checked=false`; в `s:draft` → не учитывается; to_order 0 → строки нет; запрос от executor — заказ другого автора в `in_open_orders` не попадает, `checked=true`; запрос от admin — попадает — DS-003, DS-004, DS-049, DS-078
- **TS-008.** Миграция: CHECK `i:users_supplier_chk` (role supplier без supplier_id → ошибка БД; executor с supplier_id → ошибка); CHECK `i:purchase_orders_bazaar_chk`; sequence номера; up/down на чистой БД — DS-007, DS-015
- **TS-009.** Live (§TD-8, `pkg/testutil.LiveSuite`): сессия supplier на КАЖДОМ существующем разделе — SSR `/`, `/stock`, `/clients`, `/equipment/:id`, `/catalog`, `/catalog/products`, `/catalog/services`, `/catalog/categories`, `/suppliers`, `/users`, `/calendar`, `/executor-payouts`, `/deleted`, `/tasks/:id/print/:kind`, `/tasks/:id/print/:kind/pdf`, `/purchases`, `/purchases/:id/print` → 302 на `/supplier/orders`; API `/api/v1/tasks`, `/api/v1/columns`, `/api/v1/subtasks`, `/api/v1/subtask-products`, `/api/v1/subtask-logs`, `/api/v1/attachments`, `/api/v1/assignees`, `/api/v1/stock`, `/api/v1/clients`, `/api/v1/equipment/:id`, `/api/v1/products`, `/api/v1/services`, `/api/v1/catalog-categories`, `/api/v1/suppliers`, `/api/v1/users`, `/api/v1/calendar`, `/api/v1/purchase-orders` → 403; разрешено: `PUT /api/v1/users/me/timezone`, выход — DS-017, DS-053
  - негатив: supplier А открывает заказ поставщика Б по id (§6.13, §6.14, §6.15, §6.16) → 404
- **TS-010.** Live: executor-А получает список §6.1 → только свои заказы (`total` не считает чужие); GET §6.3, PUT §6.4, POST §6.6–§6.9 и PDF §6.11 по заказу executor-Б → 404; executor DELETE своего черновика → 403; admin видит и меняет заказы обоих — 200 — DS-050, DS-051, DS-060
  - негатив: подмена `created_by` в body игнорируется — автор берётся из сессии
- **TS-011.** Live: архив и мягкое удаление поставщика с заказом в draft/sent/ready → 409 `SUPPLIER_HAS_OPEN_PURCHASES` со списком номеров; с заказами только picked_up/cancelled → ok — DS-043
- **TS-012.** Live: users — создание учётки supplier без `supplier_id` → 400; с архивным поставщиком → 400; executor с `supplier_id` → 400; блокировка учётки → следующий запрос 401 / редирект на вход — DS-044, DS-045
- **TS-013.** «Забрали» не меняет `t:products.stock` и не пишет `t:stock_movements` — DS-010

### E2E (Playwright)
- **TS-014.** Полный круг закупки у поставщика — экраны §8.2, §8.3, §8.6, §8.7 — DS-011, DS-008
  - шаги: admin → «Новый заказ» → поставщик → снять одну галочку → «Сохранить черновик» → «Отправить поставщику» → вход supplier → открыть заказ → заполнить строки (одну с «Соберу» меньше просимого) → «Собрано» → вход admin → строка с расхождением подсвечена, сумма верна → «Забрали»; проверка: бейдж «Забрали», остаток запчасти не изменился; негатив: «Собрано» с пустой строкой → строка подсвечена, статус `s:sent`
- **TS-015.** Базар — экраны §8.2, §8.3, §8.4 — DS-058
  - шаги: получатель «Базар» → сохранить → кнопки «Отправить» нет → «Печать» скачивает PDF с колонкой ☐ → «Забрали»; проверка: статус «Забрали»; негатив: POST send через API → 409
- **TS-016.** Правка отправленного заказа — экраны §8.3, §8.7 — DS-034, DS-048
  - шаги: поставщик ответил по двум строкам → мы меняем количество одной и примечание → поставщик видит «Изменён», подсвеченное примечание, у одной строки метка «Изменено заказчиком» и пустой ответ, вторая с ответом; проверка: после ответа метка снимается; негатив: мы правим после «Собрано» → тост «Поставщик уже собрал заказ — правка недоступна»
- **TS-017.** Отозвать и отправить снова — экраны §8.3, §8.6 — DS-035
  - шаги: отправить → поставщик ответил → «Отозвать» → у поставщика заказа нет в списке → «Отправить» → ответы на месте; негатив: «Отозвать» после «Собрано» → тост «Поставщик уже собрал заказ — отозвать нельзя»
- **TS-018.** Отмена с причиной — экраны §8.3, §8.6 — DS-036, DS-041
  - шаги: «Отменить» → пустая причина (кнопка неактивна) → ввести причину → бейдж «Отменён» и причина в шапке; проверка: у поставщика заказа нет; негатив: прямой URL `/supplier/orders/:id` → «Заказ не найден»
- **TS-019.** Подсказка «уже в открытом заказе» — экран §8.2 — DS-004, DS-049
  - шаги: запчасть в отправленном заказе З-0012 → «Новый заказ» → у строки подсказка «уже в открытом заказе З-0012 — N шт.», галочка снята; проверка: количество без вычета; негатив: запчасть только в черновике → подсказки нет, галочка стоит
- **TS-020.** Поставщик заперт в кабинете — экраны §8.6, §8.7 — DS-017, DS-018
  - шаги: вход supplier → редирект на `/supplier/orders`, сайдбар только «Заказы / Выход» → ввести в адресной строке `/`, `/stock`, `/clients`, `/purchases` → каждый раз возврат в `/supplier/orders`; негатив: чужой заказ по URL → «Заказ не найден»
- **TS-021.** Кабинет на телефоне — экран §8.7, вьюпорты 360×640 / 768 / 1280, светлая и тёмная темы — DS-031
  - шаги: открыть карточку → поля доступны без горизонтального скролла, итог закреплён внизу, числовая клавиатура; проверка: `scrollWidth <= clientWidth`, скриншот-база; негатив: длинное название запчасти не ломает вёрстку
- **TS-022.** Экраны «Закупки» на узком экране — §8.1–§8.3, один вьюпорт ~360px — DS-031
  - шаги: открыть список, новый заказ, карточку; проверка: нет горизонтального выхода страницы за край (таблица может скроллиться внутри своего контейнера); негатив: длинное имя поставщика не выталкивает кнопки за край
- **TS-023.** Учётка поставщика — экран §8.5 — DS-025, DS-044
  - шаги: admin → новый пользователь → роль «Поставщик» → появился дропдаун поставщика → сохранить без выбора → «Выберите поставщика» → выбрать → сохранить; проверка: вход этой учёткой ведёт в кабинет; негатив: executor открывает `/users` → 403-страница
- **TS-024.** Executor видит только свои заказы — экраны §8.1, §8.3, §8.2 — DS-051, DS-060, DS-078
  - шаги: executor открывает «Закупки» → в списке только его заказы → вставляет в адресную строку URL заказа admin → страница «Не найдено»; в «Новом заказе» у запчасти, которая есть только в отправленном заказе admin, подсказки «уже в открытом заказе» нет и галочка стоит; у запчасти из своего отправленного заказа — подсказка со ссылкой и галочка снята; проверка: admin на том же экране видит подсказку по обоим заказам; негатив: прямой POST cancel чужого → 404
- **TS-025.** Архив поставщика с открытым заказом — существующий экран «Поставщики» — DS-043
  - шаги: снять «Активен» у поставщика с отправленным заказом → ошибка «У поставщика есть незакрытые закупки: З-…»; негатив: после «Забрали» — архив проходит
- **TS-026.** Удаление и восстановление черновика — экраны §8.3 и «Удалённые» — DS-037, DS-072
  - шаги: admin → «Удалить» с причиной → черновик на вкладке «Закупки» раздела «Удалённые» → «Восстановить» → снова в списке; негатив: у отправленного заказа кнопки «Удалить» нет, API → 409

### Фикстуры и сиды (АКЦЕНТ — хронический пропуск)
- **TS-F1.** Сид e2e (`e2e/fixtures` + Go-сид live-тестов): поставщик «АвтоДеталь Плюс» с активной учёткой supplier, поставщик «Без кабинета» без учётки, второй поставщик со своей учёткой (для проверок «чужой → 404»), запчасти в состояниях `s:for_orders` / `s:below_min` / без дефицита, заказ клиента в колонке без `consumes_stock` (даёт pending) — DS-003, DS-042
- **TS-F2.** Заказы-закупки в каждом статусе (draft / sent с частичным ответом / ready / picked_up / cancelled / удалённый draft / Базар draft) у двух авторов: admin и executor — DS-011, DS-051
- **TS-F3.** Существующие хелперы входа e2e: добавить роль supplier; payload'ы хелперов типизировать wire-типами из src (§TD-6.2; ретро api-conventions-sweep: сырые ключи невидимы гейтам) — DS-007
- **TS-F4.** Существующие live-тесты, создающие пользователей (`internal/features/users/*_live_test.go`), и проверки роли (`internal/features/users/validate_test.go`): обновить под роль из трёх значений и поле `supplier_id` в той же фазе, что миграция — DS-007

## §Impact — точки касания существующего кода
Источник: `impact_scan.py --repo . --terms purchase_orders,purchase_order_items,purchase-orders,RoleAdmin,RoleExecutor,RequireSSR,RequireAPI,KindSuppliers,users_role,supplier_id,FIRM_PHONE,stock_repo,consumes_stock` (каталог фичи ещё не существует — SAMPLE-DIR-MISSING) + ручная интерпретация. Совпадения в `.claude/worktrees/*` отброшены (временные копии).

| # | Класс | Что задеваем (file:line / объект) | Дельта фичи | Решение | DS |
|---|---|---|---|---|---|
| 1 | SHARED-DEP | `internal/server/middleware/auth.go:95` `RequireAPI`, `:120` `RequireSSR`, `:26–27` роли | константа `RoleSupplier`; запрет supplier по умолчанию (API 403, SSR 302 → `/supplier/orders`) + allow-list; новый guard `RequireSupplier` | меняем общий guard, а не каждый routes.go: новые разделы закрыты автоматически | DS-017, DS-053 |
| 2 | ENUMERATION | `internal/features/users/validate.go:14–15, :87`, `internal/features/users/service.go:78`, `internal/features/users/ssr.go:86`, `internal/features/users/repo.go` (загрузка Principal) | роль `s:supplier`, поле `f:supplier_id`, `supplier_id` в Principal | расширяем | DS-007, DS-044 |
| 3 | FK | `migrations/0002_users_roles.up.sql` CHECK `role IN ('admin','executor')` | новая миграция `0019_purchase_orders`: CHECK роли из трёх значений, `f:supplier_id`, `i:users_supplier_chk`, таблицы §4.1–§4.2, sequence | MVP: без переходного периода | DS-007, DS-030 |
| 4 | FRONTEND | `internal/ssr/shell.go:20–21, :89` (меню по ролям), `web/ssr/_layout/templates/layouts/base.html:28` | пункт «Закупки» у admin и executor; отдельный упрощённый сайдбар supplier | расширяем shell | DS-018, DS-073 |
| 5 | SHARED-DEP | `internal/features/auth` (вход, редирект после логина) | после входа supplier → `/supplier/orders`; `?redirect=` на запрещённый раздел игнорируется | правка хендлера входа | DS-018 |
| 6 | ENUMERATION | `internal/features/softdelete/kinds.go:13–27` реестр `Kinds` + источники | новый вид `purchase-orders` («Закупки») + source (только draft) | расширяем реестр | DS-037, DS-072 |
| 7 | SHARED-DEP | `internal/features/suppliers/service.go` (архив через `is_active`), `internal/features/suppliers/soft_delete.go` (мягкое удаление) | guard «незакрытые закупки» → 409 `SUPPLIER_HAS_OPEN_PURCHASES` | правка сервиса поставщиков | DS-043 |
| 8 | SHARED-DEP | `internal/features/board/stock_repo.go:24` `stockPendingCTETemplate` | переиспользуем расчёт pending в §6.10 (только чтение) | вынести или экспортировать CTE без изменения поведения `/stock` | DS-005, DS-064 |
| 9 | SHARED-DEP | `internal/features/board/print_pdf.go`, `internal/features/board/fonts` | генерация PDF заказа тем же способом | переиспользуем шрифты и хелперы; при необходимости выносим общий PDF-хелпер | DS-014 |
| 10 | CONFIG | `internal/config/config.go:74` `FIRM_NAME`/`FIRM_PHONE` | читаем в кабинете и PDF | без изменений | DS-046 |
| 11 | ENUMERATION | `internal/features/*/routes.go` (board, clients, calendar, catalog, suppliers, users, softdelete, executorpayouts) | покрываются guard'ом из п. 1; `PUT /api/v1/users/me/timezone` остаётся доступен supplier | правка не нужна, только live-тесты TS-009 | DS-053 |

**Вне каталога фичи:** есть — internal/server/middleware/auth.go, internal/ssr/shell.go, web/ssr/_layout/templates/layouts/base.html, internal/features/users, internal/features/auth, internal/features/softdelete, internal/features/suppliers, internal/features/board/stock_repo.go, internal/features/board/print_pdf.go, migrations

### prd.md (только AC-строки)
нет AC-строк

### design-frontend-architecture.md (вердикты thin|rich, pw-fuzz, data-testid)
вердикт-строк не найдено (проверь вручную)

### design-errors.md
# Design Errors: purchase-orders — PENDING (error_prefix: PURCHASE_ORDERS)

<!-- scaffold stub — артефакт ещё не произведён; owner-стадия перезапишет содержимое -->

## Инвентарь доков (точечный grep разрешён, полное чтение — нет)
- draft-spec.md — 155KB
- 0-setup/draft-spec.slice-light.md — 136KB
- 8-code-review/input-bundle.md — 44KB
- 0-setup/passport.md — 22KB
- 7-executing/scaffold-manifest.json — 15KB
- 0-setup/idioms.md — 14KB
- draft-spec-decisions.md — 8KB
- 7-executing/scaffold-errors-snippet.md — 7KB
- 0-setup/light-progress.md — 3KB
- 3-design/design-e2e-strategy.md — 1KB
- 0-setup/pipeline-state.md — 0KB
- 3-design/design-errors.md — 0KB
- 3-design/design-frontend-architecture.md — 0KB
- 6-plan/code-plan.md — 0KB
- 3-design/design-go-layers.md — 0KB
- 3-design/design-tests.md — 0KB
- 8-code-review/output.md — 0KB
- 9-validation/output.md — 0KB
- 3-design/design-sql.md — 0KB
- 1-research/output.md — 0KB
- 3-design/output.md — 0KB
- 11-report/output.md — 0KB
- 0-setup/watch-gate-prefs.md — 0KB
- 6-plan/code-plan-status.md — 0KB
- 2-spec-interview/prd.md — 0KB

## Паспорт (выжимка; полный — 0-setup/passport.md)
# Passport: purchase-orders (generated — руками не править; stage_bundle.py)
**Tier:** L | **Совместимость:** MVP (pipeline-doctrine.md §CM-1; ? → уточни у оркестратора)
**Трек:** обычный (prefs нет — дефолт) — режимы агентов (`spec-track delta/micro/mini/merged`) включает ТОЛЬКО строка `Режим:` в prompt оркестратора; DS/TS-индексы ниже присутствуют на ЛЮБОМ треке при наличии draft-spec.
**Layout:** автодетект (tools/layout_resolve.py)
**UI-scope (pipeline-doctrine.md §US-1, шапка draft-spec):** swagger:yes | adaptive:full | themes:both
