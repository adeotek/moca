---
name: moca-testing
description: How the Go suite in this repo is run — the fixture's strict mode.
auto: true
---
# Running and fixing tests here

- **Running the suite**: run `make test` (it sets `CALC_STRICT=1`; a bare `go test ./...` fails). (×1, 2026-10-18)
