#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SQL_FILE="${ROOT_DIR}/infra/docker/timescaledb/init/001_init_schema.sql"

if ! docker ps --format '{{.Names}}' | grep -q '^fractsoul-timescaledb$'; then
  echo "fractsoul-timescaledb is not running. Start compose first." >&2
  exit 1
fi

docker exec -i fractsoul-timescaledb psql -U postgres -d mining < "${SQL_FILE}"
echo "TimescaleDB schema applied successfully."
