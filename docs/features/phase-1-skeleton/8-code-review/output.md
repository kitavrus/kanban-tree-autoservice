---
stage: 8-code-review
feature: phase-1-skeleton
tier: S
verdict: APPROVED
high: 0
medium: 0
low: 3
---
# Code Review: phase-1-skeleton — APPROVED
**Review level:** Slim (tier: S) · Режим: spec-track merged (+Sec 18) · re-review после фикса 10c665d (дельта f744f41..10c665d) · база бандла 7b4dd9b
**Источник истины:** docs/plan.md §1, §6 строка фазы 1, D30/D31/D38. draft-spec/PRD нет.

## Блокеры (обязательно исправить до мержа)
— нет.

## Серьёзные замечания (исправить в этом PR)
— нет.

## Незначительные замечания (можно в следующем PR)
1. **[internal/ssr/render.go:20, internal/server/router.go:68]** Пути шаблонов/статики — от корня модуля (CWD). Задокументировано в render.go; для образа — зафиксировать `WORKDIR` на этапе деплоя. Принято осознанно.
2. **[web/ssr/_layout/templates/layouts/base.html:36]** Форма выхода `POST /logout` — роута нет (есть `POST /api/v1/auth/logout`, JSON). В фазе 1 блок скрыт (нет viewer) → фаза 2.
3. **[internal/features/auth/handler.go:34-41 — генератор c9416a2]** cookie `sid` без `Secure`, нет rate limit на login; CSRF/роли/sliding TTL (D35) — задачи фазы 2.

## Повторная проверка находок первого раунда
| # | Было | Статус | Пруф |
|---|---|---|---|
| HIGH | обход `Next`-фильтра static-маунта → отдача исходников шаблонов | ✅ fixed | `web/ssr/_shared` теперь содержит только `app.css`, `favicon.svg`, `js/`, `vendor/` — шаблонов под корнем статики нет; layout → `web/ssr/_layout/templates` (`ssr.SharedRoot`); фильтр `Next` удалён (`router.go:68`). Выход за корень статики fasthttp санитизирует; `TestTemplatesNotExposed` (`shell_http_live_test.go:79-95`) шлёт пути байт-в-байт через `URL.Opaque`: `%74emplates`, `//templates`, `js/../templates`, `/ASSETS/...`, `../_layout`, `..%2f_layout`, `%2e%2e/_layout`, `../home` → не 200 и без `{{ define`; TestShellLive 4/4 (гейт оркестратора). |
| MEDIUM | z-scaffold совместим только по сигнатурам, layout-контракт расходится с D38 | ✅ fixed | рецепт из 3 шагов в doc-комментарии `internal/ssr/render.go:12-18` (удалить feature `layouts/base.html`, `Shell ssr.Shell` в `BaseVM` + `h.Shell.Build`, `NewRenderer(root, ssr.SharedRoot)` + путь в `NewShellBuilder`); D38 в plan.md переформулирован. Сверено с `~/.claude/templates/z-scaffold/ssr/layouts/base.html.tmpl` / `render.go.tmpl` — рецепт полный. |
| LOW | `label[for=...]` без экранирования | ✅ fixed | `combobox.js:44` — `CSS.escape(sel.id)`. |
| LOW | `highlight` смещается при смене длины `toLowerCase` | ✅ fixed | `combobox.js:20` — при несовпадении длин подсветка отключается, текст через `esc`. |
| LOW | нет XSS-E2E для комбобокса | ✅ fixed | SH-022 (`e2e/tests/shell.spec.ts:196-217`): HTML в option/`data-hint`/`optgroup label` → `img, b` = 0, `__xss` undefined, кнопка — текстом. |
| LOW | `.golangci.bck.yml` в рабочем дереве | ✅ fixed | файла нет. |
| LOW | CWD-пути / logout-форма / генератор auth | принято / фаза 2 | см. LOW 1–3 выше. |

## Проверено — без замечаний (первый раунд, дельтой не затронуто)
- XSS: `ui-dialog.js` — только `textContent`; `combobox.js` — всё через `esc`; html/template — контекстное экранирование, `TestRenderer_EscapesData` ✅.
- `api.js` ↔ `pkg/errorspkg` wire-формат ✅; 500 без раскрытия внутренностей ✅.
- E2E: без `waitForTimeout`/`sleep`, POM + auto-fixture чистоты консоли/сети, только `chromium`, `workers: 1` ✅; адаптив (adaptive:min) SH-030/031, темы SH-002/003 ✅.
- Архитектура: `internal/ssr` общий, `home` без логики, регистрация в `internal/server/router.go` ✅. CM-1 = MVP → rollback/breaking не флагаются.

## Sec 18 — API-контракт
Новых JSON-эндпоинтов в ручной части нет (`GET /` — HTML). `api_conventions` ✅; `contract` SKIP-RESOLVE → n/a (TS-клиента нет по D30).

## Соответствие коду дизайну (plan.md §1/§6 фаза 1, D30/D31/D38)
- [x] Состав фазы 1 (§6) — ДА
- [x] D30 (SSR + vanilla JS) — ДА
- [x] D31 (MVP; swagger:no / adaptive:min / themes:both) — ДА; Sec 15 = n/a
- [x] D38 (разметка совместима, подключение по рецепту) — ДА
- [x] Шаблоны не отдаются наружу — ДА (структурно)
- Monitoring/Alerts/Rollout/Runbook/Perf Budget — N/A (S-tier)

## Детерминированные вердикты (bundle HEAD 10c665d)
go_naming ✅ · api_conventions ✅ · date_bound ✅ · fuzz_verdict ✅ · live_suite ✅ · fixture_sync ✅ · e2e_mode ✅ · rule_usage ✅ · contract ⚠️ SKIP-RESOLVE → n/a.
Гейты после фикса (оркестратор): go vet ✅ · unit ✅ · golangci-lint 0 ✅ · TestShellLive 4/4 ✅ · E2E полный 11/11 (`--retries=1`, 0 flaky) ✅ (`9-validation/evidence/last-run.json`).

## Статус фаз
- [x] /pipeline-light, одна фаза; тесты зелёные по артефактам — ДА
