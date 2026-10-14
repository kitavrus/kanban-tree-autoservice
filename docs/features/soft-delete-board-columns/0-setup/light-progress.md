# light-progress: soft-delete-board-columns (часть 3)

- [x] миграция 0020 (columns: deleted_at/by/reason, CHECK, индекс)
- [x] columns_repo/service/handler/dto/routes: мягкое удаление 200 {id, deleted_at}, restore, purge
- [x] tasks_repo/service: LockTargetColumn (deleted → 404 COLUMN_NOT_FOUND), Restore — подмена удалённой колонки
- [x] softdelete: ColumnsSource + регистрация + purger в router.go
- [x] seeddemo: удалённая колонка + тест
- [x] фронт: board.js (uiDeleteDialog), deleted.js (вкладка «Колонки», тосты)
- [x] Go-тесты: unit (subtitle/registry), live TS-012/019/020/021/033, правка 204→200, TS-049
- [x] E2E: auth.ts (deleteColumn, purgeColumn, softDelete/restore columns), test.ts cleanup, DeletedPage, BoardPage, spec TS-042, правка board.spec
- [x] self-check: go build, go test ./..., integration board+softdelete+seeddemo, tsc, playwright --list

Решения по ходу:
- column_id удалённой колонки при создании/переносе заказа → 404 COLUMN_NOT_FOUND (спека §6.3); никогда не существовавшей — as-is 400 field column_id (не переписываем молча).
