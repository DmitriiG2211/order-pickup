package service_test

import (
	"context"
	"testing"
	"time"

	"orderissue/internal/domain/domaintest"
	"orderissue/internal/domain/shipment"
	"orderissue/internal/service"
	"orderissue/internal/usecase"
	"orderissue/internal/usecase/usecasetest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnsureShipmentDraft_NoExistingDraft_CreatesAndConfirms(t *testing.T) {
	// Arrange
	h := newHarness(domaintest.Order101)

	// Act
	result, err := h.svc.EnsureShipmentDraft(context.Background(), domaintest.Order101)

	// Assert
	require.NoError(t, err)
	assert.True(t, result.Created)
	assert.Equal(t, 1, h.shipments.CreateCalls)
	assert.NotEmpty(t, result.Draft.Ref)
}

func TestEnsureShipmentDraft_OurDraftAlreadyExists_ReturnsItWithoutCreating(t *testing.T) {
	// Arrange: черновик уже записан прошлым вызовом (или другой печатью)
	h := newHarness(domaintest.Order101)
	first, err := h.svc.EnsureShipmentDraft(context.Background(), domaintest.Order101)
	require.NoError(t, err)

	// Act
	second, err := h.svc.EnsureShipmentDraft(context.Background(), domaintest.Order101)

	// Assert
	require.NoError(t, err)
	assert.False(t, second.Created)
	assert.Equal(t, first.Draft.Ref, second.Draft.Ref)
	assert.Equal(t, 1, h.shipments.CreateCalls, "второй вызов не создаёт новый POST")
}

func TestEnsureShipmentDraft_ForeignDraftForSameOrder_IsIgnored(t *testing.T) {
	// Arrange: в общей базе по заказу уже лежит черновик другого кандидата
	h := newHarness(domaintest.Order101)
	h.shipments.Existing[domaintest.Order101] = []shipment.Recorded{{
		Ref: "foreign", Number: "00УТ-000199", OrderRef: domaintest.Order101, Comment: "черновик другого сервиса",
	}}

	// Act
	result, err := h.svc.EnsureShipmentDraft(context.Background(), domaintest.Order101)

	// Assert
	require.NoError(t, err)
	assert.True(t, result.Created)
}

func TestEnsureShipmentDraft_AmbiguousPostThenFoundOnRetry_CreatesExactlyOnce(t *testing.T) {
	// Arrange: первый POST создаёт документ, но клиент получает 503 —
	// со следующего цикла черновик должен найтись, а не создаться заново
	h := newHarness(domaintest.Order101)
	h.shipments.CreateResponses = []error{
		&usecase.UpstreamError{Code: usecase.UpstreamUnavailable, Outcome: usecase.OutcomeUnknown},
	}

	// Act
	result, err := h.svc.EnsureShipmentDraft(context.Background(), domaintest.Order101)

	// Assert
	require.NoError(t, err)
	assert.False(t, result.Created, "документ был найден поиском, а не создан этим циклом")
	assert.Equal(t, 1, h.shipments.CreateCalls, "второго POST не было")
}

func TestEnsureShipmentDraft_RejectedByOneC_ReturnsShipmentRejected(t *testing.T) {
	// Arrange: 1С отклонила запрос по существу — повторять нечего
	h := newHarness(domaintest.Order101)
	h.shipments.CreateResponses = []error{
		&usecase.UpstreamError{Code: usecase.UpstreamRequestRejected, Outcome: usecase.OutcomeNotApplied, UpstreamMessage: "не заполнен склад"},
	}

	// Act
	_, err := h.svc.EnsureShipmentDraft(context.Background(), domaintest.Order101)

	// Assert
	var rejected *usecase.ShipmentRejectedError
	require.ErrorAs(t, err, &rejected)
	assert.Equal(t, 1, h.shipments.CreateCalls)
}

func TestEnsureShipmentDraft_BlockedOrder_RefusesWithoutTouchingShipments(t *testing.T) {
	// Arrange
	h := newHarness(domaintest.Order105)

	// Act
	_, err := h.svc.EnsureShipmentDraft(context.Background(), domaintest.Order105)

	// Assert
	var notIssuable *usecase.OrderNotIssuableError
	require.ErrorAs(t, err, &notIssuable)
	assert.Equal(t, 0, h.shipments.CreateCalls)
	assert.Equal(t, 0, h.shipments.FindCalls)
}

func TestEnsureShipmentDraft_TwoConcurrentCallsSameOrder_CreatesOnlyOneDraft(t *testing.T) {
	// Arrange: два одновременных нажатия «Отметить в 1С»
	h := newHarness(domaintest.Order101)
	start := make(chan struct{})
	results := make(chan error, 2)

	for range 2 {
		go func() {
			<-start
			_, err := h.svc.EnsureShipmentDraft(context.Background(), domaintest.Order101)
			results <- err
		}()
	}
	close(start)

	// Act
	err1, err2 := <-results, <-results

	// Assert
	require.NoError(t, err1)
	require.NoError(t, err2)
	assert.Equal(t, 1, h.shipments.CreateCalls, "мьютекс на заказ не даёт создать два черновика")
}

func TestFindShipmentDraft_NoneRecorded_ReturnsShipmentDraftNotFound(t *testing.T) {
	// Arrange
	h := newHarness(domaintest.Order101)

	// Act
	_, err := h.svc.FindShipmentDraft(context.Background(), domaintest.Order101)

	// Assert
	require.ErrorIs(t, err, usecase.ErrShipmentDraftNotFound)
}

func TestFindShipmentDraft_AfterEnsure_ReturnsSameDraft(t *testing.T) {
	// Arrange
	h := newHarness(domaintest.Order101)
	created, err := h.svc.EnsureShipmentDraft(context.Background(), domaintest.Order101)
	require.NoError(t, err)

	// Act
	found, err := h.svc.FindShipmentDraft(context.Background(), domaintest.Order101)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, created.Draft.Ref, found.Ref)
}

func TestServiceStatus_DelegatesToHealthReporter(t *testing.T) {
	// Arrange
	h := newHarness(domaintest.Order101)
	h.svc = service.New(usecase.Deps{
		Orders: h.orders, Catalog: h.catalog, Stock: h.stock, Shipments: h.shipments,
		Renderer: h.renderer, Inflector: h.inflector,
		Health: &usecasetest.Health{State: usecase.IntegrationHealth{State: usecase.StateDegraded}},
		Clock:  usecasetest.Clock{Fixed: fixedNow}, ReadTimeout: time.Second, WriteTimeout: time.Second,
	})

	// Act
	status := h.svc.ServiceStatus()

	// Assert
	assert.Equal(t, usecase.StateDegraded, status.State)
}
