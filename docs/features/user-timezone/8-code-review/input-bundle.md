# Review Bundle: user-timezone — CHECKS-FAILED
**Base:** beb6d609439a91b8ec0d0b41429739359edf089e | **HEAD:** 2340483 | **Generated:** 2026-10-07 18:08 | diff: 70 files changed, 6039 insertions(+), 108 deletions(-)

> Линзы ревью: читай ЭТОТ бандл + patch своей линзы. Полное чтение research/draft-spec/PRD
> и перегон чекеров ЗАПРЕЩЕНЫ (вердикты ниже; детали — checks/<name>.txt; точечный grep по DS-NNN разрешён).

## Чекеры (детерминированные — НЕ перегонять)
| Чекер | Вердикт | exit | Нарушения (первые строки) |
|---|---|---|---|
| go_naming | ✅ | 0 | — |
| api_conventions | ✅ | 0 | — |
| contract | ✅ | 0 | — |
| date_bound | ✅ | 0 | — |
| fuzz_verdict | ✅ | 0 | — |
| live_suite | ✅ | 0 | — |
| fixture_sync | ✅ | 0 | — |
| e2e_mode | ✅ | 0 | — |
| rule_usage | ✅ | 0 | — |
| e2e_drift | ❌ | 1 | e2e-drift: FAIL (режим: TS (spec-track §ST-1.4), doc: 3 id, code: 0 в 0 файлах, проблем: 3)<br>DRIFT: TS-040 есть в draft-spec.md, теста test('TS-040: …') нет (сценарий без теста)<br>DRIFT: TS-041 есть в draft-spec.md, теста test('TS-041: …') нет (сценарий без теста)<br>DRIFT: TS-042 есть в draft-spec.md, теста test('TS-042: …') нет (сценарий без теста) |

## Diff по линзам
| Линза | Файлов | Patch |
|---|---|---|
| backend | 31 | 8-code-review/diff-backend.patch |
| migrations | 2 | 8-code-review/diff-migrations.patch |
| other | 15 | 8-code-review/diff-other.patch |
| docs | 22 | — (артефакты, не ревьюятся построчно) |

### backend
- `M` internal/features/auth/auth_http_live_test.go
- `M` internal/features/auth/dto.go
- `M` internal/features/auth/handler.go
- `M` internal/features/auth/repo.go
- `M` internal/features/auth/service.go
- `M` internal/features/board/board_fuzz_test.go
- `M` internal/features/board/card_service.go
- `M` internal/features/board/print_ssr.go
- `M` internal/features/board/tasks_filter.go
- `M` internal/features/board/tasks_handler.go
- `M` internal/features/board/tasks_repo.go
- `M` internal/features/board/tasks_service.go
- `A` internal/features/board/timezone_http_live_test.go
- `M` internal/features/clients/clients_http_live_test.go
- `M` internal/features/users/dto.go
- `M` internal/features/users/handler.go
- `A` internal/features/users/migration_integration_test.go
- `M` internal/features/users/repo.go
- `M` internal/features/users/routes.go
- `M` internal/features/users/service.go
- `M` internal/features/users/ssr.go
- `A` internal/features/users/timezone_http_live_test.go
- `M` internal/features/users/validate.go
- `M` internal/seeddemo/seeddemo.go
- `M` internal/seeddemo/seeddemo_integration_test.go
- `M` internal/server/middleware/auth.go
- `M` internal/ssr/render.go
- `M` internal/ssr/render_test.go
- `M` internal/ssr/shell.go
- `A` pkg/timezone/timezone.go
- `A` pkg/timezone/timezone_test.go

### migrations
- `A` migrations/0013_user_timezone.down.sql
- `A` migrations/0013_user_timezone.up.sql

### other
- `M` e2e/fixtures/auth.ts
- `A` e2e/fixtures/timezone.ts
- `M` e2e/pages/ShellPage.ts
- `M` e2e/pages/UsersPage.ts
- `A` e2e/tests/user-timezone.spec.ts
- `M` web/ssr/_layout/templates/layouts/base.html
- `M` web/ssr/_shared/js/board.js
- `M` web/ssr/_shared/js/executor-payouts.js
- `M` web/ssr/_shared/js/stock-transfers.js
- `M` web/ssr/_shared/js/task-card.js
- `A` web/ssr/_shared/js/timezone.js
- `M` web/ssr/board/templates/pages/print-handover.html
- `M` web/ssr/board/templates/pages/print-intake.html
- `M` web/ssr/clients/templates/pages/equipment-history.html
- `M` web/ssr/users/templates/pages/form.html

### docs
- `A` docs/features/user-timezone/0-setup/draft-spec.slice-light.md
- `A` docs/features/user-timezone/0-setup/idioms.md
- `A` docs/features/user-timezone/0-setup/light-progress.md
- `A` docs/features/user-timezone/0-setup/passport.md
- `A` docs/features/user-timezone/0-setup/pipeline-state.md
- `A` docs/features/user-timezone/0-setup/timeline.jsonl
- `A` docs/features/user-timezone/8-code-review/checks/api_conventions.txt
- `A` docs/features/user-timezone/8-code-review/checks/contract.txt
- `A` docs/features/user-timezone/8-code-review/checks/date_bound.txt
- `A` docs/features/user-timezone/8-code-review/checks/e2e_drift.txt
- `A` docs/features/user-timezone/8-code-review/checks/e2e_mode.txt
- `A` docs/features/user-timezone/8-code-review/checks/fixture_sync.txt
- `A` docs/features/user-timezone/8-code-review/checks/fuzz_verdict.txt
- `A` docs/features/user-timezone/8-code-review/checks/go_naming.txt
- `A` docs/features/user-timezone/8-code-review/checks/live_suite.txt
- `A` docs/features/user-timezone/8-code-review/checks/rule_usage.txt
- `A` docs/features/user-timezone/8-code-review/diff-backend.patch
- `A` docs/features/user-timezone/8-code-review/diff-migrations.patch
- `A` docs/features/user-timezone/8-code-review/diff-other.patch
- `A` docs/features/user-timezone/8-code-review/input-bundle.md
- `A` docs/features/user-timezone/9-validation/evidence/e2e-run.txt
- `A` docs/features/user-timezone/9-validation/evidence/last-run.json

## Экстракты документов (вместо полного чтения)
### draft-spec.md (экстракт: шапка + DS + дельта + Impact + TS)
# Draft Spec: Часовой пояс пользователя

**Tier:** M · **Совместимость:** MVP · **Режим:** new

**Дата:** 2026-10-07 | **Статус:** draft → final (после подтверждения пользователя) | **Tier:** M
**Совместимость:** MVP (DS-027 — контекст, реализуется в `transfers-core`) — живых данных нет
**UI-scope:** swagger:no | adaptive:min | themes:both (DS-028 — контекст, реализуется в `transfers-core`)
**Источник:** мастер-спека `docs/features/transfers/draft-spec.md` (§3 «Время/таймзоны», §4.5, §6.13, §6.15, §8.4) + её журнал `docs/features/transfers/draft-spec-decisions.md` (батч 6, В-24 и четыре уточнения)
**Часть 2 из 6 мастер-спеки `transfers`; зависит от: —** (независима). Номера DS и TS — мастер-идентификаторы, не перенумерованы.

> После мержа этой части: у каждого пользователя есть часовой пояс (по умолчанию Алматы); все существующие экраны и печатные формы показывают дату и время по нему, а не по браузеру или серверу. Перемещения — ещё старый экран `/stock-transfers`, его время тоже идёт по поясу. Фильтра «период» в списке перемещений ещё нет (часть 4 `transfers-partners`).

## §9. Решения (DS-index)
| DS | Формулировка | Источник | Статус |
|---|---|---|---|
| DS-031 | Время храним в UTC, показываем по часовому поясу из настройки пользователя | мастер, батч 6 | closed |
| DS-032 | Пояс пользователя применяется на всех экранах системы, где показывается дата или время | мастер, батч 6 (уточнение) | closed |
| DS-033 | Пояс меняет сам пользователь; умолчание — Алматы (`Asia/Almaty`) для новых и существующих | мастер, батч 6 (уточнение 2) | closed |
| DS-056 | Пояс любому пользователю может менять и администратор — поле в форме создания/правки в «Сотрудниках»; исполнитель чужой пояс не меняет | мастер, батч 6 (уточнение 4) | closed |
| DS-034 | Фильтр «период» в списке перемещений, границы дня по поясу пользователя | мастер, батч 6 (уточнение 3) | closed — контекст, реализуется в `transfers-partners` |
| DS-027 | Совместимость — MVP | мастер, батч 6 | closed — контекст, реализуется в `transfers-core` |
| DS-028 | UI-scope: swagger:no, adaptive:min, themes:both | мастер, батч 6 | closed — контекст, реализуется в `transfers-core` |
| DS-051 | Отдельной таблицы аудита нет | мастер, zap | closed — контекст, реализуется в `transfers-core` |
| DS-052 | UI-конвенции: ошибка под полем, тост справа вверху 5 с | мастер, inferred | closed — контекст, реализуется в `transfers-core` |

## §10. Открытые вопросы (status: open)
Открытых вопросов нет.

## §11. Тестовые сценарии (черновик — вход для design-e2e-strategy, TD-6.8)
UI-scope: swagger-сценариев нет; адаптив — проверка overflow сайдбара с select'ом в составе TS-040; темы — select читаем в обеих темах, в составе TS-040.

### Unit / Integration (Go)
- **TS-021.** Часовой пояс, свой: `e:PUT /api/v1/users/me/timezone` с `Europe/Moscow` → 200; `Mars/Olympus`, пустая строка, 65 символов, `../../etc/passwd` → 400 `timezone: invalid` / `required`; `e:GET /api/v1/auth/me` отдаёт новое значение сразу следующим запросом; чужой `user_id` в теле игнорируется — DS-031, DS-033
  - негатив: без сессии → 401
- **TS-022.** Часовой пояс, администратор другому: `e:PUT /api/v1/users/:id` с `timezone` → 200, значение в ответе; без поля — пояс не меняется; `e:POST /api/v1/users` без поля → `Asia/Almaty`; невалидный → 400 `details.fields[]` (`timezone: invalid`); `e:GET /api/v1/users` и `e:GET /api/v1/users/:id` содержат `timezone`; миграция: существующие пользователи получили `Asia/Almaty` — DS-056, DS-033
  - негатив: executor `e:PUT /api/v1/users/:id` → 403

### E2E (Playwright)
- **TS-040.** Смена своего часового пояса — экран §8.1 — DS-031, DS-033
  - шаги: войти исполнителем → сайдбар → «Москва (UTC+3)»; проверка: страница перезагрузилась, время одной и той же записи сдвинулось на −2 часа относительно пояса Алматы; после reload и повторного входа значение сохраняется; при вьюпорте 375×812 сайдбар без горизонтальной прокрутки; select читаем в светлой и тёмной теме; негатив: нет
- **TS-041.** Пояс применяется во всей системе — экраны: карточка заказа (журнал услуг, дата оплаты исполнителю), «Выплаты», «Перемещения», история оборудования, печатные формы «Приём» и «Выдача» — DS-032
  - шаги: пользователь с поясом Москва открывает каждый экран; проверка: времена на 2 часа меньше, чем у пользователя с поясом Алматы для тех же записей (фикстура TS-F5); негатив: метка срока заказа «сегодня / просрочен» на доске считается по поясу пользователя, не браузера (браузер запущен с `timezoneId`, отличным от пояса пользователя)
- **TS-042.** Администратор меняет пояс сотруднику — экран §8.1 — DS-056
  - шаги: admin → «Администрирование → Сотрудники» → форма → «Часовой пояс» → «Сохранить» → вход этим сотрудником; проверка: даты в его поясе, select в сайдбаре показывает заданное значение; негатив: в форме создания по умолчанию «Алматы (UTC+5)»; невалидный пояс подменой значения → под полем «Неизвестный часовой пояс»

### Фикстуры и сиды (АКЦЕНТ — хронический пропуск)
- **TS-F5.** Фикстура времени: запись с моментом `2026-10-06T19:30:00Z` (00:30 7 октября по Алматы, 22:30 6 октября по Москве) и два пользователя с разными поясами (`Asia/Almaty`, `Europe/Moscow`) — для TS-040, TS-041; в части `transfers-partners` та же фикстура используется для TS-023 — DS-032
- seed-demo (`internal/seeddemo/seeddemo.go`): админ — `Asia/Almaty`, один исполнитель — `Europe/Moscow`; обновить в той же фазе, что миграция (§TD-1)
- e2e (`e2e/fixtures/auth.ts`): wire-тип пользователя получает `timezone`; контексты Playwright запускать с явным `timezoneId`, отличным от пояса пользователя, чтобы тест ловил форматирование «по браузеру»; payload'ы типизировать wire-типами из исходников (§TD-6.2)

## §Impact — точки касания существующего кода
Источник: `impact_scan.py --sample internal/features/users,internal/features/auth --terms timezone,toLocaleString,PrintedAt,fmtDateTime` (114 находок, большинство — импорты `fixtures/auth` в e2e; значимые внесены ниже) + поиск мест форматирования времени (fff: `toLocaleString`, `fmtDate`, `fmtDateTime`, `.Format `, `PrintedAt`, `new Date(`).

| # | Класс | Что задеваем (file:line / объект) | Дельта фичи | Решение | DS |
|---|---|---|---|---|---|
| 1 | SHARED-DEP | `internal/features/users/dto.go:7-46`, `validate.go`, `repo.go`, `service.go`, `handler.go`, `ssr.go`, `web/ssr/users/templates/pages/form.html` | поле `timezone` в запросах, ответах, форме | править | DS-056 |
| 2 | ENUMERATION | `internal/features/users/routes.go:12-16` | `/api/v1/users` целиком под admin-guard, а `PUT /users/me/timezone` нужен любому авторизованному — регистрировать вне admin-группы и раньше `/:id` | править | DS-033 |
| 3 | SHARED-DEP | `internal/features/auth` (`/auth/me`, dto, repo) | поле `timezone` | править | DS-031 |
| 4 | SHARED-DEP | `internal/server/middleware/auth.go:78` (`ssr.Viewer{Name, Role}`), `internal/ssr/shell.go` (`Viewer`, `Shell`) | пояс пользователя в `Viewer` → доступен layout'у и шаблонам | править | DS-031 |
| 5 | SHARED-DEP | `internal/ssr/render.go:88` (`Funcs`) | функция шаблонов «время в поясе пользователя» | добавить | DS-032 |
| 6 | FRONTEND | `web/ssr/_layout/templates/layouts/base.html:34-37` | select пояса, `data-timezone` на `<body>` | править | DS-033 |
| 7 | FRONTEND | `web/ssr/_shared/js/task-card.js:529-533`, `:627-633` и вызовы `:411,492,567,578,832,879` | общий форматтер с `timeZone` | править | DS-032 |
| 8 | FRONTEND | `web/ssr/_shared/js/executor-payouts.js:148-152,186` | общий форматтер | править | DS-032 |
| 9 | FRONTEND | `web/ssr/_shared/js/stock-transfers.js:118-122` | общий форматтер (файл будет переписан в части 3 — форматтер уже общий) | править | DS-032 |
| 10 | FRONTEND | `web/ssr/_shared/js/board.js:385-387` | «сегодня» по поясу пользователя | править | DS-032 |
| 11 | FRONTEND | новый общий модуль форматирования (сейчас четыре копии в строках 7–10) | один `KT.fmtDateTime` / `KT.fmtDate` / `KT.today` с `timeZone` из `data-timezone` | добавить; подключить в `base.html` | DS-032 |
| 12 | SHARED-DEP | `web/ssr/clients/templates/pages/equipment-history.html:27,84` + хендлер `internal/features/clients/history.go` | даты в поясе пользователя | править | DS-032 |
| 13 | SHARED-DEP | `web/ssr/board/templates/pages/print-intake.html:11`, `print-handover.html:12,81`, `internal/features/board/print_ssr.go:52`, `card_service.go:231` | дата печати и оплаты в поясе пользователя | править | DS-032 |
| 14 | ENUMERATION | `internal/seeddemo/seeddemo.go`, `e2e/fixtures/auth.ts` | пояса демо-пользователей, wire-тип | править | DS-033 |
| 15 | FK | `migrations/` | новая миграция: `users.timezone` (номер — следующий свободный на момент мержа) | добавить | DS-033 |
| 18 | ENUMERATION | `project/.scaffold-lock.json:15-19,28` — хэши файлов каркаса `internal/features/auth/*`, `internal/server/middleware/auth.go` | правка этих файлов расходится с lock-файлом каркаса `/project-bootstrap` | при планировании решить: обновить lock либо вынести дельту в не-каркасные файлы | DS-031 |
| 19 | ENUMERATION | `e2e/tests/order-print.spec.ts`, `e2e/tests/executor-payouts.spec.ts`, `e2e/tests/stock-transfers.spec.ts`, `e2e/tests/order-card.spec.ts` | существующие проверки дат могут зависеть от пояса браузера | прогнать и поправить ожидания под пояс пользователя | DS-032 |
| 16 | CONFIG | env-ключи | не задеваются (пояс сервера и контейнера на показ больше не влияет) | — | DS-031 |
| 17 | MONITORING | дашборды/алерты | не задеваются | — | — |

**Вне каталога фичи:** есть — internal/features/users, internal/features/auth, internal/features/board, internal/features/clients, internal/server/middleware, internal/ssr, internal/seeddemo, web/ssr/_layout, web/ssr/_shared/js, web/ssr/users, web/ssr/clients, web/ssr/board, migrations, e2e

### prd.md
MISSING

## Инвентарь доков (точечный grep разрешён, полное чтение — нет)
- draft-spec.md — 38KB
- 0-setup/draft-spec.slice-light.md — 31KB
- 8-code-review/input-bundle.md — 20KB
- 0-setup/idioms.md — 6KB
- 0-setup/passport.md — 5KB
- 0-setup/light-progress.md — 3KB
- draft-spec-decisions.md — 1KB
- 0-setup/pipeline-state.md — 0KB
- 9-validation/evidence/last-run.json — 0KB

## Паспорт (выжимка; полный — 0-setup/passport.md)
# Passport: user-timezone (generated — руками не править; stage_bundle.py)
**Tier:** M | **Совместимость:** MVP (pipeline-doctrine.md §CM-1; ? → уточни у оркестратора)
**Трек:** обычный (prefs нет — дефолт) — режимы агентов (`spec-track delta/micro/mini/merged`) включает ТОЛЬКО строка `Режим:` в prompt оркестратора; DS/TS-индексы ниже присутствуют на ЛЮБОМ треке при наличии draft-spec.
**Layout:** автодетект (tools/layout_resolve.py)
**UI-scope (pipeline-doctrine.md §US-1, шапка draft-spec):** swagger:no | adaptive:min | themes:both
