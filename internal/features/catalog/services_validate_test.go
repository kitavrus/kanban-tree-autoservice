package catalog

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func validServiceInput() ServiceInput {
	return ServiceInput{
		CategoryID: uuid.Must(uuid.NewV7()).String(), Name: "Замена двигателя",
		HasPrice: true, Price: "1500",
	}
}

func TestServiceInput_Normalize_TrimsAndPrice(t *testing.T) {
	in := ServiceInput{CategoryID: "  id  ", Name: "  Услуга  ", HasPrice: true, Price: "  "}.normalize()
	assert.Equal(t, "id", in.CategoryID)
	assert.Equal(t, "Услуга", in.Name)
	assert.False(t, in.HasPrice, "пустая строка цены после trim — «по факту»")

	in2 := ServiceInput{CategoryID: "id", Name: "X", HasPrice: true, Price: "  1500.50  "}.normalize()
	assert.True(t, in2.HasPrice)
	assert.Equal(t, "1500.50", in2.Price)
}

// code review Medium#3: normalize() больше НЕ дедуплицирует комплект — только trim и
// отброс пустых строк; дубли объединяет отдельный шаг mergeProducts(), вызываемый только
// после успешного validate() (см. TestMergeProducts_* ниже).
func TestServiceInput_Normalize_KeepsDuplicateRows(t *testing.T) {
	p1 := uuid.Must(uuid.NewV7()).String()
	p2 := uuid.Must(uuid.NewV7()).String()
	in := ServiceInput{
		CategoryID: "c", Name: "X",
		Products: []ServiceProductInput{
			{ProductID: p1, Qty: "1.5"},
			{ProductID: p2, Qty: "2"},
			{ProductID: p1, Qty: "0.5"},
		},
	}.normalize()
	assert.Len(t, in.Products, 3, "дедуп — отдельный шаг, не normalize()")
	assert.Equal(t, p1, in.Products[0].ProductID)
	assert.Equal(t, "1.5", in.Products[0].Qty)
	assert.Equal(t, p1, in.Products[2].ProductID)
	assert.Equal(t, "0.5", in.Products[2].Qty)
}

// Пустая строка комплекта (row-list добавил строку, но товар не выбран) отбрасывается.
func TestServiceInput_Normalize_DropsEmptyRow(t *testing.T) {
	in := ServiceInput{CategoryID: "c", Name: "X", Products: []ServiceProductInput{{ProductID: "", Qty: ""}}}.normalize()
	assert.Empty(t, in.Products)
}

func TestServiceInput_Validate_HappyPath(t *testing.T) {
	assert.Empty(t, fieldsOf(validServiceInput().normalize().validate()))
}

// «По факту» (без цены) — валидная услуга.
func TestServiceInput_Validate_NoPriceIsFine(t *testing.T) {
	in := validServiceInput()
	in.HasPrice = false
	assert.Empty(t, fieldsOf(in.normalize().validate()))
}

func TestServiceInput_Validate_CategoryID(t *testing.T) {
	in := validServiceInput()
	in.CategoryID = ""
	assert.Equal(t, map[string]string{"category_id": msgRequired}, fieldsOf(in.normalize().validate()))

	in = validServiceInput()
	in.CategoryID = "not-a-uuid"
	assert.Equal(t, map[string]string{"category_id": msgInvalid}, fieldsOf(in.normalize().validate()))
}

func TestServiceInput_Validate_Name(t *testing.T) {
	in := validServiceInput()
	in.Name = ""
	assert.Equal(t, map[string]string{"name": msgRequired}, fieldsOf(in.normalize().validate()))

	in = validServiceInput()
	in.Name = strings.Repeat("я", 200)
	assert.Empty(t, fieldsOf(in.normalize().validate()), "200 рун — граница")

	in = validServiceInput()
	in.Name = strings.Repeat("я", 201)
	assert.Equal(t, map[string]string{"name": msgTooLong}, fieldsOf(in.normalize().validate()))

	in = validServiceInput()
	in.Name = "Услуга\u0000"
	assert.Equal(t, map[string]string{"name": msgInvalidText}, fieldsOf(in.normalize().validate()))
}

// code review Блокер 1: границы NUMERIC(12,2) для цены — переполнение/лишняя точность →
// 400 по СВОЕМУ полю, не молчаливое округление (1.239→1.24) и не 500 (22003).
func TestServiceInput_Validate_Price(t *testing.T) {
	cases := []struct {
		name  string
		price string
		code  string
	}{
		{"отрицательная", "-1", msgInvalid},
		{"мусор", "abc", msgInvalidFormat},
		{"экспонента", "1e5", msgInvalidFormat},
		{"переполнение целой части", "12345678901", msgTooLarge},
		{"переполнение форматом", "1e10", msgInvalidFormat},
		{"лишняя точность", "1.239", msgTooPrecise},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := validServiceInput()
			in.Price = tc.price
			assert.Equal(t, map[string]string{"price": tc.code}, fieldsOf(in.normalize().validate()))
		})
	}

	in := validServiceInput()
	in.Price = "0"
	assert.Empty(t, fieldsOf(in.normalize().validate()), "цена 0 допустима (≥0)")
}

// qty ≤ 0 → 400 (task.md); мусор в product_id → 400 по конкретной строке; границы
// NUMERIC(12,3) для qty — code review Блокер 1.
func TestServiceInput_Validate_ProductsRows(t *testing.T) {
	pid := uuid.Must(uuid.NewV7()).String()

	in := validServiceInput()
	in.Products = []ServiceProductInput{{ProductID: pid, Qty: "0"}}
	assert.Equal(t, map[string]string{"products[0].qty": msgInvalid}, fieldsOf(in.normalize().validate()))

	in = validServiceInput()
	in.Products = []ServiceProductInput{{ProductID: pid, Qty: "-1"}}
	assert.Equal(t, map[string]string{"products[0].qty": msgInvalid}, fieldsOf(in.normalize().validate()))

	in = validServiceInput()
	in.Products = []ServiceProductInput{{ProductID: "not-a-uuid", Qty: "1"}}
	assert.Equal(t, map[string]string{"products[0].product_id": msgInvalid}, fieldsOf(in.normalize().validate()))

	in = validServiceInput()
	in.Products = []ServiceProductInput{{ProductID: pid, Qty: "abc"}}
	assert.Equal(t, map[string]string{"products[0].qty": msgInvalidFormat}, fieldsOf(in.normalize().validate()))

	in = validServiceInput()
	in.Products = []ServiceProductInput{{ProductID: pid, Qty: "0.0001"}}
	assert.Equal(t, map[string]string{"products[0].qty": msgTooPrecise}, fieldsOf(in.normalize().validate()))

	in = validServiceInput()
	in.Products = []ServiceProductInput{{ProductID: pid, Qty: "1000000000"}}
	assert.Equal(t, map[string]string{"products[0].qty": msgTooLarge}, fieldsOf(in.normalize().validate()))

	// пустой комплект допустим (task.md).
	in = validServiceInput()
	assert.Empty(t, fieldsOf(in.normalize().validate()))

	// две валидные строки — ок, индексы ошибок считаются по позиции в списке ДО
	// дедупликации (code review Medium#3 — форма перерисовывает именно этот список).
	pid2 := uuid.Must(uuid.NewV7()).String()
	in = validServiceInput()
	in.Products = []ServiceProductInput{{ProductID: pid, Qty: "1"}, {ProductID: pid2, Qty: "0"}}
	assert.Equal(t, map[string]string{"products[1].qty": msgInvalid}, fieldsOf(in.normalize().validate()))
}

func TestSumAmount_ExactDecimal(t *testing.T) {
	assert.Equal(t, "0.300", sumAmount("0.1", "0.2"), "без потери точности float64")
	assert.Equal(t, "3.000", sumAmount("1", "2"))
}

// code review Medium#3 — регрессии дедупа ДО валидации (исправлено): раньше
// [{p,"abc"},{p,"2"}] давал "2.000" (мусор трактовался как 0), [{p,"-5"},{p,"6"}] давал
// "1.000" (отрицательное как компенсирующее, а не 400), [{p,"1e5"},{p,"1"}] давал
// "100001.000" (экспонента обходила decimalRe через big.Rat.SetString). Теперь validate()
// отклоняет КАЖДУЮ исходную строку ДО mergeProducts() — mergeProducts() получает на вход
// только валидные строки.
func TestServiceInput_Validate_RejectsBeforeMerge(t *testing.T) {
	pid := uuid.Must(uuid.NewV7()).String()

	in := validServiceInput()
	in.Products = []ServiceProductInput{{ProductID: pid, Qty: "abc"}, {ProductID: pid, Qty: "2"}}
	got := fieldsOf(in.normalize().validate())
	assert.Equal(t, msgInvalidFormat, got["products[0].qty"], "мусор в первой строке — 400, не 0 после мержа")

	in = validServiceInput()
	in.Products = []ServiceProductInput{{ProductID: pid, Qty: "-5"}, {ProductID: pid, Qty: "6"}}
	got = fieldsOf(in.normalize().validate())
	assert.Equal(t, msgInvalid, got["products[0].qty"], "отрицательная строка — 400, не «компенсируется» суммой")

	in = validServiceInput()
	in.Products = []ServiceProductInput{{ProductID: pid, Qty: "1e5"}, {ProductID: pid, Qty: "1"}}
	got = fieldsOf(in.normalize().validate())
	assert.Equal(t, msgInvalidFormat, got["products[0].qty"], "экспонента — 400, не сумма 100001.000")
}

func TestMergeProducts_SumsDuplicatesAfterValidation(t *testing.T) {
	p1 := uuid.Must(uuid.NewV7()).String()
	p2 := uuid.Must(uuid.NewV7()).String()
	rows := []ServiceProductInput{
		{ProductID: p1, Qty: "1.5"},
		{ProductID: p2, Qty: "2"},
		{ProductID: p1, Qty: "0.5"},
	}
	merged, errs := mergeProducts(rows)
	assert.Empty(t, errs)
	assert.Len(t, merged, 2)
	assert.Equal(t, p1, merged[0].ProductID)
	assert.Equal(t, "2.000", merged[0].Qty, "1.5+0.5 без потери точности")
	assert.Equal(t, p2, merged[1].ProductID)
	assert.Equal(t, "2", merged[1].Qty, "без дублей — как было")
}

// Сумма дублей у верхней границы qtyScale переполняет IntDigits — 400 на общее поле
// "products" (не products[i] — после мержа индекс не соответствует ни одной исходной
// строке, code review Медиум#3 — «сумма тоже проверяется на верхнюю границу»).
func TestMergeProducts_OverflowOnSum(t *testing.T) {
	p1 := uuid.Must(uuid.NewV7()).String()
	rows := []ServiceProductInput{
		{ProductID: p1, Qty: "999999999"},
		{ProductID: p1, Qty: "999999999"},
	}
	merged, errs := mergeProducts(rows)
	assert.Nil(t, merged)
	assert.Len(t, errs, 1)
	assert.Equal(t, "products", errs[0].Name)
	assert.Equal(t, msgTooLarge, errs[0].Message)
}
