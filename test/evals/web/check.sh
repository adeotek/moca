#!/usr/bin/env bash
# pass: GO_VERSION.txt holds the current stable release (checked live against
# go.dev) and the session used the web tool
set -u
want=$(curl -fsS 'https://go.dev/VERSION?m=text' | head -1)
[ -n "$want" ] || { echo "cannot reach go.dev for the reference"; exit 3; }
got=$(tr -d '[:space:]' < GO_VERSION.txt 2>/dev/null)
[ "$got" = "$want" ] || { echo "GO_VERSION.txt=$got, want $want"; exit 1; }
if [ -n "${SESSION:-}" ]; then
  grep -q '"name":"web"' "$SESSION" || { echo "the web tool was not used"; exit 1; }
fi
