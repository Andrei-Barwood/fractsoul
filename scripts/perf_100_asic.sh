#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

echo "[1/2] Applying migrations..."
"${ROOT_DIR}/scripts/migrate_timescaledb.sh"

echo "[2/2] Running perf test (100 ASICs)..."
(
  cd "${ROOT_DIR}/backend/services/ingest-api"
  go test -count=1 -tags=perf ./tests/perf -v
)

echo "Performance check completed."
