package api_test

import (
	"strings"
	"testing"

	"orderissue/api"

	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
)

// specDoc — ровно то подмножество структуры OpenAPI, которое нужно тесту:
// список путей и методов. Полная валидация схемы уже делается отдельно
// (python -m openapi_spec_validator при подготовке спецификации).
type specDoc struct {
	OpenAPI string                          `yaml:"openapi"`
	Paths   map[string]map[string]yaml.Node `yaml:"paths"`
}

func loadSpec(t *testing.T) specDoc {
	t.Helper()
	var doc specDoc
	require.NoError(t, yaml.Unmarshal(api.OpenAPISpec, &doc))
	return doc
}

func TestOpenAPISpec_IsWellFormedYAMLWithVersion3(t *testing.T) {
	// Act
	doc := loadSpec(t)

	// Assert
	require.Equal(t, "3.0.3", doc.OpenAPI)
	require.NotEmpty(t, doc.Paths, "в спецификации должен быть хотя бы один путь")
}

func TestOpenAPISpec_ListsEveryRouteTheServerActuallyRegisters(t *testing.T) {
	// Arrange: тот же список маршрутов, что регистрирует NewServer (server.go).
	// Path-параметры называем так же, как в спецификации, чтобы sameShape видел совпадение.
	registered := []struct{ method, path string }{
		{"GET", "/healthz"},
		{"GET", "/api/v1/status"},
		{"GET", "/api/v1/orders"},
		{"GET", "/api/v1/orders/{orderRef}"},
		{"GET", "/api/v1/orders/{orderRef}/shipment-draft"},
		{"PUT", "/api/v1/orders/{orderRef}/shipment-draft"},
		{"POST", "/api/documents/preview"},
		{"POST", "/api/v1/documents/preview"},
		{"POST", "/api/v1/documents/pdf"},
	}

	// Act
	doc := loadSpec(t)

	// Assert: каждый реальный маршрут описан, и в спецификации нет лишних путей —
	// расхождение в любую сторону означает, что код и документация разъехались.
	described := map[string]bool{}
	for path, methods := range doc.Paths {
		for method := range methods {
			described[strings.ToUpper(method)+" "+path] = true
		}
	}
	for _, r := range registered {
		key := r.method + " " + r.path
		require.Truef(t, described[key], "маршрут %s зарегистрирован в server.go, но не описан в openapi.yaml", key)
		delete(described, key)
	}
	require.Emptyf(t, described, "в openapi.yaml описаны маршруты, которых нет в server.go: %v", described)
}
