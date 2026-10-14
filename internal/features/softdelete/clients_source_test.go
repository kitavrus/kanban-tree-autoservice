package softdelete

import "testing"

// Заголовок строки удалённого автомобиля — госномер (soft-delete-clients §6.5), крайние
// пробелы обрезаются.
func TestEquipmentTitle(t *testing.T) {
	cases := map[string]string{
		"А123ВС 777":     "А123ВС 777",
		" А123ВС 777 ":   "А123ВС 777",
		"":               "",
		"\tР126ОР 750\n": "Р126ОР 750",
	}
	for in, want := range cases {
		if got := EquipmentTitle(in); got != want {
			t.Fatalf("EquipmentTitle(%q) = %q, ожидалось %q", in, got, want)
		}
	}
}

// deleted_with (DS-015): "client" только у единицы, ушедшей вместе с клиентом; у удалённой
// отдельно — nil (JSON null), а не пустая строка.
func TestEquipmentDeletedWith(t *testing.T) {
	if got := EquipmentDeletedWith(false); got != nil {
		t.Fatalf("удалена отдельно: ожидался nil, получено %q", *got)
	}
	got := EquipmentDeletedWith(true)
	if got == nil || *got != "client" {
		t.Fatalf("ушла с клиентом: ожидалось \"client\", получено %v", got)
	}
}
