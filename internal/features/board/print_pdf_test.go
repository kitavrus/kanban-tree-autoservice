package board

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/kitavrus/kanban-tree/internal/config"
	"github.com/kitavrus/kanban-tree/pkg/numeric"
)

func TestDescriptionHTMLToText(t *testing.T) {
	got := descriptionHTMLToText("<p>Стук &lt;двигателя&gt;<br>на прогреве</p><p>Дефектовка</p>")
	want := "Стук <двигателя>\nна прогреве\n\nДефектовка"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// Длинные ячейки и много строк — рендер не падает и даёт валидный PDF на нескольких страницах.
func TestRenderHandoverPDF(t *testing.T) {
	price := numeric.Amount("1500.00")
	subtasks := make([]CardSubtask, 0, 40)
	for range 40 {
		subtasks = append(subtasks, CardSubtask{
			ServiceName: strings.Repeat("Капитальный ремонт двигателя ", 3),
			Status:      SubtaskStatusDone,
			Price:       &price,
			Total:       "1540.00",
			Products: []CardSubtaskProduct{{
				Name: "Термопаста", Unit: "шт", Qty: "2", Price: nil, LineTotal: "40.00",
			}},
		})
	}
	vm := &PrintVM{
		Firm:        config.FirmInfo{Name: "Автосервис", Phone: "+7 900 000-00-00"},
		ClientName:  "Ковалёв Денис Андреевич",
		Phones:      "+7 926 700-11-22",
		Description: "<p>Стук на прогретом двигателе</p>",
		PrintedAt:   time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC),
		Timezone:    "Europe/Moscow",
	}
	vm.Card.Task.ID = 6
	vm.Card.Task.TotalAmount = "61600.00"
	vm.Card.Subtasks = subtasks

	out, err := renderHandoverPDF(vm)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(out, []byte("%PDF-")) {
		t.Fatal("output is not a PDF")
	}
	if n := bytes.Count(out, []byte("/Type /Page\n")); n < 2 {
		t.Fatalf("expected multi-page document, got %d pages", n)
	}
}
