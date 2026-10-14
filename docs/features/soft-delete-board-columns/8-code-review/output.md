---
stage: 8-code-review
feature: soft-delete-board-columns
tier: S
verdict: APPROVED
high: 0
medium: 2
low: 9
---
# Code Review: soft-delete-board-columns — APPROVED
**Review level:** Slim (tier: S) + Sec 18 API-линза (режим spec-track merged, §ST-1)

Диапазон: `7ab0503..9db8395`. Source of truth: `draft-spec.md` §4/§6/§8, DS/TS. Гейты сборки/тестов — по фактам оркестратора, не перегонялись. Метод: чтение кода и diff; SQL/миграции не исполнялись.

## Блокеры (обязательно исправить до мержа)
Нет.

## Серьёзные замечания (исправить в этом PR)
1. **[internal/features/board/columns_soft_delete_http_live_test.go:277–299, TS-020]** «Списание по правилу НОВОЙ колонки» (DS-016, §6.3) тестом не доказано. Колонка заказа создаётся `sdColumn` с `consumes_stock=false` (`soft_delete_http_live_test.go:174`), колонка-замена «Новые» — тоже `false`; `wantStock` выбирается по фактическому `consumes` замены → проходит и при правиле старой колонки. Код верен (`tasks_service.go:560` `target = repl`, `:569` `Reconcile(…, target.ConsumesStock, …)`). Требуется: колонка заказа с `consumes_stock=true` (остаток 7 до удаления → 10 после восстановления в «Новые») либо замена с `consumes_stock=true`.
2. **[internal/features/board/columns_soft_delete_http_live_test.go, TS-019 / DS-019]** Новый инвариант «≥1 ДЕЙСТВУЮЩАЯ нефинальная колонка» не покрыт: нет случая «остальные нефинальные мягко удалены → 409 `LAST_NON_FINAL_COLUMN`» ни для `DELETE`, ни для `PUT is_final=true` (`columns_service.go:129–141`, `:255`). `TestBoardLiveSuite_DeleteColumn_LastNonFinal` переводит остальные в финальные — фильтр `LockAll` по `Deleted` (`columns_repo.go:76`) не упражняется; его поломка тестами не ловится. Туда же — гонка «две последние нефинальные одновременно» (см. п. 1 «Особые вопросы»): по коду закрыта, тестом не закреплена; образец — `TestRace_ColumnDeleteWaitsForTaskCreate` (держать `FOR UPDATE` + ждать по `pg_stat_activity`).

## Незначительные замечания (можно в следующем PR)
1. **[draft-spec.md §6.3 ↔ tasks_service.go:716–724]** Находка (а). `column_id` удалённой колонки → 404 `COLUMN_NOT_FOUND` — это буква спеки (§6.3 строка `POST/PUT /api/v1/tasks`, TS-019 негатив, пример §6.3). Никогда не существовавшая → 400 `column_id` — as-is («404 — только id в пути»). Код не трогать; противоречива формулировка спеки «как несуществующей» — поправить текст §6.3 / зафиксировать as-is DS. Побочно: по коду ответа различимы «удалена» и «не было» — для admin/executor-only API несущественно.
2. **[columns_service.go:219–225]** Находка (г). Порядок «роль → формат id → причина → поиск»: валидный несуществующий UUID без причины → 400, не 404. Совпадает с частью 1 (`tasks_service.go:438–448`) и с частью 2; спека порядок 400/404 не фиксирует. Приемлемо, выравнивать не нужно.
3. **[migrations/0020_soft_delete_board_columns.down.sql]** Находка (в). Статически корректна (перенумерация до DROP, индекс уходит с колонкой, `DROP CONSTRAINT IF EXISTS`). Режим MVP (§CM-1) — непрогон не дефект. Для единообразия с частью 2 (`transfers/migration_revert_integration_test.go`) — прогон up→down→up.
4. **[columns_service.go:329–349, columns_repo.go:262]** `Purge` не проверяет `LAST_NON_FINAL_COLUMN`: служебный адрес может физически убрать последнюю действующую нефинальную колонку; тогда `TaskService.Restore` вернёт недокументированный для него 409 `LAST_NON_FINAL_COLUMN` (`tasks_service.go:537`). Только при `TEST_PURGE_ENABLED=true`.
5. **[board_http_live_test.go:1092, :1149]** Уборка тест-колонок прежним `DELETE` теперь мягкая — строки копятся в `columns` тестовой БД. Перевести на purge/SQL, как в `TestTS009_ColumnsKind`.
6. **[seeddemo_integration_test.go:39–46]** Находка (б), тестовая часть: пересев дублирует литерал сида миграции `0005` — расхождение при правке сида. Вынести в константу/хелпер.
7. **[columns_handler.go:92–97, tasks_handler.go:123–128, transfers/handler.go:146–151]** Третья копия разбора тела DELETE. Вынести в `softdelete` (части 4–6 добавят ещё три).
8. **[e2e/tests/soft-delete-board-columns.spec.ts ↔ board.js:707–712, :916–919]** Ветки UI §8.1 без E2E: 404 при удалении колонки другим администратором (тост + обновление доски), перенос в удалённую колонку (тост + откат карточки), `LAST_NON_FINAL_COLUMN` в окне. На S допустимо.
9. **[e2e/fixtures/test.ts:147–150]** `_boardCleanup` молча глотает `COLUMN_NOT_EMPTY`: колонка с заказом, созданным в обход `adminApi`, остаётся (теперь возможно и в «Удалённых»). Поведение прежнее (`/40[49]/`), утечка невидима.

## Особые вопросы оркестратора
1. **Блокировки.** `LockAllAny` — `ORDER BY id FOR UPDATE` по всем строкам; `LockRestoreColumns` — `ORDER BY id FOR SHARE`; `seeddemo.loadColumns` — `ORDER BY id FOR UPDATE`: единый порядок id, взаимоблокировок между ними нет. Порядок columns → tasks → product_stock соблюдён в `Delete` (заказы не блокируются, `HasTasks` — чтение), `Restore` колонки, `TaskService.Restore` (`tasks_service.go:505–511`). `Renumber`/`closeGaps` правят строки, уже взятые той же транзакцией.
   - Гонка «две последние нефинальные одновременно»: **закрыта**. Вторая транзакция ждёт на `FOR UPDATE`; после фиксации первой перечитывает строку, `c.deleted_at IS NOT NULL` считается по новой версии → `LockAll` её отфильтрует → `nonFinalOthers == 0` → 409. Тот же механизм эмпирически подтверждён `TestRace_TaskWritesWaitForColumnDelete` (признак удаления после ожидания блокировки). Отдельного теста нет — MEDIUM-2.
   - Создание/перенос заказа ↔ удаление колонки: закрыто в обе стороны (`LockTargetColumn` `FOR SHARE` + признак после ожидания; `HasTasks` отдельным оператором после `LockAll`), оба направления покрыты тестами гонок.
   - Вне объёма (as-is): `FOR UPDATE` не блокирует вставку новой колонки параллельным `Create`; `tasks_count` из `LockAll` может быть устаревшим (поэтому `HasTasks` отдельно — верно).
2. **Инварианты восстановления.** Колонка: `position = max(действующих)+1` (`columns_repo.go:249`), `is_final`/`consumes_stock`/название/цвет не трогаются — подтверждено TS-021. Заказ удалённой колонки → `firstNonFinal` (исключает финальные и удалённые), позиция в конец, `column_replaced`, списание по `repl` — код верен, тест слабый (MEDIUM-1).
3. **Полнота условий.** Все чтения `columns` в не-тестовом Go проверены: `card_repo.go:46`, `columns_repo.go` (List/total, Renumber, closeGaps, Update), `tasks_repo.go:155/:265`, `seeddemo.go:230/:910/:930`, `softdelete/columns_source.go`. Соединения `tasks ⨝ columns` (`tasks_repo.go:447/:490/:509`, `stock_repo.go:28`, `clients/history.go:209`, `users/repo.go:47`) фильтруют `t.deleted_at IS NULL`; инвариант «действующий заказ — в действующей колонке» держится всеми путями записи `tasks.column_id` (Create/Update/Move через `LockTargetColumn`, Restore, seeddemo). `HasTasks`, `TaskIDsForUpdate`, `OpenSubtasksForColumn` — только действующие заказы. Удалённый заказ со ссылкой на удалённую колонку: FK RESTRICT цел, purge колонки → 409 `COLUMN_NOT_EMPTY`, восстановление заказа — подмена. Пропусков не найдено.
4. **Находки исполнителя.** (а) приемлемо, править текст спеки — LOW-1. (б) в продукте `DELETE FROM users`/`TRUNCATE` нет — FK RESTRICT риска не несёт; `TRUNCATE users … CASCADE` в Go-тестах только в `seeddemo_integration_test.go` (пересев добавлен); остальные сьюты не затронуты — LOW-6. (в) LOW-3. (г) приемлемо — LOW-2.
5. **Контракт 204 → 200.** Клиенты обновлены: `board.js:910` (`KT.api.delWith` + `{reason}`), `e2e/fixtures/auth.ts` (`deleteColumn` → `softDelete('columns')`, `purgeColumn`), `test.ts` (уборка purge), Go live-тесты. Прямых `DELETE /api/v1/columns` без тела не осталось. Права: `softdelete.AdminOnly()` на маршруте + повторная проверка в сервисе до поиска; TS-033 покрывает 403/401, включая несуществующий id и purge. Причина: 400 `details.fields[]` `required`/`too_long`/`invalid_text`.
6. **Ослабление тестов.** Нет. Все правки существующих тестов требуются спекой: 204→200 с телом, TS-049 для колонки заменён TS-019 (проверка `COLUMN_NOT_EMPTY` перенесена на purge), фильтр `deleted_at IS NULL` в выборках первой нефинальной, вид `columns` исключён из «пустых» в TS-009/TS-011 и покрыт `TestTS009_ColumnsKind`.

## Slim-чеклист
- **Security:** pass. Только admin (маршрут + сервис); параметры SQL — `$n`; причина в журнал не пишется (`columns_service.go:273`); purge — под `TEST_PURGE_ENABLED` + `AdminOnly`; новых зависимостей нет.
- **Architecture fit:** pass. Плоский пакет `board` — as-is; `softdelete` — reader, отметку пишет `board`. `github.com/google/uuid` — as-is (пакета `pkg/ids` в проекте нет).
- **Error handling:** pass. Только `pkg/errorspkg`; коды §6.1/§6.2 совпадают; 23503/23001 → `COLUMN_NOT_EMPTY`.
- **Tests:** pass с замечаниями MEDIUM-1, MEDIUM-2. `t.Skip`/`test.fixme`/`waitForTimeout` нет; HTTP — `LiveSuite`.
- **Rollback / breaking:** MVP (§CM-1) — breaking 204→200 и непрогон down не флагаются; down-миграция есть.
- **Sec 18 API:** pass. snake_case, kebab URL, конверт списка `/deleted`; error-body `errorspkg`; `deleted_at` — RFC 3339 UTC; nullable-полей в новых DTO нет; N+1 нет.

## Чекеры (из bundle)
- `go_naming`, `api_conventions`, `date_bound`, `fuzz_verdict`, `live_suite`, `fixture_sync`, `e2e_mode`, `rule_usage` — pass.
- `contract` — SKIP-RESOLVE: Vue/TS-клиента нет, неприменим.
- `e2e_drift` FAIL — ложный, причин две: чекер запущен по `scripts/e2e` («code: 0 в 0 файлах»), спеки лежат в `e2e/tests`; канон имён `SDC-001 (TS-042): …`. TS-042 покрыт `e2e/tests/soft-delete-board-columns.spec.ts:27`, `:102`. Оркестратору: поправить путь запуска чекера.

## Соответствие кода спеке
- [x] §4.1 миграция: три поля, CHECK `columns_deleted_chk`, FK `deleted_by`, индекс `columns_deleted_at_idx` — ДА
- [x] §6.1 / §6.2 — контракты, коды, сообщения — ДА
- [x] §6.3 дельта (список, создание, правка, заказы, `/deleted`, summary, purge) — ДА
- [x] §8.1 / §8.2 — окно «Удаление», вкладка «Колонки» последней, тосты, жёлтый блок — ДА
- [x] §Impact: позиции 1–10 имеют правку в diff; 5 «не трогать» — проверено, условие не требуется — ДА
- [x] TS-012, TS-019, TS-020, TS-021, TS-033, TS-042, TS-F3, TS-F8 — тесты есть (TS-020 — MEDIUM-1) — ДА
- [x] Scope creep / Non-Goals (части 4–6) — не реализованы — ДА
- Monitoring / Runbook / Performance Budget / PRD-ADR — N/A (S, PRD нет)

## Статус фаз
- [x] `0-setup/light-progress.md` — все пункты `- [x]` — ДА

## Итог
APPROVED: блокеров нет. В этом PR — MEDIUM-1 (TS-020 не различает правило колонок) и MEDIUM-2 (тест инварианта «действующая нефинальная» + гонка двух удалений).
