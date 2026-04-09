#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LOCAL_ENV_FILE="${ROOT_DIR}/.localdev/env.sh"

if [[ -f "${LOCAL_ENV_FILE}" ]]; then
  # shellcheck source=/dev/null
  source "${LOCAL_ENV_FILE}"
fi

SITE_ID="${ENERGY_SITE_ID:-site-cl-01}"
PARTIAL_RACK_ID="${ENERGY_PARTIAL_RACK_ID:-rack-cl-01-01}"
PARTIAL_DELTA_KW="${ENERGY_PARTIAL_DELTA_KW:-120}"
ENERGY_API_URL="${ENERGY_API_URL:-http://localhost:8081}"
INGEST_API_URL="${INGEST_API_URL:-http://localhost:8080}"
NATS_MONITOR_URL="${NATS_MONITOR_URL:-http://localhost:8222}"
API_KEY="${API_KEY:-}"
API_KEY_HEADER="${API_KEY_HEADER:-X-API-Key}"
TIMESCALEDB_CONTAINER="${TIMESCALEDB_CONTAINER:-fractsoul-timescaledb}"
NATS_CONTAINER="${NATS_CONTAINER:-fractsoul-nats}"
INGEST_CONTAINER="${INGEST_CONTAINER:-fractsoul-ingest-api}"
ENERGY_CONTAINER="${ENERGY_CONTAINER:-fractsoul-energy-orchestrator}"
DB_USER="${POSTGRES_USER:-postgres}"
DB_NAME="${POSTGRES_DB:-mining}"
SKIP_BUILD="${E2E_SKIP_BUILD:-false}"

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT

usage() {
  cat <<'EOF'
Usage: ./scripts/e2e_energy_orchestrator.sh [--skip-build]

Environment overrides:
  ENERGY_SITE_ID
  ENERGY_PARTIAL_RACK_ID
  ENERGY_PARTIAL_DELTA_KW
  ENERGY_API_URL
  INGEST_API_URL
  NATS_MONITOR_URL
  API_KEY
  API_KEY_HEADER
  E2E_SKIP_BUILD=true
EOF
}

for arg in "$@"; do
  case "${arg}" in
    --skip-build)
      SKIP_BUILD="true"
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: ${arg}" >&2
      usage >&2
      exit 1
      ;;
  esac
done

require_cmd() {
  local cmd="$1"
  if ! command -v "${cmd}" >/dev/null 2>&1; then
    echo "Missing required command: ${cmd}" >&2
    exit 1
  fi
}

log_step() {
  echo
  echo "[$1] $2"
}

wait_for_health() {
  local container="$1"
  local timeout_seconds="${2:-180}"
  local waited=0
  local status=""

  while (( waited < timeout_seconds )); do
    status="$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "${container}" 2>/dev/null || true)"
    if [[ "${status}" == "healthy" || "${status}" == "running" ]]; then
      return 0
    fi
    sleep 2
    waited=$((waited + 2))
  done

  echo "Container ${container} did not become healthy within ${timeout_seconds}s." >&2
  docker logs --tail 50 "${container}" >&2 || true
  exit 1
}

curl_json() {
  local method="$1"
  local url="$2"
  local payload="${3:-}"
  local -a args

  args=(-fsS -X "${method}" -H "Content-Type: application/json")
  if [[ -n "${API_KEY}" ]]; then
    args+=(-H "${API_KEY_HEADER}: ${API_KEY}")
  fi
  if [[ -n "${payload}" ]]; then
    args+=(--data "${payload}")
  fi

  curl "${args[@]}" "${url}"
}

curl_get() {
  local url="$1"
  local -a args

  args=(-fsS)
  if [[ -n "${API_KEY}" ]]; then
    args+=(-H "${API_KEY_HEADER}: ${API_KEY}")
  fi

  curl "${args[@]}" "${url}"
}

psql_scalar() {
  local sql="$1"
  docker exec "${TIMESCALEDB_CONTAINER}" psql -tA -U "${DB_USER}" -d "${DB_NAME}" -c "${sql}" | tr -d '[:space:]'
}

energy_stream_state_field() {
  local field="$1"
  python3 -c '
import json
import sys

field = sys.argv[1]
data = json.load(sys.stdin)

for account in data.get("account_details", []):
    for stream in account.get("stream_detail", []):
        if stream.get("name") == "ENERGY":
            value = stream.get("state", {}).get(field)
            if value is None:
                raise SystemExit(f"ENERGY stream field missing: {field}")
            print(value)
            raise SystemExit(0)

raise SystemExit("ENERGY stream not found in NATS monitor output")
' "${field}"
}

require_cmd docker
require_cmd curl
require_cmd python3

log_step "1/7" "Starting minimal Docker stack for energy-orchestrator"
if [[ "${SKIP_BUILD}" == "true" ]]; then
  docker compose up -d timescaledb nats ingest-api energy-orchestrator
else
  docker compose up -d --build timescaledb nats ingest-api energy-orchestrator
fi

log_step "2/7" "Waiting for container health"
wait_for_health "${TIMESCALEDB_CONTAINER}"
wait_for_health "${NATS_CONTAINER}"
wait_for_health "${INGEST_CONTAINER}"
wait_for_health "${ENERGY_CONTAINER}"

curl_get "${INGEST_API_URL}/healthz" >/dev/null
curl_get "${ENERGY_API_URL}/healthz" >/dev/null

log_step "3/7" "Applying schema and synthetic seeds"
"${ROOT_DIR}/scripts/bootstrap_timescaledb.sh"
"${ROOT_DIR}/scripts/seed_synthetic_data.sh"

SNAPSHOTS_BEFORE="$(psql_scalar "SELECT COUNT(*) FROM energy_budget_snapshots;")"
ENERGY_MESSAGES_BEFORE="$(curl -fsS "${NATS_MONITOR_URL}/jsz?streams=true&config=true" | energy_stream_state_field messages)"

log_step "4/7" "Calling budget endpoint with fractsoul context"
BUDGET_FILE="${TMP_DIR}/budget.json"
curl_get "${ENERGY_API_URL}/v1/energy/sites/${SITE_ID}/budget?include_context=true" > "${BUDGET_FILE}"
IFS='|' read -r BUDGET_SNAPSHOT_ID BUDGET_SAFE_KW BUDGET_AVAILABLE_KW < <(
  python3 - "${BUDGET_FILE}" "${SITE_ID}" <<'PY'
import json
import sys

path = sys.argv[1]
site_id = sys.argv[2]
with open(path, "r", encoding="utf-8") as handle:
    payload = json.load(handle)

budget = payload.get("budget", {})
context = payload.get("fractsoul_context") or {}

assert payload.get("snapshot_id"), "budget response missing snapshot_id"
assert budget.get("site_id") == site_id, "budget response has unexpected site_id"
assert float(budget.get("safe_capacity_kw", 0)) > 0, "safe_capacity_kw must be positive"
assert float(budget.get("available_capacity_kw", 0)) > 0, "available_capacity_kw must be positive"
assert context.get("source") == "fractsoul-ingest-api", "missing fractsoul context"

print(f"{payload['snapshot_id']}|{budget['safe_capacity_kw']}|{budget['available_capacity_kw']}")
PY
)

log_step "5/7" "Calling advisory accepted dispatch validation"
DISPATCH_ACCEPT_FILE="${TMP_DIR}/dispatch_accepted.json"
curl_json \
  "POST" \
  "${ENERGY_API_URL}/v1/energy/sites/${SITE_ID}/dispatch/validate?include_context=true" \
  "$(cat "${ROOT_DIR}/docs/contracts/energy_dispatch_validate_request_v1.example.json")" \
  > "${DISPATCH_ACCEPT_FILE}"
IFS='|' read -r ACCEPT_SNAPSHOT_ID ACCEPT_STATUS ACCEPT_DECISIONS < <(
  python3 - "${DISPATCH_ACCEPT_FILE}" <<'PY'
import json
import sys

with open(sys.argv[1], "r", encoding="utf-8") as handle:
    payload = json.load(handle)

result = payload.get("result", {})
decisions = result.get("decisions", [])

assert payload.get("snapshot_id"), "accepted dispatch missing snapshot_id"
assert result.get("summary_status") == "accepted", "accepted dispatch should be fully accepted"
assert decisions, "accepted dispatch returned no decisions"
assert all(decision.get("status") == "accepted" for decision in decisions), "all accepted dispatch decisions must be accepted"

print(f"{payload['snapshot_id']}|{result['summary_status']}|{len(decisions)}")
PY
)

log_step "6/7" "Calling advisory partial dispatch validation"
PARTIAL_REQUEST_FILE="${TMP_DIR}/dispatch_partial_request.json"
cat > "${PARTIAL_REQUEST_FILE}" <<EOF
{
  "requested_by": "ops@fractsoul.local",
  "ambient_celsius": 29.1,
  "requests": [
    {
      "rack_id": "${PARTIAL_RACK_ID}",
      "delta_kw": ${PARTIAL_DELTA_KW},
      "reason": "forced invalid dispatch for e2e verification"
    }
  ]
}
EOF

DISPATCH_PARTIAL_FILE="${TMP_DIR}/dispatch_partial.json"
curl_json \
  "POST" \
  "${ENERGY_API_URL}/v1/energy/sites/${SITE_ID}/dispatch/validate?include_context=true" \
  "$(cat "${PARTIAL_REQUEST_FILE}")" \
  > "${DISPATCH_PARTIAL_FILE}"
IFS='|' read -r PARTIAL_SNAPSHOT_ID PARTIAL_STATUS PARTIAL_ACCEPTED_KW PARTIAL_VIOLATION_CODE < <(
  python3 - "${DISPATCH_PARTIAL_FILE}" <<'PY'
import json
import sys

with open(sys.argv[1], "r", encoding="utf-8") as handle:
    payload = json.load(handle)

result = payload.get("result", {})
decisions = result.get("decisions", [])

assert payload.get("snapshot_id"), "partial dispatch missing snapshot_id"
assert result.get("summary_status") == "partial", "partial dispatch must return partial summary_status"
assert decisions, "partial dispatch returned no decisions"

decision = decisions[0]
violations = decision.get("violations", [])

assert decision.get("status") == "partial", "partial dispatch decision must be partial"
assert violations, "partial dispatch must include violations"
assert float(decision.get("accepted_delta_kw", 0)) > 0, "partial dispatch should retain some advisory headroom"

print(f"{payload['snapshot_id']}|{result['summary_status']}|{decision['accepted_delta_kw']}|{violations[0]['code']}")
PY
)

SNAPSHOTS_AFTER="$(psql_scalar "SELECT COUNT(*) FROM energy_budget_snapshots;")"
ENERGY_STREAM_JSON="${TMP_DIR}/energy_stream.json"
curl -fsS "${NATS_MONITOR_URL}/jsz?streams=true&config=true" > "${ENERGY_STREAM_JSON}"
ENERGY_MESSAGES_AFTER="$(energy_stream_state_field messages < "${ENERGY_STREAM_JSON}")"
ENERGY_SUBJECTS_AFTER="$(energy_stream_state_field num_subjects < "${ENERGY_STREAM_JSON}")"

SNAPSHOT_DELTA=$((SNAPSHOTS_AFTER - SNAPSHOTS_BEFORE))
MESSAGE_DELTA=$((ENERGY_MESSAGES_AFTER - ENERGY_MESSAGES_BEFORE))

if (( SNAPSHOT_DELTA < 3 )); then
  echo "Expected at least 3 new energy snapshots, got ${SNAPSHOT_DELTA}." >&2
  exit 1
fi

if (( MESSAGE_DELTA < 4 )); then
  echo "Expected at least 4 new ENERGY stream messages, got ${MESSAGE_DELTA}." >&2
  exit 1
fi

if (( ENERGY_SUBJECTS_AFTER < 2 )); then
  echo "Expected at least 2 ENERGY stream subjects after partial dispatch, got ${ENERGY_SUBJECTS_AFTER}." >&2
  exit 1
fi

log_step "7/7" "Validation summary"
echo "Budget snapshot:    ${BUDGET_SNAPSHOT_ID} (safe=${BUDGET_SAFE_KW} kW, available=${BUDGET_AVAILABLE_KW} kW)"
echo "Accepted dispatch: ${ACCEPT_SNAPSHOT_ID} (status=${ACCEPT_STATUS}, decisions=${ACCEPT_DECISIONS})"
echo "Partial dispatch:  ${PARTIAL_SNAPSHOT_ID} (status=${PARTIAL_STATUS}, accepted=${PARTIAL_ACCEPTED_KW} kW, code=${PARTIAL_VIOLATION_CODE})"
echo "New snapshots:     ${SNAPSHOT_DELTA}"
echo "New ENERGY msgs:   ${MESSAGE_DELTA}"
echo "ENERGY subjects:   ${ENERGY_SUBJECTS_AFTER}"
echo
echo "Latest energy snapshots:"
docker exec "${TIMESCALEDB_CONTAINER}" psql -U "${DB_USER}" -d "${DB_NAME}" -c "\
SELECT snapshot_id, source, policy_mode, created_at
FROM energy_budget_snapshots
ORDER BY created_at DESC
LIMIT 5;"
echo
echo "Energy orchestrator E2E completed successfully."
