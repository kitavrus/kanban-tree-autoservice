package auth

import (
	"errors"
	"net/url"
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/kitavrus/kanban-tree/internal/server/middleware"
	"github.com/kitavrus/kanban-tree/internal/ssr"
	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
)

// Root — шаблоны страницы входа относительно корня модуля.
const Root = "web/ssr/auth/templates"

type Handler struct {
	svc   *Service
	auth  *middleware.Auth
	r     *ssr.Renderer
	shell ssr.ShellBuilder
}

func NewHandler(svc *Service, auth *middleware.Auth, r *ssr.Renderer, shell ssr.ShellBuilder) *Handler {
	return &Handler{svc: svc, auth: auth, r: r, shell: shell}
}

// ---- JSON API ----

// Login — POST /api/v1/auth/login. Успех: HttpOnly cookie sid + профиль.
func (h *Handler) Login(c fiber.Ctx) error {
	var req LoginRequest
	if err := c.Bind().Body(&req); err != nil {
		return errorspkg.BadRequest("invalid body")
	}
	var fields []errorspkg.Field
	if strings.TrimSpace(req.Login) == "" {
		fields = append(fields, errorspkg.Field{Name: "login", Message: "required"})
	}
	if req.Password == "" {
		fields = append(fields, errorspkg.Field{Name: "password", Message: "required"})
	}
	if len(fields) > 0 {
		return errorspkg.Validation(fields...)
	}
	sid, u, err := h.svc.Login(c.Context(), req.Login, req.Password)
	if err != nil {
		return err
	}
	if err := h.replaceSession(c, sid); err != nil {
		return err
	}
	return c.JSON(UserResponse{ID: u.ID, Login: u.Login, Name: u.Name, Role: u.Role, Timezone: u.Timezone})
}

// Logout — POST /api/v1/auth/logout (protected).
func (h *Handler) Logout(c fiber.Ctx) error {
	if err := h.logout(c); err != nil {
		return err
	}
	return c.JSON(fiber.Map{"status": "ok"})
}

// Me — GET /api/v1/auth/me (protected).
func (h *Handler) Me(c fiber.Ctx) error {
	p := middleware.CurrentPrincipal(c)
	return c.JSON(UserResponse{ID: p.ID, Login: p.Login, Name: p.Name, Role: p.Role, Timezone: p.Timezone})
}

// replaceSession — новая сессия в cookie; прежняя сессия этого браузера (вход под
// другим пользователем без выхода) убивается, а не доживает до своего TTL.
func (h *Handler) replaceSession(c fiber.Ctx, sid string) error {
	if old := c.Cookies(middleware.SessionCookieName); old != "" && old != sid {
		if err := h.svc.Logout(c.Context(), old); err != nil {
			return err
		}
	}
	h.auth.SetSessionCookie(c, sid)
	return nil
}

func (h *Handler) logout(c fiber.Ctx) error {
	if sid := c.Cookies(middleware.SessionCookieName); sid != "" {
		if err := h.svc.Logout(c.Context(), sid); err != nil {
			return err
		}
	}
	h.auth.ClearSessionCookie(c)
	return nil
}

// ---- SSR ----

// LoginPageVM — страница входа.
type LoginPageVM struct {
	Shell    ssr.Shell
	Login    string
	Error    string
	Redirect string
}

// LoginPage — GET /login. Уже вошедшего — сразу на redirect.
func (h *Handler) LoginPage(c fiber.Ctx) error {
	redirect := SafeRedirect(c.Query("redirect"))
	if middleware.CurrentPrincipal(c) != nil {
		return c.Redirect().Status(fiber.StatusSeeOther).To(redirect)
	}
	return h.renderLogin(c, fiber.StatusOK, LoginPageVM{Redirect: redirect})
}

// LoginSubmit — POST /login (форма). Успех → 303 на redirect; ошибка → форма с текстом.
func (h *Handler) LoginSubmit(c fiber.Ctx) error {
	vm := LoginPageVM{
		Login:    strings.TrimSpace(c.FormValue("login")),
		Redirect: SafeRedirect(c.FormValue("redirect")),
	}
	password := c.FormValue("password")
	if vm.Login == "" || password == "" {
		vm.Error = "Введите логин и пароль"
		return h.renderLogin(c, fiber.StatusBadRequest, vm)
	}
	sid, u, err := h.svc.Login(c.Context(), vm.Login, password)
	if err != nil {
		var app *errorspkg.AppError
		if errors.As(err, &app) && app.HTTP < fiber.StatusInternalServerError {
			vm.Error = app.Message
			return h.renderLogin(c, app.HTTP, vm)
		}
		return err
	}
	if err := h.replaceSession(c, sid); err != nil {
		return err
	}
	// Поставщик заперт в кабинете (purchase-orders DS-018): ?redirect= на любой иной раздел
	// игнорируется — сразу в /supplier/orders.
	if u != nil && u.Role == middleware.RoleSupplier && !middleware.SupplierSSRAllowed(vm.Redirect) {
		return c.Redirect().Status(fiber.StatusSeeOther).To(middleware.SupplierHome)
	}
	return c.Redirect().Status(fiber.StatusSeeOther).To(vm.Redirect)
}

// LogoutSubmit — POST /logout (форма в сайдбаре) → 303 на /login.
func (h *Handler) LogoutSubmit(c fiber.Ctx) error {
	if err := h.logout(c); err != nil {
		return err
	}
	return c.Redirect().Status(fiber.StatusSeeOther).To("/login")
}

func (h *Handler) renderLogin(c fiber.Ctx, status int, vm LoginPageVM) error {
	vm.Shell = h.shell.Build(c, "Вход")
	c.Status(status)
	return h.r.PageLayout(c, "login", "bare", vm)
}

// SafeRedirect — только локальный путь этого сайта; всё остальное (чужой хост,
// протокол-relative //evil, обратные слэши, javascript:) → "/". Защита от open redirect.
func SafeRedirect(raw string) string {
	if raw == "" || !strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "//") || strings.ContainsAny(raw, "\\\r\n\t") {
		return "/"
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "" || u.Host != "" || u.Path == "/login" {
		return "/"
	}
	return raw
}
