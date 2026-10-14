package ssr

// NavItem — пункт сайдбара. Badge — счётчик риск-товаров у «Остатков» (ADR-004, AC-016);
// 0 — бейдж скрыт (base.html), не относится к остальным пунктам меню.
type NavItem struct {
	Title     string
	Icon      string
	URL       string
	AdminOnly bool
	Badge     int
}

// NavGroup — группа пунктов с подписью.
type NavGroup struct {
	Label string
	Items []NavItem
}

// menu — полный сайдбар приложения (эталон — prototype/index.html, plan.md §5).
// Пункты справочников и администрирования видит только admin (plan.md §2, D19).
var menu = []NavGroup{
	{Label: "Работа", Items: []NavItem{
		{Title: "Доска", Icon: "▦", URL: "/"},
		{Title: "Остатки", Icon: "📊", URL: "/stock"},
		{Title: "Выплаты", Icon: "💸", URL: "/executor-payouts"},
	}},
	{Label: "Справочники", Items: []NavItem{
		{Title: "Клиенты", Icon: "🏢", URL: "/clients", AdminOnly: true},
		{Title: "Каталог", Icon: "🧰", URL: "/catalog", AdminOnly: true},
		{Title: "Поставщики", Icon: "🚛", URL: "/suppliers", AdminOnly: true},
	}},
	{Label: "Администрирование", Items: []NavItem{
		{Title: "Сотрудники", Icon: "👥", URL: "/users", AdminOnly: true},
		{Title: "Удалённые", Icon: "🗑", URL: "/deleted", AdminOnly: true}, // soft-delete-core DS-011
	}},
}

// BuildMenu — сайдбар для пользователя: только реализованные разделы (enabled) и
// только доступные по роли; пустые группы выкидываются. badges — ADR-004 (BadgeSource),
// nil допустим (нет источника бейджей — все Badge=0, ShellBuilder.Build).
func BuildMenu(enabled map[string]bool, isAdmin bool, badges map[string]int) []NavGroup {
	var out []NavGroup
	for _, g := range menu {
		var items []NavItem
		for _, it := range g.Items {
			if !enabled[it.URL] || (it.AdminOnly && !isAdmin) {
				continue
			}
			it.Badge = badges[it.URL]
			items = append(items, it)
		}
		if len(items) > 0 {
			out = append(out, NavGroup{Label: g.Label, Items: items})
		}
	}
	return out
}
