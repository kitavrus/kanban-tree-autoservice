package users

import (
	"github.com/gofiber/fiber/v3"

	"github.com/kitavrus/kanban-tree/internal/server/middleware"
)

// Register — JSON API /api/v1/users и SSR-экран /users; всё только для admin
// (plan.md §2). Удаления нет (D39) — блокировка через is_active.
//
// Исключение — PUT /api/v1/users/me/timezone (DS-033): свой пояс меняет ЛЮБОЙ авторизованный.
// Маршрут регистрируется ДО admin-группы: её гарды — префиксные Use("/users"), а хендлеры
// Fiber идут в порядке регистрации, поэтому запрос отвечает здесь и до RequireAdmin (и до
// /:id с id="me") не доходит.
func Register(app *fiber.App, api fiber.Router, a *middleware.Auth, h *Handler, s *SSR) {
	api.Put("/users/me/timezone", a.RequireAPI(), h.SetMyTimezone)

	g := api.Group("/users", a.RequireAPI(), middleware.RequireAdmin(nil))
	g.Get("", h.List)
	g.Post("", h.Create)
	g.Get("/:id", h.Get)
	g.Put("/:id", h.Update)

	p := app.Group("/users", a.RequireSSR(), middleware.RequireAdmin(s.status.Forbidden))
	p.Get("", s.list)
	p.Get("/new", s.formNew)
	p.Post("", s.create)
	p.Get("/:id", s.formEdit)
	p.Post("/:id", s.update)
}
