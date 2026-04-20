#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LOCAL_ENV_FILE="${ROOT_DIR}/.localdev/env.sh"

if [[ -f "${LOCAL_ENV_FILE}" ]]; then
  # shellcheck source=/dev/null
  source "${LOCAL_ENV_FILE}"
fi

SITE_ID="${ENERGY_SITE_ID:-site-cl-01}"
OPERATIONAL_SITE_ID="${ENERGY_OPERATIONAL_SITE_ID:-site-cl-02}"
PARTIAL_RACK_ID="${ENERGY_PARTIAL_RACK_ID:-rack-cl-01-01}"
PARTIAL_DELTA_KW="${ENERGY_PARTIAL_DELTA_KW:-120}"
REPLAY_DAY="${ENERGY_REPLAY_DAY:-$(date -u +%F)}"
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
  ENERGY_OPERATIONAL_SITE_ID
  ENERGY_PARTIAL_RACK_ID
  ENERGY_PARTIAL_DELTA_KW
  ENERGY_REPLAY_DAY
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

log_step "1/8" "Starting minimal Docker stack for energy-orchestrator"
if [[ "${SKIP_BUILD}" == "true" ]]; then
  docker compose up -d timescaledb nats ingest-api energy-orchestrator
else
  docker compose up -d --build timescaledb nats ingest-api energy-orchestrator
fi

log_step "2/8" "Waiting for container health"
wait_for_health "${TIMESCALEDB_CONTAINER}"
wait_for_health "${NATS_CONTAINER}"
wait_for_health "${INGEST_CONTAINER}"
wait_for_health "${ENERGY_CONTAINER}"

curl_get "${INGEST_API_URL}/healthz" >/dev/null
curl_get "${ENERGY_API_URL}/healthz" >/dev/null

log_step "3/8" "Applying schema and synthetic seeds"
"${ROOT_DIR}/scripts/bootstrap_timescaledb.sh"
"${ROOT_DIR}/scripts/seed_synthetic_data.sh"

SNAPSHOTS_BEFORE="$(psql_scalar "SELECT COUNT(*) FROM energy_budget_snapshots;")"
ENERGY_MESSAGES_BEFORE="$(curl -fsS "${NATS_MONITOR_URL}/jsz?streams=true&config=true" | energy_stream_state_field messages)"

log_step "4/8" "Calling budget endpoint with fractsoul context"
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

log_step "5/8" "Calling operational view, campus overview and read endpoints"
OPERATIONS_FILE="${TMP_DIR}/operations.json"
curl_get "${ENERGY_API_URL}/v1/energy/sites/${OPERATIONAL_SITE_ID}/operations?include_context=true" > "${OPERATIONS_FILE}"
IFS='|' read -r OPERATIONS_SNAPSHOT_ID OPERATIONS_CONSTRAINTS OPERATIONS_RECOMMENDATIONS OPERATIONS_BLOCKED OPERATIONS_EXPLANATIONS REVIEW_SNAPSHOT_ID REVIEW_RECOMMENDATION_ID REVIEW_RACK_ID REVIEW_ACTION REVIEW_CRITICALITY REVIEW_REQUESTED_DELTA REVIEW_RECOMMENDED_DELTA REVIEW_REASON < <(
  python3 - "${OPERATIONS_FILE}" "${OPERATIONAL_SITE_ID}" <<'PY'
import json
import sys

with open(sys.argv[1], "r", encoding="utf-8") as handle:
    payload = json.load(handle)

view = payload.get("view", {})
context = payload.get("fractsoul_context") or {}

assert payload.get("snapshot_id"), "operations endpoint missing snapshot_id"
assert view.get("site_id") == sys.argv[2], "operations endpoint has unexpected site_id"
assert isinstance(view.get("active_constraints") or [], list), "operations endpoint missing active_constraints list"
assert isinstance(view.get("pending_recommendations") or [], list), "operations endpoint missing pending_recommendations list"
assert isinstance(view.get("blocked_actions") or [], list), "operations endpoint missing blocked_actions list"
assert isinstance(view.get("explanations") or [], list), "operations endpoint missing explanations list"
assert context.get("source") == "fractsoul-ingest-api", "operations endpoint missing fractsoul context"

recommendations = view.get("pending_recommendations") or []
selected = next((item for item in recommendations if item.get("action") == "isolate"), recommendations[0] if recommendations else None)
assert selected, "operations endpoint should return at least one pending recommendation for governance validation"

print(
    f"{payload['snapshot_id']}|"
    f"{len(view.get('active_constraints') or [])}|"
    f"{len(view.get('pending_recommendations') or [])}|"
    f"{len(view.get('blocked_actions') or [])}|"
    f"{len(view.get('explanations') or [])}|"
    f"{payload['snapshot_id']}|"
    f"{selected.get('recommendation_id', '')}|"
    f"{selected.get('rack_id', '')}|"
    f"{selected.get('action', '')}|"
    f"{selected.get('criticality_class', '')}|"
    f"{selected.get('requested_delta_kw', 0)}|"
    f"{selected.get('recommended_delta_kw', 0)}|"
    f"{selected.get('reason', '')}"
)
PY
)

CAMPUS_FILE="${TMP_DIR}/campus_overview.json"
curl_get "${ENERGY_API_URL}/v1/energy/overview" > "${CAMPUS_FILE}"
DASHBOARD_FILE="${TMP_DIR}/dashboard_energy.html"
curl -fsS "${ENERGY_API_URL}/dashboard/energy/" > "${DASHBOARD_FILE}"

CONSTRAINTS_FILE="${TMP_DIR}/constraints.json"
curl_get "${ENERGY_API_URL}/v1/energy/sites/${OPERATIONAL_SITE_ID}/constraints/active" > "${CONSTRAINTS_FILE}"
RECOMMENDATIONS_FILE="${TMP_DIR}/recommendations.json"
curl_get "${ENERGY_API_URL}/v1/energy/sites/${OPERATIONAL_SITE_ID}/recommendations/pending" > "${RECOMMENDATIONS_FILE}"
BLOCKED_FILE="${TMP_DIR}/blocked_actions.json"
curl_get "${ENERGY_API_URL}/v1/energy/sites/${OPERATIONAL_SITE_ID}/actions/blocked" > "${BLOCKED_FILE}"
EXPLANATIONS_FILE="${TMP_DIR}/explanations.json"
curl_get "${ENERGY_API_URL}/v1/energy/sites/${OPERATIONAL_SITE_ID}/explanations" > "${EXPLANATIONS_FILE}"
REPLAY_FILE="${TMP_DIR}/replay.json"
curl_get "${ENERGY_API_URL}/v1/energy/sites/${OPERATIONAL_SITE_ID}/replay/historical?day=${REPLAY_DAY}" > "${REPLAY_FILE}"
IFS='|' read -r CONSTRAINT_COUNT RECOMMENDATION_COUNT BLOCKED_COUNT EXPLANATION_COUNT REPLAY_SCENARIOS REPLAY_ALERTS < <(
  python3 - "${CONSTRAINTS_FILE}" "${RECOMMENDATIONS_FILE}" "${BLOCKED_FILE}" "${EXPLANATIONS_FILE}" "${REPLAY_FILE}" <<'PY'
import json
import sys

with open(sys.argv[1], "r", encoding="utf-8") as handle:
    constraints = json.load(handle)
with open(sys.argv[2], "r", encoding="utf-8") as handle:
    recommendations = json.load(handle)
with open(sys.argv[3], "r", encoding="utf-8") as handle:
    blocked = json.load(handle)
with open(sys.argv[4], "r", encoding="utf-8") as handle:
    explanations = json.load(handle)
with open(sys.argv[5], "r", encoding="utf-8") as handle:
    replay = json.load(handle)

active_constraints = constraints.get("active_constraints") or []
pending_recommendations = recommendations.get("pending_recommendations") or []
blocked_actions = blocked.get("blocked_actions") or []
decision_explanations = explanations.get("explanations") or []
replay_result = replay.get("result") or {}
scenarios = replay_result.get("scenarios") or []

assert constraints.get("snapshot_id"), "constraints endpoint missing snapshot_id"
assert recommendations.get("snapshot_id"), "recommendations endpoint missing snapshot_id"
assert blocked.get("snapshot_id"), "blocked actions endpoint missing snapshot_id"
assert explanations.get("snapshot_id"), "explanations endpoint missing snapshot_id"
assert isinstance(active_constraints, list), "constraints endpoint must return a list"
assert isinstance(pending_recommendations, list), "recommendations endpoint must return a list"
assert isinstance(blocked_actions, list), "blocked actions endpoint must return a list"
assert isinstance(decision_explanations, list), "explanations endpoint must return a list"
assert len(scenarios) >= 2, "historical replay must return at least two scenarios"

print(
    f"{len(active_constraints)}|"
    f"{len(pending_recommendations)}|"
    f"{len(blocked_actions)}|"
    f"{len(decision_explanations)}|"
    f"{len(scenarios)}|"
    f"{replay_result.get('observed_persisted_alerts', 0)}"
)
PY
)

SHADOW_FILE="${TMP_DIR}/shadow.json"
curl_get "${ENERGY_API_URL}/v1/energy/sites/${OPERATIONAL_SITE_ID}/pilot/shadow?day=${REPLAY_DAY}" > "${SHADOW_FILE}"
IFS='|' read -r CAMPUS_SITE_COUNT CAMPUS_FIRST_SITE SHADOW_EVALUATED SHADOW_BLOCKED SHADOW_GAPS DASHBOARD_TITLE_OK < <(
  python3 - "${CAMPUS_FILE}" "${SHADOW_FILE}" "${DASHBOARD_FILE}" <<'PY'
import json
import sys

with open(sys.argv[1], "r", encoding="utf-8") as handle:
    campus = json.load(handle)
with open(sys.argv[2], "r", encoding="utf-8") as handle:
    shadow = json.load(handle)
with open(sys.argv[3], "r", encoding="utf-8") as handle:
    dashboard_html = handle.read()

overview = campus.get("overview") or {}
sites = overview.get("sites") or []
shadow_result = shadow.get("result") or {}

assert len(sites) >= 1, "campus overview should return at least one visible site"
assert isinstance(shadow_result.get("summary") or [], list), "shadow pilot must return summary lines"
assert shadow_result.get("recommendations_evaluated", 0) >= 1, "shadow pilot should evaluate at least one recommendation"
assert "Console Operativa de Campus" in dashboard_html, "dashboard HTML should expose the energy console title"

print(
    f"{len(sites)}|"
    f"{sites[0].get('site_id', '')}|"
    f"{shadow_result.get('recommendations_evaluated', 0)}|"
    f"{shadow_result.get('decisions_blocked', 0)}|"
    f"{shadow_result.get('missing_data_count', 0)}|"
    f"yes"
)
PY
)

REVIEW_REQUEST_FILE="${TMP_DIR}/review_request.json"
REVIEW_REQUEST_UNIQUE_ID="${REVIEW_RECOMMENDATION_ID}-e2e-$(date +%s)"
cat > "${REVIEW_REQUEST_FILE}" <<EOF
{
  "snapshot_id": "${REVIEW_SNAPSHOT_ID}",
  "recommendation_id": "${REVIEW_REQUEST_UNIQUE_ID}",
  "rack_id": "${REVIEW_RACK_ID}",
  "action": "${REVIEW_ACTION}",
  "criticality_class": "${REVIEW_CRITICALITY}",
  "requested_delta_kw": ${REVIEW_REQUESTED_DELTA},
  "recommended_delta_kw": ${REVIEW_RECOMMENDED_DELTA},
  "reason": "${REVIEW_REASON}",
  "decision": "approve",
  "comment": "first approval created by automated e2e validation"
}
EOF

REVIEW_POST_FILE="${TMP_DIR}/review_post.json"
curl_json \
  "POST" \
  "${ENERGY_API_URL}/v1/energy/sites/${OPERATIONAL_SITE_ID}/recommendations/reviews" \
  "$(cat "${REVIEW_REQUEST_FILE}")" \
  > "${REVIEW_POST_FILE}"
REVIEW_LIST_FILE="${TMP_DIR}/review_list.json"
curl_get "${ENERGY_API_URL}/v1/energy/sites/${OPERATIONAL_SITE_ID}/recommendations/reviews" > "${REVIEW_LIST_FILE}"
IFS='|' read -r REVIEW_STATUS REVIEW_EVENT_TYPE REVIEW_LIST_COUNT REVIEW_EVENT_COUNT < <(
  python3 - "${REVIEW_POST_FILE}" "${REVIEW_LIST_FILE}" "${REVIEW_REQUEST_UNIQUE_ID}" <<'PY'
import json
import sys

with open(sys.argv[1], "r", encoding="utf-8") as handle:
    posted = json.load(handle)
with open(sys.argv[2], "r", encoding="utf-8") as handle:
    reviews = json.load(handle)

review = posted.get("review") or {}
event = posted.get("event") or {}
items = reviews.get("items") or []
target = next((item for item in items if item.get("recommendation_id") == sys.argv[3]), None)

assert review.get("status") == "pending_second_approval", "sensitive review should remain pending second approval"
assert event.get("event_type") == "awaiting_second_approval", "first review event should request a second approver"
assert target is not None, "review list should include the posted recommendation review"
assert len(target.get("events") or []) >= 1, "review timeline should include at least one event"

print(
    f"{review.get('status', '')}|"
    f"{event.get('event_type', '')}|"
    f"{len(items)}|"
    f"{len(target.get('events') or [])}"
)
PY
)

log_step "6/8" "Calling advisory accepted dispatch validation"
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

log_step "7/8" "Calling advisory partial dispatch validation"
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

if (( SNAPSHOT_DELTA < 8 )); then
  echo "Expected at least 8 new energy snapshots, got ${SNAPSHOT_DELTA}." >&2
  exit 1
fi

if (( MESSAGE_DELTA < 9 )); then
  echo "Expected at least 9 new ENERGY stream messages, got ${MESSAGE_DELTA}." >&2
  exit 1
fi

if (( ENERGY_SUBJECTS_AFTER < 2 )); then
  echo "Expected at least 2 ENERGY stream subjects after partial dispatch, got ${ENERGY_SUBJECTS_AFTER}." >&2
  exit 1
fi

log_step "8/8" "Validation summary"
echo "Budget snapshot:    ${BUDGET_SNAPSHOT_ID} (safe=${BUDGET_SAFE_KW} kW, available=${BUDGET_AVAILABLE_KW} kW)"
echo "Operations view:    ${OPERATIONS_SNAPSHOT_ID} (constraints=${OPERATIONS_CONSTRAINTS}, recommendations=${OPERATIONS_RECOMMENDATIONS}, blocked=${OPERATIONS_BLOCKED}, explanations=${OPERATIONS_EXPLANATIONS})"
echo "Campus overview:    sites=${CAMPUS_SITE_COUNT}, first_site=${CAMPUS_FIRST_SITE}, dashboard=${DASHBOARD_TITLE_OK}"
echo "Read endpoints:     constraints=${CONSTRAINT_COUNT}, recommendations=${RECOMMENDATION_COUNT}, blocked=${BLOCKED_COUNT}, explanations=${EXPLANATION_COUNT}"
echo "Historical replay:  site=${OPERATIONAL_SITE_ID}, day=${REPLAY_DAY}, scenarios=${REPLAY_SCENARIOS}, persisted_alerts=${REPLAY_ALERTS}"
echo "Shadow pilot:       evaluated=${SHADOW_EVALUATED}, blocked=${SHADOW_BLOCKED}, gaps=${SHADOW_GAPS}"
echo "Review workflow:    status=${REVIEW_STATUS}, event=${REVIEW_EVENT_TYPE}, items=${REVIEW_LIST_COUNT}, timeline_events=${REVIEW_EVENT_COUNT}"
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
