package catalog

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
	"github.com/kitavrus/kanban-tree/pkg/sqlutil"
)

const serviceNameMax = 200

// ServiceProductInput — строка комплекта (product_id + qty, нормализованные строки).
type ServiceProductInput struct {
	ProductID string
	Qty       string
}

// ServiceInput — нормализованные поля услуги. HasPrice=false — «по факту» (NULL в БД).
// Products — ИСХОДНЫЕ (не смерженные) строки: дедупликация — отдельный шаг mergeProducts(),
// вызываемый ТОЛЬКО после успешного validate() (code review Medium#3 — дедуп ДО валидации
// принимал мусор как 0 и отрицательные как компенсирующие: [{p,"abc"},{p,"2"}]→"2.000",
// [{p,"-5"},{p,"6"}]→"1.000" проходили; экспонента "1e5" обходила decimalRe через
// big.Rat.SetString, который её прекрасно понимает).
type ServiceInput struct {
	CategoryID string
	Name       string
	HasPrice   bool
	Price      string
	Products   []ServiceProductInput
	IsActive   bool
}

// sumAmount — точное десятичное сложение (big.Rat, не float64 — без потери точности,
// CLAUDE.md §Уроки). Вызывается ТОЛЬКО из mergeProducts() над уже провалидированными
// (decimalRe-совместимыми) строками — здесь не нужно защищаться от мусора/экспоненты.
func sumAmount(a, b string) string {
	ra, ok := new(big.Rat).SetString(a)
	if !ok {
		ra = new(big.Rat)
	}
	rb, ok := new(big.Rat).SetString(b)
	if !ok {
		rb = new(big.Rat)
	}
	return new(big.Rat).Add(ra, rb).FloatString(3)
}

// normalize — trim полей комплекта БЕЗ дедупликации (code review Medium#3: дедуп —
// отдельный шаг mergeProducts(), после validate()); пустые строки (SSR row-list добавил
// строку, но ничего не выбрано) отбрасываются здесь — на них нет смысла заводить ошибку.
func (in ServiceInput) normalize() ServiceInput {
	in.CategoryID = normStr(in.CategoryID)
	in.Name = normStr(in.Name)
	if in.HasPrice {
		in.Price = strings.TrimSpace(in.Price)
		if in.Price == "" {
			in.HasPrice = false
		}
	}

	rows := make([]ServiceProductInput, 0, len(in.Products))
	for _, p := range in.Products {
		pid, qty := strings.TrimSpace(p.ProductID), strings.TrimSpace(p.Qty)
		if pid == "" && qty == "" {
			continue
		}
		rows = append(rows, ServiceProductInput{ProductID: pid, Qty: qty})
	}
	in.Products = rows
	return in
}

// validate — построчная валидация КАЖДОЙ исходной (ещё не смерженной) строки комплекта;
// индексы ошибок products[i].* — позиция в in.Products ПОСЛЕ normalize() (trim + отброс
// пустых), НО ДО mergeProducts() — форма при ошибке перерисовывает именно этот список
// (ssr_services.go serviceFormFrom вызывает только normalize()), индексы совпадают 1:1.
func (in ServiceInput) validate() []errorspkg.Field {
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
	case n > serviceNameMax:
		add("name", msgTooLong)
	}
	if sqlutil.HasInvalidText(in.Name) {
		add("name", msgInvalidText)
	}

	if in.HasPrice {
		if code := validateAmount(in.Price, priceScale, false); code != "" {
			add("price", code)
		}
	}

	for i, p := range in.Products {
		pf := fmt.Sprintf("products[%d].product_id", i)
		qf := fmt.Sprintf("products[%d].qty", i)
		if p.ProductID == "" || uuid.Validate(p.ProductID) != nil {
			add(pf, msgInvalid)
		}
		if code := validateAmount(p.Qty, qtyScale, false); code != "" {
			add(qf, code)
		} else if v, _ := strconv.ParseFloat(p.Qty, 64); v <= 0 {
			add(qf, msgInvalid)
		}
	}
	return f
}

// mergeProducts — дедуп по product_id суммой qty (D10), порядок — по первому вхождению.
// ВЫЗЫВАТЬ ТОЛЬКО после validate() без ошибок — каждая исходная строка уже гарантированно
// decimalRe-валидна и >0, sumAmount() не может получить мусор на вход. Сумма дублей всё
// равно проверяется на переполнение qtyScale (редкий кейс: две валидные строки у верхней
// границы, ~5e8 + ~5e8) — ошибка на общее поле "products" (не products[i] — после мержа
// индекс уже не соответствует ни одной из исходных строк формы, которые видит пользователь;
// маппинг «какая из исходных строк виновата» здесь не имеет смысла, т.к. виноваты обе).
func mergeProducts(rows []ServiceProductInput) ([]ServiceProductInput, []errorspkg.Field) {
	merged := make([]ServiceProductInput, 0, len(rows))
	index := map[string]int{}
	for _, p := range rows {
		if i, ok := index[p.ProductID]; ok {
			merged[i].Qty = sumAmount(merged[i].Qty, p.Qty)
			continue
		}
		index[p.ProductID] = len(merged)
		merged = append(merged, p)
	}
	for _, p := range merged {
		if code := validateAmount(p.Qty, qtyScale, false); code != "" {
			return nil, []errorspkg.Field{{Name: "products", Message: code}}
		}
	}
	return merged, nil
}
