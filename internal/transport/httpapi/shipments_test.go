package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"orderissue/internal/domain/domaintest"
	"orderissue/internal/domain/order"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShipmentDraft_Get_NoneYet_ReturnsNotFound(t *testing.T) {
	h, _ := testServer(t)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/orders/2da145f0-d5fc-11f1-a0b3-48df371887e9/shipment-draft", nil))

	require.Equal(t, http.StatusNotFound, w.Code)
	var got Problem
	require.NoError(t, json.NewDecoder(w.Body).Decode(&got))
	assert.Equal(t, "SHIPMENT_DRAFT_NOT_FOUND", got.Code)
}

func TestShipmentDraft_Put_CreatesThenIsIdempotent(t *testing.T) {
	h, _ := testServer(t)
	path := "/api/v1/orders/2da145f0-d5fc-11f1-a0b3-48df371887e9/shipment-draft"

	w1 := httptest.NewRecorder()
	h.ServeHTTP(w1, httptest.NewRequestWithContext(context.Background(), http.MethodPut, path, nil))
	require.Equal(t, http.StatusOK, w1.Code)
	var first shipmentDraftDTO
	require.NoError(t, json.NewDecoder(w1.Body).Decode(&first))
	assert.True(t, first.Created)
	assert.NotEmpty(t, first.Ref)

	w2 := httptest.NewRecorder()
	h.ServeHTTP(w2, httptest.NewRequestWithContext(context.Background(), http.MethodPut, path, nil))
	require.Equal(t, http.StatusOK, w2.Code)
	var second shipmentDraftDTO
	require.NoError(t, json.NewDecoder(w2.Body).Decode(&second))
	assert.False(t, second.Created, "второй PUT не должен создавать новый черновик")
	assert.Equal(t, first.Ref, second.Ref)

	w3 := httptest.NewRecorder()
	h.ServeHTTP(w3, httptest.NewRequestWithContext(context.Background(), http.MethodGet, path, nil))
	require.Equal(t, http.StatusOK, w3.Code)
	var found shipmentDraftDTO
	require.NoError(t, json.NewDecoder(w3.Body).Decode(&found))
	assert.Equal(t, first.Ref, found.Ref)
	assert.False(t, found.Created)
}

func TestShipmentDraft_BlockedOrder_ReturnsConflict(t *testing.T) {
	h, _ := testServerFor(t, map[order.Ref]order.Order{domaintest.Order105: domaintest.Order(domaintest.Order105)})
	w := httptest.NewRecorder()

	h.ServeHTTP(w, httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/v1/orders/ce6ca2b8-31c8-11f1-a0b1-48df371887e9/shipment-draft", nil))

	require.Equal(t, http.StatusConflict, w.Code)
	var got Problem
	require.NoError(t, json.NewDecoder(w.Body).Decode(&got))
	assert.Equal(t, "ORDER_NOT_ISSUABLE", got.Code)
}
