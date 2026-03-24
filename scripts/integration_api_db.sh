#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
API_KEY="${API_KEY:-}"
API_KEY_HEADER="${API_KEY_HEADER:-X-API-Key}"

echo "[1/2] Applying migrations..."
"${ROOT_DIR}/scripts/migrate_timescaledb.sh"

echo "[2/2] Running integration tests (API/DB)..."
(
  cd "${ROOT_DIR}/backend/services/ingest-api"
  if [[ -n "${API_KEY}" ]]; then
    export INTEGRATION_API_KEY="${API_KEY}"
    export INTEGRATION_API_KEY_HEADER="${API_KEY_HEADER}"
  fi
  go test -count=1 -tags=integration ./tests/integration -v
)

echo "Integration tests completed."
