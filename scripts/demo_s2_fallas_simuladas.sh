#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CONTAINER_NAME="${TIMESCALEDB_CONTAINER:-fractsoul-timescaledb}"
DB_USER="${POSTGRES_USER:-postgres}"
DB_NAME="${POSTGRES_DB:-mining}"
API_URL="${API_URL:-http://localhost:8080}"
API_KEY="${API_KEY:-}"
API_KEY_HEADER="${API_KEY_HEADER:-X-API-Key}"
DEMO_SITE_ID="${DEMO_SITE_ID:-site-cl-01}"
DEMO_RACK_ID="${DEMO_RACK_ID:-rack-cl-01-01}"

if ! docker ps --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
  echo "${CONTAINER_NAME} is not running. Start compose first." >&2
  exit 1
fi

echo "[1/7] Applying migrations..."
"${ROOT_DIR}/scripts/migrate_timescaledb.sh"

curl_with_auth() {
  local method="$1"
  local endpoint="$2"
  local payload="$3"
  if [[ -n "${API_KEY}" ]]; then
    curl -fsS -X "${method}" -H "Content-Type: application/json" -H "${API_KEY_HEADER}: ${API_KEY}" "${endpoint}" -d "${payload}"
  else
    curl -fsS -X "${method}" -H "Content-Type: application/json" "${endpoint}" -d "${payload}"
  fi
}

get_with_auth() {
  local endpoint="$1"
  if [[ -n "${API_KEY}" ]]; then
    curl -fsS -H "${API_KEY_HEADER}: ${API_KEY}" "${endpoint}"
  else
    curl -fsS "${endpoint}"
  fi
}

new_event_id() {
  if command -v uuidgen >/dev/null 2>&1; then
    uuidgen | tr '[:upper:]' '[:lower:]'
  else
    python3 - <<'PY'
import uuid
print(str(uuid.uuid4()))
PY
  fi
}

build_payload() {
  local event_id="$1"
  local miner_id="$2"
  local hashrate="$3"
  local power="$4"
  local temp="$5"
  local fan="$6"
  local efficiency="$7"
  local status="$8"
  local model="$9"
  local source="${10}"
  local timestamp="${11}"
  cat <<JSON
{
  "event_id": "${event_id}",
  "timestamp": "${timestamp}",
  "site_id": "${DEMO_SITE_ID}",
  "rack_id": "${DEMO_RACK_ID}",
  "miner_id": "${miner_id}",
  "firmware_version": "demo-s2-2026.1",
  "metrics": {
    "hashrate_ths": ${hashrate},
    "power_watts": ${power},
    "temp_celsius": ${temp},
    "fan_rpm": ${fan},
    "efficiency_jth": ${efficiency},
    "status": "${status}"
  },
  "tags": {
    "source": "${source}",
    "asic_model": "${model}"
  }
}
JSON
}

echo "[2/7] Cleaning previous demo data (idempotent)..."
docker exec "${CONTAINER_NAME}" psql -U "${DB_USER}" -d "${DB_NAME}" -c "\
DELETE FROM alert_notifications
WHERE alert_id IN (
  SELECT alert_id FROM alerts WHERE miner_id IN ('asic-920001', 'asic-920002', 'asic-920003', 'asic-920004')
);

DELETE FROM alerts
WHERE miner_id IN ('asic-920001', 'asic-920002', 'asic-920003', 'asic-920004');

DELETE FROM telemetry_readings
WHERE miner_id IN ('asic-920001', 'asic-920002', 'asic-920003', 'asic-920004');

DELETE FROM miners
WHERE miner_id IN ('asic-920001', 'asic-920002', 'asic-920003', 'asic-920004');" >/dev/null

echo "[3/7] Injecting deterministic fault events..."
NOW_UTC="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
PLUS_1S="$(date -u -v+1S +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || python3 - <<'PY'
from datetime import datetime, timedelta, timezone
print((datetime.now(timezone.utc)+timedelta(seconds=1)).strftime("%Y-%m-%dT%H:%M:%SZ"))
PY
)"
PLUS_2S="$(date -u -v+2S +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || python3 - <<'PY'
from datetime import datetime, timedelta, timezone
print((datetime.now(timezone.utc)+timedelta(seconds=2)).strftime("%Y-%m-%dT%H:%M:%SZ"))
PY
)"
PLUS_3S="$(date -u -v+3S +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || python3 - <<'PY'
from datetime import datetime, timedelta, timezone
print((datetime.now(timezone.utc)+timedelta(seconds=3)).strftime("%Y-%m-%dT%H:%M:%SZ"))
PY
)"
PLUS_4S="$(date -u -v+4S +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || python3 - <<'PY'
from datetime import datetime, timedelta, timezone
print((datetime.now(timezone.utc)+timedelta(seconds=4)).strftime("%Y-%m-%dT%H:%M:%SZ"))
PY
)"

# Event A/B: overheat duplicate to show suppression.
EVENT_A="$(new_event_id)"
EVENT_B="$(new_event_id)"
PAYLOAD_A="$(build_payload "${EVENT_A}" "asic-920001" "188.4" "3451.0" "101.5" "7220" "18.3" "critical" "S21" "demo-overheat" "${NOW_UTC}")"
PAYLOAD_B="$(build_payload "${EVENT_B}" "asic-920001" "186.2" "3438.0" "100.8" "7190" "18.4" "critical" "S21" "demo-overheat" "${PLUS_1S}")"

# Event C: power spike.
EVENT_C="$(new_event_id)"
PAYLOAD_C="$(build_payload "${EVENT_C}" "asic-920002" "196.0" "5020.0" "84.0" "6400" "25.6" "warning" "S21" "demo-power-spike" "${PLUS_2S}")"

# Event D: hashrate drop.
EVENT_D="$(new_event_id)"
PAYLOAD_D="$(build_payload "${EVENT_D}" "asic-920003" "62.0" "3010.0" "78.0" "6050" "48.5" "warning" "S21" "demo-hash-drop" "${PLUS_3S}")"

# Event E: normal baseline (should not create alert).
EVENT_E="$(new_event_id)"
PAYLOAD_E="$(build_payload "${EVENT_E}" "asic-920004" "195.0" "3525.0" "73.0" "5980" "18.1" "ok" "S21" "demo-baseline" "${PLUS_4S}")"

curl_with_auth "POST" "${API_URL}/v1/telemetry/ingest" "${PAYLOAD_A}" >/dev/null
curl_with_auth "POST" "${API_URL}/v1/telemetry/ingest" "${PAYLOAD_B}" >/dev/null
curl_with_auth "POST" "${API_URL}/v1/telemetry/ingest" "${PAYLOAD_C}" >/dev/null
curl_with_auth "POST" "${API_URL}/v1/telemetry/ingest" "${PAYLOAD_D}" >/dev/null
curl_with_auth "POST" "${API_URL}/v1/telemetry/ingest" "${PAYLOAD_E}" >/dev/null

echo "[4/7] Waiting for alerts to be persisted..."
ATTEMPTS=0
MAX_ATTEMPTS=20
while (( ATTEMPTS < MAX_ATTEMPTS )); do
  READY="$(docker exec "${CONTAINER_NAME}" psql -tA -U "${DB_USER}" -d "${DB_NAME}" -c "\
SELECT COUNT(*)
FROM alerts
WHERE miner_id IN ('asic-920001', 'asic-920002', 'asic-920003')
  AND last_seen_at >= NOW() - INTERVAL '5 minutes';")"
  READY="${READY//[[:space:]]/}"
  if [[ "${READY}" =~ ^[0-9]+$ ]] && (( READY >= 3 )); then
    break
  fi
  ATTEMPTS=$((ATTEMPTS+1))
  sleep 1
done

if (( ATTEMPTS >= MAX_ATTEMPTS )); then
  echo "Demo failed: alerts did not persist in expected time window." >&2
  exit 1
fi

echo "[5/7] Demo evidence: alerts snapshot"
docker exec "${CONTAINER_NAME}" psql -U "${DB_USER}" -d "${DB_NAME}" -c "\
SELECT
  miner_id,
  rule_id,
  severity,
  status,
  occurrences,
  ROUND(metric_value::numeric, 2) AS metric_value,
  ROUND(threshold_value::numeric, 2) AS threshold_value,
  to_char(last_seen_at, 'YYYY-MM-DD HH24:MI:SS') AS last_seen_utc
FROM alerts
WHERE miner_id IN ('asic-920001', 'asic-920002', 'asic-920003')
ORDER BY miner_id, rule_id;"

echo "[6/7] Demo evidence: telemetry read API samples"
get_with_auth "${API_URL}/v1/telemetry/readings?miner_id=asic-920001&limit=2" | head -c 300; echo
get_with_auth "${API_URL}/v1/telemetry/readings?miner_id=asic-920002&limit=2" | head -c 300; echo
get_with_auth "${API_URL}/v1/telemetry/readings?miner_id=asic-920003&limit=2" | head -c 300; echo

echo "[7/7] Demo summary checks"
OVERHEAT_ROW="$(docker exec "${CONTAINER_NAME}" psql -tA -U "${DB_USER}" -d "${DB_NAME}" -c "\
SELECT occurrences || '|' || status
FROM alerts
WHERE miner_id = 'asic-920001' AND rule_id = 'overheat'
ORDER BY last_seen_at DESC
LIMIT 1;")"
OVERHEAT_ROW="${OVERHEAT_ROW//[[:space:]]/}"

if [[ "${OVERHEAT_ROW}" =~ ^[0-9]+\|[a-z_]+$ ]]; then
  OCC="${OVERHEAT_ROW%%|*}"
  STATUS="${OVERHEAT_ROW##*|}"
  if (( OCC >= 2 )) && [[ "${STATUS}" == "suppressed" ]]; then
    echo "OK dedupe/suppression: overheat occurrences=${OCC} status=${STATUS}"
  else
    echo "Demo warning: expected overheat dedupe suppression, got ${OVERHEAT_ROW}" >&2
    exit 1
  fi
else
  echo "Demo failed: unable to parse overheat row (${OVERHEAT_ROW})." >&2
  exit 1
fi

echo "Demo S2 completed successfully."
