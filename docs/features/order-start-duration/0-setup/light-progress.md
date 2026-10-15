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

## Fix-loop 1 (код-ревью 8-code-review/output.md)

- [x] MEDIUM-1: бэкфилл — открытые без валидного срока → окончание = день миграции (≤60); выданные без срока → 1; тест миграции (случаи a,b,c,c2,d,e,f,g,h,h2 + DEFAULT + down/up)
  - dev-БД (уже v18): UPDATE по открытым с duration_days = 1 и due_date < сегодня → 7 строк; после — 0 открытых просроченных, 102 с окончанием сегодня; DEFAULT start_date на dev-БД тоже приведён ALTER-ом
- [x] MEDIUM-2: выдан раньше start_date → места не занимает (build: continue), на Ганте нет; repo-фильтр согласован (окно не влияет); service_test/gantt_test (TestBuildGantt_IssuedBeforeStart)/calendar live обновлены
- [x] LOW-1: DEFAULT start_date → (now() AT TIME ZONE 'Asia/Almaty')::date
- [x] LOW-3: убраны «срок не указан» (gantt.go) и null-ветки due (board.js); комментарии gantt.go GanttRow, app.css (Гант), tasks_validate.go normalize; типы DTO as-is
- [x] LOW-4: e2e PLAN-003 (order-plan.spec.ts) — метки 📅 и фильтр «Горит»/«Только просроченные»/«Любой срок» на будущем/сегодня/завтра/просроченном/выданном
- [x] Self-checks fix-loop 1: build/vet (+integration tag) OK; go test ./... 18 ok/0; integration board+calendar 369 PASS/0; node --check board/task-card/calendar OK; e2e tsc OK, order-plan --list 3 теста
- [x] Сид (доп. запрос): самодостаточный демо-сид под все фичи
  - 16 заказов на 16 разных машинах (client_id = владелец, в названии марка/модель/госномер); автопарки: ТаксиЛайн 4 заказа, СтройДоставка 2; машины без заказов — Kia Sportage, Ford Transit; снятый двигатель ВАЗ-21126 → машина Lada Priora Р126ОР 750 (капремонт, двигатель привезли снятым)
  - планы: диагностика/ТО 1–2 дня, ремонт двигателя 3–4 дня, капремонт 12; 6 записей на +1..+5 дней, 2 просроченных невыданных (Vesta — мехцех задержал ГБЦ, Qashqai — клиент не забирает), 3 «горящих» (окончание сегодня/завтра); 3 выданных с finished_at в прошлом (17:30 по Алматы, не раньше начала), оплата — в момент выдачи
  - загрузка при 8 местах: сегодня 7/8, +1..+4 — 6–7, +5 — 8/8 (под завязку), +6 — 5, дальше 2–3
  - created_at = приёмка/запись; журнал, вложения, сроки услуг — внутри окна приёмки…выдачи (validatePlan в validateDemoData); у записей без визита фото приёмки нет
  - удалённый дубль Vesta — с явным планом; тексты без «оборудован/комплектност/состояни» (услуга «ремня навесных агрегатов», характеристика «Состояние» убрана)
  - тесты: plan_test.go (unit: загрузка календаря, состав плана), seeddemo_integration_test (assertOrderPlans / assertCalendarLoad через calendar.Service / assertTexts, фото и окна времени)
- [x] Self-checks итог: build/vet OK; go test ./... 18 ok/0; integration seeddemo 10 PASS, board+calendar 369 PASS; node --check OK; e2e tsc OK, --list 189; fixture_sync PASS
