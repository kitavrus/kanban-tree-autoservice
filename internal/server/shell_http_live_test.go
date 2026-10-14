//go:build integration

package server_test

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/kitavrus/kanban-tree/pkg/testutil"
)

// ShellLiveSuite (§TD-8): боевой бинарь + реальная сеть — SSR-каркас фазы 1:
// общий layout, раздача общей статики и закрытость каталога шаблонов.
type ShellLiveSuite struct {
	testutil.LiveSuite
}

func TestShellLive(t *testing.T) {
	suite.Run(t, &ShellLiveSuite{LiveSuite: testutil.LiveSuite{BinaryPkg: "./cmd/server"}})
}

func (s *ShellLiveSuite) SetupSuite() {
	s.LiveSuite.SetupSuite()
	s.SeedUser("shell-admin", "password1", "Админ Шелл", "admin", true)
	s.LoginAs("shell-admin", "password1")
}

func (s *ShellLiveSuite) get(path string) (*http.Response, string) {
	resp, err := s.Client.Get(s.BaseURL + path)
	s.Require().NoError(err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	s.Require().NoError(err)
	return resp, string(body)
}

// TestHomeRendersLayout — «/» отдаёт доску заказов (ADR-100, фаза 5 сняла home с корня).
func (s *ShellLiveSuite) TestHomeRendersLayout() {
	resp, body := s.get("/")
	s.Equal(http.StatusOK, resp.StatusCode)
	s.True(strings.HasPrefix(resp.Header.Get("Content-Type"), "text/html"), resp.Header.Get("Content-Type"))
	s.Contains(body, `data-testid="sidebar"`)
	s.Contains(body, `data-testid="board-root"`)
	s.Contains(body, `<meta name="csrf-token"`)
	s.Contains(body, `href="/assets/ssr-shared/app.css"`)
	s.Contains(body, `<span class="app-avatar">АШ</span>`, "вошедший пользователь в сайдбаре")
}

func (s *ShellLiveSuite) TestSharedAssetsServed() {
	cases := map[string]string{
		"/assets/ssr-shared/app.css":                                  "text/css",
		"/assets/ssr-shared/vendor/bootstrap/bootstrap.min.css":       "text/css",
		"/assets/ssr-shared/vendor/bootstrap/bootstrap.bundle.min.js": "javascript",
		"/assets/ssr-shared/js/theme.js":                              "javascript",
		"/assets/ssr-shared/js/api.js":                                "javascript",
		"/assets/ssr-shared/js/ui-dialog.js":                          "javascript",
		"/assets/ssr-shared/js/combobox.js":                           "javascript",
		"/assets/ssr-shared/favicon.svg":                              "image/svg+xml",
	}
	for path, ct := range cases {
		resp, body := s.get(path)
		s.Equal(http.StatusOK, resp.StatusCode, path)
		s.Contains(resp.Header.Get("Content-Type"), ct, path)
		s.NotEmpty(body, path)
		s.Equal("no-cache", resp.Header.Get("Cache-Control"), path)
	}
}

// TestSharedAssets_RevalidateNotModified — no-cache + Last-Modified: повторный запрос с
// If-Modified-Since получает 304 без тела (ревалидация дешёвая, свежий JS — сразу после
// обновления сервера).
func (s *ShellLiveSuite) TestSharedAssets_RevalidateNotModified() {
	resp, _ := s.get("/assets/ssr-shared/js/api.js")
	s.Require().Equal(http.StatusOK, resp.StatusCode)
	lm := resp.Header.Get("Last-Modified")
	s.Require().NotEmpty(lm)

	req, err := http.NewRequest(http.MethodGet, s.BaseURL+"/assets/ssr-shared/js/api.js", nil)
	s.Require().NoError(err)
	req.Header.Set("If-Modified-Since", lm)
	resp2, err := http.DefaultClient.Do(req)
	s.Require().NoError(err)
	resp2.Body.Close()
	s.Equal(http.StatusNotModified, resp2.StatusCode)
}

// getRaw шлёт путь байт-в-байт (URL.Opaque): Go-клиент не нормализует %XX, `//`, `..`.
func (s *ShellLiveSuite) getRaw(path string) (int, string) {
	req, err := http.NewRequest(http.MethodGet, s.BaseURL+"/", nil)
	s.Require().NoError(err)
	req.URL.Opaque = path
	resp, err := s.Client.Do(req)
	s.Require().NoError(err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	s.Require().NoError(err)
	return resp.StatusCode, string(body)
}

// Исходники шаблонов не отдаются ни прямым путём, ни обходами фильтра по URL
// (кодирование, двойной слэш, `..`, регистр) — шаблоны лежат вне корня статики.
func (s *ShellLiveSuite) TestTemplatesNotExposed() {
	for _, path := range []string{
		"/assets/ssr-shared/templates/layouts/base.html",
		"/assets/ssr-shared/%74emplates/layouts/base.html",
		"/assets/ssr-shared//templates/layouts/base.html",
		"/assets/ssr-shared/js/../templates/layouts/base.html",
		"/ASSETS/ssr-shared/templates/layouts/base.html",
		"/assets/ssr-shared/../_layout/templates/layouts/base.html",
		"/assets/ssr-shared/..%2f_layout/templates/layouts/base.html",
		"/assets/ssr-shared/%2e%2e/_layout/templates/layouts/base.html",
		"/assets/ssr-shared/../board/templates/pages/board.html",
	} {
		status, body := s.getRaw(path)
		s.NotContains(body, `{{ define`, path)
		s.NotEqual(http.StatusOK, status, path)
	}
}

func (s *ShellLiveSuite) TestUnknownAssetIs404() {
	resp, _ := s.get("/assets/ssr-shared/js/nope.js")
	s.Equal(http.StatusNotFound, resp.StatusCode)
}
