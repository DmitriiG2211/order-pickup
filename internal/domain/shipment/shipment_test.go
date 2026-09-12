package shipment_test

import (
	"testing"
	"time"

	"orderissue/internal/domain/domaintest"
	"orderissue/internal/domain/pickup"
	"orderissue/internal/domain/shipment"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var now = time.Date(2026, 9, 13, 11, 0, 0, 0, time.UTC)

func draft101(t *testing.T) shipment.Draft {
	t.Helper()
	o := domaintest.Order(domaintest.Order101)
	lines, _, err := pickup.PriceLines(o, pickup.References{
		Products: domaintest.Products(), VatRates: domaintest.VatRates(),
	})
	require.NoError(t, err)
	return shipment.NewDraft(o, lines, now)
}

// recordedFrom — черновик в том виде, как его вернула бы 1С после честной записи.
func recordedFrom(d shipment.Draft, number string) shipment.Recorded {
	r := shipment.Recorded{
		Ref: "7ae91afd-dedc-11f1-a0b5-48df371887e9", Number: number, Date: d.Date,
		OrderRef: d.Order.Ref, Comment: d.Comment,
	}
	for _, l := range d.Lines {
		r.Lines = append(r.Lines, shipment.RecordedLine{
			Product: l.Product, Quantity: l.Quantity, Price: l.Price, VatRate: l.VatRate, Sums: l.Sums,
		})
	}
	return r
}

func TestNewDraft_Order101_CommentCarriesOrderNumberAndMarker(t *testing.T) {
	// Act
	d := draft101(t)

	// Assert
	assert.Equal(t, "Выдача со склада по заказу 00ДМ-000101 [pickup-service заказ 2da145f0-d5fc-11f1-a0b3-48df371887e9]", d.Comment)
	assert.Len(t, d.Lines, 4)
	assert.Equal(t, now, d.Date)
}

func TestFindOurs_ForeignDraftForSameOrder_Ignored(t *testing.T) {
	// Arrange: в общей базе по заказу 101 уже есть черновик другого кандидата
	// (00УТ-000200, без нашего маркера) и наш
	d := draft101(t)
	foreign := recordedFrom(d, "00УТ-000200")
	foreign.Comment = ""
	ours := recordedFrom(d, "00УТ-000203")

	// Act
	found, ok := shipment.FindOurs(domaintest.Order101, []shipment.Recorded{foreign, ours})

	// Assert
	require.True(t, ok)
	assert.Equal(t, "00УТ-000203", found.Number)
}

func TestFindOurs_OnlyForeignDrafts_NotFound(t *testing.T) {
	// Arrange
	foreign := recordedFrom(draft101(t), "00УТ-000200")
	foreign.Comment = "Черновик другого сервиса"

	// Act
	_, ok := shipment.FindOurs(domaintest.Order101, []shipment.Recorded{foreign})

	// Assert
	assert.False(t, ok)
}

func TestFindOurs_MarkerOfAnotherOrder_NotOurs(t *testing.T) {
	// Arrange: наш маркер, но от другого заказа — ссылка на заказ совпала случайно
	r := recordedFrom(draft101(t), "00УТ-000204")
	r.Comment = "Выдача " + shipment.MarkerFor(domaintest.Order102)

	// Act
	_, ok := shipment.FindOurs(domaintest.Order101, []shipment.Recorded{r})

	// Assert
	assert.False(t, ok)
}

func TestFindOurs_OurDraftMarkedForDeletion_Ignored(t *testing.T) {
	// Arrange: наш черновик кто-то пометил на удаление — отметки о выдаче больше нет
	r := recordedFrom(draft101(t), "00УТ-000203")
	r.DeletionMark = true

	// Act
	_, ok := shipment.FindOurs(domaintest.Order101, []shipment.Recorded{r})

	// Assert
	assert.False(t, ok)
}

func TestFindOurs_SeveralOurDrafts_ReturnsEarliest(t *testing.T) {
	// Arrange: дубль остался с запуска до появления защиты
	d := draft101(t)
	later := recordedFrom(d, "00УТ-000210")
	later.Date = now.Add(time.Hour)
	earlier := recordedFrom(d, "00УТ-000203")

	// Act
	found, ok := shipment.FindOurs(domaintest.Order101, []shipment.Recorded{later, earlier})

	// Assert
	require.True(t, ok)
	assert.Equal(t, "00УТ-000203", found.Number)
}

func TestVerify_RecordedAsSent_NoMismatches(t *testing.T) {
	// Arrange
	d := draft101(t)
	r := recordedFrom(d, "00УТ-000203")

	// Act
	mm := shipment.Verify(d, r)

	// Assert
	assert.Empty(t, mm)
}

func TestVerify_QuantityChangedIn1C_ReportsLineAndBothValues(t *testing.T) {
	// Arrange
	d := draft101(t)
	r := recordedFrom(d, "00УТ-000203")
	r.Lines[1].Quantity = domaintest.Decimal("3")

	// Act
	mm := shipment.Verify(d, r)

	// Assert
	require.Len(t, mm, 1)
	assert.Equal(t, shipment.Mismatch{Field: "количество", Line: 2, Want: "2", Got: "3"}, mm[0])
	assert.Equal(t, "строка 2, количество: ожидали 2, в 1С 3", mm[0].String())
}

func TestVerify_LineLostOnWrite_ReportsLineCount(t *testing.T) {
	// Arrange: 1С записала три строки из четырёх — ровно та беда с обрезанной страницей
	d := draft101(t)
	r := recordedFrom(d, "00УТ-000203")
	r.Lines = r.Lines[:3]

	// Act
	mm := shipment.Verify(d, r)

	// Assert
	assert.Equal(t, []shipment.Mismatch{{Field: "количество строк", Want: "4", Got: "3"}}, mm)
}

func TestVerify_MarkerMissingInComment_Reported(t *testing.T) {
	// Arrange: без маркера мы не найдём черновик при следующем вызове и создадим дубль
	d := draft101(t)
	r := recordedFrom(d, "00УТ-000203")
	r.Comment = "Выдача со склада по заказу 00ДМ-000101"

	// Act
	mm := shipment.Verify(d, r)

	// Assert
	require.Len(t, mm, 1)
	assert.Equal(t, "маркер в комментарии", mm[0].Field)
}

func TestVerify_DraftAttachedToAnotherOrder_Reported(t *testing.T) {
	// Arrange
	d := draft101(t)
	r := recordedFrom(d, "00УТ-000203")
	r.OrderRef = domaintest.Order102

	// Act
	mm := shipment.Verify(d, r)

	// Assert
	assert.Contains(t, mm, shipment.Mismatch{Field: "заказ", Want: string(domaintest.Order101), Got: string(domaintest.Order102)})
}

func TestVerify_VatAmountDiffers_ReportedWithKopecks(t *testing.T) {
	// Arrange
	d := draft101(t)
	r := recordedFrom(d, "00УТ-000203")
	r.Lines[1].Sums.VAT = 43581

	// Act
	mm := shipment.Verify(d, r)

	// Assert
	assert.Equal(t, []shipment.Mismatch{{Field: "НДС", Line: 2, Want: "435.82", Got: "435.81"}}, mm)
}
