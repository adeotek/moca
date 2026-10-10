package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLogConfig(t *testing.T) {
	c, err := Parse([]byte(`{"model":"anthropic/x"}`))
	if err != nil {
		t.Fatal(err)
	}
	if c.Log.Level != "info" {
		t.Fatalf("default level: %q", c.Log.Level)
	}
	for _, lv := range []string{"info", "debug", "off"} {
		c, err := Parse([]byte(`{"model":"anthropic/x","log":{"level":"` + lv + `"}}`))
		if err != nil || c.Log.Level != lv {
			t.Fatalf("%s: %+v %v", lv, c.Log, err)
		}
	}
	bad := []struct{ cfg, want string }{
		{`{"log":{"level":"trace"}}`, "log.level"},
		{`{"log":{"levl":"info"}}`, "levl"}, // strict decode: unknown key
	}
	for _, b := range bad {
		if _, err := Parse([]byte(b.cfg)); err == nil || !strings.Contains(err.Error(), b.want) {
			t.Fatalf("%s: want error containing %q, got %v", b.cfg, b.want, err)
		}
	}
}

func TestStateDir(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", filepath.Join("x", "state"))
	if got, want := StateDir(), filepath.Join("x", "state", "moca"); got != want {
		t.Fatalf("XDG: %q want %q", got, want)
	}
	t.Setenv("XDG_STATE_HOME", "")
	h, _ := os.UserHomeDir()
	if got, want := StateDir(), filepath.Join(h, ".local", "state", "moca"); got != want {
		t.Fatalf("fallback: %q want %q", got, want)
	}
}
