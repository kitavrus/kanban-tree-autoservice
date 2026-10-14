package calendar

import (
	"github.com/gofiber/fiber/v3"

	"github.com/kitavrus/kanban-tree/internal/server/middleware"
)

// Register — JSON API /api/v1/calendar + SSR-экран /calendar. Смотреть загрузку могут все
// авторизованные, менять вместимость — только admin. s == nil — SSR не регистрируется.
func Register(app *fiber.App, api fiber.Router, a *middleware.Auth, h *Handler, s *SSR) {
	g := api.Group("/calendar", a.RequireAPI())
	g.Get("", h.Calendar)
	g.Get("/settings", h.GetSettings)
	g.Put("/settings", middleware.RequireAdmin(nil), h.PutSettings)

	if s == nil {
		return
	}
	app.Get("/calendar", a.RequireSSR(), s.page)
}
