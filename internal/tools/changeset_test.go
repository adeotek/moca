package tools

// Workspace changeset core (SPECS §10.6, plan v7 §4). The fakes here are the
// narrow structural interfaces the real session binding implements; nothing in
// this file needs a session, a permissions engine or a provider.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

type fakeJournal struct {
	mu                                sync.Mutex
	batches                           []ChangesetBatch
	records                           []string
	aborts                            []string
	finished                          bool
	failBegin, failRecord, failFinish bool
}

func (j *fakeJournal) Begin(b ChangesetBatch) error {
	if j.failBegin {
		return errors.New("begin refused")
	}
	j.batches = append(j.batches, b)
	return nil
}
func (j *fakeJournal) Record(p string, _, _ []byte) error {
	if j.failRecord {
		return errors.New("record refused")
	}
	j.records = append(j.records, p)
	return nil
}
func (j *fakeJournal) Abort(reason string) error { j.aborts = append(j.aborts, reason); return nil }
func (j *fakeJournal) Finish() error {
	if j.failFinish {
		return errors.New("finish refused")
	}
	j.finished = true
	return nil
}

// csFakeSnap implements ImageSnapshotter and records what it was handed.
type csFakeSnap struct {
	writes    int
	pre, post []byte
	// corrupt writes something that is not post, without failing.
	corrupt bool
	// failWrite fails the write closure without calling it.
	failWrite bool
}

func (s *csFakeSnap) Guard(absPath string, write func() error) error { return write() }

func (s *csFakeSnap) GuardImage(abs string, pre, post []byte, mode os.FileMode, write func() error) error {
	s.writes++
	s.pre, s.post = append([]byte(nil), pre...), append([]byte(nil), post...)
	if s.failWrite {
		return errors.New("snapshot refused")
	}
	if err := write(); err != nil {
		return err
	}
	if s.corrupt {
		return os.WriteFile(abs, []byte("not the image"), 0o644)
	}
	return nil
}

func newBatch(t *testing.T, files map[string]string) (string, *Changeset) {
	t.Helper()
	dir := t.TempDir()
	for rel, body := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir, NewChangeset(ChangesetPolicy{Edits: true, Trusted: true, Mode: "yolo"}, dir)
}

func stageCall(t *testing.T, c *Changeset, path string, pairs ...[2]string) (string, error) {
	t.Helper()
	type pair struct {
		Old string `json:"old"`
		New string `json:"new"`
	}
	a := struct {
		Path  string `json:"path"`
		Edits []pair `json:"edits"`
	}{Path: path}
	for _, p := range pairs {
		a.Edits = append(a.Edits, pair{Old: p[0], New: p[1]})
	}
	out, err := c.Stage(context.Background(), mustJSON(t, a))
	return string(out), err
}

func TestChangesetPathPolicy(t *testing.T) {
	_, c := newBatch(t, map[string]string{
		"src/a.go":             "package a\n",
		".env.example":         "KEY=\n",
		"a/auth.json":          "{}",
		"a/.env":               "X=1\n",
		"a/.env.example.local": "X=1\n",
	})
	refused := []string{"/etc/passwd", "../x.go", "./src/a.go", "src/../a.go", ".git/config", ".moca/config.jsonc",
		"a/auth.json", "a/.env", "a/.env.example.local", "AGENTS.md", "a/CLAUDE.md", "src/", "", "src/a.go\x00"}
	for _, p := range refused {
		if _, err := c.checkPath(p); err == nil {
			t.Fatalf("%q was accepted", p)
		} else if err.(*changesetError).class == "mismatch" {
			t.Fatalf("%q refused as correctable", p)
		}
	}
	for _, p := range []string{"src/a.go", ".env.example", ".ENV.SAMPLE", "services/api/.env.template"} {
		if got, err := c.checkPath(p); err != nil || got != p {
			t.Fatalf("%q = %q, %v; want accepted", p, got, err)
		}
	}
}

func TestChangesetStageAppliesOrderedPairs(t *testing.T) {
	dir, c := newBatch(t, map[string]string{"svc.yaml": "image: api:1.4.1\ndigest: sha256:aaaa\nnote: héllo\n"})
	defer c.Close()
	out, err := stageCall(t, c, "svc.yaml",
		[2]string{"image: api:1.4.1", "image: api:1.5.1"},
		[2]string{"digest: sha256:aaaa", "digest: sha256:bbbb"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"edits":2`) || !strings.Contains(out, `"staged":true`) {
		t.Fatalf("stage result = %s", out)
	}
	if c.TargetCount() != 1 {
		t.Fatalf("targets = %d", c.TargetCount())
	}
	// The original file is untouched until apply.
	orig, _ := os.ReadFile(filepath.Join(dir, "svc.yaml"))
	if !bytes.Contains(orig, []byte("1.4.1")) {
		t.Fatal("staging wrote the file")
	}
}

// The staging result carries the bytes the host will write, so a program can
// check its own work instead of verifying through a shell in a later turn.
func TestChangesetStageReturnsPostEditContent(t *testing.T) {
	_, c := newBatch(t, map[string]string{"svc.yaml": "name: x\nreplicas: 2\n"})
	defer c.Close()
	out, err := stageCall(t, c, "svc.yaml", [2]string{"replicas: 2", "replicas: 3"})
	if err != nil {
		t.Fatalf("stage: %v", err)
	}
	var got struct {
		Staged    bool   `json:"staged"`
		Content   string `json:"content"`
		SHA256    string `json:"sha256"`
		Bytes     int    `json:"bytes"`
		Truncated bool   `json:"truncated"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("result is not json: %v (%s)", err, out)
	}
	want := "name: x\nreplicas: 3\n"
	if got.Content != want {
		t.Fatalf("content = %q, want %q", got.Content, want)
	}
	if got.Bytes != len(want) || got.Truncated {
		t.Fatalf("bytes=%d truncated=%v for a small file", got.Bytes, got.Truncated)
	}
	sum := sha256.Sum256([]byte(want))
	if got.SHA256 != hex.EncodeToString(sum[:]) {
		t.Fatalf("sha256 = %q, want %q", got.SHA256, hex.EncodeToString(sum[:]))
	}
}

// A file over the window is cut at a rune boundary, and the digest still covers
// the whole image -- identity must not depend on the text fitting.
func TestChangesetStageTruncatesLargeImage(t *testing.T) {
	pre := "name: big\n" + strings.Repeat("pad: 0123456789\n", 6000) + "END — done\n"
	_, c := newBatch(t, map[string]string{"big.yaml": pre})
	defer c.Close()
	out, err := stageCall(t, c, "big.yaml", [2]string{"END — done\n", "END — done\nadded: 1\n"})
	if err != nil {
		t.Fatalf("stage: %v", err)
	}
	var got struct {
		Content   string `json:"content"`
		SHA256    string `json:"sha256"`
		Bytes     int    `json:"bytes"`
		Truncated bool   `json:"truncated"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("result is not json: %v", err)
	}
	post := pre + "added: 1\n"
	if !got.Truncated {
		t.Fatalf("a %d B image was not truncated", len(post))
	}
	if got.Bytes != len(post) {
		t.Fatalf("bytes = %d, want %d", got.Bytes, len(post))
	}
	if len(got.Content) > maxStageContent {
		t.Fatalf("content = %d B, over the %d B window", len(got.Content), maxStageContent)
	}
	if !strings.HasPrefix(post, got.Content) {
		t.Fatal("content is not a prefix of the image")
	}
	sum := sha256.Sum256([]byte(post))
	if got.SHA256 != hex.EncodeToString(sum[:]) {
		t.Fatal("sha256 does not cover the whole image")
	}
}

func TestChangesetMismatchIsCorrectableAndRestageClearsIt(t *testing.T) {
	_, c := newBatch(t, map[string]string{"a.go": "x := 1\ny := 2\n"})
	defer c.Close()
	_, err := stageCall(t, c, "a.go", [2]string{"not here", "x"})
	if err == nil {
		t.Fatal("a missing match staged")
	}
	if err.(*changesetError).class != "mismatch" {
		t.Fatalf("class = %q, want mismatch", err.(*changesetError).class)
	}
	if _, bad := c.Unresolved(); !bad {
		t.Fatal("the target was not marked unresolved")
	}
	// A later successful restage of the same target clears it and is the only
	// thing that does.
	if _, err := stageCall(t, c, "a.go", [2]string{"x := 1", "x := 10"}); err != nil {
		t.Fatal(err)
	}
	if _, bad := c.Unresolved(); bad {
		t.Fatal("a successful restage did not clear the unresolved mark")
	}
	if best, got := c.TargetCount(), 1; best != got {
		t.Fatalf("targets = %d", best)
	}
}

func TestChangesetStageRefusals(t *testing.T) {
	_, c := newBatch(t, map[string]string{"a.go": "x := 1\n"})
	defer c.Close()
	cases := [][2]string{
		{"", "x"},            // empty old
		{"x := 1", "x := 1"}, // no change
	}
	for i, p := range cases {
		if _, err := stageCall(t, c, "a.go", p); err == nil {
			t.Fatalf("case %d staged", i)
		}
	}
	if _, err := c.Stage(context.Background(), []byte(`{"path":"a.go","edits":[]}`)); err == nil {
		t.Fatal("an empty edits array staged")
	}
	// A pair set whose net effect is no change is refused.
	if _, err := stageCall(t, c, "a.go", [2]string{"x := 1", "x := 2"}, [2]string{"x := 2", "x := 1"}); err == nil {
		t.Fatal("a no-op image staged")
	}
}

func TestChangesetDuplicateIdentityRefused(t *testing.T) {
	dir, c := newBatch(t, map[string]string{"a.go": "x := 1\n"})
	defer c.Close()
	if _, err := stageCall(t, c, "a.go", [2]string{"x := 1", "x := 2"}); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(filepath.Join(dir, "a.go"), filepath.Join(dir, "b.go")); err != nil {
		t.Skipf("hard links unsupported: %v", err)
	}
	if _, err := stageCall(t, c, "b.go", [2]string{"x := 1", "x := 3"}); err == nil {
		t.Fatal("the same inode was staged twice")
	}
}

func TestChangesetBudgetTargets(t *testing.T) {
	files := map[string]string{}
	for i := 0; i < maxChangesetTargets; i++ {
		files[filepath.ToSlash(filepath.Join("f", string(rune('a'+i%26))+string(rune('a'+i/26))+".go"))] = "x := 1\n"
	}
	_, c := newBatch(t, files)
	defer c.Close()
	n := 0
	for rel := range files {
		if _, err := stageCall(t, c, rel, [2]string{"x := 1", "x := 2"}); err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
		n++
	}
	if n != maxChangesetTargets {
		t.Fatalf("staged %d", n)
	}
	// One more target, even a successful one, is over the batch limit.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "extra.go"), []byte("x := 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c.rootAbs = dir
	if _, err := stageCall(t, c, "extra.go", [2]string{"x := 1", "x := 2"}); err == nil {
		t.Fatal("the 33rd target staged")
	}
}

func TestChangesetApplyWritesAndReports(t *testing.T) {
	dir, c := newBatch(t, map[string]string{"svc.yaml": "image: api:1.4.1\n"})
	defer c.Close()
	if _, err := stageCall(t, c, "svc.yaml", [2]string{"api:1.4.1", "api:1.5.1"}); err != nil {
		t.Fatal(err)
	}
	if _, bad := c.Unresolved(); bad {
		t.Fatal("unexpected unresolved target")
	}
	snap := &csFakeSnap{}
	journal := &fakeJournal{}
	env := &Env{Root: dir, Snap: snap, Journal: journal}
	status, receipt := c.Apply(context.Background(), env)
	if status != OutcomeApplied {
		t.Fatalf("status = %s (%s)", status, receipt)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "svc.yaml"))
	if string(got) != "image: api:1.5.1\n" {
		t.Fatalf("disk = %q", got)
	}
	if !env.Edited || env.Unverified != "svc.yaml" {
		t.Fatalf("run state not recorded: edited=%v unverified=%q", env.Edited, env.Unverified)
	}
	if snap.writes != 1 || !bytes.Equal(snap.pre, []byte("image: api:1.4.1\n")) || !bytes.Equal(snap.post, got) {
		t.Fatalf("snapshot got pre=%q post=%q writes=%d", snap.pre, snap.post, snap.writes)
	}
	if len(journal.records) != 1 || !journal.finished || journal.records[0] != "svc.yaml" {
		t.Fatalf("journal = %+v", journal)
	}
}

func TestChangesetApplyRefusesWithoutAHumanOrAYolo(t *testing.T) {
	dir, c := newBatch(t, map[string]string{"a.go": "x := 1\n"})
	defer c.Close()
	if _, err := stageCall(t, c, "a.go", [2]string{"x := 1", "x := 2"}); err != nil {
		t.Fatal(err)
	}
	// Headless ask mode has no one to ask: refuse, write nothing, never call Ask.
	called := false
	env := &Env{Root: dir, Snap: &csFakeSnap{}, Journal: &fakeJournal{},
		Ask: func(context.Context, Question) Answer { called = true; return AllowOnce }}
	c.policy = ChangesetPolicy{Edits: true, Trusted: true, Mode: "ask", Headless: true}
	if status, receipt := c.Apply(context.Background(), env); status != OutcomeDiscarded || !strings.Contains(receipt, "-p") {
		t.Fatalf("status=%s receipt=%s", status, receipt)
	}
	if called {
		t.Fatal("a headless session asked")
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "a.go")); string(got) != "x := 1\n" {
		t.Fatal("a refused changeset wrote")
	}

	// An explicit denial applies nothing.
	called = false
	c.policy = ChangesetPolicy{Edits: true, Trusted: true, Mode: "ask"}
	env.Ask = func(context.Context, Question) Answer { called = true; return Deny }
	if status, _ := c.Apply(context.Background(), env); status != OutcomeDiscarded || !called {
		t.Fatalf("denial status = %s, asked = %v", status, called)
	}

	// Untrusted refuses regardless of a surviving user opt-in.
	c.policy = ChangesetPolicy{Edits: true, Trusted: false, Mode: "yolo"}
	if status, _ := c.Apply(context.Background(), env); status != OutcomeDiscarded {
		t.Fatalf("untrusted status = %s", status)
	}
}

func TestChangesetApplyNeedsGuardAndJournal(t *testing.T) {
	dir, c := newBatch(t, map[string]string{"a.go": "x := 1\n"})
	defer c.Close()
	if _, err := stageCall(t, c, "a.go", [2]string{"x := 1", "x := 2"}); err != nil {
		t.Fatal(err)
	}
	// A snapshotter without GuardImage cannot protect the write.
	if status, receipt := c.Apply(context.Background(), &Env{Root: dir, Journal: &fakeJournal{}}); status != OutcomeDiscarded || !strings.Contains(receipt, "image guard") {
		t.Fatalf("status=%s receipt=%s", status, receipt)
	}
	// No journal, no write.
	if status, receipt := c.Apply(context.Background(), &Env{Root: dir, Snap: &csFakeSnap{}}); status != OutcomeDiscarded || !strings.Contains(receipt, "journal") {
		t.Fatalf("status=%s receipt=%s", status, receipt)
	}
	if got, _ := os.ReadFile(filepath.Join(dir, "a.go")); string(got) != "x := 1\n" {
		t.Fatal("a refused changeset wrote")
	}
}

func TestChangesetStaleTargetIsDiscarded(t *testing.T) {
	dir, c := newBatch(t, map[string]string{"a.go": "x := 1\n"})
	defer c.Close()
	if _, err := stageCall(t, c, "a.go", [2]string{"x := 1", "x := 2"}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("x := 99\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	snap, journal := &csFakeSnap{}, &fakeJournal{}
	status, receipt := c.Apply(context.Background(), &Env{Root: dir, Snap: snap, Journal: journal})
	if status != OutcomeDiscarded || !strings.Contains(receipt, "changed") {
		t.Fatalf("status=%s receipt=%s", status, receipt)
	}
	if snap.writes != 0 || len(journal.batches) != 0 {
		t.Fatal("a stale batch reached the snapshot or the journal")
	}
}

func TestChangesetReadBackMismatchIsUnknown(t *testing.T) {
	dir, c := newBatch(t, map[string]string{"a.go": "x := 1\n", "b.go": "y := 1\n"})
	defer c.Close()
	for rel, body := range map[string][2]string{"a.go": {"x := 1", "x := 2"}, "b.go": {"y := 1", "y := 2"}} {
		if _, err := stageCall(t, c, rel, body); err != nil {
			t.Fatal(err)
		}
	}
	snap := &csFakeSnap{corrupt: true}
	status, receipt := c.Apply(context.Background(), &Env{Root: dir, Snap: snap, Journal: &fakeJournal{}})
	if status != OutcomeOutcomeUnknown || !strings.Contains(receipt, "not the image") {
		t.Fatalf("status=%s receipt=%s", status, receipt)
	}
}

func TestChangesetJournalFailures(t *testing.T) {
	dir, c := newBatch(t, map[string]string{"a.go": "x := 1\n"})
	defer c.Close()
	if _, err := stageCall(t, c, "a.go", [2]string{"x := 1", "x := 2"}); err != nil {
		t.Fatal(err)
	}
	// A failed Begin means zero writes.
	snap := &csFakeSnap{}
	status, _ := c.Apply(context.Background(), &Env{Root: dir, Snap: snap, Journal: &fakeJournal{failBegin: true}})
	if status != OutcomeDiscarded || snap.writes != 0 {
		t.Fatalf("begin failure: status=%s writes=%d", status, snap.writes)
	}
	// A failed Record after a landed write is an unknown outcome, not a success.
	snap = &csFakeSnap{}
	status, receipt := c.Apply(context.Background(), &Env{Root: dir, Snap: snap, Journal: &fakeJournal{failRecord: true}})
	if status != OutcomeOutcomeUnknown || snap.writes != 1 {
		t.Fatalf("record failure: status=%s writes=%d (%s)", status, snap.writes, receipt)
	}
}

func TestChangesetReceiptBoundsAndPreview(t *testing.T) {
	dir, c := newBatch(t, map[string]string{"a.go": "x := 1\n"})
	defer c.Close()
	if _, err := stageCall(t, c, "a.go", [2]string{"x := 1", "x := 2"}); err != nil {
		t.Fatal(err)
	}
	prev := c.preview()
	if !strings.Contains(prev, "a.go (1 edit)") || !strings.Contains(prev, "- x := 1") || !strings.Contains(prev, "+ x := 2") {
		t.Fatalf("preview = %q", prev)
	}
	status, receipt := c.Apply(context.Background(), &Env{Root: dir, Snap: &csFakeSnap{}, Journal: &fakeJournal{}})
	if status != OutcomeApplied {
		t.Fatal(status)
	}
	if len(receipt) > maxReceiptBytes {
		t.Fatalf("receipt is %d bytes", len(receipt))
	}
	if !strings.Contains(receipt, "applied: 1 of 1 files written") {
		t.Fatalf("receipt = %q", receipt)
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
