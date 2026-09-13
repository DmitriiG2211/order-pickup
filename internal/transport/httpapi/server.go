package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"orderissue/internal/usecase"
)

// NewServer assembles the public HTTP surface.  It deliberately has no
// persistence: receiver data lives only for the duration of a request.
func NewServer(api usecase.API) http.Handler {
	h := &handler{orderCatalog: api, documentComposer: api, shipmentRegistrar: api, statusReporter: api}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", h.index)
	mux.HandleFunc("GET /assets/", h.asset)
	mux.HandleFunc("GET /healthz", h.healthz)
	mux.HandleFunc("GET /api/v1/status", h.status)
	mux.HandleFunc("GET /api/v1/orders", h.orders)
	mux.HandleFunc("GET /api/v1/orders/{orderRef}", h.order)
	mux.HandleFunc("GET /api/v1/orders/{orderRef}/shipment-draft", h.shipmentDraft)
	mux.HandleFunc("PUT /api/v1/orders/{orderRef}/shipment-draft", h.shipmentDraft)
	mux.HandleFunc("POST /api/documents/preview", h.preview)
	mux.HandleFunc("POST /api/v1/documents/preview", h.preview)
	mux.HandleFunc("POST /api/v1/documents/pdf", h.pdf)
	mux.HandleFunc("GET /api/v1/openapi.yaml", h.openapiSpec)
	mux.Handle("GET /swagger/", http.StripPrefix("/swagger/", http.FileServerFS(swaggerUI)))
	return withNoStore(mux)
}

func (h *handler) index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	serveAsset(w, r, "assets/index.html", "text/html; charset=utf-8")
}

func (h *handler) asset(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/")
	if name != "assets/app.js" && name != "assets/style.css" {
		http.NotFound(w, r)
		return
	}
	contentType := "application/javascript; charset=utf-8"
	if strings.HasSuffix(name, ".css") {
		contentType = "text/css; charset=utf-8"
	}
	serveAsset(w, r, name, contentType)
}

func serveAsset(w http.ResponseWriter, r *http.Request, name, contentType string) {
	b, err := assets.ReadFile(name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(b) //nolint:gosec // name is checked against a fixed allow-list before this call, not attacker-controlled
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func withNoStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}
