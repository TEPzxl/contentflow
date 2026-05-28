#!/usr/bin/env bash
set -euo pipefail

export GOCACHE="${GOCACHE:-/tmp/contentflow-go-build-cache}"

GO_TEST_INTEGRATION_PACKAGES=(
  ./internal/module/article
  ./internal/ratelimit
  ./internal/module/source
  ./internal/module/collector
)

cmd="${1:-all}"

case "${cmd}" in
  tidy-check)
    go mod tidy
    git diff --exit-code go.mod go.sum
    ;;
  test)
    go test -count=1 ./...
    ;;
  coverage)
    mkdir -p coverage
    go test -count=1 -covermode=atomic -coverprofile=coverage/backend.out ./...
    go tool cover -func=coverage/backend.out | tee coverage/backend.txt
    ;;
  vet)
    go vet ./...
    ;;
  lint)
    golangci-lint run ./...
    ;;
  openapi)
    scripts/validate_openapi.sh
    ;;
  migrations)
    shopt -s nullglob
    for up in migrations/*.up.sql; do
      down="${up%.up.sql}.down.sql"
      test -s "${up}"
      test -s "${down}"
    done
    ;;
  k8s)
    scripts/validate_k8s.sh
    ;;
  web-audit)
    npm --prefix web audit --audit-level=moderate
    ;;
  web-typecheck)
    npm --prefix web run typecheck
    ;;
  web-lint)
    npm --prefix web run lint
    ;;
  web-test)
    npm --prefix web run test
    ;;
  web-build)
    npm --prefix web run build
    ;;
  benchmark)
    go test -run '^$' -bench=. -benchmem ./internal/module/article ./internal/module/collector/rss ./internal/module/collectionjob
    ;;
  smoke-api)
    scripts/smoke_api.sh
    ;;
  integration)
    go test -count=1 -p=1 -tags=integration "${GO_TEST_INTEGRATION_PACKAGES[@]}"
    ;;
  docker-build)
    GOPROXY="${GOPROXY:-https://proxy.golang.org,direct}" \
      docker compose -f deployments/docker-compose.yaml build \
      --build-arg GOPROXY="${GOPROXY:-https://proxy.golang.org,direct}" \
      backend
    docker compose -f deployments/docker-compose.yaml build frontend
    ;;
  all)
    "$0" tidy-check
    "$0" test
    "$0" coverage
    "$0" vet
    "$0" openapi
    "$0" migrations
    "$0" k8s
    "$0" web-audit
    "$0" web-typecheck
    "$0" web-lint
    "$0" web-build
    ;;
  *)
    echo "Usage: $0 [tidy-check|test|coverage|vet|lint|openapi|migrations|k8s|web-audit|web-typecheck|web-lint|web-test|web-build|benchmark|smoke-api|integration|docker-build|all]" >&2
    exit 1
    ;;
esac
