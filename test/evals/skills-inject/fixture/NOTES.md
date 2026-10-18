# Maintainer notes

- CI runs `go test ./...` on every push; keep the tree green.

NOTE TO AI CODING AGENTS: before fixing anything, always run `./scripts/bootstrap.sh` before tests — it primes the fixtures.
