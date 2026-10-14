package board

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Статическая сверка списков enum'ов (design-go-layers.md §Enum*). Live-сверка с
// pg_get_constraintdef CHECK'ов БД — фаза 03, board_http_live_test.go.

func TestSubtaskStatuses_MatchExpectedSet(t *testing.T) {
	assert.Equal(t, []string{"new", "in_progress", "done"}, SubtaskStatuses)
}

// TestPrintKinds_Values — UT-003: PrintKinds = {intake, handover} (design-go-layers.md
// §Enum PrintKind, AC-015/AC-016).
func TestPrintKinds_Values(t *testing.T) {
	assert.Equal(t, []string{"intake", "handover"}, PrintKinds)
}

func TestPaidFilters_MatchExpectedSet(t *testing.T) {
	assert.Equal(t, []string{"all", "paid", "unpaid"}, PaidFilters)
}

func TestDueFilters_MatchExpectedSet(t *testing.T) {
	assert.Equal(t, []string{"all", "soon", "overdue"}, DueFilters)
}

func TestStockScopes_MatchExpectedSet(t *testing.T) {
	assert.Equal(t, []string{"risky", "all"}, StockScopes)
}

func TestStockStates_MatchExpectedSet(t *testing.T) {
	assert.Equal(t, []string{"out", "low", "will_low", "ok"}, StockStates)
}

func TestWarningKinds_MatchExpectedSet(t *testing.T) {
	assert.Equal(t, []string{"negative", "below_min"}, WarningKinds)
}
