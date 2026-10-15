---
stage: 8-code-review
feature: order-start-duration
tier: S
verdict: APPROVED
high: 0
medium: 2
low: 4
---
# Code Review: order-start-duration — APPROVED
**Review level:** Slim (tier: S) + Sec 18 (spec-track merged, API-линза)

Источник истины: `0-setup/pipeline-state.md` §Задача/§Решения пользователя + `0-setup/light-progress.md` (draft-spec/PRD/design-*.md нет). Режим CM-1: MVP → breaking POST/PUT (обязательные `start_date`/`duration_days`, `due_date` во входе → 400) не флагается. Чекер `e2e_mode` HEADLESS-RUN — решение пользователя, в счётчиках не учтён.

## Блокеры (обязательно исправить до мержа)
нет

## Серьёзные замечания (исправить в этом PR / решение пользователя)
1. **[migrations/0018_order_start_duration.up.sql:12-15 — решение пользователя]** Бэкфилл «без срока — 1 день от приёмки» ⇒ каждый НЕ выданный старый заказ без срока, принятый раньше сегодня, получает `due_date < today` → красная метка 📅 + `.overdue` (`board.js:387`), попадает в «Горит» и «Только просроченные» (`tasks_repo.go:486-487`), на Ганте статус «срок DD.MM просрочен» вместо прежнего «срок не указан». Загрузка календаря НЕ меняется (до и после — open-ended). Dev-БД: 97 заказов без срока. Буквально соответствует решению «без срока — 1», но массовую пометку «просрочено» пользователь мог не предвидеть. Варианты: (a) оставить (оператор перепланирует); (b) для открытых без срока — `duration_days = LEAST(GREATEST(current_date − start_date + 1, 1), 60)` → окончание = день миграции («Горит», но не «просрочено»); (c) иное. Подтвердить у пользователя.
2. **[internal/features/calendar/service.go:87 + repo.go:42 + gantt.go:142]** Выдан раньше «Начать с» (заказ с будущим началом перенесён в финальную колонку — отмена/ошибка) → `end = start`: фантомная занятость места в будущий день начала уже выданной машиной; на Ганте период «20.10 → выдан 20.10» при фактической выдаче 16.10 (неверная дата). Плюс зависимость от окна: repo отсекает по сырому `finished_on >= from`, а build — по зажатому `end`; при `finished < from <= start <= to` заказ пропадает, при окне, захватывающем `finished`, — появляется (месячная сетка и Ганта могут разойтись по занятости одного дня). Правило выбрано исполнителем (тест `service_test.go:79,94`), решением пользователя не покрыто. Fix-кандидат: `FinishedOn < start` → заказ места не занимает (`continue` в build); repo-фильтр тогда консистентен как есть.

## Незначительные замечания (можно в следующем PR)
1. **[migrations/0018_order_start_duration.up.sql:20]** `DEFAULT CURRENT_DATE` — дата в поясе сессии БД (контейнер/сервер — обычно UTC), бэкфилл — Asia/Almaty: прямой INSERT 00:00–05:00 по Алматы получит вчерашнее начало. Путь только служебный/тестовый (API и seeddemo передают явно). Fix: `DEFAULT (now() AT TIME ZONE 'Asia/Almaty')::date`.
2. **[migrations/0018_order_start_duration.up.sql:14]** Кламп 60 молча укорачивает прежний срок (dev: 3 заказа) → плановое окончание раньше прежнего, «просрочено» наступит раньше; down вернёт уже укороченный срок (необратимо). Неизбежно при CHECK 1..60 — сообщить пользователю.
3. **[internal/features/calendar/gantt.go:154, web/ssr/_shared/js/board.js:380,439, internal/features/board/tasks_dto.go:31]** Мёртвый код/nullable-рассинхрон (Sec 18): `due_date` вычисляется из NOT NULL колонок → в данных никогда не NULL; ветка «срок не указан» недостижима; `if (!t.due_date)` / `: '<span></span>'` — мёртвые; `TaskCard.DueDate *string` / calendar `Order.DueDate *string` объявлены nullable. Устаревшие комментарии: `gantt.go:48` («0 — срока нет»), `web/ssr/_shared/app.css:378` («со дня приёмки по срок»), `tasks_validate.go:45-47` (`nilIfEmpty` к `start_date` не применяется). Удалить ветки/поправить комментарии; тип `due_date` — `string` (MVP допускает).
4. **[e2e/fixtures/auth.ts:526-532, e2e/pages/BoardPage.ts:60]** Фикстура `createTask` по умолчанию даёт план «сегодня, 1 день» → `due_date = today` → каждый фикстурный заказ «Горит» (soon). Существующее покрытие НЕ маскирует (ни один spec не использует фильтр due; спеки меток задают план явно — PLAN-001 `ok`, TZ-011 `overdue/soon`), но фильтр «Горит/просрочено» на UI без E2E вообще (`filterDue` объявлен, не используется), семантика фильтра покрыта только live-тестом `TestBoardLiveSuite_TaskPlan_CreateUpdate`. Будущий filter-E2E обязан давать контрольным заказам явный будущий план. Рекомендация: E2E фильтра (просрочен / кончается сегодня / будущий план → видимость по `filter-due`).

## Вердикты по пунктам оркестратора
| # | Тема | Вердикт |
|---|---|---|
| 1 | Миграция 0018 + интеграционный тест | OK. `created_at` — `timestamptz` (`0005_board.up.sql:39`) → `AT TIME ZONE 'Asia/Almaty'` даёт локальную дату корректно. Индексов/триггеров/вью на `tasks.due_date` нет (триггер `tasks_set_finished_at` — только `column_id`), позиционных INSERT/`SELECT *` по tasks нет → DROP+ADD GENERATED STORED безопасен. Файл — один multi-statement Exec golang-migrate → неявная транзакция. Down: `DROP EXPRESSION` (PG≥13) до `DROP COLUMN` — порядок верный, nullable восстановлен. Тест: изолированный dockertest-контейнер на suite; кейсы a–g (вкл. UTC→Алматы, кламп, срок < приёмки), GENERATED-запись запрещена, CHECK 0/61, DEFAULT, up→down→up. Замечания — LOW 1, LOW 2; последствие бэкфилла — MEDIUM 1 |
| 2 | Бэкфилл без срока → «просрочено» | MEDIUM 1 — решение пользователя (буквально соответствует, последствие не проговорено) |
| 3 | Календарь build/repo/gantt | Будущее начало — место только с `start_date` ✅ (TestBuild_PlanRules, GANTT-002, PLAN-001); просрочка невыданного → open-ended ✅; граница repo `start_date <= to` ✅ (будущие за окном отсечены, открытые просроченные — включены). Выдан раньше начала — MEDIUM 2 |
| 4 | Валидация | OK. `required` (nil/пусто/пробелы — trim), `invalid_date` (формат, 2026-02-30), `out_of_range` (окно 2000-01-01..2099-12-31, дни 1..60 = CHECK); PUT пересчитывает `due_date` (GENERATED; live-тест ответ/GET/список); `due_date`/`"3"`/`1.5`/число-в-дату во входе → 400 (strictJSON/bind, не 500); отклонённый PUT версию не меняет |
| 5 | task-card.js | OK. Окончание — `KT.time.addDays` (UTC-арифметика по YMD, без сдвига), «сегодня» — `KT.time.today()` в поясе пользователя; загрузка — на день начала (`refreshStartLoad`), предупреждение — по `[max(start,today)..end]` чанками ≤62; устаревшие ответы отсекаются `todaySeq`/`dueSeq`; локальная валидация с серверными кодами; ошибки плана в PUT — под полями, `{local:true}` молча; правка в карточке — PUT с `version` |
| 6 | Фикстура createTask «сегодня» | Покрытие не маскирует, но фильтр без E2E и ловушка для будущих тестов — LOW 4 |
| 7 | Мёртвый код | LOW 3 (`gantt.go:154` недостижима; null-ветки board.js; nullable DTO) |

## Slim-чеклист
- **Security (Sec 6):** новых эндпоинтов нет; SQL параметризован (`$6::date`, `$10`/`$11`); `due_date` не биндится (strictJSON) — mass assignment закрыт; XSS: план/загрузка — `textContent`, `data-start="{{ .Date }}"` — html/template; секретов/PII в логах нет. ✅
- **Architecture fit (Sec 1):** правки в существующих слоях `board`/`calendar` (dto → validate → service → repo), бизнес-логики в handler нет; feature-структура legacy-плоская — не флагается. ✅
- **Error Handling (Sec 4):** ошибки полей через `errorspkg.Field` (`details.fields[]`), коды `required`/`invalid_date`/`out_of_range`; CHECK-нарушение БД недостижимо (валидатор раньше). ✅
- **Tests (Sec 7+12):** unit (`tasks_validate_test`, fuzz-сигнатура, `TestBuild_PlanRules`, `TestBuildGantt_FutureStart`), integration (миграция), live (`TaskPlanValidation400`, `TaskPlan_CreateUpdate` вкл. фильтр, calendar), E2E PLAN-001/002, GANTT-001/002, CALNO-001/002, CAP-001, OC-003; adversarial 400 + UI-валидация ✅; concurrency шапки — OC-004 ✅. Артефакт: `9-validation/evidence/last-run.json` — 188 passed.
- **Rollback (Sec 8):** down есть и прогнан тестом (up→down→up); потери при down: план (start/дни) и исходный срок у клампнутых — приемлемо для MVP. ✅

## Sec 18 — API-контракт
- nullable consistency: `due_date` nullable в DTO, никогда не NULL — LOW 3.
- breaking: POST/PUT требуют план, `due_date` во входе → 400 — MVP (§CM-1), не флагается.
- error-body: формат `errorspkg` ✅; конверт списка `{items,total,limit,offset}` не тронут ✅; N+1 не добавлено (скаляры в том же SELECT) ✅.
- каждый случай ошибки возвращает свой код — проверено live-тестом ✅.
- api_conventions / contract / go_naming / live_suite / fixture_sync / fuzz_verdict — PASS (bundle).

## Соответствие коду дизайну (верификация реализации)
- Решения пользователя (pipeline-state §Решения): срок = начало + дни − 1, метка 📅 и фильтр по плановому окончанию — ДА; просроченный невыданный занимает место до выдачи — ДА; правка обоих полей в карточке — ДА; бэкфилл «начало = приёмка, дни до прежнего срока, без срока 1» — ДА (с клампом 60, LOW 2).
- design-*.md / PRD / Monitoring / Runbook / Perf Budget — N/A (light-трек, S-tier).

## Статус фаз
- Все пункты `0-setup/light-progress.md` — `[x]` — ДА.

## Итог
APPROVED: блокеров 0; MEDIUM 1 — решение пользователя по дефолту бэкфилла; MEDIUM 2 — правило «выдан раньше начала» (рекомендуется исправить: не занимать место).
