package middleware

import (
	"context"
	"errors"
	"net/url"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/kitavrus/kanban-tree/internal/session"
	"github.com/kitavrus/kanban-tree/internal/ssr"
	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
)

const (
	// SessionCookieName — HttpOnly cookie с id сессии.
	SessionCookieName = "sid"
	// UserIDKey — Locals-ключ id авторизованного пользователя (string uuid).
	UserIDKey = "user_id"
	// PrincipalKey — Locals-ключ *Principal текущего пользователя.
	PrincipalKey = "principal"
	// SessionIDKey — Locals-ключ sid текущей сессии (смена своего пароля не убивает её).
	SessionIDKey = "session_id"

	RoleAdmin    = ssr.RoleAdmin
	RoleExecutor = "executor"
)

// Principal — кто делает запрос: читается из БД на каждом запросе, поэтому смена
// роли, блокировка и смена часового пояса действуют сразу, без перелогина.
type Principal struct {
	ID       string
	Login    string
	Name     string
	Role     string
	IsActive bool
	// Timezone — часовой пояс пользователя (IANA, DS-031): по нему показывается время.
	Timezone string
}

func (p *Principal) IsAdmin() bool { return p.Role == RoleAdmin }

// PrincipalLoader — пользователь по id; (nil, nil) — пользователя нет.
type PrincipalLoader func(ctx context.Context, id string) (*Principal, error)

// Auth — гарды авторизации поверх сессий.
type Auth struct {
	Store  *session.Store
	Load   PrincipalLoader
	Secure bool          // Secure-флаг cookie sid
	MaxAge time.Duration // срок cookie = абсолютный потолок сессии
}

// resolve — пользователь текущего запроса или nil. Заблокированный/удалённый
// пользователь с живой сессией — сессия убивается, запрос анонимный.
func (a *Auth) resolve(c fiber.Ctx) (*Principal, error) {
	sid := c.Cookies(SessionCookieName)
	if sid == "" {
		return nil, nil
	}
	d, err := a.Store.Get(c.Context(), sid)
	if errors.Is(err, session.ErrNoSession) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	p, err := a.Load(c.Context(), d.UserID)
	if err != nil {
		return nil, err
	}
	if p == nil || !p.IsActive {
		_ = a.Store.Kill(c.Context(), sid)
		return nil, nil
	}
	c.Locals(UserIDKey, p.ID)
	c.Locals(PrincipalKey, p)
	c.Locals(SessionIDKey, sid)
	c.Locals(ssr.LocalViewer, ssr.Viewer{Name: p.Name, Role: p.Role, Timezone: p.Timezone})
	return p, nil
}

// Optional — определить пользователя, если он вошёл; не требует входа (страница /login).
func (a *Auth) Optional() fiber.Handler {
	return func(c fiber.Ctx) error {
		if _, err := a.resolve(c); err != nil {
			return err
		}
		return c.Next()
	}
}

// RequireAPI — JSON API: без сессии → 401 UNAUTHORIZED.
func (a *Auth) RequireAPI() fiber.Handler {
	return func(c fiber.Ctx) error {
		p, err := a.resolve(c)
		if err != nil {
			return err
		}
		if p == nil {
			return errorspkg.Unauthorized("no session")
		}
		return c.Next()
	}
}

// RequireSSR — страницы: без сессии → 302 на /login?redirect=<исходный URL>.
//
// Cache-Control: no-store — КАЖДЫЙ ответ этого guard'а (и сама страница, и редирект)
// запрещено кэшировать браузеру. Без этого «Назад» после logout (AU-005) не всегда
// доходит до сервера: на быстром переходе браузер может отдать ПРЕДЫДУЩИЙ успешный
// рендер доски прямо из HTTP-кэша/bfcache — сервер вообще не видит этот запрос и не
// успевает переотправить на /login, а JS уже показанной (стухшей) страницы как ни в
// чём не бывало продолжает свои фоновые GET на /api/v1/... с уже недействительной
// cookie-сессией → серия 401 (BD-021, найдено сводным E2E-гейтом на AU-005: «консоль
// и сеть чистые» падало именно на этом). no-store одновременно исключает и обычное
// HTTP-кэширование, и bfcache (Chrome явно не кладёт в bfcache no-store-страницы) —
// «Назад»/восстановление вкладки всегда идёт новым запросом через этот guard.
func (a *Auth) RequireSSR() fiber.Handler {
	return func(c fiber.Ctx) error {
		c.Set(fiber.HeaderCacheControl, "no-store, must-revalidate")
		p, err := a.resolve(c)
		if err != nil {
			return err
		}
		if p == nil {
			return c.Redirect().Status(fiber.StatusFound).To("/login?redirect=" + url.QueryEscape(c.OriginalURL()))
		}
		return c.Next()
	}
}

// RequireAdmin — только admin (после RequireAPI/RequireSSR). Иначе — forbidden
// (для API — 403 JSON; для страниц вызывающий передаёт рендер 403-страницы).
func RequireAdmin(forbidden fiber.Handler) fiber.Handler {
	return func(c fiber.Ctx) error {
		if p := CurrentPrincipal(c); p != nil && p.IsAdmin() {
			return c.Next()
		}
		if forbidden != nil {
			return forbidden(c)
		}
		return errorspkg.Forbidden("admin only")
	}
}

// CurrentPrincipal — пользователь запроса (nil — анонимный).
func CurrentPrincipal(c fiber.Ctx) *Principal {
	p, _ := c.Locals(PrincipalKey).(*Principal)
	return p
}

// SetSessionCookie / ClearSessionCookie — cookie sid: HttpOnly, SameSite=Strict (plan.md §2).
func (a *Auth) SetSessionCookie(c fiber.Ctx, sid string) {
	c.Cookie(&fiber.Cookie{
		Name:     SessionCookieName,
		Value:    sid,
		Path:     "/",
		HTTPOnly: true,
		Secure:   a.Secure,
		SameSite: fiber.CookieSameSiteStrictMode,
		MaxAge:   int(a.MaxAge.Seconds()),
	})
}

func (a *Auth) ClearSessionCookie(c fiber.Ctx) {
	c.Cookie(&fiber.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		HTTPOnly: true,
		Secure:   a.Secure,
		SameSite: fiber.CookieSameSiteStrictMode,
		MaxAge:   -1,
	})
}
