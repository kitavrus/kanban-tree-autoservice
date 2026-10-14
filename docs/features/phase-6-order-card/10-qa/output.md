---
stage: 10-qa
feature: phase-6-order-card
tier: M
verdict: APPROVED
high: 0
medium: 0
low: 1
---
# QA: phase-6-order-card — APPROVED
**HIGH:** 0  **MEDIUM:** 0  **LOW:** 1

**Профиль:** Бизнес-фича
**Tier:** M
**Артефакты, которые читал:** `10-qa/input-bundle.md` (validation + code-review + mobile-review-вердикт + слайсы design-errors/design-go-layers/design-tests/design-sql + паспорт AC-001…017); live API `http://localhost:8080` (HEAD `2766a0f`).

## Прогнанные классы проверок

- [x] Класс 1 (семантика DTO) — 9 кейсов (итог заказа/услуги/строки, «по факту», rounding, edge qty) — evidence/class1-totals.txt
- [x] Класс 2 (regression смежных фич) — board list (`GET /api/v1/tasks`), SSR `/`, description_html (AC-002) — evidence/class2-regression-description.txt
  - [x] Sub 2.a (list ↔ detail parity) — n/a: `total_amount`/`last_log` — единственное имя итога и в списке, и в карточке (design §Card Queries), расхождению неоткуда взяться; проверено что оба совпадают на живом заказе
  - [x] Sub 2.b (N+1 detection через OTEL) — применимо: нет — Jaeger-стенд не поднят на дев-окружении (то же ограничение, что в 9-validation §Observability); статически 7 запросов без N+1 подтверждены Code Review
  - [x] Sub 2.c (pagination correctness) — применимо: нет — карточка/журнал без пагинации (весь состав заказа в одном ответе, design-go-layers.md); board list — as-is лимит, не тронут фазой
- [x] Класс 3 (edge cases) — 13 кейсов (qty 0/-1/строка/дробный/огромный, 404×5, garbage body×3, идемпотентность DELETE/POST equipment) — evidence/class3-invalid-input.txt
- [x] Класс 4 (data integrity) — touches.db=true — инвариант склада на серии add/qty-change/delete + миграция 0006 применена (используется живым стендом) — evidence/class4-stock-invariant.txt
  - [x] Sub 4.b (rolling deploy compat) — применимо: нет — не миграция/рефакторинг-профиль (ADR-103/106 — обычная фича)
- [x] Класс 5 (state machine) — 5 переходов (SUBTASKS_NOT_DONE, SUBTASK_FINAL_COLUMN×2, done→done) — evidence/class5-state-machine.txt
- [x] Класс 6 (authorization depth & privacy) — 11 сценариев (equipment IN_USE/foreign_client/not_in_order, log forbidden/own/admin-bypass, FORBIDDEN/PAID_ADMIN_ONLY, 401/CSRF_INVALID, XSS×2+print) — evidence/class6-authz.txt
- [x] Класс 7 (quotas & rate limits) — применимо: нет — фаза не вводит quota/rate-limit полей
- [x] Класс 8 (FCM/notifications) — применимо: нет — фаза не создаёт notifications/device_tokens
- [x] Класс 9 (fuzzing Go native) — применимо: 3 таргета (`FuzzSubtaskUpdateBody`, `FuzzSubtaskProductBody`, `FuzzSubtaskLogAndEquipmentBody`), 30s каждый, go-fuzz crashes: 0; pw-fuzz — n/a (tier M, risk reversible, per prompt) — evidence/class9-go-fuzz.txt
- [x] Класс 10 (supply chain) — govulncheck ./internal/features/board/...: 0 reachable vuln; go.mod/go.sum не менялись; e2e/package.json — только удаление `pg`/`@types/pg` (ADR-101) — evidence/class10-supply-chain.txt

## Findings

### HIGH (блокеры)
Нет.

### MEDIUM (advisory)
Нет.

### LOW (nice-to-have)

#### [L-01] Класс 10: npm audit не запускался
**Класс:** 10
**Локация:** `e2e/package.json`
**Обоснование:** diff фазы удаляет `pg`/`@types/pg`, новых пакетов не добавляет — риск низкий; `npm audit --omit=dev` не выполнялся (нет добавленных зависимостей, требующих аудита). Зафиксировано как процессный пробел, не блокер.
**Рекомендация:** при следующей фазе, добавляющей e2e-зависимости, прогнать `npm audit --omit=dev --audit-level=high` явно.

## Резюме

Все 17 AC (AC-001…AC-017) воспроизведены живьём на реальном стенде (создание заказа, оборудование
+ owner-проверки, услуги из каталога + комплект, подсказка наличия, списание/возврат склада на
серии правок, финальная колонка блокирует create/revert услуги, журнал с authz (own/admin-bypass/
forbidden), итоги (в т.ч. «по факту» и rounding), печать актов (реквизиты, экранирование XSS,
no-store), права executor (нет delete/paid, есть остальное)) — расхождений с
design-errors.md/design-go-layers.md/design-tests.md/design-sql.md не найдено. Инвариант склада
`stock_after = stock_before + Σ delta` подтверждён на полной серии add→qty↑→qty↓→delete.
State machine (SUBTASKS_NOT_DONE / SUBTASK_FINAL_COLUMN) и authz (TASK_EQUIPMENT_IN_USE,
foreign_client, SUBTASK_LOG_FORBIDDEN, PAID_ADMIN_ONLY, FORBIDDEN, CSRF_INVALID, 401) — код-в-код
с design-errors.md §Codes 16-22/Reused Codes. XSS во всех проверенных точках (журнал, комплектность,
описание) корректно экранирован на выходе в HTML (JSON — сырой текст, как и задумано). Go-fuzz
(3 таргета × 30s, >2.5M execs суммарно) — 0 крашей. govulncheck на изменённом пакете — 0 reachable
vulnerabilities. Regression на board list/SSR — не сломан, новые поля (`total_amount`, `last_log`,
`description_html`) присутствуют без побочных эффектов на старые поля.

HIGH=0, MEDIUM=0 → **APPROVED**. Единственная запись — LOW-advisory по непокрытому npm audit
(не блокирует релиз, не относится к функциональности фазы). Тестовые данные (`qa-oc-*`) убраны
после прогона (заказ/клиенты/оборудование/услуги удалены; 2 товара архивированы — RESTRICT
`PRODUCT_IN_USE`, как и задумано, hard delete невозможен после stock-движений).
