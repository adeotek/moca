package tools

// Pins for the v0.7.7 review pass: an unrecognised answer fails closed, and the
// config-file exclusion folds case exactly where the filesystem does.

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestUnrecognisedAnswerFailsClosed: an Answer this build does not know must
// never run the command. The old fixed-size index panicked here (recovered into
// an error result) and the first switch rewrite fell through to "run it" — a
// fail-open on exactly the value a non-conforming front end could send.
func TestUnrecognisedAnswerFailsClosed(t *testing.T) {
	env, _ := testEnv(t)
	env.ShellEnv = nil
	cmds := &fakeCmds{need: []string{"python"}}
	env.Commands = cmds
	env.Ask = func(context.Context, Question) Answer { return Answer(99) }
	r := run(t, shellTool{}, env, map[string]any{"command": "python -c pass"})
	if !r.IsError || !strings.Contains(r.Content, "not recognised") {
		t.Fatalf("an unrecognised answer must refuse, got %+v", r)
	}
	if len(cmds.allowed) != 0 {
		t.Fatalf("nothing may be learned from an unrecognised answer: %v", cmds.allowed)
	}
}

// TestUnrecognisedAnswerFailsClosedEverywhere: the shell's fail-closed rule
// holds at the other approval sites too. The web fetch and the outside-the-
// workdir write tested `== Deny`, so an out-of-range Answer read as a yes —
// the fetch ran and the file landed.
func TestUnrecognisedAnswerFailsClosedEverywhere(t *testing.T) {
	bogus := func(context.Context, Question) Answer { return Answer(99) }

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("secret"))
	}))
	defer srv.Close()
	r := webTool{}.Run(context.Background(), &Env{Ask: bogus}, json.RawMessage(`{"op":"fetch","url":"`+srv.URL+`/"}`))
	if !r.IsError || strings.Contains(r.Content, "secret") {
		t.Errorf("web: an unrecognised answer must refuse the private fetch, got %+v", r)
	}

	root := t.TempDir()
	prompts := filepath.Join(t.TempDir(), "prompts")
	os.MkdirAll(prompts, 0o755)
	target := filepath.Join(prompts, "x.md")
	env := &Env{Root: root, Paths: askRootChecker{root, prompts}, Reads: NewReadTracker(), Ask: bogus}
	r = run(t, writeTool{}, env, map[string]any{"path": target, "content": "x"})
	if !r.IsError {
		t.Errorf("write: an unrecognised answer must refuse, got %+v", r)
	}
	if _, err := os.Stat(target); err == nil {
		t.Error("write: the file landed on an unrecognised answer")
	}
}

// caseAskChecker is fileAskChecker with a folding Approvable: the target can
// arrive spelled in another case and still name the same file (the Windows /
// macOS jail's own containment rule).
type caseAskChecker struct{ root, file string }

func (c caseAskChecker) Resolve(path string, write bool) (string, error) {
	p := filepath.Clean(path)
	if !filepath.IsAbs(p) {
		p = filepath.Join(c.root, p)
	}
	if p == c.root || strings.HasPrefix(p, c.root+string(filepath.Separator)) {
		return p, nil
	}
	if write {
		return "", errors.New(path + " is outside the workdir; writes are confined to the workdir")
	}
	return p, nil
}

func (c caseAskChecker) Approvable(path string) (string, bool) {
	p := filepath.Clean(path)
	if strings.EqualFold(p, c.file) {
		return p, true
	}
	return "", false
}

func TestSamePathFoldsWhereTheFilesystemDoes(t *testing.T) {
	old := caseInsensitiveFS
	t.Cleanup(func() { caseInsensitiveFS = old })
	for _, c := range []struct {
		a, b string
		fold bool
		want bool
	}{
		{"/a/b/config.jsonc", "/a/b/config.jsonc", false, true},
		{"/a/B/config.jsonc", "/a/b/config.jsonc", false, false},
		{"/a/B/config.jsonc", "/a/b/config.jsonc", true, true},
		{`C:\Users\X\CONFIG.JSONC`, `c:\users\x\config.jsonc`, true, true},
		{"/a/b/config.jsonc", "/a/b", true, false},
		{"/a/b", "/x/b", true, false},
	} {
		caseInsensitiveFS = c.fold
		if got := samePath(c.a, c.b); got != c.want {
			t.Errorf("samePath(%q, %q) with fold=%v = %v, want %v", c.a, c.b, c.fold, got, c.want)
		}
	}
}

// TestConfigFileExclusionFoldsCase: where the filesystem folds case, a write
// spelled in another case is still the config file — the session answer must be
// withheld there too, or the exclusion has a spelling-shaped hole.
func TestConfigFileExclusionFoldsCase(t *testing.T) {
	old := caseInsensitiveFS
	caseInsensitiveFS = true
	t.Cleanup(func() { caseInsensitiveFS = old })

	root := t.TempDir()
	cfgDir := t.TempDir()
	cfg := filepath.Join(cfgDir, "config.jsonc")
	spelled := filepath.Join(cfgDir, "CONFIG.JSONC") // same file on a folding FS

	var asked []Question
	env := &Env{Root: root, Paths: caseAskChecker{root, cfg}, Reads: NewReadTracker(), ConfigFile: cfg,
		Ask: func(_ context.Context, q Question) Answer { asked = append(asked, q); return AllowOnce }}
	abs, err := askOutsideWrite(context.Background(), env, spelled, errors.New("refused"))
	if err != nil {
		t.Fatalf("the folded spelling must still be approvable: %v", err)
	}
	if abs != spelled {
		t.Fatalf("resolved target %q, want the spelling the user approved (%q)", abs, spelled)
	}
	if len(asked) != 1 || !asked[0].NoSession {
		t.Fatalf("a case-variant spelling of the config file must not offer the session answer: %+v", asked)
	}
}
