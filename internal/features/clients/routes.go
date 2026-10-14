package clients

import (
	"github.com/gofiber/fiber/v3"

	"github.com/kitavrus/kanban-tree/internal/features/softdelete"
	"github.com/kitavrus/kanban-tree/internal/server/middleware"
)

// Register — JSON API /api/v1/clients + /api/v1/equipment (чтение — RequireAPI, мутации —
// +admin; POST /clients/:id/equipment — RequireAPI без admin, D19: исполнитель создаёт
// оборудование) + SSR-экран /clients + форма единицы оборудования /equipment/:id (только
// admin, ADR-005/ADR-006). При s == nil SSR-группа не регистрируется (используется в
// тестах, где рендерер не нужен — эталон equipmenttypes/routes.go).
func Register(app *fiber.App, api fiber.Router, a *middleware.Auth, h *Handler, s *SSR) {
	admin := middleware.RequireAdmin(nil)
	// Мягкое удаление и восстановление (soft-delete-clients §6.1–§6.4): только admin. Гард
	// softdelete.AdminOnly: executor → 403 «Нет доступа» до поиска записи (DS-003).
	sdAdmin := softdelete.AdminOnly()
	g := api.Group("/clients", a.RequireAPI())
	g.Get("", h.List)
	g.Post("", admin, h.Create)
	g.Get("/:id", h.Get)
	g.Put("/:id", admin, h.Update)
	g.Delete("/:id", sdAdmin, h.Delete)
	g.Post("/:id/restore", sdAdmin, h.Restore)
	g.Get("/:id/equipment", h.ListEquipment)
	g.Post("/:id/equipment", h.CreateEquipment) // D19: без admin-guard

	e := api.Group("/equipment", a.RequireAPI())
	e.Get("", h.SearchEquipment) // transfers-core DS-057: поиск единиц по всем клиентам
	e.Get("/:id", h.GetEquipment)
	e.Get("/:id/history", h.EquipmentHistory)
	e.Put("/:id", admin, h.UpdateEquipment)
	e.Delete("/:id", sdAdmin, h.DeleteEquipment)
	e.Post("/:id/restore", sdAdmin, h.RestoreEquipment)

	if s == nil {
		return
	}
	ssrAdmin := middleware.RequireAdmin(s.status.Forbidden)
	p := app.Group("/clients", a.RequireSSR(), ssrAdmin)
	p.Get("", s.list)
	p.Get("/new", s.formNew)
	p.Post("", s.create)
	p.Get("/:id", s.formEdit)
	p.Post("/:id", s.update)
	p.Post("/:id/delete", s.remove)
	p.Get("/:id/equipment/new", s.eqNew)
	p.Post("/:id/equipment", s.eqCreate)

	q := app.Group("/equipment", a.RequireSSR(), ssrAdmin)
	q.Get("/:id", s.eqEdit)
	q.Get("/:id/history", s.eqHistory)
	q.Post("/:id", s.eqUpdate)
	q.Post("/:id/delete", s.eqRemove)
}
