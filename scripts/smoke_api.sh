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
ACCESS_TOKEN="${ACCESS_TOKEN:-}"
SOURCE_ID="${SOURCE_ID:-}"
RUN_ID="${RUN_ID:-}"
ARTICLE_ID="${ARTICLE_ID:-}"

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
      RUN_ID="$(jq -er '.data.collection_runs[0].run_id' <<<"${response}")"
      jq -r '.data.collection_runs[0] | "run #\(.run_id) status=\(.status) fetched=\(.fetched_count)"' <<<"${response}"
      return 0
    fi
    sleep "${POLL_INTERVAL_SECONDS}"
  done

  echo "collection run did not appear after ${POLL_ATTEMPTS} attempts" >&2
  return 1
}

verify_source_paths() {
  print_step "checking source CRUD paths"
  api GET "/api/v1/sources/${SOURCE_ID}" "" "${ACCESS_TOKEN}" |
    jq -e --argjson source_id "${SOURCE_ID}" '.data.source.id == $source_id' >/dev/null
  api PATCH "/api/v1/sources/${SOURCE_ID}" "$(jq -n --arg name "${SOURCE_NAME} Updated" '{name:$name}')" "${ACCESS_TOKEN}" |
    jq -e --arg name "${SOURCE_NAME} Updated" '.data.source.name == $name' >/dev/null
}

verify_collection_run_detail() {
  if [[ -z "${RUN_ID}" ]]; then
    print_step "collection run id missing; skipping run detail check"
    return 0
  fi
  print_step "checking collection run detail"
  api GET "/api/v1/collection-runs/${RUN_ID}" "" "${ACCESS_TOKEN}" |
    jq -e --argjson run_id "${RUN_ID}" '.data.collection_run.run_id == $run_id' >/dev/null
}

verify_article_paths() {
  print_step "checking article read/save paths"
  if [[ -z "${ARTICLE_ID}" ]]; then
    ARTICLE_ID="$(api GET "/api/v1/articles?limit=1" "" "${ACCESS_TOKEN}" | jq -r '.data.articles[0].id // empty')"
  fi
  if [[ -z "${ARTICLE_ID}" ]]; then
    print_step "no article available; skipping article state checks"
    return 0
  fi

  api GET "/api/v1/articles/${ARTICLE_ID}" "" "${ACCESS_TOKEN}" |
    jq -e --argjson article_id "${ARTICLE_ID}" '.data.article.id == $article_id' >/dev/null
  api PATCH "/api/v1/articles/${ARTICLE_ID}/read" '{"is_read":true}' "${ACCESS_TOKEN}" |
    jq -e '.data.article.is_read == true' >/dev/null
  api PATCH "/api/v1/articles/${ARTICLE_ID}/save" '{"is_saved":true}' "${ACCESS_TOKEN}" |
    jq -e '.data.article.is_saved == true' >/dev/null
}

verify_read_paths() {
  print_step "checking authenticated read paths"
  api GET "/api/v1/me" "" "${ACCESS_TOKEN}" | jq -e '.data.user.email | length > 0' >/dev/null
  api GET "/api/v1/sources?limit=10" "" "${ACCESS_TOKEN}" | jq -e '.data.sources | length >= 1' >/dev/null
  api GET "/api/v1/articles?limit=10" "" "${ACCESS_TOKEN}" | jq -e '.data.articles | type == "array"' >/dev/null
  api POST "/api/v1/ai/rag-search" '{"query":"smoke","limit":3}' "${ACCESS_TOKEN}" | jq -e '.data.answer.answer | length > 0' >/dev/null
  verify_source_paths
  verify_collection_run_detail
  verify_article_paths
  verify_dlq_path
}

expect_status() {
  local method="$1"
  local path="$2"
  local want_status="$3"
  local body="${4:-}"
  local body_file status
  body_file="$(mktemp)"

  local args=(-sS -o "${body_file}" -w "%{http_code}" -X "${method}" -H "Accept: application/json")
  args+=(-H "Authorization: Bearer ${ACCESS_TOKEN}")
  if [[ -n "${body}" ]]; then
    args+=(-H "Content-Type: application/json" -d "${body}")
  fi
  status="$(curl "${args[@]}" "${BASE_URL}${path}")"

  if [[ "${status}" != "${want_status}" ]]; then
    cat "${body_file}" >&2
    echo "unexpected status for ${method} ${path}: got ${status}, want ${want_status}" >&2
    rm -f "${body_file}"
    return 1
  fi
  rm -f "${body_file}"
}

verify_dlq_path() {
  print_step "checking DLQ path"
  local body_file status
  body_file="$(mktemp)"
  status="$(curl -sS -o "${body_file}" -w "%{http_code}" \
    -H "Accept: application/json" \
    -H "Authorization: Bearer ${ACCESS_TOKEN}" \
    "${BASE_URL}/api/v1/collection-dlq?status=pending&limit=10")"

  case "${status}" in
    200)
      jq -e '.data.items | type == "array"' "${body_file}" >/dev/null
      expect_status POST "/api/v1/collection-dlq/999999999/replay" 404
      expect_status POST "/api/v1/collection-dlq/999999999/handled" 404
      ;;
    404)
      print_step "DLQ route not registered; skipping DLQ check"
      ;;
    *)
      cat "${body_file}" >&2
      echo "unexpected DLQ status ${status}" >&2
      rm -f "${body_file}"
      return 1
      ;;
  esac
  rm -f "${body_file}"
}

cleanup_source() {
	print_step "deleting smoke source"
	api DELETE "/api/v1/sources/${SOURCE_ID}" "" "${ACCESS_TOKEN}" >/dev/null
}

cleanup_source_if_created() {
	if [[ -n "${ACCESS_TOKEN}" && -n "${SOURCE_ID}" ]]; then
		cleanup_source || true
	fi
}

require_tools
health_check
register_user
ACCESS_TOKEN="$(login_user)"
SOURCE_ID="$(create_source)"
trap cleanup_source_if_created EXIT
collect_source
wait_for_collection_run
verify_read_paths

print_step "smoke API checks passed"
