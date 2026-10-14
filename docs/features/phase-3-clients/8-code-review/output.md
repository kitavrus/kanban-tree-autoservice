---
stage: 8-code-review
feature: phase-3-clients
tier: M
verdict: APPROVED
high: 0
medium: 0
low: 4
---
# Code Review: phase-3-clients — APPROVED
**Review level:** Standard (tier: M) — дельта-перепроверка после fix-loop #2 (коммит 1d0506f, `git diff HEAD~1 HEAD`). Sec 1,2,3,4,6,7,8,10,12,14,17; Sec 11/12.2 n/a (SSR html/template + vanilla JS, D30/D38); Sec 15 n/a (`swagger:no`); CM-1 = MVP.

Чекеры (bundle перегенерирован, не перегонялись): go_naming ✅ · api_conventions ✅ · date_bound ✅ · fuzz_verdict ✅ · live_suite ✅ · fixture_sync ✅ · e2e_mode ✅ · rule_usage ✅ · contract SKIP → n/a (TS-клиента нет). Гейты оркестратора: build/vet ✅, unit ✅, lint ✅, integration clients -race ✅; E2E затронутых спек 25/25 ✅ (fix-loop #1), полный 57/57 — `9-validation/evidence/last-run.json`.

## Статус замечаний
| Итерация | Замечание | Статус |
|---|---|---|
| #1 MEDIUM-1 | NUL/невалидный UTF-8 → 500 | ✅ закрыто (25b7784, `sqlutil.HasInvalidText` → `invalid_text`) |
| #1 MEDIUM-2 | SSR-перерисовка не нормализована | ✅ закрыто (25b7784) |
| #1 MEDIUM-3 / #2 MEDIUM-1 | поиск телефона 7↔8 + регрессия частичного ввода | ✅ закрыто (1d0506f) — см. ниже |
| #1 MEDIUM-4 | UT-010 / метки LV | ✅ закрыто (25b7784) |
| #1 LOW 23503 / TOCTOU / PRG-flash | — | ✅ закрыто (25b7784) |
| рекомендация | `invalid_text` в design-errors §Codes Fields | ✅ добавлено (1d0506f) |

Проверка фикса 1d0506f (`internal/features/clients/service.go` `searchDigits` → `(digits, digitsAlt)`; `repo.go` `phoneDigitsExpr` / `phoneAltDigitsExpr`, `$2`/`$3` через OR):
- `+7 999 555` → `7999555` / alt `999555`; хранимый `+7 (999) 555-11-22` → raw `79995551122` ✓ (и alt `9995551122` ✓) — регрессия устранена.
- `8 (999) 555` → alt `999555` ⊂ `9995551122` ✓ — пример ADR-003 «8999123 → +7 (999) 123-…» выполняется.
- `89995551122` ↔ `+79995551122` — alt `9995551122` = alt хранимого ✓; ввод без префикса (`999 111`) — raw ✓.
- Параметризация сохранена (`$1..$3`, цифры только `[0-9]`); `digits[0]` безопасен (len ≥ `minSearchDigits`).
- Тесты: UT-010 `TestSearchDigits` (10 кейсов), live `TestList_Search_LeadingCountryDigit` (7↔8 полный, частичный, регрессии `+7 704 111`/`8 (704) 111`, негатив).

## Блокеры (обязательно исправить до мержа)
Нет.

## Серьёзные замечания (исправить в этом PR)
Нет.

## Незначительные замечания (можно в следующем PR)
1. **[internal/features/clients/service.go `searchDigits`]** `digitsAlt` не проверяется на `minSearchDigits`: запрос из 3 цифр с ведущей 7/8 (`755`, `812`) даёт alt из 2 цифр (`55`, `12`) → широкая выдача (любой номер с «55» после обрезки ведущей 7/8). Результат — надмножество корректного, не потеря; фикс: `digitsAlt` только при `len(digits)-1 >= minSearchDigits` + кейс в UT-010.
2. **[internal/features/clients/ssr.go list/formEdit]** SSR-список `Limit: maxLimit` (200) без пагинации; блок оборудования карточки обрезается на 200 — known issue MVP, зафиксировать в Report.
3. **[internal/features/equipmenttypes/handler.go:67]** Проверка `is_active` в handler с ранним return — остальные ошибки полей не возвращаются вместе; known issue.
4. **[internal/features/equipmenttypes/ssr.go update]** read-then-write `IsActive: cur.IsActive` — параллельный архив может быть откатан (LWW); known issue.

## Соответствие коду дизайну (верификация реализации)
- [x] Dataflow в коде соответствует design — ДА
- [x] Все SQL queries из 3-design/design-sql.md реализованы — ДА (поиск по телефону — два OR-сравнения, EXISTS сохранён, `total` без дублей)
- [x] Go слои по 3-design/design-go-layers.md — ДА (плоский пакет — отступление Tech Spec В-1)
- [x] Error handling соответствует 3-design/design-errors.md — ДА (включая `invalid_text`)
- [x] ADR-003 (поиск по телефону) — ДА
- [x] PRG-редиректы по §SSR Routes — ДА
- [x] Infrastructure / Swagger / Vue — N/A
- [x] Monitoring / Алерты / Rollout / Runbook / Perf Budget — N/A (deferred, MVP / M-tier)

## PRD соответствие
- [x] Каждое ADR имеет Driver — ДА
- [x] AC-001…AC-014 покрыты тестами (матрица design-tests.md) — ДА
- [x] Success Metric «0 молча потерянных правок» наблюдаема (LV-020/CL-020) — ДА
- [x] Non-Goals не реализованы — ДА

## Статус фаз
- [x] Все фазы закрыты: `6-plan/code-plan.md` Phase 1–9 `- [x]` — ДА

## Итог
APPROVED: блокеров и серьёзных замечаний нет; регрессия поиска телефона устранена. 4 LOW — 1 новое (порог длины `digitsAlt`, точность выдачи) + 3 known issues MVP.
