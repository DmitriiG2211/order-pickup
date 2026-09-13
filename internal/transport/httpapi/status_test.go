package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"orderissue/internal/domain/domaintest"
	"orderissue/internal/domain/order"
	"orderissue/internal/service"
	"orderissue/internal/usecase"
	"orderissue/internal/usecase/usecasetest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealthz_ReturnsOK(t *testing.T) {
	h, _ := testServer(t)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/healthz", nil))

	require.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"status":"ok"}`, w.Body.String())
}

func TestStatus_ReflectsHealthReporter(t *testing.T) {
	orders := &usecasetest.Orders{Byref: map[order.Ref]order.Order{domaintest.Order101: domaintest.Order(domaintest.Order101)}}
	implementation := service.New(usecase.Deps{
		Orders: orders,
		Catalog: &usecasetest.Catalog{
			ByProduct: domaintest.Products(), ByVatRate: domaintest.VatRates(),
			ByWarehouse: domaintest.Warehouses(), ByCustomer: domaintest.Customers(),
		},
		Stock:       &usecasetest.Stock{},
		Shipments:   usecasetest.NewShipments(),
		Renderer:    &usecasetest.Renderer{},
		Inflector:   &usecasetest.Inflector{},
		Health:      &usecasetest.Health{State: usecase.IntegrationHealth{State: usecase.StateDegraded, RetryAfter: 5 * time.Second}},
		Clock:       usecasetest.Clock{Fixed: time.Date(2026, 9, 13, 11, 0, 0, 0, time.UTC)},
		ReadTimeout: time.Second, WriteTimeout: time.Second,
	})
	h := NewServer(implementation)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/status", nil))

	require.Equal(t, http.StatusOK, w.Code)
	var got statusResponseDTO
	require.NoError(t, json.NewDecoder(w.Body).Decode(&got))
	assert.Equal(t, string(usecase.StateDegraded), got.State)
	assert.Equal(t, 5, got.RetryAfterSeconds)
}

func TestOpenAPISpec_ServesYAML(t *testing.T) {
	h, _ := testServer(t)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/openapi.yaml", nil))

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "yaml")
	assert.Contains(t, w.Body.String(), "openapi:")
}
