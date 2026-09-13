package pdf_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"orderissue/internal/domain/money"
	"orderissue/internal/domain/order"
	"orderissue/internal/domain/person"
	"orderissue/internal/domain/pickup"
	"orderissue/internal/domain/vat"
	renderer "orderissue/internal/infrastructure/pdf"

	pdfread "github.com/ledongthuc/pdf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func priced(article, name string, qty, price int64, vatName string, amount, vatAmount, withVAT money.Amount) pickup.PricedLine {
	return pickup.PricedLine{
		Line:    order.Line{Product: order.ProductRef(article), Quantity: money.NewInt(qty), Price: money.NewInt(price)},
		Article: article, Name: name, VatName: vatName,
		Sums: vat.Line{Amount: amount, VAT: vatAmount, WithVAT: withVAT},
	}
}

func sampleDocument() pickup.Document {
	lines := []pickup.PricedLine{
		priced("ДМ-0001", "Бумага офисная А4, 500 листов", 3, 1250, "22%", 375000, 82500, 457500),
		priced("ДМ-0002", "Ручка шариковая синяя", 2, 990, "22/122", 198100, 43582, 241682),
	}
	return pickup.Document{
		OrderNumber:      "00ДМ-000101",
		OrderDate:        time.Date(2026, 8, 3, 9, 14, 27, 0, time.UTC),
		WarehouseName:    "Центральный склад",
		ReceiverGenitive: person.FullName{Last: "Воронцова", First: "Петра", Middle: "Аркадьевича"},
		Phone:            "+79991234567",
		Email:            "p.vorontsov@example.com",
		Lines:            lines,
		Totals:           vat.Total([]vat.Line{lines[0].Sums, lines[1].Sums}),
	}
}

func extractText(t *testing.T, raw []byte) string {
	t.Helper()
	require.True(t, bytes.HasPrefix(raw, []byte("%PDF-")), "файл должен быть настоящим PDF, а не картинкой")

	r, err := pdfread.NewReader(bytes.NewReader(raw), int64(len(raw)))
	require.NoError(t, err)

	var sb strings.Builder
	for i := 1; i <= r.NumPage(); i++ {
		for _, tx := range r.Page(i).Content().Text {
			sb.WriteString(tx.S)
		}
		sb.WriteString("\n")
	}
	text := sb.String()
	require.NotEmpty(t, strings.TrimSpace(text), "из PDF должен извлекаться текст, иначе это картинка")
	return text
}

func TestRender_Order101_ProducesTextualPDFWithGenitiveNameAndTotals(t *testing.T) {
	// Arrange
	doc := sampleDocument()
	r := renderer.New()

	// Act
	raw, err := r.Render(doc)

	// Assert
	require.NoError(t, err)
	text := extractText(t, raw)
	assert.Contains(t, text, "Воронцова Петра Аркадьевича", "ФИО в родительном падеже")
	assert.Contains(t, text, "+79991234567")
	assert.Contains(t, text, "00ДМ-000101")
	assert.Contains(t, text, "Центральный склад")
	assert.Contains(t, text, "ДМ-0001")
	assert.Contains(t, text, "5731,00 ₽", "итог с копейками и обозначением валюты")
}

func TestRender_LongProductName_DoesNotBreakLayout(t *testing.T) {
	// Arrange: длинное наименование не должно ломать сборку документа
	doc := sampleDocument()
	doc.Lines[0].Name = strings.Repeat("Очень длинное наименование товара ", 4)
	r := renderer.New()

	// Act
	raw, err := r.Render(doc)

	// Assert
	require.NoError(t, err)
	assert.True(t, bytes.HasPrefix(raw, []byte("%PDF-")))
}

func TestRender_ManyLines_FlowsToSecondPage(t *testing.T) {
	// Arrange: заведомо больше строк, чем помещается на одной странице A4
	doc := sampleDocument()
	base := doc.Lines[0]
	for range 40 {
		doc.Lines = append(doc.Lines, base)
	}
	r := renderer.New()

	// Act
	raw, err := r.Render(doc)

	// Assert
	require.NoError(t, err)
	rr, err := pdfread.NewReader(bytes.NewReader(raw), int64(len(raw)))
	require.NoError(t, err)
	assert.GreaterOrEqual(t, rr.NumPage(), 2)
}

func TestRender_ZeroLines_StillProducesValidPDFWithTotalsRow(t *testing.T) {
	// Arrange: пустой заказ — вырожденный случай, документ всё равно должен собраться
	doc := sampleDocument()
	doc.Lines = nil
	doc.Totals = vat.Line{}
	r := renderer.New()

	// Act
	raw, err := r.Render(doc)

	// Assert
	require.NoError(t, err)
	text := extractText(t, raw)
	assert.Contains(t, text, "Итого")
}
