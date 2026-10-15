# one-car-order — light-progress (чеклист исполнителя)

Уточнения пользователя (приоритет над исходным заданием):
1. Машина задаётся ТОЛЬКО при создании; PUT /tasks/:id не меняет ни машину, ни клиента; в
   карточке клиент и машина — только чтение (без поиска/смены).
2. Комплектность/состояние УБРАНЫ полностью: в tasks только car_id; данные
   task_equipment.completeness/condition при миграции теряются осознанно; эндпоинта правки нет;
   в истории машины и в актах — без комплектности/состояния.

Решения:
- PUT /tasks/:id с client_id / equipment_id → 400 BAD_REQUEST (поля убраны из DTO, strictJSON
  проекта отклоняет неизвестные ключи).
- Бэкфилл car_id: машина из task_equipment (первая по plate_number, id) → иначе первая машина
  клиента заказа (не удалённая приоритетнее, по plate_number, id) → иначе новая машина клиенту
  с госномером из title (≤20 символов, иначе 'БЕЗ НОМЕРА').

- [x] Разведка: миграции, board (task_equipment*, tasks_*, card_*, subtasks*, print), clients, softdelete, seeddemo
- [x] Миграция 0017_one_car_order up/down
- [x] Backend board: схема DTO/repo/service (tasks.car_id), удаление task_equipment API (+ CarForOrder → TaskRepo, ClientExists удалён)
- [x] Backend board: subtasks без equipment_id
- [x] Backend board: card DTO `car` {equipment_id, plate_number, is_deleted}, печать (только госномер)
- [x] Backend clients (history через car_id, без комплектности; equipment-history.js удалён) / seeddemo (car на заказ). softdelete — ссылок на task_equipment нет
- [x] Тестовые INSERT INTO tasks без car_id во всех пакетах (board, calendar, catalog, clients, executorpayouts, users, seeddemo) — CTE `WITH car AS (INSERT INTO equipment …)`; HTTP-создание — helper withNewCar (board_test), apiTask/carTask (clients), plate+new_equipment (softdelete)
- [x] Live-тесты переписаны: order_card (car-блок: новый/найденный автомобиль, PUT с client_id/equipment_id/car_* → 400 BAD_REQUEST, старые /tasks/:id/equipment → 404/405), soft_delete, print, car_order, board (поиск по госномеру через car_id, создание без машины → 400), clients (история через car_id, TS-024/025/026, гонки создания заказа vs удаления машины)
- [x] Unit-тесты зелёные (`go test ./... -count=1`: 18 ok, 0 FAIL) — card_validate/tasks_validate/tasks_car_validate/board_fuzz поправлены
- [x] Integration-тесты поправлены и зелёные (свои пакеты board, clients, calendar, catalog, executorpayouts, users, softdelete, seeddemo; `-tags=integration -count=1 -v`: exit 0, 704 PASS / 0 FAIL / 0 SKIP)
- [x] Frontend: task-modal.html (секция #card-car-section/#card-car, скрыта у нового заказа), task-card.js (renderOrderCar «госномер · владелец · История работ», клиент заблокирован, без выбора машины в услугах, PUT без client_id), equipment-history.html без intake + equipment-history.js удалён, print-intake/handover — только госномер, app.css (.card-eq-title убран)
- [x] E2E: fixtures (auth.createTask сам дописывает новый автомобиль, createCarTask, без add/removeTaskEquipment, updateSubtask без equipment_id), board-types (NewTask/UpdateTask без client_id, CardCar), board.ts, OrderCardPage (car*-локаторы); специи order-card (OC-001/002/006 новый сценарий «строка машины, клиент/машина только чтение, шапка сохраняется» /008 история), soft-delete-clients, polish, user-timezone, board, calendar-new-order, new-order-autofill, new-order-plate, order-print. `npx tsc --noEmit -p .` — 0 ошибок; `--list` — 69 тестов в 9 специях, полный список 185/27
- [x] gofmt, golangci-lint (board, clients, seeddemo, server) — 0 issues; повторно unit 18 ok, integration board+calendar+clients+seeddemo 460 PASS / 0 FAIL
- [x] Миграция применена к dev-БД (`. scripts/dev/env.sh && go run ./cmd/migrate up` → version 17, dirty=f), бэкфилл:
      45 заказов; 23 — из task_equipment (4 с несколькими машинами → первая по госномеру), 1 — первая
      машина клиента, 21 — новые машины из названия (20 машин у 20 клиентов, e2e-мусор); car_id NULL = 0,
      несовпадений владельца = 0; task_equipment и subtasks.equipment_id удалены. Сухой прогон — BEGIN/ROLLBACK.
- [x] DONE (исполнитель завершил)

## Fix-loop 1 (код-ревью APPROVED с замечаниями)

- [x] MEDIUM-1: `internal/features/board/migration_one_car_order_integration_test.go` (тег integration, прецедент clients/migration_soft_delete): Migrate(16) → сид (a) 2 своих + чужая в task_equipment → первая своя по госномеру; (e) чаще в услугах → она; (b) без task_equipment / только чужая → действующая раньше удалённой; (c) клиент без машин → «РЕМОНТ ДВИГАТЕЛЬ И К» (≤20), два заказа с одним названием → одна машина, спецсимволы → «БЕЗ НОМЕРА»; (d) удалённый клиент → машина deleted_with_client с его deleted_*; Migrate(17) → NOT NULL/FK/индекс/нет task_equipment и subtasks.equipment_id/владелец = клиент; Migrate(16) → task_equipment (1 строка = car_id, пустые комплектность/состояние) и subtasks.equipment_id = car_id; повторный up — выбор прежний. PASS.
- [x] LOW-2: шаг 1 миграции — сначала машина, чаще всего указанная в subtasks.equipment_id заказа, затем plate_number, id (случай (e) теста). На dev-БД уже применён прежний шаг 1 (4 заказа с несколькими машинами — по госномеру); пере-бэкфилл через down/up невозможен (down знает только car_id) — не трогал.
- [x] LOW-4: task-card.js — убрана мёртвая ветка clientGone, ошибки полей PUT шапки → в шапку; комментарий «разделы оборудования/услуг» исправлен; stock_engine_integration_test.go (комментарий без task_equipment), board_fuzz_test.go (сиды FuzzSubtaskLogBody без equipment_id).
- [x] LOW-5: order_card_http_live_test.go Subtask_UpdateFields — PUT /api/v1/subtasks/:id с equipment_id → 400 BAD_REQUEST и строка услуги не изменилась (status/assignee/due/price/total).
- [x] Составной FK (car_id, client_id) — НЕ делаем, known issue (решение координатора).
- [x] Self-check: go build/vet (+ vet -tags=integration board) ok; node --check task-card.js ok; gofmt чисто; golangci-lint board (с тегом integration и без) — 0 issues; `go test ./... -count=1` — 18 ok / 0 FAIL; integration board+clients+executorpayouts (миграционные тесты проходят через 0017) — exit 0, 461 PASS / 0 FAIL.
- [x] DONE (fix-loop 1 завершён)
