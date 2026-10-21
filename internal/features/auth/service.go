package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"

	"github.com/kitavrus/kanban-tree/internal/config"
	"github.com/kitavrus/kanban-tree/internal/server/middleware"
	"github.com/kitavrus/kanban-tree/internal/session"
	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
)

var (
	// ErrInvalidCredentials — единый ответ на «нет такого логина» и «неверный пароль».
	ErrInvalidCredentials = errorspkg.New("INVALID_CREDENTIALS", http.StatusUnauthorized, "Неверный логин или пароль")
	// ErrBlocked — пароль верный, но учётная запись заблокирована (раскрывается только владельцу пароля).
	ErrBlocked = errorspkg.New("USER_BLOCKED", http.StatusForbidden, "Учётная запись заблокирована — обратитесь к администратору")
	// ErrTooManyAttempts — превышен лимит неудачных входов по логину.
	ErrTooManyAttempts = errorspkg.New("TOO_MANY_ATTEMPTS", http.StatusTooManyRequests, "Слишком много неудачных попыток входа — попробуйте позже")
)

const (
	failPrefix  = "login_fail:"
	maxLoginLen = 50 // = users.loginMax
)

// dummyHash — сравнение для несуществующего логина: время ответа не выдаёт, есть ли логин.
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("kanban-tree-dummy-password"), bcrypt.DefaultCost)

// Service — вход/выход поверх Redis-сессий (plan.md §2, D35).
type Service struct {
	repo  *Repo
	store *session.Store
	rdb   *redis.Client
	cfg   config.AuthConfig
}

func NewService(pool *pgxpool.Pool, rdb *redis.Client, store *session.Store, cfg config.AuthConfig) *Service {
	return &Service{repo: NewRepo(pool), store: store, rdb: rdb, cfg: cfg}
}

// NormalizeLogin — логины хранятся и сравниваются в нижнем регистре без пробелов по краям.
func NormalizeLogin(login string) string { return strings.ToLower(strings.TrimSpace(login)) }

// Login проверяет пароль и создаёт сессию; возвращает sid и пользователя.
func (s *Service) Login(ctx context.Context, login, password string) (string, *User, error) {
	login = NormalizeLogin(login)
	// Логин длиннее допустимого не существует — без обращения к Redis (иначе
	// произвольно длинный ключ счётчика).
	if utf8.RuneCountInString(login) > maxLoginLen {
		return "", nil, ErrInvalidCredentials
	}
	// Счётчик попыток — АТОМАРНО до проверки пароля: параллельные запросы получают
	// разные номера и не могут все проскочить проверку лимита (D41). Успешный вход
	// счётчик сбрасывает.
	failKey := failPrefix + login
	pipe := s.rdb.TxPipeline()
	attempt := pipe.Incr(ctx, failKey)
	pipe.ExpireNX(ctx, failKey, s.cfg.LoginFailureWindow)
	if _, err := pipe.Exec(ctx); err != nil {
		return "", nil, err
	}
	if attempt.Val() > int64(s.cfg.LoginMaxFailures) {
		return "", nil, ErrTooManyAttempts
	}

	u, err := s.repo.GetByLogin(ctx, login)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return "", nil, err
	}
	hash := dummyHash
	if u != nil {
		hash = []byte(u.PasswordHash)
	}
	// bcrypt отклоняет пароль > 72 байт ошибкой — это тоже «неверный пароль», не 500.
	if bcrypt.CompareHashAndPassword(hash, []byte(password)) != nil || u == nil {
		return "", nil, ErrInvalidCredentials
	}
	if !u.IsActive {
		return "", nil, ErrBlocked
	}
	if err := s.rdb.Del(ctx, failKey).Err(); err != nil {
		return "", nil, err
	}
	sid, err := s.store.Create(ctx, u.ID)
	if err != nil {
		return "", nil, err
	}
	return sid, u, nil
}

// Logout — убить текущую сессию.
func (s *Service) Logout(ctx context.Context, sid string) error {
	return s.store.Kill(ctx, sid)
}

// Principal — загрузчик пользователя для middleware.Auth (nil, nil — нет такого).
func (s *Service) Principal(ctx context.Context, id string) (*middleware.Principal, error) {
	if uuid.Validate(id) != nil {
		return nil, nil
	}
	u, err := s.repo.GetByID(ctx, id)
	if errors.Is(err, ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &middleware.Principal{ID: u.ID, Login: u.Login, Name: u.Name, Role: u.Role, IsActive: u.IsActive, Timezone: u.Timezone, SupplierID: u.SupplierID}, nil
}
