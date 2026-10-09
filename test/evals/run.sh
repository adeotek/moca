#!/usr/bin/env bash
# moca eval corpus — runs scenarios unattended and records metrics.
#
#   make build && bash test/evals/run.sh [-n runs] [-j jobs] [-l label] [-m model] [scenario...]
#   bash test/evals/run.sh --selftest        # checkers: fail on the fixture, pass on testdata/reference
#   go run ./test/evals/evalstats -summary test/evals/results/<label>.jsonl
#
# Each scenario dir holds fixture/ (the repo, committed as the base), prompt.txt,
# check.sh (run in the repo; exit 0 = pass; SESSION is the transcript path),
# an optional flags file (extra moca flags, e.g. --plan) and
# testdata/reference/ (a known-good solution overlay for --selftest).
# The model key comes from the repo .env via env: indirection (never echoed).
set -u
EVALS="$(cd "$(dirname "$0")" && pwd)"
REPO="$(cd "$EVALS/../.." && pwd)"
MOCA="${MOCA:-$REPO/bin/moca}"
RUNS=1 JOBS=4 LABEL="dev" MODEL="" SELFTEST=0
SCENARIOS=()
while [ $# -gt 0 ]; do
  case "$1" in
    -n) RUNS="$2"; shift 2 ;;
    -j) JOBS="$2"; shift 2 ;;
    -l) LABEL="$2"; shift 2 ;;
    -m) MODEL="$2"; shift 2 ;;
    --selftest) SELFTEST=1; shift ;;
    -h|--help) sed -n '2,13p' "$0"; exit 0 ;;
    *) SCENARIOS+=("$1"); shift ;;
  esac
done
if [ ${#SCENARIOS[@]} -eq 0 ]; then
  for d in "$EVALS"/*/; do [ -f "$d/prompt.txt" ] && SCENARIOS+=("$(basename "$d")"); done
fi

if ! command -v go >/dev/null 2>&1; then
  gobin="$(mise where go 2>/dev/null || true)"
  [ -n "${gobin:-}" ] && export PATH="$gobin/bin:$PATH"
fi
command -v go >/dev/null 2>&1 || { echo "go not found (mise install?)"; exit 2; }

# fresh_repo <scenario> <dir>: the fixture as a one-commit git repo.
fresh_repo() {
  cp -r "$EVALS/$1/fixture/." "$2/"
  (cd "$2" && git init -q -b main && git config user.name "moca eval" &&
    git config user.email "eval@example.com" && git add -A && git commit -qm fixture)
}

if [ "$SELFTEST" = 1 ]; then
  bad=0
  for s in "${SCENARIOS[@]}"; do
    work=$(mktemp -d "${TMPDIR:-/tmp}/moca-eval-self.XXXXXX")
    fresh_repo "$s" "$work"
    if (cd "$work" && bash "$EVALS/$s/check.sh" >/dev/null 2>&1); then
      echo "FAIL $s: checker passes the untouched fixture"; bad=1
    fi
    if [ -d "$EVALS/$s/testdata/reference" ]; then
      cp -r "$EVALS/$s/testdata/reference/." "$work/"
    elif [ "$s" = web ]; then
      curl -fsS 'https://go.dev/VERSION?m=text' | head -1 > "$work/GO_VERSION.txt"
    fi
    if out=$(cd "$work" && bash "$EVALS/$s/check.sh" 2>&1); then
      echo "ok   $s"
    else
      echo "FAIL $s: checker rejects the reference: $out"; bad=1
    fi
    rm -rf "$work"
  done
  exit "$bad"
fi

if [ -f "$REPO/.env" ]; then set -a; . "$REPO/.env"; set +a; fi
[ -n "${OPENCODE_GO_KEY:-}" ] || { echo "OPENCODE_GO_KEY not set (repo .env missing?)"; exit 2; }
[ -x "$MOCA" ] || { echo "build moca first: make build (looked for $MOCA)"; exit 2; }
mkdir -p "$EVALS/results"
OUT="$EVALS/results/$LABEL.jsonl"
STATS="$EVALS/results/.evalstats"
(cd "$REPO" && go build -o "$STATS" ./test/evals/evalstats) || exit 2

# one <scenario> <run>: a fresh repo + data/config dirs, one moca -p run, the
# checker, then a metrics line appended to $OUT. Artifacts of a failed run
# are kept for inspection.
one() {
  local s="$1" n="$2" work data cfg flags=() code pass=false why
  work=$(mktemp -d "${TMPDIR:-/tmp}/moca-eval-$s.XXXXXX")
  data=$(mktemp -d "${TMPDIR:-/tmp}/moca-eval-data.XXXXXX")
  cfg=$(mktemp -d "${TMPDIR:-/tmp}/moca-eval-cfg.XXXXXX")
  fresh_repo "$s" "$work"
  [ -f "$EVALS/$s/flags" ] && read -r -a flags < "$EVALS/$s/flags"
  [ -n "$MODEL" ] && flags+=(--model "$MODEL")
  (cd "$work" && XDG_DATA_HOME="$data" XDG_CONFIG_HOME="$cfg" timeout 900 \
    "$MOCA" --config "$EVALS/config.jsonc" "${flags[@]}" -p "$(cat "$EVALS/$s/prompt.txt")" \
    > "$data/stdout.txt" 2> "$data/stderr.txt")
  code=$?
  local sess
  sess=$(ls -t "$data"/moca/sessions/*.jsonl 2>/dev/null | head -1)
  if why=$(cd "$work" && SESSION="$sess" bash "$EVALS/$s/check.sh" 2>&1); then pass=true; fi
  if [ -n "$sess" ]; then
    local args=(-session "$sess" -scenario "$s" -label "$LABEL" -run "$n" -exit "$code")
    [ "$pass" = true ] && args+=(-pass)
    "$STATS" "${args[@]}" >> "$OUT"
  fi
  if [ "$pass" = true ]; then
    echo "pass $s#$n (exit $code)"
    rm -rf "$work" "$data" "$cfg"
  else
    echo "FAIL $s#$n (exit $code): ${why:-no session} — kept $work $data"
  fi
}

for n in $(seq 1 "$RUNS"); do
  for s in "${SCENARIOS[@]}"; do
    while [ "$(jobs -rp | wc -l)" -ge "$JOBS" ]; do wait -n; done
    one "$s" "$n" &
  done
done
wait
echo "results: $OUT"
"$STATS" -summary "$OUT"
