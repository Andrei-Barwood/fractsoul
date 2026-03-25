#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MODULE_DIR="${ROOT_DIR}/backend/services/ingest-api"
API_URL="${API_URL:-http://localhost:8080}"
TIMESCALEDB_CONTAINER="${TIMESCALEDB_CONTAINER:-fractsoul-timescaledb}"
INGEST_CONTAINER="${INGEST_CONTAINER:-fractsoul-ingest-api}"
DB_USER="${POSTGRES_USER:-postgres}"
DB_NAME="${POSTGRES_DB:-mining}"
BENCH_MINERS="${BENCH_MINERS:-100}"
SIM_SECONDS="${SIM_SECONDS:-30}"
REPORT_PATH="${REPORT_PATH:-${ROOT_DIR}/docs/operations/D86_benchmark_pre_post.md}"

mkdir -p "$(dirname "${REPORT_PATH}")"

wait_for_http() {
  local retries=90
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
  local retries=90
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

run_sql() {
  local sql="$1"
  docker exec "${TIMESCALEDB_CONTAINER}" psql -tA -F'|' -U "${DB_USER}" -d "${DB_NAME}" -c "${sql}"
}

readings_count() {
  run_sql "SELECT COUNT(*) FROM telemetry_readings;"
}

recreate_ingest_profile() {
  local auth_enabled="$1"
  local rbac_enabled="$2"
  local keys="$3"
  local roles="$4"
  local default_role="$5"

  API_AUTH_ENABLED="${auth_enabled}" \
  API_RBAC_ENABLED="${rbac_enabled}" \
  API_KEYS="${keys}" \
  API_KEY_ROLES="${roles}" \
  API_DEFAULT_ROLE="${default_role}" \
  docker compose up -d --no-deps --force-recreate ingest-api >/dev/null

  wait_for_container_health "${INGEST_CONTAINER}"
  wait_for_http
}

run_simulator() {
  local api_key="$1"
  local args=(
    run
    ./cmd/simulator
    -api-url "${API_URL}"
    -miners "${BENCH_MINERS}"
    -duration "${SIM_SECONDS}s"
    -tick 2s
    -concurrency 32
    -profile-mode mixed
    -schedule staggered
    -schedule-jitter 200ms
  )

  if [[ -n "${api_key}" ]]; then
    args+=( -api-key "${api_key}" )
  fi

  (
    cd "${MODULE_DIR}"
    go "${args[@]}"
  )
}

run_profile() {
  local profile="$1"
  local auth_enabled="$2"
  local rbac_enabled="$3"
  local keys="$4"
  local roles="$5"
  local default_role="$6"
  local api_key="$7"

  echo "Running profile ${profile} (auth=${auth_enabled}, rbac=${rbac_enabled})..."
  recreate_ingest_profile "${auth_enabled}" "${rbac_enabled}" "${keys}" "${roles}" "${default_role}"
  wait_for_container_health "${TIMESCALEDB_CONTAINER}"

  local window_start before_count after_count inserted ingest_rate avg_jth latency_stats alert_avg_ms alert_p95_ms alerts_count
  window_start="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  before_count="$(readings_count)"

  run_simulator "${api_key}"

  after_count="$(readings_count)"
  inserted=$((after_count - before_count))
  ingest_rate="$(awk "BEGIN { if (${SIM_SECONDS} > 0) printf \"%.2f\", ${inserted}/${SIM_SECONDS}; else print \"0.00\" }")"

  avg_jth="$(run_sql "SELECT COALESCE(ROUND(AVG(efficiency_jth)::numeric,4),0) FROM telemetry_readings WHERE ingested_at >= '${window_start}'::timestamptz;")"
  latency_stats="$(run_sql "SELECT COALESCE(ROUND(AVG(GREATEST(EXTRACT(EPOCH FROM (a.updated_at - t.ingested_at))*1000,0))::numeric,2),0), COALESCE(ROUND(PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY GREATEST(EXTRACT(EPOCH FROM (a.updated_at - t.ingested_at))*1000,0))::numeric,2),0), COUNT(*) FROM alerts a JOIN telemetry_readings t ON t.event_id = a.event_id WHERE a.updated_at >= '${window_start}'::timestamptz;")"
  IFS='|' read -r alert_avg_ms alert_p95_ms alerts_count <<< "${latency_stats}"

  echo "${profile}|${inserted}|${ingest_rate}|${avg_jth}|${alert_avg_ms}|${alert_p95_ms}|${alerts_count}" >> "${results_file}"
}

if ! docker ps --format '{{.Names}}' | grep -q "^${TIMESCALEDB_CONTAINER}$"; then
  echo "${TIMESCALEDB_CONTAINER} is not running. Start docker compose first." >&2
  exit 1
fi

results_file="$(mktemp)"
trap 'rm -f "${results_file}"' EXIT

run_profile "baseline" "false" "false" "local-dev-key" "local-dev-key:admin" "admin" ""
run_profile "hardened" "true" "true" "viewer-key,operator-key,admin-key" "viewer-key:viewer,operator-key:operator,admin-key:admin" "viewer" "operator-key"

baseline_row="$(grep '^baseline|' "${results_file}")"
hardened_row="$(grep '^hardened|' "${results_file}")"

IFS='|' read -r _ baseline_inserted baseline_rate baseline_jth baseline_alert_avg baseline_alert_p95 baseline_alert_count <<< "${baseline_row}"
IFS='|' read -r _ hardened_inserted hardened_rate hardened_jth hardened_alert_avg hardened_alert_p95 hardened_alert_count <<< "${hardened_row}"

rate_delta="$(awk "BEGIN { printf \"%.2f\", ${hardened_rate}-${baseline_rate} }")"
jth_delta="$(awk "BEGIN { printf \"%.4f\", ${hardened_jth}-${baseline_jth} }")"
p95_delta="$(awk "BEGIN { printf \"%.2f\", ${hardened_alert_p95}-${baseline_alert_p95} }")"

generated_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
cat > "${REPORT_PATH}" <<EOF
# D86 Benchmark Pre/Post (J/TH, Alert Latency)

- Generated at (UTC): \`${generated_at}\`
- Simulator setup: \`${BENCH_MINERS}\` ASICs, duration \`${SIM_SECONDS}s\`, tick \`2s\`, schedule \`staggered\`
- API URL: \`${API_URL}\`

| Profile | Inserted Events | Ingest Rate (events/s) | Avg J/TH | Alert Avg Latency (ms) | Alert P95 Latency (ms) | Alerts Count |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| baseline | ${baseline_inserted} | ${baseline_rate} | ${baseline_jth} | ${baseline_alert_avg} | ${baseline_alert_p95} | ${baseline_alert_count} |
| hardened | ${hardened_inserted} | ${hardened_rate} | ${hardened_jth} | ${hardened_alert_avg} | ${hardened_alert_p95} | ${hardened_alert_count} |

## Delta (Hardened - Baseline)
- Ingest rate delta (events/s): \`${rate_delta}\`
- Avg J/TH delta: \`${jth_delta}\`
- Alert P95 latency delta (ms): \`${p95_delta}\`

## Notes
- Baseline: auth/RBAC disabled.
- Hardened: auth + RBAC enabled with role-scoped API keys.
- Positive ingest rate delta is better; lower J/TH and lower alert latency are better.
EOF

recreate_ingest_profile "false" "false" "local-dev-key" "local-dev-key:admin" "admin"

echo "Benchmark report generated: ${REPORT_PATH}"
