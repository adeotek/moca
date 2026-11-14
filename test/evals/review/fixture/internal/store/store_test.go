package store

import (
	"path/filepath"
	"testing"
)

// The happy path the team already tests; the planted defects are all on paths
// this does not exercise, which is why they survived.
func TestSaveLoadRoundTrip(t *testing.T) {
	st := New(filepath.Join(t.TempDir(), "tasks.json"))
	if err := st.Save(Task{ID: "a", Title: "first"}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	all, err := st.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(all) != 1 || all[0].ID != "a" {
		t.Fatalf("Load = %+v", all)
	}
}
