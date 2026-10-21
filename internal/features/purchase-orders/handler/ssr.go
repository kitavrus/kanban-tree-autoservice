package handler

import (
	"errors"

	"github.com/gofiber/fiber/v3"

	"github.com/kitavrus/kanban-tree/internal/features/purchase-orders/constants"
	"github.com/kitavrus/kanban-tree/internal/features/purchase-orders/repository"
	"github.com/kitavrus/kanban-tree/internal/features/purchase-orders/service"
	"github.com/kitavrus/kanban-tree/internal/ssr"
	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
)

// Root — шаблоны экранов закупок и кабинета поставщика относительно корня модуля. Layout —
// общий shell проекта (web/ssr/_layout, ssr.SharedRoot), своего base.html у фичи нет.
const Root = "web/ssr/purchase-orders/templates"

// SSR — экраны §8.1–§8.3 (admin, executor) и §8.6–§8.7 (supplier) + PDF §6.11/§6.16.
type SSR struct {
	svc    *service.Service
	r      *ssr.Renderer
	shell  ssr.ShellBuilder
	status *ssr.StatusPages
}

func NewSSR(svc *service.Service, r *ssr.Renderer, shell ssr.ShellBuilder, status *ssr.StatusPages) *SSR {
	return &SSR{svc: svc, r: r, shell: shell, status: status}
}

// StatusOption — пункт фильтра статуса.
type StatusOption struct {
	Value, Title string
}

// ListVM — §8.1 / §8.6: оболочка, данные грузит JS.
type ListVM struct {
	Shell     ssr.Shell
	Statuses  []StatusOption
	FirmName  string
	FirmPhone string
}

// NewVM — §8.2: получатели из SSR, дефицит — JS (§6.10).
type NewVM struct {
	Shell     ssr.Shell
	Suppliers []repository.SupplierOption
	Bazaar    string
}

// ViewVM — §8.3 / §8.7: id заказа; карточку грузит JS.
type ViewVM struct {
	Shell     ssr.Shell
	ID        string
	IsAdmin   bool
	FirmName  string
	FirmPhone string
}

func statusOptions(list []string) []StatusOption {
	out := make([]StatusOption, 0, len(list))
	for _, s := range list {
		out = append(out, StatusOption{Value: s, Title: constants.StatusTitles[s]})
	}
	return out
}

// list — GET /purchases.
func (s *SSR) list(c fiber.Ctx) error {
	return s.r.Page(c, "list", ListVM{Shell: s.shell.Build(c, "Закупки"), Statuses: statusOptions(constants.Statuses)})
}

// newOrder — GET /purchases/new.
func (s *SSR) newOrder(c fiber.Ctx) error {
	sups, err := s.svc.ActiveSuppliers(c.Context())
	if err != nil {
		return err
	}
	return s.r.Page(c, "new", NewVM{Shell: s.shell.Build(c, "Новый заказ"), Suppliers: sups, Bazaar: "Базар (покупаем сами)"})
}

// notFound — 404-страница для «нет/удалён/чужой» (§6.3, §6.11), иначе ошибка как есть.
func (s *SSR) notFound(c fiber.Ctx, err error) error {
	if errors.Is(err, errorspkg.ErrPurchaseOrderNotFound) {
		return s.status.NotFound(c)
	}
	return err
}

// view — GET /purchases/:id: чужой (executor) или удалённый — 404-страница (DS-060).
func (s *SSR) view(c fiber.Ctx) error {
	a := actor(c)
	o, err := s.svc.Model(c.Context(), a, c.Params("id"))
	if err != nil {
		return s.notFound(c, err)
	}
	return s.r.Page(c, "view", ViewVM{Shell: s.shell.Build(c, "Заказ "+constants.NumberDisplay(o.Number)), ID: o.ID, IsAdmin: a.Role == service.RoleAdmin})
}

// supplierList — GET /supplier/orders (§8.6).
func (s *SSR) supplierList(c fiber.Ctx) error {
	f := s.svc.Firm()
	return s.r.Page(c, "supplier-list", ListVM{Shell: s.shell.Build(c, "Заказы"),
		Statuses: statusOptions(constants.SupplierStatuses), FirmName: f.Name, FirmPhone: f.Phone})
}

// supplierView — GET /supplier/orders/:id (§8.7): чужой/черновик/отменён — «Заказ не найден».
func (s *SSR) supplierView(c fiber.Ctx) error {
	f := s.svc.Firm()
	o, err := s.svc.SupplierModel(c.Context(), actor(c), c.Params("id"))
	if errors.Is(err, errorspkg.ErrPurchaseOrderNotFound) {
		c.Status(fiber.StatusNotFound)
		return s.r.Page(c, "supplier-view", ViewVM{Shell: s.shell.Build(c, "Заказ не найден"), FirmName: f.Name, FirmPhone: f.Phone})
	}
	if err != nil {
		return err
	}
	return s.r.Page(c, "supplier-view", ViewVM{Shell: s.shell.Build(c, "Заказ "+constants.NumberDisplay(o.Number)),
		ID: o.ID, FirmName: f.Name, FirmPhone: f.Phone})
}
