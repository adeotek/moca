package acp

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/adeotek/moca/internal/config"
)

func TestInitialize(t *testing.T) {
	c := startClient(t, testOptions(t, newScript(t)))
	res, err := c.call("initialize", InitializeRequest{ProtocolVersion: 1})
	if err != nil {
		t.Fatal(err.Message)
	}
	var r InitializeResponse
	json.Unmarshal(res, &r)
	if r.ProtocolVersion != 1 || r.AgentInfo.Name != "moca" || len(r.AuthMethods) != 1 {
		t.Fatalf("%s", res)
	}
	// Exactly one method, of ACP's default `agent` kind: the registry CI
	// requires a non-empty authMethods, and an editor needs something to
	// show beside a session that cannot open (§9.5).
	am := r.AuthMethods[0]
	if am.ID != "login" || am.Name != "Sign in" || !strings.Contains(am.Description, "moca login") {
		t.Fatalf("authMethods: %+v", am)
	}
	// ACP allows the `terminal` kind only when the client enabled its terminal
	// authentication capability, so the entry must stay the default `agent`
	// kind: no `type` discriminator on the wire.
	if b, err := json.Marshal(am); err != nil || strings.Contains(string(b), `"type"`) {
		t.Fatalf("authMethod must serialise without a type discriminator: %s (%v)", b, err)
	}
	conform(t, "InitializeResponse", r)
}

// authenticate answers the advertised method with the credential check a
// session/new makes: success only when a credential really resolves.
func TestAuthenticate(t *testing.T) {
	c := startClient(t, testOptions(t, newScript(t)))
	if _, err := c.call("authenticate", AuthenticateRequest{MethodID: "login"}); err != nil {
		t.Fatalf("a stored/env credential must authenticate: %+v", err)
	}
	// An id moca never advertised is refused rather than answered.
	if _, err := c.call("authenticate", AuthenticateRequest{MethodID: "terminal"}); err == nil || err.Code != codeInvalidParams {
		t.Fatalf("unknown method: %+v", err)
	}
	// No credential for the configured provider: the error carries the fix
	// (the same answer session/new gives), never a silent success.
	o := testOptions(t, newScript(t))
	o.Config.Model = "other/x" // MOCA_T_OTHER is empty
	c2 := startClient(t, o)
	_, err := c2.call("authenticate", AuthenticateRequest{MethodID: "login"})
	if err == nil || err.Code != codeInternal || !strings.Contains(err.Message, "moca login other") {
		t.Fatalf("missing credential: %+v", err)
	}
}

func TestUnknownMethodAndSession(t *testing.T) {
	c := startClient(t, testOptions(t, newScript(t)))
	if _, err := c.call("session/nope", map[string]any{}); err == nil || err.Code != codeMethodNotFound {
		t.Fatalf("unknown method: %+v", err)
	}
	if _, err := c.prompt("deadbeef", "hi"); err == nil || err.Code != codeResourceNotFound {
		t.Fatalf("unknown session: %+v", err)
	}
}

func TestNewSessionValidatesCwd(t *testing.T) {
	c := startClient(t, testOptions(t, newScript(t)))
	for _, cwd := range []string{"relative/dir", filepath.Join(t.TempDir(), "missing")} {
		if _, err := c.call("session/new", NewSessionRequest{Cwd: cwd}); err == nil || err.Code != codeInvalidParams {
			t.Errorf("cwd %q: %+v", cwd, err)
		}
	}
}

func TestNewSessionWithoutModel(t *testing.T) {
	o := testOptions(t, newScript(t))
	o.Config.Model = ""
	o.ConfigPath = "/cfg/config.jsonc"
	c := startClient(t, o)
	_, err := c.call("session/new", NewSessionRequest{Cwd: t.TempDir()})
	// A plain error, not ACP's auth-required: the fix is a `moca login` the
	// editor cannot run, so the message must reach the user as itself (§9.5).
	if err == nil || err.Code != codeInternal || !strings.Contains(err.Message, "moca login") || !strings.Contains(err.Message, "/cfg/config.jsonc") {
		t.Fatalf("%+v", err)
	}
}

// A model whose provider has no credential must not open a session that
// fails on every prompt (the TUI refuses the same switch): -32603, naming
// the fix. Resume/load keep their own check (agent.Resume, stored model).
func TestNewSessionWithoutCredential(t *testing.T) {
	o := testOptions(t, newScript(t))
	o.Config.Model = "other/x" // MOCA_T_OTHER is empty: no env key, no stored login
	c := startClient(t, o)
	_, err := c.call("session/new", NewSessionRequest{Cwd: t.TempDir()})
	if err == nil || err.Code != codeInternal || !strings.Contains(err.Message, "moca login other") {
		t.Fatalf("%+v", err)
	}
	// An unknown model (a config typo) is refused the same way: it is not a
	// catalog model, so no session can serve it.
	o2 := testOptions(t, newScript(t))
	o2.Config.Model = "fake/nope"
	c2 := startClient(t, o2)
	_, err = c2.call("session/new", NewSessionRequest{Cwd: t.TempDir()})
	if err == nil || err.Code != codeInternal || !strings.Contains(err.Message, "not in the catalog") {
		t.Fatalf("unknown model: %+v", err)
	}
}

// TestModelCheckWithOllamaConfigured is a real editor's setup: Options.HTTP
// is nil (cmd/moca never sets it) and providers.ollama is configured while
// the model lives on another provider. The model check's discovery ran on
// that nil client and panicked, so authenticate and every session/new
// answered -32603 "internal error … nil pointer dereference" (v0.7.8, Zed on
// Windows).
func TestModelCheckWithOllamaConfigured(t *testing.T) {
	var tags atomic.Int32
	oll := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			tags.Add(1)
		}
		io.WriteString(w, `{"models":[]}`)
	}))
	t.Cleanup(oll.Close) // registered first: closes after the client stops
	o := testOptions(t, newScript(t))
	o.HTTP = nil
	o.Config.Providers["ollama"] = config.ProviderConfig{Auth: "api_key", BaseURL: oll.URL}
	c := startClient(t, o)
	// authenticate runs only the model check, so the discovery hit is its own.
	if _, err := c.call("authenticate", AuthenticateRequest{MethodID: "login"}); err != nil {
		t.Fatalf("authenticate: %+v", err)
	}
	if tags.Load() == 0 {
		t.Fatal("the model check never asked the configured ollama")
	}
	c.newSession(t.TempDir())
}

func TestPromptStreamsTextAndEndsTurn(t *testing.T) {
	c := startClient(t, testOptions(t, newScript(t, textTurn("Hello there."))))
	sid := c.newSession(t.TempDir())
	if !regexp.MustCompile(`^[0-9a-f]{8}$`).MatchString(sid) {
		t.Fatalf("sessionId %q is not a moca id8", sid)
	}
	stop, err := c.prompt(sid, "hi")
	if err != nil || stop != stopEndTurn {
		t.Fatalf("stop %q err %+v", stop, err)
	}
	if got := c.agentText(); got != "Hello there." {
		t.Fatalf("agent text %q", got)
	}
	for _, u := range c.updatesOf(updAgentMessage) {
		conform(t, "SessionNotification", SessionNotification{SessionID: sid, Update: u})
	}
	if c.updatesOf(updUserMessage) != nil {
		t.Fatal("a live prompt must not echo the user message (the client already shows it)")
	}
	usage := c.updatesOf(updUsage)
	if len(usage) != 1 || usage[0]["size"] != float64(65536) || usage[0]["used"].(float64) <= 0 {
		t.Fatalf("usage_update after the turn: %v", usage)
	}
	conform(t, "SessionNotification", SessionNotification{SessionID: sid, Update: usage[0]})
}

func TestPromptToolCall(t *testing.T) {
	work := t.TempDir()
	os.WriteFile(filepath.Join(work, "a.txt"), []byte("hello\n"), 0o644)
	c := startClient(t, testOptions(t, newScript(t, toolTurn([2]string{"read", `{"path":"a.txt"}`}), textTurn("Read it."))))
	sid := c.newSession(work)
	if stop, err := c.prompt(sid, "read a.txt"); err != nil || stop != stopEndTurn {
		t.Fatalf("stop %q err %+v", stop, err)
	}
	starts, ends := c.updatesOf(updToolCall), c.updatesOf(updToolUpdate)
	if len(starts) != 1 || len(ends) != 1 {
		t.Fatalf("starts %v ends %v", starts, ends)
	}
	if starts[0]["kind"] != "read" || starts[0]["status"] != statusInProgress || ends[0]["status"] != statusCompleted ||
		starts[0]["toolCallId"] != ends[0]["toolCallId"] {
		t.Fatalf("start %v end %v", starts[0], ends[0])
	}
}

func TestToolCallIDsUniqueAcrossTurns(t *testing.T) {
	work := t.TempDir()
	os.WriteFile(filepath.Join(work, "a.txt"), []byte("x\n"), 0o644)
	// Both turns' calls arrive as call_0 from the provider.
	c := startClient(t, testOptions(t, newScript(t,
		toolTurn([2]string{"read", `{"path":"a.txt"}`}),
		toolTurn([2]string{"ls", `{}`}),
		textTurn("ok"))))
	sid := c.newSession(work)
	c.prompt(sid, "go")
	starts := c.updatesOf(updToolCall)
	if len(starts) != 2 || starts[0]["toolCallId"] == starts[1]["toolCallId"] {
		t.Fatalf("tool call ids not unique: %v", starts)
	}
}

func TestProjectUntrustedSkipsProjectFile(t *testing.T) {
	work := t.TempDir()
	os.MkdirAll(filepath.Join(work, ".moca"), 0o755)
	os.WriteFile(filepath.Join(work, ".moca", "config.jsonc"), []byte("{not valid"), 0o644)
	o := testOptions(t, newScript(t, textTurn("ok")))
	o.Trust = func(cwd string) (bool, error) { return cwd != work, nil } // trust is per workdir
	c := startClient(t, o)
	c.newSession(work) // an untrusted workdir's broken project file is ignored entirely
	o2 := testOptions(t, newScript(t))
	c2 := startClient(t, o2)
	if _, err := c2.call("session/new", NewSessionRequest{Cwd: work}); err == nil {
		t.Fatal("a trusted workdir's broken project file must be reported")
	}
}
