package users

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/kitavrus/kanban-tree/internal/session"
	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
	"github.com/kitavrus/kanban-tree/pkg/timezone"
)

var (
	ErrUserNotFound = errorspkg.New("USER_NOT_FOUND", http.StatusNotFound, "Сотрудник не найден")
	ErrLoginTaken   = errorspkg.New("LOGIN_TAKEN", http.StatusConflict, "Логин уже занят")
	// D29: себя не заблокировать и не остаться без администратора.
	ErrSelfBlock = errorspkg.New("SELF_BLOCK", http.StatusConflict, "Нельзя заблокировать самого себя")
	ErrLastAdmin = errorspkg.New("LAST_ADMIN", http.StatusConflict, "Нельзя снять роль или заблокировать последнего активного администратора")
)

const (
	defaultLimit = 50
	maxLimit     = 200
)

// sortColumns — whitelist сортировки: значение из запроса в SQL не попадает.
var sortColumns = map[string]string{
	"name":       "name",
	"login":      "login",
	"role":       "role",
	"is_active":  "is_active",
	"created_at": "created_at",
}

type Service struct {
	repo  *Repo
	store *session.Store
}

func NewService(pool *pgxpool.Pool, store *session.Store) *Service {
	return &Service{repo: NewRepo(pool), store: store}
}

// ListQuery — параметры списка из HTTP-слоя (сырые).
type ListQuery struct {
	Q        string
	Role     string
	IsActive *bool
	Sort     string
	Order    string
	Limit    int
	Offset   int
}

func (s *Service) List(ctx context.Context, q ListQuery) (*ListResponse, error) {
	limit, offset := q.Limit, q.Offset
	if limit <= 0 {
		limit = defaultLimit
	}
	limit = min(limit, maxLimit)
	offset = max(offset, 0)
	// Кривая сортировка/роль из URL — тихий откат к дефолту, экран не ломается.
	col, ok := sortColumns[q.Sort]
	if !ok {
		col = "name"
	}
	order := "ASC"
	if strings.EqualFold(q.Order, "desc") {
		order = "DESC"
	}
	role := q.Role
	if role != RoleAdmin && role != RoleExecutor {
		role = ""
	}
	items, total, err := s.repo.List(ctx, ListParams{
		Q: strings.TrimSpace(q.Q), Role: role, IsActive: q.IsActive,
		Sort: col, Order: order, Limit: limit, Offset: offset,
	})
	if err != nil {
		return nil, err
	}
	resp := &ListResponse{Items: make([]UserResponse, len(items)), Total: total, Limit: limit, Offset: offset}
	for i, u := range items {
		resp.Items[i] = toResponse(u)
	}
	return resp, nil
}

func (s *Service) Get(ctx context.Context, id string) (*UserResponse, error) {
	if uuid.Validate(id) != nil {
		return nil, ErrUserNotFound
	}
	u, err := s.repo.GetByID(ctx, id)
	if errors.Is(err, ErrNotFound) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	resp := toResponse(*u)
	return &resp, nil
}

func (s *Service) Create(ctx context.Context, in Input) (*UserResponse, error) {
	in = in.normalize()
	if f := in.validate(true); len(f) > 0 {
		return nil, errorspkg.Validation(f...)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	tz := timezone.Default
	if in.Timezone != nil {
		tz = *in.Timezone
	}
	u, err := s.repo.Create(ctx, in, string(hash), tz)
	if errors.Is(err, errLoginTaken) {
		return nil, ErrLoginTaken
	}
	if err != nil {
		return nil, err
	}
	resp := toResponse(*u)
	return &resp, nil
}

// Actor — кто правит: id и текущая сессия (смена своего пароля её не убивает).
type Actor struct {
	UserID    string
	SessionID string
}

// Update — полная замена карточки. Блокировка убивает все сессии сотрудника,
// смена пароля — все, кроме текущей сессии самого сотрудника.
func (s *Service) Update(ctx context.Context, actor Actor, id string, in Input) (*UserResponse, error) {
	if uuid.Validate(id) != nil {
		return nil, ErrUserNotFound
	}
	in = in.normalize()
	if f := in.validate(false); len(f) > 0 {
		return nil, errorspkg.Validation(f...)
	}
	var hash *string
	if in.Password != "" {
		h, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, err
		}
		hs := string(h)
		hash = &hs
	}
	before, after, err := s.repo.Update(ctx, id, in, hash, func(before *User, otherAdmins int) error {
		if id == actor.UserID && !in.IsActive {
			return ErrSelfBlock
		}
		wasActiveAdmin := before.Role == RoleAdmin && before.IsActive
		staysActiveAdmin := in.Role == RoleAdmin && in.IsActive
		if wasActiveAdmin && !staysActiveAdmin && otherAdmins == 0 {
			return ErrLastAdmin
		}
		return nil
	})
	switch {
	case errors.Is(err, ErrNotFound):
		return nil, ErrUserNotFound
	case errors.Is(err, errLoginTaken):
		return nil, ErrLoginTaken
	case err != nil:
		return nil, err
	}

	switch {
	case before.IsActive && !after.IsActive:
		_, err = s.store.KillUser(ctx, id, "")
	case hash != nil:
		except := ""
		if id == actor.UserID {
			except = actor.SessionID
		}
		_, err = s.store.KillUser(ctx, id, except)
	}
	if err != nil {
		return nil, err
	}
	if before.Timezone != after.Timezone {
		slog.InfoContext(ctx, "users: timezone changed", "actor_id", actor.UserID, "user_id", id,
			"old", before.Timezone, "new", after.Timezone)
	}
	resp := toResponse(*after)
	return &resp, nil
}

// SetTimezone — пользователь меняет СВОЙ пояс (DS-033): userID — только из сессии.
// Last-write-wins; сессии не трогаются — новое значение читается из БД со следующего запроса.
func (s *Service) SetTimezone(ctx context.Context, userID, tz string) (*TimezoneResponse, error) {
	if f := validateTimezone(tz); len(f) > 0 {
		return nil, errorspkg.Validation(f...)
	}
	if uuid.Validate(userID) != nil {
		return nil, ErrUserNotFound
	}
	old, err := s.repo.SetTimezone(ctx, userID, tz)
	if errors.Is(err, ErrNotFound) {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	if old != tz {
		slog.InfoContext(ctx, "users: timezone changed", "actor_id", userID, "user_id", userID, "old", old, "new", tz)
	}
	return &TimezoneResponse{Timezone: tz}, nil
}

func toResponse(u User) UserResponse {
	return UserResponse{ID: u.ID, Login: u.Login, Name: u.Name, Role: u.Role, Phone: u.Phone,
		IsActive: u.IsActive, Timezone: u.Timezone, CreatedAt: u.CreatedAt, UpdatedAt: u.UpdatedAt, ActiveTasks: u.ActiveTasks}
}
