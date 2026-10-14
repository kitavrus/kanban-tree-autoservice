package board

import (
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
}

func NewSSR(r *ssr.Renderer, shell ssr.ShellBuilder, status *ssr.StatusPages, firm config.FirmInfo, card printService) *SSR {
	return &SSR{r: r, shell: shell, status: status, firm: firm, card: card}
}

// board — GET / (доска заказов занимает корень, ADR-100 — home снят в этой фазе).
func (s *SSR) board(c fiber.Ctx) error {
	sh := s.shell.Build(c, "Доска заказов")
	vm := BoardVM{Shell: sh, ViewerID: middleware.CurrentPrincipal(c).ID, IsAdmin: sh.IsAdmin}
	return s.r.Page(c, "board", vm)
}

// stock — GET /stock.
func (s *SSR) stock(c fiber.Ctx) error {
	return s.r.Page(c, "stock", StockVM{Shell: s.shell.Build(c, "Остатки")})
}
