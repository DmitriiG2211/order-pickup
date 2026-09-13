// Package httpapi — хендлеры HTTP API поверх сценариев usecase.
//
// Здесь и только здесь ошибки usecase и domain превращаются в HTTP-ответы
// по RFC 9457 (application/problem+json) с конкретным кодом (ADR 0011).
// Хендлер не принимает решений — вызывает сценарий и переводит результат.
package httpapi

import (
	"embed"
	"io/fs"

	"orderissue/internal/usecase"
)

//go:embed assets/*
var assets embed.FS

// swaggerUI — поддерево assets/swagger-ui как отдельная файловая система,
// чтобы отдавать её через http.FileServerFS без утечки остальных assets.
var swaggerUI = mustSub(assets, "assets/swagger-ui")

func mustSub(f embed.FS, dir string) fs.FS {
	sub, err := fs.Sub(f, dir)
	if err != nil {
		panic(err) // встроенные файлы — часть сборки, ошибка здесь означает баг в go:embed
	}
	return sub
}

// handler — точка входа HTTP. Четыре узких интерфейса usecase вместо одного
// широкого: каждой группе хендлеров (orders.go, documents.go, shipments.go,
// status.go) видно только то, что ей нужно, и компилятор это проверяет.
type handler struct {
	orderCatalog      usecase.OrderCatalog
	documentComposer  usecase.DocumentComposer
	shipmentRegistrar usecase.ShipmentRegistrar
	statusReporter    usecase.StatusReporter
}
