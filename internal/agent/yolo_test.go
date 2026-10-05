package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/session"
)

func TestYoloTurnsEveryCheckOff(t *testing.T) {
	outside := filepath.Join(t.TempDir(), "out.txt")
	s := newScript(t,
		toolTurn(
			[2]string{"shell", `{"command":"uname"}`},                               // not allowlisted
			[2]string{"shell", `{"command":"eval echo hi"}`},                        // refused by analysis
			[2]string{"shell", `{"command":"chown $(id -u) a.txt"}`},                // hard-deny, harmless here
			[2]string{"write", fmt.Sprintf(`{"path":%q,"content":"x"}`, outside)},   // outside the jail
			[2]string{"edit", `{"path":"a.txt","old_string":"a","new_string":"b"}`}, // never read: guard stays on
		),
		textTurn("done"),
		toolTurn([2]string{"shell", `{"command":"uname"}`}),
		textTurn("refused"),
	)
	a, work, evs := startTest(t, s, 40, func(o *StartOptions) { o.Yolo = true })
	os.WriteFile(filepath.Join(work, "a.txt"), []byte("a\n"), 0o644)
	if _, err := a.Run(context.Background(), "go"); err != nil {
		t.Fatal(err)
	}
	var ends []ToolEnd
	for _, e := range *evs {
		if te, ok := e.(ToolEnd); ok {
			ends = append(ends, te)
		}
	}
	for i, te := range ends[:4] {
		if te.Result.IsError {
			t.Errorf("call %d must run under yolo: %s", i, te.Result.Content)
		}
	}
	if !ends[4].Result.IsError || !strings.Contains(ends[4].Result.Content, "read it first") {
		t.Fatal("read-before-write guard is not a permission; it stays on")
	}
	if b, _ := os.ReadFile(outside); string(b) != "x" {
		t.Fatal("write outside the workdir")
	}
	entries, _ := session.ReadFile(a.Session().Path())
	if !entries[0].Session.Yolo {
		t.Fatal("header records yolo")
	}

	a.SetYolo(false)
	*evs = nil
	a.Run(context.Background(), "again")
	for _, e := range *evs {
		if te, ok := e.(ToolEnd); ok && !te.Result.IsError {
			t.Fatal("checks are back after /yolo off")
		}
	}
	entries, _ = session.ReadFile(a.Session().Path())
	var modes int
	for _, e := range entries {
		if e.Type == session.TypePermissionMode {
			modes++
		}
	}
	if modes != 1 {
		t.Fatal("toggle writes one permission_mode entry")
	}
}
