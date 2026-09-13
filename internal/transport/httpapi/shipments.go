package httpapi

import "net/http"

func (h *handler) shipmentDraft(w http.ResponseWriter, r *http.Request) {
	ref, err := parseOrderRef(r.PathValue("orderRef"))
	if err != nil {
		writeError(w, err)
		return
	}
	if r.Method == http.MethodGet {
		draft, err := h.shipmentRegistrar.FindShipmentDraft(r.Context(), ref)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, newShipmentDraftDTO(draft.Ref, draft.Number, draft.Date.Format("2006-01-02T15:04:05"), false))
		return
	}
	result, err := h.shipmentRegistrar.EnsureShipmentDraft(r.Context(), ref)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, newShipmentDraftDTO(result.Draft.Ref, result.Draft.Number, result.Draft.Date.Format("2006-01-02T15:04:05"), result.Created))
}
