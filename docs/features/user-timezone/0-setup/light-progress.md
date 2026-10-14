# light-progress: user-timezone

## Backend
- [x] миграция 0013_user_timezone (up/down)
- [x] pkg/общий валидатор пояса + unit
- [x] users: repo/dto/validate/service/handler/routes (timezone в POST/PUT/GET, PUT /users/me/timezone)
- [x] auth: repo/dto/handler (timezone в /auth/me)
- [x] middleware/auth.go: пояс в locals (на каждый запрос из БД)
- [x] ssr: Viewer.Timezone, шаблонная функция, data-timezone на body
- [x] seeddemo: admin Almaty, один исполнитель Moscow

## Места показа времени
- [x] task-card.js fmtDateTime / fmtDate
- [x] board.js «сегодня» по поясу
- [x] executor-payouts.js
- [x] stock-transfers.js
- [x] equipment-history.html (SSR)
- [x] print-intake.html / print_ssr.go
- [x] print-handover.html / card_service.go
- [x] прочие места (fff-поиск): board tasks_repo.go List — фильтр due soon/overdue и subtasks_late (current_date → «сегодня» пользователя, TaskFilter.Today); TaskRepo.Get subtasks_late тоже переведён (fix-loop 1: Actor.Today — один источник «сегодня» на пакет board)

## UI
- [x] select в сайдбаре (base.html + JS)
- [x] поле в форме сотрудника (form.html + ssr.go)

## Тесты
- [x] TS-021 integration (LiveSuite)
- [x] TS-022 integration + миграционный тест
- [x] unit: валидатор пояса, Go-форматтер
- [x] TS-F5 фикстура времени (e2e)
- [x] TS-040 E2E
- [x] TS-041 E2E
- [x] TS-042 E2E
- [x] правка существующих E2E (order-print, executor-payouts, stock-transfers, order-card, equipment*)

## Self-checks
- [x] go build / vet / unit / integration по пакетам / node --check / tsc / playwright --list

## Заметки
- JS-формат унифицирован: «DD.MM.YYYY HH:MM» (в payouts/transfers раньше была запятая toLocaleString)
- web/ssr/_shared/js/timezone.js = KT.time + обработчик select (testid timezone-select, timezone-toast-error); форма: user-timezone, error-timezone
- go build + go test ./... (unit) зелёные на момент после backend
- Go integration ЗЕЛЁНЫЕ: users (timezone_http_live_test.go, migration_integration_test.go), board (timezone_http_live_test.go — TS-041 print/due), clients (equipment-history TS-041 блок), auth, seeddemo
- Остаётся: E2E (fixtures/auth.ts timezone+TS-F5, ShellPage/UsersPage, spec user-timezone.spec.ts TS-040/041/042), tsc, playwright --list, финальные self-check
- E2E написаны (НЕ прогонялись — зона оркестратора): e2e/fixtures/timezone.ts (TS-F5), e2e/tests/user-timezone.spec.ts (TZ-001/002 TS-040, TZ-010/011 TS-041, TZ-020 TS-042); существующие специи правки не потребовали (ожидания по времени — только regex даты)
- Все self-check зелёные: go build, go vet (integration tag), go test ./... , integration users/auth/board/clients/seeddemo, node --check, tsc, playwright --list
- ВСЁ ГОТОВО — остался только итоговый отчёт
- fix-loop 2: pkg/timezone — детерминированный Valid (posix/, right/, localtime, posixrules отклоняются) + кэш *time.Location (sync.Map)
