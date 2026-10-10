#!/usr/bin/env bash
# pass: tests green and untouched, validate.go holds the logic, the duplicate is gone
set -u
base=$(git rev-list --max-parents=0 HEAD)
go test ./... >/dev/null 2>&1 || { echo "tests fail"; exit 1; }
git diff --quiet "$base" -- '*_test.go' || { echo "tests were modified"; exit 1; }
[ -f validate.go ] || { echo "no validate.go"; exit 1; }
n=$(cat $(ls *.go | grep -v _test.go) | grep -c "LastIndexByte(.*'@')")
[ "$n" = 1 ] || { echo "email parsing appears $n times (want 1)"; exit 1; }
grep -q "LastIndexByte(.*'@')" validate.go || { echo "the shared logic is not in validate.go"; exit 1; }
go vet ./... >/dev/null 2>&1 || { echo "vet fails"; exit 1; }
