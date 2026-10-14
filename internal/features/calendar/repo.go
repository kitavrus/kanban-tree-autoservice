package calendar

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Repo — чтение заказов окна и настройки вместимости.
type Repo struct {
	pool *pgxpool.Pool
}

func NewRepo(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

// orderRow — заказ с датами в поясе пользователя (YYYY-MM-DD).
type orderRow struct {
	ID          int64
	Title       string
	ClientName  string
	ColumnName  string
	ColumnColor string
	StartDate   string
	DueDate     *string
	FinishedOn  *string // день выдачи; nil — не выдан
}

// Orders — действующие заказы, принятые не позже to и не выданные раньше from.
func (r *Repo) Orders(ctx context.Context, tz, from, to string) ([]orderRow, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT t.id, t.title, cl.company_name, c.name, c.color,
		       (t.created_at AT TIME ZONE $1)::date::text,
		       t.due_date::text,
		       CASE WHEN c.is_final THEN (COALESCE(t.finished_at, t.updated_at) AT TIME ZONE $1)::date::text END
		  FROM tasks t
		  JOIN columns c  ON c.id = t.column_id
		  JOIN clients cl ON cl.id = t.client_id
		 WHERE t.deleted_at IS NULL
		   AND (t.created_at AT TIME ZONE $1)::date <= $3::date
		   AND (NOT c.is_final OR (COALESCE(t.finished_at, t.updated_at) AT TIME ZONE $1)::date >= $2::date)
		 ORDER BY t.created_at, t.id`, tz, from, to)
	if err != nil {
		return nil, fmt.Errorf("calendar orders: %w", err)
	}
	defer rows.Close()
	var out []orderRow
	for rows.Next() {
		var o orderRow
		if err := rows.Scan(&o.ID, &o.Title, &o.ClientName, &o.ColumnName, &o.ColumnColor,
			&o.StartDate, &o.DueDate, &o.FinishedOn); err != nil {
			return nil, fmt.Errorf("calendar orders scan: %w", err)
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// Settings — текущая вместимость.
func (r *Repo) Settings(ctx context.Context) (Settings, error) {
	var s Settings
	if err := r.pool.QueryRow(ctx, `SELECT max_cars FROM shop_settings WHERE id`).Scan(&s.MaxCars); err != nil {
		return Settings{}, fmt.Errorf("shop settings: %w", err)
	}
	return s, nil
}

// SaveSettings — upsert единственной строки настроек.
func (r *Repo) SaveSettings(ctx context.Context, s Settings) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO shop_settings (id, max_cars) VALUES (true, $1)
		ON CONFLICT (id) DO UPDATE SET max_cars = EXCLUDED.max_cars, updated_at = now()`, s.MaxCars)
	if err != nil {
		return fmt.Errorf("save shop settings: %w", err)
	}
	return nil
}
