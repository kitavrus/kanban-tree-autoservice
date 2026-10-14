package users

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kitavrus/kanban-tree/pkg/sqlutil"
)

type User struct {
	ID        string
	Login     string
	Name      string
	Role      string
	Phone     string
	IsActive  bool
	Timezone  string // имя IANA (DS-031)
	CreatedAt time.Time
	UpdatedAt time.Time
	// ActiveTasks — заполняется только в List (nil в остальных запросах).
	ActiveTasks *int
}

type Repo struct {
	pool *pgxpool.Pool
}

func NewRepo(pool *pgxpool.Pool) *Repo { return &Repo{pool: pool} }

var (
	ErrNotFound   = pgx.ErrNoRows
	errLoginTaken = errors.New("users: login taken")
)

const columns = `id, login, name, role, phone, is_active, timezone, created_at, updated_at`

// activeTasksSQL — «Активных заказов» в списке сотрудников (как в прототипе): заказы вне
// финальной колонки, где сотрудник — ответственный заказа или исполнитель незавершённой
// услуги. Коррелирован с users.id внешнего запроса.
const activeTasksSQL = `(SELECT count(*) FROM tasks t JOIN columns c ON c.id = t.column_id
  WHERE NOT c.is_final AND t.deleted_at IS NULL AND (t.assignee_id = users.id
    OR EXISTS (SELECT 1 FROM subtasks s WHERE s.task_id = t.id AND s.assignee_id = users.id AND s.status <> 'done')))`

func scan(row pgx.Row) (*User, error) {
	var u User
	if err := row.Scan(&u.ID, &u.Login, &u.Name, &u.Role, &u.Phone, &u.IsActive, &u.Timezone, &u.CreatedAt, &u.UpdatedAt); err != nil {
		return nil, err
	}
	return &u, nil
}

// ListParams — фильтры + сортировка + пагинация. Sort/Order приходят из сервиса УЖЕ
// прошедшими whitelist: склеиваются в SQL как идентификаторы.
type ListParams struct {
	Q        string // подстрока в ФИО / логине / телефоне, регистронезависимо
	Role     string // "" = любая
	IsActive *bool  // nil = любой статус
	Sort     string
	Order    string // ASC | DESC
	Limit    int
	Offset   int
}

func (r *Repo) List(ctx context.Context, p ListParams) ([]User, int, error) {
	var conds []string
	var args []any
	add := func(cond string, val any) {
		args = append(args, val)
		conds = append(conds, fmt.Sprintf(cond, len(args)))
	}
	if p.Q != "" {
		add("(name ILIKE $%[1]d OR login ILIKE $%[1]d OR phone ILIKE $%[1]d)", "%"+sqlutil.EscapeLike(p.Q)+"%")
	}
	if p.Role != "" {
		add("role = $%d", p.Role)
	}
	if p.IsActive != nil {
		add("is_active = $%d", *p.IsActive)
	}
	where := ""
	if len(conds) > 0 {
		where = " WHERE " + strings.Join(conds, " AND ")
	}

	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM users`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	// id вторым ключом — стабильный порядок строк с равным значением сортировки.
	q := `SELECT ` + columns + `, ` + activeTasksSQL + ` FROM users` + where +
		fmt.Sprintf(" ORDER BY %s %s, id LIMIT $%d OFFSET $%d", p.Sort, p.Order, len(args)+1, len(args)+2)
	rows, err := r.pool.Query(ctx, q, append(args, p.Limit, p.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []User{}
	for rows.Next() {
		var u User
		var active int
		if err := rows.Scan(&u.ID, &u.Login, &u.Name, &u.Role, &u.Phone, &u.IsActive, &u.Timezone, &u.CreatedAt, &u.UpdatedAt,
			&active); err != nil {
			return nil, 0, err
		}
		u.ActiveTasks = &active
		out = append(out, u)
	}
	return out, total, rows.Err()
}

// GetByID — id уже провалидирован как uuid сервисом.
func (r *Repo) GetByID(ctx context.Context, id string) (*User, error) {
	return scan(r.pool.QueryRow(ctx, `SELECT `+columns+` FROM users WHERE id = $1::uuid`, id))
}

// Create — timezone уже разрешён сервисом (не передан → пояс по умолчанию).
func (r *Repo) Create(ctx context.Context, in Input, passwordHash, timezone string) (*User, error) {
	u, err := scan(r.pool.QueryRow(ctx,
		`INSERT INTO users (login, password_hash, name, role, phone, is_active, timezone)
		 VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING `+columns,
		in.Login, passwordHash, in.Name, in.Role, in.Phone, in.IsActive, timezone))
	return u, mapUnique(err)
}

// UpdateCheck — проверка перед записью внутри транзакции: before — карточка до правки,
// otherActiveAdmins — сколько активных админов, кроме правимого.
type UpdateCheck func(before *User, otherActiveAdmins int) error

// Update — полная замена карточки в транзакции. Строки всех активных админов и
// правимого пользователя блокируются (FOR UPDATE, в порядке id — без дедлоков), поэтому
// два параллельных «разжалования» админов не оставят систему без администратора (D29).
// passwordHash nil — пароль не меняется; in.Timezone nil — пояс не трогается (DS-056).
// Возвращает карточку до и после.
func (r *Repo) Update(ctx context.Context, id string, in Input, passwordHash *string, check UpdateCheck) (before, after *User, err error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err = tx.Exec(ctx,
		`SELECT id FROM users WHERE (role = 'admin' AND is_active) OR id = $1::uuid ORDER BY id FOR UPDATE`, id); err != nil {
		return nil, nil, err
	}
	if before, err = scan(tx.QueryRow(ctx, `SELECT `+columns+` FROM users WHERE id = $1::uuid`, id)); err != nil {
		return nil, nil, err
	}
	var others int
	if err = tx.QueryRow(ctx,
		`SELECT count(*) FROM users WHERE role = 'admin' AND is_active AND id <> $1::uuid`, id).Scan(&others); err != nil {
		return nil, nil, err
	}
	if err = check(before, others); err != nil {
		return nil, nil, err
	}
	after, err = scan(tx.QueryRow(ctx,
		`UPDATE users SET
		   login = $2, name = $3, role = $4, phone = $5, is_active = $6,
		   password_hash = COALESCE($7, password_hash),
		   timezone = COALESCE($8, timezone),
		   version = version + 1, updated_at = now()
		 WHERE id = $1::uuid
		 RETURNING `+columns,
		id, in.Login, in.Name, in.Role, in.Phone, in.IsActive, passwordHash, in.Timezone))
	if err != nil {
		return nil, nil, mapUnique(err)
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, nil, err
	}
	return before, after, nil
}

// SetTimezone — пользователь меняет свой пояс (DS-033): last-write-wins, version не
// проверяется и не увеличивается (открытая у администратора карточка этого пользователя не
// получает ложный конфликт версий), а updated_at обновляется — запись изменилась (D82).
// Возвращает прежнее значение (для журнала).
func (r *Repo) SetTimezone(ctx context.Context, id, timezone string) (old string, err error) {
	err = r.pool.QueryRow(ctx,
		`UPDATE users u SET timezone = $2, updated_at = now()
		   FROM (SELECT id, timezone FROM users WHERE id = $1::uuid FOR UPDATE) prev
		  WHERE u.id = prev.id
		RETURNING prev.timezone`, id, timezone).Scan(&old)
	return old, err
}

func mapUnique(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return errLoginTaken
	}
	return err
}
