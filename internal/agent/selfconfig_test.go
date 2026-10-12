package agent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/tools"
)

// TestAgentReachesItsOwnConfig: the user config file is a read root (the
// self-knowledge skill answers "what is configured now" from it) and an
// ask-write root (self-configure edits it with the user's approval, never
// silently) — exactly that file: auth.json beside it stays unreachable, for
// reads and writes alike.
func TestAgentReachesItsOwnConfig(t *testing.T) {
	s := newScript(t)
	var asked []tools.Question
	a, _, _ := startWith(t, s, 40, "", "", func(o *StartOptions) {
		o.Ask = func(_ context.Context, q tools.Question) tools.Answer {
			asked = append(asked, q)
			return tools.AllowOnce
		}
	})
	run := func(name string, in map[string]any) tools.Result {
		b, _ := json.Marshal(in)
		return a.opts.Tools.Run(context.Background(), a.opts.Env, llm.ToolCall{Name: name, Input: b})
	}

	cfgPath := config.ConfigFile()
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgPath, []byte("{\n  \"model\": \"fake/m\"\n}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Read: allowed — the config file is a read root outside the workdir.
	if r := run("read", map[string]any{"path": cfgPath}); r.IsError || !strings.Contains(r.Content, "fake/m") {
		t.Fatalf("read config: %q", r.Content)
	}
	// Write: the user is asked; allow-once writes, the question cannot be
	// remembered (CanAlways false).
	if r := run("edit", map[string]any{"path": cfgPath, "old_string": `"fake/m"`, "new_string": `"fake/m2"`}); r.IsError {
		t.Fatalf("edit config: %q", r.Content)
	}
	if len(asked) != 1 || asked[0].Kind != "write" || asked[0].CanAlways || asked[0].Subject != cfgPath {
		t.Fatalf("approval question: %+v", asked)
	}
	if b, _ := os.ReadFile(cfgPath); !strings.Contains(string(b), "fake/m2") {
		t.Fatalf("config not written: %q", b)
	}
	// A second edit asks again (nothing persists).
	if r := run("edit", map[string]any{"path": cfgPath, "old_string": `"fake/m2"`, "new_string": `"fake/m"`}); r.IsError || len(asked) != 2 {
		t.Fatalf("second edit must re-ask: %q (%d questions)", r.Content, len(asked))
	}

	// auth.json beside it: out of reach by construction — a read refuses,
	// and no approval can hand it out.
	auth := config.AuthFile()
	if err := os.WriteFile(auth, []byte("{\"apiKeys\":{\"x\":\"sk-secret\"}}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if r := run("read", map[string]any{"path": auth}); !r.IsError {
		t.Fatalf("auth.json must not be readable: %q", r.Content)
	}
	if r := run("write", map[string]any{"path": auth, "content": "x"}); !r.IsError || len(asked) != 2 {
		t.Fatalf("auth.json must not be writable/asked: %q (%d questions)", r.Content, len(asked))
	}
}
