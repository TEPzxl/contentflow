#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
EMAIL="${EMAIL:-smoke-$(date +%s)-$RANDOM@example.com}"
PASSWORD="${PASSWORD:-SmokePass123}"
DISPLAY_NAME="${DISPLAY_NAME:-Smoke User}"
SOURCE_NAME="${SOURCE_NAME:-Smoke Empty Mailbox}"
SOURCE_TYPE="${SOURCE_TYPE:-email}"
SOURCE_URL="${SOURCE_URL:-}"
SOURCE_CONFIG="${SOURCE_CONFIG:-{\"provider\":\"empty\"}}"
POLL_ATTEMPTS="${POLL_ATTEMPTS:-15}"
POLL_INTERVAL_SECONDS="${POLL_INTERVAL_SECONDS:-1}"

require_tools() {
  for tool in curl jq; do
    if ! command -v "${tool}" >/dev/null 2>&1; then
      echo "${tool} is required" >&2
      exit 1
    fi
  done
}

json_string() {
  jq -Rn --arg value "$1" '$value'
}

api() {
  local method="$1"
  local path="$2"
  local body="${3:-}"
  local token="${4:-}"
  local args=(-fsS -X "${method}" -H "Accept: application/json")

  if [[ -n "${token}" ]]; then
    args+=(-H "Authorization: Bearer ${token}")
  fi
  if [[ -n "${body}" ]]; then
    args+=(-H "Content-Type: application/json" -d "${body}")
  fi

  curl "${args[@]}" "${BASE_URL}${path}"
}

print_step() {
  printf '==> %s\n' "$1" >&2
}

health_check() {
  print_step "checking health endpoints"
  curl -fsS "${BASE_URL}/healthz" >/dev/null
  curl -fsS "${BASE_URL}/readyz" >/dev/null
}

register_user() {
  print_step "registering smoke user ${EMAIL}"
  api POST "/api/v1/auth/register" "$(jq -n \
    --arg email "${EMAIL}" \
    --arg password "${PASSWORD}" \
    --arg display_name "${DISPLAY_NAME}" \
    '{email:$email,password:$password,display_name:$display_name}')" >/dev/null
}

login_user() {
  print_step "logging in"
  api POST "/api/v1/auth/login" "$(jq -n \
    --arg email "${EMAIL}" \
    --arg password "${PASSWORD}" \
    '{email:$email,password:$password}')" |
    jq -er '.data.access_token'
}

create_source() {
  print_step "creating ${SOURCE_TYPE} source"

  local url_json
  if [[ -n "${SOURCE_URL}" ]]; then
    url_json="$(json_string "${SOURCE_URL}")"
  else
    url_json="null"
  fi

  jq -e . >/dev/null <<<"${SOURCE_CONFIG}"

  api POST "/api/v1/sources" "$(jq -n \
    --arg name "${SOURCE_NAME}" \
    --arg type "${SOURCE_TYPE}" \
    --argjson url "${url_json}" \
    --argjson config "${SOURCE_CONFIG}" \
    '{name:$name,type:$type,url:$url,config:$config}')" "${ACCESS_TOKEN}" |
    jq -er '.data.source.id'
}

collect_source() {
  print_step "triggering collection for source ${SOURCE_ID}"
  api POST "/api/v1/sources/${SOURCE_ID}/collect" "" "${ACCESS_TOKEN}" |
    tee /tmp/contentflow-smoke-collect.json |
    jq -e '(.data.collection_run.status // .data.collection_task.status) | length > 0' >/dev/null
}

wait_for_collection_run() {
  print_step "waiting for collection run"
  local attempt
  for ((attempt = 1; attempt <= POLL_ATTEMPTS; attempt++)); do
    local response
    response="$(api GET "/api/v1/sources/${SOURCE_ID}/collection-runs?limit=1" "" "${ACCESS_TOKEN}")"
    if jq -e '.data.total > 0' >/dev/null <<<"${response}"; then
      jq -r '.data.collection_runs[0] | "run #\(.run_id) status=\(.status) fetched=\(.fetched_count)"' <<<"${response}"
      return 0
    fi
    sleep "${POLL_INTERVAL_SECONDS}"
  done

  echo "collection run did not appear after ${POLL_ATTEMPTS} attempts" >&2
  return 1
}

verify_read_paths() {
  print_step "checking authenticated read paths"
  api GET "/api/v1/me" "" "${ACCESS_TOKEN}" | jq -e '.data.user.email | length > 0' >/dev/null
  api GET "/api/v1/sources?limit=10" "" "${ACCESS_TOKEN}" | jq -e '.data.sources | length >= 1' >/dev/null
  api GET "/api/v1/articles?limit=10" "" "${ACCESS_TOKEN}" | jq -e '.data.articles | type == "array"' >/dev/null
  api POST "/api/v1/ai/rag-search" '{"query":"smoke","limit":3}' "${ACCESS_TOKEN}" | jq -e '.data.answer.answer | length > 0' >/dev/null
  api GET "/api/v1/collection-dlq?status=pending&limit=10" "" "${ACCESS_TOKEN}" | jq -e '.data.items | type == "array"' >/dev/null
}

cleanup_source() {
  print_step "deleting smoke source"
  api DELETE "/api/v1/sources/${SOURCE_ID}" "" "${ACCESS_TOKEN}" >/dev/null
}

require_tools
health_check
register_user
ACCESS_TOKEN="$(login_user)"
SOURCE_ID="$(create_source)"
collect_source
wait_for_collection_run
verify_read_paths
cleanup_source

print_step "smoke API checks passed"
