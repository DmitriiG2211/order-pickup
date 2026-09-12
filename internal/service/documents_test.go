package service_test

import (
	"context"
	"errors"
	"testing"

	"orderissue/internal/domain/domaintest"
	"orderissue/internal/domain/order"
	"orderissue/internal/domain/person"
	"orderissue/internal/domain/pickup"
	"orderissue/internal/usecase"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPreviewDocument_Order101_MatchesTaskReference(t *testing.T) {
	// Arrange
	h := newHarness(domaintest.Order101)

	// Act
	doc, err := h.svc.PreviewDocument(context.Background(), usecase.DocumentRequest{
		OrderRef: domaintest.Order101, Receiver: vorontsovInput(),
	})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "00ДМ-000101", doc.OrderNumber)
	assert.Len(t, doc.Lines, 4)
	assert.True(t, h.inflector.Called)
	assert.Equal(t, person.Male, h.inflector.GotGender)
}

func TestPreviewDocument_InvalidReceiverField_NeverCallsOrderReader(t *testing.T) {
	// Arrange: неверный телефон — ошибка должна поймать до обращения к 1С
	h := newHarness(domaintest.Order101)
	in := vorontsovInput()
	in.Phone = "999"

	// Act
	_, err := h.svc.PreviewDocument(context.Background(), usecase.DocumentRequest{
		OrderRef: domaintest.Order101, Receiver: in,
	})

	// Assert
	var fieldErrs pickup.FieldErrors
	require.ErrorAs(t, err, &fieldErrs)
	assert.Equal(t, 0, h.orders.Calls, "ввод не прошёл — в 1С обращаться незачем")
}

func TestPreviewDocument_BlockedOrder105_ComposedWithBlockReasons(t *testing.T) {
	// Arrange
	h := newHarness(domaintest.Order105)

	// Act
	doc, err := h.svc.PreviewDocument(context.Background(), usecase.DocumentRequest{
		OrderRef: domaintest.Order105, Receiver: vorontsovInput(),
	})

	// Assert
	require.NoError(t, err)
	assert.NotEmpty(t, doc.BlockReasons)
}

func TestPrintDocument_Order101_RendersPDFOnce(t *testing.T) {
	// Arrange
	h := newHarness(domaintest.Order101)

	// Act
	pdf, doc, err := h.svc.PrintDocument(context.Background(), usecase.DocumentRequest{
		OrderRef: domaintest.Order101, Receiver: vorontsovInput(),
	})

	// Assert
	require.NoError(t, err)
	assert.Equal(t, []byte("%PDF-fake"), pdf)
	assert.Equal(t, "00ДМ-000101", doc.OrderNumber)
	assert.Equal(t, 1, h.renderer.Calls)
}

func TestPrintDocument_BlockedOrder105_RefusesWithoutRenderingAndExplainsWhy(t *testing.T) {
	// Arrange: помеченный на удаление и непроведённый заказ печатать нельзя
	h := newHarness(domaintest.Order105)

	// Act
	_, _, err := h.svc.PrintDocument(context.Background(), usecase.DocumentRequest{
		OrderRef: domaintest.Order105, Receiver: vorontsovInput(),
	})

	// Assert
	var notIssuable *usecase.OrderNotIssuableError
	require.ErrorAs(t, err, &notIssuable)
	assert.ElementsMatch(t, []order.BlockReason{order.BlockedByDeletionMark, order.BlockedNotPosted}, notIssuable.Reasons)
	assert.Equal(t, 0, h.renderer.Calls, "заказ, который нельзя выдавать, не печатаем")
}

func TestPrintDocument_RendererFails_ReturnsWrappedError(t *testing.T) {
	// Arrange
	h := newHarness(domaintest.Order101)
	h.renderer.Err = errors.New("шрифт не найден")

	// Act
	_, _, err := h.svc.PrintDocument(context.Background(), usecase.DocumentRequest{
		OrderRef: domaintest.Order101, Receiver: vorontsovInput(),
	})

	// Assert
	require.ErrorContains(t, err, "шрифт не найден")
}
