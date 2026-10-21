package validators

import (
	"math/big"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/kitavrus/kanban-tree/internal/features/purchase-orders/constants"
	"github.com/kitavrus/kanban-tree/internal/features/purchase-orders/models/dto"
)

// Go-fuzz валидаторов (§TD-7, in-process — исключение §TD-8): недоверенный ввод поставщика и
// сотрудника. Инварианты: без паники; принятое значение лежит в границах numeric(12,3)/(12,2) и
// длин полей; отклонённое — ошибка поля, а не молчаливое обрезание.

var seeds = []string{"0", "1", "0.001", "99999.999", "100000", "-1", "1.0001", "1,5", " 2 ", "1e3", "NaN", "",
	"9999999.99", "10000000", "00.1", "٣", "<script>alert(1)</script>", strings.Repeat("9", 40)}

func FuzzParseQty(f *testing.F) {
	for _, s := range seeds {
		f.Add(s)
	}
	max := new(big.Rat).SetFrac64(99999999, 1000)
	f.Fuzz(func(t *testing.T, s string) {
		r, ok := ParseQty(s)
		if !ok {
			return
		}
		if r.Sign() <= 0 || r.Cmp(max) > 0 {
			t.Fatalf("qty %q принято вне (0; 99999.999]: %s", s, r.FloatString(4))
		}
		if fs := r.FloatString(constants.QtyFracDigits); new(big.Rat).SetFrac(r.Num(), r.Denom()).Cmp(mustRat(fs)) != 0 {
			t.Fatalf("qty %q теряет точность при 3 знаках", s)
		}
	})
}

func mustRat(s string) *big.Rat {
	r, _ := new(big.Rat).SetString(s)
	return r
}

func FuzzValidateAnswer(f *testing.F) {
	for _, s := range seeds {
		f.Add(s, s, s, int64(1))
	}
	maxQty := new(big.Rat).SetFrac64(99999999, 1000)
	maxPrice := new(big.Rat).SetFrac64(999999999, 100)
	f.Fuzz(func(t *testing.T, qty, price, note string, version int64) {
		q, p := dto.Num(qty), dto.Num(price)
		out, fields := ValidateAnswer(dto.AnswerRequest{Version: version, SupplierQty: &q, SupplierPrice: &p, SupplierNote: note})
		bad := map[string]bool{}
		for _, fl := range fields {
			bad[fl.Name] = true
		}
		if version < 1 && !bad["version"] {
			t.Fatal("version < 1 принята")
		}
		if !bad["supplier_qty"] && out.SupplierQty != nil {
			r := mustRat(*out.SupplierQty)
			if r == nil || r.Sign() < 0 || r.Cmp(maxQty) > 0 {
				t.Fatalf("supplier_qty %q принято вне [0; 99999.999]", qty)
			}
		}
		if !bad["supplier_price"] && out.SupplierPrice != nil {
			r := mustRat(*out.SupplierPrice)
			if r == nil || r.Sign() < 0 || r.Cmp(maxPrice) > 0 {
				t.Fatalf("supplier_price %q принята вне [0; 9999999.99]", price)
			}
		}
		if !bad["supplier_note"] && utf8.RuneCountInString(note) > constants.SupplierNoteMax {
			t.Fatal("supplier_note > 500 принят")
		}
	})
}

func FuzzValidateCreate(f *testing.F) {
	for _, s := range seeds {
		f.Add(s, s, "manual", "9c8b7a6d-5e4f-4a3b-8c2d-1e0f9a8b7c6d")
	}
	f.Fuzz(func(t *testing.T, qty, note, reason, pid string) {
		out, fields := ValidateCreate(dto.CreateRequest{Note: note, Items: []dto.ItemRequest{
			{ProductID: pid, Qty: dto.Num(qty), Reason: reason},
			{ProductID: pid, Qty: dto.Num(qty), Reason: reason},
		}})
		if len(fields) > 0 {
			return
		}
		if utf8.RuneCountInString(out.Note) > constants.NoteMax {
			t.Fatal("note > 1000 принят")
		}
		if len(out.Lines) != 1 {
			t.Fatalf("дубли product_id не сведены: %d строк", len(out.Lines))
		}
		r := mustRat(out.Lines[0].Qty)
		if r == nil || r.Sign() <= 0 || r.Cmp(new(big.Rat).SetFrac64(99999999, 1000)) > 0 {
			t.Fatalf("сумма qty %q вне границ", out.Lines[0].Qty)
		}
		if !constants.In(out.Lines[0].Reason, constants.Reasons) {
			t.Fatalf("reason %q вне enum", out.Lines[0].Reason)
		}
	})
}

func FuzzValidateReason(f *testing.F) {
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		reason, fields := ValidateReason(1, &raw)
		if len(fields) > 0 {
			return
		}
		n := utf8.RuneCountInString(reason)
		if n < 1 || n > constants.CancelReasonMax || reason != strings.TrimSpace(reason) {
			t.Fatalf("причина %q принята вне 1–500 после trim", raw)
		}
	})
}
