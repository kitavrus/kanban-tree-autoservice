package equipmenttypes

import (
	"github.com/gofiber/fiber/v3"

	"github.com/kitavrus/kanban-tree/internal/features/softdelete"
	"github.com/kitavrus/kanban-tree/internal/server/middleware"
)

// Register — JSON API /api/v1/equipment-types (D28: !isAdmin — только чтение активных
// типов, мутации — admin) + SSR-экран /catalog/equipment-types (только admin,
// ADR-005/ADR-006). Редирект с /catalog (303) владеет пакет catalog (фаза 4). При s == nil
// SSR-группа не регистрируется (используется в тестах, где рендерер не нужен).
func Register(app *fiber.App, api fiber.Router, a *middleware.Auth, h *Handler, s *SSR) {
	admin := middleware.RequireAdmin(nil)
	g := api.Group("/equipment-types", a.RequireAPI())
	g.Get("", h.List)
	g.Post("", admin, h.Create)
	g.Get("/:id", h.Get)
	g.Put("/:id", admin, h.Update)
	// Мягкое удаление и восстановление (soft-delete-catalog §6.7, §6.8): только admin. Гард
	// softdelete.AdminOnly: executor → 403 «Нет доступа» до поиска записи (DS-003).
	sdAdmin := softdelete.AdminOnly()
	g.Delete("/:id", sdAdmin, h.Delete)
	g.Post("/:id/restore", sdAdmin, h.Restore)

	if s == nil {
		return
	}
	ssrAdmin := middleware.RequireAdmin(s.status.Forbidden)
	p := app.Group("/catalog/equipment-types", a.RequireSSR(), ssrAdmin)
	p.Get("", s.list)
	p.Get("/new", s.formNew)
	p.Post("", s.create)
	p.Get("/:id", s.formEdit)
	p.Post("/:id", s.update)
	p.Post("/:id/archive", s.archive)
	p.Post("/:id/restore", s.restore)
	p.Post("/:id/delete", s.remove)
}
