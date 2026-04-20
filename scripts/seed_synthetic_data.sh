#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SEEDS_DIR="${ROOT_DIR}/infra/db/seeds"
CONTAINER_NAME="${TIMESCALEDB_CONTAINER:-fractsoul-timescaledb}"
DB_USER="${POSTGRES_USER:-postgres}"
DB_NAME="${POSTGRES_DB:-mining}"

if [[ ! -d "${SEEDS_DIR}" ]]; then
  echo "Seeds directory not found: ${SEEDS_DIR}" >&2
  exit 1
fi

if ! docker ps --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
  echo "${CONTAINER_NAME} is not running. Start compose first." >&2
  exit 1
fi

for seed_file in "${SEEDS_DIR}"/*.sql; do
  if [[ ! -f "${seed_file}" ]]; then
    continue
  fi

  echo "Applying seed $(basename "${seed_file}")..."
  docker exec -i "${CONTAINER_NAME}" psql -v ON_ERROR_STOP=1 -U "${DB_USER}" -d "${DB_NAME}" < "${seed_file}"
done

echo "Seed set applied successfully."
