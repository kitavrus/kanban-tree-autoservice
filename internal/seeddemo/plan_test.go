package seeddemo

import (
	"regexp"
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

// TestValidatePlan_Acceptance — приёмка проверяется для каждого заказа напрямую, не только
// через журнал и фото: начатый заказ заведён не позже дня начала; выданный — раньше выдачи.
func TestValidatePlan_Acceptance(t *testing.T) {
	for _, task := range demoTasks {
		require.NoError(t, validatePlan(task), task.title)
	}
	bare := func(tk demoTaskItem) demoTaskItem { tk.subtasks, tk.media, tk.descHTML = nil, nil, ""; return tk }

	started := bare(demoTaskItem{column: "В ремонте", startOffset: -3, days: 4, acceptedH: 72})
	require.NoError(t, validatePlan(started))
	started.acceptedH = 71 // заведён в день после начала при прогоне в 23:59
	require.ErrorContains(t, validatePlan(started), "позже дня начала")

	booked := bare(demoTaskItem{column: "Приёмка", startOffset: 3, days: 1, acceptedH: 1})
	require.NoError(t, validatePlan(booked), "запись на будущее — заведена сегодня")

	issued := bare(demoTaskItem{column: demoFinalColumn, startOffset: -3, days: 2, acceptedH: 72, finishedOffset: -2})
	require.NoError(t, validatePlan(issued))
	issued.startOffset, issued.acceptedH = -2, 50 // приёмка 50 ч назад, выдача — не ближе 55 ч назад
	require.ErrorContains(t, validatePlan(issued), "раньше выдачи")
}

// relativeDate — относительные даты и дни недели, которые расходились бы с планом в
// зависимости от дня и часа прогона.
var relativeDate = regexp.MustCompile(`(?i)завтра|вчера|сегодня|через (\d+|два|три|пару) дн|понедельник|вторник|сред[уы]|четверг|пятниц|суббот|воскресень|вечером`)

// TestDemoTexts_PlanDates — в текстах журнала и описаниях даты — только из плана заказа
// (плейсхолдеры {start}/{end}/{fin} → «16.10 (чт)»), без относительных «завтра/в пятницу».
func TestDemoTexts_PlanDates(t *testing.T) {
	placeholder := regexp.MustCompile(`\{[a-z]+\}`)
	used := 0
	for _, task := range demoTasks {
		texts := []string{task.descHTML}
		for _, st := range task.subtasks {
			for _, l := range st.logs {
				texts = append(texts, l.text)
			}
		}
		for _, s := range texts {
			assert.NotRegexp(t, relativeDate, s, task.title)
			if placeholder.MatchString(s) {
				used++
			}
			out := planText(s, task)
			assert.NotRegexp(t, placeholder, out, "плейсхолдер не заменён: %s", task.title)
		}
	}
	assert.GreaterOrEqual(t, used, 10, "даты в текстах — из плана")

	tk := demoTaskItem{startOffset: 2, days: 3, finishedOffset: -1}
	assert.Equal(t, "с "+dayText(2)+" по "+dayText(4)+", выдан "+dayText(-1), planText("с {start} по {end}, выдан {fin}", tk))
	assert.Regexp(t, `^\d{2}\.\d{2} \((пн|вт|ср|чт|пт|сб|вс)\)$`, dayText(0))
}
