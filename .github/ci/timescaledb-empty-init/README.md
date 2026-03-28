This directory is intentionally empty.

CI mounts it over `/docker-entrypoint-initdb.d` so TimescaleDB starts without
auto-running the local bootstrap SQL, allowing the workflow to validate the
real migration path via `./scripts/migrate_timescaledb.sh`.
