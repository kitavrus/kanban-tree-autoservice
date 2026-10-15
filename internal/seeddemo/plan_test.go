package seeddemo

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// demoCapacity — вместимость по умолчанию (shop_settings.max_cars, миграция 0016).
const demoCapacity = 8

// TestDemoPlan_CalendarLoad — правила календаря загрузки (internal/features/calendar) на
// демо-плане, дни от сегодня: невыданная машина занимает место с начала по плановое
// окончание, с прошедшим окончанием — бессрочно; выданные — только в прошлом. В ближайшие
// 2 недели загрузка не выше вместимости, есть день «под завязку» и дни с 1–3 свободными.
func TestDemoPlan_CalendarLoad(t *testing.T) {
	var occupied [14]int
	for _, task := range demoTasks {
		if task.column == demoFinalColumn {
			continue // выдан в прошлом (validatePlan: finishedOffset ≤ −1)
		}
		end := task.startOffset + task.days - 1
		for d := range occupied {
			if d >= task.startOffset && (d <= end || end < 0) {
				occupied[d]++
			}
		}
	}
	full, tight := 0, 0
	for d, n := range occupied {
		require.LessOrEqual(t, n, demoCapacity, "день +%d перегружен", d)
		switch free := demoCapacity - n; {
		case free == 0:
			full++
		case free >= 1 && free <= 3:
			tight++
		}
	}
	assert.Positive(t, full, "есть день без свободных мест: %v", occupied)
	assert.GreaterOrEqual(t, tight, 3, "дни с 1–3 свободными местами: %v", occupied)
	assert.Equal(t, demoCapacity-1, occupied[0], "сегодня свободно одно место")
}

// TestDemoPlan_Mix — на доске есть записи на будущее (1–10 дней вперёд), 1–2 просроченных
// невыданных и 2–3 заказа с окончанием сегодня/завтра; у компаний с автопарком — несколько
// заказов (по одному на машину), часть машин клиентов — без заказов.
func TestDemoPlan_Mix(t *testing.T) {
	future, overdue, soon := 0, 0, 0
	perClient := map[int]int{}
	withOrder := map[string]bool{}
	for _, task := range demoTasks {
		perClient[task.clientIdx]++
		withOrder[task.car] = true
		if task.column == demoFinalColumn {
			continue
		}
		end := task.startOffset + task.days - 1
		switch {
		case task.startOffset >= 1 && task.startOffset <= 10:
			future++
		case end < 0:
			overdue++
		case end == 0 || end == 1:
			soon++
		}
	}
	assert.GreaterOrEqual(t, future, 3)
	assert.True(t, overdue >= 1 && overdue <= 2, "просроченных: %d", overdue)
	assert.True(t, soon >= 2 && soon <= 3, "горящих: %d", soon)

	fleets := 0
	for _, n := range perClient {
		if n > 1 {
			fleets++
		}
	}
	assert.GreaterOrEqual(t, fleets, 2, "компании с автопарком — по заказу на машину")
	idle := 0
	for _, e := range demoEquipment {
		if !withOrder[e.key] {
			idle++
		}
	}
	assert.Positive(t, idle, "есть машины клиентов без заказов")
}
