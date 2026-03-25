# D86 Benchmark Pre/Post (J/TH, Alert Latency)

- Generated at (UTC): `2026-03-25T21:01:54Z`
- Simulator setup: `100` ASICs, duration `30s`, tick `2s`, schedule `staggered`
- API URL: `http://localhost:8080`

| Profile | Inserted Events | Ingest Rate (events/s) | Avg J/TH | Alert Avg Latency (ms) | Alert P95 Latency (ms) | Alerts Count |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| baseline | 1500 | 50.00 | 31.0645 | 2.99 | 4.44 | 76 |
| hardened | 1500 | 50.00 | 29.2883 | 3.13 | 5.93 | 78 |

## Delta (Hardened - Baseline)
- Ingest rate delta (events/s): `0.00`
- Avg J/TH delta: `-1.7762`
- Alert P95 latency delta (ms): `1.49`

## Notes
- Baseline: auth/RBAC disabled.
- Hardened: auth + RBAC enabled with role-scoped API keys.
- Positive ingest rate delta is better; lower J/TH and lower alert latency are better.
