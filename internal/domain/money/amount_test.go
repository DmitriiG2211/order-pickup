package money_test

import (
	"testing"

	"orderissue/internal/domain/money"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMultiply_WholeQuantityAndPrice_ReturnsExactKopecks(t *testing.T) {
	// Arrange: заказ 00ДМ-000101, бумага: 3 × 1250
	qty, price := mustDecimal(t, "3"), mustDecimal(t, "1250")

	// Act
	amount, err := money.Multiply(qty, price)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, money.Amount(375000), amount)
}

func TestMultiply_FractionalQuantity_ReturnsExactKopecks(t *testing.T) {
	// Arrange: заказ 00ДМ-000103, степлер: 0.125 × 8000
	qty, price := mustDecimal(t, "0.125"), mustDecimal(t, "8000")

	// Act
	amount, err := money.Multiply(qty, price)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, money.Amount(100000), amount)
}

func TestMultiply_ExactlyHalfKopeck_RoundsUp(t *testing.T) {
	// Arrange: 3 × 0,335 = 1,005 ₽ — ровно половина копейки
	qty, price := mustDecimal(t, "3"), mustDecimal(t, "0.335")

	// Act
	amount, err := money.Multiply(qty, price)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, money.Amount(101), amount, "банковское округление дало бы 1,00")
}

func TestMultiply_JustBelowHalfKopeck_RoundsDown(t *testing.T) {
	// Arrange: 1 × 1,0049 = 100,49 копейки
	qty, price := mustDecimal(t, "1"), mustDecimal(t, "1.0049")

	// Act
	amount, err := money.Multiply(qty, price)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, money.Amount(100), amount)
}

func TestMultiply_ResultDoesNotFitInt64_ReturnsOverflow(t *testing.T) {
	// Arrange: почти миллиард × почти миллиард рублей
	huge := mustDecimal(t, "999999999")

	// Act
	_, err := money.Multiply(huge, huge)

	// Assert
	require.ErrorIs(t, err, money.ErrAmountOverflow)
}

func TestShare_ResultExactlyHalfKopeck_RoundsUp(t *testing.T) {
	// Arrange: пример из задания 1,265 → 1,27 (5,75 ₽ × 22%)
	amount := money.Amount(575)

	// Act
	vat, err := money.Share(amount, mustDecimal(t, "22"), mustDecimal(t, "100"))

	// Assert
	require.NoError(t, err)
	assert.Equal(t, money.Amount(127), vat)
}

func TestShare_ZeroDenominator_ReturnsError(t *testing.T) {
	// Act
	_, err := money.Share(money.Amount(100), mustDecimal(t, "22"), money.Zero)

	// Assert
	require.ErrorIs(t, err, money.ErrZeroDenominator)
}

func TestAmountString_WireFormat_MatchesTaskContract(t *testing.T) {
	cases := []struct {
		name    string
		kopecks money.Amount
		want    string
	}{
		{name: "целые рубли без дробной части", kopecks: 375000, want: "3750"},
		{name: "рубли с копейками", kopecks: 43582, want: "435.82"},
		{name: "десятки копеек — два знака", kopecks: 99050, want: "990.50"},
		{name: "меньше рубля", kopecks: 5, want: "0.05"},
		{name: "ноль", kopecks: 0, want: "0"},
		{name: "отрицательная", kopecks: -127, want: "-1.27"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			got := tc.kopecks.String()

			// Assert
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestAmountRubles_WholeRubles_AlwaysShowsKopecks(t *testing.T) {
	// Arrange: в PDF итоги читает человек, «3750» без копеек выглядит как ошибка
	amount := money.Amount(375000)

	// Act
	got := amount.Rubles()

	// Assert
	assert.Equal(t, "3750.00", got)
}
