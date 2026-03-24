#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CONTAINER_NAME="${TIMESCALEDB_CONTAINER:-fractsoul-timescaledb}"
DB_USER="${POSTGRES_USER:-postgres}"
DB_NAME="${POSTGRES_DB:-mining}"
API_URL="${API_URL:-http://localhost:8080}"
SIM_MINERS="${SIM_MINERS:-100}"
SIM_DURATION="${SIM_DURATION:-20s}"
SIM_TICK="${SIM_TICK:-2s}"
SIM_CONCURRENCY="${SIM_CONCURRENCY:-24}"

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
  go run ./cmd/simulator \
    -api-url "${API_URL}" \
    -miners "${SIM_MINERS}" \
    -duration "${SIM_DURATION}" \
    -tick "${SIM_TICK}" \
    -concurrency "${SIM_CONCURRENCY}"
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
READINGS_RESPONSE="$(curl -fsS "${API_URL}/v1/telemetry/readings?limit=5")"
SUMMARY_RESPONSE="$(curl -fsS "${API_URL}/v1/telemetry/summary?window_minutes=30")"

echo "Readings response snippet: ${READINGS_RESPONSE:0:200}"
echo "Summary response snippet: ${SUMMARY_RESPONSE:0:200}"

if ! grep -q '"count":' <<<"${READINGS_RESPONSE}"; then
  echo "E2E failed: read API readings endpoint returned unexpected payload." >&2
  exit 1
fi

if ! grep -q '"samples":' <<<"${SUMMARY_RESPONSE}"; then
  echo "E2E failed: read API summary endpoint returned unexpected payload." >&2
  exit 1
fi

echo "E2E validation completed successfully."
