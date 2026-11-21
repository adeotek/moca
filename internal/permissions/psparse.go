// Package permissions: the Windows shell analyser's PowerShell parse (Layer D).
//
// The word model in shell_windows_rules.go guesses at quoting, parameter binding
// and redirection. PowerShell's own parser knows all three, so on Windows the
// analyser asks it: one long-lived child process per run reads a base64 script on
// stdin and answers one JSON line describing every command, its parameters and
// values, and every file redirection.
//
// The parse never executes anything: the wrapper only calls Parser::ParseInput
// (proven by the Task 13 spike, see testdata/psparse/README.md). The script text is
// base64 on stdin and never interpolated into a command line, so a command cannot
// break out into the wrapper.
//
// Every failure path returns an error, and the caller falls back to the word model.
// Nothing here decides a permission.
package permissions

import (
	"bufio"
	"bytes"
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf16"
)

//go:embed psparse.ps1
var psWrapper string

const (
	// psTimeout bounds one request. The spike measured 41-67 ms on a warm sidecar
	// and ~400 ms for a 200-line script through a fresh child; 5 s is a wedge, not
	// a slow parse.
	psTimeout = 5 * time.Second
	// psMaxResponse caps one response. A 200-line script with 400 commands is 25 KiB,
	// so 1 MiB is four orders of magnitude of headroom for a script the analyser
	// should refuse to reason about anyway.
	psMaxResponse = 1 << 20
)

var (
	errPSNoInterpreter = errors.New("powershell parse: no pwsh or powershell.exe")
	errPSTooLarge      = errors.New("powershell parse: response over 1 MiB")
	errPSTimeout       = errors.New("powershell parse: timed out")
)

// psArg is one command element, as the wrapper emits it: a parameter (with a value
// only for `-Param:value`), a plain value, or an element whose value is unknowable.
type psArg struct {
	Param   string  `json:"p"` // parameter name without the leading `-`
	Value   *string `json:"v"` // value, for a parameter and for a bare word
	Quoted  bool    `json:"q"` // not a bareword: the user quoted it
	Numeric bool    `json:"n"` // a numeric constant
	Dynamic string  `json:"d"` // the AST node kind whose value cannot be known
}

// psWrite is a file redirection target: `>` and `>>` only. A stream merge (`2>&1`)
// is not a write and does not appear here.
type psWrite struct {
	Target string `json:"t"`
	Append bool   `json:"a"`
}

// psCmd is one parsed command. Static is the mapper's gate: false means at least one
// element was not a plain parameter or plain constant, so the command must never be
// judged from its values.
type psCmd struct {
	Name   string  `json:"n"`
	Static bool    `json:"k"`
	Args   []psArg `json:"a"`
}

// psPayload is the wrapper's reply.
type psPayload struct {
	Errors  []string  `json:"e"`
	Dynamic bool      `json:"dyn"`
	Writes  []psWrite `json:"w"`
	Cmds    []psCmd   `json:"c"`
}

// parsedScript is the analyser's view of one shell command string.
type parsedScript struct {
	Cmds    []psCmd
	Writes  []psWrite
	Dynamic bool // some scriptblock/subexpression/array/variable exists: ask once
}

// winParse parses a PowerShell command string. It is a variable so tests (and the
// non-Windows suite) can replace it.
var winParse = func(script string) (parsedScript, error) {
	return psDefault.parse(script)
}

// closeWinParser ends the parser child. Call it when a run ends; the child also dies
// on its own when stdin closes with the process, so this is tidiness, not a leak fix.
func closeWinParser() { psDefault.Close() }

var psDefault = &psSidecar{}

// psSidecar is one long-lived PowerShell child, plus a child-per-call fallback for
// when it cannot be kept alive. The spike says the difference is 293 ms vs 67 ms on
// PowerShell 5.1, which is why the child is kept.
type psSidecar struct {
	mu     sync.Mutex
	cmd    *exec.Cmd
	in     io.WriteCloser
	out    *bufio.Reader
	path   string // resolved interpreter, "" until the first lookup
	closed bool
}

// parse answers from the sidecar, restarting it once if it broke, and falls back to a
// fresh child per call rather than failing the analyser.
func (p *psSidecar) parse(script string) (parsedScript, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	var first error
	if !p.closed {
		for attempt := 0; attempt < 2; attempt++ {
			if p.cmd == nil {
				if err := p.start(); err != nil {
					first = err
					break
				}
			}
			raw, err := p.request(script)
			if err == nil {
				return decodePS(raw)
			}
			if first == nil {
				first = err
			}
			p.stop() // a wedged child must not poison the next call
		}
	}

	raw, err := p.once(script)
	if err == nil {
		return decodePS(raw)
	}
	if first != nil {
		return parsedScript{}, first
	}
	return parsedScript{}, err
}

// request writes one base64 line and reads one JSON line, giving up after psTimeout.
// A timeout leaves the child possibly wedged, so the caller stops it.
func (p *psSidecar) request(script string) ([]byte, error) {
	line := base64.StdEncoding.EncodeToString([]byte(script))
	if _, err := io.WriteString(p.in, line+"\n"); err != nil {
		return nil, fmt.Errorf("powershell parse: write: %w", err)
	}
	type reply struct {
		b   []byte
		err error
	}
	ch := make(chan reply, 1)
	go func() {
		b, err := p.out.ReadBytes('\n')
		ch <- reply{b, err}
	}()
	select {
	case r := <-ch:
		if r.err != nil {
			return nil, fmt.Errorf("powershell parse: read: %w", r.err)
		}
		if len(r.b) > psMaxResponse {
			return nil, errPSTooLarge
		}
		return bytes.TrimSpace(r.b), nil
	case <-time.After(psTimeout):
		return nil, errPSTimeout
	}
}

// once runs one child for one parse: the fallback shape the spike measured at
// ~300 ms. It is also what a caller gets after Close.
func (p *psSidecar) once(script string) ([]byte, error) {
	exe, err := p.interpreter()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), psTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe, psArgs()...)
	cmd.Stdin = strings.NewReader(base64.StdEncoding.EncodeToString([]byte(script)))
	out := &limitedBuf{max: psMaxResponse}
	cmd.Stdout = out
	cmd.Stderr = io.Discard // the wrapper reports failures in the payload
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("powershell parse: %w", err)
	}
	if out.over {
		return nil, errPSTooLarge
	}
	return bytes.TrimSpace(out.buf.Bytes()), nil
}

// start launches the sidecar child.
func (p *psSidecar) start() error {
	exe, err := p.interpreter()
	if err != nil {
		return err
	}
	cmd := exec.Command(exe, psArgs()...)
	in, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("powershell parse: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("powershell parse: %w", err)
	}
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("powershell parse: start: %w", err)
	}
	p.cmd, p.in, p.out = cmd, in, bufio.NewReaderSize(stdout, 1<<16)
	return nil
}

// stop ends the child. Closing stdin is the tidy path — the wrapper's
// [Console]::In.ReadLine() returns null and the loop ends — with a kill for a child
// that does not take the hint.
func (p *psSidecar) stop() {
	if p.in != nil {
		_ = p.in.Close()
	}
	if p.cmd != nil {
		done := make(chan struct{})
		go func() {
			_ = p.cmd.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(time.Second):
			if p.cmd.Process != nil {
				_ = p.cmd.Process.Kill()
			}
			<-done
		}
	}
	p.cmd, p.in, p.out = nil, nil, nil
}

// Close ends the sidecar for good: later parses use a child per call.
func (p *psSidecar) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closed = true
	p.stop()
}

// interpreter resolves the PowerShell to run, once. pwsh first (7 runs only what it
// is given), then the in-box 5.1 — and 5.1 by absolute path as a last resort, because
// a Store-installed pwsh is a user-scoped WindowsApps alias that may not resolve for
// another account or session.
func (p *psSidecar) interpreter() (string, error) {
	if p.path != "" {
		return p.path, nil
	}
	for _, name := range []string{"pwsh", "powershell.exe"} {
		if exe, err := exec.LookPath(name); err == nil {
			p.path = exe
			return exe, nil
		}
	}
	if root := os.Getenv("SystemRoot"); root != "" {
		exe := filepath.Join(root, "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
		if st, err := os.Stat(exe); err == nil && !st.IsDir() {
			p.path = exe
			return exe, nil
		}
	}
	return "", errPSNoInterpreter
}

// psArgs is the wrapper invocation: no profile, no interaction, and the script itself
// passed as -EncodedCommand so nothing about the command string can reach its argv.
func psArgs() []string {
	return []string{"-NoProfile", "-NonInteractive", "-EncodedCommand", encodeCommand(psWrapper)}
}

// encodeCommand is -EncodedCommand's expected base64 of UTF-16LE.
func encodeCommand(s string) string {
	u := utf16.Encode([]rune(s))
	b := make([]byte, 2*len(u))
	for i, r := range u {
		binary.LittleEndian.PutUint16(b[2*i:], r)
	}
	return base64.StdEncoding.EncodeToString(b)
}

// decodePS turns a wrapper reply into a parsedScript. An error payload is an error:
// the analyser must fall back rather than judge a script it could only partly read.
func decodePS(raw []byte) (parsedScript, error) {
	if len(raw) == 0 {
		return parsedScript{}, errors.New("powershell parse: empty response")
	}
	var pl psPayload
	if err := json.Unmarshal(raw, &pl); err != nil {
		return parsedScript{}, fmt.Errorf("powershell parse: %w", err)
	}
	if len(pl.Errors) > 0 {
		return parsedScript{}, fmt.Errorf("powershell parse: %s", strings.Join(pl.Errors, "; "))
	}
	return parsedScript{Cmds: pl.Cmds, Writes: pl.Writes, Dynamic: pl.Dynamic}, nil
}

// limitedBuf stops a runaway reply from being buffered in full.
type limitedBuf struct {
	buf  bytes.Buffer
	max  int
	over bool
}

func (l *limitedBuf) Write(b []byte) (int, error) {
	if l.buf.Len()+len(b) > l.max {
		l.over = true
		return 0, nil // swallow: the caller reports the failure once the child exits
	}
	return l.buf.Write(b)
}
