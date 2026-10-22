package ssr

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/valyala/fasthttp"
)

// Пути — от каталога пакета (go test запускается в internal/ssr). Общий layout тестируется
// через board-шаблоны (доска — фаза 5 — занимает "/", home удалён в этой фазе): "stock" —
// страница, чей content-блок не требует ничего, кроме .Shell (эталон прежнего home/index).
const (
	testSharedRoot = "../../" + SharedRoot
	testBoardRoot  = "../../web/ssr/board/templates"
)

type pageVM struct{ Shell Shell }

func render(t *testing.T, sh Shell) string {
	t.Helper()
	r, err := NewRenderer(testBoardRoot, testSharedRoot)
	require.NoError(t, err)
	var buf bytes.Buffer
	require.NoError(t, r.Render(&buf, "stock", "base", pageVM{Shell: sh}))
	return buf.String()
}

func TestRenderer_PageInSharedLayout(t *testing.T) {
	html := render(t, Shell{
		Title:      "Доска заказов",
		Menu:       BuildMenu(map[string]bool{"/": true}, false, nil),
		ActivePath: "/",
		CSRFToken:  "tok-123",
	})

	assert.Contains(t, html, "<title>Доска заказов — Kanban-tree</title>")
	assert.Contains(t, html, `<meta name="csrf-token" content="tok-123">`)
	assert.Contains(t, html, `data-testid="sidebar"`)
	assert.Contains(t, html, `data-testid="stock-root"`, "страница вставлена в блок content")
	assert.Contains(t, html, `class="app-nav-link active" aria-current="page">▦ Доска</a>`)
	for _, js := range []string{"theme.js", "api.js", "ui-dialog.js", "combobox.js", "bootstrap.bundle.min.js"} {
		assert.Contains(t, html, js)
	}
	// гость: блока пользователя и формы выхода нет
	assert.NotContains(t, html, `action="/logout"`)
}

func TestRenderer_UserBlock(t *testing.T) {
	html := render(t, Shell{Title: "x", User: Viewer{Name: "Сидоров Пётр", Role: "executor"}})
	assert.Contains(t, html, `<span class="app-avatar">СП</span>`)
	assert.Contains(t, html, "Исполнитель")
	assert.Contains(t, html, `action="/logout"`)

	html = render(t, Shell{Title: "x", User: Viewer{Name: "Админ", Role: RoleAdmin}, IsAdmin: true})
	assert.Contains(t, html, "Администратор")
}

func TestRenderer_EscapesData(t *testing.T) {
	html := render(t, Shell{Title: `<script>alert(1)</script>`, User: Viewer{Name: `<b>x</b>`}})
	assert.NotContains(t, html, "<script>alert(1)</script>")
	assert.Contains(t, html, "&lt;script&gt;alert(1)&lt;/script&gt;")
	assert.NotContains(t, html, "<b>x</b>")
}

func TestRenderer_Errors(t *testing.T) {
	r, err := NewRenderer(testBoardRoot, testSharedRoot)
	require.NoError(t, err)
	var buf bytes.Buffer
	assert.ErrorContains(t, r.Render(&buf, "nope", "base", nil), `нет страницы "nope"`)

	_, err = NewRenderer("../../web/ssr/does-not-exist")
	assert.ErrorContains(t, err, "не найдены шаблоны")

	// board (фаза 04) держит свой layout "print" (печатные акты) — конструктор без
	// shared-набора больше не падает пустотой layouts/, но "base" в этом наборе всё
	// равно нет (он живёт только в SharedRoot).
	r2, err := NewRenderer(testBoardRoot)
	require.NoError(t, err)
	var buf2 bytes.Buffer
	assert.Error(t, r2.Render(&buf2, "stock", "base", pageVM{}))
}

func TestBuildMenu(t *testing.T) {
	all := map[string]bool{"/": true, "/stock": true, "/clients": true, "/catalog": true, "/users": true}

	titles := func(groups []NavGroup) []string {
		var out []string
		for _, g := range groups {
			for _, it := range g.Items {
				out = append(out, g.Label+"/"+it.Title)
			}
		}
		return out
	}

	assert.Equal(t, []string{"Работа/Доска", "Работа/Остатки", "Справочники/Клиенты", "Справочники/Каталог", "Администрирование/Сотрудники"},
		titles(BuildMenu(all, true, nil)))
	assert.Equal(t, []string{"Работа/Доска", "Работа/Остатки"}, titles(BuildMenu(all, false, nil)),
		"исполнитель не видит справочники и сотрудников (D19); пустые группы выкинуты")
	assert.Equal(t, []string{"Работа/Доска"}, titles(BuildMenu(map[string]bool{"/": true}, true, nil)),
		"нереализованные разделы скрыты")
	assert.Empty(t, BuildMenu(nil, true, nil))

	var badge int
	for _, g := range BuildMenu(all, true, map[string]int{"/stock": 7}) {
		for _, it := range g.Items {
			if it.URL == "/stock" {
				badge = it.Badge
			}
		}
	}
	assert.Equal(t, 7, badge, "BuildMenu пробрасывает badges[url] в NavItem.Badge")
}

func TestShellBuilder_Build(t *testing.T) {
	app := fiber.New()
	fctx := &fasthttp.RequestCtx{}
	fctx.Request.SetRequestURI("/stock")
	c := app.AcquireCtx(fctx)
	defer app.ReleaseCtx(c)

	b := NewShellBuilder("/", "/stock", "/users")

	sh := b.Build(c, "Остатки")
	assert.Equal(t, "Остатки", sh.Title)
	assert.Equal(t, "/stock", sh.ActivePath)
	assert.False(t, sh.IsAdmin, "без viewer — гость")
	assert.Len(t, sh.Menu, 1, "гостю — только «Работа»")

	c.Locals(LocalViewer, Viewer{Name: "Админ", Role: RoleAdmin})
	c.Locals(LocalCSRFToken, "csrf-x")
	sh = b.Build(c, "Остатки")
	assert.True(t, sh.IsAdmin)
	assert.Equal(t, "csrf-x", sh.CSRFToken)
	assert.Len(t, sh.Menu, 2, "admin видит «Администрирование»")

	// TS-040 (user-timezone, DS-031/DS-033): пояс viewer'а доходит до layout; у viewer'а без
	// пояса (и у гостя) — умолчание Asia/Almaty, шаблон никогда не получает пустой пояс.
	assert.Equal(t, "Asia/Almaty", sh.User.Timezone)
	assert.Equal(t, "Asia/Almaty", ViewerTimezone(c))
	c.Locals(LocalViewer, Viewer{Name: "Админ", Role: RoleAdmin, Timezone: "Europe/Moscow"})
	sh = b.Build(c, "Остатки")
	assert.Equal(t, "Europe/Moscow", sh.User.Timezone)
	assert.Equal(t, "Europe/Moscow", ViewerTimezone(c))
}

// TS-040 (user-timezone): layout отдаёт пояс атрибутом <body data-timezone> и select'ом в
// сайдбаре (над «Тема»); гостю select не рисуется.
func TestTS040_Renderer_TimezoneInLayout(t *testing.T) {
	html := render(t, Shell{Title: "x", User: Viewer{Name: "Сидоров Пётр", Role: "executor", Timezone: "Europe/Moscow"}})
	assert.Contains(t, html, `<body data-timezone="Europe/Moscow">`)
	assert.Contains(t, html, `data-testid="timezone-select"`)
	assert.Contains(t, html, `<option value="Europe/Moscow" selected>Москва (UTC&#43;3)</option>`)
	assert.Contains(t, html, `<option value="Asia/Almaty">Алматы (UTC&#43;5)</option>`)
	assert.Contains(t, html, "timezone.js")
	assert.Less(t, strings.Index(html, `data-testid="timezone-select"`), strings.Index(html, `data-testid="theme-toggle"`),
		"select — над кнопкой «Тема»")
	assert.Less(t, strings.Index(html, "js/api.js"), strings.Index(html, "js/timezone.js"), "KT.time нужен KT.api")

	guest := render(t, Shell{Title: "x"})
	assert.NotContains(t, guest, `data-testid="timezone-select"`)
}

// TS-041 (user-timezone, DS-032): шаблонная функция timeIn — время в поясе пользователя.
func TestTS041_Funcs_TimeIn(t *testing.T) {
	fn, ok := Funcs["timeIn"].(func(string, string, any) string)
	require.True(t, ok, "timeIn зарегистрирована в ssr.Funcs")
	moment := time.Date(2026, 10, 6, 19, 30, 0, 0, time.UTC) // TS-F5
	assert.Equal(t, "07.10.2026 00:30", fn("Asia/Almaty", "02.01.2006 15:04", moment))
	assert.Equal(t, "06.10.2026 22:30", fn("Europe/Moscow", "02.01.2006 15:04", moment))
	assert.Equal(t, "06.10.2026", fn("Europe/Moscow", "02.01.2006", &moment))
}

func TestInitials(t *testing.T) {
	cases := map[string]string{
		"Сидоров Пётр Иванович": "СП",
		"admin":            "A",
		"  волков   олег ": "ВО",
		"":                 "",
		"Ерлан (АвтоЗапчасти Алматы)": "ЕА",
		"«Тұрақ» 24": "Т",
		"( )":        "",
	}
	for in, want := range cases {
		assert.Equal(t, want, Initials(in), in)
	}
	assert.True(t, strings.HasPrefix(Initials("ёжик"), "Ё"))
}
