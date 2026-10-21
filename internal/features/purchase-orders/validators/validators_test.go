package validators

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kitavrus/kanban-tree/internal/features/purchase-orders/models/dto"
	"github.com/kitavrus/kanban-tree/pkg/errorspkg"
)

const (
	p1 = "9c8b7a6d-5e4f-4a3b-8c2d-1e0f9a8b7c6d"
	p2 = "1f2e3d4c-5b6a-4987-8a6b-5c4d3e2f1a0b"
)

func fieldMap(f []errorspkg.Field) map[string]string {
	m := map[string]string{}
	for _, x := range f {
		m[x.Name] = x.Message
	}
	return m
}

func item(pid, qty string) dto.ItemRequest { return dto.ItemRequest{ProductID: pid, Qty: dto.Num(qty)} }

// TS-001 (DS-047, DS-055): validateCreate — пустые items, 201 строка, границы qty, note 1001,
// reason вне enum, сведение повторов product_id.
func TestTS001_ValidateCreate(t *testing.T) {
	_, f := ValidateCreate(dto.CreateRequest{})
	assert.Equal(t, MsgRequired, fieldMap(f)["items"], "items пуст → 400")

	many := make([]dto.ItemRequest, 201)
	for i := range many {
		many[i] = item(p1, "1")
	}
	_, f = ValidateCreate(dto.CreateRequest{Items: many})
	assert.Equal(t, MsgTooMany, fieldMap(f)["items"], "201 строка → 400")

	for _, bad := range []string{"0", "-1", "100000", "1.0001", "abc", "1e3", ""} {
		_, f = ValidateCreate(dto.CreateRequest{Items: []dto.ItemRequest{item(p1, bad)}})
		assert.Equal(t, MsgInvalid, fieldMap(f)["items[0].qty"], "qty %q → 400", bad)
	}
	for _, ok := range []string{"0.001", "99999.999", "4", "2.5"} {
		out, f := ValidateCreate(dto.CreateRequest{Items: []dto.ItemRequest{item(p1, ok)}})
		assert.Empty(t, f, "qty %q допустимо", ok)
		require.Len(t, out.Lines, 1)
	}

	_, f = ValidateCreate(dto.CreateRequest{Note: strings.Repeat("я", 1001), Items: []dto.ItemRequest{item(p1, "1")}})
	assert.Equal(t, MsgTooLong, fieldMap(f)["note"], "note 1001 → 400")
	_, f = ValidateCreate(dto.CreateRequest{Note: strings.Repeat("я", 1000), Items: []dto.ItemRequest{item(p1, "1")}})
	assert.Empty(t, f, "note 1000 — ok")

	_, f = ValidateCreate(dto.CreateRequest{Items: []dto.ItemRequest{{ProductID: p1, Qty: "1", Reason: "gift"}}})
	assert.Equal(t, MsgInvalid, fieldMap(f)["items[0].reason"], "reason вне enum → 400")

	_, f = ValidateCreate(dto.CreateRequest{Items: []dto.ItemRequest{item("not-uuid", "1")}})
	assert.Equal(t, MsgUnavailable, fieldMap(f)["items[0].product_id"], "битый product_id → недоступна")

	bad := "nope"
	_, f = ValidateCreate(dto.CreateRequest{SupplierID: &bad, Items: []dto.ItemRequest{item(p1, "1")}})
	assert.Equal(t, MsgUnavailable, fieldMap(f)["supplier_id"])

	// Повтор product_id → количества сложены, строка одна, reason по умолчанию manual.
	out, f := ValidateCreate(dto.CreateRequest{Items: []dto.ItemRequest{item(p1, "1.5"), item(p2, "2"), item(strings.ToUpper(p1), "2.25")}})
	require.Empty(t, f)
	require.Len(t, out.Lines, 2)
	assert.Equal(t, p1, out.Lines[0].ProductID)
	assert.Equal(t, "3.750", out.Lines[0].Qty)
	assert.Equal(t, "manual", out.Lines[0].Reason)
	assert.Equal(t, 0, out.Lines[0].Index)
	assert.Equal(t, 1, out.Lines[1].Index)
	assert.Nil(t, out.SupplierID, "null = Базар")

	// Сумма повторов больше предела → 400 на строке первого вхождения.
	_, f = ValidateCreate(dto.CreateRequest{Items: []dto.ItemRequest{item(p1, "99999"), item(p1, "1")}})
	assert.Equal(t, MsgInvalid, fieldMap(f)["items[0].qty"])
}

// TS-001: validateUpdate — версия обязательна, id строки — uuid, у существующей строки пустой
// reason значит «не менять».
func TestTS001_ValidateUpdate(t *testing.T) {
	_, f := ValidateUpdate(dto.UpdateRequest{Items: []dto.ItemRequest{item(p1, "1")}})
	assert.Equal(t, MsgRequired, fieldMap(f)["version"])

	bad := "x"
	_, f = ValidateUpdate(dto.UpdateRequest{Version: 1, Items: []dto.ItemRequest{{ID: &bad, ProductID: p1, Qty: "1"}}})
	assert.Equal(t, MsgInvalid, fieldMap(f)["items[0].id"])

	id := p2
	out, f := ValidateUpdate(dto.UpdateRequest{Version: 3, Note: "n", Items: []dto.ItemRequest{{ID: &id, ProductID: p1, Qty: "6"}, item(p1, "1")}})
	require.Empty(t, f)
	require.Len(t, out.Lines, 1, "новая строка с той же запчастью сведена в существующую")
	assert.Equal(t, p2, out.Lines[0].ID)
	assert.Equal(t, "7.000", out.Lines[0].Qty)
	assert.Equal(t, "", out.Lines[0].Reason, "у существующей строки reason не передан — не менять")
	assert.EqualValues(t, 3, out.Version)

	// В POST id строк не принимаются.
	_, f = ValidateCreate(dto.CreateRequest{Items: []dto.ItemRequest{{ID: &id, ProductID: p1, Qty: "1"}}})
	assert.Equal(t, MsgInvalid, fieldMap(f)["items[0].id"])
}

func num(s string) *dto.Num { n := dto.Num(s); return &n }

// TS-002 (DS-047): validateSupplierItem — границы «Соберу», цены и комментария.
func TestTS002_ValidateAnswer(t *testing.T) {
	for _, bad := range []string{"-0.001", "100000", "1.0001", "x"} {
		_, f := ValidateAnswer(dto.AnswerRequest{Version: 1, SupplierQty: num(bad)})
		assert.Equal(t, MsgInvalid, fieldMap(f)["supplier_qty"], "supplier_qty %q → 400", bad)
	}
	for _, ok := range []string{"0", "7", "99999.999"} {
		out, f := ValidateAnswer(dto.AnswerRequest{Version: 1, SupplierQty: num(ok)})
		assert.Empty(t, f, "supplier_qty %q допустимо (0 = нет, больше просимого можно)", ok)
		require.NotNil(t, out.SupplierQty)
	}
	for _, bad := range []string{"-1", "10000000", "1.001"} {
		_, f := ValidateAnswer(dto.AnswerRequest{Version: 1, SupplierQty: num("1"), SupplierPrice: num(bad)})
		assert.Equal(t, MsgInvalid, fieldMap(f)["supplier_price"], "supplier_price %q → 400", bad)
	}
	out, f := ValidateAnswer(dto.AnswerRequest{Version: 1, SupplierQty: num("1"), SupplierPrice: num("9999999.99")})
	assert.Empty(t, f)
	assert.Equal(t, "9999999.99", *out.SupplierPrice)

	_, f = ValidateAnswer(dto.AnswerRequest{Version: 1, SupplierNote: strings.Repeat("ы", 501)})
	assert.Equal(t, MsgTooLong, fieldMap(f)["supplier_note"])
	_, f = ValidateAnswer(dto.AnswerRequest{Version: 1, SupplierNote: strings.Repeat("ы", 500)})
	assert.Empty(t, f)

	out, f = ValidateAnswer(dto.AnswerRequest{Version: 1})
	assert.Empty(t, f)
	assert.Nil(t, out.SupplierQty, "null — ответа нет")

	_, f = ValidateAnswer(dto.AnswerRequest{SupplierQty: num("1")})
	assert.Equal(t, MsgRequired, fieldMap(f)["version"])
}

func TestValidatePageAndMessages(t *testing.T) {
	p, f := ValidatePage("status", "", []string{"draft"}, "", "", 50, 100)
	assert.Empty(t, f)
	assert.Equal(t, Page{Limit: 50}, p)
	_, f = ValidatePage("status", "zzz", []string{"draft"}, "0", "-1", 50, 100)
	m := fieldMap(f)
	assert.Equal(t, MsgInvalid, m["status"])
	assert.Equal(t, MsgInvalid, m["limit"])
	assert.Equal(t, MsgInvalid, m["offset"])
	_, f = ValidatePage("status", "", nil, "101", "", 50, 100)
	assert.Equal(t, MsgInvalid, fieldMap(f)["limit"])

	e := Error([]errorspkg.Field{{Name: "items[3].qty", Message: MsgInvalid}})
	assert.Equal(t, "VALIDATION", e.Code)
	assert.Equal(t, "Количество от 0.001 до 99 999.999", e.Message)
	assert.Equal(t, "Не больше 200 строк в заказе", Message("items", MsgTooMany))
	assert.Equal(t, "Поставщик недоступен", Message("supplier_id", MsgUnavailable))
}

func TestNumUnmarshal(t *testing.T) {
	var r dto.ItemRequest
	require.NoError(t, jsonUnmarshal(`{"product_id":"a","qty":4.5}`, &r))
	assert.Equal(t, dto.Num("4.5"), r.Qty)
	require.NoError(t, jsonUnmarshal(`{"product_id":"a","qty":"4"}`, &r))
	assert.Equal(t, dto.Num("4"), r.Qty)
	assert.Error(t, jsonUnmarshal(`{"qty":true}`, &r))
	var a dto.AnswerRequest
	require.NoError(t, jsonUnmarshal(`{"version":1,"supplier_qty":null}`, &a))
	assert.Nil(t, a.SupplierQty)
}
