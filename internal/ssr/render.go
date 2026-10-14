// Package ssr — общий SSR-рендерер проекта (Go html/template + vanilla JS, D30).
//
// Совместим с /z-scaffold: тот генерирует internal/ssr/render.go ОДИН раз на проект
// (SKIP, если файл уже есть) и в хендлерах фич зовёт ssr.NewRenderer(root) /
// r.Page / r.Fragment — сигнатуры сохранены, sharedRoots — вариадик-надстройка.
//
// Каждая фича держит свои pages/partials в web/ssr/{feature}/templates (у каждой свой
// Renderer: html/template требует уникальных {{ define }} внутри набора). Общий layout
// приложения (сайдбар, топбар, тема — "base") живёт в web/ssr/_layout/templates — ВНЕ
// корня раздаваемой статики web/ssr/_shared — и подмешивается в каждую страницу через
// sharedRoots (паттерн zerem_wms).
//
// Подключение фичи от /z-scaffold (он штампует свой layouts/base.html и BaseVM без Shell):
//  1. удалить web/ssr/{feature}/templates/layouts/base.html — иначе два {{ define "base" }};
//  2. добавить в dto.BaseVM поле `Shell ssr.Shell` и заполнять его в хендлерах
//     (h.Shell.Build(c, title)) — общий layout читает .Shell.*;
//  3. в router.go: ssr.NewRenderer("web/ssr/{feature}/templates", ssr.SharedRoot),
//     путь раздела — в ssr.NewShellBuilder(...), чтобы он появился в сайдбаре.
//
// Пути — относительно корня модуля (make run / LiveSuite запускают бинарь оттуда).
package ssr

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"path/filepath"
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/kitavrus/kanban-tree/pkg/timezone"
)

// SharedRoot — общий набор шаблонов (layout "base") относительно корня модуля.
const SharedRoot = "web/ssr/_layout/templates"

type Renderer struct {
	pages     map[string]*template.Template // имя страницы → set (layouts+partials+page)
	fragments *template.Template            // только partials — для AJAX-фрагментов
}

// NewRenderer собирает Renderer из root (layouts/partials/pages фичи) плюс layouts/
// partials каждого из sharedRoots. Страницы берутся только из root.
func NewRenderer(root string, sharedRoots ...string) (*Renderer, error) {
	var layouts, partials []string
	for _, dir := range append([]string{root}, sharedRoots...) {
		l, err := filepath.Glob(filepath.Join(dir, "layouts", "*.html"))
		if err != nil {
			return nil, err
		}
		p, err := filepath.Glob(filepath.Join(dir, "partials", "*.html"))
		if err != nil {
			return nil, err
		}
		layouts = append(layouts, l...)
		partials = append(partials, p...)
	}
	pages, err := filepath.Glob(filepath.Join(root, "pages", "*.html"))
	if err != nil {
		return nil, err
	}
	if len(layouts) == 0 || len(pages) == 0 {
		return nil, fmt.Errorf("ssr: не найдены шаблоны в %s", root)
	}

	r := &Renderer{pages: map[string]*template.Template{}}
	for _, p := range pages {
		files := append(append(append([]string{}, layouts...), partials...), p)
		name := strings.TrimSuffix(filepath.Base(p), ".html")
		t, err := template.New(name).Funcs(Funcs).ParseFiles(files...)
		if err != nil {
			return nil, fmt.Errorf("ssr: страница %s: %w", name, err)
		}
		r.pages[name] = t
	}
	// Набор без partials: ParseFiles с пустым списком паникует — fragments опциональны.
	if len(partials) > 0 {
		frag, err := template.New("fragments").Funcs(Funcs).ParseFiles(partials...)
		if err != nil {
			return nil, fmt.Errorf("ssr: partials: %w", err)
		}
		r.fragments = frag
	}
	return r, nil
}

// Funcs — функции шаблонов, общие для всех наборов.
var Funcs = template.FuncMap{
	"initials": Initials,
	// timeIn — время в поясе пользователя (DS-031, DS-032): {{ timeIn $.Shell.User.Timezone
	// "02.01.2006 15:04" .CreatedAt }}. Значение — time.Time или *time.Time (nil → "").
	// Форматировать время в шаблонах напрямую (.Format) нельзя — выйдет пояс сервера.
	"timeIn": timezone.Format,
}

// Render пишет страницу page в layout в w — без HTTP (unit-тесты, печатные формы).
func (r *Renderer) Render(w io.Writer, page, layout string, vm any) error {
	t, ok := r.pages[page]
	if !ok {
		return fmt.Errorf("ssr: нет страницы %q", page)
	}
	return t.ExecuteTemplate(w, layout, vm)
}

// Page — полная страница: layout "base" + блоки страницы.
func (r *Renderer) Page(c fiber.Ctx, page string, vm any) error {
	return r.PageLayout(c, page, "base", vm)
}

// PageLayout — страница в указанном layout ("bare" — без сайдбара: вход, ошибки).
func (r *Renderer) PageLayout(c fiber.Ctx, page, layout string, vm any) error {
	var buf bytes.Buffer
	if err := r.Render(&buf, page, layout, vm); err != nil {
		return err
	}
	c.Type("html", "utf-8")
	return c.Send(buf.Bytes())
}

// Fragment — именованный partial для AJAX-ответа.
func (r *Renderer) Fragment(c fiber.Ctx, name string, vm any) error {
	if r.fragments == nil {
		return fmt.Errorf("ssr: у набора без partials нет фрагментов (%q)", name)
	}
	var buf bytes.Buffer
	if err := r.fragments.ExecuteTemplate(&buf, name, vm); err != nil {
		return err
	}
	c.Type("html", "utf-8")
	return c.Send(buf.Bytes())
}
