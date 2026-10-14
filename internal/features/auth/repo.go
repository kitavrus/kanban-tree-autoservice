package auth

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type User struct {
	ID           string
	Login        string
	PasswordHash string
	Name         string
	Role         string
	IsActive     bool
	Timezone     string // имя IANA (DS-031)
}

type Repo struct {
	pool *pgxpool.Pool
}

func NewRepo(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

var ErrNotFound = pgx.ErrNoRows

const userColumns = `id, login, password_hash, name, role, is_active, timezone`

func scanUser(row pgx.Row) (*User, error) {
	var u User
	if err := row.Scan(&u.ID, &u.Login, &u.PasswordHash, &u.Name, &u.Role, &u.IsActive, &u.Timezone); err != nil {
		return nil, err
	}
	return &u, nil
}

// GetByLogin — логин регистронезависимый (хранится в нижнем регистре, индекс lower(login)).
func (r *Repo) GetByLogin(ctx context.Context, login string) (*User, error) {
	return scanUser(r.pool.QueryRow(ctx,
		`SELECT `+userColumns+` FROM users WHERE lower(login) = lower($1)`, login))
}

// GetByID — id уже провалидирован как uuid вызывающим кодом (иначе ::uuid → ошибка SQL).
func (r *Repo) GetByID(ctx context.Context, id string) (*User, error) {
	u, err := scanUser(r.pool.QueryRow(ctx,
		`SELECT `+userColumns+` FROM users WHERE id = $1::uuid`, id))
	if errors.Is(err, ErrNotFound) {
		return nil, ErrNotFound
	}
	return u, err
}
