---
stage: 8-code-review
feature: phase-8-polish
tier: S
verdict: APPROVED
high: 0
medium: 1
low: 3
---
# Code Review: phase-8-polish — APPROVED
**Review level:** Slim (tier: S) · Режим: spec-track merged (draft-spec нет → сверка с задачей + D-801…D-807 `0-setup/light-progress.md`) · CM-1 MVP

Base 902d8a6 → HEAD fe2b2ce. Чекеры bundle: 9/9 ✅. E2E-артефакт: `9-validation/evidence/last-run.json` status=passed, `e2e-run.txt` 134 passed / 0 failed (chromium, headed, workers=1).

## Блокеры (обязательно исправить до мержа)
Нет.

## Серьёзные замечания (исправить в этом PR)
1. **[web/ssr/_shared/app.css:70-73]** `--kt-on-accent` покрывает не все брендовые заливки: `.btn-outline-primary` / `.btn-outline-danger` в hover/active красят фон в `var(--bs-primary)`/`var(--bs-danger)` (#60A5FA/#F87171 в dark), а `--bs-btn-hover-color`/`--bs-btn-active-color` остаются дефолтным Bootstrap `#fff` → белый текст ~2.5:1 — тот же класс, что D-804 чинит. Затронуты «Удалить»/«Убрать»/«×» в карточке (`task-card.js:408,606,643`), `board.html:105,146`, `products-list.html:41`, `services-list.html:41`. Фикс: добавить `--bs-btn-hover-color: var(--kt-on-accent); --bs-btn-active-color: var(--kt-on-accent);` в оба правила (в light остаётся #fff — визуально без изменений).

## Незначительные замечания (можно в следующем PR)
1. **[web/ssr/_shared/app.css:334]** `#task-modal .modal-body .d-flex > div { min-width: 0; }` — вне media-query, действует и на десктопе на любой div-ребёнок d-flex в модалке; сейчас без видимых регрессий (PL-003 @1280 зелёный), но селектор широкий — при новых блоках возможно сжатие обёрток кнопок. Сузить до конкретных строк (журнал/товары) при следующей правке.
2. **[web/ssr/_shared/print.css:68]** `.print-page table { overflow-wrap: anywhere; }` вне `@media screen` — действует и при печати; `anywhere` снижает min-content ширину колонок auto-layout таблиц, возможны переносы посреди слова на бумаге. Рассмотреть перенос в `@media screen and (max-width: 576px)` или `break-word`.
3. **[e2e/tests/polish.spec.ts:30-42]** `bodyIsDark`/`expectNoPageOverflow` — одноразовый `page.evaluate` без auto-retry (не web-first). Корректно, т.к. тема ставится init-скриптом до CSS и замер идёт после `toBeVisible`, но при появлении асинхронной вёрстки — кандидат на `expect.poll`. `waitForTimeout` нет ✓.

## Проверено по фокусам оркестратора
- **--kt-on-accent / светлая тема:** `:root { --kt-on-accent: #fff }` = прежний хардкод → light без изменений (white/#2563EB ≈5.2:1, white/#DC2626 ≈4.8:1). Dark: #0B1220 на #60A5FA ≈7.4:1, на #F87171 ≈6.8:1 — ≥4.5:1 ✓.
- **Глобальные переопределения:** `.btn-primary` — только component-vars (Bootstrap CSS Variables API), bg/border не менялись. `.text-bg-danger` override скоуплен `[data-bs-theme="dark"]`, единственный потребитель — бейдж `nav-badge-stock` (`base.html:23`); `!important` + бо́льшая специфичность корректно перебивает Bootstrap. `.btn-danger` (`ui-dialog.js:71`) не брендирован (статичный #dc3545 + #fff) — не затронут. Pagination в шаблонах не используется.
- **Модалка/топбар:** правила скоуплены `#task-modal` / `@media (max-width:576px)`; `margin-left:0 !important` нужен против `ms-3`; `:empty{display:none !important}` перебивает `d-flex` (id-специфичность), JS чистит через `innerHTML=''` → `:empty` срабатывает. Десктоп: `flex-wrap` шапки не меняет раскладку при достаточной ширине (PL-003 @1280 ✓). Замечание LOW-1.
- **print.css / акты светлые:** `layouts/print.html:14` `<body data-bs-theme="light">`, app.css/theme-скрипт не подключены → токены body светлые в любой теме; PL-004 проверяет фактический фон в обеих темах ✓.
- **Перенос sentinel:** `errorspkg.ErrRequestTooLarge` = `New("REQUEST_TOO_LARGE", http.StatusRequestEntityTooLarge, "Слишком большой запрос")` — код/статус/текст идентичны удалённому `middleware.ErrRequestTooLarge`. Остаточных ссылок на старый символ в коде/тестах нет (`git grep`). `pkg/errorspkg` — лист, цикла нет; зависимость feature→server-слой ради ошибки устранена (закрывает замечание `phase-7-attachments/8-code-review/output.md:33`). Семантика `err` в defer `attachments_handler.go:66` не изменилась.
- **E2E PL-001…011:** матрица тема×{360,1280} (D-801), фон body + `data-bs-theme` (D-802), overflow `.modal-header/.modal-body` (D-803); overflow = `scrollWidth ≤ clientWidth` ✓; `test.use({colorScheme:'light'})` исключает влияние system-темы; данные через `adminApi`/`uniq`, заказы — autouse `_boardCleanup`, справочники не удаляются (D-806, конвенция прочих спек).

## Slim-секции
- **Security:** новых endpoint'ов/биндинга/ввода нет; 413 по-прежнему через `errorspkg` wire-формат, `SetConnectionClose` сохранён (`body_limit.go:63`). HTML/JS не менялся (D-807) → XSS-поверхность без изменений. ✓
- **Architecture fit:** sentinel в `pkg/` (2 потребителя: middleware + board) ✓.
- **Error Handling:** единый `errorspkg`, поведение 413 неизменно ✓.
- **Tests:** 11 E2E (happy по всем SSR-экранам + adaptive/theme инварианты); backend — поведение неизменно, существующие integration middleware/board зелёные по гейту Executing. ✓
- **Rollback / breaking:** миграций/API-изменений нет; экспортируемый `middleware.ErrRequestTooLarge` удалён без алиаса — внешних потребителей нет, CM-1 MVP ✓.

## Sec 18 — API-контракт (слитая линза)
N/A по сути: контракты не менялись; error-body 413 — формат `errorspkg` (класс H-01 не затронут), код `REQUEST_TOO_LARGE` тот же.

## Соответствие коду дизайну
- Решения D-801…D-807 реализованы в diff — ДА
- Error handling (код/статус/текст sentinel) неизменен — ДА
- Прочие пункты (SQL/Go-слои/Swagger/Monitoring/Rollout/Perf) — N/A (S-tier, spec-track, без backend-фич)

## Статус фаз
- Все пункты чеклиста `0-setup/light-progress.md` `- [x]` — ДА
