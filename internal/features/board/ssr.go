package board

import (
	"context"

	"github.com/gofiber/fiber/v3"

	"github.com/kitavrus/kanban-tree/internal/config"
	"github.com/kitavrus/kanban-tree/internal/server/middleware"
	"github.com/kitavrus/kanban-tree/internal/ssr"
)

// Root — шаблоны доски/остатков относительно корня модуля (design-go-layers.md §Handler
// SSR). Данные грузит JS (rich-экран, D30) — SSR отдаёт только каркас + data-* атрибуты.
const Root = "web/ssr/board/templates"

// BoardVM — каркас страницы «/» (design-go-layers.md §Handler SSR); наполнение — фаза 06.
type BoardVM struct {
	Shell    ssr.Shell
	ViewerID string
	IsAdmin  bool
	// Warehouses — опции фильтра «Склад» (DS-009, D59): рендерятся сервером, т.к.
	// /api/v1/warehouses — только admin, а фильтр доступен и executor.
	Warehouses []CardWarehouse
}

// StockVM — каркас страницы «/stock»; наполнение — фаза 07.
type StockVM struct {
	Shell ssr.Shell
}

// SSR — страницы доски (`/`), остатков (`/stock`) и печати актов (`/tasks/:id/print/:kind`,
// фаза 04, ADR-102).
type SSR struct {
	r      *ssr.Renderer
	shell  ssr.ShellBuilder
	status *ssr.StatusPages
	firm   config.FirmInfo
	card   printService
	// warehouses — источник опций фильтра «Склад» (WithWarehouses; nil — фильтр без опций).
	warehouses func(context.Context) ([]CardWarehouse, error)
}

// WithWarehouses — источник складов фильтра доски (сеттер: сигнатура NewSSR не меняется).
func (s *SSR) WithWarehouses(fn func(context.Context) ([]CardWarehouse, error)) *SSR {
	s.warehouses = fn
	return s
}

func NewSSR(r *ssr.Renderer, shell ssr.ShellBuilder, status *ssr.StatusPages, firm config.FirmInfo, card printService) *SSR {
	return &SSR{r: r, shell: shell, status: status, firm: firm, card: card}
}

// board — GET / (доска заказов занимает корень, ADR-100 — home снят в этой фазе).
func (s *SSR) board(c fiber.Ctx) error {
	sh := s.shell.Build(c, "Доска заказов")
	vm := BoardVM{Shell: sh, ViewerID: middleware.CurrentPrincipal(c).ID, IsAdmin: sh.IsAdmin, Warehouses: []CardWarehouse{}}
	if s.warehouses != nil {
		list, err := s.warehouses(c.Context())
		if err != nil {
			return err
		}
		vm.Warehouses = list
	}
	return s.r.Page(c, "board", vm)
}

// stock — GET /stock.
func (s *SSR) stock(c fiber.Ctx) error {
	return s.r.Page(c, "stock", StockVM{Shell: s.shell.Build(c, "Остатки")})
}
