package board

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"html"
	"regexp"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
	"github.com/gofiber/fiber/v3"

	"github.com/kitavrus/kanban-tree/internal/ssr"
	"github.com/kitavrus/kanban-tree/pkg/numeric"
	"github.com/kitavrus/kanban-tree/pkg/timezone"
)

// PDF-версия акта выдачи — тот же PrintVM и то же содержимое, что print-handover.html
// (клиент, телефоны, госномер, описание, все услуги с запчастями, итоги работ и запчастей, итог task.total_amount,
// отметка оплаты, подписи), но файлом PDF без «Печать → Сохранить как PDF» в браузере.
// Кириллица — встроенный TTF DejaVu Sans Condensed (из go-pdf/fpdf, свободная лицензия
// Bitstream Vera/DejaVu).

//go:embed fonts/DejaVuSansCondensed.ttf
var pdfFontRegular []byte

//go:embed fonts/DejaVuSansCondensed-Bold.ttf
var pdfFontBold []byte

const pdfFont = "dejavu"

// printPDF — GET /tasks/:id/print/:kind/pdf (RequireSSR, D19 — как и HTML-печать). Пока
// поддерживается только handover; иной kind или чужой/несуществующий заказ — 404-страница.
func (s *SSR) printPDF(c fiber.Ctx) error {
	if c.Params("kind") != PrintKindHandover {
		return s.status.NotFound(c)
	}

	vm, err := s.card.PrintData(c.Context(), currentActor(c), c.Params("id"))
	if err != nil {
		if errors.Is(err, ErrTaskNotFound) {
			return s.status.NotFound(c)
		}
		return err
	}
	vm.Kind = PrintKindHandover
	vm.Firm = s.firm
	vm.PrintedAt = time.Now()
	vm.Timezone = ssr.ViewerTimezone(c)

	body, err := renderHandoverPDF(vm)
	if err != nil {
		return err
	}

	c.Set(fiber.HeaderContentType, "application/pdf")
	c.Set(fiber.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="akt-vydachi-%d.pdf"`, vm.Card.Task.ID))
	return c.Send(body)
}

var subtaskStatusTitles = map[string]string{
	SubtaskStatusNew:        "Новая",
	SubtaskStatusInProgress: "В работе",
	SubtaskStatusDone:       "Выполнена",
}

func renderHandoverPDF(vm *PrintVM) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.SetAutoPageBreak(true, 15)
	pdf.AddUTF8FontFromBytes(pdfFont, "", pdfFontRegular)
	pdf.AddUTF8FontFromBytes(pdfFont, "B", pdfFontBold)
	pdf.SetTitle(fmt.Sprintf("Акт выдачи №%d", vm.Card.Task.ID), true)
	pdf.AddPage()

	pageW, _ := pdf.GetPageSize()
	left, _, right, _ := pdf.GetMargins()
	contentW := pageW - left - right

	// Реквизиты фирмы — каждое поле отдельно (как {{ if }} в HTML-шаблоне).
	if f := vm.Firm; f.Name != "" || f.Address != "" || f.Phone != "" {
		if f.Name != "" {
			pdf.SetFont(pdfFont, "B", 12)
			pdf.MultiCell(contentW, 6, f.Name, "", "L", false)
		}
		pdf.SetFont(pdfFont, "", 10)
		if f.Address != "" {
			pdf.MultiCell(contentW, 5, f.Address, "", "L", false)
		}
		if f.Phone != "" {
			pdf.MultiCell(contentW, 5, f.Phone, "", "L", false)
		}
		pdf.Ln(1)
		y := pdf.GetY()
		pdf.Line(left, y, left+contentW, y)
		pdf.Ln(4)
	}

	pdf.SetFont(pdfFont, "B", 18)
	pdf.MultiCell(contentW, 9, fmt.Sprintf("Акт выдачи №%d", vm.Card.Task.ID), "", "L", false)
	pdf.SetFont(pdfFont, "", 10)
	pdf.MultiCell(contentW, 5, "Дата: "+timezone.Format(vm.Timezone, "02.01.2006", vm.PrintedAt), "", "L", false)

	section := func(title string) {
		pdf.Ln(4)
		pdf.SetFont(pdfFont, "B", 11)
		pdf.MultiCell(contentW, 6, title, "", "L", false)
		pdf.SetFont(pdfFont, "", 10)
	}

	section("Клиент")
	pdf.MultiCell(contentW, 5, vm.ClientName, "", "L", false)
	if vm.Phones != "" {
		pdf.MultiCell(contentW, 5, "Телефон: "+vm.Phones, "", "L", false)
	}

	section("Автомобиль")
	pdf.MultiCell(contentW, 5, "Госномер: "+vm.Card.Car.PlateNumber, "", "L", false)

	if desc := descriptionHTMLToText(string(vm.Description)); desc != "" {
		section("Описание")
		pdf.MultiCell(contentW, 5, desc, "", "L", false)
	}

	section("Услуги")
	if len(vm.Card.Subtasks) == 0 {
		pdf.MultiCell(contentW, 5, "Услуги не добавлены.", "", "L", false)
	} else {
		widths := []float64{contentW * 0.24, contentW * 0.14, contentW * 0.10, contentW * 0.28, contentW * 0.13, contentW * 0.11}
		aligns := []string{"L", "L", "R", "L", "R", "R"}
		pdf.SetFont(pdfFont, "B", 9)
		pdfTableRow(pdf, widths, aligns, []string{"Услуга", "Статус", "Работа", "Запчасти", "Стоимость запчастей", "Итого"}, true)
		pdf.SetFont(pdfFont, "", 9)
		for _, st := range vm.Card.Subtasks {
			status, ok := subtaskStatusTitles[st.Status]
			if !ok {
				status = st.Status
			}
			products := make([]string, 0, len(st.Products))
			for _, p := range st.Products {
				products = append(products, fmt.Sprintf("%s: %s %s × %s = %s", p.Name, p.Qty, p.Unit, amountOrDash(p.Price), p.LineTotal))
			}
			productsCell := "—"
			if len(products) > 0 {
				productsCell = strings.Join(products, "\n")
			}
			pdfTableRow(pdf, widths, aligns, []string{st.ServiceName, status, amountOrDash(st.Price), productsCell, st.PartsTotal.String(), st.Total.String()}, false)
		}
	}

	pdf.Ln(4)
	pdf.SetFont(pdfFont, "", 10)
	pdf.MultiCell(contentW, 5, "Работы: "+vm.Card.WorksAmount.String(), "", "L", false)
	pdf.MultiCell(contentW, 5, "Запчасти: "+vm.Card.PartsAmount.String(), "", "L", false)
	pdf.SetFont(pdfFont, "B", 13)
	pdf.MultiCell(contentW, 7, "Итого: "+vm.Card.Task.TotalAmount.String(), "", "L", false)
	pdf.SetFont(pdfFont, "", 10)
	payment := "Не оплачено"
	if vm.Card.Task.IsPaid {
		payment = "Оплачено"
		if d := timezone.Format(vm.Timezone, "02.01.2006", vm.Card.Task.PaidAt); d != "" {
			payment += " " + d
		}
	}
	pdf.MultiCell(contentW, 5, payment, "", "L", false)

	pdf.Ln(16)
	half := contentW / 2
	pdf.CellFormat(half, 6, "Выдал: ______________________", "", 0, "L", false, 0, "")
	pdf.CellFormat(half, 6, "Получил: ______________________", "", 1, "R", false, 0, "")

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("board: render handover pdf: %w", err)
	}
	return buf.Bytes(), nil
}

// pdfTableRow — строка таблицы с переносом текста: высота строки = max числа строк в
// ячейках; строка, не влезающая на страницу, целиком переносится на следующую.
func pdfTableRow(pdf *fpdf.Fpdf, widths []float64, aligns []string, cells []string, header bool) {
	const lineH = 4.5
	const pad = 1.0
	lines := make([][]string, len(cells))
	maxLines := 1
	for i, txt := range cells {
		var ls []string
		for _, part := range strings.Split(txt, "\n") {
			ls = append(ls, pdf.SplitText(part, widths[i]-2*pad)...)
		}
		if len(ls) == 0 {
			ls = []string{""}
		}
		lines[i] = ls
		maxLines = max(maxLines, len(ls))
	}
	rowH := float64(maxLines)*lineH + 2*pad

	_, pageH := pdf.GetPageSize()
	_, _, _, bottom := pdf.GetMargins()
	if pdf.GetY()+rowH > pageH-bottom {
		pdf.AddPage()
	}

	x, y := pdf.GetXY()
	for i := range cells {
		if header {
			pdf.SetFillColor(235, 235, 235)
			pdf.Rect(x, y, widths[i], rowH, "FD")
		} else {
			pdf.Rect(x, y, widths[i], rowH, "D")
		}
		for j, l := range lines[i] {
			pdf.SetXY(x+pad, y+pad+float64(j)*lineH)
			pdf.CellFormat(widths[i]-2*pad, lineH, l, "", 0, aligns[i], false, 0, "")
		}
		x += widths[i]
	}
	left, _, _, _ := pdf.GetMargins()
	pdf.SetXY(left, y+rowH)
}

// amountOrDash — цена «по факту» (NULL) печатается прочерком, как в HTML-акте.
func amountOrDash(a *numeric.Amount) string {
	if a == nil {
		return "—"
	}
	return a.String()
}

var (
	reBreak   = regexp.MustCompile(`(?i)<br\s*/?>`)
	reParaEnd = regexp.MustCompile(`(?i)</p\s*>`)
	reTag     = regexp.MustCompile(`<[^>]*>`)
	reBlank   = regexp.MustCompile(`\n{3,}`)
)

// descriptionHTMLToText — description_html (ADR-006: <p> с <br>, уже экранирован) → текст
// с переносами строк; абзацы разделены пустой строкой (как descriptionHtmlToText в JS).
func descriptionHTMLToText(s string) string {
	s = reBreak.ReplaceAllString(s, "\n")
	s = reParaEnd.ReplaceAllString(s, "\n\n")
	s = reTag.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	s = reBlank.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}
