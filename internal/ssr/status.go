package ssr

import "github.com/gofiber/fiber/v3"

// StatusVM — служебная страница (403/404) внутри общего layout.
type StatusVM struct {
	Shell   Shell
	Code    int
	Heading string
	Message string
}

// StatusPages — рендер служебных страниц из общего набора (web/ssr/_layout/templates/pages).
type StatusPages struct {
	R     *Renderer
	Shell ShellBuilder
}

// NewStatusPages — r собран как NewRenderer(SharedRoot): страницы общего набора.
func NewStatusPages(r *Renderer, shell ShellBuilder) *StatusPages {
	return &StatusPages{R: r, Shell: shell}
}

// Forbidden — 403 для страниц, недоступных по роли (исполнитель → справочники).
func (s *StatusPages) Forbidden(c fiber.Ctx) error {
	return s.render(c, fiber.StatusForbidden, "Нет доступа", "Раздел доступен только администратору.")
}

// NotFound — 404 для несуществующих записей в SSR-экранах.
func (s *StatusPages) NotFound(c fiber.Ctx) error {
	return s.render(c, fiber.StatusNotFound, "Не найдено", "Запись не существует или была удалена.")
}

func (s *StatusPages) render(c fiber.Ctx, code int, heading, msg string) error {
	c.Status(code)
	return s.R.Page(c, "status", StatusVM{Shell: s.Shell.Build(c, heading), Code: code, Heading: heading, Message: msg})
}
