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
		// часть 6 (soft-delete-suppliers): поставщики — та же форма строки.
		{NewSuppliersSource(nil), KindSuppliers, "suppliers"},
	}
	for _, c := range cases {
		if c.src.kind != c.kind || c.src.table != c.table {
			t.Fatalf("источник %s: вид %q, таблица %q", c.kind, c.src.kind, c.src.table)
		}
	}
}

// Часть 6 (soft-delete-suppliers §6.5): подзаголовок строки удалённого поставщика — контактное
// лицо; вид есть в реестре с ключом счётчика.
func TestSuppliersSource(t *testing.T) {
	if got := NewSuppliersSource(nil).subtitleSQL; got != "r.contact_name" {
		t.Fatalf("поставщик: подзаголовок — контактное лицо, получено %q", got)
	}
	if !IsKind(string(KindSuppliers)) || KindSuppliers.CountKey() != string(KindSuppliers) {
		t.Fatalf("вид %s: нет в реестре либо неверный ключ счётчика %q", KindSuppliers, KindSuppliers.CountKey())
	}
}
