package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The inspection primitives are the code path every codemode read/search/ls
// takes, so the tests here are the red list from the plan: containment,
// budgets, clipping, cancellation, and the cursor chain.

func newInspect(t *testing.T, dir string) *Inspection {
	t.Helper()
	in, err := NewInspection(dir)
	if err != nil {
		t.Fatalf("NewInspection: %v", err)
	}
	t.Cleanup(func() { in.Close() })
	return in
}

func writeFile(t *testing.T, dir, rel, body string) string {
	t.Helper()
	abs := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return abs
}

func lines(n int) string {
	var sb strings.Builder
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&sb, "line %02d\n", i)
	}
	return sb.String()
}

func call[T any](t *testing.T, fn func(context.Context, json.RawMessage) (any, error), args string) (T, error) {
	t.Helper()
	var zero T
	got, err := fn(context.Background(), json.RawMessage(args))
	if err != nil {
		return zero, err
	}
	typed, ok := got.(T)
	if !ok {
		t.Fatalf("payload type %T, want %T", got, zero)
	}
	return typed, nil
}

// ---- containment ---------------------------------------------------------

func TestInspectRefusesOutside(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "inside.txt", "ok\n")
	outside := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(outside, []byte("secret\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A symlink INSIDE the root pointing outside it: os.Root refuses to follow
	// it, which is the property this test exists for — a string check would pass.
	if err := os.Symlink(outside, filepath.Join(dir, "escape.txt")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	in := newInspect(t, dir)

	if _, err := in.Read(context.Background(), json.RawMessage(`{"path":"../secret.txt"}`)); err == nil {
		t.Fatal("../ was accepted")
	}
	if _, err := in.Read(context.Background(), json.RawMessage(`{"path":"/etc/passwd"}`)); err == nil {
		t.Fatal("an absolute path was accepted")
	}
	_, err := in.Read(context.Background(), json.RawMessage(`{"path":"escape.txt"}`))
	if err == nil {
		t.Fatal("a symlink out of the workspace was followed")
	}
	if ie := (*InspectError)(nil); !errors.As(err, &ie) {
		t.Fatalf("want an InspectError, got %T", err)
	}
	if _, err := in.Read(context.Background(), json.RawMessage(`{"path":"inside.txt"}`)); err != nil {
		t.Fatalf("a file inside the root was refused: %v", err)
	}
}

func TestInspectRefusesNonRegular(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	in := newInspect(t, dir)
	if _, err := in.Read(context.Background(), json.RawMessage(`{"path":"sub"}`)); err == nil {
		t.Fatal("a directory was read as a file")
	}
	if _, err := in.Read(context.Background(), json.RawMessage(`{"path":"missing.txt"}`)); err == nil {
		t.Fatal("a missing file was reported as read")
	}
}

// ---- read: window, clipping, escaping ------------------------------------

// A long line in the middle must stop the window, not be skipped over: the
// reported range has to be contiguous (the same rule the direct read now
// follows).
func TestInspectReadStopsAtTheFirstRowThatDoesNotFit(t *testing.T) {
	dir := t.TempDir()
	var sb strings.Builder
	for i := 1; i <= 24; i++ {
		fmt.Fprintf(&sb, "%d|%s\n", i, strings.Repeat("a", 2100))
	}
	sb.WriteString(strings.Repeat("b", 3000) + "\n")
	for i := 26; i <= 30; i++ {
		fmt.Fprintf(&sb, "tail %d\n", i)
	}
	writeFile(t, dir, "wide.txt", sb.String())
	in := newInspect(t, dir)

	p, err := call[readPayload](t, in.Read, `{"path":"wide.txt","offset":1,"limit":30}`)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if p.Complete {
		t.Fatal("a page stopped by the window must not claim complete")
	}
	if len(p.Lines) == 0 {
		t.Fatal("no lines returned")
	}
	for i, ln := range p.Lines {
		if ln.N != i+1 {
			t.Fatalf("line %d out of order: got n=%d", i+1, ln.N)
		}
	}
	if last := p.Lines[len(p.Lines)-1].N; last != len(p.Lines) {
		t.Fatalf("range is not contiguous from offset 1: last n=%d, %d rows", last, len(p.Lines))
	}
	if p.Next == "" {
		t.Fatal("a truncated page must offer a continuation")
	}
}

func TestInspectReadClipsLongLines(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "long.txt", strings.Repeat("x", 5000)+"\nsecond\n")
	in := newInspect(t, dir)
	p, err := call[readPayload](t, in.Read, `{"path":"long.txt"}`)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(p.Lines) != 2 {
		t.Fatalf("lines = %d", len(p.Lines))
	}
	if !p.Lines[0].Clipped {
		t.Fatal("a 5000-char line was not marked clipped")
	}
	if len(p.Lines[0].Text) > readMaxLineLen {
		t.Fatalf("clipped line is %d bytes", len(p.Lines[0].Text))
	}
	if p.Complete {
		t.Fatal("clipping must leave the read incomplete")
	}
	if !strings.Contains(strings.Join(p.Reasons, ","), "line_clipped") {
		t.Fatalf("reasons = %v", p.Reasons)
	}
}

// Payloads are JSON: wide characters, quotes and backslashes must round-trip.
func TestInspectReadEscapesJSON(t *testing.T) {
	dir := t.TempDir()
	body := "quote \" backslash \\ tab \t cjk 日本語 emoji 🙂\n"
	writeFile(t, dir, "wide.txt", body)
	in := newInspect(t, dir)
	p, err := call[readPayload](t, in.Read, `{"path":"wide.txt"}`)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(p.Lines) != 1 {
		t.Fatalf("lines = %d", len(p.Lines))
	}
	if p.Lines[0].Text != strings.TrimSuffix(body, "\n") {
		t.Fatalf("text round trip = %q", p.Lines[0].Text)
	}
	enc, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("payload is not JSON-encodable: %v", err)
	}
	if !json.Valid(enc) {
		t.Fatal("encoded payload is not valid JSON")
	}
}

// ---- read: paging and the cursor chain -----------------------------------

func TestInspectPagingContinues(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "f.txt", lines(10))
	in := newInspect(t, dir)

	p1, err := call[readPayload](t, in.Read, `{"path":"f.txt","offset":3,"limit":3}`)
	if err != nil {
		t.Fatalf("page 1: %v", err)
	}
	if len(p1.Lines) != 3 || p1.Lines[0].N != 3 || p1.Next == "" {
		t.Fatalf("page 1 = %+v", p1)
	}
	p2, err := call[readPayload](t, in.Read, fmt.Sprintf(`{"next":%q}`, p1.Next))
	if err != nil {
		t.Fatalf("page 2: %v", err)
	}
	if len(p2.Lines) != 3 || p2.Lines[0].N != 6 {
		t.Fatalf("page 2 = %+v", p2)
	}
	if p2.Complete {
		t.Fatal("page 2 of 10 lines must not claim complete")
	}
	p3, err := call[readPayload](t, in.Read, fmt.Sprintf(`{"next":%q}`, p2.Next))
	if err != nil {
		t.Fatalf("page 3: %v", err)
	}
	if !p3.Complete || len(p3.Lines) != 2 || p3.Lines[0].N != 9 {
		t.Fatalf("page 3 = %+v", p3)
	}
	if p3.Next != "" {
		t.Fatal("a complete page must not offer a cursor")
	}
}

// The cursor is sealed with the chain's start: a chain that began at offset 3
// must verify lines 3..end, not 1..end.
func TestInspectCursorRefusesFreshArgs(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "f.txt", lines(10))
	in := newInspect(t, dir)
	p1, err := call[readPayload](t, in.Read, `{"path":"f.txt","offset":2,"limit":2}`)
	if err != nil {
		t.Fatalf("page 1: %v", err)
	}
	for _, bad := range []string{
		fmt.Sprintf(`{"next":%q,"path":"f.txt"}`, p1.Next),
		fmt.Sprintf(`{"next":%q,"offset":3}`, p1.Next),
		fmt.Sprintf(`{"next":%q,"limit":9}`, p1.Next),
	} {
		_, err := in.Read(context.Background(), json.RawMessage(bad))
		var ie *InspectError
		if !errors.As(err, &ie) || ie.Class != "invalid_cursor_args" {
			t.Fatalf("%s => %v, want invalid_cursor_args", bad, err)
		}
	}
}

// Changing a line AFTER page two was accepted must invalidate the chain that
// described page one — that is the whole point of the digest.
func TestInspectCursorDetectsChangedPageOne(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, "f.txt", lines(12))
	in := newInspect(t, dir)

	p1, err := call[readPayload](t, in.Read, `{"path":"f.txt","limit":3}`)
	if err != nil {
		t.Fatalf("page 1: %v", err)
	}
	p2, err := call[readPayload](t, in.Read, fmt.Sprintf(`{"next":%q}`, p1.Next))
	if err != nil {
		t.Fatalf("page 2: %v", err)
	}
	// Mutate page one, leaving the file the same length so a size check alone
	// would accept it.
	body := "LINE 01\n" + lines(12)[len("line 01\n"):]
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = in.Read(context.Background(), json.RawMessage(fmt.Sprintf(`{"next":%q}`, p2.Next)))
	var ie *InspectError
	if !errors.As(err, &ie) || ie.Class != "changed" {
		t.Fatalf("page 3 = %v, want changed", err)
	}
	// The chain is gone: the same cursor must not work twice.
	if _, err := in.Read(context.Background(), json.RawMessage(fmt.Sprintf(`{"next":%q}`, p2.Next))); err == nil {
		t.Fatal("an invalidated cursor was accepted again")
	}
}

// ---- budgets -------------------------------------------------------------

func TestInspectAttemptsLatch(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "f.txt", "one\n")
	in := newInspect(t, dir)

	for k := 0; k < inspectAttempts; k++ {
		if _, err := in.Read(context.Background(), json.RawMessage(`{"path":"f.txt"}`)); err != nil {
			t.Fatalf("attempt %d: %v", k+1, err)
		}
	}
	_, err := in.Read(context.Background(), json.RawMessage(`{"path":"f.txt"}`))
	var ie *InspectError
	if !errors.As(err, &ie) || ie.Class != "limit" {
		t.Fatalf("attempt 51 = %v, want limit", err)
	}
	// Latched: the 52nd fails the same way rather than getting one more chance.
	if _, err := in.Read(context.Background(), json.RawMessage(`{"path":"f.txt"}`)); err == nil {
		t.Fatal("the attempt cap did not latch")
	}
}

func TestInspectSourceBudget(t *testing.T) {
	dir := t.TempDir()
	// Two files just under the budget each: the second read must stop at the cap
	// rather than reading 512 MiB.
	half := (inspectSourceBytes / 2) + 1024
	writeFile(t, dir, "big1.txt", strings.Repeat("a", int(half)))
	writeFile(t, dir, "big2.txt", strings.Repeat("b", int(half)))
	in := newInspect(t, dir)
	if _, err := in.Read(context.Background(), json.RawMessage(`{"path":"big1.txt","limit":5}`)); err != nil {
		t.Fatalf("first read: %v", err)
	}
	_, err := in.Read(context.Background(), json.RawMessage(`{"path":"big2.txt","limit":5}`))
	var ie *InspectError
	if !errors.As(err, &ie) || ie.Class != "limit" {
		t.Fatalf("second read = %v, want limit", err)
	}
}

// ---- ls ------------------------------------------------------------------

func TestInspectLS(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.txt", "x")
	writeFile(t, dir, "sub/b.txt", "y")
	in := newInspect(t, dir)

	p, err := call[lsPayload](t, in.LS, `{"path":"."}`)
	if err != nil {
		t.Fatalf("ls: %v", err)
	}
	if !p.Complete || len(p.Entries) != 2 {
		t.Fatalf("entries = %+v", p.Entries)
	}
	kinds := map[string]string{}
	for _, e := range p.Entries {
		kinds[e.Name] = e.Kind
	}
	if kinds["a.txt"] != "file" || kinds["sub"] != "dir" {
		t.Fatalf("kinds = %v", kinds)
	}
	if _, err := in.LS(context.Background(), json.RawMessage(`{"path":"a.txt"}`)); err == nil {
		t.Fatal("ls of a file was accepted")
	}
}

func TestInspectLSCapsPerCall(t *testing.T) {
	dir := t.TempDir()
	for k := 0; k < inspectLSEntries+5; k++ {
		writeFile(t, dir, fmt.Sprintf("f%04d.txt", k), "x")
	}
	in := newInspect(t, dir)
	p, err := call[lsPayload](t, in.LS, `{"path":"."}`)
	if err != nil {
		t.Fatalf("ls: %v", err)
	}
	if len(p.Entries) != inspectLSEntries {
		t.Fatalf("entries = %d, want %d", len(p.Entries), inspectLSEntries)
	}
	if p.Complete {
		t.Fatal("a capped listing must not claim complete")
	}
	if !strings.Contains(strings.Join(p.Reasons, ","), "entry_budget") {
		t.Fatalf("reasons = %v", p.Reasons)
	}
}

// ---- search --------------------------------------------------------------

func TestInspectSearchFindsAndSkips(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "src/a.go", "package a\nfunc Needle() {}\n")
	writeFile(t, dir, "src/ignored.log", "needle\n")
	writeFile(t, dir, ".hidden/secret.txt", "needle\n")
	writeFile(t, dir, ".gitignore", "*.log\n")
	in := newInspect(t, dir)

	p, err := call[searchPayload](t, in.Search, `{"pattern":"needle"}`)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(p.Matches) != 1 || p.Matches[0].Path != "src/a.go" || p.Matches[0].Line != 2 {
		t.Fatalf("matches = %+v complete=%v reasons=%v", p.Matches, p.Complete, p.Reasons)
	}
	if !p.Complete {
		t.Fatalf("complete = false, reasons %v", p.Reasons)
	}
}

func TestInspectSearchCapsHits(t *testing.T) {
	dir := t.TempDir()
	var sb strings.Builder
	for k := 0; k < inspectSearchHits+20; k++ {
		sb.WriteString("needle here\n")
	}
	writeFile(t, dir, "many.txt", sb.String())
	in := newInspect(t, dir)
	p, err := call[searchPayload](t, in.Search, `{"pattern":"needle"}`)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(p.Matches) != inspectSearchHits {
		t.Fatalf("matches = %d, want %d", len(p.Matches), inspectSearchHits)
	}
	if p.Complete {
		t.Fatal("a capped search must not claim complete")
	}
}

func TestInspectSearchSkipsOversizedIgnore(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "keep.txt", "needle\n")
	writeFile(t, dir, ".gitignore", strings.Repeat("*.log\n", (inspectIgnoreMax/6)+10))
	in := newInspect(t, dir)
	p, err := call[searchPayload](t, in.Search, `{"pattern":"needle"}`)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if !strings.Contains(strings.Join(p.Reasons, ","), "ignore_too_large") {
		t.Fatalf("reasons = %v", p.Reasons)
	}
	if len(p.Matches) != 1 {
		t.Fatalf("matches = %d, want the file to still be searched", len(p.Matches))
	}
}

func TestInspectSearchCancelled(t *testing.T) {
	dir := t.TempDir()
	for k := 0; k < 200; k++ {
		writeFile(t, dir, fmt.Sprintf("d%03d/f.txt", k), "needle\n")
	}
	in := newInspect(t, dir)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancelled before the walk starts
	_, err := in.Search(ctx, json.RawMessage(`{"pattern":"needle"}`))
	var ie *InspectError
	if !errors.As(err, &ie) || ie.Class != "cancelled" {
		t.Fatalf("search = %v, want cancelled", err)
	}
}

// ---- the direct read fix -------------------------------------------------

// The window must stop at the first row that does not fit and report the
// contiguous range; before the fix a later, shorter line was printed instead.
func TestDirectReadWindowIsContiguous(t *testing.T) {
	dir := t.TempDir()
	var sb strings.Builder
	for i := 1; i <= 24; i++ {
		fmt.Fprintf(&sb, "%s\n", strings.Repeat("a", 2100))
	}
	sb.WriteString(strings.Repeat("b", 5000) + "\n")
	for i := 26; i <= 30; i++ {
		fmt.Fprintf(&sb, "tail %d\n", i)
	}
	path := writeFile(t, dir, "wide.txt", sb.String())

	res := readTool{}.Run(context.Background(), &Env{Root: dir, Paths: rootChecker{dir}, Reads: NewReadTracker()}, json.RawMessage(fmt.Sprintf(`{"path":%q}`, path)))
	if res.IsError {
		t.Fatalf("read failed: %s", res.Content)
	}
	if strings.Contains(res.Content, "tail 26") {
		t.Fatal("the read printed a line after the one that did not fit")
	}
	if !strings.Contains(res.Content, "stopped at the") {
		t.Fatalf("footer does not name the window: %q", res.Content[len(res.Content)-120:])
	}
}

// The root is a per-execution resource, opened on demand: a refusal-only
// inspection (no engine) must be constructible without touching the disk, and a
// script that never inspects must not pay for a root.
func TestInspectionOpensTheRootLazily(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.txt", "x\n")
	in, err := NewInspection(dir)
	if err != nil {
		t.Fatal(err)
	}
	if in.RootOpened() {
		t.Fatal("NewInspection opened the root")
	}
	if err := in.Close(); err != nil {
		t.Fatalf("Close on an unopened inspection: %v", err)
	}
	if _, err := in.Read(context.Background(), json.RawMessage(`{"path":"a.txt"}`)); err != nil {
		t.Fatalf("read after Close: %v", err)
	}
	if !in.RootOpened() {
		t.Fatal("a read did not open the root")
	}
	in.Close()
}

// The nested path shares parseIgnore/ignored with the direct search, so the
// cases the hand-rolled subset got wrong must now behave: a negation restores a
// file, and a nested ignore file overrides its parent.
func TestInspectSearchHonoursNegationAndNestedIgnores(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".gitignore", "*.log\n!keep.log\n")
	writeFile(t, dir, "a.log", "needle\n")
	writeFile(t, dir, "keep.log", "needle\n")
	writeFile(t, dir, "sub/.gitignore", "secret.txt\n")
	writeFile(t, dir, "sub/secret.txt", "needle\n")
	writeFile(t, dir, "sub/open.txt", "needle\n")
	in := newInspect(t, dir)

	p, err := call[searchPayload](t, in.Search, `{"pattern":"needle"}`)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	got := map[string]bool{}
	for _, m := range p.Matches {
		got[m.Path] = true
	}
	if got["a.log"] {
		t.Fatal("an ignored file was searched")
	}
	if !got["keep.log"] {
		t.Fatal("a negated rule did not restore the file")
	}
	if got["sub/secret.txt"] {
		t.Fatal("a nested .gitignore was not applied")
	}
	if !got["sub/open.txt"] {
		t.Fatal("a nested ignore file hid an unrelated file")
	}
}

// A directive directory rule still prunes the walk.
func TestInspectSearchSkipsIgnoredDirectories(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".gitignore", "build/\n")
	writeFile(t, dir, "build/gen.txt", "needle\n")
	writeFile(t, dir, "src/real.txt", "needle\n")
	in := newInspect(t, dir)
	p, err := call[searchPayload](t, in.Search, `{"pattern":"needle"}`)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	for _, m := range p.Matches {
		if strings.HasPrefix(m.Path, "build/") {
			t.Fatalf("an ignored directory was searched: %s", m.Path)
		}
	}
	if len(p.Matches) != 1 {
		t.Fatalf("matches = %+v", p.Matches)
	}
}
