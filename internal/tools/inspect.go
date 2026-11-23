package tools

// Nested inspection: the primitive behind `tools.read`, `tools.search` and
// `tools.ls` inside a codemode script. It is NOT a wrapper over the direct
// tools: it runs no session bookkeeping (no Reads.Record, no TestSeen, no
// FailingTest), it never spills, and it returns typed payloads plus `complete`
// and `reasons` instead of prose. What it shares with them is the rule that
// matters: stop at the first row that does not fit.
//
// One Inspection belongs to one execution. The attempt count, the byte budget,
// the entry budget and the cursors are execution-local by construction, so a
// script cannot spend another script's budget or reuse another's cursor.

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	inspectAttempts     = 50        // per script, latching
	inspectSourceBytes  = 256 << 20 // total source bytes read, including re-reads
	inspectEntries      = 20_000    // directory entries across calls
	inspectLSEntries    = 1_000     // per ls call
	inspectSearchHits   = 100       // per search call
	inspectGlobPaths    = 1_000     // per glob call
	inspectSearchLine   = 300       // chars per match
	inspectIgnoreMax    = 64 << 10  // a larger ignore file is skipped, with a reason
	inspectReasonSample = 5         // examples kept in coverage
	inspectCursorMax    = 64        // live cursors per execution
)

// InspectError is a typed inspection failure. Class is the class the adapter
// turns into a codemode.CallError; nothing here knows about the engine.
type InspectError struct {
	Class   string
	Message string
}

func (e *InspectError) Error() string { return e.Class + ": " + e.Message }

func inspectErr(class, format string, a ...any) *InspectError {
	return &InspectError{Class: class, Message: fmt.Sprintf(format, a...)}
}

// Inspection is one script's view of the workspace. The root is opened on first
// use, not at construction: the refusal-only factory must be able to build one
// without touching the filesystem, and a script that never inspects anything
// should cost nothing.
type Inspection struct {
	dir      string
	ws       *workspace
	attempts int
	latched  bool
	bytes    int64
	entries  int
	cursors  map[string]cursor
	nextID   int
	reasonN  int      // gaps over the whole execution, for the host ledger
	callGaps int      // gaps during the current call
	sample   []string // ≤inspectReasonSample examples from the current call
}

// cursor is the opaque continuation state. start is the chain's ORIGINAL
// offset and never changes: without it a chain that began at a non-default
// offset could not re-read the span its digest claims to verify (F19).
type cursor struct {
	path   string
	start  int
	next   int
	limit  int
	digest string
}

// NewInspection records the execution's containment boundary. It opens nothing.
func NewInspection(dir string) (*Inspection, error) {
	if dir == "" {
		return nil, &InspectError{Class: "denied", Message: "no workspace root"}
	}
	return &Inspection{dir: dir, cursors: map[string]cursor{}}, nil
}

// openRoot opens the root once per execution. A single script is single
// threaded by contract (one slot), so no lock is needed.
func (i *Inspection) openRoot() (*workspace, error) {
	if i.ws != nil {
		return i.ws, nil
	}
	w, err := openWorkspace(i.dir)
	if err != nil {
		return nil, err
	}
	i.ws = w
	return w, nil
}

// RootOpened reports whether this execution has actually touched the
// filesystem — the property the refusal-only factory is tested against.
func (i *Inspection) RootOpened() bool { return i.ws != nil }

// Close releases the root; it is safe on an Inspection that never opened one.
func (i *Inspection) Close() error {
	if i.ws == nil {
		return nil
	}
	err := i.ws.close()
	i.ws = nil
	return err
}

// ---- budgets and coverage ------------------------------------------------

// charge takes one attempt. The 51st latches: every later call fails the same
// way, so a script cannot spend its way past the cap one error at a time.
func (i *Inspection) charge() error {
	if i.latched {
		return inspectErr("limit", "inspection attempts exhausted (%d)", inspectAttempts)
	}
	if i.attempts >= inspectAttempts {
		i.latched = true
		i.gap("attempts")
		return inspectErr("limit", "inspection attempts exhausted (%d)", inspectAttempts)
	}
	i.attempts++
	return nil
}

// chargeBytes accounts for source bytes read; the budget covers re-reads too,
// which is why a long cursor chain ends incomplete rather than overrunning.
func (i *Inspection) chargeBytes(n int) error {
	i.bytes += int64(n)
	if i.bytes > inspectSourceBytes {
		i.gap("source_budget")
		return inspectErr("limit", "source budget exhausted (%d MiB)", inspectSourceBytes>>20)
	}
	return nil
}

// begin starts one call's coverage. Reasons are per payload; the execution-wide
// count stays in reasonN for the host ledger.
func (i *Inspection) begin() {
	i.callGaps = 0
	i.sample = i.sample[:0]
}

// gap records an incompleteness reason, keeping a bounded number of examples.
func (i *Inspection) gap(reason string) {
	i.reasonN++
	i.callGaps++
	if len(i.sample) < inspectReasonSample {
		i.sample = append(i.sample, reason)
	}
}

// coverage returns this call's reasons, with a count when truncated.
func (i *Inspection) coverage() []string {
	out := append([]string(nil), i.sample...)
	if n := i.callGaps - len(out); n > 0 {
		out = append(out, fmt.Sprintf("+%d more", n))
	}
	return out
}

// ---- payloads ------------------------------------------------------------

type readRow struct {
	N       int    `json:"n"`
	Text    string `json:"text"`
	Clipped bool   `json:"clipped,omitempty"`
}

type readPayload struct {
	Lines    []readRow `json:"lines"`
	Complete bool      `json:"complete"`
	Reasons  []string  `json:"reasons"`
	Next     string    `json:"next,omitempty"`
}

type searchMatch struct {
	Path    string `json:"path"`
	Line    int    `json:"line"`
	Text    string `json:"text"`
	Clipped bool   `json:"clipped,omitempty"`
}

type searchPayload struct {
	Matches  []searchMatch `json:"matches"`
	Complete bool          `json:"complete"`
	Reasons  []string      `json:"reasons"`
}

type lsEntry struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
}

type globPayload struct {
	Paths    []string `json:"paths"`
	Complete bool     `json:"complete"`
	Reasons  []string `json:"reasons"`
}

type lsPayload struct {
	Entries  []lsEntry `json:"entries"`
	Complete bool      `json:"complete"`
	Reasons  []string  `json:"reasons"`
}

// ---- read ----------------------------------------------------------------

// Read answers `tools.read`. A fresh read starts at offset (default 1); a
// continuation passes the cursor name and nothing else. The two forms are
// mutually exclusive, because a cursor that could be combined with a new path
// would describe a chain it never verified (S2).
func (i *Inspection) Read(ctx context.Context, args json.RawMessage) (any, error) {
	var a struct {
		Path   string  `json:"path"`
		Offset *int    `json:"offset"`
		Limit  *int    `json:"limit"`
		Next   *string `json:"next"`
	}
	if r := decodeArgs(args, &a); r != nil {
		return nil, r
	}
	i.begin()
	i.begin()
	i.begin()
	if err := i.charge(); err != nil {
		return nil, err
	}

	var (
		c        cursor
		pageFrom int
		verify   bool
		limit    int
	)
	if a.Next != nil {
		if a.Path != "" || a.Offset != nil || a.Limit != nil {
			return nil, inspectErr("invalid_cursor_args", "next is exclusive with path, offset and limit")
		}
		prev, ok := i.cursors[*a.Next]
		if !ok {
			return nil, inspectErr("invalid_cursor_args", "unknown or already closed cursor %q", *a.Next)
		}
		c, verify, pageFrom = prev, true, prev.next
		limit = prev.limit
		if limit < 1 {
			limit = readDefaultLines
		}
	} else {
		offset := 1
		if a.Offset != nil {
			offset = *a.Offset
		}
		if offset < 1 {
			return nil, inspectErr("invalid_args", "offset must be >= 1")
		}
		limit = readDefaultLines
		if a.Limit != nil {
			limit = min(*a.Limit, readMaxLines)
		}
		if limit < 1 {
			return nil, inspectErr("invalid_args", "limit must be >= 1")
		}
		rel, err := cleanRel(a.Path)
		if err != nil {
			return nil, err
		}
		c = cursor{path: rel, start: offset, next: offset, limit: limit}
		pageFrom = offset
	}

	ws, err := i.openRoot()
	if err != nil {
		return nil, err
	}
	f, fi, err := ws.openFile(c.path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if fi.Size() > readMaxFileSize {
		return nil, inspectErr("limit", "%s is %d bytes; too large to inspect (limit %d MiB)", c.path, fi.Size(), readMaxFileSize>>20)
	}

	payload := readPayload{Lines: []readRow{}}
	pageEnd := pageFrom + limit
	var (
		snapshot string
		clipped  bool
		window   bool
		chars    int
		total    int
	)
	// The running digest is always built: even a first page seals the span it
	// returned, which is what lets page two verify page one.
	h := sha256.New()

	br := newLineReader(f)
	for {
		if err := ctx.Err(); err != nil {
			i.gap("cancelled")
			return nil, inspectErr("cancelled", "inspection cancelled")
		}
		line, raw, more, rerr := br.next(readMaxLineLen)
		if rerr != nil {
			if rerr != io.EOF {
				return nil, inspectErr("denied", "%s", rerr.Error())
			}
			break
		}
		if err := i.chargeBytes(len(raw)); err != nil {
			return nil, err
		}
		total++
		if total < c.start {
			continue
		}
		// The running digest covers the chain's start through the page just
		// returned; the prefix ending at the previous page's last line is the
		// witness a continuation compares against.
		if h != nil && total < pageFrom {
			h.Write(raw)
			if total == pageFrom-1 {
				snapshot = hex.EncodeToString(h.Sum(nil))
			}
		}
		if total >= pageFrom && total < pageEnd && !window {
			if more {
				clipped = true
			}
			if chars+len(line) > readMaxChars && total > pageFrom {
				window = true
				i.gap("char_window")
			} else {
				payload.Lines = append(payload.Lines, readRow{N: total, Text: line, Clipped: more})
				chars += len(line)
				if h != nil {
					h.Write(raw)
				}
			}
		}
	}

	if verify && snapshot != c.digest {
		// Mixed versions are worse than a gap: the chain is invalidated and
		// nothing claims a snapshot it did not verify.
		delete(i.cursors, *a.Next)
		i.gap("changed")
		return nil, inspectErr("changed", "%s changed since the previous page; the chain is invalidated", c.path)
	}
	if total == 0 && c.start == 1 {
		payload.Complete = true
		payload.Reasons = i.coverage()
		return payload, nil
	}
	if total < c.start {
		return nil, inspectErr("invalid_args", "offset %d is past the end (%d lines)", c.start, total)
	}
	if clipped {
		i.gap("line_clipped")
	}
	if pageEnd <= total && !window {
		i.gap("page_limit")
	}
	payload.Complete = total < pageEnd && !clipped && !window

	if !payload.Complete && len(payload.Lines) > 0 {
		last := payload.Lines[len(payload.Lines)-1].N
		name, err := i.storeCursor(cursor{path: c.path, start: c.start, next: last + 1, limit: limit, digest: digestTo(h, c.start, last)})
		if err != nil {
			return nil, err
		}
		payload.Next = name
	}
	payload.Reasons = i.coverage()
	return payload, nil
}

// digestTo seals a chain: the digest of the span [start, last], which the next
// continuation re-reads and compares before returning anything.
func digestTo(h hash.Hash, start, last int) string {
	if h == nil || last < start {
		return ""
	}
	return hex.EncodeToString(h.Sum(nil))
}

// storeCursor gives the cursor an opaque name and keeps the live set bounded.
func (i *Inspection) storeCursor(c cursor) (string, error) {
	if c.digest == "" {
		return "", nil
	}
	if len(i.cursors) >= inspectCursorMax {
		return "", inspectErr("limit", "too many open cursors (%d)", inspectCursorMax)
	}
	i.nextID++
	name := fmt.Sprintf("c%d", i.nextID)
	i.cursors[name] = c
	return name, nil
}

// ---- ls ------------------------------------------------------------------

// LS answers `tools.ls`: one directory, names and kinds, bounded per call and
// across the execution.
func (i *Inspection) LS(ctx context.Context, args json.RawMessage) (any, error) {
	var a struct {
		Path string `json:"path"`
	}
	if r := decodeArgs(args, &a); r != nil {
		return nil, r
	}
	if err := i.charge(); err != nil {
		return nil, err
	}
	rel, err := cleanRel(a.Path)
	if err != nil {
		return nil, err
	}
	payload := lsPayload{Entries: []lsEntry{}}
	ws, err := i.openRoot()
	if err != nil {
		return nil, err
	}
	dir, oerr := ws.root.Open(rel)
	if oerr != nil {
		return nil, inspectErr("not_found", "no such directory: %s", rel)
	}
	defer dir.Close()
	if fi, serr := dir.Stat(); serr != nil || !fi.IsDir() {
		return nil, inspectErr("not_regular", "%s is not a directory", rel)
	}
	for {
		batch, rerr := dir.ReadDir(128)
		for _, d := range batch {
			if i.entries >= inspectEntries || len(payload.Entries) >= inspectLSEntries {
				i.gap("entry_budget")
				payload.Complete = false
				payload.Reasons = i.coverage()
				return payload, nil
			}
			i.entries++
			payload.Entries = append(payload.Entries, lsEntry{Name: d.Name(), Kind: kindOf(d)})
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return nil, inspectErr("denied", "%s", rerr.Error())
		}
		select {
		case <-ctx.Done():
			i.gap("cancelled")
			return nil, inspectErr("cancelled", "inspection cancelled")
		default:
		}
	}
	payload.Complete = true
	payload.Reasons = i.coverage()
	return payload, nil
}

func kindOf(d os.DirEntry) string {
	switch {
	case d.IsDir():
		return "dir"
	case d.Type().IsRegular():
		return "file"
	default:
		return "other"
	}
}

// ---- glob ----------------------------------------------------------------

// Glob answers `tools.glob`: every path under a directory whose path relative to
// that directory matches a pattern, with the same walk, ignore rules and entry
// budget as search.
//
// It exists because "loop over the files" is impossible to write without their
// names: `ls` answers one directory, so a script that needs twelve paths it
// cannot enumerate has to call the tool twelve times, which is the REPL pattern
// the pilot measured (Task 7 rounds 1-3: 3-5 scripts per task, 9.0 steps
// against 6.3). Naming the paths is the missing half of the idiom.
func (i *Inspection) Glob(ctx context.Context, args json.RawMessage) (any, error) {
	var a struct {
		Pattern string `json:"pattern"`
		Path    string `json:"path"`
	}
	if r := decodeArgs(args, &a); r != nil {
		return nil, r
	}
	pat := strings.TrimSpace(a.Pattern)
	if pat == "" {
		return nil, inspectErr("invalid_args", "pattern is required")
	}
	if strings.HasPrefix(pat, "/") || strings.Contains(pat, "..") {
		return nil, inspectErr("invalid_args", "pattern must be relative and must not contain ..")
	}
	// A pattern is model-authored: cap its segments so `**` backtracking stays
	// bounded (the walk is already capped by the entry budget).
	if len(strings.Split(pat, "/")) > 16 {
		return nil, inspectErr("invalid_args", "pattern has too many segments (max 16)")
	}
	if err := i.charge(); err != nil {
		return nil, err
	}
	root := "."
	if a.Path != "" {
		var err error
		if root, err = cleanRel(a.Path); err != nil {
			return nil, err
		}
	}
	ws, err := i.openRoot()
	if err != nil {
		return nil, err
	}
	segments := strings.Split(pat, "/")
	rules := i.loadAncestorIgnores(ws, root)
	payload := globPayload{Paths: []string{}}
	err = ws.walk(ctx, root, func(rel string, d os.DirEntry) bool {
		return skipEntry(d) || ignored(rules, rel, d.IsDir())
	}, func(rel string, d os.DirEntry) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if skipEntry(d) || ignored(rules, rel, d.IsDir()) {
			return nil
		}
		if d.IsDir() {
			// This directory's own ignore file applies to its children.
			i.loadIgnoresInto(ws, &rules, rel)
		}
		if i.entries >= inspectEntries || len(payload.Paths) >= inspectGlobPaths {
			i.gap("entry_budget")
			return errGlobDone
		}
		i.entries++
		// The pattern is relative to the searched directory, so the comparison
		// drops its prefix: a glob of "services/*/service.yaml" from "." and the
		// same pattern from "services" (where it is "* /service.yaml") both name
		// files the model can read.
		relPath := rel
		if root != "." {
			relPath = strings.TrimPrefix(rel, root+"/")
			if relPath == root {
				return nil
			}
		}
		if matchGlob(segments, strings.Split(relPath, "/")) {
			payload.Paths = append(payload.Paths, rel)
		}
		return nil
	})
	if err != nil && !errors.Is(err, errGlobDone) {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			i.gap("cancelled")
			return nil, inspectErr("cancelled", "inspection cancelled")
		}
		return nil, err
	}
	payload.Complete = !errors.Is(err, errGlobDone)
	payload.Reasons = i.coverage()
	return payload, nil
}

var errGlobDone = errors.New("glob done")

// matchGlob matches path segments against pattern segments: `**` spans any
// number of segments (including none), every other segment uses path.Match, so
// `*`, `?` and `[a-z]` behave as they do everywhere else in the tool.
func matchGlob(pat, path []string) bool {
	if len(pat) == 0 {
		return len(path) == 0
	}
	if pat[0] == "**" {
		for i := 0; i <= len(path); i++ {
			if matchGlob(pat[1:], path[i:]) {
				return true
			}
		}
		return false
	}
	if len(path) == 0 {
		return false
	}
	ok, err := filepath.Match(pat[0], path[0])
	if err != nil || !ok {
		return false
	}
	return matchGlob(pat[1:], path[1:])
}

// ---- search --------------------------------------------------------------

// Search answers `tools.search`: a case-insensitive substring match over the
// tree, skipping hidden entries, ignore-listed paths and oversized or binary
// files, with every skip recorded as a reason rather than silently dropped.
func (i *Inspection) Search(ctx context.Context, args json.RawMessage) (any, error) {
	var a struct {
		Pattern string `json:"pattern"`
		Path    string `json:"path"`
	}
	if r := decodeArgs(args, &a); r != nil {
		return nil, r
	}
	if strings.TrimSpace(a.Pattern) == "" {
		return nil, inspectErr("invalid_args", "pattern is required")
	}
	if err := i.charge(); err != nil {
		return nil, err
	}
	root := "."
	if a.Path != "" {
		var err error
		if root, err = cleanRel(a.Path); err != nil {
			return nil, err
		}
	}
	ws, err := i.openRoot()
	if err != nil {
		return nil, err
	}
	needle := strings.ToLower(a.Pattern)
	rules := i.loadAncestorIgnores(ws, root)
	payload := searchPayload{Matches: []searchMatch{}}
	err = ws.walk(ctx, root, func(rel string, d os.DirEntry) bool {
		return skipEntry(d) || ignored(rules, rel, d.IsDir())
	}, func(rel string, d os.DirEntry) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if skipEntry(d) || ignored(rules, rel, d.IsDir()) {
			return nil
		}
		if d.IsDir() {
			// This directory's own ignore file applies to its children.
			i.loadIgnoresInto(ws, &rules, rel)
			return nil
		}
		if len(payload.Matches) >= inspectSearchHits {
			i.gap("hit_budget")
			return errSearchDone
		}
		if !d.Type().IsRegular() {
			i.gap("not_regular")
			return nil
		}
		info, err := d.Info()
		if err != nil {
			i.gap("unreadable")
			return nil
		}
		if info.Size() > searchMaxFileSize {
			i.gap("file_too_large")
			return nil
		}
		f, _, err := ws.openFile(rel)
		if err != nil {
			i.gap("unreadable")
			return nil
		}
		defer f.Close()
		if err := i.scanFile(ctx, f, rel, needle, &payload); err != nil {
			if errors.Is(err, errStop) {
				return errSearchDone
			}
			return err
		}
		return nil
	})
	if err != nil && !errors.Is(err, errSearchDone) {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			i.gap("cancelled")
			return nil, inspectErr("cancelled", "inspection cancelled")
		}
		return nil, err
	}
	payload.Complete = !errors.Is(err, errSearchDone)
	payload.Reasons = i.coverage()
	return payload, nil
}

var (
	errSearchDone = errors.New("search done")
	errStop       = errors.New("stop")
)

// scanFile matches one file line by line, charging every byte it reads.
func (i *Inspection) scanFile(ctx context.Context, f *os.File, rel, needle string, out *searchPayload) error {
	br := newLineReader(f)
	for {
		if err := ctx.Err(); err != nil {
			return ctx.Err()
		}
		if len(out.Matches) >= inspectSearchHits {
			i.gap("hit_budget")
			return errStop
		}
		line, raw, more, err := br.next(inspectSearchLine)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return inspectErr("denied", "%s", err.Error())
		}
		if cerr := i.chargeBytes(len(raw)); cerr != nil {
			return cerr
		}
		if strings.Contains(strings.ToLower(line), needle) {
			out.Matches = append(out.Matches, searchMatch{Path: rel, Line: br.n, Text: line, Clipped: more})
			if more {
				i.gap("line_clipped")
			}
		}
	}
}

// skipEntry is the traversal filter: hidden names, VCS metadata and ignore
// files themselves are never searched.
func skipEntry(d os.DirEntry) bool {
	name := d.Name()
	if name == ".git" || name == "node_modules" || strings.HasPrefix(name, ".") {
		return true
	}
	return false
}

// ---- args decoding -------------------------------------------------------

// decodeArgs turns a raw object into a typed struct, refusing anything else.
func decodeArgs(raw json.RawMessage, v any) *InspectError {
	if len(raw) == 0 {
		raw = json.RawMessage("{}")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return inspectErr("invalid_args", "bad arguments: %v", err)
	}
	return nil
}

// ---- the line reader ------------------------------------------------------

// lineReader walks a file line by line and keeps each line's RAW bytes, so a
// cursor digest can be computed over exactly the bytes a later continuation
// re-reads. It applies the same clipping rule as the direct read.
type lineReader struct {
	br  *bufio.Reader
	n   int
	raw []byte
}

func newLineReader(f io.Reader) *lineReader {
	return &lineReader{br: bufio.NewReaderSize(f, 64<<10)}
}

// next returns the next line, its raw bytes (newline included) and whether it
// was clipped. io.EOF is returned only when nothing remained.
func (r *lineReader) next(max int) (string, []byte, bool, error) {
	var sb strings.Builder
	r.raw = r.raw[:0]
	more := false
	for {
		b, err := r.br.ReadByte()
		if err != nil {
			if len(r.raw) == 0 {
				return "", nil, false, err
			}
			r.n++
			return strings.TrimSuffix(sb.String(), "\r"), r.raw, more, nil
		}
		r.raw = append(r.raw, b)
		if b == '\n' {
			r.n++
			return strings.TrimSuffix(sb.String(), "\r"), r.raw, more, nil
		}
		if sb.Len() < max {
			sb.WriteByte(b)
		} else {
			more = true
		}
	}
}

// ---- ignore files ---------------------------------------------------------

// loadIgnoresInto appends the rules of rel's .gitignore/.ignore. It reads
// through the root and keys each rule to rel, exactly like the direct search
// does — the matcher (parseIgnore/ignored) is shared, so the two views of the
// workspace cannot disagree. A file larger than the cap is not parsed at all and
// is reported, because applying half of it would silently change what "search
// the workspace" means.
func (i *Inspection) loadIgnoresInto(ws *workspace, rules *[]ignoreRule, rel string) {
	base := ""
	if rel != "." {
		base = rel
	}
	for _, name := range []string{".gitignore", ".ignore"} {
		p := name
		if base != "" {
			p = base + "/" + name
		}
		f, fi, err := ws.openFile(p)
		if err != nil {
			continue
		}
		if fi.Size() > inspectIgnoreMax {
			f.Close()
			i.gap("ignore_too_large")
			continue
		}
		data, _ := io.ReadAll(io.LimitReader(f, inspectIgnoreMax))
		f.Close()
		*rules = append(*rules, parseIgnore(base, data)...)
	}
}

// loadAncestorIgnores loads the root's rules and then every directory from the
// workspace down to the search root: a rule in a parent still applies, and a
// deeper file overrides it because ignored() is last-match-wins.
func (i *Inspection) loadAncestorIgnores(ws *workspace, root string) []ignoreRule {
	var rules []ignoreRule
	i.loadIgnoresInto(ws, &rules, ".")
	if root == "." {
		return rules
	}
	cur := "."
	for _, part := range strings.Split(root, "/") {
		if cur == "." {
			cur = part
		} else {
			cur += "/" + part
		}
		i.loadIgnoresInto(ws, &rules, cur)
	}
	return rules
}
