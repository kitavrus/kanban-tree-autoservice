package suppliers

import (
	"github.com/gofiber/fiber/v3"

	"github.com/kitavrus/kanban-tree/internal/features/softdelete"
	"github.com/kitavrus/kanban-tree/internal/server/middleware"
)

// Register — JSON API /api/v1/suppliers (§6.1–§6.3) + SSR-экран /suppliers; всё — только
// admin (DS-007: executor → 403). Архив — PUT is_active; мягкое удаление и восстановление —
// soft-delete-warehouses-suppliers §6.3, §6.4.
// s == nil — SSR не регистрируется.
func Register(app *fiber.App, api fiber.Router, a *middleware.Auth, h *Handler, s *SSR) {
	admin := middleware.RequireAdmin(nil)
	g := api.Group("/suppliers", a.RequireAPI())
	g.Get("", admin, h.List)
	g.Post("", admin, h.Create)
	g.Put("/:id", admin, h.Update)
	// Гард softdelete.AdminOnly: executor → 403 «Нет доступа» до поиска записи (DS-003).
	sdAdmin := softdelete.AdminOnly()
	g.Delete("/:id", sdAdmin, h.Delete)
	g.Post("/:id/restore", sdAdmin, h.Restore)

	if s == nil {
		return
	}
	p := app.Group("/suppliers", a.RequireSSR(), middleware.RequireAdmin(s.status.Forbidden))
	p.Get("", s.page)
}
