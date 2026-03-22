# D10 - CI minimo (lint + tests)

Fecha: 2026-03-22

## Objetivo
Garantizar feedback automatico en cada push/PR.

## Implementacion
- Workflow: `.github/workflows/ci.yml`
- Scope inicial: `backend/services/ingest-api`

## Checks
1. `go mod download`
2. `gofmt` check
3. `go vet ./...`
4. `go test ./...`
