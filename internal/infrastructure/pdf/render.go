package pdf

import (
	"bytes"
	"fmt"
	"strings"

	"orderissue/internal/domain/pickup"

	"github.com/signintech/gopdf"
)

func New() *Renderer { return &Renderer{} }

// Render собирает документ на выдачу. Ошибка возможна только при повреждении
// вшитого шрифта — это дефект сборки, а не времени исполнения.
func (r *Renderer) Render(doc pickup.Document) ([]byte, error) {
	gp := &gopdf.GoPdf{}
	// Размер страницы и координаты ниже заданы в миллиметрах. Нельзя брать
	// PageSizeA4Landscape: его 842×595 уже выражены в points.
	gp.Start(gopdf.Config{PageSize: gopdf.Rect{W: pageWidthMM, H: pageHeightMM}, Unit: gopdf.UnitMM})
	gp.SetInfo(gopdf.PdfInfo{Title: fmt.Sprintf("Выдача по заказу %s", doc.OrderNumber)})
	if err := gp.AddTTFFontData(fontRegular, regularFont); err != nil {
		return nil, fmt.Errorf("pdf: шрифт %s: %w", fontRegular, err)
	}
	if err := gp.AddTTFFontData(fontBold, boldFont); err != nil {
		return nil, fmt.Errorf("pdf: шрифт %s: %w", fontBold, err)
	}
	gp.AddPage()

	contentW := pageWidthMM - 2*marginMM
	gp.SetMargins(marginMM, marginMM, marginMM, marginMM)
	gp.SetXY(marginMM, marginMM)

	if err := writeText(gp, fontBold, titleSize, contentW,
		fmt.Sprintf("Выдать %s", doc.ReceiverGenitive.String())); err != nil {
		return nil, err
	}
	gp.Br(lineGap)
	if err := writeLine(gp, contentW, "Телефон: "+doc.Phone); err != nil {
		return nil, err
	}
	if err := writeLine(gp, contentW, "Почта: "+doc.Email); err != nil {
		return nil, err
	}
	gp.Br(lineGap)
	if err := writeLine(gp, contentW, fmt.Sprintf("Заказ: %s от %s", doc.OrderNumber, doc.OrderDate.Format("02.01.2006"))); err != nil {
		return nil, err
	}
	if err := writeLine(gp, contentW, "Склад: "+doc.WarehouseName); err != nil {
		return nil, err
	}
	gp.Br(lineGap)

	if err := drawTableHeader(gp); err != nil {
		return nil, err
	}
	for _, l := range doc.Lines {
		if gp.GetY() > pageHeightMM-marginMM-rowH {
			gp.AddPage()
			gp.SetXY(marginMM, marginMM)
			if err := drawTableHeader(gp); err != nil {
				return nil, err
			}
		}
		if err := drawTableRow(gp, l); err != nil {
			return nil, err
		}
	}
	if err := drawTotalsRow(gp, doc); err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if _, err := gp.WriteTo(&buf); err != nil {
		return nil, fmt.Errorf("pdf: сборка файла: %w", err)
	}
	return buf.Bytes(), nil
}

func writeText(gp *gopdf.GoPdf, font string, size float64, width float64, text string) error {
	if err := gp.SetFont(font, "", size); err != nil {
		return fmt.Errorf("pdf: шрифт %s: %w", font, err)
	}
	return gp.Cell(&gopdf.Rect{W: width, H: rowH}, text)
}

func writeLine(gp *gopdf.GoPdf, width float64, text string) error {
	if err := writeText(gp, fontRegular, textSize, width, text); err != nil {
		return err
	}
	gp.Br(lineGap)
	return nil
}

func drawTableHeader(gp *gopdf.GoPdf) error {
	if err := gp.SetFont(fontBold, "", headerSize); err != nil {
		return fmt.Errorf("pdf: шрифт %s: %w", fontBold, err)
	}
	x, y := gp.GetX(), gp.GetY()
	gp.SetFillColor(235, 235, 235)
	gp.RectFromUpperLeftWithStyle(x, y, tableWidth(), rowH, "F")
	gp.SetFillColor(0, 0, 0)

	cellX := x
	for _, col := range columns {
		if err := drawCell(gp, cellX, y, col.width, rowH, col.title, col.align); err != nil {
			return fmt.Errorf("pdf: заголовок таблицы: %w", err)
		}
		cellX += col.width
	}
	gp.SetXY(x, y+rowH)
	return nil
}

func drawTableRow(gp *gopdf.GoPdf, l pickup.PricedLine) error {
	if err := gp.SetFont(fontRegular, "", textSize); err != nil {
		return fmt.Errorf("pdf: шрифт %s: %w", fontRegular, err)
	}
	values := []string{
		l.Article, l.Name, l.Quantity.String(), humanDecimal(l.Price),
		l.VatName, humanAmount(l.Sums.Amount), humanAmount(l.Sums.VAT), humanAmount(l.Sums.WithVAT),
	}
	x, y := gp.GetX(), gp.GetY()
	cellX := x
	for i, v := range values {
		if err := drawCell(gp, cellX, y, columns[i].width, rowH, v, columns[i].align); err != nil {
			return fmt.Errorf("pdf: строка %s: %w", l.Article, err)
		}
		cellX += columns[i].width
	}
	gp.SetXY(x, y+rowH)
	return nil
}

func drawTotalsRow(gp *gopdf.GoPdf, doc pickup.Document) error {
	if err := gp.SetFont(fontBold, "", textSize); err != nil {
		return fmt.Errorf("pdf: шрифт %s: %w", fontBold, err)
	}
	labelWidth := columns[0].width + columns[1].width + columns[2].width + columns[3].width + columns[4].width
	x, y := gp.GetX(), gp.GetY()
	if err := drawCell(gp, x, y, labelWidth, rowH, "Итого:", gopdf.Right); err != nil {
		return fmt.Errorf("pdf: итоги: %w", err)
	}
	totals := []string{humanAmount(doc.Totals.Amount), humanAmount(doc.Totals.VAT), humanAmount(doc.Totals.WithVAT)}
	cellX := x + labelWidth
	for i, v := range totals {
		col := columns[5+i]
		if err := drawCell(gp, cellX, y, col.width, rowH, v, gopdf.Right); err != nil {
			return fmt.Errorf("pdf: итоги: %w", err)
		}
		cellX += col.width
	}
	gp.SetXY(x, y+rowH)
	return nil
}

// drawCell рисует рамку ячейки в её полных границах и текст с горизонтальным
// отступом cellPadX. Раздельно, потому что CellWithOption рисует текст и рамку
// одним вызовом на одной ширине — без этого текст «Сумма с НДС» упирался в линии сетки.
func drawCell(gp *gopdf.GoPdf, x, y, w, h float64, text string, align int) error {
	gp.RectFromUpperLeftWithStyle(x, y, w, h, "D")
	gp.SetXY(x+cellPadX, y)
	inner := w - 2*cellPadX
	return gp.CellWithOption(&gopdf.Rect{W: inner, H: h}, text, gopdf.CellOption{Align: align | gopdf.Middle})
}

func tableWidth() float64 {
	var w float64
	for _, c := range columns {
		w += c.width
	}
	return w
}

// humanDecimal сохраняет точность цены, но приводит десятичный разделитель
// и минимум две дробные цифры к привычному для кладовщика виду.
func humanDecimal(value fmt.Stringer) string {
	raw := strings.Replace(value.String(), ".", ",", 1)
	whole, fraction, found := strings.Cut(raw, ",")
	switch {
	case !found:
		return whole + ",00 ₽"
	case len(fraction) == 1:
		return whole + "," + fraction + "0 ₽"
	default:
		return whole + "," + fraction + " ₽"
	}
}

func humanAmount(amount interface{ Rubles() string }) string {
	return strings.Replace(amount.Rubles(), ".", ",", 1) + " ₽"
}
