package pickup_test

import (
	"testing"
	"time"

	"orderissue/internal/domain/domaintest"
	"orderissue/internal/domain/money"
	"orderissue/internal/domain/order"
	"orderissue/internal/domain/person"
	"orderissue/internal/domain/pickup"
	"orderissue/internal/domain/stock"
	"orderissue/internal/domain/vat"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeInflector запоминает, что попросили просклонять, и отдаёт заданный ответ:
// само склонение проверяется в пакете petrovich.
type fakeInflector struct {
	gotName   person.FullName
	gotGender person.Gender
	result    person.FullName
}

func (f *fakeInflector) Genitive(n person.FullName, g person.Gender) person.FullName {
	f.gotName, f.gotGender = n, g
	return f.result
}

func refsFor(o order.Order) pickup.References {
	return pickup.References{
		Products:  domaintest.Products(),
		VatRates:  domaintest.VatRates(),
		Warehouse: domaintest.Warehouses()[o.Warehouse],
	}
}

func vorontsov(t *testing.T) pickup.Receiver {
	t.Helper()
	r, err := pickup.NewReceiver(pickup.ReceiverInput{
		LastName: "Воронцов", FirstName: "Пётр", MiddleName: "Аркадьевич", Gender: "м",
		Phone: "8 (999) 123-45-67", Email: "p.vorontsov@example.com",
	})
	require.NoError(t, err)
	return r
}

func sums(amount, vatAmount, withVAT money.Amount) vat.Line {
	return vat.Line{Amount: amount, VAT: vatAmount, WithVAT: withVAT}
}

func TestComposeDocument_Order101_MatchesTaskReference(t *testing.T) {
	// Arrange: пример ответа из задания для заказа 00ДМ-000101
	o := domaintest.Order(domaintest.Order101)
	inflector := &fakeInflector{result: person.FullName{Last: "Воронцова", First: "Петра", Middle: "Аркадьевича"}}

	// Act
	doc, err := pickup.ComposeDocument(o, refsFor(o), vorontsov(t), inflector)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "00ДМ-000101", doc.OrderNumber)
	assert.Equal(t, time.Date(2026, 8, 3, 9, 14, 27, 0, time.UTC), doc.OrderDate)
	assert.Equal(t, "Центральный склад", doc.WarehouseName)
	assert.False(t, doc.PriceIncludesVAT)
	assert.Equal(t, "Воронцова Петра Аркадьевича", doc.ReceiverGenitive.String())
	assert.Equal(t, "+79991234567", doc.Phone)
	assert.Equal(t, "p.vorontsov@example.com", doc.Email)

	require.Len(t, doc.Lines, 4)
	paper, pen := doc.Lines[0], doc.Lines[1]
	assert.Equal(t, []string{"ДМ-0001", "Бумага офисная А4, 500 листов", "3", "1250", "22%"},
		[]string{paper.Article, paper.Name, paper.Quantity.String(), paper.Price.String(), paper.VatName})
	assert.Equal(t, sums(375000, 82500, 457500), paper.Sums)
	assert.Equal(t, []string{"ДМ-0002", "Ручка шариковая синяя", "2", "990.5", "22/122"},
		[]string{pen.Article, pen.Name, pen.Quantity.String(), pen.Price.String(), pen.VatName})
	assert.Equal(t, sums(198100, 43582, 241682), pen.Sums)

	assert.Equal(t, sums(2118100, 321082, 2439182), doc.Totals, "итоги по всем четырём строкам")
	assert.Empty(t, doc.BlockReasons)
}

func TestComposeDocument_Order102PriceIncludesVAT_TotalWithVatEqualsAmount(t *testing.T) {
	// Arrange: заказ 00ДМ-000102 с флагом «цена включает НДС», ставки 22%, 20% и 10%
	o := domaintest.Order(domaintest.Order102)

	// Act
	doc, err := pickup.ComposeDocument(o, refsFor(o), vorontsov(t), &fakeInflector{})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, sums(960000, 173115, 960000), doc.Lines[0].Sums, "эталон задания: 4 × 2400")
	assert.Equal(t, sums(1134244, 195774, 1134244), doc.Totals)
}

func TestComposeDocument_Order103_TotalsAccountForHalfKopeckAndFractionalQuantity(t *testing.T) {
	// Arrange: в заказе 00ДМ-000103 скотч с НДС 1,265 → 1,27, степлер 0,125 шт.
	// и ручка двумя строками
	o := domaintest.Order(domaintest.Order103)

	// Act
	doc, err := pickup.ComposeDocument(o, refsFor(o), vorontsov(t), &fakeInflector{})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, sums(575, 127, 702), doc.Lines[0].Sums)
	assert.Equal(t, sums(100000, 22000, 122000), doc.Lines[1].Sums)
	assert.Equal(t, sums(335575, 73827, 409402), doc.Totals, "совпадает с СуммаДокумента в 1С")
}

func TestComposeDocument_ReceiverGiven_PassesNameAndGenderToInflector(t *testing.T) {
	// Arrange: пол задан явно, склонять надо именно с ним
	o := domaintest.Order(domaintest.Order101)
	r, err := pickup.NewReceiver(pickup.ReceiverInput{
		LastName: "Седых", FirstName: "Вера", MiddleName: "Павловна", Gender: "ж",
		Phone: "9991234567", Email: "v.sedykh@example.com",
	})
	require.NoError(t, err)
	inflector := &fakeInflector{}

	// Act
	_, err = pickup.ComposeDocument(o, refsFor(o), r, inflector)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, person.FullName{Last: "Седых", First: "Вера", Middle: "Павловна"}, inflector.gotName)
	assert.Equal(t, person.Female, inflector.gotGender)
}

func TestComposeDocument_CancelledLine_ExcludedFromLinesAndTotals(t *testing.T) {
	// Arrange: отменяем строку со степлером за 15000
	o := domaintest.Order(domaintest.Order101)
	o.Lines[3].Cancelled = true

	// Act
	doc, err := pickup.ComposeDocument(o, refsFor(o), vorontsov(t), &fakeInflector{})

	// Assert
	require.NoError(t, err)
	assert.Len(t, doc.Lines, 3)
	assert.Equal(t, sums(618100, 126082, 744182), doc.Totals)
}

func TestComposeDocument_VatRateMissingFromCatalog_ReturnsLineNumber(t *testing.T) {
	// Arrange: ставки второй строки нет в справочнике
	o := domaintest.Order(domaintest.Order101)
	refs := refsFor(o)
	delete(refs.VatRates, o.Lines[1].VatRate)

	// Act
	_, err := pickup.ComposeDocument(o, refs, vorontsov(t), &fakeInflector{})

	// Assert
	var missing *pickup.MissingReferenceError
	require.ErrorAs(t, err, &missing)
	assert.Equal(t, 2, missing.LineNumber)
	assert.Equal(t, "ставка НДС", missing.Kind)
}

func TestComposeDocument_ProductMissingFromCatalog_ReturnsLineNumber(t *testing.T) {
	// Arrange
	o := domaintest.Order(domaintest.Order101)
	refs := refsFor(o)
	delete(refs.Products, domaintest.Product0004)

	// Act
	_, err := pickup.ComposeDocument(o, refs, vorontsov(t), &fakeInflector{})

	// Assert
	var missing *pickup.MissingReferenceError
	require.ErrorAs(t, err, &missing)
	assert.Equal(t, 4, missing.LineNumber)
	assert.Equal(t, "номенклатура", missing.Kind)
}

func TestComposeDocument_BlockedOrder105_ComposedWithReasons(t *testing.T) {
	// Arrange: заказ помечен на удаление и не проведён — документ собирается,
	// а печатать или нет, решает сценарий
	o := domaintest.Order(domaintest.Order105)

	// Act
	doc, err := pickup.ComposeDocument(o, refsFor(o), vorontsov(t), &fakeInflector{})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, []order.BlockReason{order.BlockedByDeletionMark, order.BlockedNotPosted}, doc.BlockReasons)
	assert.Equal(t, sums(10000, 2200, 12200), doc.Totals)
}

func TestComposeSheet_Order101_AttachesStockAndSuggestsReceiverFromCustomerCard(t *testing.T) {
	// Arrange
	o := domaintest.Order(domaintest.Order101)
	positions := stock.Positions(o, domaintest.Balances(), domaintest.Reservations())
	customer := domaintest.Customers()[o.Customer]

	// Act
	sheet, err := pickup.ComposeSheet(o, refsFor(o), customer, positions)

	// Assert
	require.NoError(t, err)
	require.Len(t, sheet.Lines, 4)
	paper := sheet.Lines[0].Stock
	assert.Equal(t, []string{"2", "3", "1", "4"},
		[]string{paper.OnHand.String(), paper.Needed.String(), paper.Shortage.String(), paper.ReservedByOthers.String()})
	require.NotNil(t, sheet.SuggestedReceiver)
	assert.Equal(t, "Воронцов Пётр Аркадьевич", sheet.SuggestedReceiver.Name.String())
	assert.Equal(t, person.Male, sheet.SuggestedReceiver.Gender)
}

func TestSuggestReceiver_OrganizationCustomer_NoSuggestion(t *testing.T) {
	// Arrange: клиент заказа 105 — ООО «Ромашка», ФИО у организации нет
	o := domaintest.Order(domaintest.Order105)
	customer := domaintest.Customers()[o.Customer]

	// Act
	suggestion := pickup.SuggestReceiver(customer)

	// Assert
	assert.Nil(t, suggestion)
}

func TestSuggestReceiver_PersonWithoutPatronymic_SuggestsNameWithUnknownGender(t *testing.T) {
	// Arrange
	customer := order.Customer{Name: "Иванов Иван", IsPerson: true}

	// Act
	suggestion := pickup.SuggestReceiver(customer)

	// Assert
	require.NotNil(t, suggestion)
	assert.Equal(t, "Иванов Иван", suggestion.Name.String())
	assert.Zero(t, suggestion.Gender, "пол кладовщик выберет сам")
}

func TestSuggestReceiver_PersonNameNotTwoOrThreeWords_NoSuggestion(t *testing.T) {
	// Arrange: по одному слову не понять, где фамилия, а где имя
	customer := order.Customer{Name: "Воронцов", IsPerson: true}

	// Act
	suggestion := pickup.SuggestReceiver(customer)

	// Assert
	assert.Nil(t, suggestion)
}
