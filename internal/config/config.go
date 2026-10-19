package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

const MinContextWindow = 16384

var DefaultShellAllow = []string{"go", "git", "gh", "grep", "rg", "find", "ls", "cat", "head", "tail",
	"wc", "sort", "uniq", "diff", "which", "jq", "mkdir", "sed", "awk",
	"curl", "make", "mise", "python", "pytest", "node", "npm",
	"docker", "kubectl", "terraform", "ansible", "rtk", "graphify"}

// BuiltinProviders are always known; config entries with these names
// override fields, they don't need baseUrl/protocol.
var BuiltinProviders = map[string]string{
	"anthropic":   "env:ANTHROPIC_API_KEY",
	"opencode-go": "env:OPENCODE_API_KEY",
	"openai":      "env:OPENAI_API_KEY",
}

// LocalProviders are built-in providers that need no API key and have no
// static catalog (their models are discovered from the server). Unlike
// BuiltinProviders they exist only when the user opts in — a providers entry,
// or a model reference such as "ollama/llama3.1" — so nothing probes a local
// port for people who never asked. They need no baseUrl or protocol.
var LocalProviders = []string{"ollama"}

// IsLocalProvider reports whether name is one of LocalProviders.
func IsLocalProvider(name string) bool { return slices.Contains(LocalProviders, name) }

// UseProvider makes sure a local provider has an entry (the implicit opt-in
// of a model reference). The Providers map is replaced by a copy, so a caller
// sharing the Config value is not mutated behind its back.
func (c *Config) UseProvider(name string) {
	if _, ok := c.Providers[name]; ok || !IsLocalProvider(name) {
		return
	}
	next := make(map[string]ProviderConfig, len(c.Providers)+1)
	for k, v := range c.Providers {
		next[k] = v
	}
	next[name] = ProviderConfig{Auth: "api_key"}
	c.Providers = next
}

var Protocols = []string{"anthropic-messages", "openai-completions", "openai-responses"}

// OAuthUnsupported lists providers whose subscription OAuth was rejected by
// the phase-7 policy gate (docs/specs/oauth-verification.md): config
// `auth: "oauth"` for one of these is an error citing the reason. openai's
// Sign in with ChatGPT token sharing is supported, so it is not listed.
var OAuthUnsupported = map[string]string{
	"anthropic": "Anthropic does not permit third-party clients to use Claude subscription OAuth (docs/specs/oauth-verification.md); use an API key",
}

// OAuthProviders lists the providers whose subscription OAuth flow is
// implemented — provider/oauth_providers.go holds the endpoint table and a
// provider test pins the two lists together. `auth: "oauth"` for anything
// else is a config error at load, not a request-time surprise with a
// runtime exit code.
var OAuthProviders = []string{"openai"}

type Config struct {
	Model     string                    `json:"model"`
	ModelHard string                    `json:"modelHard"`
	Providers map[string]ProviderConfig `json:"providers"`
	Shell     ShellConfig               `json:"shell"`
	MCP       MCPConfig                 `json:"mcp"`
	Web       WebConfig                 `json:"web"`
	Log       LogConfig                 `json:"log"`
	Snapshot  SnapshotConfig            `json:"snapshot"`
	Context   ContextConfig             `json:"context"`
	TUI       TUIConfig                 `json:"tui"`
	Yolo      bool                      `json:"yolo"` // legacy: all permission checks off by default (§7.5); see Mode
	// Permissions holds the user-level mode and rules. Project is the trusted
	// project file's content (set by cmd after the trust decision, never
	// decoded): its permissions block joins the rule layers, its mcp block
	// joins the MCP servers (§4, §10.5).
	Permissions PermissionsConfig `json:"permissions"`
	Skills      SkillsConfig      `json:"skills"`
	Project     *ProjectConfig    `json:"-"`
}

// SkillsConfig is the self-managed project skills switch (SPECS §4/§12). In
// the user config it is the opt-in; in a trusted project file `false` opts the
// repository out and `true` is declarative — it never enables the feature on
// its own (Decisions 2 and 13; ResolveSelfManagedSkills).
type SkillsConfig struct {
	SelfManaged *bool `json:"selfManaged,omitempty"`
	// RecordNudge is the end-of-run recording nudge (SPECS §12.1, v0.7.1). It
	// is on by default; either layer can turn it off with false and no layer
	// needs to turn it on — a project true is declarative, like SelfManaged's.
	RecordNudge *bool `json:"recordNudge,omitempty"`
}

// SelfManagedState is the resolved skills.selfManaged switch (§3.3). Warning
// and Note are user-facing texts the agent emits once per start/resume
// (prepare(), task 3); both are empty for an untrusted workdir, where
// cfg.Project is nil.
type SelfManagedState struct {
	On      bool
	Warning string
	Note    string
}

// ResolveSelfManagedSkills: on only when the user config says true and the
// trusted project file does not say false. A project true never enables it
// alone — it warns a user who has not opted in.
func ResolveSelfManagedSkills(cfg Config) SelfManagedState {
	user := cfg.Skills.SelfManaged
	var proj *bool
	if cfg.Project != nil {
		proj = cfg.Project.Skills.SelfManaged
	}
	userOn := user != nil && *user
	switch {
	case userOn && proj != nil && !*proj:
		return SelfManagedState{Note: "self-managed skills are off in this project: its .moca/config.jsonc sets skills.selfManaged false"}
	case userOn:
		return SelfManagedState{On: true}
	case proj != nil && *proj:
		how := "does not enable it"
		if user != nil {
			how = "sets it false"
		}
		return SelfManagedState{Warning: fmt.Sprintf("this project's .moca/config.jsonc sets skills.selfManaged true, but your user config %s — "+
			"self-managed skills are off; set \"skills\": {\"selfManaged\": true} in %s and restart moca to take part", how, ConfigFile())}
	}
	return SelfManagedState{}
}

// ResolveRecordNudge: the end-of-run recording nudge is on unless a layer sets
// skills.recordNudge false. Unlike selfManaged it needs no opt-in — the
// default is on — so a false is the only value that decides anything, and
// either the user config or the trusted project file can supply it (a project
// true never overrides a user's false).
func ResolveRecordNudge(cfg Config) bool {
	if cfg.Skills.RecordNudge != nil && !*cfg.Skills.RecordNudge {
		return false
	}
	if cfg.Project != nil && cfg.Project.Skills.RecordNudge != nil && !*cfg.Project.Skills.RecordNudge {
		return false
	}
	return true
}

// PermissionsConfig is the permissions block of the user config and, with
// fewer allowed keys, of a project .moca/config.jsonc.
type PermissionsConfig struct {
	DefaultMode string     `json:"defaultMode,omitempty"` // "default" | "auto" | "plan" | "yolo"
	Allow       []string   `json:"allow,omitempty"`
	Ask         []string   `json:"ask,omitempty"`
	Deny        []string   `json:"deny,omitempty"`
	Auto        AutoConfig `json:"auto"`
}

// AutoConfig tunes auto mode.
type AutoConfig struct {
	Classifier *bool  `json:"classifier,omitempty"` // default true
	Model      string `json:"model,omitempty"`      // default: config `model`
}

// PermModes are the valid permission modes — the `--mode` flag's values and
// permissions.defaultMode in both config layers (§3, §4). "plan" is the plan
// scope: wherever a mode is consumed it resolves to (default, plan=true).
var PermModes = []string{"default", "auto", "yolo", "plan"}

func (p PermissionsConfig) validate(key string) error {
	if p.DefaultMode != "" && !slices.Contains(PermModes, p.DefaultMode) {
		return fmt.Errorf("%s.defaultMode must be one of %s, got %q", key, strings.Join(PermModes, ", "), p.DefaultMode)
	}
	for _, l := range []struct {
		name string
		list []string
	}{{"allow", p.Allow}, {"ask", p.Ask}, {"deny", p.Deny}} {
		for i, r := range l.list {
			if len(strings.Fields(r)) == 0 {
				return fmt.Errorf("%s.%s[%d]: empty rule", key, l.name, i)
			}
		}
	}
	return nil
}

// Mode is the user-level default permission mode (§11): permissions.defaultMode,
// else the legacy yolo flag, else "default". A configured "plan" is the plan
// scope — ResolveMode maps any "plan" to (default, plan=true). CLI flags and
// a trusted project file are applied on top by cmd. It may return "plan",
// which is not an agent.PermMode — resolve through ResolveMode/
// ResolveExecuteMode, never cast.
func (c Config) Mode() string {
	switch {
	case c.Permissions.DefaultMode != "":
		return c.Permissions.DefaultMode
	case c.Yolo:
		return "yolo"
	}
	return "default"
}

// ResolveMode resolves a run's permission mode and plan scope (§3): an
// explicit flag value wins — "plan" is the plan scope at the default mode —
// else a trusted project file's defaultMode (the project layer wins over the
// user's), else the config's mode. A configured or flagged "plan" always
// resolves to (default, plan=true), so it never counts as yolo for the trust
// decision either. Shared by the CLI front ends and the ACP server.
func ResolveMode(flag string, c Config) (string, bool) {
	return resolveMode(flag, c, false)
}

// ResolveExecuteMode is ResolveMode for `moca execute` (§3): executing a plan
// is plan-exiting, so a configured layer whose mode is "plan" is skipped and
// the next layer decides — a project plan over a user auto runs at auto. The
// flag cannot be "plan" here (parseArgs rejects execute + --mode plan).
func ResolveExecuteMode(flag string, c Config) string {
	mode, _ := resolveMode(flag, c, true)
	return mode
}

// resolveMode walks flag → trusted project → user; skipPlan drops a layer
// whose value is "plan". A "plan" that decides is the plan scope.
func resolveMode(flag string, c Config, skipPlan bool) (string, bool) {
	for _, mode := range []string{flag, c.Project.Mode(), c.Mode()} {
		if mode == "" || skipPlan && mode == "plan" {
			continue
		}
		if mode == "plan" {
			return "default", true
		}
		return mode, false
	}
	return "default", false
}

// ClassifierOn reports whether auto mode consults the classifier (default on).
func (a AutoConfig) ClassifierOn() bool { return a.Classifier == nil || *a.Classifier }

type ProviderConfig struct {
	Auth     string                   `json:"auth"`               // "api_key" | "oauth"
	APIKey   string                   `json:"apiKey,omitempty"`   // must be "env:VAR"
	BaseURL  string                   `json:"baseUrl,omitempty"`  // overrides every protocol's base URL
	BaseURLs map[string]string        `json:"baseUrls,omitempty"` // per-protocol override (opencode-go's dual URLs)
	Protocol string                   `json:"protocol,omitempty"` // custom providers: default protocol of its models
	Models   map[string]ModelOverride `json:"models,omitempty"`
}

type ModelOverride struct {
	Protocol        string      `json:"protocol,omitempty"`
	ContextWindow   int         `json:"contextWindow,omitempty"`
	MaxOutputTokens int         `json:"maxOutputTokens,omitempty"`
	Cost            *CostConfig `json:"cost,omitempty"`
}

type CostConfig struct {
	Input      float64 `json:"input"`
	Output     float64 `json:"output"`
	CacheRead  float64 `json:"cacheRead"`
	CacheWrite float64 `json:"cacheWrite"`
}

type ShellConfig struct {
	Allow []string `json:"allow"`
}

// WebConfig configures the web tool's search op (§4, rev 19). fetch needs
// no config.
type WebConfig struct {
	Search WebSearchConfig `json:"search"`
}

type WebSearchConfig struct {
	Provider string `json:"provider,omitempty"` // "tavily" (default; keyless when apiKey is omitted) | "exa"
	APIKey   string `json:"apiKey,omitempty"`   // must be "env:VAR"; optional for tavily, required for exa
}

// LogConfig configures the diagnostic log (SPECS §13.5): a metadata-only
// logfmt file per process under <state>/logs. MOCA_LOG overrides level.
type LogConfig struct {
	Level string `json:"level,omitempty"` // "info" (default) | "debug" | "off"
}

type MCPConfig struct {
	IdleTimeout int                  `json:"idleTimeout"` // seconds
	Servers     map[string]MCPServer `json:"servers"`
}

type MCPServer struct {
	Command     string            `json:"command,omitempty"`
	Args        []string          `json:"args,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	URL         string            `json:"url,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	Description string            `json:"description,omitempty"`
	Approve     []string          `json:"approve,omitempty"`
}

type SnapshotConfig struct {
	RetentionDays *int `json:"retentionDays,omitempty"` // nil → 30; 0 → forever
}

// TUIConfig holds the interactive front end's few knobs.
type TUIConfig struct {
	// Notify is how the TUI signals a finished long run or a waiting approval
	// while the terminal is unfocused: "osc9" (desktop notification escape),
	// "bell", or "off".
	Notify string `json:"notify"`
	// Theme overrides scrollback/status colors. Every field is optional; an
	// empty value keeps the built-in default (internal/tui/theme.go).
	Theme ThemeConfig `json:"theme"`
}

// ThemeConfig overrides the TUI's colors (§3.5, §4). Both `light` and `dark`
// sub-objects exist because moca picks a variant from the terminal's reported
// background; a `both` sub-object applies to whichever variant the terminal
// reports and is the common case for a color that reads well on either. A
// key's precedence is per-key and per-variant: `light`/`dark` for the reported
// theme, else `both`, else the built-in default. The block is user-only: a
// project file cannot carry colors (§4).
type ThemeConfig struct {
	Light ThemeColors `json:"light"`
	Dark  ThemeColors `json:"dark"`
	Both  ThemeColors `json:"both"`
}

// ThemeColors is one variant's color overrides — every field a #rrggbb hex or
// an ANSI color number ("1", "208", …), empty = keep the default.
// internal/tui uses this type as its palette (tui.Colors); every field needs a
// built-in default in tui.DefaultTheme (TestDefaultThemeComplete).
type ThemeColors struct {
	// Scrollback bands (message_style.go).
	UserBg  string `json:"userBg,omitempty"`
	UserFg  string `json:"userFg,omitempty"`
	RespBg  string `json:"respBg,omitempty"`
	RespFg  string `json:"respFg,omitempty"`
	CodeBg  string `json:"codeBg,omitempty"`
	CodeFg  string `json:"codeFg,omitempty"`
	Muted   string `json:"muted,omitempty"`   // thinking/tool/notice lines
	Error   string `json:"error,omitempty"`   // error: lines
	Warning string `json:"warning,omitempty"` // warning: lines, `?` prompts
	Accent  string `json:"accent,omitempty"`  // welcome product name, pager selection
	Plan    string `json:"plan,omitempty"`    // plan-mode notice
	DiffAdd string `json:"diffAdd,omitempty"` // edit-diff added lines
	DiffDel string `json:"diffDel,omitempty"` // edit-diff removed lines
	// Status-bar roles (status_style.go): a #rrggbb hex or ANSI number per segment.
	// statusMode also colors the default-mode badge.
	StatusMode   string `json:"statusMode,omitempty"`
	StatusModel  string `json:"statusModel,omitempty"`
	StatusEffort string `json:"statusEffort,omitempty"`
	StatusPct    string `json:"statusPct,omitempty"`
	StatusHint   string `json:"statusHint,omitempty"`
	StatusCwd    string `json:"statusCwd,omitempty"`
	StatusBranch string `json:"statusBranch,omitempty"`
	StatusCost   string `json:"statusCost,omitempty"`
	StatusMuted  string `json:"statusMuted,omitempty"`
	StatusSep    string `json:"statusSep,omitempty"`
	// Mode badges + context-pressure colors.
	BadgeAuto string `json:"badgeAuto,omitempty"`
	BadgePlan string `json:"badgePlan,omitempty"`
	BadgeYolo string `json:"badgeYolo,omitempty"`
	PctWarn   string `json:"pctWarn,omitempty"` // context percent ≥ 70%
	PctHot    string `json:"pctHot,omitempty"`  // at/past the compact trigger
}

// NotifyModes are the valid tui.notify values.
var NotifyModes = []string{"osc9", "bell", "off"}

// themeHexRe matches a #rrggbb color (the form the built-in defaults use;
// bare ANSI numbers are the short form).
var themeHexRe = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// validateTheme checks the user config's tui.theme block. Every
// non-empty value must be a #rrggbb hex or an ANSI color number (0-255);
// empty keeps the built-in default (internal/tui/theme.go).
func validateTheme(key string, t ThemeConfig) error {
	for _, v := range []struct {
		name string
		c    ThemeColors
	}{{"light", t.Light}, {"dark", t.Dark}, {"both", t.Both}} {
		if err := validateThemeColors(key+"."+v.name, v.c); err != nil {
			return err
		}
	}
	return nil
}

// validateThemeColors walks ThemeColors' fields, so adding a role cannot
// silently skip validation (the json tag is the key users write).
func validateThemeColors(key string, c ThemeColors) error {
	rv := reflect.ValueOf(c)
	rt := rv.Type()
	for i := range rt.NumField() {
		name := strings.TrimSuffix(rt.Field(i).Tag.Get("json"), ",omitempty")
		if err := validateColor(key+"."+name, rv.Field(i).String()); err != nil {
			return err
		}
	}
	return nil
}

// validateColor accepts a #rrggbb hex or an ANSI color number 0-255.
func validateColor(key, v string) error {
	if v == "" || themeHexRe.MatchString(v) {
		return nil
	}
	if n, err := strconv.Atoi(v); err == nil && n >= 0 && n <= 255 {
		return nil
	}
	return fmt.Errorf("%s: %q must be a #rrggbb color or an ANSI color number 0-255", key, v)
}

type ContextConfig struct {
	ReserveTokens    int `json:"reserveTokens"`
	KeepRecentTokens int `json:"keepRecentTokens"`
	MaxSteps         int `json:"maxSteps"`
	// ProgressNudgeAt: the tool-batch count after which a run that has not
	// created or edited a file gets one §14 progress nudge. nil = the default
	// (DefaultProgressNudgeAt), 0 disables the nudge — a pointer so an
	// explicit 0 is distinguishable from an absent key.
	ProgressNudgeAt *int `json:"progressNudgeAt"`
}

// DefaultProgressNudgeAt is the §14 progress nudge's threshold when
// context.progressNudgeAt is absent.
const DefaultProgressNudgeAt = 12

// ProgressNudge resolves the §14 progress-nudge threshold: the configured
// value, else the default; 0 (or below) means the nudge is off.
func (c ContextConfig) ProgressNudge() int {
	if c.ProgressNudgeAt == nil {
		return DefaultProgressNudgeAt
	}
	return *c.ProgressNudgeAt
}

func (c Config) RetentionDays() int {
	if c.Snapshot.RetentionDays == nil {
		return 30
	}
	return *c.Snapshot.RetentionDays
}

func Default() Config {
	c := Config{}
	c.applyDefaults()
	return c
}

func (c *Config) applyDefaults() {
	if c.Providers == nil {
		c.Providers = map[string]ProviderConfig{}
	}
	for name, key := range BuiltinProviders {
		p := c.Providers[name]
		if p.Auth == "" {
			p.Auth = "api_key"
		}
		if p.Auth == "api_key" && p.APIKey == "" {
			p.APIKey = key
		}
		c.Providers[name] = p
	}
	for _, q := range []string{c.Model, c.ModelHard} {
		if p, _, err := SplitModel(q); err == nil {
			c.UseProvider(p)
		}
	}
	for _, name := range LocalProviders {
		if p, ok := c.Providers[name]; ok && p.Auth == "" {
			p.Auth = "api_key"
			c.Providers[name] = p
		}
	}
	if c.Shell.Allow == nil {
		c.Shell.Allow = slices.Clone(DefaultShellAllow)
	}
	if c.MCP.IdleTimeout == 0 {
		c.MCP.IdleTimeout = 600
	}
	if c.Context.ReserveTokens == 0 {
		c.Context.ReserveTokens = 16384
	}
	if c.Context.KeepRecentTokens == 0 {
		c.Context.KeepRecentTokens = 20000
	}
	if c.Context.MaxSteps == 0 {
		c.Context.MaxSteps = 40
	}
	if c.TUI.Notify == "" {
		c.TUI.Notify = "osc9"
	}
	if c.Web.Search.Provider == "" {
		c.Web.Search.Provider = "tavily"
	}
	if c.Log.Level == "" {
		c.Log.Level = "info"
	}
}

// Load reads and parses path. A missing file yields Default().
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Default(), nil
	}
	if err != nil {
		return Config{}, err
	}
	c, err := Parse(data)
	if err != nil {
		return Config{}, fmt.Errorf("%s: %w", path, err)
	}
	return c, nil
}

func Parse(data []byte) (Config, error) {
	std, err := Standardize(data)
	if err != nil {
		return Config{}, err
	}
	var c Config
	dec := json.NewDecoder(bytes.NewReader(std))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil {
		return Config{}, withPosition(data, err)
	}
	c.applyDefaults()
	if err := c.Validate(); err != nil {
		return Config{}, err
	}
	return c, nil
}

func withPosition(src []byte, err error) error {
	var off int64 = -1
	var se *json.SyntaxError
	var te *json.UnmarshalTypeError
	switch {
	case errors.As(err, &se):
		off = se.Offset
	case errors.As(err, &te):
		off = te.Offset
	}
	if off < 0 {
		return err
	}
	line, col := 1, 1
	for _, b := range src[:min(int(off), len(src))] {
		if b == '\n' {
			line, col = line+1, 1
		} else {
			col++
		}
	}
	return fmt.Errorf("%d:%d: %w", line, col, err)
}

func SplitModel(q string) (string, string, error) {
	p, m, ok := strings.Cut(q, "/")
	if !ok || p == "" || m == "" {
		return "", "", fmt.Errorf("model %q must be provider-qualified: provider/model (e.g. opencode-go/glm-5.3-flash)", q)
	}
	return p, m, nil
}

// Validate checks structure only. Whether a built-in provider knows a model
// id is checked by provider.Registry (it owns the catalog).
func (c Config) Validate() error {
	if err := c.Permissions.validate("permissions"); err != nil {
		return err
	}
	if c.Yolo && c.Permissions.DefaultMode != "" && c.Permissions.DefaultMode != "yolo" {
		return fmt.Errorf("yolo: true conflicts with permissions.defaultMode %q; drop one", c.Permissions.DefaultMode)
	}
	for _, q := range []string{c.Model, c.ModelHard} {
		if q == "" {
			continue
		}
		p, _, err := SplitModel(q)
		if err != nil {
			return err
		}
		if _, ok := c.Providers[p]; !ok {
			return fmt.Errorf("model %q: unknown provider %q (known: %s)", q, p, strings.Join(c.providerNames(), ", "))
		}
	}
	for name, p := range c.Providers {
		switch p.Auth {
		case "api_key":
			// apiKey is an optional env indirection: omitted, the provider
			// resolves from the credential store (a key saved with /login or
			// `moca login <provider>`), which also wins over the env value
			// when both exist.
			if p.APIKey != "" && !strings.HasPrefix(p.APIKey, envPrefix) {
				return fmt.Errorf("providers.%s.apiKey must be an env: reference (e.g. \"env:MY_KEY\"), never a literal (omit it to rely on the key stored by /login)", name)
			}
		case "oauth":
			if reason, ok := OAuthUnsupported[name]; ok {
				return fmt.Errorf("providers.%s.auth \"oauth\" is not available: %s", name, reason)
			}
			if !slices.Contains(OAuthProviders, name) {
				return fmt.Errorf("providers.%s.auth \"oauth\" is not available: no OAuth support for this provider; set \"api_key\"", name)
			}
		default:
			return fmt.Errorf("providers.%s.auth must be \"api_key\" or \"oauth\", got %q", name, p.Auth)
		}
		_, builtin := BuiltinProviders[name]
		builtin = builtin || IsLocalProvider(name)
		if !builtin && p.BaseURL == "" && len(p.BaseURLs) == 0 {
			return fmt.Errorf("providers.%s: custom provider needs baseUrl and protocol", name)
		}
		if p.Protocol != "" && !slices.Contains(Protocols, p.Protocol) {
			return fmt.Errorf("providers.%s.protocol %q unknown (want %s)", name, p.Protocol, strings.Join(Protocols, "|"))
		}
		if !builtin && p.Protocol == "" {
			return fmt.Errorf("providers.%s: custom provider needs protocol", name)
		}
		for mid, m := range p.Models {
			if m.Protocol != "" && !slices.Contains(Protocols, m.Protocol) {
				return fmt.Errorf("providers.%s.models.%s.protocol %q unknown", name, mid, m.Protocol)
			}
			if !builtin && m.ContextWindow == 0 {
				return fmt.Errorf("providers.%s.models.%s: custom models must declare contextWindow", name, mid)
			}
			if m.ContextWindow != 0 && m.ContextWindow < MinContextWindow {
				return fmt.Errorf("providers.%s.models.%s: contextWindow %d below the %d minimum", name, mid, m.ContextWindow, MinContextWindow)
			}
		}
	}
	if c.Context.MaxSteps < 1 || c.Context.ReserveTokens < 1 || c.Context.KeepRecentTokens < 1 {
		return fmt.Errorf("context: reserveTokens, keepRecentTokens and maxSteps must be positive")
	}
	if c.Context.ProgressNudgeAt != nil && *c.Context.ProgressNudgeAt < 0 {
		return fmt.Errorf("context.progressNudgeAt must be >= 0 (0 disables the progress nudge, absent = %d)", DefaultProgressNudgeAt)
	}
	if !slices.Contains(NotifyModes, c.TUI.Notify) {
		return fmt.Errorf("tui.notify %q unknown (want %s)", c.TUI.Notify, strings.Join(NotifyModes, "|"))
	}
	if err := validateTheme("tui.theme", c.TUI.Theme); err != nil {
		return err
	}
	if c.Snapshot.RetentionDays != nil && *c.Snapshot.RetentionDays < 0 {
		return fmt.Errorf("snapshot.retentionDays must be >= 0")
	}
	if c.MCP.IdleTimeout < 0 {
		return fmt.Errorf("mcp.idleTimeout must not be negative (0 means the default, 600)")
	}
	for name, s := range c.MCP.Servers {
		if (s.Command == "") == (s.URL == "") {
			return fmt.Errorf("mcp.servers.%s: exactly one of command (stdio) or url (streamable HTTP)", name)
		}
	}
	switch c.Web.Search.Provider {
	case "tavily", "exa":
	default:
		return fmt.Errorf("web.search.provider %q unknown (want tavily|exa)", c.Web.Search.Provider)
	}
	if k := c.Web.Search.APIKey; k != "" && !strings.HasPrefix(k, envPrefix) {
		return fmt.Errorf("web.search.apiKey must be an env: reference (e.g. \"env:TAVILY_API_KEY\"), never a literal")
	}
	if c.Web.Search.Provider == "exa" && c.Web.Search.APIKey == "" {
		return fmt.Errorf("web.search: provider \"exa\" needs apiKey (an env: reference) — exa has no keyless mode; tavily works without a key")
	}
	switch c.Log.Level {
	case "info", "debug", "off":
	default:
		return fmt.Errorf("log.level %q unknown (want info|debug|off)", c.Log.Level)
	}
	return nil
}

func (c Config) providerNames() []string {
	var n []string
	for k := range c.Providers {
		n = append(n, k)
	}
	slices.Sort(n)
	return n
}
