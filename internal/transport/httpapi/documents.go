package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"

	"orderissue/internal/usecase"
)

func (h *handler) preview(w http.ResponseWriter, r *http.Request) {
	req, ok := h.documentRequest(w, r)
	if !ok {
		return
	}
	doc, err := h.documentComposer.PreviewDocument(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, newPreviewResponse(doc))
}

func (h *handler) pdf(w http.ResponseWriter, r *http.Request) {
	req, ok := h.documentRequest(w, r)
	if !ok {
		return
	}
	pdf, doc, err := h.documentComposer.PrintDocument(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	// filename* сохраняет кириллицу и делает имя уникальным для каждого заказа.
	w.Header().Set("Content-Disposition", "attachment; filename*=UTF-8''"+url.PathEscape(doc.OrderNumber+".pdf"))
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(pdf)
}

func (h *handler) documentRequest(w http.ResponseWriter, r *http.Request) (usecase.DocumentRequest, bool) {
	defer func() { _ = r.Body.Close() }()
	var body previewRequestDTO
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	if err := dec.Decode(&body); err != nil {
		writeProblem(w, Problem{Status: http.StatusBadRequest, Code: "INVALID_JSON", Title: "Неверное тело запроса", Detail: "Ожидается JSON с orderRef и customer"})
		return usecase.DocumentRequest{}, false
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeProblem(w, Problem{Status: http.StatusBadRequest, Code: "INVALID_JSON", Title: "Неверное тело запроса", Detail: "В теле запроса должен быть один JSON-объект"})
		return usecase.DocumentRequest{}, false
	}
	ref, err := parseOrderRef(body.OrderRef)
	if err != nil {
		writeError(w, err)
		return usecase.DocumentRequest{}, false
	}
	return usecase.DocumentRequest{OrderRef: ref, Receiver: body.Customer.toDomain()}, true
}
