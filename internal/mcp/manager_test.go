package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/adeotek/moca/internal/config"
)

func countingServer(t *testing.T) (config.MCPServer, func() int) {
	f := filepath.Join(t.TempDir(), "starts")
	s := fakeServer("")
	s.Env["MOCA_FAKE_STARTS"] = f
	return s, func() int { b, _ := os.ReadFile(f); return len(b) }
}

func TestLazyStartAndPersistedIndex(t *testing.T) {
	s, starts := countingServer(t)
	ixPath := filepath.Join(t.TempDir(), "ix.json")
	ix, _ := LoadIndex(ixPath)
	m := NewManager(map[string]config.MCPServer{"docs": s}, time.Minute, ix, Options{BaseEnv: os.Environ()})
	if starts() != 0 || len(m.Running()) != 0 {
		t.Fatal("nothing starts at construction")
	}
	hits, err := m.Search(context.Background(), "documentation", "")
	if err != nil || len(hits) != 1 || hits[0].Tool != "read_doc" || starts() != 1 {
		t.Fatal(hits, err, starts())
	}
	m.Close()

	// second "session": same index file, zero starts for search
	ix2, _ := LoadIndex(ixPath)
	m2 := NewManager(map[string]config.MCPServer{"docs": s}, time.Minute, ix2, Options{BaseEnv: os.Environ()})
	defer m2.Close()
	if hits, _ := m2.Search(context.Background(), "issue", ""); len(hits) != 1 || starts() != 1 {
		t.Fatal("search served from the persisted index; no new process", starts())
	}
	tool, err := m2.Describe(context.Background(), "docs", "read_doc")
	if err != nil || !strings.Contains(string(tool.InputSchema), "lib") || starts() != 2 {
		t.Fatal(tool, err)
	}
	r, _, err := m2.Call(context.Background(), "docs", "read_doc", json.RawMessage(`{"lib":"x"}`))
	if err != nil || r.Content[0].Text == "" || starts() != 2 {
		t.Fatal("call reuses the running server", r, err)
	}
}

func TestIdleStopAndRestart(t *testing.T) {
	s, starts := countingServer(t)
	ix, _ := LoadIndex(filepath.Join(t.TempDir(), "ix.json"))
	m := NewManager(map[string]config.MCPServer{"docs": s}, 150*time.Millisecond, ix, Options{BaseEnv: os.Environ()})
	defer m.Close()
	m.Call(context.Background(), "docs", "read_doc", nil)
	time.Sleep(400 * time.Millisecond)
	if len(m.Running()) != 0 {
		t.Fatal("stopped after idle timeout")
	}
	if _, _, err := m.Call(context.Background(), "docs", "read_doc", nil); err != nil || starts() != 2 {
		t.Fatal("restarts on demand", err, starts())
	}
}

// TestCallSurvivesIdleTimer (rev 11): a call landing exactly at idle expiry
// must not hit a nil client. 1ns idle → every call races the timer; run with
// -race. The one-critical-section Call (ensure + lookup + busy++ + timer stop
// under st.mu) closes the old two-section window.
func TestCallSurvivesIdleTimer(t *testing.T) {
	s, starts := countingServer(t)
	ix, _ := LoadIndex(filepath.Join(t.TempDir(), "ix.json"))
	m := NewManager(map[string]config.MCPServer{"docs": s}, time.Nanosecond, ix, Options{BaseEnv: os.Environ()})
	defer m.Close()
	var wg sync.WaitGroup
	for range 25 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, _, err := m.Call(context.Background(), "docs", "read_doc", nil); err != nil {
				t.Errorf("call at idle expiry: %v", err)
			}
		}()
	}
	wg.Wait()
	if starts() < 1 {
		t.Fatal("server started at least once:", starts())
	}
}

func TestUnknownServerAndTool(t *testing.T) {
	ix, _ := LoadIndex(filepath.Join(t.TempDir(), "ix.json"))
	m := NewManager(map[string]config.MCPServer{"docs": fakeServer("")}, time.Minute, ix, Options{BaseEnv: os.Environ()})
	defer m.Close()
	if _, err := m.Search(context.Background(), "x", "nope"); err == nil || !strings.Contains(err.Error(), "configured: docs") {
		t.Fatal(err)
	}
	if _, _, err := m.Call(context.Background(), "docs", "nope", nil); err == nil || !strings.Contains(err.Error(), "action=search") {
		t.Fatal(err)
	}
}
