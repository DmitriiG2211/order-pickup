package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOrders_ListsSeededOrder(t *testing.T) {
	h, _ := testServer(t)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/orders", nil))

	require.Equal(t, http.StatusOK, w.Code)
	var got []orderSummaryDTO
	require.NoError(t, json.NewDecoder(w.Body).Decode(&got))
	require.Len(t, got, 1)
	assert.Equal(t, "00ДМ-000101", got[0].Number)
	assert.True(t, got[0].Issuable)
}

func TestOrder_ReturnsSheetWithStockAndTotals(t *testing.T) {
	h, _ := testServer(t)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/orders/2da145f0-d5fc-11f1-a0b3-48df371887e9", nil))

	require.Equal(t, http.StatusOK, w.Code)
	var got sheetResponseDTO
	require.NoError(t, json.NewDecoder(w.Body).Decode(&got))
	assert.Equal(t, "00ДМ-000101", got.Order.Number)
	require.Len(t, got.Lines, 4)
	assert.Equal(t, wireNumber("21181"), got.Totals.Amount)
}

func TestOrder_UnknownRef_ReturnsNotFoundProblem(t *testing.T) {
	h, _ := testServer(t)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/orders/00000000-0000-0000-0000-000000000000", nil))

	require.Equal(t, http.StatusNotFound, w.Code)
	var got Problem
	require.NoError(t, json.NewDecoder(w.Body).Decode(&got))
	assert.Equal(t, "ORDER_NOT_FOUND", got.Code)
}

func TestOrder_InvalidRefFormat_ReturnsBadRequest(t *testing.T) {
	h, _ := testServer(t)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/orders/not-a-guid", nil))

	require.Equal(t, http.StatusBadRequest, w.Code)
	var got Problem
	require.NoError(t, json.NewDecoder(w.Body).Decode(&got))
	assert.Equal(t, "INVALID_ORDER_REF", got.Code)
}
