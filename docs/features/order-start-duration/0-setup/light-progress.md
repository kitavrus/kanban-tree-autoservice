# light-progress — order-start-duration

- [x] Разведка: доктрина §§, миграции, board/calendar слои, фронт, e2e
- [x] Миграция 0018 up/down + integration-тест миграции (board/migration_order_start_duration_integration_test.go)
  - решение: пояс бэкфилла — Asia/Almaty (pkg/timezone.Default); кламп дней до 60
  - решение: start_date DEFAULT CURRENT_DATE, duration_days DEFAULT 1 (прямые INSERT тестов/служебные)
  - down: ALTER COLUMN due_date DROP EXPRESSION (значение плана сохраняется)
- [x] Backend board: DTO/validate/repo/service (start_date, duration_days; due_date вычисляемый)
  - решение: start_date окно 2000-01-01..2099-12-31 (out_of_range), формат — invalid_date; duration 1..60 — out_of_range
  - TaskCard (+ list/get/mutations) отдаёт start_date, duration_days, due_date
- [x] Backend calendar: repo (start_date) / dto doc / ssr подписи; build() и gantt — логика прежняя, проверена тестами
- [x] seeddemo на новые поля (startOffset/days, 2 с будущим началом) + проверка в seeddemo_integration_test
- [x] Unit-тесты (validate, fuzz-сигнатура, calendar build/gantt) — зелёные
- [x] Integration live-тесты board/calendar/timezone/clients/softdelete + новые кейсы — зелёные
  (board+calendar 368 PASS; clients, softdelete, seeddemo — ok)
- [x] Frontend: task-modal (task-start/task-duration/task-planned-end + слоты ошибок), task-card.js
  (план, подсказка окончания, загрузка дня начала, предупреждение по плановым дням, локальная
  валидация, ошибки плана в PUT — под полями), calendar.js/html (data-start, «+ Заказ с началом в
  этот день», маркер «Плановое окончание»), board.js (title метки 📅)
- [x] E2E: fixtures (createTask/updateTask — план по умолчанию), board-types, POM (NewOrderDialog/
  BoardPage/OrderCardPage), specs board/order-card/user-timezone/calendar-gantt/calendar-new-order/
  new-order-capacity + новый order-plan.spec.ts (PLAN-001, PLAN-002) — tsc OK, --list 188 тестов
- [x] Миграция на dev-БД v17 → v18: 116 заказов; 1 день — 105 (97 без срока + 6 со сроком раньше
  приёмки + 2 со сроком в день приёмки), 2..59 дней — 8, кламп 60 — 3; start_date 2026-10-14..15;
  due_date = start + days − 1 у всех (0 расхождений)
- [x] Self-checks: gofmt/vet OK, go test ./... 18 ok/0 fail, integration своих пакетов зелёные,
  fixture_sync PASS, api_conventions PASS, e2e tsc OK + --list OK
  (frontend_phase_gate.sh неприменим: нет каталога frontend/ — SSR-проект)
- [x] DONE (исполнитель завершил)
