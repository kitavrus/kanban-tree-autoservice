---
stage: 8-code-review
feature: order-start-duration
tier: S
verdict: APPROVED
high: 0
medium: 0
low: 3
---
# Code Review: order-start-duration — APPROVED
**Review level:** Slim (tier: S) + Sec 18 (spec-track merged, API-линза). Итоговое состояние: `fb10d6b` + fix-loop 1 `fab3064`.

Источник истины: `0-setup/pipeline-state.md` §Задача/§Решения пользователя (вкл. решение fix-loop 1 «открытым без срока — до сегодня») + `0-setup/light-progress.md`. Режим CM-1: MVP → breaking POST/PUT (обязательные `start_date`/`duration_days`, `due_date` во входе → 400) не флагается. Чекер `e2e_mode` HEADLESS-RUN — решение пользователя, в счётчиках не учтён.

## Повторное ревью (fix-loop 1)

### Статус прежних замечаний
| ID | Было | Статус | Пруф |
|---|---|---|---|
| MEDIUM-1 | бэкфилл «без срока — 1» → массово «просрочено» | ЗАКРЫТО (решение пользователя «до сегодня») | `0018…up.sql:16-20`: открытые без валидного срока → `LEAST(GREATEST(today_almaty − start + 1, 1), 60)` (окончание = день миграции), выданные без срока → 1, со сроком ≥ приёмки → до срока. `UPDATE … FROM columns` покрывает все строки (`tasks.column_id NOT NULL REFERENCES columns`, `0005_board.up.sql:29`) → `SET NOT NULL` не упадёт. Тест: кейсы b/c/c2/g/h/h2, up→down→up, даты относительно now по Алматы |
| MEDIUM-2 | выдан раньше начала → фантомный день + зависимость от окна | ЗАКРЫТО | `calendar/service.go:82-86` `continue` (ни Days, ни Orders → Гант не видит); старый clamp `end=start` удалён; repo-фильтр теперь консистентен. Тесты: `TestBuildGantt_IssuedBeforeStart` (окна с днём выдачи и без), `TestBuild_PlanRules`, live `TestCalendar_OccupancyByDay` (`NotContains early`) |
| LOW-1 | `DEFAULT CURRENT_DATE` в поясе сессии БД | ЗАКРЫТО | `up.sql:26` `DEFAULT (now() AT TIME ZONE 'Asia/Almaty')::date`; тест (i) |
| LOW-2 | кламп 60 укорачивает прежний срок | ОТКРЫТО (информационно) → LOW 1 ниже | расширено кейсом c2 |
| LOW-3 | мёртвый код / комментарии / nullable | ЗАКРЫТО (nullable-тип принят для MVP) | ветка «срок не указан» удалена (`gantt.go:148-151`); null-ветки `board.js` удалены (при `null` деградирует безопасно: `"null"` не < даты, `fmtDate(null)=''`); комментарии `gantt.go:43,47-48`, `app.css:378`, `tasks_validate.go:45-47` исправлены. `DueDate *string` оставлен — не считаю |
| LOW-4 | фильтр «Горит/просрочено» без E2E | ЗАКРЫТО | `e2e/tests/order-plan.spec.ts` PLAN-003: 5 контрольных заказов с явным планом (будущий/сегодня/завтра/просрочен/выдан), метки `data-due`, `filter-due` soon/overdue/all + URL `due=` |

### Демо-сид (НОВОЕ, `internal/seeddemo`) — вердикт: OK, 2 LOW
- **Один заказ = одна машина клиента:** `validateDemoData` (`seeddemo.go:273-285`) — авто у клиента, без повторов, модель+госномер в названии; integration `titleNoPlate=0`, `tasksWithEquipment=16`. ✅
- **План:** `validatePlan` — дни 1..14, выданный: `finishedOffset ≤ −1` и `≥ startOffset`, невыданный — без дня выдачи; сроки услуг внутри плана; журнал/вложения в окне «приёмка…выдача» с запасом +7 ч на время суток (`minAgo = −fo·24+7` ≥ макс. давности выдачи 17:30 = `−fo·24+6.5`). Вызывается при каждом `Run` (`seeddemo.go:99`) и в unit `media_test.go:18`. ✅
- **finished_at/created_at/paid_at:** `($12::date + '17:30') AT TIME ZONE 'Asia/Almaty'`; триггер `tasks_set_finished_at` делает `COALESCE(NEW.finished_at, now())` (`0016_calendar.up.sql:25`) — не перетирает; `paid_at = finished_at` → `tasks_paid_at_chk` соблюдён; `created_at = now − acceptedH`. Сверил по данным все 16 заказов: приёмка ≤ дня начала (`acceptedH ≥ −so·24`), приёмка < выдачи (`acceptedH ≥ minAgo`), журнал/фото внутри окна — нарушений 0. Integration: `issuedBad`, `openFinished`, `logsOut`, `outOfWindow`, `startedNoPhoto` = 0. ✅
- **Календарь:** пересчитал вручную по правилам build — сегодня 7/8, +1..+3 → 6, +4 → 7, +5 → 8/8, +6 → 5, далее 3→2; просроченные open-ended — vesta, qashqai; «Горит» — gazelle/solaris/octavia; будущие — 6. Совпадает с `TestDemoPlan_*` и `assertCalendarLoad` (реальный `calendar.Service`, `max_cars=8`). ✅
- **Детерминизм:** все даты — смещения от «сегодня по Алматы» (`dayFromToday` и `svc.Today(timezone.Default)` и `almatyToday` в SQL — один пояс); логики дня недели нет; `AddDate` корректен через границу месяца/года; в Asia/Almaty нет DST → сид и его тесты не зависят от дня недели/месяца; от времени суток защищены запасом +7 ч и 17:30. Остаточный риск — только переход полуночи по Алматы между сидом и проверкой (пренебрежимо). `SetupTest` — `TRUNCATE … CASCADE` → изоляция от `TasksExist_NoOp`. E2E от демо-сида не зависит (ссылок нет). ✅

## Блокеры (обязательно исправить до мержа)
нет

## Серьёзные замечания (исправить в этом PR)
нет

## Незначительные замечания (можно в следующем PR)
1. **[migrations/0018_order_start_duration.up.sql:17,18 — сообщить пользователю]** Кламп 60 (CHECK 1..60): (а) прежний срок дальше 60 дней от приёмки укорачивается (dev: 3 заказа), down вернёт укороченный; (б) открытый без срока, принятый > 60 дней назад, получает окончание `start+59` < сегодня → «просрочено», а не «до сегодня» (кейс c2 в тесте — осознанно). На dev-данных (приёмка 14–15.10) (б) не срабатывает.
2. **[internal/seeddemo/data_tasks.go:125,216,293,301,147,341,13-14]** Тексты демо-журнала не привязаны к плану: prado — лог 26 ч назад «Записан на завтра» при начале +1 (завтра лога = сегодня); ducato — лог 20 ч назад «через 3 дня» при начале +3; camry — лог 18 ч назад «поставка через 3 дня» + описание «ремонт записан на день поставки» при начале +3 (совпадение зависит от часа прогона); «к пятнице»/«обещал в пятницу» не следуют за днём недели. Комментарий `:13-14` ссылается на несуществующий `TestValidateDemoData` (есть `TestDemoPlan_CalendarLoad`) и «в ближайшую неделю — 5–7» (на +7 занято 3). Косметика демо.
3. **[internal/seeddemo/seeddemo.go:310-353]** `validatePlan` не проверяет напрямую «приёмка не позже дня начала» (`startOffset ≤ 0 → acceptedH ≥ −startOffset·24`) и «приёмка раньше выдачи» без журнала/фото (`final → acceptedH ≥ minAgo`) — сейчас держится только данными (запас 2 ч у 9 заказов); integration ловит лишь `finished_at < created_at`. Добавить 2 условия в `validatePlan`.

## Slim-чеклист (итоговое состояние)
- **Security (Sec 6):** новых эндпоинтов нет; SQL параметризован (вкл. сид `$12`/`$13`); `due_date` не биндится (strictJSON); вывод — `textContent`/html/template. ✅
- **Architecture fit (Sec 1):** правки в существующих слоях `board`/`calendar`/`seeddemo`; бизнес-логики в handler нет. ✅
- **Error Handling (Sec 4):** поля через `errorspkg.Field`, коды `required`/`invalid_date`/`out_of_range`; CHECK БД недостижим. ✅
- **Tests (Sec 7+12):** unit (validate, fuzz, build/gantt вкл. IssuedBeforeStart, `TestDemoPlan_*`), integration (миграция a–i, сид-инварианты), live (board/calendar), E2E PLAN-001/002/003, GANTT-001/002, CALNO, CAP, OC-003/004. Артефакт прогона fix-loop — у оркестратора (повторный прогон идёт; прежний — `9-validation/evidence/last-run.json`, 188 passed).
- **Rollback (Sec 8):** down (`DROP EXPRESSION` → `DROP COLUMN`) прогнан тестом up→down→up. ✅

## Sec 18 — API-контракт
- nullable `due_date` (никогда не NULL) — принято для MVP.
- breaking POST/PUT — MVP (§CM-1), не флагается.
- error-body `errorspkg`, конверт списка, N+1 — без изменений/без регрессий. ✅

## Соответствие коду решениям пользователя
- срок = начало + дни − 1, метка 📅 и фильтр — ДА (E2E PLAN-003);
- просроченный невыданный занимает место до выдачи — ДА;
- правка обоих полей в карточке — ДА;
- бэкфилл: начало = приёмка; дни до прежнего срока; открытые без срока — до сегодня; выданные без срока — 1 — ДА (с клампом 60, LOW 1).

## Статус фаз
- `0-setup/light-progress.md` — все пункты `[x]` (вкл. fix-loop 1) — ДА.

## Итог
APPROVED: блокеров 0, серьёзных 0; MEDIUM-1/2 и LOW-1/3/4 закрыты; открыто LOW 1 (информировать пользователя о клампе 60) и 2 косметических LOW по демо-сиду.
