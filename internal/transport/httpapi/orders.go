package httpapi

import "net/http"

func (h *handler) orders(w http.ResponseWriter, r *http.Request) {
	items, err := h.orderCatalog.ListOrders(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	result := make([]orderSummaryDTO, len(items))
	for i, item := range items {
		result[i] = newOrderSummary(item)
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *handler) order(w http.ResponseWriter, r *http.Request) {
	ref, err := parseOrderRef(r.PathValue("orderRef"))
	if err != nil {
		writeError(w, err)
		return
	}
	sheet, err := h.orderCatalog.GetOrder(r.Context(), ref)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, newSheetResponse(sheet))
}
