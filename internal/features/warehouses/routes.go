package warehouses

import (
	"github.com/gofiber/fiber/v3"

	"github.com/kitavrus/kanban-tree/internal/features/softdelete"
	"github.com/kitavrus/kanban-tree/internal/server/middleware"
)

// Register — JSON API /api/v1/warehouses (DS-011) + SSR-экран /warehouses; всё — только
// admin (DS-010, TS-014: executor → 403). Склады для select'ов доски/карточки товара
// приходят в их собственных ответах, не через этот API. s == nil — SSR не регистрируется.
func Register(app *fiber.App, api fiber.Router, a *middleware.Auth, h *Handler, s *SSR) {
	admin := middleware.RequireAdmin(nil)
	g := api.Group("/warehouses", a.RequireAPI())
	g.Get("", admin, h.List)
	g.Post("", admin, h.Create)
	g.Put("/:id", admin, h.Update)
	// Мягкое удаление и восстановление из «Удалённых» (soft-delete-warehouses-suppliers §6.1, §6.2).
	// Гард softdelete.AdminOnly: executor → 403 «Нет доступа» до поиска записи (DS-003).
	sdAdmin := softdelete.AdminOnly()
	g.Delete("/:id", sdAdmin, h.Delete)
	g.Post("/:id/restore", sdAdmin, h.Restore)

	if s == nil {
		return
	}
	p := app.Group("/warehouses", a.RequireSSR(), middleware.RequireAdmin(s.status.Forbidden))
	p.Get("", s.list)
	p.Post("", s.create)
	p.Get("/:id", s.formEdit)
	p.Post("/:id", s.update)
	p.Post("/:id/archive", s.archive)
	p.Post("/:id/restore", s.restore) // возврат из АРХИВА (не восстановление удалённого)
	p.Post("/:id/delete", s.remove)
}
