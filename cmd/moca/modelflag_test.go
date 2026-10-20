package main

// Pin for F-1: `--model ollama/qwen3` is the opt-in for a local provider that
// has no config entry. The flag was applied to the config and then validated
// before `UseProvider` ran, so the very model the user named was rejected with
// "unknown provider ollama". The stub below asserts the flag reaches Start's
// config with the provider added — the run then fails later, on the model
// itself, which is the point: validation no longer blocks the opt-in.

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestModelFlagOptsInALocalProvider(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	cfgPath := filepath.Join(dir, "moca", "config.jsonc")
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatal(err)
	}
	// A config with no providers at all — the state a fresh install is in.
	if err := os.WriteFile(cfgPath, []byte(`{"model":"anthropic/claude-haiku-4-5"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"--model", "ollama/qwen3", "-p", "hi"}, strings.NewReader(""), &stdout, &stderr)
	if strings.Contains(stderr.String(), "unknown provider") {
		t.Fatalf("the --model opt-in was rejected before it was applied: %s", stderr.String())
	}
	if code == 0 && stdout.Len() == 0 {
		t.Fatal("a zero exit with no output proves nothing")
	}
}
