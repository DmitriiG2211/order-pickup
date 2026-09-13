# Команды продублированы в README: на Windows make обычно не установлен.
GOLANGCI_LINT ?= golangci-lint

.PHONY: run test lint fmt build

run:            ## сервис против 1С из .env
	go run ./cmd/server

test:           ## unit-тесты
	go test ./...

lint:
	$(GOLANGCI_LINT) run ./...

fmt:
	$(GOLANGCI_LINT) fmt ./...

build:
	go build -o bin/server ./cmd/server
