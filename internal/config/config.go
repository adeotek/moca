package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"slices"
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
	Yolo      bool                      `json:"yolo"` // all permission checks off by default (§7.5)
}

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
}

// NotifyModes are the valid tui.notify values.
var NotifyModes = []string{"osc9", "bell", "off"}

type ContextConfig struct {
	ReserveTokens    int `json:"reserveTokens"`
	KeepRecentTokens int `json:"keepRecentTokens"`
	MaxSteps         int `json:"maxSteps"`
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
	if !slices.Contains(NotifyModes, c.TUI.Notify) {
		return fmt.Errorf("tui.notify %q unknown (want %s)", c.TUI.Notify, strings.Join(NotifyModes, "|"))
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
