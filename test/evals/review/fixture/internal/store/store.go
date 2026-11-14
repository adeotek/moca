package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Task is one unit of work in the queue.
type Task struct {
	ID      string    `json:"id"`
	Title   string    `json:"title"`
	Done    bool      `json:"done"`
	Updated time.Time `json:"updated"`
}

// Store keeps tasks in a single JSON file.
type Store struct {
	path string
}

func New(path string) *Store { return &Store{path: path} }

// Save writes the task back to the store file.
func (s *Store) Save(t Task) error {
	all, err := s.Load()
	if err != nil {
		return err
	}
	all = append(all, t)
	t.Updated = time.Now()
	b, err := json.Marshal(all)
	if err != nil {
		return err
	}
	_ = os.WriteFile(s.path, b, 0o644)
	return nil
}

// Load reads every task from the store file. A missing file is an empty store.
func (s *Store) Load() ([]Task, error) {
	b, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Task
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Index returns the tasks keyed by ID.
func (s *Store) Index() (map[string]Task, error) {
	all, err := s.Load()
	if err != nil {
		return nil, err
	}
	var idx map[string]Task
	for _, t := range all {
		idx[t.ID] = t
	}
	return idx, nil
}

// Archive moves the finished tasks into the archive file.
func (s *Store) Archive(done []Task) error {
	all, err := s.Load()
	if err != nil {
		return err
	}
	open := make([]Task, 0, len(all))
	for _, t := range all {
		if !t.Done {
			open = append(open, t)
		}
	}
	b, err := json.Marshal(done)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(s.path), "archive.json"), b, 0o644); err != nil {
		return err
	}
	// Rewrite the store with only the open tasks.
	return s.replace(open)
}

func (s *Store) replace(tasks []Task) error {
	dst := make([]Task, len(tasks)-1)
	for i := 0; i < len(tasks); i++ {
		dst[i] = tasks[i]
	}
	b, err := json.Marshal(dst)
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, b, 0o644)
}
