package board

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// positionQuerySQL — ADR-001 (design-sql.md §Position Query). Соседи читаются только из
// целевой колонки и не равные самому заказу; мягко удалённые заказы в расчёте позиций не
// участвуют (soft-delete-core DS-010 — их позиция не меняется, пока заказ удалён); «чужой»/несуществующий сосед игнорируется.
// needs_rebalance (code review MEDIUM-1): PG ограничивает scale результата деления
// NUMERIC значением max(16, scale входов) — порог `scale(pos) > 20` недостижим, середина
// округляется до scale=16 и после ~64 последовательных вставок в один зазор совпадает с
// соседом. Триггерим ребалансировку раньше — как только середина перестаёт быть строго
// МЕЖДУ соседями (pos <= prev_pos OR pos >= next_pos), либо когда scale(pos) уже достиг
// потолка (>= 16) и следующее деление пополам гарантированно даст совпадение.
const positionQuerySQL = `
WITH prev AS (SELECT position FROM tasks WHERE id = $3 AND column_id = $1 AND id <> $2 AND deleted_at IS NULL),
     next AS (SELECT position FROM tasks WHERE id = $4 AND column_id = $1 AND id <> $2 AND deleted_at IS NULL),
     last AS (SELECT max(position) AS p FROM tasks WHERE column_id = $1 AND id <> $2 AND deleted_at IS NULL),
     calc AS (
         SELECT CASE
             WHEN (SELECT position FROM prev) IS NOT NULL AND (SELECT position FROM next) IS NOT NULL
                  THEN trim_scale(((SELECT position FROM prev) + (SELECT position FROM next)) / 2)
             WHEN (SELECT position FROM prev) IS NOT NULL THEN (SELECT position FROM prev) + 1024
             WHEN (SELECT position FROM next) IS NOT NULL THEN (SELECT position FROM next) - 1024
             ELSE COALESCE((SELECT p FROM last), 0) + 1024
         END AS pos,
         (SELECT position FROM prev) AS prev_pos,
         (SELECT position FROM next) AS next_pos
     )
SELECT pos::text,
       (prev_pos IS NOT NULL AND next_pos IS NOT NULL AND (prev_pos >= next_pos OR pos <= prev_pos OR pos >= next_pos))
       OR scale(pos) >= 16 AS needs_rebalance
FROM calc;`

// renumberColumnSQL — ребалансировка колонки (одна итерация): перенумеровать 1024*rn всех
// заказов колонки, КРОМЕ переносимого $2 (design-sql.md §Position Query, второй блок).
// version НЕ инкрементится (порядок ≠ правка заказа).
const renumberColumnSQL = `
UPDATE tasks t SET position = r.rn * 1024
FROM (SELECT id, row_number() OVER (ORDER BY position, id) AS rn
      FROM tasks WHERE column_id = $1 AND id <> $2 AND deleted_at IS NULL) r
WHERE t.id = r.id;`

// nextPosition — позиция карточки $2 между prev/next в колонке $1 (ADR-001); при
// неустойчивом результате колонка перенумеровывается и позиция считается заново один раз.
// Используется TaskRepo.NextPosition (design-go-layers.md §Repository Interface, фаза 03).
func nextPosition(ctx context.Context, tx pgx.Tx, columnID string, taskID int64, prevID, nextID *int64) (string, error) {
	pos, rebalance, err := queryPosition(ctx, tx, columnID, taskID, prevID, nextID)
	if err != nil {
		return "", err
	}
	if !rebalance {
		return pos, nil
	}
	if _, err := tx.Exec(ctx, renumberColumnSQL, columnID, taskID); err != nil {
		return "", err
	}
	pos, _, err = queryPosition(ctx, tx, columnID, taskID, prevID, nextID)
	return pos, err
}

func queryPosition(ctx context.Context, tx pgx.Tx, columnID string, taskID int64, prevID, nextID *int64) (pos string, rebalance bool, err error) {
	err = tx.QueryRow(ctx, positionQuerySQL, columnID, taskID, prevID, nextID).Scan(&pos, &rebalance)
	return pos, rebalance, err
}
