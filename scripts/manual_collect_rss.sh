#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
EMAIL="${EMAIL:-demo@example.com}"
PASSWORD="${PASSWORD:-password123}"
SOURCE_NAME="${SOURCE_NAME:-Go Blog}"
SOURCE_URL="${SOURCE_URL:-https://go.dev/blog/feed.atom}"

require_jq() {
  if ! command -v jq >/dev/null 2>&1; then
    echo "jq is required to parse API responses" >&2
    exit 1
  fi
}

login() {
  curl -fsS \
    -H "Content-Type: application/json" \
    -d "{\"email\":\"${EMAIL}\",\"password\":\"${PASSWORD}\"}" \
    "${BASE_URL}/api/v1/auth/login"
}

create_source() {
  curl -fsS \
    -H "Authorization: Bearer ${ACCESS_TOKEN}" \
    -H "Content-Type: application/json" \
    -d "{\"name\":\"${SOURCE_NAME}\",\"type\":\"rss\",\"url\":\"${SOURCE_URL}\",\"config\":{}}" \
    "${BASE_URL}/api/v1/sources"
}

collect_source() {
  curl -fsS \
    -X POST \
    -H "Authorization: Bearer ${ACCESS_TOKEN}" \
    "${BASE_URL}/api/v1/sources/${SOURCE_ID}/collect"
}

require_jq

if [[ -z "${ACCESS_TOKEN:-}" ]]; then
  ACCESS_TOKEN="$(login | jq -r '.data.access_token')"
fi

if [[ -z "${SOURCE_ID:-}" ]]; then
  SOURCE_ID="$(create_source | jq -r '.data.source.id')"
fi

collect_source | jq .
