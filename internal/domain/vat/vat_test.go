package vat_test

import (
	"testing"

	"orderissue/internal/domain/money"
	"orderissue/internal/domain/vat"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func dec(t *testing.T, s string) money.Decimal {
	t.Helper()
	d, err := money.ParseDecimal(s)
	require.NoError(t, err)
	return d
}

func rate(t *testing.T, name, percent string) vat.Rate {
	t.Helper()
	r, err := vat.NewRate(name, dec(t, percent))
	require.NoError(t, err)
	return r
}

func TestCalculateLine_PriceWithoutVAT_AddsVatOnTop(t *testing.T) {
	// Arrange: эталон задания, заказ 00ДМ-000101, бумага ДМ-0001
	qty, price, r := dec(t, "3"), dec(t, "1250"), rate(t, "22%", "22")

	// Act
	line, err := vat.CalculateLine(qty, price, r, vat.MethodFor(false))

	// Assert
	require.NoError(t, err)
	assert.Equal(t, vat.Line{Amount: 375000, VAT: 82500, WithVAT: 457500}, line)
}

func TestCalculateLine_CalculatedRateInOrderWithoutFlag_StillAddsVatOnTop(t *testing.T) {
	// Arrange: эталон задания, ручка ДМ-0002 со ставкой «22/122» в заказе,
	// где цена НДС не включает. Название ставки не должно переключать способ.
	qty, price, r := dec(t, "2"), dec(t, "990.5"), rate(t, "22/122", "22")

	// Act
	line, err := vat.CalculateLine(qty, price, r, vat.MethodFor(false))

	// Assert
	require.NoError(t, err)
	assert.Equal(t, vat.Line{Amount: 198100, VAT: 43582, WithVAT: 241682}, line)
}

func TestCalculateLine_PriceIncludesVAT_ExtractsVatAndKeepsTotal(t *testing.T) {
	// Arrange: эталон задания, заказ 00ДМ-000102 с флагом «цена включает НДС»,
	// строка 4 × 2400 со ставкой «22%» — название обычное, решает флаг
	qty, price, r := dec(t, "4"), dec(t, "2400"), rate(t, "22%", "22")

	// Act
	line, err := vat.CalculateLine(qty, price, r, vat.MethodFor(true))

	// Assert
	require.NoError(t, err)
	assert.Equal(t, vat.Line{Amount: 960000, VAT: 173115, WithVAT: 960000}, line,
		"amountWithVat равен amount, НДС сидит внутри")
}

func TestCalculateLine_VatExactlyHalfKopeck_RoundsHalfUp(t *testing.T) {
	// Arrange: заказ 00ДМ-000103, скотч 5 × 1,15 = 5,75; НДС 1,265 → 1,27
	qty, price, r := dec(t, "5"), dec(t, "1.15"), rate(t, "22%", "22")

	// Act
	line, err := vat.CalculateLine(qty, price, r, vat.AddedOnTop)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, vat.Line{Amount: 575, VAT: 127, WithVAT: 702}, line)
}

func TestCalculateLine_VatTakenFromRoundedAmount_NotFromRawProduct(t *testing.T) {
	// Arrange: 1 × 2,2451 = 2,2451 → сумма 2,25.
	// От округлённой суммы: 2,25 × 22% = 0,495 → 0,50.
	// От сырого произведения было бы 0,4939 → 0,49 — на копейку меньше.
	qty, price, r := dec(t, "1"), dec(t, "2.2451"), rate(t, "22%", "22")

	// Act
	line, err := vat.CalculateLine(qty, price, r, vat.AddedOnTop)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, money.Amount(225), line.Amount)
	assert.Equal(t, money.Amount(50), line.VAT)
}

func TestCalculateLine_NonStandardRate_UsesPercentFromCatalog(t *testing.T) {
	// Arrange: заказ 00ДМ-000101, степлер со ставкой 13%, которой нет в НК РФ
	qty, price, r := dec(t, "1"), dec(t, "15000"), rate(t, "13%", "13")

	// Act
	line, err := vat.CalculateLine(qty, price, r, vat.AddedOnTop)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, vat.Line{Amount: 1500000, VAT: 195000, WithVAT: 1695000}, line)
}

func TestCalculateLine_WithoutVatRate_VatIsZero(t *testing.T) {
	// Arrange: заказ 00ДМ-000101, папки «Без НДС»
	qty, price, r := dec(t, "10"), dec(t, "45"), rate(t, "Без НДС", "0")

	// Act
	line, err := vat.CalculateLine(qty, price, r, vat.AddedOnTop)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, vat.Line{Amount: 45000, VAT: 0, WithVAT: 45000}, line)
}

func TestCalculateLine_FractionalRateIncluded_CalculatesWithoutFloat(t *testing.T) {
	// Arrange: 100 ₽ со ставкой 16,67% внутри: 100 × 16,67 / 116,67 = 14,2881 → 14,29
	qty, price, r := dec(t, "1"), dec(t, "100"), rate(t, "16,67%", "16.67")

	// Act
	line, err := vat.CalculateLine(qty, price, r, vat.Included)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, money.Amount(1429), line.VAT)
}

func TestCalculateLine_NegativeQuantity_ReturnsError(t *testing.T) {
	// Arrange
	qty, price, r := dec(t, "-1"), dec(t, "100"), rate(t, "22%", "22")

	// Act
	_, err := vat.CalculateLine(qty, price, r, vat.AddedOnTop)

	// Assert
	require.ErrorIs(t, err, vat.ErrNegativeLineInput)
}

func TestNewRate_PercentOutsideZeroToHundred_ReturnsError(t *testing.T) {
	for _, percent := range []string{"-1", "100", "150"} {
		t.Run(percent, func(t *testing.T) {
			// Act
			_, err := vat.NewRate("странная", dec(t, percent))

			// Assert
			require.ErrorIs(t, err, vat.ErrInvalidPercent)
		})
	}
}

func TestNewRate_TrimsTrailingWhitespaceFromDisplayName(t *testing.T) {
	rate, err := vat.NewRate("22/122 \u00a0", money.NewInt(22))

	require.NoError(t, err)
	assert.Equal(t, "22/122", rate.Name)
}

func TestTotal_Order101_MatchesReferenceTotals(t *testing.T) {
	// Arrange: все четыре строки заказа 00ДМ-000101
	lines := []vat.Line{
		{Amount: 375000, VAT: 82500, WithVAT: 457500},
		{Amount: 198100, VAT: 43582, WithVAT: 241682},
		{Amount: 45000, VAT: 0, WithVAT: 45000},
		{Amount: 1500000, VAT: 195000, WithVAT: 1695000},
	}

	// Act
	total := vat.Total(lines)

	// Assert: эталон задания 21181 / 3210.82 / 24391.82
	assert.Equal(t, vat.Line{Amount: 2118100, VAT: 321082, WithVAT: 2439182}, total)
}

func TestTotal_SumOfRoundedLines_DiffersFromRecalculationOnTotal(t *testing.T) {
	// Arrange: две строки по 0,25 ₽ с НДС 22%: 0,055 → 0,06 в каждой.
	// Итог по строкам 0,12, а пересчёт от общей суммы 0,50 дал бы 0,11.
	r := rate(t, "22%", "22")
	line, err := vat.CalculateLine(dec(t, "1"), dec(t, "0.25"), r, vat.AddedOnTop)
	require.NoError(t, err)

	// Act
	total := vat.Total([]vat.Line{line, line})

	// Assert
	assert.Equal(t, money.Amount(12), total.VAT)
}
