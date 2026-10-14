package executorpayouts

import (
	"github.com/gofiber/fiber/v3"

	"github.com/kitavrus/kanban-tree/internal/server/middleware"
)

// Register — JSON API /api/v1/executor-payouts + SSR-экран /executor-payouts. Доступно
// всем авторизованным: выплаты отмечают и admin, и executor (D62). s == nil — SSR не
// регистрируется.
func Register(app *fiber.App, api fiber.Router, a *middleware.Auth, h *Handler, s *SSR) {
	g := api.Group("/executor-payouts", a.RequireAPI())
	g.Get("", h.List)
	g.Get("/summary", h.Summary)
	g.Post("/mark", h.Mark)
	g.Put("/:id", h.UpdatePayout)

	if s == nil {
		return
	}
	app.Get("/executor-payouts", a.RequireSSR(), s.page)
}
