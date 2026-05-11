APP_NAME := contentflow

.PHONY: run
run:
	@go run ./cmd/server

.PHONY: tidy
tidy:
	@go mod tidy

.PHONY: fmt
fmt:
	@go fmt ./...

.PHONY: test
test:
	@go test ./...

.PHONY: build
build:
	@go build -o $(APP_NAME) ./cmd/server
