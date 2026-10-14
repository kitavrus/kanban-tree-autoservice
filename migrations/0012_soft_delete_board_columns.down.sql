-- Откат части soft-delete-board-columns: отметки удаления колонок стираются — мягко удалённые
-- колонки снова становятся действующими и встают в конец доски (позиции действующих идут
-- подряд 1..N, удалённые хранили прежнюю позицию без смысла); между собой ожившие упорядочены
-- по (прежняя позиция, id). «Кто, когда, почему» теряются безвозвратно. Откат выполним на любых
-- данных: названия колонок и позиции не уникальны. Ожившая колонка возвращается со своими
-- признаками (is_final, consumes_stock) и своими заказами — действующих в ней не было, а
-- удалённые оживит только откат 0018.

UPDATE columns c SET position = o.rn
FROM (SELECT id, row_number() OVER (ORDER BY (deleted_at IS NOT NULL), position, id) AS rn FROM columns) o
WHERE c.id = o.id AND c.position <> o.rn;

DROP INDEX IF EXISTS columns_deleted_at_idx;

ALTER TABLE columns
    DROP CONSTRAINT IF EXISTS columns_deleted_chk,
    DROP COLUMN delete_reason,
    DROP COLUMN deleted_by,
    DROP COLUMN deleted_at;
