#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CONTAINER_NAME="${TIMESCALEDB_CONTAINER:-fractsoul-timescaledb}"
DB_USER="${POSTGRES_USER:-postgres}"
SOURCE_DB="${POSTGRES_DB:-mining}"
TARGET_DB="${RESTORE_DB:-mining_restore_test}"

TABLES=(
  telemetry_readings
  alerts
  recommendation_changes
  daily_reports
)

if ! docker ps --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
  echo "${CONTAINER_NAME} is not running. Start docker compose first." >&2
  exit 1
fi

backup_file="$("${ROOT_DIR}/scripts/backup_timescaledb.sh")"
RESTORE_DB="${TARGET_DB}" "${ROOT_DIR}/scripts/restore_timescaledb.sh" "${backup_file}" >/dev/null

query_count() {
  local db="$1"
  local table="$2"
  docker exec "${CONTAINER_NAME}" psql -tA -U "${DB_USER}" -d "${db}" -c "SELECT COUNT(*) FROM ${table};"
}

echo "Comparing source (${SOURCE_DB}) vs restored (${TARGET_DB}) counts..."
for table in "${TABLES[@]}"; do
  source_count="$(query_count "${SOURCE_DB}" "${table}")"
  target_count="$(query_count "${TARGET_DB}" "${table}")"
  echo "- ${table}: source=${source_count} restored=${target_count}"

  if [[ "${source_count}" != "${target_count}" ]]; then
    echo "Mismatch detected for table ${table}" >&2
    exit 1
  fi
done

echo "Backup/restore validation passed."
