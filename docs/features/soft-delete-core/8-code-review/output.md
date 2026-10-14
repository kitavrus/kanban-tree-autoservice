---
stage: 8-code-review
feature: soft-delete-core
tier: M
verdict: APPROVED
high: 0
medium: 0
low: 4
---
# Code Review: soft-delete-core — APPROVED
**Review level:** Standard (tier: M) · режим spec-track merged (Sec 17 по DS/TS, Sec 18 API-линза) · диапазон 022d8d6..c3195c3 (5529da1, 0199660, c3195c3) · повторное ревью после fix-loop (фикс: 0199660..c3195c3) · совместимость MVP (§CM-1)

Гейты (повторное судейство после c3195c3, не перегонялись): build ok; unit ok; lint 0; integration FULL `-race` 22 ok; E2E 213 passed, flaky 0 (`.last-run.json` passed). Чекеры (бандл 13:40, HEAD c3195c3): 8 pass; `contract` SKIP-RESOLVE — n/a (Vue нет, SSR); `e2e_drift` FAIL — ложный класс (канон заголовка `SD-00N (TS-0NN)`), все 8 E2E-TS покрыты.

## Блокеры (обязательно исправить до мержа)
Нет.

## Серьёзные замечания (исправить в этом PR)
Нет открытых.

### Закрыто в c3195c3
1. **ЗАКРЫТО — гонка удаление↔запись о работе.** `subtask_logs_repo.go` `Insert` / `mutate` (`Update`, `Delete`): одна транзакция, первым шагом `… JOIN tasks t … WHERE … t.deleted_at IS NULL FOR KEY SHARE OF t`, затем запись. `LockTask` удаления (`FOR UPDATE`) с `KEY SHARE` несовместим: либо удаление ждёт запись, либо запись ждёт удаление и после перечитывания условия получает 0 строк → `errNotFound`. Отдельные неблокирующие `SubtaskTaskID` / `Owner` удалены — окна между проверкой и записью нет.
2. **ЗАКРЫТО — гонка у вложений.** `attachments_repo.go` `Delete(ctx, id, allow)`: чтение строки с `FOR KEY SHARE OF t` + условие «не удалён» → права → `DELETE` → commit в одной транзакции; `attachments_service.go:128` файл стирается только после commit. `Insert`: `INSERT … SELECT … FOR KEY SHARE OF t` — вставка ждёт удаление и строки заказа не видит (0 строк → `TASK_NOT_FOUND`, загруженный файл убирается). Комментарий приведён в соответствие.
3. **ЗАКРЫТО — тело DELETE без JSON content-type** (было «незначительное» 5). `tasks_handler.go:121-126`: тело разбирается `json.Unmarshal` независимо от `Content-Type`; нечитаемое тело = нет причины → 400 `VALIDATION` `reason: required` с `details.fields[]`. Покрыто в `TestTS001_DeleteReasonValidation` (`text/plain`). Порядок «роль первой» не затронут (проверка роли — в гарде и первой строкой сервиса).
4. **ЗАКРЫТО — `--retries=1` в E2E** (было «незначительное» 6): повторный прогон — flaky 0.

## Проверка фикса
- **Взаимоблокировки — не найдено.** Запись части берёт ровно одну блокировку строки `tasks` (`KEY SHARE`), далее строки `subtask_logs` / `task_attachments` (и `KEY SHARE` на `subtasks` через FK при INSERT) — порядок tasks → subtasks → части, тот же, что у `LockTask`-путей, `Purge` (каскад) и `executorpayouts`. `columns` и `product_stock` запись частей не трогает — цикла с `Restore` (columns → tasks → product_stock) и `ColumnRepo.LockAll` → `TaskIDsForUpdate` нет. `KEY SHARE` совместим с `KEY SHARE` transfers (`checkRefs`) и с неключевым UPDATE заказа; конфликтует только с `FOR UPDATE` — ожидание одностороннее.
- **Порядок 404→403 сохранён.** `mutate` / `AttachmentRepo.Delete`: 0 строк (нет записи либо заказ удалён) → 404 своего вида; затем `allow` → 403 `SUBTASK_LOG_FORBIDDEN` / `ATTACHMENT_FORBIDDEN`; затем запись. При 403 транзакция откатывается, ничего не изменено. Не-автор по удалённому заказу получает 404 — как и до фикса.
- **Тест гонки не ложнозелёный.** `soft_delete_race_http_live_test.go`:
  - `TestRace_PartWritesWaitForTaskSoftDelete` держит `FOR UPDATE` + UPDATE отметки (те же шаги, что `TaskService.Delete`) и ждёт, пока все 5 запросов встанут в ожидание блокировки (`pg_stat_activity`, `wait_event_type = 'Lock'`). Без блокировки в коде 4 из 5 запросов прошли бы не дожидаясь — условие `== 5` не выполнилось бы, тест упал бы по таймауту. После commit проверяются 404 своего вида, неизменность записей, строки вложения, файла на диске и уборка файла отклонённой загрузки.
  - `TestRace_TaskSoftDeleteWaitsForPartWrite` — обратный порядок: удаление ждёт `KEY SHARE`, заказ не помечен до commit.
  - Счётчик ожидающих — по всему кластеру, но у сьюты свой контейнер PG (`pkg/testutil/base_suite.go:42`) — посторонних ожиданий нет. `sleep` нет.
  - Ограничение: во втором тесте запись части имитируется SQL-блокировкой теста, а не реальным HTTP-запросом — проверяется несовместимость блокировок, а не путь сервиса; путь сервиса покрыт первым тестом.

## Незначительные замечания (можно в следующем PR)
1. **[internal/features/board/tasks_service.go, `Purge`]** Физическая уборка в журнал не пишется; `Delete`/`Restore` пишут (`slog.InfoContext "soft delete"`). Добавить строку `action=purge`.
2. **[migrations/0018_soft_delete_core.down.sql; soft_delete_http_live_test.go `TestTS012_MigrationCheck`]** TS-012: CHECK и NULL по умолчанию покрыты, сценария откат→накат нет. В MVP (DS-022, §CM-1) не дефект. Down снимает отметки — удалённые заказы становятся действующими с товаром, возвращённым на склад; в комментарии down это названо.
3. **[internal/features/board/position.go:19-21, :44; tasks_repo.go `Restore`]** Удалённые заказы исключены из расчёта и перенумерации позиций, `Restore` позицию не пересчитывает (кроме DS-039). После восстановления возможны совпадающая позиция и иной порядок в колонке; лечится существующим `needs_rebalance`. Теста нет.
4. **[internal/features/transfers/repo.go:1157]** `checkRefs` принимает удалённый заказ — прямым API к нему создаётся новый документ. По §Impact #11 / DS-034 — зона части 2; вход части 2, не дефект части 1.

## Проверено по вопросам оркестратора (первый проход, актуально)
1. **Условие «заказ не удалён».** Все обращения к `tasks` в non-test коде просмотрены: список/поиск/get, карточка и печать, история оборудования (карточка и экран единицы), «Остатки», «Выплаты» (список, итоги, мутации), счётчики сотрудников и колонок, позиции, вложения, мутации услуг/товаров/оборудования/переноса (`LockTask` → 404 после ожидания блокировки), записи о работе и вложения (после фикса — под `KEY SHARE`). Намеренно без условия: `ColumnRepo.HasTasks`, `clients/repo.go:341, :523` (DS-035), `ExistingKeys` (DS-013), `transfers/repo.go:185, :1157` (DS-034).
2. **Остатки.** Delete: `LockTask` → `blockingTaskTransfers` → `Reconcile(false)` → `SoftDelete`; Restore: `Restore` → `Reconcile(target.ConsumesStock)`; нехватка → `warnings`. Правка колонки удалённые заказы не пересчитывает (`TaskIDsForUpdate`, тест `TestIT010` шаг 5а, TS-052). Повтор удаления → 404, восстановления → 409 `NOT_DELETED`.
3. **Служебная уборка.** Роут только при `cfg.TestPurgeEnabled` (строго `"true"`); без переменной 404; `RequireAPI` + `AdminOnly` + проверка роли в сервисе; вид ≠ `tasks` → 404. Обходов не найдено.
4. **Права.** `softdelete.AdminOnly()` на DELETE/restore/purge и `/api/v1/deleted`; SSR `/deleted` — `RequireAdmin`; пункт меню `AdminOnly`. 403 «Нет доступа» до поиска записи, 401 без сессии.
5. **Миграция 0018.** Nullable-колонки — существующие строки действующие; CHECK согласован с `ValidateReason`; частичный индекс соответствует `ORDER BY` списка.
6. **Время и причина.** Период — по поясу пользователя, `[с; по)` (TS-010); отображение — `KT.time.fmtDateTime`; причина → 400 `details.fields[]`.
7. **Тесты.** Ожидания существующих тестов изменены ровно в объёме DS-034 / DS-013 / DS-004; ослаблений сверх нет. E2E: TS-034…039, 046, 047 = SD-001…SD-008.

## Sec 18 — API-контракт
- URL kebab, JSON snake_case (чекер pass); список — `{items, total, limit, offset}`, `limit` 1–200.
- Ошибки — только `pkg/errorspkg`; `NOT_DELETED` 409, `TASK_NOT_FOUND` 404, purge — `NOT_FOUND` «Не найдено».
- `warnings` / `notices` / `items` — всегда массивы; `deleted_with: null`, `deleted_at` UTC.
- `counts` — все 11 видов, совпадают с `total` списков; N+1 нет.

## Соответствие кода спеке
- [x] Dataflow §6.1/§6.2 (одна транзакция под блокировкой заказа) — ДА
- [x] DDL §4.1 ↔ миграция 0018 — ДА
- [x] Ошибки §6 (коды, тексты, порядок «роль первой») — ДА
- [x] §Impact #1–#18: каждая позиция имеет правку в diff; #10, #11 «не трогать» — не тронуты — ДА
- [x] Переходные правила DS-034, DS-035, DS-036, DS-039 — реализованы и покрыты (TS-048, TS-049, TS-013, TS-052) — ДА
- [x] DS-026 / DS-013 под конкуренцией — ДА (после c3195c3)
- [x] Scope creep — НЕТ
- Monitoring / Rollout / Runbook / Perf — N/A; Swagger — n/a (`swagger:no`); Vue Sec 11 / 12.2 — n/a (SSR)

## Статус фаз
- [x] `0-setup/light-progress.md` — все пункты `[x]` — ДА

## Итог
APPROVED. Блокеров и серьёзных замечаний нет: обе гонки закрыты, новых взаимоблокировок и смены порядка ответов не найдено, тест гонки проверяет реальное ожидание блокировки. Осталось 4 незначительных.
