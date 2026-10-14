package catalog

import (
	"github.com/gofiber/fiber/v3"

	"github.com/kitavrus/kanban-tree/internal/features/softdelete"
	"github.com/kitavrus/kanban-tree/internal/server/middleware"
)

// Register — JSON API /api/v1/catalog-categories, /api/v1/products (D19: !isAdmin —
// только чтение активных записей, мутации — admin) + SSR-экраны /catalog/products,
// /catalog/services (заглушка), управление категориями. /catalog — 303 на /catalog/services
// (task.md: «/catalog → 303 /catalog/services»). При s == nil SSR-группы не регистрируются
// (используется в тестах, где рендерер не нужен).
func Register(app *fiber.App, api fiber.Router, a *middleware.Auth, catH *CategoryHandler, prodH *ProductHandler, svcH *ServiceHandler, s *SSR) {
	admin := middleware.RequireAdmin(nil)
	// Мягкое удаление и восстановление (soft-delete-catalog §6.1–§6.6): только admin. Гард
	// softdelete.AdminOnly: executor → 403 «Нет доступа» до поиска записи (DS-003).
	sdAdmin := softdelete.AdminOnly()

	cg := api.Group("/catalog-categories", a.RequireAPI())
	cg.Get("", catH.List)
	cg.Post("", admin, catH.Create)
	cg.Get("/:id", catH.Get)
	cg.Put("/:id", admin, catH.Update)
	cg.Delete("/:id", sdAdmin, catH.Delete)
	cg.Post("/:id/restore", sdAdmin, catH.Restore)

	pg := api.Group("/products", a.RequireAPI())
	pg.Get("", prodH.List)
	pg.Post("", admin, prodH.Create)
	pg.Get("/:id", prodH.Get)
	pg.Put("/:id", admin, prodH.Update)
	pg.Delete("/:id", sdAdmin, prodH.Delete)
	pg.Post("/:id/restore", sdAdmin, prodH.Restore)

	sg := api.Group("/services", a.RequireAPI())
	sg.Get("", svcH.List)
	sg.Post("", admin, svcH.Create)
	sg.Get("/:id", svcH.Get)
	sg.Put("/:id", admin, svcH.Update)
	sg.Delete("/:id", sdAdmin, svcH.Delete)
	sg.Post("/:id/restore", sdAdmin, svcH.Restore)

	if s == nil {
		return
	}
	ssrAdmin := middleware.RequireAdmin(s.status.Forbidden)
	app.Get("/catalog", a.RequireSSR(), ssrAdmin, func(c fiber.Ctx) error {
		return c.Redirect().Status(fiber.StatusSeeOther).To("/catalog/services")
	})

	svc := app.Group("/catalog/services", a.RequireSSR(), ssrAdmin)
	svc.Get("", s.servicesList)
	svc.Get("/new", s.serviceFormNew)
	svc.Post("", s.serviceCreate)
	svc.Get("/:id", s.serviceFormEdit)
	svc.Post("/:id", s.serviceUpdate)
	svc.Post("/:id/archive", s.serviceArchive)
	svc.Post("/:id/restore", s.serviceRestore)
	svc.Post("/:id/delete", s.serviceRemove)

	prod := app.Group("/catalog/products", a.RequireSSR(), ssrAdmin)
	prod.Get("", s.productsList)
	prod.Get("/new", s.productFormNew)
	prod.Post("", s.productCreate)
	prod.Get("/:id", s.productFormEdit)
	prod.Post("/:id", s.productUpdate)
	prod.Post("/:id/archive", s.productArchive)
	prod.Post("/:id/restore", s.productRestore)
	prod.Post("/:id/delete", s.productRemove)

	cats := app.Group("/catalog/categories", a.RequireSSR(), ssrAdmin)
	cats.Post("", s.cat.create)
	cats.Post("/:id/rename", s.cat.rename)
	cats.Post("/:id/archive", s.cat.archive)
	cats.Post("/:id/restore", s.cat.restore)
	cats.Post("/:id/delete", s.categoryRemove)
}
