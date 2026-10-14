package auth

import (
	"github.com/gofiber/fiber/v3"

	"github.com/kitavrus/kanban-tree/internal/server/middleware"
)

// Register монтирует /api/v1/auth/* (§API-1) и SSR-страницы /login, /logout.
// Fiber выполняет хэндлеры В ПОРЯДКЕ АРГУМЕНТОВ: гард идёт первым.
func Register(app *fiber.App, api fiber.Router, h *Handler, a *middleware.Auth) {
	g := api.Group("/auth")
	g.Post("/login", h.Login)
	g.Post("/logout", a.RequireAPI(), h.Logout)
	g.Get("/me", a.RequireAPI(), h.Me)

	app.Get("/login", a.Optional(), h.LoginPage)
	app.Post("/login", h.LoginSubmit)
	app.Post("/logout", h.LogoutSubmit)
}
