package agent

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io/fs"
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
	Yolo    bool
	Plan    bool
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
	spillDir   string // this process's overflow dir — the only one its jail can read
}

func prepare(o StartOptions, jailRoot string) (*setup, error) {
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
	promptsDir := config.PromptsDir()
	// readOnly: the prompt templates are readable (the agent may update a
	// command it is shown). askWrite: a write there — outside the workdir —
	// needs the user's approval (the write/edit tools ask; the shell's
	// redirect check does not).
	// This process's overflow dir is readable too: over-cap tool outputs are
	// saved there in full and the cut result names the file (tools.Spill).
	// Only its own dir — other sessions' outputs (other projects, possibly
	// secrets) share the overflow area and must stay out of reach.
	spillDir, err := newSpillDir()
	if err != nil {
		return nil, &StartError{err}
	}
	jail, err := permissions.NewJail(jailRoot, []string{globalSkills, builtinDir, promptsDir, spillDir}, []string{promptsDir})
	if err != nil {
		return nil, &StartError{err}
	}
	return &setup{cfg: cfg, reg: reg, builtinDir: builtinDir, jail: jail, spillDir: spillDir}, nil
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
	env := &tools.Env{Root: jail.Root(), Paths: jail, Commands: permissions.NewShell(cfg.Shell.Allow, jail, runtime.GOOS),
		Ask: o.Ask, Reads: tools.NewReadTracker(), Snap: snaps,
		ShellEnv:    tools.ShellEnv(os.Environ(), config.EnvRefs(cfg)),
		WebProvider: cfg.Web.Search.Provider, WebKey: webKey,
		SpillDir: st.spillDir, SpillPrefix: w.ID8() + "-"}
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
		Session: w, Snapshots: snaps, System: system, Model: model, Effort: effort, Emit: o.Emit, Prior: prior, MCP: mgr})
	if err != nil {
		return nil, err
	}
	if o.Yolo {
		a.applyYolo(true)
	}
	if o.Plan {
		a.applyPlan(true)
	}
	a.logger().Info("session", "resumed", prior != nil, "workdir", jail.Root(), "model", model,
		"effort", string(effort), "yolo", o.Yolo, "plan", o.Plan)
	return a, nil
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
	sk = filterSkills(sk)
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
	system := BuildSystemPrompt(PromptInput{Workdir: jail.Root(), OS: osName, Arch: arch, Verify: verify, VerifySource: verifySrc,
		Date: time.Now().Format("2006-01-02"), Git: GitState(jail.Root()), Version: config.Version,
		RTK: toolOnPath("rtk"), Skills: sk, Servers: servers, Instructions: instr})

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
		MocaVersion: config.Version, SystemPrompt: system, Yolo: o.Yolo}, slug)
	if err != nil {
		return nil, &StartError{err}
	}
	return build(o, st, w, system, cfg.Model, effort, nil, filepath.Join(config.DataDir(), "snapshot"))
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
