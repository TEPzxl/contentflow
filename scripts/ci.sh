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
  integration)
    go test -count=1 -tags=integration "${GO_TEST_INTEGRATION_PACKAGES[@]}"
    ;;
  docker-build)
    GOPROXY="${GOPROXY:-https://proxy.golang.org,direct}" \
      docker compose -f deployments/docker-compose.yaml build \
      --build-arg GOPROXY="${GOPROXY:-https://proxy.golang.org,direct}" \
      backend
    ;;
  all)
    "$0" tidy-check
    "$0" test
    "$0" vet
    "$0" openapi
    "$0" migrations
    "$0" k8s
    ;;
  *)
    echo "Usage: $0 [tidy-check|test|vet|lint|openapi|migrations|k8s|integration|docker-build|all]" >&2
    exit 1
    ;;
esac
