// Package api хранит контракт HTTP API в одном месте и встраивает его в
// бинарник: спецификация всегда та же, что реально отдаёт сервис, а
// /swagger/ и /api/v1/openapi.yaml работают без доступа в интернет.
package api

import _ "embed"

//go:embed openapi.yaml
var OpenAPISpec []byte
