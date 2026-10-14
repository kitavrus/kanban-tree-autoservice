package executorpayouts

import (
	"github.com/gofiber/fiber/v3"

	"github.com/kitavrus/kanban-tree/internal/ssr"
)

// Root — шаблоны экрана «Выплаты» относительно корня модуля.
const Root = "web/ssr/executorpayouts/templates"

// SSR — экран-отчёт «Выплаты» (D69): группы «исполнитель → его услуги», итоги и массовую
// отметку рисует js/executor-payouts.js поверх JSON API (как «Остатки»).
type SSR struct {
	r     *ssr.Renderer
	shell ssr.ShellBuilder
}

func NewSSR(r *ssr.Renderer, shell ssr.ShellBuilder) *SSR {
	return &SSR{r: r, shell: shell}
}

// PageVM — оболочка + роль: executor по умолчанию видит «мои» услуги, admin — все.
type PageVM struct {
	Shell   ssr.Shell
	IsAdmin bool
}

// page — GET /executor-payouts.
func (h *SSR) page(c fiber.Ctx) error {
	sh := h.shell.Build(c, "Выплаты")
	return h.r.Page(c, "list", PageVM{Shell: sh, IsAdmin: sh.IsAdmin})
}
