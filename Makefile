ifneq (,$(wildcard ./.env))
    include .env
    export
endif

.PHONY: generate migrate run

generate:
	go tool oapi-codegen -config openapi.yaml contracts/openapi/trip-service.openapi.yaml

migrate:
	go tool goose -dir ./migrations postgres "$(DATABASE_URL)" up

run:
	go run ./cmd/trip-service
