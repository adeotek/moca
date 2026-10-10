#!/usr/bin/env bash
# pass: tests green and the tests untouched
set -u
base=$(git rev-list --max-parents=0 HEAD)
go test ./... >/dev/null 2>&1 || { echo "tests fail"; exit 1; }
git diff --quiet "$base" -- '*_test.go' || { echo "tests were modified"; exit 1; }
