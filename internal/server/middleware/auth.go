package middleware

import (
	"context"
	"errors"
	"net/url"
	"strings"
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
	// RoleSupplier — учётка поставщика (purchase-orders DS-007): видит только свой кабинет.
	RoleSupplier = ssr.RoleSupplier

	// SupplierHome — куда SSR-guard отправляет поставщика с любого чужого раздела (DS-018).
	SupplierHome = "/supplier/orders"
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
	// SupplierID — поставщик учётки роли supplier (purchase-orders DS-053): кабинет берёт
	// поставщика только отсюда, никогда из запроса. "" у admin/executor.
	SupplierID string
}

func (p *Principal) IsAdmin() bool { return p.Role == RoleAdmin }

// IsSupplier — учётка поставщика (purchase-orders DS-007).
func (p *Principal) IsSupplier() bool { return p.Role == RoleSupplier }

// SupplierSSRAllowed — страница, открытая роли supplier (DS-053): только кабинет /supplier/*.
func SupplierSSRAllowed(path string) bool {
	return path == "/supplier" || strings.HasPrefix(path, "/supplier/")
}

// supplierAPIAllowed — JSON API, открытый роли supplier («по умолчанию запрещено», DS-053):
// кабинет /api/v1/supplier/*, выход, свой профиль и смена своего часового пояса.
func supplierAPIAllowed(method, path string) bool {
	switch {
	case strings.HasPrefix(path, "/api/v1/supplier/"):
		return true
	case method == fiber.MethodPost && path == "/api/v1/auth/logout":
		return true
	case method == fiber.MethodGet && path == "/api/v1/auth/me":
		return true
	case method == fiber.MethodPut && path == "/api/v1/users/me/timezone":
		return true
	}
	return false
}

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
		// Поставщик: всё вне allow-list — 403 (DS-017, DS-053). Правило в общем guard'е,
		// поэтому любой новый раздел закрыт для поставщика автоматически.
		if p.IsSupplier() && !supplierAPIAllowed(c.Method(), c.Path()) {
			return errorspkg.Forbidden("Нет доступа")
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
		// Поставщик заперт в кабинете (DS-017, DS-018): любая иная страница → /supplier/orders.
		if p.IsSupplier() && !SupplierSSRAllowed(c.Path()) {
			return c.Redirect().Status(fiber.StatusFound).To(SupplierHome)
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

// RequireSupplier — только роль supplier с привязкой к поставщику (после RequireAPI/
// RequireSSR; DS-053). Иначе — forbidden (страница 403) либо 403 JSON «Раздел только для
// поставщиков».
func RequireSupplier(forbidden fiber.Handler) fiber.Handler {
	return func(c fiber.Ctx) error {
		if p := CurrentPrincipal(c); p != nil && p.IsSupplier() && p.SupplierID != "" {
			return c.Next()
		}
		if forbidden != nil {
			return forbidden(c)
		}
		return errorspkg.Forbidden("Раздел только для поставщиков")
	}
}

// RequireStaff — admin или executor (закупки §6.1–§6.11): supplier уже отсечён общим
// guard'ом, правило здесь — защитный дубль для роутов вне /api/v1.
func RequireStaff(forbidden fiber.Handler) fiber.Handler {
	return func(c fiber.Ctx) error {
		if p := CurrentPrincipal(c); p != nil && (p.Role == RoleAdmin || p.Role == RoleExecutor) {
			return c.Next()
		}
		if forbidden != nil {
			return forbidden(c)
		}
		return errorspkg.Forbidden("Нет доступа")
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
