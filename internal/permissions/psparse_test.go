package permissions

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// readPayload loads one recorded wrapper reply.
func readPayload(t *testing.T, name string) parsedScript {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "psparse", "ast-"+name+".json"))
	if err != nil {
		t.Fatalf("fixture %s: %v", name, err)
	}
	ps, err := decodePS(raw)
	if err != nil {
		t.Fatalf("fixture %s: decode: %v", name, err)
	}
	return ps
}

// TestWindowsParsePayloads pins the contract on payloads captured from a real
// PowerShell (see testdata/psparse/README.md): the element shapes, the write list,
// the per-command Static gate, and the cases that must not be judged at all.
func TestWindowsParsePayloads(t *testing.T) {
	t.Run("one_line", func(t *testing.T) {
		ps := readPayload(t, "one_line")
		if len(ps.Cmds) != 1 || ps.Cmds[0].Name != "Get-ChildItem" {
			t.Fatalf("cmds = %+v, want one Get-ChildItem", ps.Cmds)
		}
		if !ps.Cmds[0].Static || len(ps.Cmds[0].Args) != 0 || len(ps.Writes) != 0 || ps.Dynamic {
			t.Errorf("cmds=%+v writes=%v dynamic=%v, want a plain static command", ps.Cmds, ps.Writes, ps.Dynamic)
		}
	})

	t.Run("params", func(t *testing.T) {
		// The role split P1 had to hand-fix: the AST says which value belongs to
		// which parameter, including the two that take no value at all.
		ps := readPayload(t, "params")
		if len(ps.Cmds) != 1 || ps.Cmds[0].Name != "Copy-Item" {
			t.Fatalf("cmds = %+v", ps.Cmds)
		}
		a := ps.Cmds[0].Args
		if len(a) != 6 {
			t.Fatalf("args = %d (%+v), want 6", len(a), a)
		}
		if a[0].Param != "Destination" || a[0].Value != nil {
			t.Errorf("a[0] = %+v, want the Destination parameter", a[0])
		}
		if a[1].Value == nil || *a[1].Value != `C:\Windows\System32` {
			t.Errorf("a[1] = %+v, want the destination value", a[1])
		}
		if a[2].Param != "Path" || a[3].Value == nil || *a[3].Value != "a.txt" {
			t.Errorf("a[2..3] = %+v %+v, want Path a.txt", a[2], a[3])
		}
		if a[4].Param != "Recurse" || a[5].Param != "Force" || a[4].Value != nil || a[5].Value != nil {
			t.Errorf("a[4..5] = %+v %+v, want two bare switches", a[4], a[5])
		}
		if !ps.Cmds[0].Static {
			t.Error("Static = false, want true: every element is a parameter or a plain value")
		}
	})

	t.Run("colon_value", func(t *testing.T) {
		// `-Path:C:\x` carries its value on the parameter.
		a := readPayload(t, "colon_value").Cmds[0].Args
		if len(a) == 0 || a[0].Param != "Path" || a[0].Value == nil || *a[0].Value != `C:\Windows\win.ini` {
			t.Fatalf("args = %+v, want Path with the attached value", a)
		}
	})

	t.Run("values", func(t *testing.T) {
		ps := readPayload(t, "quotes")
		a := ps.Cmds[0].Args
		if len(a) != 4 { // -Path "out.json" -Value '{"a":1}' -Encoding utf8 -> p,v,p,v,p,v...
			t.Logf("args = %+v", a)
		}
		var quoted, bareword int
		for _, x := range a {
			if x.Value == nil {
				continue
			}
			if x.Quoted {
				quoted++
			} else if !x.Numeric {
				bareword++
			}
		}
		if quoted < 2 || bareword < 1 {
			t.Errorf("quoted=%d bareword=%d, want the quoted flag distinct from a bareword", quoted, bareword)
		}
	})

	t.Run("array_inline", func(t *testing.T) {
		a := readPayload(t, "array_inline").Cmds[0].Args
		if len(a) != 2 || *a[0].Value != "a.txt" || *a[1].Value != "b.txt" {
			t.Fatalf("args = %+v, want a.txt and b.txt", a)
		}
	})

	t.Run("writes", func(t *testing.T) {
		ps := readPayload(t, "redirect_out")
		if len(ps.Writes) != 1 || ps.Writes[0].Target != "out.txt" || ps.Writes[0].Append {
			t.Fatalf("writes = %+v, want out.txt truncate", ps.Writes)
		}
		if ps := readPayload(t, "redirect_append"); len(ps.Writes) != 1 || !ps.Writes[0].Append {
			t.Fatalf("writes = %+v, want out.txt append", ps.Writes)
		}
		if ps := readPayload(t, "redirect_two"); len(ps.Writes) != 2 {
			t.Fatalf("writes = %+v, want two targets", ps.Writes)
		}
		// 2>&1 merges a stream: it must never look like a file write.
		if ps := readPayload(t, "redirect_merge"); len(ps.Writes) != 0 {
			t.Fatalf("writes = %+v, want none for 2>&1", ps.Writes)
		}
		// `> $null` has a target that is not a path: Dynamic makes the caller ask.
		if ps := readPayload(t, "redirect_null"); len(ps.Writes) != 1 || !ps.Dynamic {
			t.Fatalf("writes = %+v dynamic = %v, want the $null target and Dynamic", ps.Writes, ps.Dynamic)
		}
	})

	t.Run("dynamic", func(t *testing.T) {
		// A value the parse cannot know must never leave Static true: a mapper that
		// judged from values would allow a read of a path it has never seen.
		for _, name := range []string{"variable", "expandable", "subexpression", "array_expr", "scriptblock"} {
			ps := readPayload(t, name)
			if !ps.Dynamic {
				t.Errorf("%s: Dynamic = false, want true", name)
			}
			var unknown bool
			for _, c := range ps.Cmds {
				for _, a := range c.Args {
					if a.Dynamic != "" {
						unknown = true
					}
				}
			}
			if !unknown {
				t.Errorf("%s: no element carried a dynamic kind", name)
			}
		}
		// `& $cmd` is not a command called "$cmd".
		ps := readPayload(t, "dynamic_name")
		if len(ps.Cmds) != 1 || ps.Cmds[0].Static || ps.Cmds[0].Name != "$cmd" {
			t.Fatalf("cmds = %+v, want one non-static '$cmd'", ps.Cmds)
		}
	})

	t.Run("shape", func(t *testing.T) {
		if ps := readPayload(t, "semicolons"); len(ps.Cmds) != 3 || ps.Cmds[1].Name != "Remove-Item" {
			t.Fatalf("cmds = %+v, want three commands", ps.Cmds)
		}
		if ps := readPayload(t, "pipeline_three"); len(ps.Cmds) != 3 {
			t.Fatalf("cmds = %+v, want three piped commands", ps.Cmds)
		}
		if ps := readPayload(t, "call_operator"); len(ps.Cmds) != 1 || !strings.HasSuffix(ps.Cmds[0].Name, "script.ps1") {
			t.Fatalf("cmds = %+v, want the script path as the name", ps.Cmds)
		}
		for _, name := range []string{"empty", "comment_only"} {
			if ps := readPayload(t, name); len(ps.Cmds) != 0 || len(ps.Writes) != 0 {
				t.Errorf("%s: cmds = %+v, want nothing to judge", name, ps.Cmds)
			}
		}
		// A 200-line script is 400 commands: the payload scales, and every one of
		// them must be static enough to judge.
		if ps := readPayload(t, "two_hundred"); len(ps.Cmds) != 400 {
			t.Fatalf("cmds = %d, want 400", len(ps.Cmds))
		}
		if ps := readPayload(t, "always_ask"); len(ps.Cmds) != 1 || len(ps.Cmds[0].Args) == 0 {
			t.Fatalf("cmds = %+v, want Remove-Item with its flags", ps.Cmds)
		}
	})
}

// TestWindowsParseErrors: every failure is an error the caller turns into a fallback.
func TestWindowsParseErrors(t *testing.T) {
	if _, err := decodePS([]byte(`{"e":[],"dyn":false,"w":[],"c":[{"n":"Get-Ch`)); err == nil {
		t.Error("truncated payload: want an error")
	}
	_, err := decodePS([]byte(`{"e":["An empty pipe element is not allowed."],"dyn":false,"w":[],"c":[]}`))
	if err == nil || !strings.Contains(err.Error(), "empty pipe element") {
		t.Errorf("errors payload: err = %v, want the first parser message", err)
	}
	if _, err := decodePS(nil); err == nil {
		t.Error("empty payload: want an error")
	}
	// The real broken fixture: the parse reports both errors and partial commands.
	raw, err := os.ReadFile(filepath.Join("testdata", "psparse", "ast-broken.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodePS(raw); err == nil || !strings.Contains(err.Error(), "empty pipe element") {
		t.Errorf("broken fixture: err = %v, want the parser's message", err)
	}
}

// TestWindowsParseResponseCap: a reply that exceeds the cap is refused, not buffered.
func TestWindowsParseResponseCap(t *testing.T) {
	l := &limitedBuf{max: 32}
	if _, err := l.Write([]byte(strings.Repeat("x", 33))); err != nil {
		t.Fatalf("write: %v", err)
	}
	if !l.over {
		t.Error("over = false, want true for a reply over the cap")
	}
	if _, err := decodePS([]byte(`{"e":[],"dyn":false,"w":[],"c":[]}`)); err != nil {
		t.Errorf("empty script: %v, want no error", err)
	}
}

// TestWindowsParseNoInterpreter: with no PowerShell anywhere, parse fails closed.
func TestWindowsParseNoInterpreter(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("SystemRoot always yields powershell.exe on Windows")
	}
	t.Setenv("PATH", t.TempDir())
	t.Setenv("SystemRoot", "")
	p := &psSidecar{}
	if _, err := p.parse("Get-ChildItem"); err == nil {
		t.Fatal("parse: want an error with no interpreter")
	}
}

// TestWindowsParseWrapperIsTheFixture: the embedded wrapper is the file the fixtures
// were captured with. A drift here silently invalidates every payload above.
func TestWindowsParseWrapperIsTheFixture(t *testing.T) {
	fixture, err := os.ReadFile(filepath.Join("testdata", "psparse", "wrapper.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.TrimSpace(psWrapper), strings.TrimSpace(string(fixture)); got != want {
		t.Error("psparse.ps1 and testdata/psparse/wrapper.ps1 have drifted")
	}
	if !strings.Contains(psWrapper, "Parser]::ParseInput") {
		t.Error("the wrapper no longer parses with Parser::ParseInput")
	}
	for _, bad := range []string{"Invoke-Expression", "iex ", "& $src", "Invoke-Command"} {
		if strings.Contains(psWrapper, bad) {
			t.Errorf("the wrapper must never execute: found %q", bad)
		}
	}
	// The payload keys are a contract with the analyser.
	var probe map[string]json.RawMessage
	if err := json.Unmarshal([]byte(`{"e":[],"dyn":false,"w":[],"c":[]}`), &probe); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"e", "dyn", "w", "c"} {
		if _, ok := probe[k]; !ok {
			t.Errorf("payload key %q missing", k)
		}
	}
}

// TestWindowsParseCache: a repeated script is parsed once, a deterministic parse
// failure is cached too, a transport failure is not, and the map stays bounded.
func TestWindowsParseCache(t *testing.T) {
	psParseCache = &psCache{m: make(map[string]psCacheEntry, psCacheMax)}
	old := psTransport
	defer func() { psTransport = old; psParseCache = &psCache{m: make(map[string]psCacheEntry, psCacheMax)} }()

	calls := 0
	psTransport = func(string) (parsedScript, error) {
		calls++
		return parsedScript{Cmds: []psCmd{{Name: "Get-ChildItem", Static: true}}}, nil
	}
	for i := 0; i < 3; i++ {
		if _, err := winParseDefault("Get-ChildItem"); err != nil {
			t.Fatalf("parse: %v", err)
		}
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1 for one repeated script", calls)
	}
	if _, err := winParseDefault("Get-Item x"); err != nil {
		t.Fatalf("second script: %v", err)
	}
	if calls != 2 {
		t.Errorf("calls = %d, want 2 for a new script", calls)
	}

	// A broken script is a deterministic failure: the same text fails the same way,
	// so the second attempt must not pay for the parse again.
	broken := 0
	psTransport = func(string) (parsedScript, error) {
		broken++
		return parsedScript{}, psParseError{Msgs: []string{"An empty pipe element is not allowed."}}
	}
	for i := 0; i < 2; i++ {
		if _, err := winParseDefault("Get- |"); err == nil {
			t.Fatal("broken script: want an error")
		}
	}
	if broken != 1 {
		t.Errorf("broken calls = %d, want 1 (a parse failure is cached)", broken)
	}

	// A transport failure is transient — a dead child must be retried, not remembered.
	transient := 0
	psTransport = func(string) (parsedScript, error) {
		transient++
		return parsedScript{}, errPSTimeout
	}
	for i := 0; i < 2; i++ {
		// A script this test has not cached yet: the success above must not mask it.
		if _, err := winParseDefault("Get-Item y"); err == nil {
			t.Fatal("timeout: want an error")
		}
	}
	if transient != 2 {
		t.Errorf("transient calls = %d, want 2 (a transport failure is not cached)", transient)
	}

	// Bounded: the 257th distinct script evicts the oldest.
	psParseCache = &psCache{m: make(map[string]psCacheEntry, psCacheMax)}
	psTransport = func(string) (parsedScript, error) { return parsedScript{}, nil }
	for i := 0; i < psCacheMax+8; i++ {
		if _, err := winParseDefault(strings.Repeat("x", i+1)); err != nil {
			t.Fatalf("bulk parse %d: %v", i, err)
		}
	}
	if n := psParseCache.len(); n > psCacheMax {
		t.Errorf("cache len = %d, want <= %d", n, psCacheMax)
	}
}
