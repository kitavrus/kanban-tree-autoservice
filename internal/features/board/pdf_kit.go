package board

import "github.com/go-pdf/fpdf"

// Общий PDF-набор для печатных форм других фич (purchase-orders §6.11/§6.16, DS-014): тот же
// способ, что акт выдачи — встроенные шрифты DejaVu (кириллица) и строка таблицы с переносом.

// PDFFont — имя семейства шрифтов документа NewPDFDoc ("" — обычный, "B" — жирный).
const PDFFont = pdfFont

// NewPDFDoc — A4 книжный, поля 15 мм, автоперенос страниц, шрифты DejaVu подключены.
func NewPDFDoc() *fpdf.Fpdf {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.SetAutoPageBreak(true, 15)
	pdf.AddUTF8FontFromBytes(pdfFont, "", pdfFontRegular)
	pdf.AddUTF8FontFromBytes(pdfFont, "B", pdfFontBold)
	return pdf
}

// PDFTableRow — строка таблицы с переносом текста (см. pdfTableRow).
func PDFTableRow(pdf *fpdf.Fpdf, widths []float64, aligns []string, cells []string, header bool) {
	pdfTableRow(pdf, widths, aligns, cells, header)
}
