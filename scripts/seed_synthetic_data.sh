#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SEED_SQL_FILE="${ROOT_DIR}/infra/db/seeds/001_synthetic_fleet.sql"
CONTAINER_NAME="${TIMESCALEDB_CONTAINER:-fractsoul-timescaledb}"
DB_USER="${POSTGRES_USER:-postgres}"
DB_NAME="${POSTGRES_DB:-mining}"

if [[ ! -f "${SEED_SQL_FILE}" ]]; then
  echo "Seed file not found: ${SEED_SQL_FILE}" >&2
  exit 1
fi

if ! docker ps --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
  echo "${CONTAINER_NAME} is not running. Start compose first." >&2
  exit 1
fi

docker exec -i "${CONTAINER_NAME}" psql -v ON_ERROR_STOP=1 -U "${DB_USER}" -d "${DB_NAME}" < "${SEED_SQL_FILE}"
echo "Synthetic seed applied successfully."
