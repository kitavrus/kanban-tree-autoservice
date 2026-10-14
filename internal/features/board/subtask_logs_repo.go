package board

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SubtaskLogRepo — журнал работ услуги (design-sql.md §Subtask Log Queries). Свой pool:
// заказ под FOR UPDATE не берётся (журнал склад не трогает) — FK-проверка на INSERT берёт
// KEY SHARE на subtasks, цикла с DELETE услуги (tasks→subtasks) нет.
type SubtaskLogRepo struct {
	pool *pgxpool.Pool
}

func NewSubtaskLogRepo(pool *pgxpool.Pool) *SubtaskLogRepo { return &SubtaskLogRepo{pool: pool} }

// Запись журнала и строка заказа (soft-delete-core DS-026). Журнал склад не трогает и заказ
// под FOR UPDATE не берёт, но каждая мутация идёт в транзакции, первым шагом которой заказ
// услуги блокируется FOR KEY SHARE с условием «не удалён». Мягкое удаление заказа берёт
// LockTask (FOR UPDATE) — он с KEY SHARE несовместим: либо удаление ждёт конца записи, либо
// запись ждёт удаление и после него строки заказа уже не видит (0 строк → errNotFound).
// Блокировка одна (tasks), порядок общий — tasks → subtasks → subtask_logs: взаимных
// блокировок с Delete / Restore / LockAll нет.

// inTx — транзакция пула с откатом при ошибке.
func (r *SubtaskLogRepo) inTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Insert — запись журнала услуги subtaskID; возвращает заказ услуги (для Re-fetch-after-write
// ответа). Услуги нет, её заказ мягко удалён либо услуга удалена между запросами (23503 на
// subtask_id, LV-174) → errNotFound (ErrSubtaskNotFound).
func (r *SubtaskLogRepo) Insert(ctx context.Context, subtaskID, userID, text string) (int64, error) {
	var taskID int64
	err := r.inTx(ctx, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			SELECT s.task_id FROM subtasks s JOIN tasks t ON t.id = s.task_id
			WHERE s.id = $1 AND t.deleted_at IS NULL FOR KEY SHARE OF t`, subtaskID).Scan(&taskID)
		if errors.Is(err, pgx.ErrNoRows) {
			return errNotFound
		}
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO subtask_logs (subtask_id, user_id, text) VALUES ($1, $2, $3)`, subtaskID, userID, text)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return errNotFound
		}
		return err
	})
	return taskID, err
}

// LogOwner — автор записи + task_id для проверки прав (D27) и Re-fetch-after-write.
type LogOwner struct {
	UserID string
	TaskID int64
}

// mutate — правка / удаление записи id под блокировкой её заказа: владелец читается с
// FOR KEY SHARE заказа (0 строк, в т.ч. заказ мягко удалён → errNotFound), allow проверяет
// права (D27), затем выполняется sql ($1 — id). 0 затронутых строк → errNotFound.
func (r *SubtaskLogRepo) mutate(ctx context.Context, id string, allow func(LogOwner) error, sql string, args ...any) (*LogOwner, error) {
	var o LogOwner
	err := r.inTx(ctx, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			SELECT l.user_id, s.task_id FROM subtask_logs l JOIN subtasks s ON s.id = l.subtask_id
			JOIN tasks t ON t.id = s.task_id WHERE l.id = $1 AND t.deleted_at IS NULL FOR KEY SHARE OF t`, id).
			Scan(&o.UserID, &o.TaskID)
		if errors.Is(err, pgx.ErrNoRows) {
			return errNotFound
		}
		if err != nil {
			return err
		}
		if err := allow(o); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, sql, append([]any{id}, args...)...)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return errNotFound
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &o, nil
}

// Update — design-sql.md §Subtask Log Queries: текст записи; права проверяет allow.
func (r *SubtaskLogRepo) Update(ctx context.Context, id, text string, allow func(LogOwner) error) (*LogOwner, error) {
	return r.mutate(ctx, id, allow, `UPDATE subtask_logs SET text = $2, updated_at = now() WHERE id = $1`, text)
}

// Delete — удаление записи; права проверяет allow.
func (r *SubtaskLogRepo) Delete(ctx context.Context, id string, allow func(LogOwner) error) (*LogOwner, error) {
	return r.mutate(ctx, id, allow, `DELETE FROM subtask_logs WHERE id = $1`)
}
