// Package tools defines the Tool interface and the built-in tools. It never
// imports permissions or session; those implement the small interfaces below
// structurally and agent injects them through Env.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"github.com/adeotek/moca/internal/llm"
)

type Result struct {
	Content string
	IsError bool
	Summary string
	Detail  string
}

type Tool interface {
	Spec() llm.ToolSpec
	Run(ctx context.Context, env *Env, input json.RawMessage) Result
}

type PathChecker interface {
	Resolve(path string, write bool) (string, error)
}

type CommandChecker interface {
	Check(command string) (needApproval, askEveryTime, unsure []string, err error)
	Allow(name string)
}

type Snapshotter interface {
	Guard(absPath string, write func() error) error
}

type Answer int

const (
	Deny Answer = iota
	AllowOnce
	AllowAlways
	// AllowSession allows the question's session key for the rest of this
	// session: in memory only, gone when the Agent is replaced (/clear,
	// /resume, a new ACP session) and never written to any config file.
	AllowSession
)

// String is the log/UX spelling of an answer. It is the safe indexing form
// too: an out-of-range Answer must never panic a tool loop, and it must not
// read as a real answer either.
func (a Answer) String() string {
	switch a {
	case AllowOnce:
		return "once"
	case AllowAlways:
		return "always"
	case AllowSession:
		return "session"
	case Deny:
		return "deny"
	}
	return "invalid"
}

// Question is an approval request. Kind is "shell", "mcp", "web" (a fetch of a private address) or "write"
// (a tool write outside the workdir: the global prompt templates — slash
// commands the agent writes for the user); Subject the command name, the
// server/tool or the path; Detail the full command, the args or the reason.
type Question struct {
	Kind, Subject, Detail string
	CanAlways             bool
	// NoSession marks a question that must keep asking and must never be
	// answered "for this session": moca's own config file (it holds the
	// permission policy itself) and a web fetch's redirect hop (approving a
	// page must not open the host it is steered to).
	NoSession bool
}

// SessionExact reports whether a session allow must match the exact command
// line rather than the command name. A shell question that is not offered
// allow-always (rm, a rule-Ask, auto's ask-every-time, an unlearnable name)
// can never be learned by name, so only the full line is a meaningful key.
func (q Question) SessionExact() bool { return q.Kind == "shell" && !q.CanAlways }

// SessionKey is the identity a session allow is remembered under. The tier
// separator matters: a session `git` (name tier) must never answer a
// rule-Ask `git push` (exact tier), so the exact tier appends the detail.
// Kinds never mix either — a shell `fetch example.com` is not a web allow.
func (q Question) SessionKey() string {
	k := q.Kind + "\x00" + q.Subject
	if q.SessionExact() {
		k += "\x00" + q.Detail
	}
	return k
}

type Asker func(ctx context.Context, q Question) Answer

// AutoAllow is the yolo-mode Asker: every approval is granted once.
func AutoAllow(context.Context, Question) Answer { return AllowOnce }

// ApprovablePaths is the optional PathChecker capability behind ask-write
// roots: a jail may point out a refused write the user can approve (the
// global prompt templates). permissions.Jail implements it.
type ApprovablePaths interface {
	Approvable(path string) (string, bool)
}

// LearnableCommands is the optional CommandChecker capability behind the
// allow-always offer: Learnable reports whether Allow(name) would record
// the name (permissions.Shell never learns hard-deny or ask-every-time
// names such as rm). Checkers without it offer allow-always as before.
type LearnableCommands interface {
	Learnable(name string) bool
}

// askOutsideWrite resolves a write the jail refused. A target under an
// ask-write root (the global prompt templates) is allowed only on the user's
// yes; anything else keeps the refusal, and no asker (a -p run) means denial.
func askOutsideWrite(ctx context.Context, env *Env, path string, refused error) (string, error) {
	ap, ok := env.Paths.(ApprovablePaths)
	if !ok {
		return "", refused
	}
	abs, ok := ap.Approvable(path)
	if !ok {
		return "", refused
	}
	// The read guard comes first: an unread (or stale) existing file is
	// refused anyway, and prompting for it would only spend the user's
	// attention on a write that cannot land.
	if _, err := os.Stat(abs); err == nil && env.Reads != nil {
		if rerr := env.Reads.Check(abs); rerr != nil {
			return "", rerr
		}
	}
	// The resolved target, not the model's spelling of it: the user approves
	// the file that will actually be written.
	q := Question{Kind: "write", Subject: abs,
		Detail: "outside the workdir — moca slash commands (global prompt templates)"}
	// moca's own config file holds the permission policy itself: a session
	// allow there would let one approval widen the rules for the rest of the
	// session, so that question always asks. Its path is resolved here, by
	// the same jail and at the same moment as the target — no start-time
	// copy to drift — and one that cannot be resolved fails closed: whether
	// this target is the config file cannot be told, so it is treated as if
	// it were (the cost is a missing session row, never a remembered write).
	if env.ConfigFile != "" {
		switch cfg, err := env.Paths.Resolve(env.ConfigFile, false); {
		case err != nil:
			q.NoSession = true
			slog.Debug("ask-write: config file unresolved, session answer withheld", "error", err)
		case samePath(cfg, abs):
			q.NoSession = true
			q.Detail = "moca's own config file — it holds the permission policy"
		}
	}
	if env.Ask == nil || env.Ask(ctx, q) == Deny {
		return "", fmt.Errorf("%s is outside the workdir and the user did not approve writing it — keep project slash commands in .moca/prompts/ inside the workdir, or ask the user first", path)
	}
	return abs, nil
}

// samePath reports whether two resolved paths name the same file: byte-equal,
// or EqualFold per component where the filesystem folds case — the same fold
// the jail's own containment test applies, so spelling the config path in
// another case cannot side-step the session answer's exclusion.
func samePath(a, b string) bool {
	if a == b {
		return true
	}
	if !caseInsensitiveFS {
		return false
	}
	ac, bc := foldComps(a), foldComps(b)
	if len(ac) != len(bc) {
		return false
	}
	for i := range ac {
		if !strings.EqualFold(ac[i], bc[i]) {
			return false
		}
	}
	return true
}

type Env struct {
	Root     string
	Paths    PathChecker
	Commands CommandChecker
	Ask      Asker
	Reads    *ReadTracker
	Snap     Snapshotter
	ShellEnv []string
	// ConfigFile is moca's own user config file as configured (unresolved).
	// The ask-write approval resolves it through Paths when it asks and
	// compares the target against that, so a write there is never offered
	// "for this session" (it holds the permission policy).
	ConfigFile string
	// AutoJail carries the auto-mode path checker (the default jail with the
	// temp roots writable) from start.go to the agent's checker setup; nil
	// for Envs built by hand.
	AutoJail PathChecker
	// WebProvider/WebKey are the resolved web.search config for the web
	// tool (tavily|exa; the empty key is tavily's keyless mode).
	WebProvider string
	WebKey      string
	// SpillDir/SpillPrefix: where over-cap outputs are saved in full
	// (overflow.go); empty SpillDir turns spilling off.
	SpillDir    string
	SpillPrefix string
	// Plan/PlanWrote drive plan mode (rev 20): Plan confines write/edit to
	// docs/plans/*.md and suppresses the failing-test investigation banner
	// (plan runs inspect, they don't fix); PlanWrote is set by a successful
	// write/edit there and read by the agent at run end.
	Plan      bool
	PlanWrote bool
	// Judge clears the auto table's unsure tier with one classifier call
	// (built by the agent in auto mode when permissions.auto.classifier is
	// on; nil otherwise — the tier then lands in the ask ladder).
	Judge func(ctx context.Context, command string) (allow bool, reason string)
	// Edited/Unverified/MaxRepeat are per-run (runstate.go, reset by
	// BeginRun): Edited is set by any successful write/edit; Unverified
	// names the last code file changed since a verifying shell command;
	// MaxRepeat is the highest identical-failure count of the run.
	Edited     bool
	Unverified string
	MaxRepeat  int
	failures   map[string]int
	// TestSeen/TestFailed/FailingTest/Searched drive the investigation hints:
	// a failing test run sets TestFailed and FailingTest (the file name parsed
	// from its output); reading any *_test.go sets TestSeen and clears
	// FailingTest; a search call sets Searched; a green test run clears
	// TestFailed/FailingTest (shell.go/read.go/search.go).
	TestSeen    bool
	TestFailed  bool
	FailingTest string
	Searched    bool
	// copied holds the stateful files (abs paths) the run has byte-copied:
	// the SQLite sidecar guard stops refusing their opens then (§10).
	// Per-run (runstate.go, reset by BeginRun).
	copied map[string]struct{}
	// SkillDir is the project's skills dir, symlink-resolved — build it with
	// jail.Resolve(".moca/skills", true), never filepath.Join(Root(), …); ""
	// = no self-managed skills for this session. SkillGuard validates a write
	// under it lands; rel is abs relative to SkillDir, slash-separated. Both
	// are set together (agent.build) or not at all. Structural on purpose:
	// internal/tools must not import internal/skills (§2).
	SkillDir   string
	SkillGuard func(abs, rel string, data []byte) error
	// SkillGuardRefusing marks C5b's guard: installed on a resumed session
	// whose frozen prompt still carries the block after the switch went off.
	// A guard must be installed there (block ⇒ guard), but it only refuses —
	// and nothing may answer "this session learns" from SkillGuard != nil
	// alone. The single predicate is Env.SkillLearning().
	SkillGuardRefusing bool
	// SkillCreations/SkillUserSignal/SkillTyped/SkillHintArmed/SkillHintDone
	// are per-run (runstate.go, reset by BeginRun): creations counts SKILL.md
	// files created under SkillDir this run, after they landed; the user
	// signal gates new (user, …) rules (Decision 11 — set ONLY by
	// Agent.Remember); SkillTyped is the run's typed texts (Decision 15);
	// the hint fields drive the one-shot nudge.
	// Executed is the run's own shell segments (ExecutedSegments), appended by
	// Registry.Run for every call that actually ran — the guard's evidence for
	// the command-claim rule (a recorded rule may only cite a command this
	// session ran). Per-run (runstate.go, reset by BeginRun).
	Executed        []string
	SkillCreations  int
	SkillUserSignal bool
	SkillTyped      []string
	SkillHintArmed  bool
	SkillHintDone   bool
	// SkillCold is set by the agent when the project carries no recorded
	// auto skill: it arms the hint on the run's first successful verification
	// (see Registry.Run), the moment a cold repo's own procedure gets proved.
	// guardedWrite clears it once this session creates a skill.
	SkillCold bool
	// SkillRecordNudge is the §12.1 end-of-run recording nudge's switch
	// (skills.recordNudge, on by default, resolved by the agent at build).
	// With it set the mid-run hint is never delivered — one invitation per
	// run, and it lands at the end where it cannot be scrolled past — while
	// every arming trigger below still runs: SkillHintArmed is what the agent
	// reads to decide the record is due.
	SkillRecordNudge bool
	// SkillProvenCmd is the verifying command that armed a recording this run
	// (a recovery or a cold repo's first green check), first line only — the
	// fact the run actually proved. The end-of-run nudge names it, so the
	// record starts from something the run established rather than from
	// whatever the model found most interesting (an injected "NOTE TO AI
	// CODING AGENTS" got recorded as a rule without this: BASELINE.md,
	// v0.7.1's injection re-check).
	SkillProvenCmd string
	// lastFailedVerify is the recovery trigger's state: the normalized
	// command of the last failing verifying shell call (shell.go), cleared by
	// noteChange (an edit in between means the fix was in the code).
	lastFailedVerify string
	// skillRefused/skillWrote/internalRefusal are per-call (reset by
	// Registry.Run, set by guardedWrite / the internal guards): the current
	// call was refused by the skill guard / landed a write to a skill file
	// (SKILL.md) under SkillDir / came from an internal guard.
	skillRefused    bool
	skillWrote      bool
	internalRefusal bool
}

// SkillLearning reports whether this session may learn: a VALIDATING guard is
// installed. False with the feature off (nil guard) and false for C5b's
// refusing guard — the only state where the two differ, and the one that must
// not advertise the feature (Agent.SelfManagedSkills reads it too).
func (e *Env) SkillLearning() bool { return e.SkillGuard != nil && !e.SkillGuardRefusing }

type Registry struct {
	order []string
	tools map[string]Tool
}

func NewRegistry(ts ...Tool) *Registry {
	r := &Registry{tools: map[string]Tool{}}
	for _, t := range ts {
		r.Register(t)
	}
	return r
}

func (r *Registry) Register(t Tool) {
	name := t.Spec().Name
	if _, ok := r.tools[name]; !ok {
		r.order = append(r.order, name)
	}
	r.tools[name] = t
}

func (r *Registry) Specs() []llm.ToolSpec {
	out := make([]llm.ToolSpec, len(r.order))
	for i, n := range r.order {
		out[i] = r.tools[n].Spec()
	}
	return out
}

func (r *Registry) Run(ctx context.Context, env *Env, call llm.ToolCall) Result {
	t, ok := r.tools[call.Name]
	if !ok {
		return errorf("unknown tool %q; available: %s", call.Name, strings.Join(r.order, ", "))
	}
	if !json.Valid(call.Input) {
		return errorf("invalid JSON arguments for %s (the call was probably cut off at the output limit). "+
			"Split the work into smaller calls, e.g. write a large file in parts with edit.", call.Name)
	}
	env.skillRefused, env.skillWrote, env.internalRefusal = false, false, false // per-call flags
	start := time.Now()
	res := runRecovered(ctx, t, env, call)
	// A shell call that ran is evidence for the command-claim rule. A refusal
	// (permission, guard, plan-mode) proves nothing about the command and must
	// not count — the model may not cite what it was not allowed to run.
	if call.Name == "shell" && !strings.HasPrefix(res.Content, "refused:") {
		env.Executed = append(env.Executed, ExecutedFromCall(call)...)
		if len(env.Executed) > executedCap {
			env.Executed = env.Executed[:executedCap]
		}
	}
	slog.Debug("tool", "name", call.Name, "duration", time.Since(start).Round(time.Millisecond),
		"is_error", res.IsError, "bytes", len(res.Content))
	if env.skillWrote {
		env.SkillHintDone = true // unconditional — before any arming logic (H4)
	}
	if res.IsError {
		rh := trackFailure(env, call.Name, call.Input, res.Content)
		res.Content += rh
		if env.SkillLearning() && !env.skillRefused && !env.internalRefusal &&
			(rh != "" || strings.HasPrefix(res.Content, "refused:")) { // both branches guarded (H2)
			env.SkillHintArmed = true
		}
	} else if env.lastFailedVerify != "" && verifyingCall(call) &&
		normCommand(cmdOf(call)) != env.lastFailedVerify {
		env.SkillHintArmed = true     // recovery (H1) — skillHint delivers on this same result
		if env.SkillProvenCmd == "" { // first proof wins: a later call cannot overwrite it
			env.SkillProvenCmd = callCommand(call)
		}
	} else if env.SkillCold && verificationShaped(call) {
		// Cold repo: the first verification that succeeds there is itself the
		// durable fact ("this is how the repo is checked"), and waiting for a
		// recovery means the nudge almost never arms in the ordinary flow —
		// measured at 0/6 in test/evals/skills-cold (BASELINE.md, v0.7.0).
		env.SkillHintArmed = true
		if env.SkillProvenCmd == "" {
			env.SkillProvenCmd = callCommand(call)
		}
	}
	if h := investigationHint(env); h != "" {
		res.Content += h
	}
	if h := skillHint(env, call, res); h != "" {
		res.Content += h
	}
	return res
}

// runRecovered runs one tool call, turning a panic inside the tool into an
// error result: a bug in one tool must not kill the session (and lose the
// run) — the model is told the call may have partly applied.
func runRecovered(ctx context.Context, t Tool, env *Env, call llm.ToolCall) (res Result) {
	defer func() {
		if p := recover(); p != nil {
			slog.Error("tool panic", "tool", call.Name, "panic", fmt.Sprint(p), "stack", string(debug.Stack()))
			res = errorf("internal error in %s: %v — this is a moca bug, not your input; the call may have partly applied, so check the state (e.g. re-read the file) before retrying", call.Name, p)
		}
	}()
	return t.Run(ctx, env, call.Input)
}

// investigationHint is the protocol banner appended to every tool result
// while a failing test run is unresolved: weak models fix from the failure
// text (or shell out entirely with cat/sed) without reading the failing test
// or locating the cause with search; the §14 demo requires both before the
// fix. State: shell.go (TestFailed/FailingTest, cleared by a green run),
// read.go (TestSeen/FailingTest), search.go (Searched).
func investigationHint(env *Env) string {
	if env.Plan {
		// Plan mode inspects, it does not fix: a failing test run during
		// planning must not nag the fix-loop protocol on every result.
		return ""
	}
	if env.FailingTest != "" {
		return fmt.Sprintf("\n[hint: tests failed: (1) read the failing test file (%s) with the read tool, (2) locate the cause with the search tool, (3) only then edit]", env.FailingTest)
	}
	if env.TestFailed && !env.Searched {
		return "\n[hint: locate the cause with the search tool before editing]"
	}
	return ""
}

// investigationRefusal is the edit gate for the same protocol: while a
// failing test run's investigation is incomplete the edit tool refuses —
// weak models treat "read the file"/"locate the cause" as satisfied by
// shell equivalents (cat/sed), so hints alone lose; the guard is what binds.
func investigationRefusal(env *Env) string {
	if env.Plan {
		// Edits in plan mode can only touch docs/plans/*.md — the fix-loop
		// protocol governs code edits and must not block plan updates.
		return ""
	}
	if !env.TestFailed || (env.TestSeen && env.Searched) {
		return ""
	}
	if !env.TestSeen {
		name := env.FailingTest
		if name == "" {
			name = "*_test.go"
		}
		return fmt.Sprintf("refused: tests are failing — read the failing test file (%s) with the read tool and locate the cause with the search tool before editing", name)
	}
	return "refused: tests are failing — locate the cause with the search tool before editing"
}

// skillHintMark is the stable prefix of the skill-learning nudge's text (the
// tests match on it, so the prose below can change without rewriting them).
const skillHintMark = "[hint: record what this just proved"

// callCommand is a shell call's command, one line and short: the hint names
// the concrete fact it is asking about (the command that just worked) rather
// than describing the category of fact in the abstract — the abstract form was
// measured at 0/6 conversions on a cold repo (BASELINE.md, v0.7.0).
func callCommand(call llm.ToolCall) string {
	if call.Name != "shell" {
		return ""
	}
	var in struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(call.Input, &in); err != nil {
		return ""
	}
	c := strings.TrimSpace(strings.SplitN(strings.TrimSpace(in.Command), "\n", 2)[0])
	if r := []rune(c); len(r) > 120 {
		c = string(r[:120]) + "…"
	}
	return c
}

// skillHint is the one-shot skill-learning nudge (SPECS §11/§12). At most
// once per run; never in plan mode; never without a validating guard (C5b's
// refusing one does not count — it would invite a write it then rejects);
// never on a read, a search, or any non-verifying result.
func skillHint(env *Env, call llm.ToolCall, res Result) string {
	if env.Plan || !env.SkillLearning() || !env.SkillHintArmed || env.SkillHintDone || res.IsError {
		return ""
	}
	if env.skillWrote { // a landed skill write satisfies it before anything else (H4)
		env.SkillHintDone = true
		return ""
	}
	if env.SkillRecordNudge {
		// Option A (SPECS §12.1, v0.7.1): the invitation moves to the end of
		// the run (agent.recordNudge) — nothing is said mid-run, and the
		// arming flags stay as they are for the agent's end-of-run gate.
		return ""
	}
	if !verifyingCall(call) {
		return ""
	}
	env.SkillHintDone = true
	what := "that verification"
	if cmd := callCommand(call); cmd != "" {
		what = "`" + cmd + "`"
	}
	return fmt.Sprintf("\n%s — %s is how this repo verifies, and that is what .moca/skills/moca-<domain>/SKILL.md is for: write it now with write/edit (frontmatter `auto: true`, one `- **when**: rule (×1, <today>)` line), unless the file already records it]",
		skillHintMark, what)
}

func decode[T any](input json.RawMessage, v *T) *Result {
	dec := json.NewDecoder(strings.NewReader(string(input)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		r := errorf("invalid arguments: %v", err)
		return &r
	}
	return nil
}

func errorf(format string, a ...any) Result {
	return Result{Content: fmt.Sprintf(format, a...), IsError: true}
}
