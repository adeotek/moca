package main

// Pre-change goldens (codemode plan v13, Task 1). Codemode must not move a
// byte of the eight frozen tool schemas, the rendered system prompt, or the
// direct tools' output — Tasks 3/4/8 compare against exactly these captures,
// so they have to be taken BEFORE any of that work lands. The capture is a
// program, not a hand-edited file: every case rebuilds its fixture from fixed
// constants in a temp dir, so the golden is reproducible on any machine.
//
// The direct-output case list intentionally includes the read window's current
// behaviour around an over-budget line (`long-hole`). Task 3 fixes that bug and
// updates this one case with its reason; every other case must stay
// byte-identical.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/adeotek/moca/internal/agent"
	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/tools"
)

// repoRoot resolves the repository root from this source file, so the capture
// does not depend on the caller's working directory.
func repoRoot() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("cannot locate evalstats sources")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..")), nil
}

// fixedPromptInput is the environment Task 8 compares: every varying field is
// pinned, so the rendered bytes depend only on internal/agent/system-prompt*.md.
func fixedPromptInput() agent.PromptInput {
	return agent.PromptInput{
		Workdir: "/work",
		OS:      "linux",
		Arch:    "amd64",
		Date:    "2026-11-22",
		Git:     "main",
		Version: "v0.8.5",
	}
}

// wireTools is the model-facing tool order, with each spec exactly as the
// adapter will marshal it.
func wireTools() ([]llm.ToolSpec, error) {
	var out []llm.ToolSpec
	for _, t := range tools.Builtins() {
		s := t.Spec()
		if !json.Valid(s.Schema) {
			return nil, fmt.Errorf("tool %s has an invalid schema", s.Name)
		}
		out = append(out, s)
	}
	return out, nil
}

// prechangeCase is one direct-tool call in the golden.
type prechangeCase struct {
	Name string          `json:"name"`
	Tool string          `json:"tool"`
	Args json.RawMessage `json:"args"`
	// Result is the tool's Result as the model sees it.
	Content string `json:"content"`
	IsError bool   `json:"is_error,omitempty"`
	Summary string `json:"summary,omitempty"`
	Detail  string `json:"detail,omitempty"`
}

// jail is the smallest PathChecker the direct tools accept: a workdir and the
// refusal of anything outside it. The real jail adds the spill and skills
// roots; none of these cases needs one.
type jail struct{ root string }

func (j jail) Resolve(p string, write bool) (string, error) {
	abs := p
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(j.root, abs)
	}
	abs = filepath.Clean(abs)
	if abs != j.root && !strings.HasPrefix(abs, j.root+string(filepath.Separator)) {
		return "", fmt.Errorf("%s is outside the workdir", p)
	}
	return abs, nil
}

const (
	plainLines = 30
	longWidth  = 2000 // one read line's byte cap
)

// buildFixture writes the fixed input tree. Everything is generated from the
// constants above, and the two oversize files are sparse: the golden must not
// cost 300 MB on disk to reproduce a size check.
func buildFixture(root string) error {
	var plain strings.Builder
	for i := 1; i <= plainLines; i++ {
		fmt.Fprintf(&plain, "line %02d\n", i)
	}
	files := map[string]string{
		"plain.txt": plain.String(),
		// 24 full-width lines fill 48,000 of the 50,000-byte budget; line 25
		// does not fit and is skipped, then five short lines still fit. The
		// footer claims 1-30 and the output has a hole at 25 — the bug Task 3
		// fixes, captured as it is today.
		"long.txt": longHole(),
		"clip.txt": "short first line\n" + strings.Repeat("x", 3000) + "\nafter the long one\n",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			return err
		}
	}
	if err := os.WriteFile(filepath.Join(root, "binary.bin"), []byte("text\x00then a NUL\n"), 0o644); err != nil {
		return err
	}
	huge, err := os.Create(filepath.Join(root, "huge.bin"))
	if err != nil {
		return err
	}
	// readMaxFileSize is 256 MiB: one byte over is the refusal.
	if err := huge.Truncate((256 << 20) + 1); err != nil {
		huge.Close()
		return err
	}
	if err := huge.Close(); err != nil {
		return err
	}

	tree := filepath.Join(root, "tree")
	if err := os.MkdirAll(filepath.Join(tree, "nested"), 0o755); err != nil {
		return err
	}
	// 40 matching files: enough for a multi-hit result, far below the 5000-hit
	// spill cap, so the golden is about the hit format, not the overflow path.
	for i := 0; i < 40; i++ {
		body := fmt.Sprintf("package p%d\n\n// needle in file %d\nvar v%d = %d\n", i, i, i, i)
		if err := os.WriteFile(filepath.Join(tree, fmt.Sprintf("f%02d.go", i)), []byte(body), 0o644); err != nil {
			return err
		}
	}
	for i := 0; i < 3; i++ {
		if err := os.WriteFile(filepath.Join(tree, "nested", fmt.Sprintf("n%d.go", i)), []byte("// needle nested\n"), 0o644); err != nil {
			return err
		}
	}
	if err := os.WriteFile(filepath.Join(tree, "clean.txt"), []byte("nothing to see\n"), 0o644); err != nil {
		return err
	}
	// Over the 4 MiB search limit: the result must report the skip.
	big, err := os.Create(filepath.Join(tree, "big.txt"))
	if err != nil {
		return err
	}
	if err := big.Truncate((4 << 20) + 1); err != nil {
		big.Close()
		return err
	}
	if err := big.Close(); err != nil {
		return err
	}

	lsdir := filepath.Join(root, "lsdir")
	if err := os.MkdirAll(filepath.Join(lsdir, "sub"), 0o755); err != nil {
		return err
	}
	for i := 0; i < 12; i++ {
		if err := os.WriteFile(filepath.Join(lsdir, fmt.Sprintf("e%02d.txt", i)), []byte("x\n"), 0o644); err != nil {
			return err
		}
	}
	if err := os.WriteFile(filepath.Join(lsdir, ".hidden"), []byte("h\n"), 0o644); err != nil {
		return err
	}
	if err := os.Symlink("e00.txt", filepath.Join(lsdir, "link")); err != nil {
		return err
	}
	return nil
}

// longHole is the over-budget read window described above.
func longHole() string {
	var b strings.Builder
	for i := 1; i <= 24; i++ {
		fmt.Fprintf(&b, "%s\n", strings.Repeat(fmt.Sprintf("%d", i%10), longWidth-1))
	}
	fmt.Fprintf(&b, "%s\n", strings.Repeat("A", longWidth-1)) // line 25: skipped
	for i := 26; i <= plainLines; i++ {
		fmt.Fprintf(&b, "tail line %02d\n", i)
	}
	return b.String()
}

// DirectOutput captures the direct tools' results over the fixed fixture.
func DirectOutput() ([]prechangeCase, error) {
	root, err := os.MkdirTemp("", "moca-prechange-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(root)
	if err := buildFixture(root); err != nil {
		return nil, err
	}
	byName := map[string]tools.Tool{}
	for _, t := range tools.Builtins() {
		byName[t.Spec().Name] = t
	}
	env := &tools.Env{Root: root, Paths: jail{root: root}, Reads: tools.NewReadTracker()}
	cases := []struct {
		name, tool string
		args       map[string]any
	}{
		{"read-window", "read", map[string]any{"path": "plain.txt", "offset": 5, "limit": 3}},
		{"read-long-hole", "read", map[string]any{"path": "long.txt"}},
		{"read-clip", "read", map[string]any{"path": "clip.txt"}},
		{"read-binary", "read", map[string]any{"path": "binary.bin"}},
		{"read-oversize", "read", map[string]any{"path": "huge.bin"}},
		{"read-missing", "read", map[string]any{"path": "nope.txt"}},
		{"search-hits", "search", map[string]any{"pattern": "needle", "path": "tree"}},
		{"search-files-only", "search", map[string]any{"pattern": "needle", "path": "tree", "files_only": true}},
		{"search-no-matches", "search", map[string]any{"pattern": "absent-term", "path": "tree"}},
		{"ls-dir", "ls", map[string]any{"path": "lsdir"}},
		{"ls-hidden", "ls", map[string]any{"path": "lsdir", "hidden": true}},
		{"ls-file", "ls", map[string]any{"path": "plain.txt"}},
	}
	var out []prechangeCase
	for _, c := range cases {
		tool, ok := byName[c.tool]
		if !ok {
			return nil, fmt.Errorf("no builtin tool %q", c.tool)
		}
		raw, err := json.Marshal(c.args)
		if err != nil {
			return nil, err
		}
		r := tool.Run(context.Background(), env, raw)
		out = append(out, prechangeCase{
			Name: c.name, Tool: c.tool, Args: raw,
			// The temp root is the only machine-dependent string in a result.
			Content: strings.ReplaceAll(r.Content, root, "<root>"),
			IsError: r.IsError,
			Summary: strings.ReplaceAll(r.Summary, root, "<root>"),
			Detail:  strings.ReplaceAll(r.Detail, root, "<root>"),
		})
	}
	return out, nil
}

// prechangeRelPaths are the committed locations, relative to the repo root.
const (
	prechangePrompt = "test/evals/testdata/prechange/system-prompt.txt"
	prechangeWire   = "test/evals/testdata/prechange/wire-tools.json"
	prechangeDirect = "internal/tools/testdata/direct-output.golden.json"
)

// CapturePrechange renders every pre-change golden into memory, keyed by its
// repo-relative path.
func CapturePrechange() (map[string][]byte, error) {
	wire, err := wireTools()
	if err != nil {
		return nil, err
	}
	wireJSON, err := json.MarshalIndent(wire, "", "  ")
	if err != nil {
		return nil, err
	}
	direct, err := DirectOutput()
	if err != nil {
		return nil, err
	}
	directJSON, err := json.MarshalIndent(direct, "", "  ")
	if err != nil {
		return nil, err
	}
	return map[string][]byte{
		prechangePrompt: append([]byte(agent.BuildSystemPrompt(fixedPromptInput())), '\n'),
		prechangeWire:   append(wireJSON, '\n'),
		prechangeDirect: append(directJSON, '\n'),
	}, nil
}

// writeGoldens writes a capture under dir (the repo root, or a temp dir in a
// test) and returns the paths it wrote, sorted.
func writeGoldens(dir string, files map[string][]byte) ([]string, error) {
	paths := make([]string, 0, len(files))
	for rel := range files {
		paths = append(paths, rel)
	}
	sort.Strings(paths)
	for _, rel := range paths {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(p, files[rel], 0o644); err != nil {
			return nil, err
		}
	}
	return paths, nil
}

// WritePrechange captures and writes every golden (the capture command).
func WritePrechange() error {
	root, err := repoRoot()
	if err != nil {
		return err
	}
	files, err := CapturePrechange()
	if err != nil {
		return err
	}
	paths, err := writeGoldens(root, files)
	if err != nil {
		return err
	}
	for _, rel := range paths {
		fmt.Println("wrote", filepath.Join(root, rel))
	}
	return nil
}

// checkGoldens compares a capture with the files committed under dir. It
// reports the first differing line per file — a raw diff of the prompt would
// bury the actual change.
func checkGoldens(dir string, files map[string][]byte) error {
	paths := make([]string, 0, len(files))
	for rel := range files {
		paths = append(paths, rel)
	}
	sort.Strings(paths)
	var errs []string
	for _, rel := range paths {
		p := filepath.Join(dir, rel)
		want, err := os.ReadFile(p)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", rel, err))
			continue
		}
		if string(want) == string(files[rel]) {
			continue
		}
		errs = append(errs, fmt.Sprintf("%s: first difference at %s", rel, firstDiff(string(want), string(files[rel]))))
	}
	if len(errs) > 0 {
		return fmt.Errorf("pre-change goldens moved:\n  %s", strings.Join(errs, "\n  "))
	}
	return nil
}

// CheckPrechange verifies the committed goldens against a fresh capture.
func CheckPrechange() error {
	root, err := repoRoot()
	if err != nil {
		return err
	}
	files, err := CapturePrechange()
	if err != nil {
		return err
	}
	return checkGoldens(root, files)
}

// firstDiff names the line and the two values, so a failed pin says what
// changed rather than only that bytes differ.
func firstDiff(want, got string) string {
	wl, gl := strings.Split(want, "\n"), strings.Split(got, "\n")
	for i := 0; i < len(wl) || i < len(gl); i++ {
		var w, g string
		if i < len(wl) {
			w = wl[i]
		}
		if i < len(gl) {
			g = gl[i]
		}
		if w != g {
			return fmt.Sprintf("line %d: committed %q, captured %q", i+1, short(w), short(g))
		}
	}
	return "no difference (unreachable)"
}

func short(s string) string {
	if len(s) > 120 {
		return s[:120] + "…"
	}
	return s
}
