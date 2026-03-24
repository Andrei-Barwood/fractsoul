#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CONTAINER_NAME="${TIMESCALEDB_CONTAINER:-fractsoul-timescaledb}"
DB_USER="${POSTGRES_USER:-postgres}"
DB_NAME="${POSTGRES_DB:-mining}"
API_URL="${API_URL:-http://localhost:8080}"
API_KEY="${API_KEY:-}"
API_KEY_HEADER="${API_KEY_HEADER:-X-API-Key}"

if ! docker ps --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
  echo "${CONTAINER_NAME} is not running. Start compose first." >&2
  exit 1
fi

echo "[1/4] Applying migrations..."
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

EVENT1="$(new_event_id)"
EVENT2="$(new_event_id)"
NOW_UTC="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"

echo "[2/4] Sending deterministic alert events..."
PAYLOAD1="$(cat <<JSON
{
  "event_id": "${EVENT1}",
  "timestamp": "${NOW_UTC}",
  "site_id": "site-cl-01",
  "rack_id": "rack-cl-01-01",
  "miner_id": "asic-909001",
  "firmware_version": "e2e-alerts-2026.1",
  "metrics": {
    "hashrate_ths": 188.4,
    "power_watts": 3450.2,
    "temp_celsius": 101.1,
    "fan_rpm": 7190,
    "efficiency_jth": 18.3,
    "status": "critical"
  },
  "tags": {
    "source": "e2e-alerts-script",
    "asic_model": "S21"
  }
}
JSON
)"
PAYLOAD2="$(cat <<JSON
{
  "event_id": "${EVENT2}",
  "timestamp": "${NOW_UTC}",
  "site_id": "site-cl-01",
  "rack_id": "rack-cl-01-01",
  "miner_id": "asic-909001",
  "firmware_version": "e2e-alerts-2026.1",
  "metrics": {
    "hashrate_ths": 187.2,
    "power_watts": 3440.0,
    "temp_celsius": 100.5,
    "fan_rpm": 7165,
    "efficiency_jth": 18.4,
    "status": "critical"
  },
  "tags": {
    "source": "e2e-alerts-script",
    "asic_model": "S21"
  }
}
JSON
)"

curl_with_auth "POST" "${API_URL}/v1/telemetry/ingest" "${PAYLOAD1}" >/dev/null
sleep 1
curl_with_auth "POST" "${API_URL}/v1/telemetry/ingest" "${PAYLOAD2}" >/dev/null

echo "[3/4] Waiting for alert aggregation/dedup..."
ATTEMPTS=0
MAX_ATTEMPTS=20
ALERT_ROW=""
while (( ATTEMPTS < MAX_ATTEMPTS )); do
  ALERT_ROW="$(docker exec "${CONTAINER_NAME}" psql -tA -U "${DB_USER}" -d "${DB_NAME}" -c "\
SELECT occurrences || '|' || status
FROM alerts
WHERE miner_id = 'asic-909001' AND rule_id = 'overheat'
ORDER BY last_seen_at DESC
LIMIT 1;")"
  ALERT_ROW="${ALERT_ROW//[[:space:]]/}"
  if [[ -n "${ALERT_ROW}" ]]; then
    OCC="${ALERT_ROW%%|*}"
    STATUS="${ALERT_ROW##*|}"
    if [[ "${OCC}" =~ ^[0-9]+$ ]] && (( OCC >= 2 )); then
      echo "Alert state: occurrences=${OCC} status=${STATUS}"
      break
    fi
  fi
  ATTEMPTS=$((ATTEMPTS+1))
  sleep 1
done

if [[ -z "${ALERT_ROW}" ]]; then
  echo "E2E alerts failed: overheat alert not persisted." >&2
  exit 1
fi

OCC="${ALERT_ROW%%|*}"
STATUS="${ALERT_ROW##*|}"
if [[ "${OCC}" =~ ^[0-9]+$ ]] && (( OCC >= 2 )) && [[ "${STATUS}" == "suppressed" ]]; then
  :
else
  echo "E2E alerts failed: expected occurrences>=2 and status=suppressed, got ${ALERT_ROW}" >&2
  exit 1
fi

echo "[4/4] Alert notifications summary..."
NOTIFICATION_COUNT="$(docker exec "${CONTAINER_NAME}" psql -tA -U "${DB_USER}" -d "${DB_NAME}" -c "\
SELECT COUNT(*)
FROM alert_notifications
WHERE alert_id IN (SELECT alert_id FROM alerts WHERE miner_id = 'asic-909001');")"
NOTIFICATION_COUNT="${NOTIFICATION_COUNT//[[:space:]]/}"
echo "alert_notifications rows: ${NOTIFICATION_COUNT}"

echo "Alert flow E2E completed successfully."
