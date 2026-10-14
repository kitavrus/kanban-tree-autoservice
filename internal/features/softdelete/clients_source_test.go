package softdelete

import "testing"

// Заголовок строки удалённой единицы — «тип марка модель» (soft-delete-clients §6.5): пустая
// марка пропускается без двойного пробела, крайние пробелы частей обрезаются.
func TestEquipmentTitle(t *testing.T) {
	cases := []struct {
		name, typeName, brand, model, want string
	}{
		{"все части", "Принтер", "HP", "LaserJet 1020", "Принтер HP LaserJet 1020"},
		{"без марки", "Принтер", "", "LaserJet 1020", "Принтер LaserJet 1020"},
		{"марка из пробелов", "Принтер", "   ", "LaserJet", "Принтер LaserJet"},
		{"пробелы по краям", " Ноутбук ", " Dell ", " XPS ", "Ноутбук Dell XPS"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := EquipmentTitle(c.typeName, c.brand, c.model); got != c.want {
				t.Fatalf("EquipmentTitle(%q, %q, %q) = %q, ожидалось %q", c.typeName, c.brand, c.model, got, c.want)
			}
		})
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
