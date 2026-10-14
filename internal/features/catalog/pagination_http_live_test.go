//go:build integration

package catalog_test

import (
	"context"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// ---- pagination DS-005/DS-006: SSR-пейджер /catalog/products и /catalog/services ----
// Методы CatalogLiveSuite (§TD-8: реальный бинарь + TCP + логин). Позиции — своей
// категории (генерация SQL одним INSERT … generate_series — через API 150 POST'ов дольше
// без пользы для сценария); выдача фильтруется по category — чужие данные suite не мешают.

var (
	pgPrevRe  = regexp.MustCompile(`href="([^"]*)" rel="prev" data-testid="catalog-prev"`)
	pgNextRe  = regexp.MustCompile(`href="([^"]*)" rel="next" data-testid="catalog-next"`)
	pgShownRe = regexp.MustCompile(`data-testid="catalog-shown">([^<]*)<`)
	pgRowRe   = regexp.MustCompile(`data-testid="(?:product|service)-row" data-name="([^"]*)"`)
)

// pgSeed — n позиций kind ('product'|'service') в новой категории; имена «<prefix> 001…».
func (s *CatalogLiveSuite) pgSeed(kind, prefix string, n int) string {
	ctx := context.Background()
	var catID string
	s.Require().NoError(s.Pool.QueryRow(ctx,
		`INSERT INTO catalog_categories (kind, name) VALUES ($1, $2 || ' ' || gen_random_uuid()) RETURNING id`,
		kind, prefix).Scan(&catID))
	table := "products"
	if kind == "service" {
		table = "services"
	}
	_, err := s.Pool.Exec(ctx, `INSERT INTO `+table+` (category_id, name)
		SELECT $1::uuid, $2 || ' ' || lpad(g::text, 3, '0') FROM generate_series(1, $3::int) g`, catID, prefix, n)
	s.Require().NoError(err)
	return catID
}

func (s *CatalogLiveSuite) pgPage(path string) string {
	st, raw := s.admin.Do(http.MethodGet, path, nil)
	s.Require().Equal(http.StatusOK, st, path)
	return string(raw)
}

func pgHref(re *regexp.Regexp, page string) (*url.URL, bool) {
	m := re.FindStringSubmatch(page)
	if m == nil {
		return nil, false
	}
	u, err := url.Parse(html.UnescapeString(m[1]))
	if err != nil {
		return nil, false
	}
	return u, true
}

func pgRows(page string) []string {
	var out []string
	for _, m := range pgRowRe.FindAllStringSubmatch(page, -1) {
		out = append(out, html.UnescapeString(m[1]))
	}
	return out
}

// TS-005: /catalog/products?offset=100 при >100 товарах → 200, пейджер «Показаны 101–…»,
// ссылка назад; offset=-5/abc → как 0.
func (s *CatalogLiveSuite) TestCatalogLiveSuite_SSRProductsPager_TS005() {
	cat := s.pgSeed("product", "TS005 Товар", 150)
	base := "/catalog/products?category=" + cat

	first := s.pgPage(base)
	s.Contains(first, `data-testid="catalog-pager"`)
	s.Equal("Показаны 1–100 из 150", pgShownRe.FindStringSubmatch(first)[1])
	rows := pgRows(first)
	s.Require().Len(rows, 100)
	s.Equal("TS005 Товар 001", rows[0])
	_, hasPrev := pgHref(pgPrevRe, first)
	s.False(hasPrev, "на первой странице «Назад» неактивна")
	next, ok := pgHref(pgNextRe, first)
	s.Require().True(ok)
	s.Equal("100", next.Query().Get("offset"))
	s.Equal(cat, next.Query().Get("category"))
	s.NotContains(first, `data-testid="categories-truncated"`, "DS-006: категорий меньше лимита — пометки нет")

	second := s.pgPage(base + "&offset=100")
	s.Contains(second, "Показаны 101–150 из 150")
	rows = pgRows(second)
	s.Require().Len(rows, 50)
	s.Equal("TS005 Товар 101", rows[0])
	prev, ok := pgHref(pgPrevRe, second)
	s.Require().True(ok, "ссылка назад")
	s.Equal("", prev.Query().Get("offset"), "назад на первую страницу — без offset")
	_, hasNext := pgHref(pgNextRe, second)
	s.False(hasNext)

	for _, bad := range []string{"-5", "abc"} {
		p := s.pgPage(base + "&offset=" + url.QueryEscape(bad))
		s.Contains(p, "Показаны 1–100 из 150", "offset=%s → как 0", bad)
	}

	// за пределами total → пустая таблица + «Назад» (не 404).
	beyond := s.pgPage(base + "&offset=1000")
	s.Empty(pgRows(beyond))
	back, ok := pgHref(pgPrevRe, beyond)
	s.Require().True(ok)
	s.Equal("100", back.Query().Get("offset"), "назад — на последнюю непустую страницу")
}

// TS-006: /catalog/services?offset=… — то же для услуг, фильтры q/category/archived
// сохраняются в ссылках пейджера.
func (s *CatalogLiveSuite) TestCatalogLiveSuite_SSRServicesPager_TS006() {
	cat := s.pgSeed("service", "TS006 Услуга", 230)
	q := "TS006 Услуга"
	base := "/catalog/services?category=" + cat + "&archived=1&q=" + url.QueryEscape(q)

	first := s.pgPage(base)
	s.Contains(first, "Показаны 1–100 из 230")
	s.Len(pgRows(first), 100)
	next, ok := pgHref(pgNextRe, first)
	s.Require().True(ok)
	s.Equal("/catalog/services", next.Path)
	s.Equal("100", next.Query().Get("offset"))
	s.Equal(cat, next.Query().Get("category"))
	s.Equal("1", next.Query().Get("archived"))
	s.Equal(q, next.Query().Get("q"))

	lastURL := *next
	v := lastURL.Query()
	v.Set("offset", "200")
	lastURL.RawQuery = v.Encode()
	page := s.pgPage(lastURL.String())
	s.Contains(page, "Показаны 201–230 из 230")
	rows := pgRows(page)
	s.Require().Len(rows, 30)
	s.True(strings.HasPrefix(rows[0], "TS006 Услуга 201"), rows[0])
	prev, ok := pgHref(pgPrevRe, page)
	s.Require().True(ok)
	s.Equal("100", prev.Query().Get("offset"))
	s.Equal(cat, prev.Query().Get("category"))
	s.Equal("1", prev.Query().Get("archived"))
	s.Equal(q, prev.Query().Get("q"))

	// некратный offset округляется вниз до начала страницы.
	v.Set("offset", strconv.Itoa(150))
	lastURL.RawQuery = v.Encode()
	s.Contains(s.pgPage(lastURL.String()), "Показаны 101–200 из 230")
}
