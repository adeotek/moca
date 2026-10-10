# Plan: on-disk response cache

Goal: cache GET bodies on disk with a TTL.

- [ ] Add a Cache type (dir, TTL) in cache.go; verify with unit tests.
- [ ] Use it from Client.Get in fetch.go; verify with fetch_test.go.
- [ ] Add a TTL option to New; verify go test ./... passes.
