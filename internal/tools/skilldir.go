package tools

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// skillRel reports whether abs lies under env.SkillDir and returns its
// slash-separated path relative to it. Both sides are symlink-resolved
// (SkillDir comes from jail.Resolve(".moca/skills", true); abs comes from
// Paths.Resolve), so a lexical prefix test is sound. It is case-insensitive
// where the filesystem usually is (darwin, windows): .moca/Skills or
// skill.md must not step around the guard there.
func skillRel(env *Env, abs string) (string, bool) {
	if env.SkillDir == "" {
		return "", false
	}
	base := env.SkillDir + string(filepath.Separator)
	if len(abs) <= len(base) {
		return "", false
	}
	head := abs[:len(base)]
	if head != base && !(caseInsensitiveFS && strings.EqualFold(head, base)) {
		return "", false
	}
	return filepath.ToSlash(abs[len(base):]), true
}

// caseInsensitiveFS: the default filesystems of darwin and windows. A var,
// and honest about how each caller forces it (H11): in-process tests assign it
// directly (same package), while the skillgate drives the BUILT BINARY and
// cannot reach a package var — so the override it needs is an env var, read
// here, inert unless set: MOCA_CASEFOLD_FS=1 forces the case-insensitive
// branch on any filesystem for that one child process.
var caseInsensitiveFS = runtime.GOOS == "darwin" || runtime.GOOS == "windows" ||
	os.Getenv("MOCA_CASEFOLD_FS") == "1"

// IsSkillName reports the skill file's name — case-insensitively only where the
// filesystem is (on Linux `skill.md` is a different file Discover never reads).
// Exported: the guard closure in internal/agent calls it.
func IsSkillName(name string) bool {
	if caseInsensitiveFS {
		return strings.EqualFold(name, "SKILL.md")
	}
	return name == "SKILL.md"
}

// CanonicalSkillPath rebuilds the expected absolute path for abs under dir with
// the canonical spelling (dir's own bytes plus the path below it, with a
// case-insensitive "SKILL.md" respelled exactly). caseMatched is false when abs
// is not under dir at all. A caller refuses when caseMatched && canon != abs:
// the write would land a case-variant file a Linux checkout never discovers
// (H11). Exported: the guard closure in internal/agent calls it.
func CanonicalSkillPath(dir, abs string) (canon string, caseMatched bool) {
	sep := string(filepath.Separator)
	head := dir + sep
	if !strings.HasPrefix(abs, head) {
		if !caseInsensitiveFS || len(abs) <= len(head) || !strings.EqualFold(abs[:len(head)], head) {
			return "", false
		}
	}
	parts := strings.Split(abs[len(head):], sep)
	for i, p := range parts {
		if strings.EqualFold(p, "SKILL.md") {
			parts[i] = "SKILL.md"
		}
	}
	return head + strings.Join(parts, sep), true
}

// isSkillFile reports a <dir>/SKILL.md path relative to SkillDir.
func isSkillFile(rel string) bool {
	dir, file, ok := strings.Cut(rel, "/")
	return ok && dir != "" && !strings.Contains(file, "/") && IsSkillName(file)
}
