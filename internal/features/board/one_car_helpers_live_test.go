//go:build integration

package board_test

import (
	"strconv"
	"strings"
	"sync/atomic"
)

// one-car-order: заказа без автомобиля нет — POST /api/v1/tasks требует plate_number и
// найденный (equipment_id) либо новый (new_equipment) автомобиль. Хелперы ниже дописывают в
// тело фикстуры новый автомобиль клиента client_id с уникальным госномером из одних букв (цифры
// в госномере совпадали бы с поиском доски по номеру заказа «#id»).

var testCarSeq atomic.Int64

// testPlate — уникальный в процессе госномер фикстуры: «Ю» + номер, записанный буквами.
func testPlate() string {
	const letters = "АБВГДЕЖЗИК"
	n := strconv.FormatInt(testCarSeq.Add(1), 10)
	var b strings.Builder
	b.WriteString("Ю")
	for _, d := range n {
		b.WriteString(string([]rune(letters)[d-'0']))
	}
	return b.String()
}

// withNewCar — копия тела POST /api/v1/tasks с новым автомобилем (plate_number + new_equipment),
// если тело ещё не задаёт автомобиль.
func withNewCar(body map[string]any) map[string]any {
	out := make(map[string]any, len(body)+2)
	for k, v := range body {
		out[k] = v
	}
	if _, ok := out["plate_number"]; !ok {
		out["plate_number"] = testPlate()
	}
	_, hasFound := out["equipment_id"]
	_, hasNew := out["new_equipment"]
	if !hasFound && !hasNew {
		out["new_equipment"] = map[string]any{}
	}
	return out
}
