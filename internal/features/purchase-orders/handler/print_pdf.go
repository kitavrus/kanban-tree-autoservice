package handler

import (
	"bytes"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/kitavrus/kanban-tree/internal/config"
	"github.com/kitavrus/kanban-tree/internal/features/board"
	"github.com/kitavrus/kanban-tree/internal/features/purchase-orders/constants"
	"github.com/kitavrus/kanban-tree/internal/features/purchase-orders/mappers"
	"github.com/kitavrus/kanban-tree/internal/features/purchase-orders/models"
	"github.com/kitavrus/kanban-tree/internal/ssr"
	"github.com/kitavrus/kanban-tree/pkg/numeric"
	"github.com/kitavrus/kanban-tree/pkg/timezone"
)

// PDF заказа (§6.11, §6.16, §8.4) — тем же способом, что акт выдачи (board.NewPDFDoc: fpdf +
// встроенный DejaVu, DS-014). Время — в поясе смотрящего (DS-033).

// pdfMode — чей экземпляр: наш (Базар — колонка ☐; после ответа — цены) или поставщика.
type pdfMode int

const (
	pdfOurs pdfMode = iota
	pdfSupplier
)

// print — GET /purchases/:id/print: любой статус; executor — только свой (DS-060, DS-066).
func (s *SSR) print(c fiber.Ctx) error {
	o, err := s.svc.Model(c.Context(), actor(c), c.Params("id"))
	if err != nil {
		return s.notFound(c, err)
	}
	return s.sendPDF(c, o, pdfOurs)
}

// supplierPrint — GET /supplier/orders/:id/print: экземпляр поставщика без reason и наших данных.
func (s *SSR) supplierPrint(c fiber.Ctx) error {
	o, err := s.svc.SupplierModel(c.Context(), actor(c), c.Params("id"))
	if err != nil {
		return s.notFound(c, err)
	}
	return s.sendPDF(c, o, pdfSupplier)
}

func (s *SSR) sendPDF(c fiber.Ctx, o *models.PurchaseOrder, mode pdfMode) error {
	body, err := renderOrderPDF(o, mode, s.svc.Firm(), ssr.ViewerTimezone(c))
	if err != nil {
		return err
	}
	name := constants.NumberDisplay(o.Number) + ".pdf"
	c.Set(fiber.HeaderContentType, "application/pdf")
	c.Set(fiber.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`,
		strings.Replace(name, "З", "Z", 1), url.PathEscape(name)))
	return c.Send(body)
}

// dash — пустой ответ поставщика печатается «—», иначе значение через format
// (numeric.Amount.RUQty для количества, .RU для денег).
func dash(s *string, format func(numeric.Amount) string) string {
	if s == nil {
		return "—"
	}
	return format(numeric.Amount(*s))
}

func qtyRU(a numeric.Amount) string   { return a.RUQty() }
func moneyRU(a numeric.Amount) string { return a.RU() }

func renderOrderPDF(o *models.PurchaseOrder, mode pdfMode, firm config.FirmInfo, tz string) ([]byte, error) {
	pdf := board.NewPDFDoc()
	number := constants.NumberDisplay(o.Number)
	pdf.SetTitle("Заказ "+number, true)
	pdf.AddPage()

	pageW, _ := pdf.GetPageSize()
	left, _, right, _ := pdf.GetMargins()
	contentW := pageW - left - right
	font := board.PDFFont

	if firm.Name != "" || firm.Phone != "" {
		if firm.Name != "" {
			pdf.SetFont(font, "B", 12)
			pdf.MultiCell(contentW, 6, firm.Name, "", "L", false)
		}
		pdf.SetFont(font, "", 10)
		if firm.Phone != "" {
			pdf.MultiCell(contentW, 5, firm.Phone, "", "L", false)
		}
		pdf.Ln(1)
		y := pdf.GetY()
		pdf.Line(left, y, left+contentW, y)
		pdf.Ln(4)
	}

	date := o.CreatedAt
	if o.SentAt != nil {
		date = *o.SentAt
	}
	pdf.SetFont(font, "B", 16)
	pdf.MultiCell(contentW, 8, fmt.Sprintf("Заказ %s от %s", number, timezone.Format(tz, "02.01.2006", date)), "", "L", false)
	pdf.SetFont(font, "", 10)
	if mode == pdfOurs {
		recipient := "Получатель: " + mappers.RecipientName(o)
		if o.SupplierID != nil && o.SupplierPhone != "" {
			recipient += ", тел. " + o.SupplierPhone
		}
		pdf.MultiCell(contentW, 5, recipient, "", "L", false)
		pdf.MultiCell(contentW, 5, "Статус: "+constants.StatusTitles[o.Status], "", "L", false)
	}
	pdf.Ln(3)

	bazaar := mode == pdfOurs && o.SupplierID == nil
	answered := mode == pdfSupplier || o.AnsweredCount > 0
	headers := []string{"№", "Запчасть", "Артикул", "Кол-во", "Ед."}
	widths := []float64{0.06, 0.40, 0.18, 0.12, 0.08}
	aligns := []string{"R", "L", "L", "R", "L"}
	switch {
	case bazaar:
		headers = append(headers, "☐")
		widths = append(widths, 0.08)
		aligns = append(aligns, "C")
	case answered:
		headers = append(headers, "Соберёт", "Цена", "Сумма")
		widths = []float64{0.05, 0.31, 0.14, 0.10, 0.06, 0.10, 0.11, 0.13}
		aligns = append(aligns, "R", "R", "R")
	}
	for i := range widths {
		widths[i] *= contentW
	}
	pdf.SetFont(font, "B", 9)
	board.PDFTableRow(pdf, widths, aligns, headers, true)
	pdf.SetFont(font, "", 9)
	for i, it := range o.Items {
		row := []string{fmt.Sprint(i + 1), it.Name, it.Sku, numeric.Amount(it.Qty).RUQty(), it.Unit}
		switch {
		case bazaar:
			row = append(row, "")
		case answered:
			row = append(row, dash(it.SupplierQty, qtyRU), dash(it.SupplierPrice, moneyRU), dash(it.LineAmount, moneyRU))
		}
		board.PDFTableRow(pdf, widths, aligns, row, false)
	}

	pdf.Ln(3)
	if answered && o.TotalAmount != nil {
		pdf.SetFont(font, "B", 12)
		pdf.MultiCell(contentW, 7, "Итого: "+numeric.Amount(*o.TotalAmount).RU(), "", "R", false)
	}
	if strings.TrimSpace(o.Note) != "" {
		pdf.SetFont(font, "B", 10)
		pdf.MultiCell(contentW, 6, "Примечание", "", "L", false)
		pdf.SetFont(font, "", 10)
		pdf.MultiCell(contentW, 5, o.Note, "", "L", false)
	}
	pdf.Ln(14)
	pdf.SetFont(font, "", 10)
	pdf.CellFormat(contentW, 6, "Принял: ______________________", "", 1, "L", false, 0, "")
	pdf.SetFont(font, "", 8)
	pdf.CellFormat(contentW, 5, "Сформировано "+timezone.Format(tz, "02.01.2006 15:04", time.Now()), "", 1, "L", false, 0, "")

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("purchase-orders: render pdf: %w", err)
	}
	return buf.Bytes(), nil
}
