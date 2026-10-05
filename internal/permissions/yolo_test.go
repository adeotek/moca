package permissions

import (
	"path/filepath"
	"testing"
)

func TestUnjailedAndAllowAll(t *testing.T) {
	root := t.TempDir()
	u := NewUnjailed(root)
	if p, err := u.Resolve("/etc/hosts", true); err != nil || p != "/etc/hosts" {
		t.Fatal(p, err)
	}
	if p, _ := u.Resolve("sub/x.go", true); p != filepath.Join(root, "sub/x.go") {
		t.Fatal(p)
	}
	for _, cmd := range []string{"sudo rm -rf /tmp/x", "eval $CMD", "if then fi (", "cat > ../outside"} {
		if need, every, err := (AllowAll{}).Check(cmd); need != nil || every != nil || err != nil {
			t.Errorf("%q: %v %v %v", cmd, need, every, err)
		}
	}
}
