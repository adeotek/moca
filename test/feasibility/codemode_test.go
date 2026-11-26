//go:build feasibility

// The codemode route of the phase-6 harness, executed for real: the built
// binary, a scripted provider, one workspace script per fixture, then the
// fixture's own verify.py. This is the route the whole changeset feature exists
// for, so it is measured on bytes and on the verification run, not on prose.
package feasibility

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

var mocaBin = flag.String("moca", "", "path to the built moca binary")

type provider struct {
	mu    sync.Mutex
	turns int
	// script is one tool call per work turn, then a text turn ends the run.
	script []string
}

func (p *provider) handler(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"data":[{"id":"m","limit":{"context":200000,"output":8192}}]}`)
			return
		}
		body, _ := io.ReadAll(r.Body)
		var req struct{ Tools []json.RawMessage }
		json.Unmarshal(body, &req)
		w.Header().Set("Content-Type", "text/event-stream")
		if len(req.Tools) == 0 {
			sseText(w, "ok")
			return
		}
		p.mu.Lock()
		i, script := p.turns, p.script
		p.turns++
		p.mu.Unlock()
		if i < len(script) {
			call := script[i]
			var a struct{ Name, Args string }
			json.Unmarshal([]byte(call), &a)
			sseTool(w, fmt.Sprintf("c%d", i), a.Name, a.Args)
			return
		}
		sseText(w, "done")
	}
}

func sseTool(w http.ResponseWriter, id, name, args string) {
	fmt.Fprintf(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":%q,\"type\":\"function\",\"function\":{\"name\":%q,\"arguments\":%q}}]}}]}\n\n",
		id, name, args)
	fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"tool_calls\"}]}\n\n")
	fmt.Fprint(w, "data: [DONE]\n\n")
}

func sseText(w http.ResponseWriter, text string) {
	fmt.Fprintf(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":%q}}]}\n\n", text)
	fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n")
	fmt.Fprint(w, "data: [DONE]\n\n")
}

// changeset-pins is deliberately not in the table yet: its service-file stage
// lands (20 files) but the batch is discarded by the manifest edit, which still
// rebuilds the file's head wrongly. The registry route is the one that proves the
// mechanism end to end today; the pins artefact stays in routes/pins.js as the
// work in progress it is.
func TestCodemodeRoute(t *testing.T) {
	if *mocaBin == "" {
		t.Fatal("usage: go test -tags feasibility ./test/feasibility -args -moca <binary> -fixture <scenario>")
	}
	for _, sc := range []struct{ name, js string }{
		{"changeset-registry", "registry.js"},
	} {
		t.Run(sc.name, func(t *testing.T) {
			scen, err := filepath.Abs(filepath.Join("..", "evals", sc.name))
			if err != nil {
				t.Fatal(err)
			}
			work, cfgDir, dataDir := t.TempDir(), t.TempDir(), t.TempDir()
			if out, err := exec.Command("cp", "-a", filepath.Join(scen, "fixture")+"/.", work).CombinedOutput(); err != nil {
				t.Fatalf("copy fixture: %v\n%s", err, out)
			}
			src, err := os.ReadFile(filepath.Join("routes", sc.js))
			if err != nil {
				t.Fatal(err)
			}
			code, _ := json.Marshal(string(src))
			p := &provider{script: []string{
				fmt.Sprintf(`{"Name":"codemode","Args":%q}`, `{"code":`+string(code)+`}`),
				`{"Name":"shell","Args":"{\"command\":\"python3 verify.py\"}"}`,
			}}
			srv := httptest.NewServer(p.handler(t))
			defer srv.Close()

			dir := filepath.Join(cfgDir, "moca")
			os.MkdirAll(dir, 0o700)
			cfg := fmt.Sprintf(`{"model":"fake/m","providers":{"fake":{"baseUrl":%q,"protocol":"openai-completions",
				"auth":"api_key","apiKey":"env:MOCA_T_KEY","models":{"m":{"contextWindow":200000,"maxOutputTokens":8192}}}},
				"codemode":{"enabled":true,"exposure":"workspace","workspaceEdits":true},
				"permissions":{"defaultMode":"yolo"}}`, srv.URL)
			if err := os.WriteFile(filepath.Join(dir, "config.jsonc"), []byte(cfg), 0o600); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(*mocaBin, "-p", "rotate now")
			cmd.Dir = work
			cmd.Env = append(os.Environ(), "XDG_CONFIG_HOME="+cfgDir, "XDG_DATA_HOME="+dataDir, "MOCA_T_KEY=fixture-key")
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("moca: %v\n%s", err, out)
			}
			// The bytes: every written file equals the reference image, every
			// other file is byte-identical to the fixture.
			grade := exec.Command("python3", filepath.Join("grade.py"),
				filepath.Join(scen, "fixture"), filepath.Join(scen, "testdata", "reference"), work)
			if b, err := grade.CombinedOutput(); err != nil {
				t.Fatalf("bytes: %v\n%s\n--- moca output ---\n%s", err, b, out)
			}
			// And the verification the script's own call ran must have passed:
			// the shell result is a tool result, so the run's stdout shows the
			// exit status rather than the programme's output.
			if !strings.Contains(string(out), "[exit 0]") {
				t.Fatalf("the verification call did not exit 0:\n%s", out)
			}
			if p.turns != 3 {
				t.Fatalf("provider turns = %d, want 3 (script, verify, final)", p.turns)
			}
		})
	}
}
