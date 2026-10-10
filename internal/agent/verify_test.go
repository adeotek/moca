package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectVerify(t *testing.T) {
	cases := []struct {
		name  string
		files map[string]string
		want  string
	}{
		{"makefile targets win", map[string]string{"Makefile": "build:\n\tgo build\nvet:\n\tgo vet\ntest: build\n\tgo test\nX := 1\n", "go.mod": "module x\n"}, "make vet && make test"},
		{"makefile without checks falls through", map[string]string{"Makefile": "build:\n\tgo build\n", "go.mod": "module x\n"}, "go vet ./... && go test ./..."},
		{"pnpm scripts", map[string]string{"package.json": `{"scripts":{"test":"vitest","lint":"eslint ."}}`, "pnpm-lock.yaml": ""}, "pnpm run lint && pnpm test"},
		{"npm test only", map[string]string{"package.json": `{"scripts":{"test":"jest"}}`}, "npm test"},
		{"cargo", map[string]string{"Cargo.toml": "[package]\n"}, "cargo check && cargo test"},
		{"python with ruff", map[string]string{"pyproject.toml": "[tool.ruff]\nline-length = 100\n"}, "ruff check . && pytest"},
		{"dotnet", map[string]string{"app.sln": ""}, "dotnet build && dotnet test"},
		{"nothing", map[string]string{"README.md": "hi"}, ""},
	}
	for _, c := range cases {
		dir := t.TempDir()
		for n, body := range c.files {
			if err := os.WriteFile(filepath.Join(dir, n), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if got, _ := DetectVerify(dir); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}
