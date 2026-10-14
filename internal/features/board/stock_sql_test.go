package board

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// soft-delete-catalog DS-042: SQL отчёта «Остатки» собирается из шаблона — подстановки раскрыты
// полностью; условие видимости товара стоит и в парах, и в строках; принудительное включение
// товара ($2::uuid — ответ корректировки, обнулившей остаток удалённого товара) есть только в
// варианте Adjust и только для удалённого товара.
func TestStockPairsSQL(t *testing.T) {
	plain, forced := stockPairsSQL(""), stockPairsSQL("$2::uuid")
	for name, q := range map[string]string{"список": plain, "корректировка": forced} {
		assert.NotContains(t, q, "$VISIBLE", name)
		assert.NotContains(t, q, "$FORCE", name)
		assert.Equal(t, 2, strings.Count(q, "p.deleted_at IS NULL AND p.is_active"), "%s: условие видимости — в pairs и в rows", name)
		assert.Equal(t, 2, strings.Count(q, "x.stock <> 0"), "%s: удалённый товар — только с ненулевым остатком", name)
		assert.Contains(t, q, "p.deleted_at IS NOT NULL AS is_deleted", name)
		// soft-delete-warehouses-suppliers DS-049: условие видимости склада раскрыто; архивный либо
		// удалённый склад входит в отчёт только с ненулевым остатком товара отчёта; пара на таком
		// складе всегда 'ok' (в счётчики риска не входит).
		assert.NotContains(t, q, "$WH_VISIBLE", name)
		assert.Equal(t, 1, strings.Count(q, "ws.stock <> 0"), "%s: недействующий склад — только с ненулевым остатком", name)
		assert.Contains(t, q, "(w.is_active AND w.deleted_at IS NULL) AS wh_live", name)
		assert.Contains(t, q, "CASE WHEN NOT wh_live OR p_archived THEN 'ok'", name)
		// fix-loop 1: архивный товар входит в отчёт только с ненулевым остатком на удалённом складе.
		assert.NotContains(t, q, "$AFORCE", name)
		assert.Equal(t, 2, strings.Count(q, "sa.stock <> 0 AND sw.deleted_at IS NOT NULL"), "%s: архивный товар — в pairs и в rows", name)
		assert.Contains(t, q, "wp.deleted_at IS NOT NULL OR wp.is_active", name)
	}
	assert.Equal(t, plain, stockPairsCTE)
	assert.NotContains(t, plain, "p.id = $2::uuid")
	assert.Equal(t, 2, strings.Count(forced, "FROM product_stock x WHERE x.product_id = p.id AND x.stock <> 0) OR p.id = $2::uuid))"),
		"принудительное включение — внутри ветки «товар удалён», не для архивных действующих")
}

// fix-loop 1 части 6: принудительное включение архивного товара — только в варианте корректировки
// на удалённом складе; обычная корректировка архивного товара строку не возвращает (404, как раньше).
func TestStockPairsSQL_ForceArchivedOnlyForDeletedWarehouse(t *testing.T) {
	const forcedArchived = "sw.deleted_at IS NOT NULL) OR p.id = $2::uuid)"
	assert.Equal(t, 0, strings.Count(stockPairsSQLFor("$2::uuid", false), forcedArchived))
	assert.Equal(t, 2, strings.Count(stockPairsSQLFor("$2::uuid", true), forcedArchived))
	assert.Equal(t, stockPairsSQL("$2::uuid"), stockPairsSQLFor("$2::uuid", false))
}
