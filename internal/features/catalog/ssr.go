package catalog

import (
	"log/slog"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/requestid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kitavrus/kanban-tree/internal/ssr"
)

// SSR — экраны «Каталог»: товары (список/форма), заглушка услуг, управление категориями
// (общее для обеих вкладок). Типы оборудования — отдельный SSR (equipmenttypes), но
// делит с этим набором полосу вкладок (catalog-tabs partial + TabCounts).
type SSR struct {
	catSvc  *CategoryService
	prodSvc *ProductService
	svcSvc  *ServiceService
	pool    *pgxpool.Pool
	r       *ssr.Renderer
	shell   ssr.ShellBuilder
	status  *ssr.StatusPages
	cat     ssrCategories
}

func NewSSR(catSvc *CategoryService, prodSvc *ProductService, svcSvc *ServiceService, pool *pgxpool.Pool, r *ssr.Renderer, shell ssr.ShellBuilder, status *ssr.StatusPages) *SSR {
	return &SSR{catSvc: catSvc, prodSvc: prodSvc, svcSvc: svcSvc, pool: pool, r: r, shell: shell, status: status, cat: ssrCategories{svc: catSvc, status: status}}
}

// TabsVM — полоса вкладок каталога (шаблон web/ssr/_layout/templates/partials/catalog-tabs.html).
type TabsVM struct {
	Active   string // "services" | "products" | "equipment-types"
	Services int
	Products int
	Types    int
}

// tabs — счётчики полосы вкладок; ошибка НЕ проглатывается молча (code review LOW#2) —
// логируется с request_id, страница рендерится дальше с нулевыми счётчиками (не 500 из-за
// вспомогательной подписи вкладки).
func (h *SSR) tabs(c fiber.Ctx, active string) TabsVM {
	cnt, err := LoadTabCounts(c.Context(), h.pool)
	if err != nil {
		slog.ErrorContext(c.Context(), "catalog: не удалось посчитать вкладки", "error", err, "request_id", requestid.FromContext(c))
		return TabsVM{Active: active}
	}
	return TabsVM{Active: active, Services: cnt.Services, Products: cnt.Products, Types: cnt.Types}
}

var flashes = map[string]string{
	"created":           "Товар создан",
	"saved":             "Изменения сохранены",
	"archived":          "Товар отправлен в архив",
	"restored":          "Товар возвращён из архива",
	"service_created":   "Услуга создана",
	"service_saved":     "Изменения сохранены",
	"service_archived":  "Услуга отправлена в архив",
	"service_restored":  "Услуга возвращена из архива",
	"category_created":  "Категория создана",
	"category_saved":    "Категория переименована",
	"category_archived": "Категория отправлена в архив",
	"category_restored": "Категория возвращена из архива",
}
