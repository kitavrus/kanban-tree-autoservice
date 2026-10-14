package catalog

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func validProductInput() ProductInput {
	return ProductInput{
		CategoryID: uuid.Must(uuid.NewV7()).String(), Name: "Фильтр масляный", SKU: "F-001",
		Unit: "шт", Price: "199.90", Stock: "5", MinStock: "2", Box: "К-07", Place: "Стеллаж A",
	}
}

func TestProductInput_Normalize(t *testing.T) {
	in := ProductInput{
		CategoryID: "  id  ", Name: "  Фильтр  ", SKU: " F1 ", Unit: " шт ",
		Price: "  ", Stock: " 5 ", MinStock: "", Box: " К1 ", Place: " Полка ",
	}.normalize()
	assert.Equal(t, "id", in.CategoryID)
	assert.Equal(t, "Фильтр", in.Name)
	assert.Equal(t, "F1", in.SKU)
	assert.Equal(t, "шт", in.Unit)
	assert.Equal(t, "0", in.Price, "пустая цена нормализуется в 0")
	assert.Equal(t, "5", in.Stock)
	assert.Equal(t, "0", in.MinStock)
	assert.Equal(t, "К1", in.Box)
	assert.Equal(t, "Полка", in.Place)
}

func TestProductInput_Validate_HappyPath(t *testing.T) {
	assert.Empty(t, fieldsOf(validProductInput().normalize().validate()))
}

func TestProductInput_Validate_CategoryID(t *testing.T) {
	in := validProductInput()
	in.CategoryID = ""
	assert.Equal(t, map[string]string{"category_id": msgRequired}, fieldsOf(in.normalize().validate()))

	in = validProductInput()
	in.CategoryID = "not-a-uuid"
	assert.Equal(t, map[string]string{"category_id": msgInvalid}, fieldsOf(in.normalize().validate()))
}

func TestProductInput_Validate_Name(t *testing.T) {
	in := validProductInput()
	in.Name = ""
	assert.Equal(t, map[string]string{"name": msgRequired}, fieldsOf(in.normalize().validate()))

	in = validProductInput()
	in.Name = strings.Repeat("я", 200)
	assert.Empty(t, fieldsOf(in.normalize().validate()), "200 рун — граница")

	in = validProductInput()
	in.Name = strings.Repeat("я", 201)
	assert.Equal(t, map[string]string{"name": msgTooLong}, fieldsOf(in.normalize().validate()))

	in = validProductInput()
	in.Name = "Товар\u0000"
	assert.Equal(t, map[string]string{"name": msgInvalidText}, fieldsOf(in.normalize().validate()))
}

func TestProductInput_Validate_Unit(t *testing.T) {
	in := validProductInput()
	in.Unit = ""
	assert.Empty(t, fieldsOf(in.normalize().validate()), "пустая единица допустима")

	in = validProductInput()
	in.Unit = "тонна"
	assert.Equal(t, map[string]string{"unit": msgInvalid}, fieldsOf(in.normalize().validate()))

	for _, u := range Units {
		in := validProductInput()
		in.Unit = u
		assert.Empty(t, fieldsOf(in.normalize().validate()), "единица %q из списка — ок", u)
	}
}

func TestProductInput_Validate_Amounts(t *testing.T) {
	cases := []struct {
		name  string
		mut   func(*ProductInput)
		field string
		code  string
	}{
		{"цена отрицательная", func(in *ProductInput) { in.Price = "-1" }, "price", msgInvalid},
		{"цена мусор", func(in *ProductInput) { in.Price = "abc" }, "price", msgInvalidFormat},
		{"цена экспонента — отклоняется", func(in *ProductInput) { in.Price = "1e5" }, "price", msgInvalidFormat},
		{"цена NaN — отклоняется", func(in *ProductInput) { in.Price = "NaN" }, "price", msgInvalidFormat},
		{"минимум отрицательный", func(in *ProductInput) { in.MinStock = "-0.5" }, "min_stock", msgInvalid},
		{"остаток мусор", func(in *ProductInput) { in.Stock = "12.34.56" }, "stock", msgInvalidFormat},
		// code review Блокер 1 — границы NUMERIC(12,2)/(12,3): переполнение и лишняя
		// точность → 400 по СВОЕМУ полю, не молчаливое округление/500.
		{"цена переполнение 1e10 форматом", func(in *ProductInput) { in.Price = "1e10" }, "price", msgInvalidFormat},
		{"цена переполнение целой части", func(in *ProductInput) { in.Price = "12345678901" }, "price", msgTooLarge},
		{"цена лишняя точность", func(in *ProductInput) { in.Price = "1.239" }, "price", msgTooPrecise},
		{"остаток лишняя точность", func(in *ProductInput) { in.Stock = "0.0004" }, "stock", msgTooPrecise},
		{"остаток переполнение целой части", func(in *ProductInput) { in.Stock = "1000000000" }, "stock", msgTooLarge},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := validProductInput()
			tc.mut(&in)
			got := fieldsOf(in.normalize().validate())
			assert.Equal(t, tc.code, got[tc.field], "%v", got)
		})
	}
}

// D44/task.md: остаток товара может быть отрицательным (долг) — не ошибка.
func TestProductInput_Validate_NegativeStockAllowed(t *testing.T) {
	in := validProductInput()
	in.Stock = "-3.5"
	assert.Empty(t, fieldsOf(in.normalize().validate()))
}

func TestProductInput_Validate_SkuBoxPlaceTooLong(t *testing.T) {
	in := validProductInput()
	in.SKU = strings.Repeat("a", 101)
	assert.Equal(t, map[string]string{"sku": msgTooLong}, fieldsOf(in.normalize().validate()))

	in = validProductInput()
	in.Box = strings.Repeat("a", 101)
	assert.Equal(t, map[string]string{"box": msgTooLong}, fieldsOf(in.normalize().validate()))

	in = validProductInput()
	in.Place = strings.Repeat("a", 101)
	assert.Equal(t, map[string]string{"place": msgTooLong}, fieldsOf(in.normalize().validate()))
}
