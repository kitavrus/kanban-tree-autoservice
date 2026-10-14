
## 2026-09-26 phase-3-clients (tier M, бизнес-фича, /pipeline)
- Циклы: design-review-loop 1 (mobile HIGH: один код 409 на два случая имени типа → EQUIPMENT_TYPE_NAME_ARCHIVED); artifact-lint:1-research 1 (нет file:line у фактов, 42KB>30KB); fix-loop фаза 08 ×1 (EQ-004: фильтр «заполненных» характеристик по k||v вместо v); code-review fix-loop ×2 (NUL/UTF-8 → 500; индексы ошибок строк формы; поиск телефона ADR-003 — фикс внёс регрессию частичного ввода, закрыта 2-й итерацией); watch-gate 1 (CL-002 таймаут от slow-mo → timeout 180 с при PWSLOWMO).
- Report-verifier/evidence: срабатываний нет; все гейт-отчёты PASS линта.
- Тайминг: чистое ~5ч 20м / Executing 3ч 38м и Code Review 43м — топ; ожидание пользователя ~11.5ч (план и watch-gate между сессиями).
- Наблюдения: без draft-spec бандлы не несут тексты AC и plan.md/prototype (7 warn-нарушений контракта); stage_bundle не подхватывает Tier/CM из PRD (паспорт «?»); Postgres ON DELETE RESTRICT → SQLSTATE 23001, не 23503 (design-sql ошибся).
- Кандидат в доктрину: нет (паттерны не повторились в ≥2 записях).

## 2026-09-27 phase-5-board (tier M, бизнес-фича, /pipeline)
- Циклы: artifact-lint:1-research 1 (нет file:line у 22 фактов, 61KB>30KB); design-review-loop 1 (swimlane выдумал коды INCOMPLETE_SUBTASKS и поля DTO; mobile MEDIUM — один FORBIDDEN на два случая → PAID_ADMIN_ONLY); seed-demo fix ×1 («в стиле прототипа» вместо данных прототипа); фаза 06 fix-loop ×2 (фикстура клиента без phones; combobox закрывался от авто-прокрутки при клике); сводный гейт fix-loop ×1 (AU-005: bfcache показывал доску после выхода → no-store в RequireSSR; попутно — серверный поиск клиентов вместо выгрузки справочника); code-review fix-loop ×2 (HIGH последняя нефинальная через PUT; MEDIUM порог ребалансировки scale>20 недостижим, version после PAID_ADMIN_ONLY, финальная колонка с открытыми услугами; регрессия BD-006 — гард по обрезанному limit=500 списку).
- Report-verifier/evidence: срабатываний нет; все гейт-отчёты PASS линта; служебный .claude/pipeline-stage-contract.json попал в коммит (git add -A при активном контракте) → удалён, добавлен в info/exclude.
- Тайминг: total 24ч 58м / чистое 5ч 44м; топ — Executing 3ч 54м, Code Review 35м; переделки 2ч 14м; ожидание пользователя 19ч 14м (между сессиями).
- Наблюдения: phase-bundle не пришивает design-tests.md (5 warn-нарушений — исполнители читали его напрямую); разрастание dev-БД от E2E ломает тесты, завязанные на первую страницу списков (US-004, BD-006, uniq() коллизии) — E2E должны фильтровать по своим сущностям.
- Кандидат в доктрину (повтор ≥2): (1) research стабильно превышает 30KB и теряет file:line (фазы 3 и 5) — ужать шаблон research-агента; (2) бандлы без draft-spec теряют нужные секции (фаза 3: AC/plan.md; фаза 5: design-tests в phase-bundle) — правило в stage_contracts.

## 2026-09-27 phase-6-order-card (tier M, бизнес-фича, /pipeline; с design-review — автономный режим пользователя)
- Циклы: design-review-loop 1 (swimlane снова выдумал поля DTO — при явном запрете; правил design-агент как владелец фактов; mobile H-01 self-race автосохранения → ADR-105 rowQueue); фаза 03 fix ×1 (флаки конкурентных IT: ассерт «ни одной ошибки» вместо инвариантов); фаза 05 fix ×1 (BD-006: O(cards×columns) меню «Переместить в…» вешало рендер + лимит колонок); сводный гейт fix ×1 (изоляция catalog/users E2E от объёма БД, лимит каталога 2000); code-review fix ×2 (HIGH спаны не по дизайну; MEDIUM гонки ответов карточки/шапка/коды 404; LOW seq-guard окно).
- Report-verifier/evidence: срабатываний нет; 20 warn-нарушений контракта (исполнители читали design-* мимо phase-bundle — бандл не пришивает design-tests/§Frontend Modules; агенты брали фазу 5 как образец).
- Тайминг: total 5ч 48м / чистое 5ч 40м; топ — Executing (~65%), Code Review 42м; переделки ~56м.
- Наблюдения: dev-стенд, общий для E2E и ручной проверки, раздут прогонами (сотни колонок/категорий/заказов) — порождает класс «тест не находит своё на первой странице» и лимит-заплатки в продукте.
- Кандидат в доктрину (повтор ≥2: фазы 5 и 6): (1) swimlane-designer выдумывает коды/поля DTO — сверка swimlane с design-errors/design-go-layers должна быть детерминированным чекером, а не на совести агента; (2) phase-bundle не пришивает design-tests.md/§Frontend Modules — правило в stage_contracts; (3) E2E на общем dev-стенде без уборки/изолированной БД → отдельная e2e-БД или обязательный teardown в фикстурах.

## 2026-09-27 phase-7-attachments + phase-8-polish (tier S, /pipeline-light, автономный режим)
- Фаза 7: судейство fix ×1 (ui-dialog.js не singleton — два диалога в DOM при быстром повторе), ревью fix ×1 (HIGH: CL.0 request desync при StreamRequestBody — отказ CSRF/auth до хендлера оставлял тело на keep-alive; MEDIUM: chunked без лимита). Тайминг 1ч 02м.
- Фаза 8: без переделок; MEDIUM ревью (контраст outline-кнопок в тёмной теме) — 2 строки CSS оркестратором. Тайминг 17м. Финальный E2E 134/134.
- Наблюдения: ревьюер light-трека нашёл реальную уязвимость воспроизведением (standalone-программа) — merged-линза с явным перечнем рисков безопасности окупается; fff в worktree не настроен весь прогон (preflight FAIL, сессию не перезапустить в автономном режиме).
- Кандидат в доктрину: глобальные изменения транспорта (StreamRequestBody/лимиты) — обязательный raw-TCP тест desync в live-сьюте.
