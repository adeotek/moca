package permissions

// Pin for T-9: only `~` and `~/…` are expanded. `~root/x` and `~+/x` are host
// shell forms moca does not perform; joining them to the workdir addressed a
// directory literally named `~root` while the command analyser refused the same
// form — two layers disagreeing about the same path.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveRefusesUnsupportedTildeForms(t *testing.T) {
	root := t.TempDir()
	j, err := NewJail(root, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"~root/x", "~+/x", "~other"} {
		if got, err := j.Resolve(p, true); err == nil {
			t.Errorf("Resolve(%q) = %q, want a refusal", p, got)
		}
	}

	// Controls: the two supported forms still resolve, and a plain relative
	// path is unaffected.
	if err := os.WriteFile(filepath.Join(root, "x"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, err := j.Resolve("x", true); err != nil || !strings.HasSuffix(got, "x") {
		t.Fatalf("Resolve(x) = %q, %v", got, err)
	}
	// ~/x is expanded (and then refused for being outside the workdir — a
	// different reason; the guard must not swallow the supported form).
	if _, err := j.Resolve("~/x", true); err == nil || strings.Contains(err.Error(), "does not resolve this form") {
		t.Fatalf("Resolve(~/x) = %v, want an outside-the-workdir refusal, not the tilde guard", err)
	}
}
