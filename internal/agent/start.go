package agent

import (
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/llm"
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
	Yolo    bool
	Ask     tools.Asker
	Emit    func(Event)
	HTTP    *http.Client
	Slug    string
}

// setup is what Start and Resume need before a session exists: the config
// with the --model override applied, the provider registry, the built-in
// skills dir and the path jail. It is built once and handed to build.
type setup struct {
	cfg        config.Config
	reg        *provider.Registry
	builtinDir string
	jail       *permissions.Jail
}

func prepare(o StartOptions, jailRoot string) (*setup, error) {
	cfg := o.Config
	if o.Model != "" {
		cfg.Model = o.Model
	}
	hc := o.HTTP
	if hc == nil {
		hc = http.DefaultClient
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
	builtinDir, err := skills.ExtractBuiltins(config.DataDir())
	if err != nil {
		return nil, &StartError{err}
	}
	globalSkills := filepath.Join(config.ConfigDir(), "skills")
	jail, err := permissions.NewJail(jailRoot, []string{globalSkills, builtinDir})
	if err != nil {
		return nil, &StartError{err}
	}
	return &setup{cfg: cfg, reg: reg, builtinDir: builtinDir, jail: jail}, nil
}

// build constructs everything Start and Resume share past setup: shell
// checker, tool env, snapshot store and the agent itself. Start pre-computes
// the system prompt and passes nil prior entries; Resume passes the stored
// prompt verbatim and the transcript it read back.
func build(o StartOptions, st *setup, w *session.Writer, system, model string, effort llm.Effort, prior []session.Entry, snapshotDir string) (*Agent, error) {
	cfg, jail := st.cfg, st.jail
	snaps := session.NewSnapshots(w, snapshotDir, prior)
	env := &tools.Env{Root: jail.Root(), Paths: jail, Commands: permissions.NewShell(cfg.Shell.Allow, jail, runtime.GOOS),
		Ask: o.Ask, Reads: tools.NewReadTracker(), Snap: snaps,
		ShellEnv: tools.ShellEnv(os.Environ(), config.EnvRefs(cfg))}
	a, err := New(Options{Config: cfg, Providers: st.reg, Tools: tools.NewRegistry(tools.Builtins()...), Env: env,
		Session: w, Snapshots: snaps, System: system, Model: model, Effort: effort, Emit: o.Emit, Prior: prior})
	if err != nil {
		return nil, err
	}
	if o.Yolo {
		a.applyYolo(true)
	}
	return a, nil
}

func Start(o StartOptions) (*Agent, error) {
	st, err := prepare(o, o.Workdir)
	if err != nil {
		return nil, err
	}
	cfg, reg, jail, builtinDir, emit := st.cfg, st.reg, st.jail, st.builtinDir, o.Emit
	globalSkills := filepath.Join(config.ConfigDir(), "skills")
	projectSkills := filepath.Join(o.Workdir, ".moca", "skills")
	var dirs []skills.Dir
	if o.Trusted {
		dirs = append(dirs, skills.Dir{Path: projectSkills, Source: "project"})
	}
	dirs = append(dirs, skills.Dir{Path: globalSkills, Source: "global"}, skills.Dir{Path: builtinDir, Source: "builtin"})
	sk, skErrs := skills.Discover(dirs)
	for _, se := range skErrs {
		if emit != nil {
			emit(Warning{Text: se.Error()})
		}
	}
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
	system := BuildSystemPrompt(PromptInput{Workdir: jail.Root(), OS: osName, Arch: arch,
		Date: time.Now().Format("2006-01-02"), Git: GitState(jail.Root()), Version: config.Version,
		Skills: sk, Servers: servers, Instructions: instr})

	session.Prune(filepath.Join(config.DataDir(), "snapshot"), cfg.RetentionDays())
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
		MocaVersion: config.Version, SystemPrompt: system, Yolo: o.Yolo}, slug)
	if err != nil {
		return nil, &StartError{err}
	}
	return build(o, st, w, system, cfg.Model, effort, nil, filepath.Join(config.DataDir(), "snapshot"))
}
