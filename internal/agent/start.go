package agent

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/mcp"
	"github.com/adeotek/moca/internal/permissions"
	"github.com/adeotek/moca/internal/provider"
	"github.com/adeotek/moca/internal/session"
	"github.com/adeotek/moca/internal/skills"
	"github.com/adeotek/moca/internal/tools"
)

// StartError marks a run-setup failure (data dir, session files, skills) as
// opposed to a config or model error; cmd maps it to a runtime exit code.
type StartError struct{ Err error }

func (e *StartError) Error() string { return e.Err.Error() }
func (e *StartError) Unwrap() error { return e.Err }

type StartOptions struct {
	Config  config.Config
	Workdir string
	Model   string
	Effort  string
	Trusted bool
	Mode    PermMode // default | auto | yolo (the plan's Defaults first, then this)
	Plan    bool
	// Headless marks a one-shot task run (-p): there a text-only reply with
	// no tool call at all is a defect (the action nudge applies), while an
	// interactive turn may legitimately be a question.
	Headless bool
	Ask      tools.Asker
	Emit     func(Event)
	HTTP     *http.Client
	Slug     string
	// ConfigPath is the user config file this run loaded (warnings name it);
	// "" means config.ConfigFile().
	ConfigPath string
}

// setup is what Start and Resume need before a session exists: the config
// with the --model override applied, the provider registry, the built-in
// skills dir and the path jail. It is built once and handed to build.
type setup struct {
	cfg        config.Config
	reg        *provider.Registry
	builtinDir string
	jail       *permissions.Jail
	spillDir   string // this process's overflow dir — the only one its jail can read
	// skills/skillDirs: the discovered, filtered skill list and the dirs it
	// came from — shared by Start (system prompt + the agent's picker list)
	// and Resume, and re-read by Agent.ReloadSkills after a run.
	skills    []skills.Skill
	skillDirs []skills.Dir
	// Self-managed project skills (SPECS §12): the switch, resolved once here,
	// and the project's skills dir — symlink-resolved by the jail's own
	// resolver — shared by the prompt block condition, build()'s guard, the
	// shell refusal, hygiene and the resume rule (C5a). No call site
	// re-resolves; a resolution error is handled in exactly one place.
	sm          config.SelfManagedState
	skillDir    string
	skillDirErr error
	sessionDate time.Time
	// smOn is the feature for a fresh session: switch on, trusted, the dir
	// resolved. Start's block, the header field and hygiene read it; build()
	// adds the resume rule on top.
	smOn bool
}

func prepare(o StartOptions, jailRoot string, applyHygiene bool) (*setup, error) {
	cfg := o.Config
	if o.Model != "" {
		cfg.Model = o.Model
	}
	// `--model ollama/x` or /clear after a switch: a local provider needs no
	// config entry, naming one of its models is the opt-in.
	if pn, _, err := config.SplitModel(cfg.Model); err == nil {
		cfg.UseProvider(pn)
	}
	hc := o.HTTP
	if hc == nil {
		hc = defaultHTTPClient()
	}
	emit := o.Emit
	reg, err := provider.NewRegistry(cfg, hc, func(n provider.RetryNotice) {
		if emit != nil {
			emit(Retry{n})
		}
	})
	if err != nil {
		return nil, err
	}
	if err := discoverLocal(reg, cfg, emit); err != nil {
		return nil, err
	}
	builtinDir, err := skills.ExtractBuiltins(config.DataDir())
	if err != nil {
		return nil, &StartError{err}
	}
	globalSkills := filepath.Join(config.ConfigDir(), "skills")
	// rtk and graphify are standard skills: planted here when missing and
	// never touched again, so the user's edits/overrides win. A config dir
	// that cannot be written must not stop moca — warn and carry on.
	if _, err := skills.SeedSkills(globalSkills); err != nil && emit != nil {
		emit(Warning{Text: fmt.Sprintf("could not seed the standard skills (rtk, graphify) into %s: %v", globalSkills, err)})
	}
	promptsDir := config.PromptsDir()
	// readOnly: the prompt templates are readable (the agent may update a
	// command it is shown), the global skills dir (self-configure may add a
	// skill), and the config file itself (self-knowledge answers "what is
	// configured now" from it). askWrite: a write to any of them — outside
	// the workdir — needs the user's approval (the write/edit tools ask;
	// the shell's redirect check does not, so config edits go through
	// write/edit and its snapshots either way). The config entry is the
	// FILE, not its directory: auth.json sits beside it and stays
	// unreachable (pinned by test).
	configFile := config.ConfigFile()
	// This process's overflow dir is readable too: over-cap tool outputs are
	// saved there in full and the cut result names the file (tools.Spill).
	// Only its own dir — other sessions' outputs (other projects, possibly
	// secrets) share the overflow area and must stay out of reach.
	spillDir, err := newSpillDir()
	if err != nil {
		return nil, &StartError{err}
	}
	jail, err := permissions.NewJail(jailRoot,
		[]string{globalSkills, builtinDir, promptsDir, spillDir, configFile},
		[]string{promptsDir, globalSkills, configFile})
	if err != nil {
		return nil, &StartError{err}
	}
	// The self-managed skills switch and its one-time messages (§3.3): the
	// user config is the only opt-in; a trusted project file can opt the
	// repository out (false) or declare it (true — accepted, never enabling
	// the feature alone, and it warns a user who has not opted in).
	sm := config.ResolveSelfManagedSkills(cfg)
	if o.Trusted && emit != nil {
		if sm.Warning != "" {
			emit(Warning{Text: sm.Warning})
		}
		if sm.Note != "" {
			emit(Note{Text: sm.Note})
		}
	}
	// The dir the feature writes into, resolved by the jail's own resolver so
	// a symlinked workdir, .moca or .moca/skills resolves exactly as a write
	// target would — never filepath.Join(jailRoot, ".moca", "skills").
	// Resolved for EVERY start and resume, trusted or not (Resolve creates
	// nothing): C5b installs the refusing guard on a resumed session whose
	// trust was withdrawn, and env.SkillDir must be non-empty there or
	// skillRel never routes a write to the guard — the block would be present
	// with a dead guard, and the write lands unvalidated. When resolution
	// itself fails (a broken symlink, .moca/skills replaced by a file) no
	// SkillDir exists to route to, and none is needed: every write into the
	// broken structure fails in this same resolver before any guard is
	// consulted — nothing lands unvalidated there either.
	skillDir, skillDirErr := jail.Resolve(filepath.Join(".moca", "skills"), true)
	smOn := sm.On && o.Trusted && skillDirErr == nil
	// Hygiene runs before discovery so the listing reflects the pruning. It
	// is interactive-Start only (never Resume: a deletion would contradict the
	// frozen prompt the session resumed with) and report-only in headless
	// runs, which must never touch the checkout.
	if applyHygiene && smOn {
		pruned, perr := skills.PruneAutoSkills(skillDir, time.Now(), o.Headless)
		if perr != nil && emit != nil {
			emit(Warning{Text: fmt.Sprintf("skill hygiene: %v", perr)})
		}
		if msg := hygieneSummary(pruned, o.Headless); msg != "" && emit != nil {
			emit(Warning{Text: msg})
		}
	}
	// Skill discovery happens once here, so Start's system prompt and a
	// resumed agent share one list — and the TUI's picker reads it off the
	// agent (Agent.Skills). The project dir belongs to the session's
	// workdir (jailRoot: for a resume that is the stored one, not the
	// process's cwd) and needs the trust decision.
	var skillDirs []skills.Dir
	if o.Trusted {
		skillDirs = append(skillDirs, skills.Dir{Path: filepath.Join(jailRoot, ".moca", "skills"), Source: "project"})
	}
	skillDirs = append(skillDirs, skills.Dir{Path: globalSkills, Source: "global"}, skills.Dir{Path: builtinDir, Source: "builtin"})
	sk, skErrs := skills.Discover(skillDirs)
	for _, se := range skErrs {
		if emit != nil {
			emit(Warning{Text: se.Error()})
		}
	}
	sk = filterSkills(sk)
	// The startup note: how many agent-learned skills the project carries —
	// trusted, whenever auto skills are listed, switch on or off (M10).
	if o.Trusted && emit != nil {
		if msg := autoSkillsNote(sk); msg != "" {
			emit(Note{Text: msg})
		}
	}
	// A trusted project's MCP servers join the user layer (§10.5): the project
	// config file's `mcp.servers` (cmd loaded it with the trust decision) plus
	// the ecosystem's project-level files (`.mcp.json`, `opencode.json[c]`,
	// `.pi/mcp.json`) — a cloned repo's servers start only for a workdir the
	// user chose to trust. Conflicts and skips warn with the server name and
	// the files involved.
	var projMCP config.MCPConfig
	if cfg.Project != nil {
		projMCP = cfg.Project.MCP
	}
	if o.Trusted && (len(mcp.ProjectSources(jailRoot)) > 0 || len(projMCP.Servers) > 0) {
		userSrc := o.ConfigPath
		if userSrc == "" {
			userSrc = config.ConfigFile()
		}
		merged, warns := mcp.MergeProject(jailRoot, userSrc, config.ProjectConfigFile(jailRoot), cfg.MCP.Servers, projMCP)
		cfg.MCP.Servers = merged
		for _, w := range warns {
			if emit != nil {
				emit(Warning{Text: w})
			}
		}
	}
	if o.Trusted && projMCP.IdleTimeout > 0 {
		cfg.MCP.IdleTimeout = projMCP.IdleTimeout
	}
	return &setup{cfg: cfg, reg: reg, builtinDir: builtinDir, jail: jail, spillDir: spillDir, skills: sk, skillDirs: skillDirs,
		sm: sm, skillDir: skillDir, skillDirErr: skillDirErr, sessionDate: time.Now(), smOn: smOn}, nil
}

// hygieneSummary renders one startup warning for what the hygiene pass did
// (or, in a headless run, would do).
func hygieneSummary(p skills.PrunedSkills, dryRun bool) string {
	var parts []string
	if p.RulesRemoved > 0 && len(p.Touched) > 0 {
		noun := "stale rules"
		if p.RulesRemoved == 1 {
			noun = "stale rule"
		}
		if dryRun {
			parts = append(parts, fmt.Sprintf("%d %s would be removed from %s — run moca interactively to apply", p.RulesRemoved, noun, strings.Join(p.Touched, ", ")))
		} else {
			parts = append(parts, fmt.Sprintf("removed %d %s from %s", p.RulesRemoved, noun, strings.Join(p.Touched, ", ")))
		}
	}
	if len(p.SkillsRemoved) > 0 {
		names := strings.Join(p.SkillsRemoved, ", ")
		if dryRun {
			parts = append(parts, fmt.Sprintf("%s holds no rules left and would be removed — run moca interactively to apply", names))
		} else {
			parts = append(parts, "removed empty "+names)
		}
	}
	if len(p.PinnedAged) > 0 {
		parts = append(parts, fmt.Sprintf("a pinned rule in %s is over a year old — review it", strings.Join(p.PinnedAged, ", ")))
	}
	if len(parts) == 0 {
		return ""
	}
	return "skill hygiene: " + strings.Join(parts, "; ")
}

// hasAutoSkill reports whether the project already carries a recorded skill:
// the hint's cold-repo arming only applies while there is nothing to read yet.
func hasAutoSkill(sk []skills.Skill) bool {
	for _, s := range sk {
		if s.Auto && s.Source == "project" {
			return true
		}
	}
	return false
}

// autoSkillsNote names how many auto skills a project carries (trusted, when
// the listing holds any — switch on or off).
func autoSkillsNote(sk []skills.Skill) string {
	var names []string
	for _, s := range sk {
		if s.Auto && s.Source == "project" {
			names = append(names, s.Name)
		}
	}
	if len(names) == 0 {
		return ""
	}
	noun := "skills"
	if len(names) == 1 {
		noun = "skill"
	}
	return fmt.Sprintf("self-managed skills: %d %s in this project (%s)", len(names), noun, strings.Join(names, ", "))
}

// discoverLocal reads a configured Ollama server's models into the registry
// and then checks that the configured models exist. A server that is down is
// not an error by itself — only if the default or hard model lives on it
// (Verify says why). Warnings concern the model in use only: a window the
// server did not report is a guess, and one under the minimum an agent can
// work in will truncate the prompt silently.
func discoverLocal(reg *provider.Registry, cfg config.Config, emit func(Event)) error {
	if _, ok := cfg.Providers["ollama"]; ok {
		d, _ := reg.DiscoverOllama(context.Background())
		if pn, id, err := config.SplitModel(cfg.Model); err == nil && pn == "ollama" && emit != nil {
			if w, found := d.Windows[id]; found {
				switch {
				case w < config.MinContextWindow:
					emit(Warning{fmt.Sprintf("ollama/%s has a %d-token context window, too small for an agent (system prompt and tools alone need several thousand): raise num_ctx in its Modelfile or start the server with OLLAMA_CONTEXT_LENGTH=32768", id, w)})
				case slices.Contains(d.Assumed, id):
					emit(Warning{fmt.Sprintf("ollama/%s: the server reported no context window; assuming %d. If its num_ctx is smaller, long sessions are silently truncated — start the server with OLLAMA_CONTEXT_LENGTH or set providers.ollama.models.%s.contextWindow to match", id, w, id)})
				}
			}
		}
	}
	return reg.Verify()
}

// build constructs everything Start and Resume share past setup: shell
// checker, tool env, snapshot store and the agent itself. Start pre-computes
// the system prompt and passes nil prior entries; Resume passes the stored
// prompt verbatim and the transcript it read back.
func build(o StartOptions, st *setup, w *session.Writer, system, model string, effort llm.Effort, prior []session.Entry, snapshotDir string) (*Agent, error) {
	cfg, jail := st.cfg, st.jail
	snaps := session.NewSnapshots(w, snapshotDir, prior)
	webKey, err := webSearchKey(cfg)
	if err != nil {
		return nil, err
	}
	m := o.mode()
	env := &tools.Env{
		Root: jail.Root(), Paths: jail, AutoJail: jail.WithWritable(permissions.TempRoots()...),
		Ask: o.Ask, Reads: tools.NewReadTracker(), Snap: snaps,
		ShellEnv:    tools.ShellEnv(os.Environ(), config.EnvRefs(cfg)),
		WebProvider: cfg.Web.Search.Provider, WebKey: webKey,
		SpillDir: st.spillDir, SpillPrefix: w.ID8() + "-"}
	shellRules, err := rules(cfg, o.Trusted)
	if err != nil {
		return nil, err
	}
	shell := permissions.NewShell(shellRules, jail, runtime.GOOS)
	env.Commands = shell
	// Self-managed project skills (SPECS §12): installed when the feature
	// resolves on (§3.3's table), the workdir is trusted (ACP's default trust
	// counts — Decision 14), and prepare()'s resolution succeeded. The dir is
	// prepare()'s — no call site re-resolves it (C5a).
	// The resume rule: a resumed session learns only when its header records
	// the block AND the switch resolves on now — a session that started
	// without the block has no contract in its frozen prompt, so it stays off.
	hadBlock := len(prior) > 0 && prior[0].Session != nil && prior[0].Session.SelfManagedSkills
	resumed := prior != nil
	switchOn := st.smOn && (!resumed || hadBlock)
	switch {
	case hadBlock && st.skillDirErr != nil && o.Emit != nil:
		// The frozen prompt still invites writes, but the dir cannot resolve
		// right now (a broken symlink, .moca/skills replaced by a file): name
		// the real cause — the switch may well be on.
		o.Emit(Warning{Text: fmt.Sprintf("this resumed session's instructions still describe project skills, but the skills dir cannot be resolved now (%v) — auto-skill writes are refused; /clear starts a session without them", st.skillDirErr)})
	case st.skillDirErr != nil && st.sm.On && o.Trusted && o.Emit != nil:
		o.Emit(Warning{Text: fmt.Sprintf("self-managed skills are off for this session: %v", st.skillDirErr)}) // switch-gated (H7)
	case st.smOn && !hadBlock && resumed && o.Emit != nil: // resumed from a session started with the feature off
		o.Emit(Warning{Text: "self-managed skills stay off in this resumed session (it started without them) — /clear starts a session with them on"})
	case hadBlock && !st.smOn && o.Emit != nil: // resumed after the switch went off or trust was withdrawn
		o.Emit(Warning{Text: "this resumed session's instructions still describe project skills, but skills.selfManaged is off (or the workdir is not trusted) now — auto-skill writes are refused; /clear starts a session without them"})
	}
	if switchOn {
		env.SkillDir = st.skillDir
		snaps.RefuseUnder(st.skillDir)           // N-18: undo never rolls a skill file back
		env.SkillCold = !hasAutoSkill(st.skills) // no recorded skills yet: the first
		// successful verification here is the durable fact (see tools' arming)
		env.SkillRecordNudge = config.ResolveRecordNudge(cfg) // §12.1: on unless a layer says false
		if cs, ok := env.Commands.(interface{ SetSkillDir(string) }); ok {
			cs.SetSkillDir(st.skillDir) // the shell refusal (C1)
		}
		env.SkillGuard = skillGuard(env, st.skillDir, st.sessionDate)
	} else if hadBlock { // the frozen prompt still invites writes: block ⇒ guard
		// C5b: the frozen prompt still invites writes the absent guard would
		// not check — install a refusing one instead of going nil (§2.1's
		// invariant: block present ⇒ guard installed, validating or refusing).
		// When the dir cannot resolve, route writes at the lexical location
		// anyway: while the structure is broken every write into it fails in
		// the same resolver first, but a model that REPAIRS it mid-session
		// (rm .moca/skills && mkdir …) must not get unvalidated writes — the
		// prompt it is following still advertises the block.
		// The shell check is not installed here: it belongs to the feature
		// being on, and a shell write in this state is caught at load.
		dir := st.skillDir
		if st.skillDirErr != nil {
			dir = filepath.Join(jail.Root(), ".moca", "skills")
		}
		env.SkillDir = dir
		snaps.RefuseUnder(dir) // N-18: the refusing guard's scope is the undo bound too
		env.SkillGuard = refusingAutoGuard(dir)
		env.SkillGuardRefusing = true
	}
	reg := tools.NewRegistry(tools.Builtins()...)
	// With servers configured, the frozen `mcp` stub is replaced by the lazy
	// proxy (§10.5): nothing starts here, and no server tool schema ever
	// reaches the prompt — the roster lines above are all the model sees.
	var mgr *mcp.Manager
	if len(cfg.MCP.Servers) > 0 {
		ix, _ := mcp.LoadIndex(filepath.Join(config.DataDir(), "mcp-index.json"))
		mgr = mcp.NewManager(cfg.MCP.Servers, time.Duration(cfg.MCP.IdleTimeout)*time.Second, ix, mcp.Options{BaseEnv: os.Environ()})
		reg.Register(mcp.NewTool(mgr, cfg.MCP.Servers))
	}
	a, err := New(Options{Config: cfg, Providers: st.reg, Tools: reg, Env: env,
		Session: w, Snapshots: snaps, System: system, Model: model, Effort: effort, Emit: o.Emit, Prior: prior, MCP: mgr,
		Headless: o.Headless})
	if err != nil {
		return nil, err
	}
	// The mode and plan never cross: plan rides only on default (a plan
	// run at default even when the resolved mode is wider — the TUI's
	// SetPlan-to-plan lands there the same way).
	a.skills, a.skillDirs = st.skills, st.skillDirs
	a.applyMode(o.mode())
	if o.Plan {
		a.applyPlan(true)
	}
	a.logger().Info("session", "resumed", prior != nil, "workdir", jail.Root(), "model", model,
		"effort", string(effort), "mode", m, "plan", o.Plan)
	return a, nil
}

// mode is the starting permission mode: default when unset, and always
// default under plan (plan rides only on default, so the TUI's plan
// toggle lands there too — this guards every caller).
func (o StartOptions) mode() PermMode {
	switch {
	case o.Mode == "", o.Plan:
		return ModeDefault
	}
	return o.Mode
}

// rules builds the permission rules once for both shell views: the trusted
// project layer first (.moca/config.jsonc), then the user layer — the legacy
// shell.allow names (never a hard-deny or ask-every-time one) plus the
// permissions block's own rules. Config validation rejects invalid rule
// syntax upstream, so the parse steps below cannot fail in practice; any
// rule that does not parse fails the build loudly (never a silently
// dropped check). Untrusted: the project layer is absent.
func rules(cfg config.Config, trusted bool) (*permissions.Rules, error) {
	var layers []permissions.RuleSet
	if trusted && cfg.Project != nil {
		p := cfg.Project.Permissions
		proj, err := permissions.ParseRuleSet("project", p.Allow, p.Ask, p.Deny)
		if err != nil {
			return nil, err
		}
		layers = append(layers, proj)
	}
	var user permissions.RuleSet
	// shell.allow is the legacy list: bare command names only, filtered.
	for _, n := range permissions.LegacyAllowNames(slices.Clone(cfg.Shell.Allow)) {
		r, err := permissions.ParseRule(n)
		if err != nil {
			return nil, err
		}
		user.Allow = append(user.Allow, r)
	}
	// permissions.allow carries full rules (wildcards, named arguments) and
	// goes through the same parser as ask/deny — running it through the
	// legacy name filter silently dropped every `rm build/*`-shaped entry.
	// A rule that does not parse is an error here, never a dropped check.
	allowed, err := permissions.ParseRuleSet("user", cfg.Permissions.Allow, nil, nil)
	if err != nil {
		return nil, err
	}
	user.Allow = append(user.Allow, allowed.Allow...)
	user.Name = "user"
	rs, err := permissions.ParseRuleSet("user", nil, cfg.Permissions.Ask, cfg.Permissions.Deny)
	if err != nil {
		return nil, err
	}
	user.Ask, user.Deny = rs.Ask, rs.Deny
	layers = append(layers, user)
	return permissions.NewRules(layers...), nil
}

// overflowDir is the area holding the full text of over-cap tool outputs
// (§10): one random subdirectory per process, pruned with the snapshot
// retention.
func overflowDir() string { return filepath.Join(config.DataDir(), "overflow") }

// newSpillDir names this process's overflow subdirectory (created lazily by
// tools.Spill). Random, so no other session can guess or share it.
func newSpillDir() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return filepath.Join(overflowDir(), hex.EncodeToString(b)), nil
}

// pruneOverflow removes overflow entries (per-process dirs, and flat files of
// the first rev-21 layout) older than days; 0 keeps everything.
func pruneOverflow(days int) {
	if days <= 0 {
		return
	}
	cutoff := time.Now().AddDate(0, 0, -days)
	ents, err := os.ReadDir(overflowDir())
	if err != nil {
		return
	}
	for _, e := range ents {
		p := filepath.Join(overflowDir(), e.Name())
		if newestModTime(p).Before(cutoff) {
			os.RemoveAll(p)
		}
	}
}

// newestModTime is the latest modification time under p (p itself included):
// a directory's own mtime only moves when an entry is added or removed, so a
// per-process spill dir is judged by its freshest file.
func newestModTime(p string) time.Time {
	var newest time.Time
	filepath.WalkDir(p, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if fi, err := d.Info(); err == nil && fi.ModTime().After(newest) {
			newest = fi.ModTime()
		}
		return nil
	})
	return newest
}

// webSearchKey resolves web.search.apiKey at session start: a configured
// but unset env: variable is a config error (exit 2), matching the
// provider-key stance. The key never reaches the model's shell environment
// either — config.EnvRefs strips it from the shell tool's env.
func webSearchKey(cfg config.Config) (string, error) {
	if cfg.Web.Search.APIKey == "" {
		return "", nil
	}
	key, err := config.ResolveEnv(cfg.Web.Search.APIKey)
	if err != nil {
		return "", fmt.Errorf("web.search.apiKey: %w", err)
	}
	return key, nil
}

func Start(o StartOptions) (*Agent, error) {
	st, err := prepare(o, o.Workdir, true)
	if err != nil {
		return nil, err
	}
	cfg, reg, jail := st.cfg, st.reg, st.jail
	instr, err := skills.LoadInstructions(config.ConfigDir(), o.Workdir, o.Trusted)
	if err != nil {
		return nil, &StartError{err}
	}
	var servers []ServerLine
	for name, s := range cfg.MCP.Servers {
		servers = append(servers, ServerLine{Name: name, Description: s.Description})
	}
	slices.SortFunc(servers, func(a, b ServerLine) int { return strings.Compare(a.Name, b.Name) })
	osName, arch := Platform()
	verify, verifySrc := DetectVerify(jail.Root())
	// The block is added only when a validating guard will be installed for
	// the same dir (C5): block present ⇒ guard installed.
	var selfManaged *SelfManagedInput
	if st.smOn {
		selfManaged = &SelfManagedInput{Dir: st.skillDir}
	}
	system := BuildSystemPrompt(PromptInput{Workdir: jail.Root(), OS: osName, Arch: arch, Verify: verify, VerifySource: verifySrc,
		Date: time.Now().Format("2006-01-02"), Git: GitState(jail.Root()), Version: config.Version,
		ConfigDir: config.ConfigDir(), PromptsDir: config.PromptsDir(), SkillsDir: filepath.Join(config.ConfigDir(), "skills"),
		RTK: toolOnPath("rtk"), Skills: st.skills, Servers: servers, Instructions: instr, SelfManaged: selfManaged})

	session.Prune(filepath.Join(config.DataDir(), "snapshot"), cfg.RetentionDays())
	pruneOverflow(cfg.RetentionDays())
	m, _, err := reg.Resolve(cfg.Model)
	if err != nil {
		return nil, err
	}
	effort := m.DefaultEffort()
	if o.Effort != "" {
		e, _ := llm.ParseEffort(o.Effort)
		effort = m.ClampEffort(e)
	}
	slug := o.Slug
	if slug == "" {
		slug = "session"
	}
	w, err := session.Create(filepath.Join(config.DataDir(), "sessions"), session.Header{
		Workdir: jail.Root(), Provider: m.Provider, Model: cfg.Model, Effort: string(effort),
		MocaVersion: config.Version, SystemPrompt: system, Yolo: o.mode() == ModeYolo, Mode: string(o.mode()),
		SelfManagedSkills: selfManaged != nil}, slug)
	if err != nil {
		return nil, &StartError{err}
	}
	a, err := build(o, st, w, system, cfg.Model, effort, nil, filepath.Join(config.DataDir(), "snapshot"))
	if err != nil {
		// F-5: the writer holds the session file's lock and the file itself is
		// a half-written session (a header, nothing else) that no run will ever
		// append to. Close it, then delete it: leaving it behind both held the
		// lock (a retry saw ErrInUse on a session nobody was using) and put an
		// empty session in /resume's list. Close precedes Remove — Windows
		// refuses to delete an open file.
		_ = w.Close()
		if rmErr := os.Remove(w.Path()); rmErr != nil {
			slog.Debug("failed start: session file remains", "path", w.Path(), "err", rmErr)
		}
		return nil, &StartError{err}
	}
	return a, nil
}

// headerTimeout bounds the wait for provider response headers: the SSE stall
// timeout only covers the response body, so a server that accepts a request
// and then goes silent would hang an unattended run forever (seen once in
// the live ship-gate runs).
var headerTimeout = 120 * time.Second

func defaultHTTPClient() *http.Client {
	tr, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return &http.Client{Transport: http.DefaultTransport}
	}
	ct := tr.Clone()
	ct.ResponseHeaderTimeout = headerTimeout
	return &http.Client{Transport: ct}
}

// skillGuard is the validating guard: it enforces the skill contract (§12) on
// the exact bytes that would land, for paths under dir, plus the per-run
// creation cap. Structural — internal/tools holds it as a func.
func skillGuard(env *tools.Env, dir string, sessionDate time.Time) func(abs, rel string, data []byte) error {
	return func(abs, rel string, data []byte) error {
		parts := strings.Split(rel, "/")
		if len(parts) < 2 {
			return nil // a loose file in the skills root is not a skill
		}
		if canon, caseMatched := tools.CanonicalSkillPath(dir, abs); caseMatched && canon != abs {
			// A case-variant spelling lands a file a Linux checkout (a
			// teammate, CI) never discovers (H11). caseMatched is false when
			// abs is not under dir at all; on Linux the scope test is
			// byte-exact, so only darwin/windows (or MOCA_CASEFOLD_FS=1) get
			// here.
			return fmt.Errorf("refused: write skill files as .moca/skills/<dir>/SKILL.md exactly — this spelling (%s) is invisible on case-sensitive systems", rel)
		}
		skillMD := filepath.Join(dir, parts[0], "SKILL.md")
		if len(parts) > 2 || !tools.IsSkillName(parts[1]) {
			// A resource file — and the write ORDER matters: a dir without
			// SKILL.md is not a skill yet, and every skill the model creates
			// while the feature is on is an auto skill — a notes.md-first
			// write would land a permanent companion file inside what
			// SKILL.md then makes an auto skill.
			cur, rerr := os.ReadFile(skillMD)
			if errors.Is(rerr, fs.ErrNotExist) {
				return fmt.Errorf("refused: .moca/skills/%s/ has no SKILL.md yet, and a skill starts with it — write SKILL.md first (a skill you create must be an auto skill), or keep this file outside the skills dir", parts[0])
			} else if rerr != nil {
				return fmt.Errorf("refused: cannot read %s/SKILL.md to check the write: %v", parts[0], rerr)
			}
			name := parts[0]
			if f, perr := skills.ParseSkill(cur, parts[0]); perr == nil {
				if f.Auto {
					return fmt.Errorf("refused: %s is an auto skill, and auto skills are a single SKILL.md — put this in its body, or keep the file outside .moca/skills", parts[0])
				}
				name = f.Name
			}
			if !skills.Named(env.SkillTyped, name, parts[0]) { // a hand-written skill's files (Decision 15)
				return fmt.Errorf("refused: %s is a hand-written skill — change its files only when the user's request names it", name)
			}
			return nil
		}
		prev, err := os.ReadFile(abs)
		if errors.Is(err, fs.ErrNotExist) {
			prev = nil
		} else if err != nil {
			return fmt.Errorf("refused: cannot read %s to check the write: %v", rel, err)
		}
		named := false // Decision 15: only an existing hand-written SKILL.md consults it
		if prev != nil {
			name := parts[0]
			if f, perr := skills.ParseSkill(prev, parts[0]); perr == nil {
				name = f.Name
			}
			named = skills.Named(env.SkillTyped, name, parts[0])
		}
		others, err := skills.CountAutoSkills(dir, parts[0]) // by DIRECTORY NAME, not by path (H11)
		if err != nil {
			return fmt.Errorf("refused: cannot read .moca/skills to count the project's skills: %v", err)
		}
		if err := skills.ValidateSkill(data, skills.WriteContext{DirName: parts[0], Prev: prev,
			Others: others, UserSignal: env.SkillUserSignal, Named: named,
			Today: time.Now(), SessionDate: sessionDate}); err != nil {
			return err
		}
		if prev == nil && env.SkillCreations >= skills.SkillCreationsPerRun {
			return fmt.Errorf("refused: %d new skills were already created in this run (cap %d) — update an existing skill instead, or record it in a later request",
				env.SkillCreations, skills.SkillCreationsPerRun)
		}
		return nil
	}
}

// skillRefusingText is C5b's refusal: the session's frozen prompt still
// describes project skills, but the switch is off now.
const skillRefusingText = "refused: self-managed skills were turned off after this session started — don't write under .moca/skills; /clear starts a session without them"

// refusingAutoGuard refuses any write whose previous or new content is an auto
// skill; hand-written skill edits pass (unvalidated, like any write with the
// feature off).
func refusingAutoGuard(dir string) func(abs, rel string, data []byte) error {
	return func(abs, rel string, data []byte) error {
		parts := strings.Split(rel, "/")
		name := parts[0]
		if prev, err := os.ReadFile(abs); err == nil && isAutoSkill(prev, name) {
			return errors.New(skillRefusingText)
		}
		if isAutoSkill(data, name) {
			return errors.New(skillRefusingText)
		}
		// A resource write INTO an auto skill's directory corrupts the
		// machine-owned artifact without either check above seeing it: the
		// target file is not itself a skill, and its absent previous content
		// is not one either. Auto skills are a single SKILL.md — the
		// validating guard refuses companions (and SKILL.md-first for that
		// matter), so this one refuses them too; otherwise the write lands
		// contract-breaking and the next session's load-time re-validation
		// drops a skill this session's frozen prompt still advertises.
		if len(parts) >= 2 && !tools.IsSkillName(parts[1]) {
			if cur, err := os.ReadFile(filepath.Join(dir, name, "SKILL.md")); err == nil &&
				isAutoSkill(cur, name) {
				return errors.New(skillRefusingText)
			}
		}
		return nil
	}
}

// isAutoSkill reports whether data parses as an auto skill (false on any
// error — a file that cannot be parsed is not one the machinery owns).
func isAutoSkill(data []byte, dirName string) bool {
	f, err := skills.ParseSkill(data, dirName)
	return err == nil && f.Auto
}
