#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CONTAINER_NAME="${TIMESCALEDB_CONTAINER:-fractsoul-timescaledb}"
DB_USER="${POSTGRES_USER:-postgres}"
DB_NAME="${POSTGRES_DB:-mining}"
BACKUP_DIR="${BACKUP_DIR:-${ROOT_DIR}/backups/timescaledb}"

mkdir -p "${BACKUP_DIR}"

if [[ ! -d "${BACKUP_DIR}" ]]; then
  echo "Unable to create backup directory: ${BACKUP_DIR}" >&2
  exit 1
fi

if ! docker ps --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
  echo "${CONTAINER_NAME} is not running. Start docker compose first." >&2
  exit 1
fi

timestamp="$(date -u +%Y%m%d_%H%M%S)"
backup_file="${BACKUP_FILE:-${BACKUP_DIR}/${DB_NAME}_${timestamp}.tar.gz}"

echo "Creating backup from ${DB_NAME} (${CONTAINER_NAME})..." >&2
tmp_dir="$(mktemp -d)"
trap 'rm -rf "${tmp_dir}"' EXIT

metadata_file="${tmp_dir}/metadata.sql"
telemetry_file="${tmp_dir}/telemetry_readings.csv"

table_args=(
	--table=public.sites
	--table=public.racks
	--table=public.miners
	--table=public.alerts
	--table=public.alert_notifications
	--table=public.recommendation_changes
	--table=public.daily_reports
)

docker exec "${CONTAINER_NAME}" pg_dump \
	--data-only \
	--inserts \
	--column-inserts \
	--no-owner \
	--no-privileges \
	"${table_args[@]}" \
	-U "${DB_USER}" \
	-d "${DB_NAME}" > "${metadata_file}"

docker exec "${CONTAINER_NAME}" psql -tA -U "${DB_USER}" -d "${DB_NAME}" -c "
COPY (
  SELECT
    ts,
    event_id,
    site_id,
    rack_id,
    miner_id,
    firmware_version,
    hashrate_ths,
    power_watts,
    temp_celsius,
    fan_rpm,
    efficiency_jth,
    status,
    load_pct,
    tags,
    raw_payload,
    ingested_at
  FROM telemetry_readings
  ORDER BY ts ASC, event_id ASC
) TO STDOUT WITH CSV
" > "${telemetry_file}"

tar -czf "${backup_file}" -C "${tmp_dir}" metadata.sql telemetry_readings.csv

if [[ ! -s "${backup_file}" ]]; then
  echo "Backup file is empty: ${backup_file}" >&2
  exit 1
fi

echo "Backup created: ${backup_file}" >&2
echo "${backup_file}"
