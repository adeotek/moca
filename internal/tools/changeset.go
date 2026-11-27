package tools

// Workspace changesets (SPECS §10.6, plan v7 §4): one bounded, ordered set of
// exact text edits per target, computed by a workspace script and applied by
// the host after the program returns. The guest never writes; the host owns
// every transition from staging to applied, and every byte it writes is a byte
// it validated and snapshotted first.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

// Changeset limits (§4.6). Every failed staging still charges the script's
// attempt budget, which the Inspection does on its own; these are the batch's.
const (
	maxChangesetTargets   = 32
	maxChangesetFileBytes = 512 << 10
	maxEditTextPerTarget  = 32 << 10
	maxEditTextTotal      = 256 << 10
	maxRetainedImages     = 2 << 20
	// maxStageContent bounds the post-edit image a staging result carries back to
	// the guest. 32 targets x this window equals maxRetainedImages, so a program
	// that holds every image it staged still cannot outgrow the host's own cap.
	maxStageContent     = 64 << 10
	changesetHostBudget = 30 * time.Second
	maxReceiptBytes     = 4096
)

// Changeset outcomes (§4.8). These are the only values the receipt carries.
const (
	OutcomeInspected      = "inspected"
	OutcomeApplied        = "applied"
	OutcomeDiscarded      = "discarded"
	OutcomePartial        = "partial"
	OutcomeOutcomeUnknown = "outcome_unknown"
)

// ChangesetPolicy is the narrow, deterministic authority a changeset reads. It
// is built once from the session header and current config (never inferred
// from the presence of an Asker): only a yolo session applies without a
// question, ask/auto ask exactly once, plan refuses, and a headless session
// refuses wherever a human would be needed.
type ChangesetPolicy struct {
	Edits    bool   // the session's edit authority (workspaceEdits ∩ policy)
	Mode     string // "default" | "ask" | "auto" | "yolo" | "plan"
	Headless bool   // -p
	Trusted  bool   // the workspace is trusted
}

// Allows reports whether a changeset may proceed without a question. Plan mode
// never may; yolo may (a multi-file write is exactly what yolo pre-authorises)
// and headless only under yolo.
func (p ChangesetPolicy) Allows() bool {
	return p.Edits && p.Trusted && p.Mode == "yolo"
}

// NeedsQuestion reports whether a human has to approve. Headless sessions have
// none, so a non-yolo headless run refuses instead of asking into the void.
func (p ChangesetPolicy) NeedsQuestion() bool {
	if !p.Edits || !p.Trusted {
		return false
	}
	switch p.Mode {
	case "yolo", "plan":
		return false
	}
	return true
}

// refuseReason names why a non-yolo session cannot apply, for the receipt.
func (p ChangesetPolicy) refuseReason() string {
	// Order matters: the most actionable reason wins. A -p run is untrusted by
	// construction (oneshot trusts only yolo), so reporting trust first would
	// hide the fact that no headless mode but yolo can ever apply.
	switch {
	case !p.Edits:
		return "workspace edits are not enabled for this session"
	case p.Mode == "plan":
		return "plan mode does not apply changesets"
	case p.Headless:
		return "a changeset needs approval, and -p has no one to ask; run with --mode yolo or apply the edits directly"
	case !p.Trusted:
		return "the workspace is not trusted, so multi-file writes are refused"
	}
	return "a changeset needs approval"
}

// ChangesetStage is one target's validated pair of images. Pre is the exact
// preimage read at stage time; Post is what the host will write.
type ChangesetStage struct {
	Rel   string // slash path inside the workspace, as the model wrote it
	Abs   string // filepath.Join(holder.Root, Rel) — for the snapshot record only
	Pre   []byte
	Post  []byte
	Mode  fs.FileMode
	Edits int
	ident os.FileInfo // identity + mtime + size at stage time
}

// Changeset accumulates one script's staged targets. It is per call.
type Changeset struct {
	policy  ChangesetPolicy
	root    *os.Root
	rootAbs string
	now     func() time.Time
	start   time.Time

	// applied is the number of files whose write landed and read back as the
	// postimage; the receipt reports it.
	applied    int
	targets    []*ChangesetStage
	unresolved map[string]string // rel -> why, cleared only by a successful restage
	editBytes  int
	retained   int
}

// NewChangeset starts a batch under rootAbs. It opens nothing; a script that
// never stages never pays for a root.
func NewChangeset(policy ChangesetPolicy, rootAbs string) *Changeset {
	return &Changeset{
		policy:     policy,
		rootAbs:    rootAbs,
		now:        time.Now,
		unresolved: map[string]string{},
	}
}

// errChangesetFatal wraps a fatal stage error so dispatch can classify it.
type changesetError struct {
	class string
	msg   string
}

func (e *changesetError) Error() string { return e.msg }

// fatal builds a fatal-for-this-changeset error (everything but a mismatch).
func fatal(class, format string, args ...any) *changesetError {
	return &changesetError{class: class, msg: fmt.Sprintf(format, args...)}
}

// Stage validates one stageEdit call and records it. A mismatch is correctable
// and commits no state for that target; everything else is fatal for the batch.
func (c *Changeset) Stage(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	var a struct {
		Path  string `json:"path"`
		Edits []struct {
			Old string `json:"old"`
			New string `json:"new"`
		} `json:"edits"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return nil, fatal("refused", "stageEdit: invalid arguments")
	}
	rel, err := c.checkPath(a.Path)
	if err != nil {
		return nil, err
	}
	if len(a.Edits) == 0 {
		return nil, fatal("refused", "stageEdit %s: edits must be a nonempty array", rel)
	}
	if err := c.budget(len(a.Edits)); err != nil {
		return nil, err
	}

	root, err := c.openRoot()
	if err != nil {
		return nil, err
	}
	pre, mode, ident, err := c.readTarget(root, rel)
	if err != nil {
		return nil, err
	}

	post, textBytes, err := applyEdits(rel, pre, a.Edits)
	if err != nil {
		// A mismatch is correctable: commit no staging state for this target and
		// mark it unresolved. Only a later successful restage clears the mark.
		if ce, ok := err.(*changesetError); ok && ce.class == "mismatch" {
			c.drop(rel)
			c.unresolved[rel] = ce.msg
		}
		return nil, err
	}
	if bytes.Equal(pre, post) {
		return nil, fatal("refused", "stageEdit %s: the edits leave the file unchanged", rel)
	}
	if textBytes > maxEditTextPerTarget {
		return nil, fatal("budget", "stageEdit %s: edit text exceeds %d KiB", rel, maxEditTextPerTarget>>10)
	}
	if c.editBytes+textBytes > maxEditTextTotal {
		return nil, fatal("budget", "stageEdit: edit text exceeds %d KiB per changeset", maxEditTextTotal>>10)
	}
	if err := c.growth(len(pre), len(post)); err != nil {
		return nil, err
	}

	// Duplicate identity: the same filesystem object may be staged once. A
	// different spelling (case, link, ./-prefix) must not create a second target
	// that would be applied twice and validated against a stale preimage.
	for _, t := range c.targets {
		if os.SameFile(t.ident, ident) {
			return nil, fatal("refused", "stageEdit %s: the same file is already staged as %s", rel, t.Rel)
		}
	}

	st := &ChangesetStage{Rel: rel, Abs: filepath.Join(c.rootAbs, rel), Pre: pre, Post: post, Mode: mode, Edits: len(a.Edits), ident: ident}
	// A successful restage replaces the earlier stage of the same path and
	// clears its unresolved mark; the budget it already charged is not refunded.
	for i, t := range c.targets {
		if t.Rel == rel {
			c.targets[i] = st
			delete(c.unresolved, rel)
			c.editBytes += textBytes
			return stagedResult(rel, len(a.Edits), post), nil
		}
	}
	c.targets = append(c.targets, st)
	c.editBytes += textBytes
	delete(c.unresolved, rel)

	return stagedResult(rel, len(a.Edits), post), nil
}

// drop removes an earlier stage of rel, if any (a failed restage supersedes it).
func (c *Changeset) drop(rel string) {
	for i, t := range c.targets {
		if t.Rel == rel {
			c.targets = append(c.targets[:i], c.targets[i+1:]...)
			return
		}
	}
}

// stagedResult is what a successful stage returns to the guest: the identifying
// fields, the size and digest of the whole post-edit image, and the image itself
// up to maxStageContent. The guest needs the bytes to check its own work before
// the host applies anything -- without them, verification has to happen in a
// later turn through a shell, which is where the phase-7 campaign's ordering
// failures came from.
func stagedResult(rel string, edits int, post []byte) json.RawMessage {
	sum := sha256.Sum256(post)
	body, truncated := post, false
	if len(body) > maxStageContent {
		cut := maxStageContent
		for cut > 0 && !utf8.RuneStart(post[cut]) {
			cut-- // never hand the guest half a rune
		}
		body, truncated = post[:cut], true
	}
	out, _ := json.Marshal(map[string]any{
		"staged": true, "path": rel, "edits": edits,
		"bytes": len(post), "sha256": hex.EncodeToString(sum[:]),
		"content": string(body), "truncated": truncated,
	})
	return out
}

// applyEdits applies the ordered pairs to pre and returns the image. A missing
// or ambiguous match is correctable: the caller marks the target unresolved and
// commits nothing for it.
func applyEdits(rel string, pre []byte, edits []struct {
	Old string `json:"old"`
	New string `json:"new"`
}) ([]byte, int, error) {
	cur := append([]byte(nil), pre...)
	textBytes := 0
	for i, e := range edits {
		if e.Old == "" {
			return nil, 0, &changesetError{class: "mismatch", msg: fmt.Sprintf("stageEdit %s: edit %d has an empty old text", rel, i+1)}
		}
		if e.Old == e.New {
			return nil, 0, fatal("refused", "stageEdit %s: edit %d does not change anything", rel, i+1)
		}
		textBytes += len(e.Old) + len(e.New)
		if n := bytes.Count(cur, []byte(e.Old)); n != 1 {
			if n == 0 {
				return nil, 0, &changesetError{class: "mismatch", msg: fmt.Sprintf("stageEdit %s: edit %d: old text not found", rel, i+1)}
			}
			return nil, 0, &changesetError{class: "mismatch", msg: fmt.Sprintf("stageEdit %s: edit %d: old text matches %d places, needs exactly one", rel, i+1, n)}
		}
		cur = bytes.Replace(cur, []byte(e.Old), []byte(e.New), 1)
	}
	return cur, textBytes, nil
}

// checkPath enforces §4.5: a workspace-relative existing ordinary file, no
// escapes, no dot components (bar an admitted env example), no credentials,
// no repo instructions.
func (c *Changeset) checkPath(p string) (string, error) {
	if p == "" {
		return "", fatal("refused", "stageEdit: path is empty")
	}
	if strings.ContainsRune(p, 0) {
		return "", fatal("refused", "stageEdit %q: path contains a NUL byte", p)
	}
	if !utf8.ValidString(p) {
		return "", fatal("refused", "stageEdit: path is not valid UTF-8")
	}
	if strings.HasPrefix(p, "/") || filepath.IsAbs(p) || filepath.VolumeName(p) != "" {
		return "", fatal("refused", "stageEdit %s: absolute paths are refused", p)
	}
	clean := path.Clean(p)
	if clean != p || clean == "." {
		return "", fatal("refused", "stageEdit %s: path must be canonical (got %s)", p, clean)
	}
	parts := strings.Split(clean, "/")
	for i, part := range parts {
		if part == ".." || part == "." || part == "" {
			return "", fatal("refused", "stageEdit %s: path escapes the workspace", p)
		}
		if strings.HasPrefix(part, ".") {
			// The env-example exception: only the final component, and only the
			// admitted suffixes. Every other dot component stays refused.
			if i == len(parts)-1 && envExample(part) {
				continue
			}
			return "", fatal("refused", "stageEdit %s: dot components are refused", p)
		}
		lower := strings.ToLower(part)
		switch lower {
		case ".git", ".moca":
			return "", fatal("refused", "stageEdit %s: %s is not editable", p, part)
		}
		if credentialName(lower) {
			return "", fatal("refused", "stageEdit %s: credential files are not editable", p)
		}
		if repoInstruction(lower) {
			return "", fatal("refused", "stageEdit %s: repository instructions are not edited by a changeset", p)
		}
	}
	return clean, nil
}

// envExample is the one helper both call sites use (§4.5): the dot-component
// exemption and the credential-name refusal. A base qualifies only when it
// starts with ".env" and ends in an admitted example suffix.
func envExample(base string) bool {
	b := strings.ToLower(base)
	if !strings.HasPrefix(b, ".env") {
		return false
	}
	for _, suf := range []string{".example", ".sample", ".template"} {
		if strings.HasSuffix(b, suf) {
			return true
		}
	}
	return false
}

func credentialName(lower string) bool {
	if strings.HasPrefix(lower, ".env") {
		return !envExample(lower)
	}
	switch lower {
	case "auth.json", "credentials.json", "id_rsa", "id_ed25519":
		return true
	}
	return strings.HasSuffix(lower, ".key") || strings.HasSuffix(lower, ".pem") || strings.HasSuffix(lower, ".p12") || strings.HasSuffix(lower, ".pfx")
}

func repoInstruction(lower string) bool {
	switch lower {
	case "agents.md", "claude.md", ".cursorrules", "skill.md", ".windsurfrules":
		return true
	}
	return false
}

// openRoot opens the workspace root once for the batch.
func (c *Changeset) openRoot() (*os.Root, error) {
	if c.root != nil {
		return c.root, nil
	}
	r, err := os.OpenRoot(c.rootAbs)
	if err != nil {
		return nil, fatal("refused", "stageEdit: the workspace cannot be opened for writing: %v", err)
	}
	c.root = r
	return r, nil
}

// Close releases the batch's root. Safe on a nil changeset.
func (c *Changeset) Close() {
	if c != nil && c.root != nil {
		_ = c.root.Close()
		c.root = nil
	}
}

// readTarget opens rel through the root, refuses anything but a regular
// single-link file, and returns the bounded preimage with its mode and identity.
func (c *Changeset) readTarget(root *os.Root, rel string) ([]byte, fs.FileMode, os.FileInfo, error) {
	fi, err := root.Lstat(rel)
	if err != nil {
		return nil, 0, nil, fatal("refused", "stageEdit %s: %v", rel, err)
	}
	if !fi.Mode().IsRegular() {
		return nil, 0, nil, fatal("refused", "stageEdit %s: only ordinary files can be edited", rel)
	}
	if fi.Mode()&fs.ModeSymlink != 0 {
		return nil, 0, nil, fatal("refused", "stageEdit %s: symbolic links are refused", rel)
	}
	if fi.Size() > maxChangesetFileBytes {
		return nil, 0, nil, fatal("budget", "stageEdit %s: the file exceeds %d KiB", rel, maxChangesetFileBytes>>10)
	}
	f, err := root.OpenFile(rel, os.O_RDONLY, 0)
	if err != nil {
		return nil, 0, nil, fatal("refused", "stageEdit %s: %v", rel, err)
	}
	defer f.Close()
	ident, err := f.Stat()
	if err != nil {
		return nil, 0, nil, fatal("refused", "stageEdit %s: %v", rel, err)
	}
	pre, err := io.ReadAll(io.LimitReader(f, maxChangesetFileBytes+1))
	if err != nil {
		return nil, 0, nil, fatal("refused", "stageEdit %s: %v", rel, err)
	}
	if len(pre) > maxChangesetFileBytes {
		return nil, 0, nil, fatal("budget", "stageEdit %s: the file exceeds %d KiB", rel, maxChangesetFileBytes>>10)
	}
	if err := textOnly(rel, "file", pre); err != nil {
		return nil, 0, nil, err
	}
	return pre, ident.Mode(), ident, nil
}

func textOnly(rel, what string, b []byte) error {
	if bytes.IndexByte(b, 0) >= 0 {
		return fatal("refused", "stageEdit %s: the %s is binary (NUL byte)", rel, what)
	}
	if !utf8.Valid(b) {
		return fatal("refused", "stageEdit %s: the %s is not valid UTF-8", rel, what)
	}
	return nil
}

func (c *Changeset) budget(int) error {
	if len(c.targets) >= maxChangesetTargets {
		return fatal("budget", "stageEdit: at most %d files per changeset", maxChangesetTargets)
	}
	if c.editBytes > maxEditTextTotal {
		return fatal("budget", "stageEdit: edit text exceeds %d KiB per changeset", maxEditTextTotal>>10)
	}
	return nil
}

func (c *Changeset) growth(pre, post int) error {
	c.retained += pre + post
	if c.retained > maxRetainedImages {
		return fatal("budget", "stageEdit: staged images exceed %d MiB", maxRetainedImages>>20)
	}
	if post > maxChangesetFileBytes {
		return fatal("budget", "stageEdit: the result would exceed %d KiB", maxChangesetFileBytes>>10)
	}
	return nil
}

// Unresolved reports whether the batch has a target that failed to stage.
func (c *Changeset) Unresolved() (string, bool) {
	for rel, why := range c.unresolved {
		return rel + ": " + why, true
	}
	return "", false
}

// TargetCount is the number of distinct files staged.
func (c *Changeset) TargetCount() int { return len(c.targets) }

// Apply runs the whole-set preflight, the consent question (§4.10), the
// post-approval revalidation, the journaled apply and the read-back, and
// returns a bounded receipt. It never panics and never retries.
func (c *Changeset) Apply(ctx context.Context, env *Env) (status, receipt string) {
	if c == nil || len(c.targets) == 0 {
		return OutcomeInspected, ""
	}
	if u, bad := c.Unresolved(); bad {
		return OutcomeDiscarded, "discarded: " + u
	}
	if !c.policy.Edits || !c.policy.Trusted || c.policy.Mode == "plan" {
		return OutcomeDiscarded, "discarded: " + c.policy.refuseReason()
	}
	if c.policy.NeedsQuestion() {
		if env.Ask == nil || c.policy.Headless {
			return OutcomeDiscarded, "discarded: " + c.policy.refuseReason()
		}
		q := Question{
			Kind:      "changeset",
			Subject:   fmt.Sprintf("%d files, %d edits", len(c.targets), c.edits()),
			Detail:    c.preview(),
			CanAlways: false,
			NoSession: true,
		}
		if !env.Ask(ctx, q).Allows() {
			return OutcomeDiscarded, "discarded: the changeset was denied"
		}
	}

	// Preflight once before asking (a stale target must not reach a human) and
	// again after approval (a changed target invalidates the approval).
	if why := c.revalidate(); why != "" {
		return OutcomeDiscarded, "discarded: " + why
	}
	if c.policy.NeedsQuestion() && !c.policy.Headless {
		if why := c.revalidate(); why != "" {
			return OutcomeDiscarded, "discarded: " + why
		}
	}

	img, ok := env.Snap.(ImageSnapshotter)
	if !ok || img == nil {
		return OutcomeDiscarded, "discarded: this session cannot snapshot a changeset safely (no image guard); use edit"
	}
	if env.Journal == nil {
		return OutcomeDiscarded, "discarded: this session has no changeset journal; use edit"
	}
	batch := ChangesetBatch{
		CallID: callIDOf(ctx),
		Paths:  make([]string, len(c.targets)),
		Pre:    make([][32]byte, len(c.targets)),
		Post:   make([][32]byte, len(c.targets)),
	}
	for i, t := range c.targets {
		batch.Paths[i] = t.Rel
		batch.Pre[i] = sha256Sum(t.Pre)
		batch.Post[i] = sha256Sum(t.Post)
	}
	if err := env.Journal.Begin(batch); err != nil {
		return OutcomeDiscarded, "discarded: the changeset journal could not record the batch: " + err.Error()
	}

	deadline := c.now().Add(changesetHostBudget)
	applied := 0
	for _, t := range c.targets {
		if ctx.Err() != nil || c.now().After(deadline) {
			env.Journal.Abort("interrupted")
			return c.final(applied, len(c.targets), OutcomeDiscarded, "stopped before "+t.Rel)
		}
		err := img.GuardImage(t.Abs, t.Pre, t.Post, t.Mode.Perm(), func() error {
			return c.rename(t)
		})
		if err != nil {
			env.Journal.Abort("snapshot: " + err.Error())
			return c.final(applied, len(c.targets), OutcomeOutcomeUnknown, "snapshot failed for "+t.Rel+": "+err.Error())
		}
		landed, rerr := c.readBack(t)
		if rerr != nil {
			env.Journal.Abort("read-back: " + rerr.Error())
			return c.final(applied+1, len(c.targets), OutcomeOutcomeUnknown, "read-back failed for "+t.Rel+": "+rerr.Error())
		}
		if !landed {
			env.Journal.Abort("read-back mismatch")
			return c.final(applied+1, len(c.targets), OutcomeOutcomeUnknown, "the file on disk is not the image this changeset wrote: "+t.Rel)
		}
		applied++
		if err := env.Journal.Record(t.Rel, t.Pre, t.Post); err != nil {
			env.Journal.Abort("record: " + err.Error())
			// The write landed; without its record the outcome is unknown.
			return c.final(applied, len(c.targets), OutcomeOutcomeUnknown, "the journal could not record "+t.Rel+": "+err.Error())
		}
	}
	if err := env.Journal.Finish(); err != nil {
		return c.final(applied, len(c.targets), OutcomeUnknownOrPartial(applied), "the journal could not finish: "+err.Error())
	}
	for _, t := range c.targets {
		noteChange(env, t.Abs)
	}
	c.applied = applied
	return c.final(applied, len(c.targets), OutcomeApplied, "")
}

// paths returns the ordered staged paths for the receipt.
func (c *Changeset) paths() []string {
	out := make([]string, len(c.targets))
	for i, t := range c.targets {
		out[i] = t.Rel
	}
	return out
}

func (c *Changeset) edits() int {
	n := 0
	for _, t := range c.targets {
		n += t.Edits
	}
	return n
}

// revalidate re-reads every target through the pinned root and compares bytes,
// identity, size and modification time. It returns why the set is stale, or "".
func (c *Changeset) revalidate() string {
	root, err := c.openRoot()
	if err != nil {
		return "the workspace cannot be reopened"
	}
	for _, t := range c.targets {
		f, err := root.OpenFile(t.Rel, os.O_RDONLY, 0)
		if err != nil {
			return t.Rel + " is no longer readable"
		}
		ident, serr := f.Stat()
		var cur []byte
		if serr == nil {
			cur, serr = io.ReadAll(io.LimitReader(f, maxChangesetFileBytes+1))
		}
		f.Close()
		if serr != nil {
			return t.Rel + " could not be re-read"
		}
		if !os.SameFile(t.ident, ident) || !ident.ModTime().Equal(t.ident.ModTime()) || ident.Size() != t.ident.Size() || !bytes.Equal(cur, t.Pre) {
			return t.Rel + " changed while the changeset was being prepared"
		}
	}
	return ""
}

// rename replaces the target through the pinned root: a temp file in the same
// directory, created with the target's mode at creation (never Chmod, which is
// racy on Unix once the target can become a symlink), fsynced, then renamed.
func (c *Changeset) rename(t *ChangesetStage) error {
	root, err := c.openRoot()
	if err != nil {
		return err
	}
	dir := path.Dir(t.Rel)
	base := path.Base(t.Rel)
	var tmp string
	var f *os.File
	for i := 0; i < 8; i++ {
		tmp = path.Join(dir, fmt.Sprintf(".%s.moca-%d-%d", base, os.Getpid(), c.now().UnixNano()+int64(i)))
		f, err = root.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, t.Mode.Perm())
		if err == nil {
			break
		}
		if !errors.Is(err, fs.ErrExist) {
			return err
		}
	}
	if f == nil {
		return errors.New("could not create a temporary file")
	}
	if _, err := f.Write(t.Post); err != nil {
		f.Close()
		_ = root.Remove(tmp)
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		_ = root.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		_ = root.Remove(tmp)
		return err
	}
	if err := root.Rename(tmp, t.Rel); err != nil {
		_ = root.Remove(tmp)
		return err
	}
	return nil
}

// readBack verifies the landed image through the same root.
func (c *Changeset) readBack(t *ChangesetStage) (bool, error) {
	root, err := c.openRoot()
	if err != nil {
		return false, err
	}
	got, err := root.ReadFile(t.Rel)
	if err != nil {
		return false, err
	}
	return bytes.Equal(got, t.Post), nil
}

// preview renders the consent body: ordered paths with edit counts, then the
// hunks, bounded to 16 KiB with an elision marker.
func (c *Changeset) preview() string {
	var b strings.Builder
	for _, t := range c.targets {
		fmt.Fprintf(&b, "  %s (%d edit", t.Rel, t.Edits)
		if t.Edits != 1 {
			b.WriteString("s")
		}
		b.WriteString(")\n")
	}
	b.WriteString("\n")
	hunks := 0
	for _, t := range c.targets {
		for _, line := range diffLines(t.Rel, t.Pre, t.Post) {
			if b.Len()+len(line)+1 > 16<<10 {
				fmt.Fprintf(&b, "… %d more hunks omitted\n", totalHunks(c.targets)-hunks)
				return b.String()
			}
			b.WriteString(line)
			b.WriteByte('\n')
			hunks++
		}
	}
	return b.String()
}

func totalHunks(ts []*ChangesetStage) int {
	n := 0
	for _, t := range ts {
		n += len(diffLines(t.Rel, t.Pre, t.Post))
	}
	return n
}

// diffLines is a compact, bounded rendering: whole-line context is not
// attempted — the changeset's pairs are exact and few — so it lists the
// before/after of each pair with its file.
func diffLines(rel string, pre, post []byte) []string {
	// Cheap and deterministic: show the changed line ranges by walking both
	// images. The full diff belongs to the receipt after apply, so this stays a
	// summary by design.
	preLines, postLines := changesetLines(pre), changesetLines(post)
	var out []string
	// Longest common prefix/suffix keeps the rendering small for the common
	// case (one line changed in a 30-line file).
	p := 0
	for p < len(preLines) && p < len(postLines) && preLines[p] == postLines[p] {
		p++
	}
	s := 0
	for s < len(preLines)-p && s < len(postLines)-p && preLines[len(preLines)-1-s] == postLines[len(postLines)-1-s] {
		s++
	}
	out = append(out, fmt.Sprintf("  @@ %s @@", rel))
	for _, l := range preLines[p : len(preLines)-s] {
		out = append(out, "  - "+l)
	}
	for _, l := range postLines[p : len(postLines)-s] {
		out = append(out, "  + "+l)
	}
	return out
}

func changesetLines(b []byte) []string {
	s := strings.Split(string(b), "\n")
	if len(s) > 0 && s[len(s)-1] == "" {
		s = s[:len(s)-1]
	}
	return s
}

// final records how many files landed and renders the one-line status.
func (c *Changeset) final(applied, total int, status, why string) (string, string) {
	c.applied = applied
	var b strings.Builder
	fmt.Fprintf(&b, "%s: %d of %d files written", status, applied, total)
	if why != "" {
		fmt.Fprintf(&b, " (%s)", why)
	}
	return status, b.String()
}

// OutcomeUnknownOrPartial is the status a failed Finish can still claim: every
// write landed, so the files are the new images even though the record is open.
func OutcomeUnknownOrPartial(applied int) string {
	if applied > 0 {
		return OutcomeOutcomeUnknown
	}
	return OutcomeDiscarded
}

// sha256Sum is the digest form the journal records.
func sha256Sum(b []byte) [32]byte { return sha256.Sum256(b) }

// callIDOf returns the outer call identity from the context, for the journal.
func callIDOf(ctx context.Context) string {
	if v, ok := ctx.Value(callIDKey{}).(string); ok {
		return v
	}
	return ""
}

type callIDKey struct{}

// WithCallID binds the provider call identity to ctx (agent sets it before
// running the tool; the journal identifies a batch by it, never by JS input).
func WithCallID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, callIDKey{}, id)
}
