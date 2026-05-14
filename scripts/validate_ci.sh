#!/usr/bin/env bash
set -euo pipefail

workflow=".github/workflows/ci.yaml"

test -f "${workflow}"
test -f scripts/ci.sh

grep -q "quality:" "${workflow}"
grep -q "integration:" "${workflow}"
grep -q "docker-build:" "${workflow}"
grep -q "scripts/ci.sh test" "${workflow}"
grep -q "scripts/ci.sh vet" "${workflow}"
grep -q "scripts/ci.sh openapi" "${workflow}"
grep -q "scripts/ci.sh migrations" "${workflow}"
grep -q "scripts/ci.sh k8s" "${workflow}"
grep -q "scripts/ci.sh docker-build" "${workflow}"
grep -q "golangci-lint" "${workflow}"
