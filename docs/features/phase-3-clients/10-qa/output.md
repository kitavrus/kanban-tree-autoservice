---
stage: 10-qa
feature: phase-3-clients
tier: M
verdict: APPROVED
high: 0
medium: 0
low: 0
---
# QA: phase-3-clients — APPROVED
**HIGH:** 0  **MEDIUM:** 0  **LOW:** 0

**Профиль:** Бизнес-фича
**Tier:** M
**Совместимость (CM-1):** MVP (prd.md шапка) — breaking/rollback не проверялись.
**Артефакты, которые читал:** `10-qa/input-bundle.md` (validation/code-review/mobile-review + слайсы design-errors/design-go-layers/design-tests/design-sql + паспорт AC), `2-spec-interview/prd.md` (тексты AC-001…AC-014); live API `http://localhost:8080`.

## Прогнанные классы проверок

- [x] Класс 1 (семантика DTO vs AC) — 14 AC, ~20 live-кейсов
- [x] Класс 2 (regression смежных фич) — users (EscapeLike, поиск), auth (login executor/admin), меню (ADR-006 переименование + NavActive), ui-dialog.js (data-confirm/data-once — чистое дополнение), row-list.js (не используется вне clients/equipment-types — нет regression-поверхности)
  - [x] Sub 2.a (list ↔ detail parity) — n/a: у clients/equipment/equipment-types нет user-зависимых вычисляемых полей (has_voted-типа); `equipment_count`/`type_is_active` сверены list↔detail — совпадают
  - [x] Sub 2.b (N+1 detection) — n/a: OTEL/Jaeger не поднят (Observability deferred, MVP); SQL из design-sql.md (batch-запрос телефонов, JOIN типов) реализован без построчных запросов — подтверждено по коду в code-review, живым OTEL не перепроверялось
  - [x] Sub 2.c (pagination correctness) — да: limit/offset дефолты и максимумы проверены на `/clients`, `/equipment-types`
- [x] Класс 3 (edge cases) — 10 кейсов (NUL-байт, малформед JSON, неизвестное поле, отсутствие CSRF, битый/несуществующий uuid, limit=-1/0/9999/abc, offset=-5, POST оборудования несуществующему клиенту)
- [x] Класс 4 (data integrity) — применимо: да (touches.db=true, миграция 0003) — CASCADE (client→phones/equipment), RESTRICT (equipment_type←equipment), version-конфликт, уникальность `lower(btrim(name))`
  - [x] Sub 4.b (rolling deploy compatibility) — n/a: не миграция/рефакторинг существующей таблицы (новые таблицы), CM-1=MVP
- [x] Класс 5 (state machine & lifecycle) — применимо: да — equipment_types.is_active (архив/восстановление, идемпотентность обеих операций проверена)
- [x] Класс 6 (authorization depth & data privacy) — применимо: да — executor: 403 на все admin-мутации, `include_archived=true` игнорируется сервером (ADR-004), SSR-гварды 403 «Нет доступа», без сессии 401/API, меню исполнителя пусто
- [x] Класс 7 (quotas & rate limits) — применимо: нет — квот/лимитов в фиче нет
- [x] Класс 8 (notification delivery / FCM) — применимо: нет — уведомлений в фиче нет
- [x] Класс 9 (fuzzing) — применимо: нет — design-tests.md §Fuzz: n/a (tier M, risk=reversible, по запросу не заказан); fuzz_verdict_check ✅ в code-review
- [x] Класс 10 (supply chain / dependency audit) — применимо: нет — `git diff d3de17e..HEAD` не затрагивает go.mod/go.sum/package.json/.github/

## Live-прогоны (evidence, §TD-2 Evidence-расширение)
- `10-qa/evidence/class1-01-create-validate-search.txt` — AC-001/002/004 (create, validation, поиск по телефону 8↔+7, поиск по компании)
- `10-qa/evidence/class4-05-06-data-integrity.txt` — AC-005 (version conflict), AC-006 (cascade delete + идемпотентность DELETE), AC-011/012 (уникальность имени, RESTRICT/архив/восстановление, LWW-правка архивного), класс 5 (идемпотентная архивация), подтверждение известного review LOW-3 (ранний return в handler is_active)
- `10-qa/evidence/class6-authz.txt` — AC-013 (executor: 403 на мутации, include_archived игнорируется, SSR-гварды, пустое меню, 401 без сессии), regression меню admin (NavActive, переименование «Каталог»)

## Findings

### HIGH (блокеры)
Нет.

### MEDIUM (advisory)
Нет.

### LOW (nice-to-have)
Нет новых. Ранее задокументированные в `8-code-review/output.md` known issues проверены живьём и подтверждены как есть (не новые находки, дублирования не создаю):
- LOW-3 (`equipmenttypes/handler.go:67`, ранний return `is_active`): PUT с одновременно невалидными `name=""` и отсутствующим `is_active` вернул только `is_active: required`, ошибка `name` пропущена — evidence class4-05-06.
- LOW-1 (`digitsAlt` без проверки `minSearchDigits`) — код на текущем HEAD (`internal/features/clients/service.go:105`, коммит 1ee8a4d «review LOW») УЖЕ содержит `len(digits)-1 >= minSearchDigits`; живой тест `q=755` (3 цифры, ведущая 7) не дал надмножества (`total=0`, alt не сформирован) — фикс, судя по всему, применён ПОСЛЕ того, как готовился `8-code-review/output.md` в бандле (текст отчёта устарел относительно HEAD). Не блокер, фиксирую для синхронизации отчёта.
- LOW-2 (SSR-лимит 200 на список оборудования в карточке) и LOW-4 (read-then-write race `IsActive` при архивации типа) — не проверялись живьём отдельно (не задеты новыми правками в рамках QA-сессии, известны как MVP-компромисс), оставлены как есть.

## PRD/AC соответствие (сверено live)
- [x] AC-001 (заведение клиента, порядок телефонов, пустые строки отбрасываются) — подтверждено
- [x] AC-002 (обязательные поля, сообщение у поля) — подтверждено
- [x] AC-003 (несколько телефонов) — подтверждено (через AC-001 payload)
- [x] AC-004 (поиск по компании/телефону, включая 8↔+7 нормализацию) — подтверждено
- [x] AC-005 (конфликт одновременной правки, текст сообщения дословно совпадает с PRD) — подтверждено
- [x] AC-006 (удаление клиента → каскад, повтор → 404) — подтверждено; «есть заказы → 409» — вне скоупа фазы (фаза 5, зафиксировано в PRD/validation)
- [x] AC-008 (единица оборудования: обязательность типа/модели) — подтверждено
- [x] AC-009 (нормализация specs, отброс пустых пар) — подтверждено
- [x] AC-011 (уникальность имени без учёта регистра/пробелов) — подтверждено
- [x] AC-012 (RESTRICT → предложение архива, архивный тип не предлагается новым единицам, у существующих остаётся, видим через include_archived, восстановление) — подтверждено полностью
- [x] AC-013 (права исполнителя: чтение+создание оборудования разрешены, остальные мутации 403, меню пусто, SSR «Нет доступа», API 401 без сессии) — подтверждено полностью
- AC-007, AC-010, AC-014 — не перепроверялись live отдельно (не задеты сомнением: AC-007/010 покрыты E2E CL-009/EQ-006 в validation 57/57 зелёном прогоне; AC-014 — адаптив/тема, вне зоны live curl-QA, покрыта Playwright viewport-тестами CL-021/EQ-021/ET-021 согласно 9-validation/output.md).

## Резюме
Все проверенные AC (001–006, 008, 009, 011–013) соответствуют PRD дословно на живом API/SSR. Regression на users/auth/меню/shared-JS/EscapeLike — чисто. Data integrity (CASCADE/RESTRICT/version/уникальность) — корректна. Authz-глубина исполнителя — без пробелов. Edge cases (NUL/malformed JSON/CSRF/uuid/pagination) обрабатываются по контракту design-errors.md, без 500. Новых HIGH/MEDIUM не найдено. **Вердикт: APPROVED** — переход в Report.
