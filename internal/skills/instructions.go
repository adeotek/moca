package skills

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

const instructionCap = 32_000

type Instruction struct {
	Path      string
	Content   string
	Truncated bool
}

func LoadInstructions(globalDir, workdir string, trusted bool) ([]Instruction, error) {
	var out []Instruction
	load := func(p string) (bool, error) {
		b, err := os.ReadFile(p)
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		in := Instruction{Path: p, Content: string(b)}
		if len(in.Content) > instructionCap {
			in.Content, in.Truncated = in.Content[:instructionCap]+"\n[… truncated at 32K chars]", true
		}
		out = append(out, in)
		return true, nil
	}
	if _, err := load(filepath.Join(globalDir, "AGENTS.md")); err != nil {
		return nil, err
	}
	if trusted {
		found, err := load(filepath.Join(workdir, "AGENTS.md"))
		if err != nil {
			return nil, err
		}
		if !found {
			if _, err := load(filepath.Join(workdir, "CLAUDE.md")); err != nil {
				return nil, err
			}
		}
	}
	return out, nil
}
