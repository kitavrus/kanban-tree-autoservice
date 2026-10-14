package ssr

import (
	"context"
	"strings"
	"time"
	"unicode"

	"github.com/gofiber/fiber/v3"

	"github.com/kitavrus/kanban-tree/pkg/timezone"
)

// BadgeSource — источник счётчиков сайдбара (ADR-004): StockService.NavBadges реализует
// его структурно. Ошибка внутри реализации — её забота (слог + nil), не роняет Build.
type BadgeSource interface {
	NavBadges(ctx context.Context) map[string]int
}

// RoleAdmin — роль администратора (plan.md §2); остальные роли видят меню «Работа».
const RoleAdmin = "admin"

// LocalViewer — Locals-ключ текущего пользователя для layout. Кладёт auth-middleware
// (фаза 2); нет значения — гость: меню строится без admin-пунктов, блок пользователя скрыт.
const LocalViewer = "ssr_viewer"

// LocalCSRFToken — Locals-ключ CSRF-токена запроса (CSRF-middleware, фаза 2).
const LocalCSRFToken = "ssr_csrf_token"

// Viewer — кто смотрит страницу (для сайдбара и прав в шаблонах).
type Viewer struct {
	Name string
	Role string
	// Timezone — часовой пояс пользователя (IANA, DS-031). В Shell.User всегда непустой:
	// Build подставляет пояс по умолчанию. Шаблоны: {{ timeIn $.Shell.User.Timezone … }}.
	Timezone string
}

// ViewerTimezone — пояс пользователя текущего запроса для экранов без Shell (печатные
// формы); гость/пусто → пояс по умолчанию.
func ViewerTimezone(c fiber.Ctx) string {
	if v, _ := c.Locals(LocalViewer).(Viewer); v.Timezone != "" {
		return v.Timezone
	}
	return timezone.Default
}

// Shell — данные общего layout "base", одинаковые для любой страницы любой фичи.
// Встраивается во view-model страницы полем Shell; хендлер заполняет его через
// ShellBuilder.Build перед Renderer.Page (сам Renderer сессию не читает).
type Shell struct {
	Title      string // заголовок топбара и <title>
	Menu       []NavGroup
	ActivePath string
	User       Viewer
	IsAdmin    bool
	// CSRFToken — формы отдают полем _csrf, fetch — заголовком X-Csrf-Token
	// (скрипты берут из <meta name="csrf-token">).
	CSRFToken string
}

// ShellBuilder собирает Shell из запроса. Один экземпляр на процесс.
type ShellBuilder struct {
	enabled map[string]bool
	src     BadgeSource
}

// NewShellBuilder — enabled: пути разделов, уже реализованных в приложении. Пункты
// меню остальных разделов скрыты, чтобы сайдбар не вёл на 404 между фазами плана.
func NewShellBuilder(enabled ...string) ShellBuilder {
	b := ShellBuilder{enabled: map[string]bool{}}
	for _, p := range enabled {
		b.enabled[p] = true
	}
	return b
}

// WithBadges — копия ShellBuilder с источником счётчиков сайдбара (ADR-004). src == nil
// допустим (снимает источник) — Build тогда строит меню без бейджей (все Badge=0).
func (b ShellBuilder) WithBadges(src BadgeSource) ShellBuilder {
	b.src = src
	return b
}

// Build — Shell для текущего запроса с заголовком title.
func (b ShellBuilder) Build(c fiber.Ctx, title string) Shell {
	v, _ := c.Locals(LocalViewer).(Viewer)
	token, _ := c.Locals(LocalCSRFToken).(string)
	isAdmin := v.Role == RoleAdmin
	v.Timezone = ViewerTimezone(c)
	var badges map[string]int
	if b.src != nil {
		badges = b.src.NavBadges(c.Context())
	}
	return Shell{
		Title:      title,
		Menu:       BuildMenu(b.enabled, isAdmin, badges),
		ActivePath: c.Path(),
		User:       v,
		IsAdmin:    isAdmin,
		CSRFToken:  token,
	}
}

// Timezones — пункты select'а «Часовой пояс» в сайдбаре (подписи смещения — на сейчас).
func (s Shell) Timezones() []timezone.Option {
	return timezone.Options(time.Now(), s.User.Timezone)
}

// NavActive — подсвечивать ли пункт меню с URL url для текущего ActivePath (ADR-006).
// "/" — только точное совпадение (иначе подсвечивался бы на любой странице); остальные
// пункты — точное совпадение либо ActivePath — вложенный путь (url + "/" + …).
func (s Shell) NavActive(url string) bool {
	if url == "/" {
		return s.ActivePath == "/"
	}
	return s.ActivePath == url || strings.HasPrefix(s.ActivePath, url+"/")
}

// Initials — «Сидоров Пётр» → «СП» (аватар в сайдбаре, как в прототипе).
func Initials(name string) string {
	var out []rune
	for _, w := range strings.Fields(name) {
		r := []rune(w)
		out = append(out, unicode.ToUpper(r[0]))
		if len(out) == 2 {
			break
		}
	}
	return string(out)
}
