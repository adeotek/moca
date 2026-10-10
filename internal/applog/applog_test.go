package applog

import (
	"bytes"
	"errors"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func keepDefault(t *testing.T) {
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
}

func TestParseLevel(t *testing.T) {
	for in, want := range map[string]Level{"": LevelInfo, "info": LevelInfo, "DEBUG": LevelDebug, " off ": LevelOff} {
		if got, err := ParseLevel(in); err != nil || got != want {
			t.Fatalf("%q: %v %v", in, got, err)
		}
	}
	if _, err := ParseLevel("trace"); err == nil || !strings.Contains(err.Error(), "want info|debug|off") {
		t.Fatalf("invalid level: %v", err)
	}
}

func TestSegments(t *testing.T) {
	cases := map[string][]string{
		"api_key":       {"api", "key"},
		"oauthToken":    {"oauth", "token"},
		"APIKey":        {"apikey"},
		"tokens_before": {"tokens", "before"},
		"usage.in":      {"usage", "in"},
		"x-api-key":     {"x", "api", "key"},
	}
	for in, want := range cases {
		if got := segments(in); !slices.Equal(got, want) {
			t.Errorf("segments(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestRedaction(t *testing.T) {
	var b bytes.Buffer
	l := slog.New(slog.NewTextHandler(&b, &slog.HandlerOptions{ReplaceAttr: replaceAttr}))
	u, _ := url.Parse("https://user:pw@api.example.com/v1/x?token=abc#frag")
	l.Info("m", "api_key", "s1", "Authorization", "s2", "oauthToken", "s3", slog.Group("apikey", "v", "s4"),
		"x-api-key", "s5", "tokens_before", 1234, slog.Group("usage", "in", 5), "keyed", true,
		"url", u, "err", errors.New(strings.Repeat("e", 900)))
	out := b.String()
	for _, leak := range []string{"=s1", "=s2", "=s3", "=s4", "=s5", "user", "pw", "token=abc", "frag"} {
		if strings.Contains(out, leak) {
			t.Errorf("leaked %q: %s", leak, out)
		}
	}
	for _, keep := range []string{"tokens_before=1234", "usage.in=5", "keyed=true", "url=https://api.example.com/v1/x", "[redacted]"} {
		if !strings.Contains(out, keep) {
			t.Errorf("missing %q: %s", keep, out)
		}
	}
	if strings.Contains(out, strings.Repeat("e", errMax+1)) || !strings.Contains(out, strings.Repeat("e", errMax)) {
		t.Errorf("error text not cut to %d bytes", errMax)
	}
}

// wc is an in-memory WriteCloser that counts writes and can fail them.
type wc struct {
	bytes.Buffer
	writes int
	fail   bool
}

func (w *wc) Write(p []byte) (int, error) {
	w.writes++
	if w.fail {
		return 0, errors.New("disk full")
	}
	return w.Buffer.Write(p)
}
func (w *wc) Close() error { return nil }

func TestCappedWriterRecordAndFileCaps(t *testing.T) {
	old := recordMax
	recordMax = 32
	t.Cleanup(func() { recordMax = old })
	w := &wc{}
	c := &cappedWriter{w: w, max: 100}
	if n, err := c.Write([]byte(strings.Repeat("é", 40) + "\n")); n != 81 || err != nil {
		t.Fatalf("Write must report the full length and no error: %d %v", n, err)
	}
	first := w.String()
	if len(first) > 32 || !strings.HasSuffix(first, "\n") || !strings.Contains(first, "[cut]") {
		t.Fatalf("record cap: %q", first)
	}
	for range 10 {
		c.Write([]byte("0123456789012345678901234\n"))
	}
	out := w.String()
	if strings.Count(out, "log truncated: size cap reached") != 1 {
		t.Fatalf("want exactly one truncation line: %q", out)
	}
	if !strings.HasSuffix(out, "size cap reached\"\n") {
		t.Fatalf("nothing may follow the truncation line: %q", out)
	}
}

func TestCappedWriterWriteErrorTurnsOff(t *testing.T) {
	w := &wc{fail: true}
	c := &cappedWriter{w: w, max: 1 << 20}
	for range 3 {
		if n, err := c.Write([]byte("x\n")); n != 2 || err != nil {
			t.Fatalf("errors must never reach slog: %d %v", n, err)
		}
	}
	if w.writes != 1 {
		t.Fatalf("a write error turns logging off: %d writes", w.writes)
	}
}

func TestOpenWritesAtLevel(t *testing.T) {
	keepDefault(t)
	dir := filepath.Join(t.TempDir(), "logs")
	c, path, err := Open(dir, LevelInfo, 30)
	if err != nil {
		t.Fatal(err)
	}
	slog.Info("hello", "n", 1)
	slog.Debug("hidden")
	c.Close()
	slog.Info("after close") // dropped, no panic
	want := time.Now().Format("2006-01-02") + "-"
	if filepath.Dir(path) != dir || !strings.HasPrefix(filepath.Base(path), want) || filepath.Ext(path) != ".log" {
		t.Fatalf("path %q", path)
	}
	b, _ := os.ReadFile(path)
	if !strings.Contains(string(b), "msg=hello n=1") || strings.Contains(string(b), "hidden") || strings.Contains(string(b), "after close") {
		t.Fatalf("content: %s", b)
	}

	c, path, err = Open(dir, LevelDebug, 30)
	if err != nil {
		t.Fatal(err)
	}
	slog.Debug("shown")
	c.Close()
	if b, _ := os.ReadFile(path); !strings.Contains(string(b), "msg=shown") {
		t.Fatalf("debug level: %s", b)
	}
}

func TestOpenOffTouchesNothing(t *testing.T) {
	keepDefault(t)
	dir := filepath.Join(t.TempDir(), "logs")
	c, path, err := Open(dir, LevelOff, 30)
	if err != nil || path != "" || c == nil {
		t.Fatalf("%v %q %v", c, path, err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("off must not create the directory")
	}
}

func TestOpenUnwritableKeepsDefault(t *testing.T) {
	keepDefault(t)
	Discard()
	before := slog.Default().Handler()
	f := filepath.Join(t.TempDir(), "file")
	os.WriteFile(f, nil, 0o600)
	if _, _, err := Open(filepath.Join(f, "logs"), LevelInfo, 30); err == nil {
		t.Fatal("a dir under a regular file cannot be created")
	}
	if slog.Default().Handler() != before {
		t.Fatal("a failed Open must leave the default handler alone")
	}
}

func TestPrune(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	old := now.AddDate(0, 0, -40)
	for name, mt := range map[string]time.Time{"old.log": old, "fresh.log": now, "old.txt": old} {
		p := filepath.Join(dir, name)
		os.WriteFile(p, nil, 0o600)
		os.Chtimes(p, mt, mt)
	}
	prune(dir, 0, now)
	if _, err := os.Stat(filepath.Join(dir, "old.log")); err != nil {
		t.Fatal("days<=0 keeps everything")
	}
	prune(dir, 30, now)
	for name, gone := range map[string]bool{"old.log": true, "fresh.log": false, "old.txt": false} {
		_, err := os.Stat(filepath.Join(dir, name))
		if gone != os.IsNotExist(err) {
			t.Errorf("%s: gone=%v err=%v", name, gone, err)
		}
	}
}
