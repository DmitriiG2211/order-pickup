package order_test

import (
	"testing"

	"orderissue/internal/domain/domaintest"
	"orderissue/internal/domain/order"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRef_GUIDInAnyCase_NormalizesToLowercase(t *testing.T) {
	// Arrange
	raw := " 2DA145F0-D5FC-11F1-A0B3-48DF371887E9 "

	// Act
	ref, err := order.ParseRef(raw)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, domaintest.Order101, ref)
}

func TestParseRef_NotAGUID_ReturnsInvalidRef(t *testing.T) {
	cases := map[string]string{
		"пусто":                   "",
		"номер документа":         "00ДМ-000101",
		"не хватает символа":      "2da145f0-d5fc-11f1-a0b3-48df371887e",
		"в фигурных скобках":      "{2da145f0-d5fc-11f1-a0b3-48df371887e9}",
		"не шестнадцатеричный":    "2da145f0-d5fc-11f1-a0b3-48df371887zz",
		"без дефисов":             "2da145f0d5fc11f1a0b348df371887e9",
		"ограничитель в 1С-стиле": "guid'2da145f0-d5fc-11f1-a0b3-48df371887e9'",
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			// Act
			_, err := order.ParseRef(raw)

			// Assert
			require.ErrorIs(t, err, order.ErrInvalidRef)
		})
	}
}

func TestBlockReasons_PostedAndNotDeleted_OrderIsIssuable(t *testing.T) {
	// Arrange: заказ 00ДМ-000101 проведён, к отгрузке
	o := domaintest.Order(domaintest.Order101)

	// Act
	reasons := o.BlockReasons()

	// Assert
	assert.Empty(t, reasons)
	assert.True(t, o.Issuable())
}

func TestBlockReasons_DeletionMarkedAndNotPosted_ReportsBothReasons(t *testing.T) {
	// Arrange: заказ 00ДМ-000105 в демо-базе помечен на удаление и не проведён
	o := domaintest.Order(domaintest.Order105)

	// Act
	reasons := o.BlockReasons()

	// Assert
	assert.Equal(t, []order.BlockReason{order.BlockedByDeletionMark, order.BlockedNotPosted}, reasons)
	assert.False(t, o.Issuable())
}

func TestBlockReasons_PostedButDeletionMarked_Blocked(t *testing.T) {
	// Arrange
	o := domaintest.Order(domaintest.Order101)
	o.DeletionMark = true

	// Act
	reasons := o.BlockReasons()

	// Assert
	assert.Equal(t, []order.BlockReason{order.BlockedByDeletionMark}, reasons)
}

func TestActiveLines_CancelledLine_Excluded(t *testing.T) {
	// Arrange: отменяем вторую строку заказа из четырёх
	o := domaintest.Order(domaintest.Order101)
	o.Lines[1].Cancelled = true

	// Act
	active := o.ActiveLines()

	// Assert
	numbers := make([]int, 0, len(active))
	for _, l := range active {
		numbers = append(numbers, l.Number)
	}
	assert.Equal(t, []int{1, 3, 4}, numbers)
}
