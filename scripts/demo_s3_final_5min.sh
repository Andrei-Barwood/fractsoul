#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TIMESCALEDB_CONTAINER="${TIMESCALEDB_CONTAINER:-fractsoul-timescaledb}"
DB_USER="${POSTGRES_USER:-postgres}"
DB_NAME="${POSTGRES_DB:-mining}"
API_URL="${API_URL:-http://localhost:8080}"
API_KEY="${API_KEY:-}"
API_KEY_HEADER="${API_KEY_HEADER:-X-API-Key}"
REPORT_TIMEZONE="${REPORT_TIMEZONE:-America/Santiago}"
REPORT_DATE="${REPORT_DATE:-$(date +%Y-%m-%d)}"

DEMO_SITE_ID="${DEMO_SITE_ID:-site-cl-01}"
DEMO_RACK_ID="${DEMO_RACK_ID:-rack-cl-01-01}"
DEMO_MINER_ID="${DEMO_MINER_ID:-asic-920001}"
REQUESTED_BY="${REQUESTED_BY:-ops-demo@fractsoul.local}"

wait_for_http() {
  local retries=60
  while (( retries > 0 )); do
    if curl -fsS "${API_URL}/healthz" >/dev/null 2>&1; then
      return 0
    fi
    retries=$((retries - 1))
    sleep 1
  done
  echo "Timed out waiting for ${API_URL}/healthz" >&2
  return 1
}

run_sql() {
  local sql="$1"
  docker exec "${TIMESCALEDB_CONTAINER}" psql -tA -F'|' -U "${DB_USER}" -d "${DB_NAME}" -c "${sql}"
}

curl_get_auth() {
  local endpoint="$1"
  if [[ -n "${API_KEY}" ]]; then
    curl -fsS -H "${API_KEY_HEADER}: ${API_KEY}" "${endpoint}"
  else
    curl -fsS "${endpoint}"
  fi
}

curl_post_auth() {
  local endpoint="$1"
  local payload="$2"
  if [[ -n "${API_KEY}" ]]; then
    curl -fsS -X POST \
      -H "Content-Type: application/json" \
      -H "${API_KEY_HEADER}: ${API_KEY}" \
      "${endpoint}" \
      -d "${payload}"
  else
    curl -fsS -X POST \
      -H "Content-Type: application/json" \
      "${endpoint}" \
      -d "${payload}"
  fi
}

if ! command -v python3 >/dev/null 2>&1; then
  echo "python3 is required for parsing JSON responses." >&2
  exit 1
fi

if ! docker ps --format '{{.Names}}' | grep -q "^${TIMESCALEDB_CONTAINER}$"; then
  echo "${TIMESCALEDB_CONTAINER} is not running. Start docker compose first." >&2
  exit 1
fi

START_TS="$(date +%s)"

echo "[1/9] Preflight: healthcheck API + DB..."
wait_for_http
run_sql "SELECT 1;" >/dev/null

echo "[2/9] Injecting deterministic fault scenario (reuse D59)..."
API_KEY="${API_KEY}" API_KEY_HEADER="${API_KEY_HEADER}" \
  "${ROOT_DIR}/scripts/demo_s2_fallas_simuladas.sh" >/dev/null

echo "[3/9] Efficiency snapshots (miner/rack/site)..."
miner_eff_json="$(curl_get_auth "${API_URL}/v1/efficiency/miners?miner_id=${DEMO_MINER_ID}&window_minutes=120&limit=5")"
rack_eff_json="$(curl_get_auth "${API_URL}/v1/efficiency/racks?site_id=${DEMO_SITE_ID}&rack_id=${DEMO_RACK_ID}&window_minutes=120&limit=5")"
site_eff_json="$(curl_get_auth "${API_URL}/v1/efficiency/sites?site_id=${DEMO_SITE_ID}&window_minutes=120&limit=5")"

miner_count="$(printf '%s' "${miner_eff_json}" | python3 -c 'import json,sys; d=json.load(sys.stdin); print(d.get("count",0))')"
rack_count="$(printf '%s' "${rack_eff_json}" | python3 -c 'import json,sys; d=json.load(sys.stdin); print(d.get("count",0))')"
site_count="$(printf '%s' "${site_eff_json}" | python3 -c 'import json,sys; d=json.load(sys.stdin); print(d.get("count",0))')"
miner_comp_jth="$(printf '%s' "${miner_eff_json}" | python3 -c 'import json,sys; d=json.load(sys.stdin); items=d.get("items") or []; print(items[0].get("compensated_jth","n/a") if items else "n/a")')"
echo "Efficiency count miner/rack/site: ${miner_count}/${rack_count}/${site_count}"
echo "Miner ${DEMO_MINER_ID} compensated_jth: ${miner_comp_jth}"

echo "[4/9] Running anomaly analysis..."
anomaly_json="$(curl_get_auth "${API_URL}/v1/anomalies/miners/${DEMO_MINER_ID}/analyze?resolution=minute&limit=120")"
anomaly_summary="$(printf '%s' "${anomaly_json}" | python3 -c 'import json,sys; d=json.load(sys.stdin); print(d.get("summary_line","n/a"))')"
severity_label="$(printf '%s' "${anomaly_json}" | python3 -c 'import json,sys; d=json.load(sys.stdin); print((d.get("report") or {}).get("severity_label","n/a"))')"
severity_score="$(printf '%s' "${anomaly_json}" | python3 -c 'import json,sys; d=json.load(sys.stdin); print((d.get("report") or {}).get("severity_score","n/a"))')"
echo "Anomaly summary: ${anomaly_summary}"
echo "Anomaly severity: ${severity_label} (${severity_score})"

echo "[5/9] Applying + rolling back one guarded recommendation..."
apply_json="$(curl_post_auth "${API_URL}/v1/anomalies/miners/${DEMO_MINER_ID}/changes/apply?resolution=minute&limit=120" \
  "{\"reason\":\"D87 final demo apply\", \"requested_by\":\"${REQUESTED_BY}\"}")"
change_id="$(printf '%s' "${apply_json}" | python3 -c 'import json,sys; d=json.load(sys.stdin); print((d.get("change") or {}).get("change_id",""))')"

if [[ -z "${change_id}" ]]; then
  echo "Failed to extract change_id from apply response." >&2
  exit 1
fi

rollback_json="$(curl_post_auth "${API_URL}/v1/anomalies/changes/${change_id}/rollback" \
  "{\"reason\":\"D87 final demo rollback\", \"requested_by\":\"${REQUESTED_BY}\"}")"
rollback_change_id="$(printf '%s' "${rollback_json}" | python3 -c 'import json,sys; d=json.load(sys.stdin); print((d.get("change") or {}).get("change_id",""))')"
rollback_change_status="$(printf '%s' "${rollback_json}" | python3 -c 'import json,sys; d=json.load(sys.stdin); print((d.get("change") or {}).get("status",""))')"
apply_change_status="$(run_sql "SELECT status FROM recommendation_changes WHERE change_id='${change_id}' LIMIT 1;")"
changes_json="$(curl_get_auth "${API_URL}/v1/anomalies/changes?miner_id=${DEMO_MINER_ID}&limit=5")"
changes_count="$(printf '%s' "${changes_json}" | python3 -c 'import json,sys; d=json.load(sys.stdin); print(d.get("count",0))')"
echo "Apply change id: ${change_id}"
echo "Apply change final status: ${apply_change_status}"
echo "Rollback change id: ${rollback_change_id}"
echo "Rollback transaction status: ${rollback_change_status}"
echo "Recent changes count for ${DEMO_MINER_ID}: ${changes_count}"

echo "[6/9] Generating executive-operational daily report..."
"${ROOT_DIR}/scripts/generate_daily_report.sh" \
  -timezone "${REPORT_TIMEZONE}" \
  -date "${REPORT_DATE}" >/dev/null

report_row="$(run_sql "
SELECT
  report_id,
  report_date::text,
  timezone,
  to_char(generated_at AT TIME ZONE 'UTC', 'YYYY-MM-DD\"T\"HH24:MI:SS\"Z\"'),
  COALESCE((report_json->'global'->>'samples')::int, 0),
  COALESCE((report_json->'alerts'->>'total')::int, 0),
  COALESCE((report_json->'changes'->>'applied')::int, 0),
  COALESCE((report_json->'changes'->>'rolled_back')::int, 0)
FROM daily_reports
ORDER BY generated_at DESC
LIMIT 1;
")"

if [[ -z "${report_row}" ]]; then
  echo "No daily report row found after report generation." >&2
  exit 1
fi

IFS='|' read -r report_id report_date timezone generated_utc report_samples report_alerts report_applied report_rolled_back <<< "${report_row}"
echo "Daily report: id=${report_id} date=${report_date} tz=${timezone} generated=${generated_utc}"
echo "Daily report KPIs: samples=${report_samples} alerts=${report_alerts} applied=${report_applied} rolled_back=${report_rolled_back}"

echo "[7/9] Observability snapshot..."
metrics_lines="$(curl_get_auth "${API_URL}/metrics" | grep -E '^(fractsoul_http_requests_total|fractsoul_ingest_events_total|fractsoul_processor_events_total|fractsoul_alerts_evaluations_total|fractsoul_alerts_notifications_total)' | head -n 12 || true)"
if [[ -z "${metrics_lines}" ]]; then
  echo "No fractsoul metrics found in /metrics output." >&2
  exit 1
fi
printf '%s\n' "${metrics_lines}"

echo "[8/9] Alert status summary for demo miners..."
alert_status_row="$(run_sql "
SELECT
  COUNT(*) AS total,
  COALESCE(SUM(CASE WHEN status = 'open' THEN 1 ELSE 0 END), 0) AS open_count,
  COALESCE(SUM(CASE WHEN status = 'suppressed' THEN 1 ELSE 0 END), 0) AS suppressed_count
FROM alerts
WHERE miner_id IN ('asic-920001', 'asic-920002', 'asic-920003', 'asic-920004');
")"
IFS='|' read -r alerts_total alerts_open alerts_suppressed <<< "${alert_status_row}"
echo "Alerts total/open/suppressed: ${alerts_total}/${alerts_open}/${alerts_suppressed}"

echo "[9/9] Final timing..."
END_TS="$(date +%s)"
ELAPSED="$((END_TS - START_TS))"
echo "D87 final demo run completed in ${ELAPSED}s."
