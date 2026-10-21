---
stage: 8-code-review
feature: purchase-orders
tier: L
verdict: APPROVED
high: 0
medium: 1
low: 5
---
# Code Review: purchase-orders — APPROVED
**Review level:** Full (tier: L; Режим: spec-track merged, Sec 18 включена; трек light) — цикл 2, HEAD 09d3143 (фикс 11f5d93)

Pre-condition (повторное судейство оркестратора, не перегонялось): build ok; go test 22 ok; lint 0; integration-scoped 25 ok; e2e full 213/213, 0 flaky; swagger актуален.

## Закрытие замечаний цикла 1
| # | Было | Статус | Проверено |
|---|---|---|---|
| HIGH-1 | `users/service.go` — блокировка учётки архивного поставщика → 400 | ЗАКРЫТО | `checkSupplier` только при `in.Role==supplier && (before.Role!=supplier \|\| before.SupplierID!=in.SupplierID)`; `SupplierID` нормализуется `ToLower` (сравнение с `uuid::text`); live `TestBlockAccountOfArchivedSupplier` |
| MEDIUM-1 | DS-043 обходился Restore/Send/гонкой | ЗАКРЫТО | `Restore` — tx: заказ `FOR UPDATE` → поставщик `FOR SHARE` → 409 `PURCHASE_ORDER_SUPPLIER_UNAVAILABLE`; `lockHead` берёт поставщика `FOR SHARE`, `SendCheck` → 400 `supplier_id: unavailable`; suppliers Update(архив)/SoftDelete — `lockNoOpenPurchases` (`FOR UPDATE` строки поставщика + проверка) в одной tx; race-тесты `TestRace_CreateWaitsForArchive`, `TestRace_ArchiveWaitsForCreate` |
| MEDIUM-2 | enum-теги DTO | ЗАКРЫТО | `enums:` на `Status`/`Reason` (7 полей); `models/dto/enums_test.go` `TestSwagEnumsMatchConstants` сверяет с `constants` |
| MEDIUM-3 | visual-регрессия TS-021 | НЕ СДЕЛАНО (known issue оркестратора) | см. MEDIUM 1 ниже |
| LOW | роли-строки, «Базар (сами)» | ЗАКРЫТО | `service.Role* = middleware.Role*`; поиск `coalesce(s.name,'Базар (сами)')` |

### Дедлоки (порядок блокировок поставщик ↔ заказ) — не найдено
- Переходы/Answer/Ready/Restore: заказ (`FOR UPDATE`/`FOR SHARE OF o`) → поставщик `FOR SHARE`.
- Create: поставщик `FOR SHARE` → INSERT нового заказа (на существующие заказы не блокируется).
- Архив/удаление поставщика: поставщик `FOR UPDATE` → чтение `purchase_orders` БЕЗ блокировки → UPDATE suppliers. Строки заказов не блокируются → цикла «заказ↔поставщик» нет; конкурент только ждёт.
- `FOR SHARE` ↔ `FOR SHARE` (разные заказы одного поставщика) совместимы; обычная правка поставщика (`is_active=true`) ждёт короткие tx закупок — ожидание, не дедлок.

## Блокеры
Нет.

## Серьёзные замечания
1. **[e2e/tests/purchase-orders.spec.ts:242–270 TS-021]** §12.1.8 на L: нет `toHaveScreenshot` baseline (только `testInfo.attach`). Overflow/вьюпорты/темы проверены. Принято оркестратором как known issue — вынести в долги отчёта.

## Незначительные замечания
1. **[pkg/errorspkg/purchase_orders.go:23,73 `PURCHASE_ORDER_SUPPLIER_UNAVAILABLE`]** Нового кода нет в draft-spec (§6 / таблица ошибок) и в swag `@Failure` restore (handler/orders.go:222–234). Поведение — следствие DS-043, не scope creep; дописать to-be DS + строку ошибки (§6.5 / «Удалённые») и `@Failure 409`.
2. **[service/service.go:280 `SendCheck`]** Отказ отправки при недоступном поставщике — 400 VALIDATION с `details.fields[supplier_id]` на действии без формы; UI покажет общий текст. Рассмотреть 409 с понятным сообщением (как у restore). Спекой не задано.
3. **[handler/purchase_orders_http_live_test.go:1081]** Race-тесты ждут по `count(*) FROM pg_stat_activity WHERE wait_event_type='Lock'` по всей БД — при параллельных пакетах на общей БД возможен ложный сигнал; фильтровать по `query ILIKE`/`application_name`.
4. **[web/ssr/_shared/js/purchases.js:472]** UI-ветка `PURCHASE_ORDER_VERSION_CONFLICT` без E2E (known issue; бэк — live TS-004).
5. **[checks/fuzz_verdict.txt]** SPEC-MISSING — ложное срабатывание чекера (путь `frontend/e2e/fuzz`); спека `e2e/tests/fuzz/purchase-orders_fuzz.spec.ts` + Go-fuzz есть (known issue).

## Проверено без замечаний (цикл 1, без регрессий в фиксе)
- Изоляция supplier: deny-by-default в `RequireAPI` (403) / `RequireSSR` (302 → `/supplier/orders`), узкий allow-list; Fiber v3.5.0 prefix с границей сегмента; TS-009 (39 путей).
- Supplier только свои (поставщик из сессии, SQL-фильтр, 404); DTO кабинета без `reason` (DS-046).
- Executor только свои (`owner()`/`own()` → 404); Delete/Restore — admin-only.
- Конкурентность: версии заказа/строки, блокировки шапки; qty нормализована под `numeric(12,3)::text`.
- Миграция 0019 (CHECK, unique, sequence; down — MVP §CM-1).
- Общие места: `board/stock_repo.go` CTE без изменения `/stock`; softdelete-вид; `e2e/fixtures/test.ts` `uniq`.
- XSS: `textContent`; Sec 18: конверт списков, nullable `supplier_id`, без N+1, ошибки — `pkg/errorspkg`.

## Соответствие кода дизайну
- [x] Dataflow / SQL / Go-слои / ошибки ↔ draft-spec §4–§6 — ДА (код ошибки — LOW 1)
- [x] Swagger: 15 эндпоинтов + enum-теги DTO — ДА
- [ ] Infrastructure / Monitoring / Alerts / Rollout / Runbook / Perf Budget — N/A (в дизайне нет; light, MVP)

## Соответствие спеке (DS/TS)
- [x] TS-001…TS-026 + TS-F1…F4 покрыты — ДА
- [x] DS-инварианты (вкл. DS-043, DS-045) не нарушены — ДА
- [x] Scope creep (DS-028) нет — ДА
- [x] §Impact: 11 позиций «вне каталога» — правки в diff — ДА

## Статус фаз
- [x] `0-setup/light-progress.md` — все пункты `- [x]` — ДА
