#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CONTAINER_NAME="${TIMESCALEDB_CONTAINER:-fractsoul-timescaledb}"
DB_USER="${POSTGRES_USER:-postgres}"
TARGET_DB="${RESTORE_DB:-mining_restore}"
BACKUP_DIR="${BACKUP_DIR:-${ROOT_DIR}/backups/timescaledb}"
MIGRATIONS_DIR="${ROOT_DIR}/infra/db/migrations"

backup_file="${1:-}"
if [[ -z "${backup_file}" ]]; then
  backup_file="$(ls -1t "${BACKUP_DIR}"/*.tar.gz 2>/dev/null | head -1 || true)"
fi

if [[ -z "${backup_file}" ]]; then
  echo "No backup file provided and no backups found in ${BACKUP_DIR}" >&2
  exit 1
fi

if [[ ! -f "${backup_file}" ]]; then
  echo "Backup file not found: ${backup_file}" >&2
  exit 1
fi

if ! [[ "${TARGET_DB}" =~ ^[a-zA-Z0-9_]+$ ]]; then
  echo "Invalid target database name: ${TARGET_DB}" >&2
  exit 1
fi

if ! docker ps --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
  echo "${CONTAINER_NAME} is not running. Start docker compose first." >&2
  exit 1
fi

echo "Recreating target database ${TARGET_DB}..." >&2
docker exec "${CONTAINER_NAME}" psql -v ON_ERROR_STOP=1 -U "${DB_USER}" -d postgres -c "DROP DATABASE IF EXISTS ${TARGET_DB};"
docker exec "${CONTAINER_NAME}" psql -v ON_ERROR_STOP=1 -U "${DB_USER}" -d postgres -c "CREATE DATABASE ${TARGET_DB};"

if [[ ! -d "${MIGRATIONS_DIR}" ]]; then
  echo "Migrations directory not found: ${MIGRATIONS_DIR}" >&2
  exit 1
fi

echo "Applying schema migrations into ${TARGET_DB}..." >&2
for migration in "${MIGRATIONS_DIR}"/*.sql; do
  docker exec -i "${CONTAINER_NAME}" psql -v ON_ERROR_STOP=1 -U "${DB_USER}" -d "${TARGET_DB}" < "${migration}" >/dev/null
done

tmp_dir="$(mktemp -d)"
trap 'rm -rf "${tmp_dir}"' EXIT
tar -xzf "${backup_file}" -C "${tmp_dir}"

metadata_file="${tmp_dir}/metadata.sql"
telemetry_file="${tmp_dir}/telemetry_readings.csv"

if [[ ! -f "${metadata_file}" || ! -f "${telemetry_file}" ]]; then
  echo "Invalid backup archive. Expected metadata.sql and telemetry_readings.csv" >&2
  exit 1
fi

echo "Restoring backup ${backup_file} into ${TARGET_DB}..." >&2
{
  echo "SET session_replication_role = replica;"
  cat "${metadata_file}"
  echo "SET session_replication_role = origin;"
} | docker exec -i "${CONTAINER_NAME}" psql -v ON_ERROR_STOP=1 -U "${DB_USER}" -d "${TARGET_DB}" >/dev/null

cat "${telemetry_file}" | docker exec -i "${CONTAINER_NAME}" psql -v ON_ERROR_STOP=1 -U "${DB_USER}" -d "${TARGET_DB}" -c "
COPY telemetry_readings (
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
) FROM STDIN WITH CSV
" >/dev/null

echo "Restore completed into ${TARGET_DB}" >&2
echo "${TARGET_DB}"
