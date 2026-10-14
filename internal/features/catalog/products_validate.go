package catalog

import (
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
	"github.com/kitavrus/kanban-tree/pkg/sqlutil"
)

// Units — единицы измерения товара (прототип, UNITS). Пустая строка — «не указана».
var Units = []string{"шт", "л", "м", "кг", "компл", "час"}

func isValidUnit(u string) bool {
	if u == "" {
		return true
	}
	for _, v := range Units {
		if v == u {
			return true
		}
	}
	return false
}

const (
	productNameMax = 200
	skuMax         = 100
	boxPlaceMax    = 100
)

// ProductInput — нормализованные поля товара (общие для API и SSR-формы).
type ProductInput struct {
	CategoryID string
	Name       string
	SKU        string
	Unit       string
	Price      string
	Stocks     []ProductStockInput // D46: остатки по складам (перечисленные склады)
	IsActive   bool
}

func normStr(s string) string { return strings.TrimSpace(s) }

func normAmount(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "0"
	}
	return s
}

func (in ProductInput) normalize() ProductInput {
	in.CategoryID = normStr(in.CategoryID)
	in.Name = normStr(in.Name)
	in.SKU = normStr(in.SKU)
	in.Unit = normStr(in.Unit)
	in.Price = normAmount(in.Price)
	in.Stocks = normStocks(in.Stocks)
	return in
}

// validate — ошибки полей после normalize(). Остатки по складам — validateStocks
// (остаток может быть < 0, D44; минимум — нет).
func (in ProductInput) validate() []errorspkg.Field {
	var f []errorspkg.Field
	add := func(name, msg string) { f = append(f, errorspkg.Field{Name: name, Message: msg}) }

	if in.CategoryID == "" {
		add("category_id", msgRequired)
	} else if uuid.Validate(in.CategoryID) != nil {
		add("category_id", msgInvalid)
	}

	switch n := utf8.RuneCountInString(in.Name); {
	case n == 0:
		add("name", msgRequired)
	case n > productNameMax:
		add("name", msgTooLong)
	}
	if sqlutil.HasInvalidText(in.Name) {
		add("name", msgInvalidText)
	}

	if utf8.RuneCountInString(in.SKU) > skuMax {
		add("sku", msgTooLong)
	}
	if sqlutil.HasInvalidText(in.SKU) {
		add("sku", msgInvalidText)
	}
	if !isValidUnit(in.Unit) {
		add("unit", msgInvalid)
	}

	addAmount := func(field, val string, scale amountScale, allowNegative bool) {
		if code := validateAmount(val, scale, allowNegative); code != "" {
			add(field, code)
		}
	}
	addAmount("price", in.Price, priceScale, false)
	f = append(f, validateStocks(in.Stocks)...)
	return f
}
