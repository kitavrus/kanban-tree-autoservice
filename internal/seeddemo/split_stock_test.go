package seeddemo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSplitStock — DS-012/D60: остаток ≥20 делится ¼ в Москву (вниз), меньше/нецелый —
// целиком на «Алматы».
func TestSplitStock(t *testing.T) {
	cases := []struct{ in, almaty, moscow string }{
		{"150", "113", "37"},
		{"20", "15", "5"},
		{"19", "19", "0"},
		{"0", "0", "0"},
		{"2.5", "2.5", "0"},
	}
	for _, c := range cases {
		a, m := splitStock(c.in)
		assert.Equal(t, c.almaty, a, c.in)
		assert.Equal(t, c.moscow, m, c.in)
	}
}

// TestDemoPlacement_Consistent — заказы «Москвы»/«в пути» существуют в demoTasks, у них
// есть оборудование, и заказ «в пути» не на «Москве» (отправка идёт Алматы → Москва).
func TestDemoPlacement_Consistent(t *testing.T) {
	for i := range demoMoscowTasks {
		if assert.Less(t, i, len(demoTasks)) {
			assert.NotEmpty(t, demoTasks[i].equipment, "заказ %d", i)
		}
	}
	if assert.Less(t, demoTransitTask, len(demoTasks)) {
		assert.NotEmpty(t, demoTasks[demoTransitTask].equipment)
	}
	assert.False(t, demoMoscowTasks[demoTransitTask])
}
