---
stage: 8-code-review
feature: purchase-orders
tier: L
verdict: CHANGES REQUESTED
high: 1
medium: 3
low: 4
---
# Code Review: purchase-orders — CHANGES REQUESTED
**Review level:** Full (tier: L; Режим: spec-track merged, Sec 18 включена; трек light, design-*.md — stubs → сверка против draft-spec)

Pre-condition (судейство оркестратора, не перегонялось): make build ok; go test 21 ok; lint 0; integration-scoped 24 ok; e2e full 213 passed/0 failed (`9-validation/evidence/last-run.json` status passed).

## Блокеры (обязательно исправить до мержа)
1. **[internal/features/users/service.go:153 `Update` → `checkSupplier`; validate.go:107]** Нельзя заблокировать/изменить учётку supplier, если её поставщик архивирован или мягко удалён. `checkSupplier` вызывается безусловно при `role=supplier` и требует `is_active AND deleted_at IS NULL` у поставщика; архив/удаление поставщика (suppliers/service.go:100, soft_delete.go:107) проверяет только незакрытые закупки (DS-043), учётки не трогает. Итог: PUT `/api/v1/users/:id` с `is_active=false` → 400 `supplier_id: unavailable` — отзыв доступа (DS-045) невозможен, вход поставщика удалённого поставщика продолжает работать; единственный обход — сменить роль на executor (выдаёт доступ сотрудника). Sec 6 (инвалидация сессии). Фикс: проверять доступность поставщика только когда роль становится supplier или меняется `supplier_id` (сравнить с текущей записью), либо пропускать при `is_active=false`; live-тест: архив поставщика → блокировка его учётки → 200 + следующий запрос 401.

## Серьёзные замечания (исправить в этом PR)
1. **[internal/features/suppliers/open_purchases.go:13; purchase-orders/repository/repository.go:408 `Restore`; service/service.go:257 `SendCheck`]** Инвариант DS-043 «у архивного/удалённого поставщика нет незакрытых закупок» обходится: (а) `Restore` восстанавливает черновик, не проверяя `suppliers.is_active/deleted_at`; `SendCheck` тоже не проверяет — такой черновик отправляется удалённому поставщику; (б) `CheckNoOpenPurchases` выполняется вне транзакции архива/удаления — гонка с `Create` (FOR SHARE на suppliers не спасает: проверка уже прошла). Фикс: в Restore/Send — проверка доступности поставщика (400/409); в suppliers Update/Delete — `SELECT … FROM suppliers WHERE id=$1 FOR UPDATE` + check в одной tx.
2. **[internal/features/purchase-orders/models/dto/purchase_orders_dto.go, supplier_dto.go]** Sec 15.1: enum-поля DTO (`status`, `reason` строки/подсказки, `status` кабинета) без swag-тега `enums:"…"`; enum есть только у query-`@Param` (handler/orders.go:15,247,272). Модели в `docs/swagger/swagger.json` без `enum` для этих полей; sync-теста тег↔`constants.Statuses/Reasons` нет. Правила CLAUDE.md §8.1 в проекте нет → MEDIUM, не блокер.
3. **[e2e/tests/purchase-orders.spec.ts:242–270 TS-021]** §12.1.8 на L: visual-регрессии нет — скриншот только `testInfo.attach`, без `toHaveScreenshot` baseline (TS-021 «скриншот-база»). Overflow/вьюпорты/темы проверены — блокерная часть §12.1.8 выполнена.

## Незначительные замечания (можно в следующем PR)
1. **[checks/fuzz_verdict.txt]** SPEC-MISSING — ложное срабатывание чекера (путь `frontend/e2e/fuzz` захардкожен). По существу ок: `e2e/tests/fuzz/purchase-orders_fuzz.spec.ts` есть (qty/note/supplier_*/cancel_reason; инвариант <500, 400=VALIDATION+fields | BAD_REQUEST для структурно битых тел) + Go-fuzz `validators/validators_fuzz_test.go` (4 таргета). Починить чекер (layout_resolve).
2. **[internal/features/purchase-orders/service/service.go:26–30]** Роли продублированы строками вместо `middleware.RoleAdmin/RoleExecutor/RoleSupplier` — риск рассинхрона.
3. **[web/ssr/_shared/js/purchases.js:472 `showReload`]** UI-ветка `PURCHASE_ORDER_VERSION_CONFLICT` (два сотрудника правят один заказ) не покрыта E2E; бэк — live TS-004, двухконтекстный конфликт staff↔supplier — TS-016.
4. **[internal/features/softdelete/purchase_orders_source.go:26 vs :45]** Поиск по `coalesce(s.name,'Базар')`, показ — `'Базар (сами)'`; косметика. Отдельный `POST /api/v1/purchase-orders/:id/restore` вместо «эндпоинтов раздела Удалённые» (draft-spec §6.5) — соответствует as-is паттерну проекта (общего restore у softdelete нет), ок.

## Проверено без замечаний (ключевые риски)
- **Изоляция supplier** (`internal/server/middleware/auth.go`): deny-by-default в `RequireAPI` (403) / `RequireSSR` (302 → `/supplier/orders`); allow-list: `/api/v1/supplier/*`, `POST auth/logout`, `GET auth/me`, `PUT users/me/timezone`. Fiber v3.5.0 `Use`-prefix с границей сегмента (`/supplier` ≠ `/suppliers`); регистр/trailing-slash — отказ в безопасную сторону. Live TS-009: 19 SSR + 20 API путей. Логин: `?redirect=` вне кабинета игнорируется.
- **Supplier только свои**: поставщик из сессии (`Principal.SupplierID`), SQL `supplierWhere` (`supplier_id=$1`, sent/ready/picked_up, `deleted_at IS NULL`), чужой → 404; DTO кабинета без `reason`/наших данных (DS-046).
- **Executor только свои**: `owner()` в List/Get/Suggestions + `own()` во всех HeadCheck → 404; Delete/Restore — admin-only до поиска; `created_by` из сессии.
- **Конкурентность**: шапка `FOR UPDATE OF o` во всех переходах/правке; ответ — `FOR SHARE` + строка `FOR UPDATE` + версия строки; Ready vs Recall сериализованы; qty нормализуется `FloatString(3)` = `numeric(12,3)::text` — ложных сбросов ответа нет; `UpdateCheck` отвергает чужие `items[].id`.
- **Миграция 0019**: CHECK `users_supplier_chk`, `purchase_orders_bazaar_chk`, cancel/deleted CHECK, unique `(order_id, product_id)`, sequence OWNED BY; down корректен для MVP (§CM-1).
- **Общие места**: `board/stock_repo.go` — CTE вынесен дословно (`/stock` без изменений); softdelete-вид; suppliers 409 `SUPPLIER_HAS_OPEN_PURCHASES`; `e2e/fixtures/test.ts` `uniq` — хэш теста + retry-суффикс.
- XSS: JS рендерит через `textContent`/`el()`; единственный `innerHTML` — статичная разметка модалки.
- Sec 18: списки `{items,total,limit,offset}`; nullable `supplier_id` (`*string`, без omitempty) ↔ NULL; N+1 нет (строки батчем `ANY($1)`); ошибки — `pkg/errorspkg`.
- UUID: проект as-is на `github.com/google/uuid`, `pkg/ids` отсутствует → §4.1 N/A.

## Соответствие кода дизайну
- [x] Dataflow ↔ draft-spec §5–§6 — ДА
- [x] SQL ↔ draft-spec §4 — ДА
- [x] Go-слои (handler/service/repository/validators/mappers/models/dto) — ДА
- [x] Error handling ↔ draft-spec §6 (коды, 404/403/409) — ДА
- [ ] Infrastructure — N/A
- [x] Swagger покрывает 15 эндпоинтов — ДА (enum-теги DTO — MEDIUM 2)
- [ ] Monitoring Spec / Алерты / Rollout flag / Runbook / Perf Budget — N/A (в дизайне нет; light-трек, MVP)

## Соответствие спеке (spec-track: DS/TS вместо PRD/AC)
- [ ] ADR Driver — N/A (PRD нет)
- [x] TS-001…TS-026 + TS-F1…F4 покрыты тестами — ДА
- [ ] DS-инварианты не нарушены — НЕТ: DS-045 (Блокер 1), DS-043 (MEDIUM 1); прочие ДА
- [x] Scope creep (DS-028 «потом») не реализован — ДА
- [x] §Impact: все 11 позиций «вне каталога» имеют правку в diff — ДА

## Статус фаз
- [x] `0-setup/light-progress.md` — все работы/тесты/FIX-LOOP `- [x]` — ДА
