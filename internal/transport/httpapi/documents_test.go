package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPreview_ContractFieldsMatchReference(t *testing.T) {
	h, _ := testServer(t)
	r := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/documents/preview", bytes.NewReader(validRequest()))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	body := w.Body.Bytes()
	var got previewResponseDTO
	require.NoError(t, json.Unmarshal(body, &got))
	assert.Equal(t, "Воронцова Петра Аркадьевича", got.Customer.FullNameGenitive)
	assert.Equal(t, "+79991234567", got.Customer.Phone)
	assert.Equal(t, "00ДМ-000101", got.Order.Number)
	assert.Equal(t, wireNumber("21181"), got.Totals.Amount)
	assert.Equal(t, wireNumber("3210.82"), got.Totals.VatAmount)
	assert.Equal(t, wireNumber("24391.82"), got.Totals.AmountWithVAT)
	assert.Contains(t, string(body), `"amount":3750`, "деньги на проводе — числа, как в примере задания, а не строки")
}

func TestPreview_InvalidReceiver_ReturnsFieldErrorWithoutOneCRead(t *testing.T) {
	h, orders := testServer(t)
	body := []byte(`{"orderRef":"2da145f0-d5fc-11f1-a0b3-48df371887e9","customer":{"lastName":"","firstName":"","phone":"oops","email":"bad"}}`)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/documents/preview", bytes.NewReader(body)))

	require.Equal(t, http.StatusUnprocessableEntity, w.Code)
	assert.Equal(t, 0, orders.Calls)
	var got Problem
	require.NoError(t, json.NewDecoder(w.Body).Decode(&got))
	assert.Equal(t, "VALIDATION_FAILED", got.Code)
	assert.Contains(t, got.Errors, "phone")
	assert.Contains(t, got.Errors, "email")
}

func TestPDF_UsesAttachmentAndPDFContentType(t *testing.T) {
	h, _ := testServer(t)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/documents/pdf", bytes.NewReader(validRequest())))

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/pdf", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Header().Get("Content-Disposition"), "filename*=UTF-8''")
	assert.Contains(t, w.Header().Get("Content-Disposition"), "000101.pdf")
	assert.Equal(t, []byte("%PDF-fake"), w.Body.Bytes())
}
