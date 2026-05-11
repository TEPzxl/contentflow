APP_NAME := contentflow

.PHONY: run dev tidy fmt test build compose-up compose-down compose-logs

compose-up:
	@docker compose -f deployments/docker-compose.yaml up -d

compose-down:
	@docker compose -f deployments/docker-compose.yaml down

compose-logs:
	@docker compose -f deployments/docker-compose.yaml logs -f

run:
	@go run ./cmd/server

dev:
	@go run ./cmd/server

tidy:
	@go mod tidy

fmt:
	@go fmt ./...

test:
	@go test ./...

build:
	@go build -o $(APP_NAME) ./cmd/server
