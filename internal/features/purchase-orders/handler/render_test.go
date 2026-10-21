package handler

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kitavrus/kanban-tree/internal/config"
	"github.com/kitavrus/kanban-tree/internal/features/purchase-orders/models"
	"github.com/kitavrus/kanban-tree/internal/features/purchase-orders/repository"
	"github.com/kitavrus/kanban-tree/internal/ssr"
)

// chdirRoot — шаблоны адресуются от корня модуля (как у бинаря).
func chdirRoot(t *testing.T) {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(dir)
		require.NotEqual(t, parent, dir, "go.mod не найден")
		dir = parent
	}
	t.Chdir(dir)
}

// Экраны §8.1–§8.3, §8.6–§8.7 собираются в общем layout проекта (сайдбар/шапка — base из
// web/ssr/_layout), своих layout'ов у фичи нет.
func TestPagesRenderInSharedShell(t *testing.T) {
	chdirRoot(t)
	r, err := ssr.NewRenderer(Root, ssr.SharedRoot)
	require.NoError(t, err)
	shell := ssr.Shell{Title: "Закупки", User: ssr.Viewer{Name: "Админ", Role: "admin", Timezone: "Asia/Almaty"}, IsAdmin: true,
		Menu: ssr.BuildMenu(map[string]bool{"/purchases": true}, true, nil)}
	pages := map[string]any{
		"list":          ListVM{Shell: shell, Statuses: statusOptions([]string{"draft", "sent"})},
		"new":           NewVM{Shell: shell, Suppliers: []repository.SupplierOption{{ID: "1", Name: "АвтоДеталь"}}, Bazaar: "Базар"},
		"view":          ViewVM{Shell: shell, ID: "x", IsAdmin: true},
		"supplier-list": ListVM{Shell: ssr.Shell{Menu: ssr.SupplierMenu(), IsSupplier: true}, FirmName: "Мастерская", FirmPhone: "+7"},
		"supplier-view": ViewVM{Shell: ssr.Shell{Menu: ssr.SupplierMenu(), IsSupplier: true}, ID: "x"},
	}
	for name, vm := range pages {
		var buf bytes.Buffer
		require.NoError(t, r.Render(&buf, name, "base", vm), name)
		html := buf.String()
		assert.Contains(t, html, `data-testid="sidebar"`, name)
		assert.Contains(t, html, "/assets/ssr-shared/js/", name)
	}
	var buf bytes.Buffer
	require.NoError(t, r.Render(&buf, "supplier-view", "base", ViewVM{Shell: ssr.Shell{Menu: ssr.SupplierMenu(), IsSupplier: true, User: ssr.Viewer{Name: "Ерлан", Role: "supplier"}}}))
	assert.Contains(t, buf.String(), `data-testid="so-not-found"`)
	assert.Contains(t, buf.String(), "Поставщик", "подпись роли в сайдбаре")
}

func sp(s string) *string { return &s }

// PDF §8.4: Базар — колонка ☐, после ответа поставщика — цены и итог; экземпляр поставщика.
func TestRenderOrderPDF(t *testing.T) {
	sup := "s"
	now := time.Now()
	o := &models.PurchaseOrder{Number: 13, Status: "draft", Note: "до 17:00", CreatedAt: now,
		Items: []models.PurchaseOrderItem{{Name: "Фильтр масляный", Sku: "OC-90", Qty: "4.000", Unit: "шт"}}}
	firm := config.FirmInfo{Name: "СТО", Phone: "+7 700"}
	for _, mode := range []pdfMode{pdfOurs, pdfSupplier} {
		body, err := renderOrderPDF(o, mode, firm, "Asia/Almaty")
		require.NoError(t, err)
		assert.True(t, bytes.HasPrefix(body, []byte("%PDF")))
	}
	o.SupplierID = &sup
	o.AnsweredCount = 1
	o.TotalAmount = sp("9800.00")
	o.Items[0].SupplierQty, o.Items[0].SupplierPrice, o.Items[0].LineAmount = sp("4.000"), sp("2450.00"), sp("9800.00")
	body, err := renderOrderPDF(o, pdfOurs, firm, "Asia/Almaty")
	require.NoError(t, err)
	assert.True(t, bytes.HasPrefix(body, []byte("%PDF")))
}
