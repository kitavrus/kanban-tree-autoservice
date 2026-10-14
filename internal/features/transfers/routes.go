package transfers

import (
	"github.com/gofiber/fiber/v3"

	"github.com/kitavrus/kanban-tree/internal/features/softdelete"
	"github.com/kitavrus/kanban-tree/internal/server/middleware"
)

// Register — JSON API /api/v1/transfers + SSR-экран /transfers: любой авторизованный
// (transfers-partners DS-018). Права по направлениям — в сервисе: исполнитель работает только
// с документами с клиентом (чужое направление: создание → 403, остальное → 404).
// s == nil — SSR не регистрируется.
func Register(app *fiber.App, api fiber.Router, a *middleware.Auth, h *Handler, s *SSR) {
	g := api.Group("/transfers", a.RequireAPI())
	g.Get("", h.List)
	g.Post("", h.Create)
	g.Get("/:id", h.Get)
	g.Put("/:id", h.Update)
	g.Post("/:id/send", h.Send)
	g.Post("/:id/receive", h.Receive)
	g.Post("/:id/cancel", h.Cancel)
	// transfers-revert: только администратор — роль проверяет сервис (исполнитель → 403).
	g.Post("/:id/unreceive", h.Unreceive)
	// soft-delete-transfers §6.1, §6.2: удаление — мягкое, с причиной; восстановление — только
	// admin. Гард softdelete.AdminOnly: executor → 403 «Нет доступа» до поиска записи (DS-003).
	sdAdmin := softdelete.AdminOnly()
	g.Delete("/:id", sdAdmin, h.Delete)
	g.Post("/:id/restore", sdAdmin, h.Restore)

	if s == nil {
		return
	}
	p := app.Group("/transfers", a.RequireSSR())
	p.Get("", s.page)
}
