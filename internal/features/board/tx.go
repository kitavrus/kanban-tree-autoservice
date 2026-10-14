package board

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// txMaxRetries — ADR-104: 40P01 (deadlock_detected) / 40001 (serialization_failure) на
// операциях склада повторяются до 3 раз, затем ошибка уходит наружу без специальной
// обработки — errorspkg.FiberErrorHandler маппит неизвестную (не *AppError) ошибку в
// 500 INTERNAL.
const txMaxRetries = 3

// withTx — READ COMMITTED транзакция (Lock Order design-sql.md: columns→tasks→products)
// с ретраем на serialization-конфликты и на errTaskMovedRetry (ADR-100: колонка заказа
// сменилась между чтением column_id и LockTask внутри lockRowsScope).
func withTx(ctx context.Context, pool *pgxpool.Pool, fn func(tx pgx.Tx) error) error {
	var err error
	for attempt := 0; attempt < txMaxRetries; attempt++ {
		err = runTx(ctx, pool, fn)
		if !isSerializationConflict(err) && !errors.Is(err, errTaskMovedRetry) {
			return err
		}
	}
	return err
}

func runTx(ctx context.Context, pool *pgxpool.Pool, fn func(tx pgx.Tx) error) error {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func isSerializationConflict(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}
	return pgErr.Code == "40P01" || pgErr.Code == "40001"
}
