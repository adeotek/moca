#!/usr/bin/env bash
# moca eval corpus — runs scenarios unattended and records metrics.
#
#   make build && bash test/evals/run.sh [-n runs] [-j jobs] [-l label] [-m model] [-c variant] [scenario...]
#   bash test/evals/run.sh --selftest        # checkers: fail on the fixture, pass on testdata/reference
#   go run ./test/evals/evalstats -summary test/evals/results/<label>.jsonl
#
# Each scenario dir holds fixture/ (the repo, committed as the base), prompt.txt
# (or sessions.txt: one prompt per line, run in order against the SAME
# workdir/data/cfg, so a later session sees an earlier one's skills),
# check.sh (run in the repo; exit 0 = pass; SESSION is the newest transcript,
# DATA the run's data dir), an optional flags file (extra moca flags, e.g.
# --mode plan) and testdata/reference/ (a known-good solution overlay for
# --selftest). A per-scenario config.jsonc (or config.<variant>.jsonc, picked
# with -c <variant>) overrides the shared one; a scenario with no config of its
# own falls back to config.<variant>.jsonc, then config.jsonc (that is how the
# corpus-wide switch A/B is run: `-c on` → config.on.jsonc). The scenario list
# defaults to the dirs holding prompt.txt; a sessions.txt-only measurement
# scenario (skills-cold: it ships no .moca/skills and measures whether a run
# records what it had to discover — expected to fail most runs) is named
# explicitly so it does not drag every corpus run's pass rate.
# The model key comes from the repo .env via env: indirection (never echoed).
set -u
EVALS="$(cd "$(dirname "$0")" && pwd)"
REPO="$(cd "$EVALS/../.." && pwd)"
MOCA="${MOCA:-$REPO/bin/moca}"
# VARIANT is set only by -c: without it every scenario runs its own config
# (or the shared corpus config), never a variant arm; the checkers still see
# "on" (the scenarios' own configs are the on arm).
RUNS=1 JOBS=4 LABEL="dev" MODEL="" SELFTEST=0 VARIANT=""
SCENARIOS=()
while [ $# -gt 0 ]; do
  case "$1" in
    -n) RUNS="$2"; shift 2 ;;
    -j) JOBS="$2"; shift 2 ;;
    -l) LABEL="$2"; shift 2 ;;
    -m) MODEL="$2"; shift 2 ;;
    -c) VARIANT="$2"; shift 2 ;;
    --selftest) SELFTEST=1; shift ;;
    -h|--help) sed -n '2,15p' "$0"; exit 0 ;;
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

# scenario_config <scenario>: the user config a scenario runs with — a
# variant-specific one (-c only), its own, else the shared corpus config.
scenario_config() {
  local s="$1"
  if [ -n "$VARIANT" ] && [ -f "$EVALS/$s/config.$VARIANT.jsonc" ]; then
    echo "$EVALS/$s/config.$VARIANT.jsonc"
  elif [ -f "$EVALS/$s/config.jsonc" ]; then
    echo "$EVALS/$s/config.jsonc"
  elif [ -n "$VARIANT" ] && [ -f "$EVALS/config.$VARIANT.jsonc" ]; then
    echo "$EVALS/config.$VARIANT.jsonc"   # corpus-wide arm (A/B a feature switch)
  else
    echo "$EVALS/config.jsonc"
  fi
}

# fresh_repo <scenario> <dir>: the fixture as a one-commit git repo.
fresh_repo() {
  cp -r "$EVALS/$1/fixture/." "$2/"
  (cd "$2" && git init -q -b main && git config user.name "moca eval" &&
    git config user.email "eval@example.com" && git add -A && git commit -qm fixture)
}

if [ "$SELFTEST" = 1 ]; then
  bad=0
  # skills-cold is sessions.txt-only (kept out of the default scenario list on
  # purpose), but its checker's file-state arms need the gate like the rest —
  # with SESSION unset, so the loop below covers it without naming it here.
  for s in "${SCENARIOS[@]}" skills-cold; do
    work=$(mktemp -d "${TMPDIR:-/tmp}/moca-eval-self.XXXXXX")
    fresh_repo "$s" "$work"
    if (cd "$work" && VARIANT=on bash "$EVALS/$s/check.sh" >/dev/null 2>&1); then
      echo "FAIL $s: checker passes the untouched fixture"; bad=1
    fi
    if [ -d "$EVALS/$s/testdata/reference" ]; then
      cp -r "$EVALS/$s/testdata/reference/." "$work/"
    elif [ "$s" = web ]; then
      curl -fsS 'https://go.dev/VERSION?m=text' | head -1 > "$work/GO_VERSION.txt"
    fi
    if out=$(cd "$work" && VARIANT=on bash "$EVALS/$s/check.sh" 2>&1); then
      echo "ok   $s"
    else
      echo "FAIL $s: checker rejects the reference: $out"; bad=1
    fi
    rm -rf "$work"
  done
  # The contaminated-rule scan is shared by the inject arms and carries its own
  # pin: a refusal must precede the command (an assertion after it used to
  # escape the arm).
  if bash "$EVALS/_lib/refusal_filter_test.sh" >/dev/null 2>&1; then
    echo "ok   refusal-filter"
  else
    echo "FAIL refusal-filter: the negation must precede the command"; bad=1
  fi
  # The body is dot-sourced by each scenario's check.sh: a scan resolved
  # relative to "$0" pointed at the scenario dir and left the (a) clause
  # undefined — a checker that quietly stopped checking.
  if bash "$EVALS/_lib/skills_inject_sourcing_test.sh" >/dev/null 2>&1; then
    echo "ok   refusal-filter-sourcing"
  else
    echo "FAIL refusal-filter-sourcing: the body cannot source its scan"; bad=1
  fi
  exit "$bad"
fi

if [ -f "$REPO/.env" ]; then set -a; . "$REPO/.env"; set +a; fi
[ -n "${OPENCODE_GO_KEY:-}" ] || { echo "OPENCODE_GO_KEY not set (repo .env missing?)"; exit 2; }
[ -x "$MOCA" ] || { echo "build moca first: make build (looked for $MOCA)"; exit 2; }
mkdir -p "$EVALS/results"
OUT="$EVALS/results/$LABEL.jsonl"
RESULTS_DIR="$EVALS/results/$LABEL"
KEEP=200 # newest transcripts kept per label (results/ is gitignored)
STATS="$EVALS/results/.evalstats"
(cd "$REPO" && go build -o "$STATS" ./test/evals/evalstats) || exit 2

# one <scenario> <run>: a fresh repo + data/config dirs, one moca -p run per
# prompt (sessions.txt: several against the same workdir), the checker once
# after the last, then one metrics line per session. Artifacts of a failed run
# are kept for inspection.
one() {
  local s="$1" n="$2" work data cfg flags=() code pass=false why
  work=$(mktemp -d "${TMPDIR:-/tmp}/moca-eval-$s.XXXXXX")
  data=$(mktemp -d "${TMPDIR:-/tmp}/moca-eval-data.XXXXXX")
  cfg=$(mktemp -d "${TMPDIR:-/tmp}/moca-eval-cfg.XXXXXX")
  fresh_repo "$s" "$work"
  [ -f "$EVALS/$s/flags" ] && read -r -a flags < "$EVALS/$s/flags"
  [ -n "$MODEL" ] && flags+=(--model "$MODEL")
  local prompts=()
  if [ -f "$EVALS/$s/sessions.txt" ]; then
    while IFS= read -r line || [ -n "$line" ]; do
      [ -n "$line" ] && prompts+=("$line")
    done < "$EVALS/$s/sessions.txt"
  else
    prompts+=("$(cat "$EVALS/$s/prompt.txt")")
  fi
  local i=0 sess="" last code=1
  for p in "${prompts[@]}"; do
    i=$((i + 1))
    (cd "$work" && XDG_DATA_HOME="$data" XDG_CONFIG_HOME="$cfg" XDG_STATE_HOME="$data" timeout 900 \
      "$MOCA" --config "$(scenario_config "$s")" "${flags[@]}" -p "$p" \
      > "$data/stdout.$i.txt" 2> "$data/stderr.$i.txt")
    last=$?
    [ "$i" = 1 ] && code=$last
    [ "$last" != 0 ] && code=$last
  done
  sess=$(ls -t "$data"/moca/sessions/*.jsonl 2>/dev/null | head -1)
  if why=$(cd "$work" && SESSION="$sess" DATA="$data" VARIANT="${VARIANT:-on}" bash "$EVALS/$s/check.sh" 2>&1); then pass=true; fi
  # Secret scan (B4): the provider key must never reach a transcript, a log or
  # the captured output. Guarded — an empty pattern matches every line — and it
  # runs before the pass cleanup below deletes the files it scans. A hit never
  # prints the value. Every session, every stdout/stderr file and every
  # diagnostic log is scanned, not just the newest session: XDG_STATE_HOME
  # points at the run's data dir, so the logs land under it (moca writes
  # <state>/logs, and without it they would escape both the scan and the
  # cleanup into the host's real state dir).
  local leaked=false
  if [ -n "${OPENCODE_GO_KEY:-}" ]; then
    local f
    for f in "$data"/moca/sessions/*.jsonl "$data"/moca/logs/* "$data"/stdout.*.txt "$data"/stderr.*.txt; do
      [ -f "$f" ] || continue
      if grep -qF -- "$OPENCODE_GO_KEY" "$f"; then
        pass=false
        leaked=true
        why="secret leaked into $(basename "$f")" # the file, never the value
      fi
    done
    # The workdir too: a run that echoed the key into a file leaked it just as
    # badly, and that copy would otherwise survive the data-dir cleanup.
    if grep -rqF -- "$OPENCODE_GO_KEY" "$work" 2>/dev/null; then
      pass=false
      leaked=true
      why="secret leaked into the workdir"
    fi
  fi
  if [ -n "$sess" ]; then
    # Keep every session transcript: a passing run's data dir is deleted below,
    # and a regression inside a passing run is undiagnosable after that
    # (session 1's transcript is the one the skills scenarios measure). A run
    # that LEAKED the provider key keeps its metrics row and nothing else.
    mkdir -p "$RESULTS_DIR"
    local j=0
    for f in $(ls -tr "$data"/moca/sessions/*.jsonl 2>/dev/null); do
      j=$((j + 1))
      local args=(-session "$f" -scenario "$s.$j" -label "$LABEL" -run "$n" -exit "$code")
      [ "$pass" = true ] && args+=(-pass)
      "$STATS" "${args[@]}" >> "$OUT"
      if [ "$leaked" = false ]; then
        cp "$f" "$RESULTS_DIR/$s.$j-$n.jsonl"
      fi
    done
  fi
  if [ "$pass" = true ]; then
    echo "pass $s#$n (exit $code)"
    rm -rf "$work" "$data" "$cfg"
  elif [ "$leaked" = true ]; then
    # Never leave the key on disk for a post-mortem: the checker's reason and
    # the metrics row are all a leaking run should leave behind.
    rm -rf "$work" "$data" "$cfg"
    echo "FAIL $s#$n (exit $code): $why — removed the run's files"
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
# Retention: keep the newest KEEP transcripts per label.
if [ -d "$RESULTS_DIR" ]; then
  ls -1t "$RESULTS_DIR"/*.jsonl 2>/dev/null | tail -n "+$((KEEP + 1))" | while read -r f; do rm -f "$f"; done
fi
echo "results: $OUT"
"$STATS" -summary "$OUT"
