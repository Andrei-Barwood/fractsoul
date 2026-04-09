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

migration_materialized_sql() {
  case "$1" in
    0001_initial_schema.sql)
      cat <<'SQL'
SELECT to_regclass('public.telemetry_readings') IS NOT NULL
   AND to_regclass('public.telemetry_latest') IS NOT NULL;
SQL
      ;;
    0002_timescale_optimizations_s2.sql)
      cat <<'SQL'
SELECT to_regclass('public.telemetry_agg_minute') IS NOT NULL
   AND to_regclass('public.telemetry_agg_hour') IS NOT NULL;
SQL
      ;;
    0003_alerts_engine_s2.sql)
      cat <<'SQL'
SELECT to_regclass('public.alerts') IS NOT NULL
   AND to_regclass('public.alert_notifications') IS NOT NULL;
SQL
      ;;
    0004_s3_recommendation_changes.sql)
      cat <<'SQL'
SELECT to_regclass('public.recommendation_changes') IS NOT NULL;
SQL
      ;;
    0005_s3_daily_reports.sql)
      cat <<'SQL'
SELECT to_regclass('public.daily_reports') IS NOT NULL;
SQL
      ;;
    0006_s4_energy_orchestrator.sql)
      cat <<'SQL'
SELECT to_regclass('public.energy_site_profiles') IS NOT NULL
   AND to_regclass('public.energy_rack_profiles') IS NOT NULL;
SQL
      ;;
    0007_s4_energy_snapshots.sql)
      cat <<'SQL'
SELECT to_regclass('public.energy_budget_snapshots') IS NOT NULL;
SQL
      ;;
    *)
      return 1
      ;;
  esac
}

mark_existing_migration() {
  local version="$1"
  local sql
  local materialized

  if ! sql="$(migration_materialized_sql "${version}")"; then
    return 1
  fi

  materialized="$(
    docker exec "${CONTAINER_NAME}" psql -tA -U "${DB_USER}" -d "${DB_NAME}" -c "${sql}" \
      | tr -d '[:space:]'
  )"

  if [[ "${materialized}" != "t" ]]; then
    return 1
  fi

  echo "Backfilling ${version} in schema_migrations (objects already present)"
  docker exec "${CONTAINER_NAME}" psql -v ON_ERROR_STOP=1 -U "${DB_USER}" -d "${DB_NAME}" \
    -c "INSERT INTO schema_migrations(version) VALUES ('${version}') ON CONFLICT (version) DO NOTHING" \
    >/dev/null
  return 0
}

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

  if mark_existing_migration "${version}"; then
    continue
  fi

  echo "Applying ${version}..."
  docker exec -i "${CONTAINER_NAME}" psql -v ON_ERROR_STOP=1 -U "${DB_USER}" -d "${DB_NAME}" < "${migration}"
  docker exec "${CONTAINER_NAME}" psql -v ON_ERROR_STOP=1 -U "${DB_USER}" -d "${DB_NAME}" -c "INSERT INTO schema_migrations(version) VALUES ('${version}')"
  echo "Applied ${version}"
done

echo "Migrations completed successfully."
