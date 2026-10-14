---
stage: 8-code-review
feature: user-timezone
tier: M
verdict: APPROVED
high: 0
medium: 1
low: 8
---
# Code Review: user-timezone — APPROVED
**Review level:** Standard (tier: M), режим spec-track merged (Sec 17 по DS/TS, Sec 18 API-линза)

Источники: `input-bundle.md`, `diff-backend.patch`, `diff-migrations.patch`, `diff-other.patch`, точечно `draft-spec.md` §3/§4/§6/§8.1/§11/§Impact. Сборка/тесты/линт/E2E не перегонялись (гейт оркестратора). Чекеры: 9 ✅, `e2e_drift` ❌ — см. L7.

## Блокеры (обязательно исправить до мержа)
Нет.

## Серьёзные замечания (исправить в этом PR)
1. **[project/.scaffold-lock.json; §Impact #18]** Решение «обновить lock либо вынести дельту» не зафиксировано ни в diff, ни в `0-setup/light-progress.md`. Факт: все 10 затронутых фичей lock-файлов (`internal/features/auth/{dto,handler,repo,service}.go`, `internal/features/users/{dto,handler,repo,routes,service}.go`, `internal/server/middleware/auth.go`) не совпадали с lock уже на базе `beb6d60` (sha256 содержимого ≠ lock у 24 из 36 отслеживаемых файлов; lock менялся только в `c9416a2`). Оговорка: алгоритм хэша `/project-bootstrap` не сверялся — если он нормализует содержимое, счёт «24 из 36» неточен. Новый класс расхождения фича не вносит, `/project-bootstrap check` покажет эти файлы изменёнными как и до неё. Исправление: записать решение (as-is DS «каркасные auth/users/middleware ведутся вручную, lock не обновляется» либо обновить lock) — выбор за пользователем.

## Незначительные замечания (можно в следующем PR)
1. **[pkg/timezone/timezone.go:27-33]** `Valid` опирается на `time.LoadLocation`, который сначала читает системный zoneinfo, и только потом вшитый `time/tzdata`. Следствие (вывод из семантики Go, прогоном не проверено): на Linux-хосте с полным zoneinfo пройдут `posix/Europe/Moscow`, `right/Europe/Moscow`, `localtime`; на macOS — имена в другом регистре. Браузерный `Intl` такие имена отвергает → `timezone.js:27` молча падает на `Asia/Almaty`, а SSR форматирует по реальному поясу: на одной странице два пояса. Достижимо только прямым вызовом API. Исправление: отклонять префиксы `posix/`, `right/` и `localtime`, либо сверять с перечнем вшитой базы.
2. **[pkg/timezone/timezone.go:36-47,50]** `Load` вызывает `LoadLocation` дважды (в `Valid` и сам) на каждый `timeIn`; кэша нет. История оборудования с N записями журнала = 2N разборов файла пояса. Исправление: `sync.Map` имя → `*time.Location`.
3. **[internal/features/users/repo.go:183-190]** `SetTimezone` не обновляет `users.updated_at`; путь администратора (`repo.go:164-168`) обновляет. Спека фиксирует только «version не увеличивается» (§3, строка 54). Вынести пользователю.
4. **[web/ssr/_shared/js/executor-payouts.js:149, stock-transfers.js:119]** Формат времени сменился с «07.10.2026, 00:30» на «07.10.2026 00:30» — as-is поведение вне спеки. Допустимо (совпадает с SSR-форматом `02.01.2006 15:04`), но оформить to-be DS; вынести пользователю.
5. **[web/ssr/_shared/js/timezone.js:68-85]** Третья копия тоста (`board.js:723`, `suppliers.js:50`). При реализации DS-052 вынести в `KT.toast`.
6. **[web/ssr/_shared/js/timezone.js:106]** `window.location.reload()` после смены пояса: на странице-ответе POST (форма сотрудника с 422) браузер предложит повторную отправку формы; несохранённый ввод на любой странице теряется без предупреждения.
7. **[checks/e2e_drift.txt]** FAIL — ложное срабатывание инструмента: ищет `test('TS-NNN: …')` в `scripts/e2e`, в проекте — `e2e/tests/`, имена `TZ-001 (TS-040): …`. Покрытие проверено по существу (ниже). Адресат — оркестратор: настроить путь/шаблон чекера.
8. **[e2e/tests/user-timezone.spec.ts:124-129]** Проверка тем — «цвет текста ≠ фон»: ловит только полное слияние. Для `themes:both` достаточно, усилить при случае.

## Ответы на девять вопросов
1. **Полнота показа времени — допустимо.** Независимый проход по `web/ssr/**/*.js` и `**/*.html`, `internal/`, `pkg/`, `cmd/`: мест мимо пояса нет. JS: все `*_at` идут через `KT.time` (`task-card.js:411,492,563,574,822,869`, `executor-payouts.js:183`, `stock-transfers.js:136,193-196`); `due_date` — календарная дата, строковый разбор (`board.js:393-397`). Шаблоны: четыре места, все через `timeIn`. Go: `.Format` только в `tasks_handler.go:46` (пояс пользователя), `clients/history.go:343` (дата без времени), `attachments_handler.go:124` (HTTP-заголовок). SQL: `current_date` только внутри `COALESCE($N::date, current_date)`.
2. **«Сегодня» — допустимо.** `Today` формируется сервером (`time.Format(time.DateOnly)`), передаётся параметром — инъекции нет; пустое → `nil` → `current_date`. Единственный прод-источник `Actor` в board — `currentActor` (`tasks_handler.go:38-41`). Граница суток покрыта `TestTS041_DueFilter_*` и `TestTS041_CardAndMutations_*` (UTC+14 против UTC−12, устойчиво к времени прогона).
3. **`PUT /users/me/timezone` — допустимо.** Зарегистрирован до admin-группы (`routes.go:17`), id только из сессии; `user_id` в теле и в query игнорируется (тест `:160-172`). CSRF — общий `app.Use` (`router.go:92`), лимит тела — общий `BodyLimit` (`router.go:59`). Обход `strictJSON` оправдан спекой (§3 abuse case 2); мусор после объекта `json.Unmarshal` отклоняет сам. Content-Type не проверяется — закрыто CSRF-заголовком, проверкой Origin и `SameSite=Strict`.
4. **Валидация IANA — допустимо**, соответствует §6: `/me/timezone` пусто → `required`, иное → `invalid`; POST/PUT сотрудника пусто → `invalid`. `Local` отклонён, `time/tzdata` вшит. Оговорка — L1.
5. **Пояс в запросе — допустимо.** Читается тем же запросом `Principal` (`auth/service.go:106-117`, +1 колонка), лишнего запроса нет, кэша нет — смена администратором действует со следующего запроса (E2E TZ-010, строки 554-562).
6. **Формат без запятой — вынести пользователю** (L4).
7. **E2E-фикстура — не тавтология.** JS-экраны: подменяется только значение `*_at`, строку строит боевой `KT.time`; браузер в третьем поясе (`America/New_York`), ожидания — точные строки двух поясов. SSR: разница 120 минут между двумя пользователями на одной записи и разные календарные даты при UTC+14/UTC−12 — это реальное серверное форматирование; точные значения закрыты Go-тестами (`TestTS041_Handover_PaidDate_*`, `TestTS041_Funcs_TimeIn`).
8. **Scaffold-lock — вынести пользователю** (серьёзное замечание 1).
9. **Миграция 0013 — допустимо.** `ADD COLUMN … NOT NULL DEFAULT` заполняет существующие строки, CHECK длины 1..64, down — `DROP COLUMN IF EXISTS`. Тест `TestTS022_Migration0013_UpDown` проверяет backfill, NOT NULL, CHECK, down и повторный up.

## Sec 18 — API-контракт
- nullable: `timezone` в ответах — непустая строка, в БД NOT NULL; в запросах сотрудника `*string` (`null`/нет поля → умолчание при создании, без изменений при правке) — согласовано.
- breaking: только добавление поля в `UserResponse` (auth, users) — нет.
- error-body: `errorspkg`, 400 → `details.fields[]`; конверт списка не менялся; N+1 нет.

## Соответствие кода спеке
- [x] DS-031 (хранение UTC, показ по поясу) — ДА
- [x] DS-032 (все экраны) — ДА
- [x] DS-033 (сам пользователь, умолчание Asia/Almaty) — ДА
- [x] DS-056 (администратор любому, исполнитель чужой — 403) — ДА (`TestTS022_ExecutorCannotSetForeignTimezone`)
- [x] DS-034 — вне части (реализуется в `transfers-partners`), не дефект
- [x] TS-021, TS-022, TS-040, TS-041, TS-042, TS-F5 покрыты именованными тестами, проверяющими заявленное — ДА
- [x] §Impact «Вне каталога фичи»: все позиции имеют правку в diff, кроме #18 (замечание 1) — ДА с оговоркой
- [x] Scaffold-стабы (`t.Skip("scaffold stub`, `test.fixme`) — не найдены
- Monitoring / Rollout / Runbook / Performance Budget / Swagger — N/A (в спеке нет; `swagger:no`)

## Статус фаз
- [x] Все чекбоксы `0-setup/light-progress.md` — `[x]` — ДА. Запись в строке 20 («НЕ переведён TaskRepo.Get subtasks_late») устарела: переведён в `d01b9c4` (`tasks_repo.go:269-292`).

## Итог
APPROVED: блокеров нет. До мержа — зафиксировать решение по scaffold-lock; пользователю вынести три вопроса: lock, формат времени без запятой, `updated_at` при смене своего пояса.
