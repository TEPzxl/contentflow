#!/usr/bin/env bash
set -euo pipefail

workflow=".github/workflows/ci.yaml"

test -f "${workflow}"
test -f scripts/ci.sh

grep -q "quality:" "${workflow}"
grep -q "integration:" "${workflow}"
grep -q "docker-build:" "${workflow}"
grep -q "scripts/ci.sh test" "${workflow}"
grep -q "scripts/ci.sh coverage" "${workflow}"
grep -q "scripts/ci.sh vet" "${workflow}"
grep -q "scripts/ci.sh web-audit" "${workflow}"
grep -q "scripts/ci.sh web-typecheck" "${workflow}"
grep -q "scripts/ci.sh web-lint" "${workflow}"
grep -q "scripts/ci.sh web-build" "${workflow}"
grep -q "scripts/ci.sh web-test" "${workflow}"
grep -q "scripts/ci.sh openapi" "${workflow}"
grep -q "scripts/ci.sh migrations" "${workflow}"
grep -q "scripts/ci.sh k8s" "${workflow}"
grep -q "scripts/ci.sh docker-build" "${workflow}"
grep -q "golangci-lint" "${workflow}"
grep -q "actions/setup-node" "${workflow}"
grep -q "playwright install" "${workflow}"
grep -q "backend-coverage" "${workflow}"
