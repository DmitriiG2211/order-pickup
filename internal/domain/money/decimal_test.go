package money_test

import (
	"testing"

	"orderissue/internal/domain/money"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustDecimal(t *testing.T, s string) money.Decimal {
	t.Helper()
	d, err := money.ParseDecimal(s)
	require.NoError(t, err)
	return d
}

func TestParseDecimal_NumbersAsSentBy1C_KeepExactValue(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{name: "целая цена", input: "1250", want: "1250"},
		{name: "цена с половиной рубля", input: "990.5", want: "990.5"},
		{name: "дробное количество", input: "0.125", want: "0.125"},
		{name: "нули в конце дробной части не меняют значение", input: "1.2500", want: "1.25"},
		{name: "отрицательное", input: "-3", want: "-3"},
		{name: "ноль с дробной частью", input: "0.000", want: "0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange — вход в таблице

			// Act
			got, err := money.ParseDecimal(tc.input)

			// Assert
			require.NoError(t, err)
			assert.Equal(t, tc.want, got.String())
		})
	}
}

func TestParseDecimal_ExponentNotation_ParsesWithoutFloat(t *testing.T) {
	// Arrange: JSON-сериализатор может отдать маленькое число в экспоненте
	inputs := map[string]string{"2E-05": "0.00002", "1.5e2": "150"}

	for input, want := range inputs {
		// Act
		got, err := money.ParseDecimal(input)

		// Assert
		require.NoError(t, err, input)
		assert.Equal(t, want, got.String(), input)
	}
}

func TestParseDecimal_NotANumber_ReturnsInvalidDecimal(t *testing.T) {
	for _, input := range []string{"", "abc", "1.", ".5", "1,5", "--1", "1e", "12a"} {
		t.Run(input, func(t *testing.T) {
			// Act
			_, err := money.ParseDecimal(input)

			// Assert
			require.ErrorIs(t, err, money.ErrInvalidDecimal)
		})
	}
}

func TestParseDecimal_MoreThanSixFractionDigits_ReturnsTooPrecise(t *testing.T) {
	// Act
	_, err := money.ParseDecimal("0.0000001")

	// Assert
	require.ErrorIs(t, err, money.ErrTooPrecise)
}

func TestParseDecimal_BillionOrMore_ReturnsOutOfRange(t *testing.T) {
	// Act
	_, err := money.ParseDecimal("1000000000")

	// Assert
	require.ErrorIs(t, err, money.ErrOutOfRange)
}

func TestDecimalAdd_DifferentScales_AddsExactly(t *testing.T) {
	// Arrange: две строки одного товара с разной точностью количества
	a, b := mustDecimal(t, "0.125"), mustDecimal(t, "2")

	// Act
	sum := a.Add(b)

	// Assert
	assert.Equal(t, "2.125", sum.String())
}

func TestDecimalSub_SubtrahendGreater_ReturnsNegative(t *testing.T) {
	// Arrange: в наличии 2, нужно 3
	onHand, needed := mustDecimal(t, "2"), mustDecimal(t, "3")

	// Act
	diff := onHand.Sub(needed)

	// Assert
	assert.Equal(t, "-1", diff.String())
	assert.Equal(t, -1, diff.Sign())
}

func TestDecimalCmp_SameValueDifferentNotation_AreEqual(t *testing.T) {
	// Arrange
	a, b := mustDecimal(t, "1.50"), mustDecimal(t, "1.5")

	// Act
	result := a.Cmp(b)

	// Assert
	assert.Equal(t, 0, result)
}
