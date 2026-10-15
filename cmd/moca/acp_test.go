package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/permissions"
)

const acpCfg = `{"model":"loc/m","providers":{"loc":{"baseUrl":"http://127.0.0.1:1","protocol":"openai-completions",
	"auth":"api_key","apiKey":"env:MOCA_T_KEY","models":{"m":{"contextWindow":65536}}}}}`

func TestACPServesOnlyJSONRPC(t *testing.T) {
	isolate(t)
	t.Setenv("XDG_STATE_HOME", t.TempDir()) // the run's log file
	t.Setenv("MOCA_T_KEY", "k")
	cfg := writeCfg(t, acpCfg)
	in := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":1}}` + "\n")
	var out, errb bytes.Buffer
	if code := run(context.Background(), []string{"--config", cfg, "acp"}, in, &out, &errb); code != exitOK {
		t.Fatalf("exit %d stderr %q", code, errb.String())
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("stdout must hold only the initialize response, got %q", out.String())
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &m); err != nil || m["jsonrpc"] != "2.0" || m["result"] == nil {
		t.Fatalf("not a JSON-RPC response: %q", lines[0])
	}
}

// The resolver is decideTrust without the prompt: flag > yolo > saved
// decision > trusted. A saved "no" from the TUI must survive in an editor.
func TestACPTrust(t *testing.T) {
	dir := t.TempDir()
	trustPath := filepath.Join(dir, "trust.json")
	yes, no, fresh := filepath.Join(dir, "yes"), filepath.Join(dir, "no"), filepath.Join(dir, "new")
	for _, d := range []string{yes, no, fresh} {
		os.MkdirAll(d, 0o755) // trust keys are canonical (symlink-resolved) paths
	}
	ts, err := permissions.LoadTrust(trustPath)
	if err != nil {
		t.Fatal(err)
	}
	if ts.Set(yes, true) != nil || ts.Set(no, false) != nil {
		t.Fatal("seeding trust.json failed")
	}
	before, _ := os.ReadFile(trustPath)
	tr, fa := true, false
	cases := []struct {
		name    string
		approve *bool
		yolo    bool
		cwd     string
		want    bool
	}{
		{"saved no", nil, false, no, false},
		{"saved yes", nil, false, yes, true},
		{"unknown dir", nil, false, fresh, true},
		{"--approve beats saved no", &tr, false, no, true},
		{"--no-approve beats everything", &fa, true, yes, false},
		{"yolo trusts", nil, true, no, true},
	}
	for _, c := range cases {
		got, err := acpTrust(c.approve, c.yolo, trustPath)(c.cwd)
		if err != nil || got != c.want {
			t.Errorf("%s: got %v err %v, want %v", c.name, got, err, c.want)
		}
	}
	if after, _ := os.ReadFile(trustPath); string(after) != string(before) {
		t.Error("the resolver must never write trust.json")
	}
}

func TestACPBadFlags(t *testing.T) {
	isolate(t)
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	cfg := writeCfg(t, acpCfg)
	for _, args := range [][]string{{"acp", "--bogus"}, {"acp", "--mode", "turbo"}, {"acp", "--effort", "ultra"}, {"acp", "extra"},
		{"acp", "--approve", "--no-approve"}} {
		var out, errb bytes.Buffer
		code := run(context.Background(), append([]string{"--config", cfg}, args...), strings.NewReader(""), &out, &errb)
		if code != exitUsage || out.Len() != 0 {
			t.Errorf("%v: exit %d stdout %q", args, code, out.String())
		}
	}
}
