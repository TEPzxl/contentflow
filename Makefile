APP_NAME := contentflow
DATABASE_URL ?= postgres://contentflow:contentflow@localhost:5432/contentflow?sslmode=disable

.PHONY: run dev tidy fmt test build compose-up compose-down compose-logs migrate-up migrate-down migrate-version migrate-force \
	mock test-integration

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

mock:
	@mockgen -source=internal/module/user/repository.go -destination=internal/module/user/mocks/repository_mock.go -package=usermocks
	@mockgen -source=internal/module/auth/refresh_token_repository.go -destination=internal/module/auth/mocks/refresh_token_repository_mock.go -package=authmocks
	@mockgen -source=internal/module/auth/token.go -destination=internal/module/auth/mocks/token_mock.go -package=authmocks
	@mockgen -source=internal/module/auth/service.go -destination=internal/module/auth/mocks/service_mock.go -package=authmocks
	@mockgen -source=internal/module/source/service.go -destination=internal/module/source/mocks/service_mock.go -package=sourcemocks
	@mockgen -source=internal/module/source/repository.go -destination=internal/module/source/mocks/repository_mock.go -package=sourcemocks


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

test-integration:
	@go test ./... -tags=integration -v

build:
	@go build -o $(APP_NAME) ./cmd/server
