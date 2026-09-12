package stock_test

import (
	"testing"

	"orderissue/internal/domain/domaintest"
	"orderissue/internal/domain/order"
	"orderissue/internal/domain/stock"

	"github.com/stretchr/testify/assert"
)

func item(p order.ProductRef) stock.Item {
	return stock.Item{Product: p, Characteristic: "00000000-0000-0000-0000-000000000000"}
}

func assertDecimal(t *testing.T, want string, got interface{ String() string }, msg string) {
	t.Helper()
	assert.Equal(t, want, got.String(), msg)
}

func TestPositions_PaperInOrder101_ShowsShortageAndOthersReservation(t *testing.T) {
	// Arrange: в демо-базе на Центральном складе бумаги 2, заказ 101 просит 3,
	// ещё 4 пачки зарезервированы под заказ 102
	o := domaintest.Order(domaintest.Order101)

	// Act
	positions := stock.Positions(o, domaintest.Balances(), domaintest.Reservations())

	// Assert
	paper := positions[item(domaintest.Product0001)]
	assertDecimal(t, "2", paper.OnHand, "лежит на складе заказа")
	assertDecimal(t, "3", paper.Needed, "нужно по заказу")
	assertDecimal(t, "1", paper.Shortage, "не хватает")
	assertDecimal(t, "4", paper.ReservedByOthers, "резерв заказа 102")
	assert.True(t, paper.HasShortage())
}

func TestPositions_ProductOnSeveralWarehouses_CountsOnlyOrderWarehouse(t *testing.T) {
	// Arrange: бумаги на Северном 90 и на Витрине 5, но заказ 101 с Центрального
	o := domaintest.Order(domaintest.Order101)

	// Act
	positions := stock.Positions(o, domaintest.Balances(), domaintest.Reservations())

	// Assert
	assertDecimal(t, "2", positions[item(domaintest.Product0001)].OnHand, "другие склады не считаются")
}

func TestPositions_OwnReservation_NotCountedAsOthers(t *testing.T) {
	// Arrange: у степлера в заказе 101 резерв только свой
	o := domaintest.Order(domaintest.Order101)

	// Act
	positions := stock.Positions(o, domaintest.Balances(), domaintest.Reservations())

	// Assert
	stapler := positions[item(domaintest.Product0004)]
	assertDecimal(t, "0", stapler.ReservedByOthers, "свой резерв не чужой")
	assert.False(t, stapler.HasShortage())
}

func TestPositions_SameProductInTwoLines_ComparesStockWithTheirSum(t *testing.T) {
	// Arrange: в заказе 103 ручка стоит двумя строками (2 и 3 шт.), на Северном 12
	o := domaintest.Order(domaintest.Order103)

	// Act
	positions := stock.Positions(o, domaintest.Balances(), domaintest.Reservations())

	// Assert
	pen := positions[item(domaintest.Product0002)]
	assertDecimal(t, "5", pen.Needed, "сумма двух строк")
	assertDecimal(t, "12", pen.OnHand, "")
	assert.False(t, pen.HasShortage())
}

func TestPositions_TwoLinesTogetherExceedStock_ReportsShortage(t *testing.T) {
	// Arrange: каждая строка по отдельности помещается в остаток, а вместе — нет
	o := domaintest.Order(domaintest.Order103)
	balances := []stock.Balance{
		{Item: item(domaintest.Product0002), Warehouse: domaintest.WarehouseNorth, OnHand: domaintest.Decimal("4")},
	}

	// Act
	positions := stock.Positions(o, balances, nil)

	// Assert
	assertDecimal(t, "1", positions[item(domaintest.Product0002)].Shortage, "нужно 5, есть 4")
}

func TestPositions_FractionalQuantity_ComparedExactly(t *testing.T) {
	// Arrange: в заказе 103 степлер 0,125 шт., на Северном 7
	o := domaintest.Order(domaintest.Order103)

	// Act
	positions := stock.Positions(o, domaintest.Balances(), domaintest.Reservations())

	// Assert
	stapler := positions[item(domaintest.Product0004)]
	assertDecimal(t, "0.125", stapler.Needed, "")
	assert.False(t, stapler.HasShortage())
}

func TestPositions_NoBalanceRecord_WholeNeedIsShortage(t *testing.T) {
	// Arrange: товара на складе нет вовсе — в регистре просто нет строки
	o := domaintest.Order(domaintest.Order101)

	// Act
	positions := stock.Positions(o, nil, nil)

	// Assert
	paper := positions[item(domaintest.Product0001)]
	assertDecimal(t, "0", paper.OnHand, "")
	assertDecimal(t, "3", paper.Shortage, "")
}

func TestPositions_BalanceOfOtherCharacteristic_NotCounted(t *testing.T) {
	// Arrange: остаток есть, но у другой характеристики товара
	o := domaintest.Order(domaintest.Order101)
	balances := []stock.Balance{{
		Item:      stock.Item{Product: domaintest.Product0001, Characteristic: "11111111-1111-1111-1111-111111111111"},
		Warehouse: domaintest.WarehouseCentral,
		OnHand:    domaintest.Decimal("100"),
	}}

	// Act
	positions := stock.Positions(o, balances, nil)

	// Assert
	assertDecimal(t, "0", positions[item(domaintest.Product0001)].OnHand, "")
}

func TestPositions_CancelledLine_NotNeeded(t *testing.T) {
	// Arrange: отменяем строку с бумагой
	o := domaintest.Order(domaintest.Order101)
	o.Lines[0].Cancelled = true

	// Act
	positions := stock.Positions(o, domaintest.Balances(), domaintest.Reservations())

	// Assert
	_, present := positions[item(domaintest.Product0001)]
	assert.False(t, present, "отменённую строку не выдаём и остаток по ней не показываем")
}
