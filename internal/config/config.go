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

var Protocols = []string{"anthropic-messages", "openai-completions", "openai-responses"}

type Config struct {
	Model     string                    `json:"model"`
	ModelHard string                    `json:"modelHard"`
	Providers map[string]ProviderConfig `json:"providers"`
	Shell     ShellConfig               `json:"shell"`
	MCP       MCPConfig                 `json:"mcp"`
	Snapshot  SnapshotConfig            `json:"snapshot"`
	Context   ContextConfig             `json:"context"`
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
			if !strings.HasPrefix(p.APIKey, envPrefix) {
				return fmt.Errorf("providers.%s.apiKey must be an env: reference (e.g. \"env:MY_KEY\"), never a literal", name)
			}
		case "oauth":
		default:
			return fmt.Errorf("providers.%s.auth must be \"api_key\" or \"oauth\", got %q", name, p.Auth)
		}
		_, builtin := BuiltinProviders[name]
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
	if c.Snapshot.RetentionDays != nil && *c.Snapshot.RetentionDays < 0 {
		return fmt.Errorf("snapshot.retentionDays must be >= 0")
	}
	if c.MCP.IdleTimeout < 0 {
		return fmt.Errorf("mcp.idleTimeout must be > 0 seconds (0 means the default, 600)")
	}
	for name, s := range c.MCP.Servers {
		if (s.Command == "") == (s.URL == "") {
			return fmt.Errorf("mcp.servers.%s: exactly one of command (stdio) or url (streamable HTTP)", name)
		}
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
