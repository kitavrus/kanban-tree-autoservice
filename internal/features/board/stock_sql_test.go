package board

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// soft-delete-catalog DS-042: SQL отчёта «Остатки» собирается из шаблона — подстановка раскрыта;
// удалённый товар входит в отчёт только с ненулевым остатком; принудительное включение товара
// ($1::uuid — ответ корректировки, обнулившей остаток удалённого товара) есть только в варианте
// Adjust и только внутри ветки «товар удалён».
func TestStockPendingSQL(t *testing.T) {
	plain, forced := stockPendingSQL(""), stockPendingSQL("$1::uuid")
	for name, q := range map[string]string{"список": plain, "корректировка": forced} {
		assert.NotContains(t, q, "$FORCE", name)
		assert.Contains(t, q, "(p.deleted_at IS NULL AND p.is_active)", name)
		assert.Contains(t, q, "p.deleted_at IS NOT NULL AS is_deleted", name)
		assert.Contains(t, q, "t.deleted_at IS NULL", "%s: потребность удалённых заказов не учитывается", name)
	}
	assert.Equal(t, plain, stockPendingCTE)
	assert.Contains(t, plain, "(p.deleted_at IS NOT NULL AND (p.stock <> 0))")
	assert.NotContains(t, plain, "p.id = $1::uuid")
	assert.Equal(t, 1, strings.Count(forced, "(p.deleted_at IS NOT NULL AND (p.stock <> 0 OR p.id = $1::uuid))"),
		"принудительное включение — внутри ветки «товар удалён», не для архивных действующих")
}
