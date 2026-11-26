#!/usr/bin/env bash
# Two-turn trial runner (codemode plan v13, Task 1 / E2). Sourced by run.sh,
# which owns the corpus globals this file reads: EVALS, MOCA, LABEL, MODEL,
# VARIANT, STATS, OUT, RESULTS_DIR, and the helpers fresh_repo, size_of and
# scenario_config. The harness is one program split in two so run.sh stays
# readable; nothing here is safe to call on its own.
#
# A trial is two processes over ONE session: the first answers prompt.txt, the
# second resumes with --continue and answers followup.txt. What must be proven
# is not just "the checker liked the last answer" — a fresh session in process
# two would be a cheaper history for the wrong reason — so the harness checks
# the session count, the transcript's turn boundaries and its two answers, the
# carried-history observation, and both stdout/stderr/exit records.

# is_pilot <scenario>: the opt-in marker (R36/R43). Default discovery skips
# these directories; naming one explicitly is the only way to run it.
is_pilot() { [ -f "$EVALS/$1/PILOT" ]; }

# pilot_preflight <scenario...>: a pilot run needs its own arm file, and the
# arm has to be named with -c. Refuses before any binary runs, so a label can
# never quietly measure the baseline config (E4/G4).
pilot_preflight() {
  local s bad=0
  for s in "$@"; do
    is_pilot "$s" || continue
    # A pilot spends real money, so its ceiling is part of the approval and
    # cannot be left to memory: no -cap, no run.
    if [ -z "${CAP_USD:-}" ]; then
      echo "pilot scenario $s needs -cap <usd> — the spend ceiling is part of the approval, not an honour system"
      bad=1
    fi
    # A factorial pilot ships more than one arm, and its arms are separate
    # labels: -cap alone would hand each one its own ceiling, which is how three
    # labels x a $20 cap becomes a $60 campaign. One ledger, one scope.
    if [ -n "${CAP_USD:-}" ] && [ -z "${CAP_SCOPE:-}" ]; then
      arms=$(ls "$EVALS/$s"/config.cm-*.jsonc 2>/dev/null | wc -l)
      if [ "$arms" -gt 1 ]; then
        echo "pilot scenario $s ships $arms arms: -cap alone caps each arm separately — pass -cap-scope '$EVALS/results/campaign.*.jsonl' so the ceiling covers the campaign"
        bad=1
      fi
    fi
    # A scope that matches nothing guards nothing — but a brand-new campaign
    # legitimately matches nothing yet. The two are told apart by the scope's own
    # family: if files exist under the literal prefix but the glob misses them,
    # the glob is a typo; if the family is empty, this is its first run.
    if [ -n "${CAP_SCOPE:-}" ]; then
      family=$(printf '%s' "$CAP_SCOPE" | sed 's/[][*?].*//')
      if [ -n "$family" ] && compgen -G "$family*" >/dev/null && ! compgen -G "$CAP_SCOPE" >/dev/null; then
        echo "-cap-scope $CAP_SCOPE matches no results file although $family* exists — a scope whose family is there but whose glob misses it is a typo, and a typo is a ceiling that guards nothing"
        bad=1
      fi
    fi
    case "$VARIANT" in
      cm-*)
        # Any arm of the codemode family, so a model arm (cm-on.glm) runs on its
        # own config — but only if that config exists: the refusal this whole
        # case exists for is an arm label measured on another arm's config.
        if [ ! -f "$EVALS/$s/config.$VARIANT.jsonc" ]; then
          echo "pilot scenario $s has no config.$VARIANT.jsonc — refusing to run an arm label on another arm's config"
          bad=1
        fi ;;
      "")
        echo "pilot scenario $s is opt-in and needs -c cm-on|cm-off (or a model arm, e.g. cm-on.glm)"
        bad=1 ;;
      *)
        echo "pilot scenario $s cannot run under -c $VARIANT (its arms are cm-*; 'on'/'off' are the corpus skills arm)"
        bad=1 ;;
    esac
  done
  return "$bad"
}

# secret_leak <data> <work>: sets LEAK_WHY (never the value) when the provider
# key reached a transcript, a log, a captured stream or the workdir. Guarded:
# an unset key would otherwise match every line.
LEAK_WHY=""
secret_leak() {
  local data="$1" work="$2" f
  LEAK_WHY=""
  [ -n "${OPENCODE_API_KEY:-}" ] || return 0
  for f in "$data"/moca/sessions/*.jsonl "$data"/moca/logs/* "$data"/stdout.*.txt "$data"/stderr.*.txt; do
    [ -f "$f" ] || continue
    if grep -qF -- "$OPENCODE_API_KEY" "$f"; then
      LEAK_WHY="secret leaked into $(basename "$f")" # the file, never the value
      return 0
    fi
  done
  if grep -rqF -- "$OPENCODE_API_KEY" "$work" 2>/dev/null; then
    LEAK_WHY="secret leaked into the workdir"
  fi
}

# run_trial <scenario> <n>: one schema-2 row (or a named failure). Mirrors
# one()'s artifact handling — a passing trial's directories go away, a failed
# one's are kept — and adds the trial's own contract checks.
run_trial() {
  local s="$1" n="$2"
  local work data cfg flags=()
  work=$(mktemp -d "${TMPDIR:-/tmp}/moca-eval-$s.XXXXXX")
  data=$(mktemp -d "${TMPDIR:-/tmp}/moca-eval-data.XXXXXX")
  cfg=$(mktemp -d "${TMPDIR:-/tmp}/moca-eval-cfg.XXXXXX")
  fresh_repo "$s" "$work"
  local base="$REPO_BASE"
  [ -f "$EVALS/$s/flags" ] && read -r -a flags < "$EVALS/$s/flags"
  [ -n "$MODEL" ] && flags+=(--model "$MODEL")
  local modeargs=(--mode "${MODE:-ask}")
  local arm="${VARIANT:-on}"
  local cfgfile; cfgfile=$(scenario_config "$s")
  local p1f="$EVALS/$s/prompt.txt" p2f="$EVALS/$s/followup.txt"

  local t0 t1 ms code1 code2
  t0=$(date +%s%N)
  # The env assignments go BEFORE `timeout`: `timeout 900 VAR=x cmd` would ask
  # timeout to exec a program named VAR=x, and the carry hook would observe
  # nothing.
  (cd "$work" && XDG_DATA_HOME="$data" XDG_CONFIG_HOME="$cfg" XDG_STATE_HOME="$data" \
    MOCA_CARRY_OUT="$data/carry.1.jsonl" MOCA_TRIAL="$n" timeout 900 \
    "$MOCA" --config "$cfgfile" "${modeargs[@]}" ${flags[@]+"${flags[@]}"} -p "$(cat "$p1f")" \
    > "$data/stdout.1.txt" 2> "$data/stderr.1.txt")
  code1=$?
  (cd "$work" && XDG_DATA_HOME="$data" XDG_CONFIG_HOME="$cfg" XDG_STATE_HOME="$data" \
    MOCA_CARRY_OUT="$data/carry.2.jsonl" MOCA_TRIAL="$n" timeout 900 \
    "$MOCA" --config "$cfgfile" "${modeargs[@]}" ${flags[@]+"${flags[@]}"} --continue -p "$(cat "$p2f")" \
    > "$data/stdout.2.txt" 2> "$data/stderr.2.txt")
  code2=$?
  t1=$(date +%s%N)
  ms=$(( (t1 - t0) / 1000000 ))

  local code=$code1
  [ "$code2" != 0 ] && code=$code2
  local sessions; sessions=$(ls -t "$data"/moca/sessions/*.jsonl 2>/dev/null)
  local nsess; nsess=$(printf '%s\n' "$sessions" | sed '/^$/d' | wc -l)
  local sess; sess=$(printf '%s\n' "$sessions" | head -1)
  local id8=""
  if [ -n "$sess" ]; then
    id8=$(basename "$sess" .jsonl)
    id8=${id8##*-}
  fi

  local why=""
  if [ -z "$sess" ]; then
    why="no session transcript"
  elif [ "$nsess" -ne 1 ]; then
    # --continue appends to the session the first process created. Two files
    # mean the second process started a new conversation: it would carry
    # nothing, which is exactly what this instrument must not mistake for a win.
    why="a new session instead of --continue ($nsess session files)"
  elif [ -z "$(cat "$data/stdout.1.txt")" ]; then
    why="process one wrote nothing to stdout (missing first answer)"
  elif ! out=$("$STATS" -trialCheck "$sess" -p1 "$p1f" -p2 "$p2f" 2>&1); then
    why="transcript: $out"
  fi
  # The carry observation of process two, turn 1: the history the resumed
  # process actually sent. A real zero is a measurement; an absent file is not,
  # so it is recorded as the gap it is and never as a zero.
  local carry=""
  local carryargs=(-carryMissing)
  if carry=$("$STATS" -carryOf "$data/carry.2.jsonl,1" 2>/dev/null); then
    carryargs=(-carry "$carry")
  else
    why="${why:-missing carry observation (build with -tags codemodecarry)}"
  fi

  # Work is the changeset campaign's endpoint: one observation per turn()
  # boundary, so the sum over both processes is the number of model requests
  # the trial spent making progress. Absent is not zero (same rule as carry),
  # and the attempts figure is a diagnostic beside it, never added to it.
  local work="" workargs=(-workMissing)
  if work=$("$STATS" -workOf "$data/carry.1.jsonl,$data/carry.2.jsonl" 2>/dev/null); then
    workargs=(-work "$work")
  else
    why="${why:-missing work observation (build with -tags codemodecarry)}"
  fi
  local attemptargs=()
  local attempts=""
  if attempts=$("$STATS" -attemptsOf "$data/carry.1.jsonl,$data/carry.2.jsonl" 2>/dev/null); then
    attemptargs=(-attempts "$attempts")
  fi
  local pass=false
  if [ -z "$why" ]; then
    if why=$(cd "$work" && SESSION="$sess" DATA="$data" VARIANT="$arm" bash "$EVALS/$s/check.sh" 2>&1); then
      pass=true
      why=""
    fi
  fi
  secret_leak "$data" "$work"
  local leaked=false
  if [ -n "$LEAK_WHY" ]; then
    pass=false
    leaked=true
    why="$LEAK_WHY"
  fi

  if [ -n "$sess" ]; then
    mkdir -p "$RESULTS_DIR"
    local s_added=0 s_removed=0 s_files=0 s_new=0
    read -r s_added s_removed s_files s_new <<<"$(size_of "$work" "$base")"
    local args=(-session "$sess" -scenario "$s" -label "$LABEL" -schema 2 -trial "$n" -turns 2
      -arm "$arm" -sessionId "$id8" -ms "$ms" -exit "$code"
      "${workargs[@]}" ${attemptargs[@]+"${attemptargs[@]}"}
      "${carryargs[@]}" -added "$s_added" -removed "$s_removed" -files "$s_files" -newFiles "$s_new")
    [ "$pass" = true ] && args+=(-pass)
    "$STATS" "${args[@]}" >> "$OUT"
    if [ "$leaked" = false ]; then
      cp "$sess" "$RESULTS_DIR/$s.1-$n.jsonl"
    fi
  fi

  if [ "$pass" = true ]; then
    echo "pass $s#$n (exit $code)"
    rm -rf "$work" "$data" "$cfg"
  elif [ "$leaked" = true ]; then
    rm -rf "$work" "$data" "$cfg"
    echo "FAIL $s#$n (exit $code): $why — removed the run's files"
  else
    echo "FAIL $s#$n (exit $code): ${why:-no session} — kept $work $data"
  fi
}

# spend_total <jsonl>: the dollars already recorded for a label, 6dp.
spend_total() {
  [ -f "$1" ] || { echo "0.000000"; return; }
  grep -o '"cost":[0-9eE.+-]*' "$1" 2>/dev/null | cut -d: -f2 |
    awk '{s += $1} END {printf "%.6f", s + 0}'
}

# spend_total_scope <glob>: the dollars recorded across every results file the
# scope names, 6dp. A campaign is several labels — a factorial's arms are
# separate runs — and one ceiling has to see all of them (R81).
spend_total_scope() {
  local f total=0
  for f in $1; do
    [ -f "$f" ] || continue
    total=$(awk -v a="$total" -v b="$(spend_total "$f")" 'BEGIN {printf "%.6f", a + b}')
  done
  printf "%.6f" "$total"
}

# spend_total_for <jsonl>: the number the ceiling is checked against — the
# scope's total when -cap-scope is set, the label's own file otherwise.
spend_total_for() {
  if [ -n "${CAP_SCOPE:-}" ]; then
    spend_total_scope "$CAP_SCOPE"
  else
    spend_total "$1"
  fi
}

# spend_guard <jsonl> <cap>: true when the recorded spend has reached the cap.
# Checked before each run is launched, so a serial pilot stops on the run that
# would cross the line rather than after it.
spend_guard() {
  [ -n "${2:-}" ] || return 1
  awk -v t="$(spend_total_for "$1")" -v c="$2" 'BEGIN {exit !(t >= c)}'
}
