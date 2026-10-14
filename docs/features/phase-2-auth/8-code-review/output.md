---
stage: 8-code-review
feature: phase-2-auth
tier: M
verdict: APPROVED
high: 0
medium: 0
low: 1
---
# Code Review: phase-2-auth — APPROVED
**Review level:** Standard (tier: M) · Режим: spec-track merged (Sec 18 вкл.) · повторное ревью после ca186d5 (база b7f600b) · источник истины — docs/plan.md §2/§4/§6, D19 D28 D29 D35 D39 D40 D41 D42 · CM-1=MVP · US-1: swagger:no / adaptive:min / themes:both

## Блокеры (обязательно исправить до мержа)
Нет.

## Серьёзные замечания (исправить в этом PR)
Нет.

## Незначительные замечания (можно в следующем PR)
1. **[internal/features/users/service.go:173-185 + internal/session/store.go KillUser]** Гонка «вход по старому паролю одновременно со сменой пароля»: `Store.Create` после `SMembers` в `KillUser` переживает смену пароля. Окно узкое; блокировку не затрагивает (D42 — `resolve` проверяет `is_active`). Оставлено осознанно → known issues Report. Вариант на будущее: `password_changed_at` в Principal, отказ сессиям с `CreatedAt` раньше.

## Верификация фиксов прошлой итерации (ca186d5)
| Прошлая находка | Статус | Доказательство |
|---|---|---|
| HIGH D41 check-then-increment | ✅ fixed | `internal/features/auth/service.go`: `TxPipeline` `Incr`+`ExpireNX` ДО bcrypt, `attempt.Val() > LoginMaxFailures` → 429; успех → `DEL`. Параллельные запросы получают разные номера. `TestLogin_BruteForceLimit_Parallel`: 20 параллельных при лимите 3 → `{401: 3, 429: 17}`, затем верный пароль → 429 |
| MEDIUM bcrypt >72 байт → 500 | ✅ fixed | `users/validate.go`: `passwordMaxBytes = 72`, `len(in.Password)` (байты); `fieldText` — понятный текст; `maxlength="72"` в `form.html`/`login.html`; вход с паролем >72 байт → `CompareHashAndPassword` ошибка → 401. Unit 72/73 байта, 36/37 кириллицей; live create → 400 `too_long` / 201; E2E US-002 (37×«п» → текст под полем) |
| LOW рост индекса `user_sessions:<uid>` | ✅ fixed | `session/store.go` `pruneIndex` (EXISTS по членам, SREM мёртвых) в `Create`; `TestCreatePrunesDeadSessionsFromIndex` |
| LOW старая сессия при повторном входе | ✅ fixed | `auth/handler.go` `replaceSession` (API и SSR) убивает прежнюю `sid`; `TestRelogin_KillsPreviousSession` |
| LOW неограниченная длина логина → ключ Redis | ✅ fixed | `maxLoginLen = 50` → `INVALID_CREDENTIALS` до Redis/БД; `TestLogin_OversizedInput` (401) |
| LOW список /users >200 без подсказки | ✅ fixed | `list.html`: «показаны первые N, уточните поиск» при `Total > len(Items)` |
| LOW plan.md seed-demo в фазе 2 | ✅ fixed | строка фазы 2: `seed-demo` — фаза 5 |
| LOW гонка смена пароля/вход | ⏸ accepted | known issue (см. выше) |

## Проверено — без замечаний
- **CSRF (D40)** `middleware/csrf.go`: 256-бит токен, `subtle.ConstantTimeCompare`, cookie `kt_csrf` HttpOnly+Strict+Secure-флаг, проверка `Origin`, все мутации вкл. `/api/v1/*`, `POST /login`, `POST /logout`. `TestCSRF_RequiredOnMutations` ✅.
- **Сессии (D35)**: sid 256 бит, новый sid на каждый вход (fixation нет) + прежний убивается; sliding TTL ≤ `absoluteMax`; cookie `sid` HttpOnly+SameSite=Strict+`SESSION_COOKIE_SECURE`.
- **D42**: Principal из БД на каждом запросе; неактивный → сессия убивается; битый `user_id` → аноним.
- **Вход**: одинаковый 401 `INVALID_CREDENTIALS` для нет-логина/неверного пароля, dummy bcrypt той же cost; `USER_BLOCKED` только при верном пароле.
- **Open redirect**: `SafeRedirect` + `TestSafeRedirect` ✅.
- **IDOR/эскалация**: `RequireAPI|RequireSSR` + `RequireAdmin` на `/api/v1/users*` и `/users*`; DTO без server-side полей.
- **D29 LAST_ADMIN** `users/repo.go:136-148`: `FOR UPDATE` активных админов + цели в порядке id; пересчёт после блокировки (READ COMMITTED + EvalPlanQual). `TestLastActiveAdminProtected_AndRaceKeepsOne` (×5, -race) ✅. SELF_BLOCK ✅.
- **D39**: DELETE нет, `TestNoDelete` ✅.
- **TD-4**: `strictJSON` → 400; битый uuid → 404; whitelist sort/order/role; SQL параметрами, `escapeLike`; пароль >72 байт → 400/401, не 500.
- **XSS**: `html/template`, сырых вставок нет.
- **Миграция 0002**: up/down пара; необратимость lower(login) — норма CM-1=MVP.
- **E2E**: POM + fixtures, без `waitForTimeout`; adversarial (403, 404, валидация, занятый логин, SELF_BLOCK, блокировка выкидывает вкладку), конкурентность US-020 (LWW, D28), overflow 360/1280. Полный прогон 32/32 (21067df) + затронутые auth+users 21/21 после фикса (EE-1).
- Чекеры бандла: 8 ✅; contract SKIP-RESOLVE — TS-клиента нет (D30), n/a.

## Соответствие коду дизайну (верификация реализации)
- [x] Поведение §2 «Авторизация» (D35, D40, D41, D42) — ДА
- [x] API §4 «Сотрудники»/«Вход» (URL, фильтры, конверт `{items,total,limit,offset}`, `LOGIN_TAKEN`/`SELF_BLOCK`/`LAST_ADMIN` 409) — ДА
- [x] SSR `/login`, `/logout`, `/users`, `/users/new`, `/users/:id` — ДА
- [ ] Swagger — N/A (swagger:no)
- [ ] Monitoring/алерты/rollout/runbook/perf — N/A (M, pipeline-light, не задекларированы)

## Sec 18 — API-контракт (слитая линза)
- Nullable: `is_active` Create — `*bool` (нет → true), Update — обязателен (400) — консистентно с plan.md §4.
- Error-body — единый формат `errorspkg`; SSR раскладывает `fields` под поля.
- Конверт списка ✅; N+1 нет.
- Breaking: удалён DELETE `/api/v1/users/:id` (D39) — MVP, не флагается.

## PRD соответствие
- N/A — PRD/draft-spec нет; D-решения plan.md: D19 ✅, D28 ✅, D29 ✅, D35 ✅, D39 ✅, D40 ✅, D41 ✅, D42 ✅.

## Статус фаз
- [x] pipeline-light: носитель — `0-setup/pipeline-state.md`; гейты после ca186d5 (vet/unit/lint 0 issues/integration-scoped -race/E2E auth+users 21/21) зелёные — pre-condition выполнено.

## Итог
APPROVED: блокеров 0; HIGH D41 и MEDIUM bcrypt-72 исправлены и покрыты тестами; 1 LOW принят как known issue.
