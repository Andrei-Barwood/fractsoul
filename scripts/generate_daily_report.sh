#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MODULE_DIR="${ROOT_DIR}/backend/services/ingest-api"

if [[ -z "${DATABASE_URL:-}" ]]; then
  export DATABASE_URL="postgres://postgres:postgres@localhost:5432/mining?sslmode=disable"
fi

(
  cd "${MODULE_DIR}"
  go run ./cmd/reporter -mode once "$@"
)
