---
stage: 8-code-review
feature: one-car-order
tier: S
verdict: APPROVED
high: 0
medium: 2
low: 5
---
# Code Review: one-car-order — APPROVED
**Review level:** Slim (tier: S) + Sec 18 API-линза (spec-track merged)

Источник истины: задача + «Решения пользователя» `0-setup/pipeline-state.md` (draft-spec/design-*.md нет). База `cd5ab29`, HEAD `86f3840`.
Pre-conditions (судейство): make build OK; go test 18 ok; golangci-lint 1 issue `calendar_http_live_test.go:113` pre-existing; integration 20 пакетов ok; чекеры 9/9 pass; E2E 9 специй 69 passed (`9-validation/evidence/last-run.json`).

## Блокеры
нет

## Серьёзные замечания (исправить в этом PR)
1. **[migrations/0017_one_car_order.up.sql]** Нет интеграционного теста бэкфилла `car_id` (3 шага + DROP). Прецедент есть: `internal/features/clients/migration_soft_delete_integration_test.go` (`migrate.New` → `Migrate(N)` → сид → `Migrate(N+1)` → asserts). Сейчас 0017 проходится только неявно — `internal/features/board/subtask_products_position_integration_test.go:68-69` (`Migrate(5)` → defer `Up()`), без единой проверки. Корректность подтверждена лишь сухим прогоном на dev-БД (45 заказов). Нужен `migration_one_car_order_integration_test.go`: `Migrate(16)` → сид:
   - (a) заказ с 2 своими + 1 чужой единицей в `task_equipment` → первая своя по `plate_number, id`;
   - (b) заказ без `task_equipment`, у клиента удалённая + действующая → действующая;
   - (c) клиент без машин: title с мусором/длиной > 20 → нормализованный госномер ≤ 20; два заказа с одним title → одна машина; title из одних спецсимволов → `'БЕЗ НОМЕРА'`;
   - (d) мягко удалённый клиент → машина `deleted_with_client = true` с его `deleted_*`;
   - → `Migrate(17)`: `car_id NOT NULL`, владелец = `client_id`, `task_equipment`/`subtasks.equipment_id` отсутствуют → `Migrate(16)`: `task_equipment` восстановлена из `car_id`.
2. **[e2e/fixtures/auth.ts:517-534, e2e/fixtures/board-types.ts, e2e/fixtures/board.ts]** Общие фикстуры изменены: `createTask` сам дописывает новую машину, `updateSubtask` больше не шлёт `equipment_id`, из `UpdateTask` убран `client_id`. Эти фикстуры используют 11 специй вне прогона: `board-guards`, `calendar-gantt`, `executor-payouts` (26 вызовов), `new-order-capacity`, `order-attachments`, `pagination` (10), `soft-delete-board-columns`, `soft-delete-catalog`, `soft-delete-core` (6), `stock`, `users`. Для них проверены только `tsc --noEmit` и `--list`, реального прогона нет (TD-2). До мержа прогнать их или весь набор (chromium headed workers=1) и приложить артефакт.

## Незначительные замечания (можно в следующем PR)
1. **[migrations/0017_one_car_order.up.sql:9]** Инвариант «`tasks.client_id` = владелец `tasks.car_id`» держит только сервис: `TaskService.resolveCar` (`tasks_service.go:154`) плюс неизменяемость. В prod-коде нет `UPDATE` для `tasks.client_id`/`car_id` и для `equipment.client_id` — проверено. SQL-фикстуры и сиды пишут `tasks` напрямую, и БД нарушение не поймает. Как защиту в глубину стоит добавить составной FK: `UNIQUE (id, client_id)` на `equipment` и `FOREIGN KEY (car_id, client_id) REFERENCES equipment (id, client_id)` на `tasks`. Тогда бэкфилл и фикстуры станут самопроверяемыми.
2. **[migrations/0017_one_car_order.up.sql:11-29]** Для заказа с несколькими машинами (шаг 1) и для клиента с несколькими машинами (шаг 2) машина выбирается по алфавиту госномера, то есть произвольно. До DROP в `subtasks.equipment_id` ещё есть сигнал, с какой машиной работали. Лучше сначала брать машину, чаще всего указанную в услугах заказа, и только потом сортировать по `plate_number`. История второй машины такого заказа теряет его — осознанно (light-progress «Решения»), но в отчёте стоит это явно отметить. На dev затронуто 4 + 1 заказ.
3. **[0-setup/pipeline-state.md: axes]** Режим совместимости не задан («дефолты»), а по §CM-1 без пометки действует `prod`. В `prod` `DROP TABLE task_equipment` без expand-contract и удаление `/tasks/:id/equipment`, `card.equipment[]`, `subtasks[].equipment_id` и полей истории `accepted/completeness/condition` были бы BLOCKER. Блокером не считаю: удаление прямо требует задача пользователя, потребитель API — только SSR-JS этого же репо (деплоится атомарно), down-миграция есть, у прежних фич проекта режим MVP (DS-022). Зафиксировать `Совместимость: MVP` в pipeline-state. Побочный эффект: вкладка, открытая до деплоя, на сохранении шапки получит 400 `BAD_REQUEST`. Ассеты отдаются с `no-cache`, так что перезагрузка лечит.
4. **[web/ssr/_shared/js/task-card.js:1679-1683, :1663]** Мёртвый код и устаревшие комментарии:
   - ветка `clientGone` недостижима: PUT больше не шлёт `client_id`, а ошибки полей POST ловятся раньше (`!taskId && err.fields`);
   - комментарий «разделы оборудования/услуг» устарел;
   - `internal/features/board/stock_engine_integration_test.go:53` — в комментарии осталось `task_equipment`;
   - `internal/features/board/board_fuzz_test.go:310` — сиды `FuzzSubtaskLogBody` с `equipment_id`, хвост бывшего фаззера оборудования (безвреден).
5. **[internal/features/board/order_card_http_live_test.go:452]** Отказ PUT с `client_id`/`equipment_id`/`car_*` → 400 закреплён только для `/tasks/:id`. Для `PUT /api/v1/subtasks/:id` с `equipment_id` → 400 `BAD_REQUEST` явной проверки нет — держится на глобальном `strictJSON` (`internal/server/router.go:46`). Добавить одну проверку в live-тест.

## Вердикты по пунктам особого внимания
1. **Миграция 0017 — корректна; теста нет (MEDIUM-1).**
   - Шаг 1: `DISTINCT ON task_id`, только единицы клиента заказа (`e.client_id = o.client_id`), чужие пропускаются → шаг 2.
   - Шаг 2: действующая раньше удалённой.
   - Шаг 3 срабатывает только у клиентов совсем без машин, поэтому дублей с существующими единицами нет.
   - Госномер: `left(…,20)` + `btrim` удовлетворяют CHECK `0003:22`. Формат совпадает с `clients.NormalizePlateNumber` (схлопнутые пробелы, upper).
   - `DISTINCT (client_id, plate)` гарантирует одну машину на пару. Шаг тотален (каждый заказ имеет клиента по FK), поэтому `SET NOT NULL` упасть не может.
   - Мягко удалённый клиент → `deleted_at/by/reason` клиента + `deleted_with_client = true`. Проходит `equipment_deleted_chk` и `equipment_deleted_with_client_chk` (`0013:24-29`), согласовано с восстановлением клиента (`clients/soft_delete.go:139-145`).
   - Файл выполняется одним simple-query, то есть в одной неявной транзакции.
   - Down восстанавливает `task_equipment` и индексы как в `0005`. Потери (комплектность пустая, `subtasks.equipment_id` проставлен всем услугам) задокументированы в комментарии. Down реально исполняется в `subtask_products_position_integration_test.go:68`.
2. **Инвариант «клиент заказа = владелец машины» — соблюдается.**
   - Create (`resolveCar`): найденная машина → `client_id := owner` (при переданном чужом `client_id` → 400 `foreign_client`); новая машина → создаётся у `client_id`/`new_client`.
   - Update не трогает ни `client_id`, ни `car_id` (`tasks_repo.go` `Update`, `taskRow` комментарий).
   - `equipment.client_id` не меняется: `clients/repo.go:420` правит только `plate_number`.
   - Удаление и восстановление клиента/машины меняют только `deleted_*` и не трогают связи.
   - Сиды: `validateDemoData` сверяет `clientIdx`; `seedDeletedTask` использует `vesta` → `clients[0]`; инвариант проверяется в `seeddemo_integration_test.go:175-178`.
   - Фикстуры (выборочно): CTE `INSERT INTO equipment` с тем же `client_id`.
   - Гонка удаления машины/клиента при создании заказа: `CarForOrder` (`tasks_repo.go:352`, `FOR SHARE OF e`) перенесён дословно из `task_equipment_repo.go`; race-тесты `TestRace_CarOrderCreateWaitsForUnitDelete`/`TestRace_TaskCreateWaitsForClientDelete` зелёные.
   - Защиты на уровне БД нет (LOW-1).
3. **PUT `/tasks/:id` и `/subtasks/:id` — клиенты не сломаны.**
   - Глобальный `strictJSON` (`router.go:46`).
   - `task-card.js` `headerPayload()` (`:1600`) шлёт `title, column_id, assignee_id, due_date, is_paid, description` + `version`, без `client_id`. Снимок услуги (diff `:488`) — `status, assignee_id, due_date, price` + patch-поля.
   - В остальных SSR-JS (`board.js`, `calendar.js`, `executor-payouts.js`, `deleted.js`) PUT задач/услуг с удалёнными полями нет.
   - e2e: единственный сырой PUT (`board.spec.ts:432`, BD-011) исправлен оркестратором.
4. **Мягкое удаление машины/клиента с открытыми заказами — правила сохранены.**
   - Отказа как не было, так и нет (DS-014): заказ держит машину с бейджем — `CardRepo.Car` (`card_repo.go:55`) не фильтрует `e.deleted_at`, карточка рисует `card-car-deleted`/`card-car-owner-deleted`.
   - `DeletedRefs` (`tasks_repo.go:385`) переведён на `t.car_id`, нотисы восстановления `client_deleted`/`equipment_deleted` прежние (TS-026).
   - Purge машины, на которую ссылается заказ: FK RESTRICT → `mapPurgeErr` → 409, как раньше с `task_equipment`.
   - Новый заказ на удалённую машину или машину удалённого клиента → 400 `equipment_id: not_found`.
5. **Остатки удалённого блока — чисто, кроме LOW-4.**
   - grep по `card-equipment`, `equipment-history.js`, `card-eq-title`, `history-intake`, `data-intake`, `print-equipment`, `TASK_EQUIPMENT`, `CardEquipment`, `task_equipment` (вне миграций): пусто.
   - Локатор `OrderCardPage.subtaskEquipment` оставлен намеренно — для негативной проверки (`order-card.spec.ts:219`).
   - Удаление `{{ define "scripts" }}` в `equipment-history.html` безопасно: layout использует `{{ block "scripts" }}`.

## Slim-чеклист
- **Security (Sec 6): ок.**
  - Mass assignment сузился: `client_id` убран из `TaskHeader`, поэтому PUT больше не может перепривязать заказ.
  - Подмену машины при создании закрывает `foreign_client`/`plate_mismatch`.
  - Нарушение FK `tasks_car_id_fkey` маппится в ошибку поля (`tasks_service.go` `mapTaskFKError`), не в 500.
  - XSS: `renderOrderCar` экранирует `plate_number`, `client_name`, `equipment_id` через `esc()`.
  - Гарды маршрутов прежние; удалённые маршруты отдают 404/405 (live-тест).
- **Architecture fit: ок.**
  - `CarForOrder` перенесён в `TaskRepo`, `TaskEquipmentRepo`/`Service`/`validate` удалены, DI в `router.go` сокращён.
  - SQL только в repo-слое; `clients/history.go` читает таблицы доски SQL'ем (ADR-102, как раньше).
- **Error Handling: ок.**
  - Удалены `ErrTaskEquipmentNotFound`/`ErrTaskEquipmentInUse`, ссылок не осталось.
  - Коды полей `foreign_client`/`plate_mismatch` перенесены без изменений.
  - `Card` при гонке с удалением заказа: `errNotFound` → `ErrTaskNotFound`.
- **Tests: ок, кроме MEDIUM-1, MEDIUM-2 и LOW-5.**
  - Happy и error path покрыты live-тестами: создание по найденной/новой машине; 400 `required`/`foreign_client`/`plate_mismatch`/`not_found`; PUT с `client_id`/`equipment_id` → 400; маршруты оборудования → 404/405; история через `car_id`; гонки удаления.
- **Rollback: ок.** Down есть, исполняется в тестах; потери задокументированы.

## Sec 18 — API-контракт (слитая линза)
- Nullable: `card.car` — всегда объект: DB `NOT NULL`, в Go не указатель, по контракт-чекеру TS pass. Поля `equipment_id`/`plate_number`/`is_deleted` не nullable. Согласовано.
- Breaking (`card.equipment[]` → `card.car`, `subtasks[].equipment_id`, поля истории, эндпоинты) — LOW-3, единственный потребитель обновлён атомарно.
- Error-body: 400 `BAD_REQUEST`/`VALIDATION` в формате `pkg/errorspkg`, `details.fields[]` с путями `equipment_id`/`plate_number`/`client_id`.
- Конверт списков: история `{items,total,limit,offset}` прежняя.
- N+1: в `Card` запрос `Equipment` с подзапросом-историей заменён одним `QueryRow` `Car`. В поиске доски `EXISTS` по `t.car_id`, есть индекс `tasks_car_id_idx`.
- §API-1 (snake_case, kebab): чекер pass.

## Соответствие кода задаче (design-файлов нет — spec-track без draft-spec)
- Схема (`car_id NOT NULL` FK equipment, без комплектности/состояния; DROP `task_equipment`, `subtasks.equipment_id`) — ДА
- API (убраны POST/PUT/DELETE `/tasks/:id/equipment`; PUT не меняет клиента/машину; создание без машины → 400) — ДА
- Карточка («госномер · владелец · История работ», клиент/машина только чтение, без выбора машины в услугах) — ДА
- Печать актов — одна машина, только госномер — ДА
- История машины `/equipment/:id/history` через `car_id`, без комплектности и без правки — ДА
- Переименование equipment→car в коде/БД не делалось — ДА (`CardCar`/`car` — только имена DTO, как в задаче)

## PRD соответствие
n/a — PRD/draft-spec нет.

## Статус фаз
- `0-setup/light-progress.md` — все пункты `[x]`, DONE — ДА

## Вне скоупа (pre-existing, не учтено в счётчиках)
- Возможный deadlock: создание заказа держит машину `FOR SHARE`, затем FK-проверка `tasks.client_id` берёт `FOR KEY SHARE` на клиента; встречно удаление клиента держит клиента `FOR UPDATE` и ждёт `FOR NO KEY UPDATE` на машину. Если жертвой станет board, `withTx` повторит запрос на 40P01 (`board/tx.go:49`), а `clients` — нет. Код перенесён дословно, дифф поведение не менял.
