package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadProject(t *testing.T) {
	wd := t.TempDir()
	if p, err := LoadProject(wd); p != nil || err != nil {
		t.Fatalf("absent file: %v %v", p, err)
	}
	if err := os.MkdirAll(filepath.Join(wd, ".moca"), 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(s string) {
		if err := os.WriteFile(ProjectConfigFile(wd), []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	write(`{ // project
	  "permissions": {"defaultMode": "auto", "allow": ["make *"], "deny": ["git push *"],},
	}`)
	p, err := LoadProject(wd)
	if err != nil || p.DefaultMode != "auto" || p.Allow[0] != "make *" {
		t.Fatalf("%+v %v", p, err)
	}
	for src, want := range map[string]string{
		`{"model":"x/y"}`:                                 "unknown field",
		`{"permissions":{"defaultMode":"yolo"}}`:          "yolo",
		`{"permissions":{"allow":["*"]}}`:                 `"*"`,
		`{"permissions":{"allow":["**"]}}`:                "matches every command",
		`{"permissions":{"allow":["* *"]}}`:               "matches every command",
		`{"permissions":{"allow":["* rm"]}}`:              "matches every command",
		`{"permissions":{"allow":["  *"]}}`:               "matches every command",
		`{"permissions":{"auto":{"model":"openai/gpt"}}}`: "auto.model",
	} {
		write(src)
		if _, err := LoadProject(wd); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: err = %v, want %q", src, err, want)
		}
	}
	for _, ok := range []string{`["make *"]`, `["git status"]`} {
		write(`{"permissions":{"allow":` + ok + `}}`)
		if _, err := LoadProject(wd); err != nil {
			t.Errorf("%s: %v", ok, err)
		}
	}
}
