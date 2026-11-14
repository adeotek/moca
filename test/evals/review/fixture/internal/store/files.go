package store

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Export writes one JSON file per task into dir.
func Export(dir string, tasks []Task) error {
	for _, t := range tasks {
		f, err := os.Create(filepath.Join(dir, t.ID+".json"))
		if err != nil {
			return err
		}
		defer f.Close()
		b, err := json.MarshalIndent(t, "", "  ")
		if err != nil {
			return err
		}
		if _, err := f.Write(b); err != nil {
			return err
		}
	}
	return nil
}
