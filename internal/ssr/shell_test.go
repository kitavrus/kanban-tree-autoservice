package ssr

import (
	"context"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// TestShell_NavActive — UT-011 (ADR-006): "/" только точное совпадение; вложенные
// пути подсвечивают родительский пункт; похожие, но не вложенные — нет.
func TestShell_NavActive(t *testing.T) {
	cases := []struct {
		name       string
		activePath string
		url        string
		want       bool
	}{
		{"root exact match", "/", "/", true},
		{"root does not match nested", "/stock", "/", false},
		{"clients matches nested", "/clients/abc", "/clients", true},
		{"clients exact match", "/clients", "/clients", true},
		{"users does not match users-x", "/users-x", "/users", false},
		{"catalog matches nested", "/catalog/products", "/catalog", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sh := Shell{ActivePath: c.activePath}
			if got := sh.NavActive(c.url); got != c.want {
				t.Errorf("Shell{ActivePath:%q}.NavActive(%q) = %v, want %v", c.activePath, c.url, got, c.want)
			}
		})
	}
}

// fakeBadgeSource — тестовая реализация BadgeSource (ADR-004), эталон board.StockService.
type fakeBadgeSource struct{ badges map[string]int }

func (f fakeBadgeSource) NavBadges(context.Context) map[string]int { return f.badges }

func newShellTestCtx(t *testing.T, path string) fiber.Ctx {
	t.Helper()
	app := fiber.New()
	fctx := &fasthttp.RequestCtx{}
	fctx.Request.SetRequestURI(path)
	c := app.AcquireCtx(fctx)
	t.Cleanup(func() { app.ReleaseCtx(c) })
	return c
}

// TestShellBuilder_WithBadges_PassesBadgesToMenu — ADR-004: badges[url] источника попадает
// в NavItem.Badge соответствующего пункта меню.
func TestShellBuilder_WithBadges_PassesBadgesToMenu(t *testing.T) {
	c := newShellTestCtx(t, "/stock")
	b := NewShellBuilder("/", "/stock").WithBadges(fakeBadgeSource{badges: map[string]int{"/stock": 5}})

	sh := b.Build(c, "Остатки")
	require.Len(t, sh.Menu, 1)
	var got int
	found := false
	for _, it := range sh.Menu[0].Items {
		if it.URL == "/stock" {
			got, found = it.Badge, true
		}
	}
	require.True(t, found, "пункт /stock должен быть в меню")
	assert.Equal(t, 5, got)
}

// TestShellBuilder_NoBadgeSource_BadgeZero — без WithBadges (src == nil) все Badge — 0,
// Build не паникует.
func TestShellBuilder_NoBadgeSource_BadgeZero(t *testing.T) {
	c := newShellTestCtx(t, "/stock")
	b := NewShellBuilder("/", "/stock")

	sh := b.Build(c, "Остатки")
	require.NotEmpty(t, sh.Menu)
	for _, g := range sh.Menu {
		for _, it := range g.Items {
			assert.Equal(t, 0, it.Badge)
		}
	}
}
