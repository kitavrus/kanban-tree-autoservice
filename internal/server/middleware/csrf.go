package middleware

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net/url"

	"github.com/gofiber/fiber/v3"

	"github.com/kitavrus/kanban-tree/internal/ssr"
	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
)

const (
	CSRFCookieName = "kt_csrf"
	CSRFHeaderName = "X-Csrf-Token"
	CSRFFormField  = "_csrf"
	csrfTokenLen   = 64 // hex от 32 байт
)

// CSRF — stateless double-submit (plan.md §2): токен живёт в HttpOnly-cookie kt_csrf,
// страница получает его через <meta name="csrf-token"> (ssr.LocalCSRFToken), формы шлют
// полем _csrf, fetch — заголовком X-Csrf-Token. Небезопасный метод без совпадения
// присланного токена с cookie → 403 CSRF_INVALID. Хранилища нет: токен переживает
// рестарт сервера и не протухает на долго открытой форме.
//
// Дополнительно: при наличии заголовка Origin он обязан совпадать с хостом запроса
// (запрос с чужого сайта отбивается даже при утечке токена).
func CSRF(secure bool) fiber.Handler {
	return func(c fiber.Ctx) error {
		cookieToken := c.Cookies(CSRFCookieName)
		token := cookieToken
		if !validCSRFToken(token) {
			token = newCSRFToken()
			c.Cookie(&fiber.Cookie{
				Name:     CSRFCookieName,
				Value:    token,
				Path:     "/",
				HTTPOnly: true,
				Secure:   secure,
				SameSite: fiber.CookieSameSiteStrictMode,
			})
		}
		c.Locals(ssr.LocalCSRFToken, token)

		switch c.Method() {
		case fiber.MethodGet, fiber.MethodHead, fiber.MethodOptions, fiber.MethodTrace:
			return c.Next()
		}
		if origin := c.Get(fiber.HeaderOrigin); origin != "" && !sameOrigin(origin, c.Host()) {
			return errorspkg.New("CSRF_INVALID", fiber.StatusForbidden, "cross-origin request")
		}
		sent := c.Get(CSRFHeaderName)
		if sent == "" {
			sent = c.FormValue(CSRFFormField)
		}
		if !validCSRFToken(cookieToken) || subtle.ConstantTimeCompare([]byte(sent), []byte(cookieToken)) != 1 {
			return errorspkg.New("CSRF_INVALID", fiber.StatusForbidden, "csrf token missing or invalid")
		}
		return c.Next()
	}
}

func newCSRFToken() string {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		panic(err) // crypto/rand не падает на поддерживаемых ОС
	}
	return hex.EncodeToString(raw)
}

func validCSRFToken(t string) bool {
	if len(t) != csrfTokenLen {
		return false
	}
	_, err := hex.DecodeString(t)
	return err == nil
}

func sameOrigin(origin, host string) bool {
	u, err := url.Parse(origin)
	return err == nil && u.Host != "" && u.Host == host
}
