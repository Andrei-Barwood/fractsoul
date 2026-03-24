#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CONTAINER_NAME="${TIMESCALEDB_CONTAINER:-fractsoul-timescaledb}"
DB_USER="${POSTGRES_USER:-postgres}"
DB_NAME="${POSTGRES_DB:-mining}"
API_URL="${API_URL:-http://localhost:8080}"
API_KEY="${API_KEY:-}"
API_KEY_HEADER="${API_KEY_HEADER:-X-API-Key}"
SIM_MINERS="${SIM_MINERS:-100}"
SIM_DURATION="${SIM_DURATION:-20s}"
SIM_TICK="${SIM_TICK:-2s}"
SIM_CONCURRENCY="${SIM_CONCURRENCY:-24}"
SIM_API_KEY="${SIM_API_KEY:-${API_KEY}}"

if ! docker ps --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
  echo "${CONTAINER_NAME} is not running. Start compose first." >&2
  exit 1
fi

echo "[1/5] Applying migrations..."
"${ROOT_DIR}/scripts/migrate_timescaledb.sh"

echo "[2/5] Loading synthetic seed..."
"${ROOT_DIR}/scripts/seed_synthetic_data.sh"

echo "[3/5] Running simulator..."
(
  cd "${ROOT_DIR}/backend/services/ingest-api"
  if [[ -n "${SIM_API_KEY}" ]]; then
    go run ./cmd/simulator \
      -api-url "${API_URL}" \
      -api-key "${SIM_API_KEY}" \
      -miners "${SIM_MINERS}" \
      -duration "${SIM_DURATION}" \
      -tick "${SIM_TICK}" \
      -concurrency "${SIM_CONCURRENCY}"
  else
    go run ./cmd/simulator \
      -api-url "${API_URL}" \
      -miners "${SIM_MINERS}" \
      -duration "${SIM_DURATION}" \
      -tick "${SIM_TICK}" \
      -concurrency "${SIM_CONCURRENCY}"
  fi
)

echo "[4/5] Validating DB persistence..."
ROW_COUNT="$(docker exec "${CONTAINER_NAME}" psql -tA -U "${DB_USER}" -d "${DB_NAME}" -c "SELECT COUNT(*) FROM telemetry_readings WHERE ts >= NOW() - INTERVAL '30 minutes';")"
ROW_COUNT="${ROW_COUNT//[[:space:]]/}"

echo "Rows in telemetry_readings (last 30 min): ${ROW_COUNT}"
if [[ "${ROW_COUNT}" =~ ^[0-9]+$ ]] && (( ROW_COUNT > 0 )); then
  :
else
  echo "E2E failed: no telemetry persisted." >&2
  exit 1
fi

echo "[5/5] Validating read API..."
curl_with_auth() {
  local endpoint="$1"
  if [[ -n "${API_KEY}" ]]; then
    curl -fsS -H "${API_KEY_HEADER}: ${API_KEY}" "${endpoint}"
  else
    curl -fsS "${endpoint}"
  fi
}

READINGS_RESPONSE="$(curl_with_auth "${API_URL}/v1/telemetry/readings?limit=5")"
SUMMARY_RESPONSE="$(curl_with_auth "${API_URL}/v1/telemetry/summary?window_minutes=30")"
RACK_RESPONSE="$(curl_with_auth "${API_URL}/v1/telemetry/sites/site-cl-01/racks/rack-cl-01-01/readings?limit=5")"
NOW_UTC="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
FROM_UTC="$(date -u -v-2H +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || python3 - <<'PY'
from datetime import datetime, timedelta, timezone
print((datetime.now(timezone.utc)-timedelta(hours=2)).strftime("%Y-%m-%dT%H:%M:%SZ"))
PY
)"
TIMESERIES_RESPONSE="$(curl_with_auth "${API_URL}/v1/telemetry/miners/asic-000001/timeseries?resolution=minute&from=${FROM_UTC}&to=${NOW_UTC}&limit=120")"

echo "Readings response snippet: ${READINGS_RESPONSE:0:200}"
echo "Summary response snippet: ${SUMMARY_RESPONSE:0:200}"
echo "Rack response snippet: ${RACK_RESPONSE:0:200}"
echo "Timeseries response snippet: ${TIMESERIES_RESPONSE:0:200}"

if ! grep -q '"count":' <<<"${READINGS_RESPONSE}"; then
  echo "E2E failed: read API readings endpoint returned unexpected payload." >&2
  exit 1
fi

if ! grep -q '"samples":' <<<"${SUMMARY_RESPONSE}"; then
  echo "E2E failed: read API summary endpoint returned unexpected payload." >&2
  exit 1
fi

if ! grep -q '"count":' <<<"${RACK_RESPONSE}"; then
  echo "E2E failed: rack endpoint returned unexpected payload." >&2
  exit 1
fi

if ! grep -q '"count":' <<<"${TIMESERIES_RESPONSE}"; then
  echo "E2E failed: timeseries endpoint returned unexpected payload." >&2
  exit 1
fi

echo "E2E validation completed successfully."
