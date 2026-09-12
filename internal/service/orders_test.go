package service_test

import (
	"context"
	"testing"

	"orderissue/internal/domain/domaintest"
	"orderissue/internal/domain/order"
	"orderissue/internal/domain/pickup"
	"orderissue/internal/usecase"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListOrders_Order105_MarksBlockReasonsAndResolvesNames(t *testing.T) {
	// Arrange: демо-заказ 105 — организация, помечен на удаление, не проведён
	h := newHarness(domaintest.Order105)

	// Act
	list, err := h.svc.ListOrders(context.Background())

	// Assert
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "00ДМ-000105", list[0].Number)
	assert.Equal(t, "ООО «Ромашка»", list[0].CustomerName)
	assert.Equal(t, "Центральный склад", list[0].WarehouseName)
	assert.ElementsMatch(t, []order.BlockReason{order.BlockedByDeletionMark, order.BlockedNotPosted}, list[0].BlockReasons)
}

func TestListOrders_CustomerMissingFromCatalog_StillListsOrder(t *testing.T) {
	// Arrange: одна битая ссылка на партнёра не должна обрушить весь список
	h := newHarness(domaintest.Order101)
	delete(h.catalog.ByCustomer, h.orders.Byref[domaintest.Order101].Customer)

	// Act
	list, err := h.svc.ListOrders(context.Background())

	// Assert
	require.NoError(t, err)
	assert.Empty(t, list[0].CustomerName)
}

func TestGetOrder_Order101_ComposesSheetWithStockAndSuggestion(t *testing.T) {
	// Arrange
	h := newHarness(domaintest.Order101)

	// Act
	sheet, err := h.svc.GetOrder(context.Background(), domaintest.Order101)

	// Assert
	require.NoError(t, err)
	require.Len(t, sheet.Lines, 4)
	assert.Equal(t, "2", sheet.Lines[0].Stock.OnHand.String())
	require.NotNil(t, sheet.SuggestedReceiver)
	assert.Equal(t, "Воронцов Пётр Аркадьевич", sheet.SuggestedReceiver.Name.String())
}

func TestGetOrder_UnknownRef_ReturnsOrderNotFound(t *testing.T) {
	// Arrange
	h := newHarness(domaintest.Order101)

	// Act
	_, err := h.svc.GetOrder(context.Background(), order.Ref("00000000-0000-0000-0000-000000000000"))

	// Assert
	require.ErrorIs(t, err, usecase.ErrOrderNotFound)
}

func TestGetOrder_VatRateMissingFromCatalog_ReturnsMissingReference(t *testing.T) {
	// Arrange
	h := newHarness(domaintest.Order101)
	o := h.orders.Byref[domaintest.Order101]
	delete(h.catalog.ByVatRate, o.Lines[1].VatRate)

	// Act
	_, err := h.svc.GetOrder(context.Background(), domaintest.Order101)

	// Assert
	var missing *pickup.MissingReferenceError
	require.ErrorAs(t, err, &missing)
	assert.Equal(t, 2, missing.LineNumber)
}

func TestGetOrder_WarehouseMissingFromCatalog_ReturnsMissingReference(t *testing.T) {
	// Arrange
	h := newHarness(domaintest.Order101)
	delete(h.catalog.ByWarehouse, domaintest.WarehouseCentral)

	// Act
	_, err := h.svc.GetOrder(context.Background(), domaintest.Order101)

	// Assert
	var missing *pickup.MissingReferenceError
	require.ErrorAs(t, err, &missing)
	assert.Equal(t, "склад", missing.Kind)
}
