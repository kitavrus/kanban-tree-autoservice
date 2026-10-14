package softdelete

import (
	"github.com/gofiber/fiber/v3"

	"github.com/kitavrus/kanban-tree/internal/ssr"
)

// Root — шаблоны экрана «Удалённые» относительно корня модуля.
const Root = "web/ssr/softdelete/templates"

// SSR — экран «Удалённые» (admin, §8.1): вкладки, фильтры, таблицу и восстановление рисует
// js/deleted.js поверх JSON API; вкладка, фильтры и страница списка — в адресной строке.
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

// page — GET /deleted.
func (h *SSR) page(c fiber.Ctx) error {
	return h.r.Page(c, "list", PageVM{Shell: h.shell.Build(c, "Удалённые")})
}
