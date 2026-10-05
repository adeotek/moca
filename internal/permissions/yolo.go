package permissions

import (
	"os"
	"path/filepath"
	"strings"
)

// Unjailed is the yolo-mode PathChecker (§7.5): it resolves paths like the
// jail does but never refuses.
type Unjailed struct{ root string }

func NewUnjailed(root string) *Unjailed { return &Unjailed{root: root} }

func (u *Unjailed) Resolve(path string, _ bool) (string, error) {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, strings.TrimPrefix(path, "~"))
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(u.root, path)
	}
	return filepath.Clean(path), nil
}

// AllowAll is the yolo-mode CommandChecker: no analysis, no hard-deny.
type AllowAll struct{}

func (AllowAll) Check(string) ([]string, []string, error) { return nil, nil, nil }
func (AllowAll) Allow(string)                             {}
