ifneq (,$(wildcard ./.env))
    include .env
    export
endif

.PHONY: generate migrate run test test-integration lint

generate:
	go tool oapi-codegen -config openapi.yaml contracts/openapi/trip-service.openapi.yaml
	go generate ./internal/usecase/...

migrate:
	go tool goose -dir ./migrations postgres "$(DATABASE_URL)" up

run:
	go run ./cmd/trip-service

test:
	go test -race ./...

test-integration:
	go test -race -count=2 -tags=integration ./internal/repository/...

lint:
	golangci-lint run ./...
