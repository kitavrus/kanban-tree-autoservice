package suppliers

import (
	"github.com/gofiber/fiber/v3"

	"github.com/kitavrus/kanban-tree/internal/ssr"
)

// Root — шаблоны экрана «Поставщики» относительно корня модуля.
const Root = "web/ssr/suppliers/templates"

// SSR — экран «Поставщики» (admin, DS-007, §8.1): таблица, поиск, toggle архивных и форма
// рисует js/suppliers.js поверх JSON API (как экран «Остатки»); открытая форма —
// ?id=<uuid> (правка) либо ?new=1 (создание).
type SSR struct {
	r      *ssr.Renderer
	shell  ssr.ShellBuilder
	status *ssr.StatusPages
}

func NewSSR(r *ssr.Renderer, shell ssr.ShellBuilder, status *ssr.StatusPages) *SSR {
	return &SSR{r: r, shell: shell, status: status}
}

// PageVM — только оболочка: данные грузит JS.
type PageVM struct {
	Shell ssr.Shell
}

// page — GET /suppliers.
func (h *SSR) page(c fiber.Ctx) error {
	return h.r.Page(c, "list", PageVM{Shell: h.shell.Build(c, "Поставщики")})
}
