// Package handler — HTTP-слой закупок: JSON API (§6.1–§6.15, CLAUDE.md §API-1), SSR-экраны
// (§8.1–§8.3, §8.6–§8.7: оболочка в общем shell проекта, данные рисует JS поверх JSON API — как
// «Поставщики»/«Остатки») и PDF (§6.11, §6.16). Ошибки — pkg/errorspkg (FiberErrorHandler).
package handler

import (
	"bytes"
	"encoding/json"

	"github.com/gofiber/fiber/v3"

	"github.com/kitavrus/kanban-tree/internal/features/purchase-orders/service"
	"github.com/kitavrus/kanban-tree/internal/server/middleware"
	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
)

// Handler — JSON API закупок и кабинета поставщика.
type Handler struct {
	svc *service.Service
}

func NewHandler(svc *service.Service) *Handler { return &Handler{svc: svc} }

// actor — пользователь запроса только из сессии (роут закрыт RequireAPI/RequireSSR — principal
// не nil); поставщик учётки — тоже из сессии, не из тела (DS-053).
func actor(c fiber.Ctx) service.Actor {
	p := middleware.CurrentPrincipal(c)
	if p == nil {
		return service.Actor{}
	}
	return service.Actor{ID: p.ID, Role: p.Role, SupplierID: p.SupplierID}
}

// decode — тело JSON без строгой проверки ключей: посторонние поля (created_by, supplier_id,
// order_id и т.п.) игнорируются — автор и поставщик берутся из сессии (§3 abuse cases 3, TS-010).
// Нечитаемое тело — 400 BAD_REQUEST; пустое — нулевой запрос (валидация скажет, чего нет).
func decode(c fiber.Ctx, v any) error {
	body := bytes.TrimSpace(c.Body())
	if len(body) == 0 {
		return nil
	}
	if err := json.Unmarshal(body, v); err != nil {
		return errorspkg.BadRequest("invalid body")
	}
	return nil
}

// Register — JSON API /api/v1/purchase-orders (admin, executor), /api/v1/supplier/purchase-orders
// (supplier) и SSR /purchases, /supplier/orders. Роль supplier дополнительно отсечена общим
// guard'ом RequireAPI/RequireSSR (DS-053).
func Register(app *fiber.App, api fiber.Router, a *middleware.Auth, h *Handler, s *SSR) {
	po := api.Group("/purchase-orders", a.RequireAPI(), middleware.RequireStaff(nil))
	po.Get("", h.list)
	po.Post("", h.create)
	po.Get("/suggestions", h.suggestions)
	po.Get("/:id", h.get)
	po.Put("/:id", h.update)
	po.Delete("/:id", h.delete)
	po.Post("/:id/send", h.send)
	po.Post("/:id/recall", h.recall)
	po.Post("/:id/pick-up", h.pickUp)
	po.Post("/:id/cancel", h.cancel)
	po.Post("/:id/restore", h.restore)

	sp := api.Group("/supplier/purchase-orders", a.RequireAPI(), middleware.RequireSupplier(nil))
	sp.Get("", h.supplierList)
	sp.Get("/:id", h.supplierGet)
	sp.Put("/:id/items/:item_id", h.supplierAnswer)
	sp.Post("/:id/ready", h.supplierReady)

	if s == nil {
		return
	}
	p := app.Group("/purchases", a.RequireSSR(), middleware.RequireStaff(s.status.Forbidden))
	p.Get("", s.list)
	p.Get("/new", s.newOrder)
	p.Get("/:id/print", s.print)
	p.Get("/:id", s.view)

	sup := app.Group("/supplier", a.RequireSSR(), middleware.RequireSupplier(s.status.Forbidden))
	sup.Get("", func(c fiber.Ctx) error { return c.Redirect().Status(fiber.StatusFound).To(middleware.SupplierHome) })
	sup.Get("/orders", s.supplierList)
	sup.Get("/orders/:id/print", s.supplierPrint)
	sup.Get("/orders/:id", s.supplierView)
}
