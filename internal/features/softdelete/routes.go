package softdelete

import (
	"github.com/gofiber/fiber/v3"

	"github.com/kitavrus/kanban-tree/internal/server/middleware"
	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
)

// AdminOnly — гард роутов удаления, восстановления и раздела «Удалённые» (DS-003): не admin →
// 403 FORBIDDEN «Нет доступа» до любого чтения записи. Общий для всех видов записей.
func AdminOnly() fiber.Handler {
	return middleware.RequireAdmin(func(fiber.Ctx) error { return errorspkg.Forbidden("Нет доступа") })
}

// Register — JSON API /api/v1/deleted (§6.3, §6.4) + SSR-экран /deleted; всё — только admin
// (DS-003: executor → 403 «Нет доступа»). s == nil — SSR не регистрируется.
func Register(app *fiber.App, api fiber.Router, a *middleware.Auth, h *Handler, s *SSR) {
	g := api.Group("/deleted", a.RequireAPI(), AdminOnly())
	g.Get("", h.List)
	g.Get("/summary", h.Summary)

	if s == nil {
		return
	}
	p := app.Group("/deleted", a.RequireSSR(), middleware.RequireAdmin(s.status.Forbidden))
	p.Get("", s.page)
}
