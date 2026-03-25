#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
API_URL="${API_URL:-http://localhost:8080}"
API_KEY="${API_KEY:-}"
API_KEY_HEADER="${API_KEY_HEADER:-X-API-Key}"
TIMESCALEDB_CONTAINER="${TIMESCALEDB_CONTAINER:-fractsoul-timescaledb}"
NATS_CONTAINER="${NATS_CONTAINER:-fractsoul-nats}"
INGEST_CONTAINER="${INGEST_CONTAINER:-fractsoul-ingest-api}"
DB_USER="${POSTGRES_USER:-postgres}"
DB_NAME="${POSTGRES_DB:-mining}"

wait_for_http() {
  local retries=60
  while (( retries > 0 )); do
    if curl -fsS "${API_URL}/healthz" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
    retries=$((retries - 1))
  done
  echo "Timed out waiting for ${API_URL}/healthz" >&2
  return 1
}

wait_for_container_health() {
  local container="$1"
  local retries=60
  while (( retries > 0 )); do
    health="$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' "${container}" 2>/dev/null || true)"
    if [[ "${health}" == "healthy" || "${health}" == "none" ]]; then
      return 0
    fi
    sleep 1
    retries=$((retries - 1))
  done
  echo "Timed out waiting for ${container} health" >&2
  return 1
}

readings_count() {
  docker exec "${TIMESCALEDB_CONTAINER}" psql -tA -U "${DB_USER}" -d "${DB_NAME}" -c "SELECT COUNT(*) FROM telemetry_readings;"
}

send_event() {
  local event_id timestamp payload status
  event_id="$(uuidgen | tr '[:upper:]' '[:lower:]')"
  timestamp="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  payload="$(cat <<JSON
{
  "event_id": "${event_id}",
  "timestamp": "${timestamp}",
  "site_id": "site-cl-01",
  "rack_id": "rack-cl-01-01",
  "miner_id": "asic-000001",
  "firmware_version": "braiins-2026.1",
  "metrics": {
    "hashrate_ths": 126.5,
    "power_watts": 3325,
    "temp_celsius": 71.2,
    "fan_rpm": 6400,
    "efficiency_jth": 26.3,
    "status": "ok"
  },
  "tags": {
    "pool": "mainnet",
    "site_zone": "north"
  }
}
JSON
)"

  for attempt in {1..10}; do
    cmd=(
      curl -s -o /tmp/resilience_ingest_resp.json -w "%{http_code}" -X POST
      "${API_URL}/v1/telemetry/ingest"
      -H "Content-Type: application/json"
      --data "${payload}"
    )
    if [[ -n "${API_KEY}" ]]; then
      cmd+=( -H "${API_KEY_HEADER}: ${API_KEY}" )
    fi

    status="$("${cmd[@]}")"
    if [[ "${status}" == "202" ]]; then
      echo "${event_id}"
      return 0
    fi

    sleep 2
  done

  echo "Failed to ingest event after retries. Last response: $(cat /tmp/resilience_ingest_resp.json 2>/dev/null || true)" >&2
  return 1
}

wait_for_count_at_least() {
  local expected="$1"
  local retries=60
  while (( retries > 0 )); do
    current="$(readings_count)"
    if (( current >= expected )); then
      echo "${current}"
      return 0
    fi
    sleep 1
    retries=$((retries - 1))
  done
  echo "Timed out waiting telemetry_readings >= ${expected}" >&2
  return 1
}

echo "[D85] Starting resilience restart/failover smoke test..."
wait_for_container_health "${TIMESCALEDB_CONTAINER}"
wait_for_container_health "${NATS_CONTAINER}"
wait_for_container_health "${INGEST_CONTAINER}"
wait_for_http

baseline_count="$(readings_count)"
echo "Baseline telemetry_readings: ${baseline_count}"

echo "Step 1/3: restart ingest-api"
docker compose restart ingest-api >/dev/null
wait_for_container_health "${INGEST_CONTAINER}"
wait_for_http
send_event >/dev/null
baseline_count="$(wait_for_count_at_least $((baseline_count + 1)))"
echo "telemetry_readings after ingest-api restart: ${baseline_count}"

echo "Step 2/3: restart nats"
docker compose restart nats >/dev/null
wait_for_container_health "${NATS_CONTAINER}"
wait_for_http
send_event >/dev/null
baseline_count="$(wait_for_count_at_least $((baseline_count + 1)))"
echo "telemetry_readings after nats restart: ${baseline_count}"

echo "Step 3/3: restart timescaledb"
docker compose restart timescaledb >/dev/null
wait_for_container_health "${TIMESCALEDB_CONTAINER}"
wait_for_http
send_event >/dev/null
final_count="$(wait_for_count_at_least $((baseline_count + 1)))"
echo "telemetry_readings after timescaledb restart: ${final_count}"

echo "[D85] Resilience test passed. Final telemetry_readings: ${final_count}"
