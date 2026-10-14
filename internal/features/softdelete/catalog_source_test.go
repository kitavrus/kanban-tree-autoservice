package softdelete

import "testing"

// Подзаголовок строки удалённой категории (soft-delete-catalog §6.9): вид product — «товары»,
// service — «услуги».
func TestCategoryKindSubtitle(t *testing.T) {
	if got := CategoryKindSubtitle("product"); got != "товары" {
		t.Fatalf("product: ожидалось «товары», получено %q", got)
	}
	if got := CategoryKindSubtitle("service"); got != "услуги" {
		t.Fatalf("service: ожидалось «услуги», получено %q", got)
	}
}

// Источники каталога привязаны к своему виду и таблице: перепутанная пара отдала бы строки
// чужой вкладки (вид строки — из источника, не из запроса).
func TestCatalogSourcesKindAndTable(t *testing.T) {
	cases := []struct {
		src   *CatalogSource
		kind  Kind
		table string
	}{
		{NewProductsSource(nil), KindProducts, "products"},
		{NewServicesSource(nil), KindServices, "services"},
		{NewCatalogCategoriesSource(nil), KindCatalogCategories, "catalog_categories"},
		{NewEquipmentTypesSource(nil), KindEquipmentTypes, "equipment_types"},
		// часть 6 (soft-delete-warehouses-suppliers): склады и поставщики — та же форма строки.
		{NewWarehousesSource(nil), KindWarehouses, "warehouses"},
		{NewSuppliersSource(nil), KindSuppliers, "suppliers"},
	}
	for _, c := range cases {
		if c.src.kind != c.kind || c.src.table != c.table {
			t.Fatalf("источник %s: вид %q, таблица %q", c.kind, c.src.kind, c.src.table)
		}
	}
}

// Часть 6 (soft-delete-warehouses-suppliers §6.5): подзаголовок строки удалённого поставщика —
// контактное лицо, у склада — пустая строка; оба вида есть в реестре с ключами счётчиков.
func TestWarehousesSuppliersSources(t *testing.T) {
	if got := NewWarehousesSource(nil).subtitleSQL; got != "''" {
		t.Fatalf("склад: подзаголовок должен быть пустым, получено %q", got)
	}
	if got := NewSuppliersSource(nil).subtitleSQL; got != "r.contact_name" {
		t.Fatalf("поставщик: подзаголовок — контактное лицо, получено %q", got)
	}
	for _, k := range []Kind{KindWarehouses, KindSuppliers} {
		if !IsKind(string(k)) || k.CountKey() != string(k) {
			t.Fatalf("вид %s: нет в реестре либо неверный ключ счётчика %q", k, k.CountKey())
		}
	}
}
