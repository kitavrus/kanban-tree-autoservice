---
stage: 8-code-review
feature: executor-payouts
tier: S
verdict: APPROVED
high: 0
medium: 2
low: 7
---
# Code Review: executor-payouts — APPROVED
**Review level:** Slim + Sec 18 API-линза (tier: S, /pipeline-light; режим spec-track merged, спеки нет — сверка с D62–D72 `docs/plan.md` и TS-001…TS-033 `0-setup/light-progress.md`)

Третий проход. База 489a741. История: 243df5b — CHANGES REQUESTED (high 1 / medium 4); da45c76 — APPROVED после fix-loop; текущий — 188262b (доработка D72), проверена дельта `git diff da45c76 188262b -- internal web e2e docs/plan.md`. Гейты (build/unit/lint/integration, E2E 72 passed, чекеры 9/9) — pre-condition оркестратора, не перегонялись. Вживую экран не открывался: находки по JS — по чтению кода.

## Блокеры (обязательно исправить до мержа)
Нет.

## Доработка D72 — правка суммы выплаты на экране «Выплаты»

### Серьёзные замечания (исправить в этом PR)

1. **[web/ssr/_shared/js/executor-payouts.js:326-329 (`savePayout` → `render`), :204-205 (`render`)] После каждого сохранения суммы список перерисовывается целиком — фокус и недописанный ввод в другом поле теряются.**
   `render()` заменяет `#ep-groups.innerHTML`; вызывается после ответа PUT и перезапроса `/summary`.
   Сценарий (основной поток — ввод сумм по колонке): ввёл сумму в строке A → кликнул в поле строки B (blur A → PUT) → через время ответа PUT + `/summary` DOM пересоздан: поле B теряет фокус, набранные к этому моменту цифры заменяются сохранённым значением, `change` у уничтоженного узла не срабатывает — ввод в B молча отброшен, дальнейшие нажатия уходят в никуда. С Tab то же: фокус сбрасывается на начало страницы. Срабатывает на каждом переходе «поле → поле», не только при медленной сети; ручная проверка оркестратора (Enter в одном поле) этот путь не задевает, EP-009/EP-010 — тоже (каждое поле завершается Enter/Tab и ожиданием).
   Сопутствующее: `state.items.indexOf(it)` (:326) ищет по ссылке на объект — если за время запроса список перезагружен (смена фильтра, `mark()` → `load()`), строка не обновится и покажет старую сумму при новых итогах.
   Фикс: после успеха обновлять точечно (значение поля строки + шапки групп/общие итоги) без пересоздания таблиц, либо сохранять/восстанавливать фокус и несохранённые значения полей при `render()`; элемент искать по `subtask_id`. Тест E2E: ввод в A → клик в B → набор → значение B сохранено.
2. **[web/ssr/_shared/js/executor-payouts.js:175] Денежное поле с автосохранением сделано `type="number"`; в карточке то же поле — `type="text" inputmode="decimal"`.**
   Сценарии: (а) клик по стрелке-спиннеру даёт `change` на каждый шаг → PUT + полная перерисовка на каждый клик (поле блокируется, фокус теряется — «накликать» сумму нельзя); (б) в браузерах, где колесо мыши меняет значение сфокусированного number-поля, прокрутка страницы над полем меняет сумму на ±0.01·n, и уход из поля сохраняет её без подтверждения — тихая порча суммы выплаты (на стенде не проверял); (в) запятая: в number-поле вне ru-локали «1,5» даёт `badInput` и сообщение «введите число», в карточке запятая принимается (`replace(',', '.')`) — разное поведение одного поля на двух экранах.
   Фикс: `type="text" inputmode="decimal"` + тот же разбор, что в `task-card.js` (запятая → точка, regex).

### Проверено по D72, замечаний нет

- **Блокировки (repo.go `UpdatePayout`):** заказ `FOR UPDATE` → услуга `FOR UPDATE` → UPDATE → чтение строки отчёта в той же tx. Тот же порядок, что у `Mark` (tasks → subtasks) и board (column FOR SHARE → task → subtask); колонку отчёт не блокирует — цикла ожидания нет. TS-032 гоняет конкуренцию с `mark` и PUT карточки.
- **Изоляция полей:** UPDATE трогает только `executor_payout`/`updated_at`; флаг, дата, цена, статус, исполнитель не меняются (TS-029). Карточка после fix-loop шлёт сумму только при правке самой суммы — второй писатель её не затирает.
- **Пустая строка vs мусор (`savePayout` :300-307):** `value === ''` при `validity.badInput` → сообщение и откат, запрос не уходит; `null` уходит только при действительно пустом поле. Экспонента, >2 знаков, отрицательное — отсекает regex до запроса.
- **Двойное сохранение Enter+blur:** Enter → `preventDefault` + `blur()` → один `change`; на время запроса поле `disabled`, повторный вход отсекается.
- **Валидация суммы:** `payoutScale{IntDigits:10, FracDigits:2}` = board `priceScale` (`subtasks_validate.go:28`), тот же `Validate(scale, false)`; коды совпадают с карточкой. Поле обязательно (нет поля → 400 `required`), `null` — очистить (TS-030, TS-031).
- **Права:** admin и executor, в т.ч. сумма чужой услуги — по D62 («любой авторизованный»), в D72 зафиксировано явно; TS-029/EP-010.
- **404:** услуги нет или без исполнителя → `EXECUTOR_PAYOUT_NOT_FOUND` (ресурс — строка отчёта), в D72 обосновано; битый uuid → 400 `id: invalid`. Маршрут `PUT /:id` не конфликтует с `GET /summary` и `POST /mark` (разные методы).
- **§API-1 / errorspkg / LiveSuite / XSS:** URL kebab, тело и ответ snake_case, ответ — элемент списка; ошибки через `errorspkg`; live-тесты на LiveSuite; значение поля в разметке через `esc()`, id в URL — `encodeURIComponent`, ошибки — `textContent`.

## Фича целиком (D62–D71) — состояние после fix-loop da45c76

Находки первого ревью закрыты и подтверждены по коду на втором проходе; дельта D72 их не затрагивает:

| Находка | Статус | Где |
|---|---|---|
| HIGH-1 карточка затирала отметку выплаты устаревшим снимком | закрыто | `web/ssr/_shared/js/task-card.js:961-970` |
| MEDIUM-1 `executor_payout`: нет поля → NULL | закрыто | `internal/features/board/optional_amount.go`, `subtasks_repo.go` Update |
| MEDIUM-2 выбор до 5000 строк при лимите mark 500 | закрыто | `executor-payouts.js` `mark()` |
| MEDIUM-3 нет тестов конкуренции | закрыто | TS-025…TS-027, EP-008 |
| MEDIUM-4 `no_assignee` по итоговому состоянию | закрыто | `internal/features/board/subtasks_service.go:153-158`, TS-028 |

Без замечаний: Security (оба ресурса за `RequireAPI()`/`RequireSSR()`, SQL параметрами, лимиты), атомарность `Mark`, SQL фильтра и метки доски (D67/D68, N+1 нет), миграция 0011 up/down, §API-1/§GO-1, Sec 18 (nullable согласован, конверт `{items,total,limit,offset}`, breaking нет), отсутствие scaffold-стабов/`waitForTimeout`/`app.Test`.

## Незначительные замечания (можно в следующем PR)

1. **[internal/features/executorpayouts/dto.go `optionalAmount`; internal/features/board/optional_amount.go; validate.go `payoutScale`]** Тип продублирован 1:1, границы суммы — вторым экземпляром `priceScale`. Вынести в `pkg/numeric` (`OptionalAmount` рядом с `Amount` — пакет уже общий для board и catalog, ADR-101) стоит: два писателя одного поля обязаны разбирать его одинаково, расхождение копий = разная семантика `null`. Не срочно — копии сейчас идентичны.
2. **[web/ssr/_shared/js/executor-payouts.js `savePayout`]** Regex пропускает сколь угодно длинное целое; `Number` → JSON в экспоненциальной записи (`1e+24`) — сервер ответит кодом формата, пользователь увидит «введите число» вместо «слишком большая сумма». Ограничить длину целой части на клиенте.
3. **[docs/features/executor-payouts/0-setup/light-progress.md:90]** TS-033 остался `[ ]` «прогон за оркестратором» при улике `9-validation/evidence/e2e-run.txt` — 72 passed.
4. **[web/ssr/_shared/js/executor-payouts.js `loadItems`/`load`]** Нет фильтра периода; при `status=all` тянется до 5000 строк. Оставлено осознанно (D69).
5. **[internal/features/executorpayouts/repo.go `List`/`Summary`]** `count` и выборка вне одной транзакции. Оставлено осознанно.
6. **[migrations/0011_executor_payouts.up.sql:13]** `CREATE INDEX` без `CONCURRENTLY` — норма режима MVP (§CM-1). Оставлено осознанно.
7. **[8-code-review/checks/live_suite.txt]** Чекер слеп к плоскому пакету («HTTP-слоя нет» при наличии `handler.go`/`routes.go`) — его PASS неинформативен; адресат — инструмент.

## Соответствие коду дизайну (верификация реализации)
- [x] D62–D72 реализованы — ДА
- [x] Error handling: 409 `SUBTASK_EXECUTOR_PAID`, 404 `EXECUTOR_PAYOUT_NOT_FOUND`, 400 по полям/индексам — ДА
- [x] TS-001…TS-033 привязаны к тестам — ДА
- design-*.md / Monitoring / Rollout / Runbook / Performance Budget / Swagger / PRD-ADR — N/A (S-tier, /pipeline-light, спеки и PRD нет)

## Статус фаз
- [ ] `light-progress.md` — всё `[x]`, кроме TS-033 (LOW-3: прогон есть, носитель не обновлён) — НЕТ

## Итог
APPROVED: блокеров нет, серверная часть D72 корректна (блокировки, изоляция полей, валидация, права). Открыты 2 MEDIUM по экрану отчёта — оба в `executor-payouts.js`; MEDIUM-1 (потеря фокуса и ввода при переходе между полями) затрагивает основной сценарий доработки и рекомендуется к исправлению до мержа.
