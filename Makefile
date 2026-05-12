APP_NAME := contentflow
DATABASE_URL ?= postgres://contentflow:contentflow@localhost:5432/contentflow?sslmode=disable

.PHONY: run dev tidy fmt test build compose-up compose-down compose-logs migrate-up migrate-down migrate-version migrate-force

compose-up:
	@docker compose -f deployments/docker-compose.yaml up -d

compose-down:
	@docker compose -f deployments/docker-compose.yaml down

compose-logs:
	@docker compose -f deployments/docker-compose.yaml logs -f

migrate-up:
	@migrate -path migrations -database "$(DATABASE_URL)" up

migrate-down:
	@migrate -path migrations -database "$(DATABASE_URL)" down 1

migrate-version:
	@migrate -path migrations -database "$(DATABASE_URL)" version

migrate-force:
	@test -n "$(VERSION)" || (echo "VERSION is required. Example: make migrate-force VERSION=1" && exit 1)
		migrate -path migrations -database "$(DATABASE_URL)" force $(VERSION)

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
