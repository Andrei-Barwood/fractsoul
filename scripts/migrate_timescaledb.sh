#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MIGRATIONS_DIR="${ROOT_DIR}/infra/db/migrations"
CONTAINER_NAME="${TIMESCALEDB_CONTAINER:-fractsoul-timescaledb}"
DB_USER="${POSTGRES_USER:-postgres}"
DB_NAME="${POSTGRES_DB:-mining}"

if [[ ! -d "${MIGRATIONS_DIR}" ]]; then
  echo "Migrations directory not found: ${MIGRATIONS_DIR}" >&2
  exit 1
fi

if ! docker ps --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
  echo "${CONTAINER_NAME} is not running. Start compose first." >&2
  exit 1
fi

docker exec -i "${CONTAINER_NAME}" psql -v ON_ERROR_STOP=1 -U "${DB_USER}" -d "${DB_NAME}" <<'SQL'
CREATE TABLE IF NOT EXISTS schema_migrations (
  version TEXT PRIMARY KEY,
  applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
SQL

for migration in "${MIGRATIONS_DIR}"/*.sql; do
  version="$(basename "${migration}")"
  already_applied="$(docker exec "${CONTAINER_NAME}" psql -tA -U "${DB_USER}" -d "${DB_NAME}" -c "SELECT 1 FROM schema_migrations WHERE version = '${version}'")"

  if [[ "${already_applied}" == "1" ]]; then
    echo "Skipping ${version} (already applied)"
    continue
  fi

  echo "Applying ${version}..."
  docker exec -i "${CONTAINER_NAME}" psql -v ON_ERROR_STOP=1 -U "${DB_USER}" -d "${DB_NAME}" < "${migration}"
  docker exec "${CONTAINER_NAME}" psql -v ON_ERROR_STOP=1 -U "${DB_USER}" -d "${DB_NAME}" -c "INSERT INTO schema_migrations(version) VALUES ('${version}')"
  echo "Applied ${version}"
done

echo "Migrations completed successfully."
