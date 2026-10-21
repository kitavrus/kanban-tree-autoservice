# Design: E2E-стратегия — purchase-orders

**Скоуп e2e:** сценарии §11 draft-spec (TS-014…TS-026) — экраны §8.1–§8.7, «Пользователи», «Поставщики», «Удалённые».
**Режим:** Playwright, только chromium, headed, workers=1 (§TD-3); стенд с TEST_PURGE_ENABLED=true.
**Спеки:** `e2e/tests/purchase-orders.spec.ts`; POM `e2e/pages/PurchaseOrdersPage.ts`; фикстуры `e2e/fixtures/purchase-orders.ts` (TS-F1…TS-F3, авто-уборка).
**Fuzz (§TD-7):** Tier L + недоверенный ввод внешнего пользователя (поставщик).

**pw-fuzz:** да — формы ответа поставщика (supplier_qty, supplier_price, supplier_note) и строки/примечание заказа (qty, note, cancel_reason); мутаторы: границы/знаки/дроби/локаль-разделители/юникод/длины/XSS-инжекты

- pw-fuzz спека: `e2e/tests/fuzz/purchase-orders_fuzz.spec.ts` (layout проекта: testDir = e2e/tests) — инвариант: 400 с details.fields[] либо 2xx, никогда 5xx; XSS-строки на экране экранированы.
- Go-fuzz (in-process, исключение §TD-8): `internal/features/purchase-orders/validators/validators_fuzz_test.go` — без паники, инварианты границ numeric(12,3)/(12,2) и длин.
