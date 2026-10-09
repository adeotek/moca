package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestParseVersion(t *testing.T) {
	cases := []struct {
		in    string
		core  [3]int
		pre   []string
		dev   int
		dirty bool
		err   bool
	}{
		{in: "v0.1.1", core: [3]int{0, 1, 1}},
		{in: "0.1.1", core: [3]int{0, 1, 1}},
		{in: "v1.2", core: [3]int{1, 2, 0}},
		{in: "v0.1.0-alpha", core: [3]int{0, 1, 0}, pre: []string{"alpha"}},
		{in: "v0.1.1-alpha.2", core: [3]int{0, 1, 1}, pre: []string{"alpha", "2"}},
		{in: "v0.1.1-alpha-2-g0b9528d", core: [3]int{0, 1, 1}, pre: []string{"alpha"}, dev: 2},
		{in: "v0.1.1-2-g0b9528d", core: [3]int{0, 1, 1}, dev: 2},
		{in: "v0.1.1-2-g0b9528d-dirty", core: [3]int{0, 1, 1}, dev: 2, dirty: true},
		{in: "v0.1.1-dirty", core: [3]int{0, 1, 1}, dirty: true},
		{in: "0.0.0-dev", core: [3]int{0, 0, 0}, pre: []string{"dev"}},
		{in: "0b9528d", err: true},
		{in: "", err: true},
		{in: "v1.2.3.4", err: true},
		{in: "v1.x.0", err: true},
		{in: "v0.1.1-", err: true},
	}
	for _, c := range cases {
		v, err := ParseVersion(c.in)
		if c.err {
			if err == nil {
				t.Errorf("ParseVersion(%q): want error, got %+v", c.in, v)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseVersion(%q): %v", c.in, err)
			continue
		}
		if v.Core != c.core || v.Dev != c.dev || v.Dirty != c.dirty || !slices.Equal(v.Pre, c.pre) {
			t.Errorf("ParseVersion(%q) = core %v pre %v dev %d dirty %v, want core %v pre %v dev %d dirty %v",
				c.in, v.Core, v.Pre, v.Dev, v.Dirty, c.core, c.pre, c.dev, c.dirty)
		}
	}
}

func TestCompare(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"v0.1.1", "v0.1.1", 0},
		{"v0.1.2", "v0.1.1", 1},
		{"v0.1.1", "v0.1.2", -1},
		{"v1.0.0", "v0.9.9", 1},
		{"v0.1.1", "v0.1.1-alpha", 1},             // a release beats any prerelease
		{"v0.1.1-alpha", "v0.1.1-alpha.1", -1},    // shorter prerelease list is lower
		{"v0.1.1-alpha.2", "v0.1.1-alpha.10", -1}, // numeric identifiers compare numerically
		{"v0.1.1-beta", "v0.1.1-alpha", 1},
		{"v0.1.1-alpha", "v0.1.1-1", 1}, // alphanumeric > numeric
		{"v0.1.2", "v0.1.1-9-gabc1234", 1},
		{"v0.1.1", "v0.1.1-2-g0b9528d", 0}, // dev suffix does not affect the order
	}
	for _, c := range cases {
		va, err := ParseVersion(c.a)
		if err != nil {
			t.Fatalf("ParseVersion(%q): %v", c.a, err)
		}
		vb, err := ParseVersion(c.b)
		if err != nil {
			t.Fatalf("ParseVersion(%q): %v", c.b, err)
		}
		if got := Compare(va, vb); got != c.want {
			t.Errorf("Compare(%s, %s) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestFindAsset(t *testing.T) {
	rel := Release{Tag: "v9.9.9", Assets: []Asset{
		{Name: "moca-v9.9.9-linux-amd64.tar.gz"},
		{Name: "moca-v9.9.9-linux-arm64.tar.gz"},
		{Name: "moca-v9.9.9-windows-amd64.zip"},
		{Name: "checksums.txt"},
	}}
	for _, c := range []struct{ goos, goarch, want string }{
		{"linux", "amd64", "moca-v9.9.9-linux-amd64.tar.gz"},
		{"linux", "arm64", "moca-v9.9.9-linux-arm64.tar.gz"},
		{"windows", "amd64", "moca-v9.9.9-windows-amd64.zip"},
	} {
		a, err := FindAsset(rel, c.goos, c.goarch)
		if err != nil || a.Name != c.want {
			t.Errorf("FindAsset(%s/%s) = %q, %v; want %q", c.goos, c.goarch, a.Name, err, c.want)
		}
	}
	if _, err := FindAsset(rel, "darwin", "arm64"); err == nil || !strings.Contains(err.Error(), "no package for darwin/arm64") {
		t.Errorf("missing platform: err = %v", err)
	}
	// The asset name's tag may lag the release tag; a suffix match covers it.
	lag := Release{Tag: "v9.9.9", Assets: []Asset{{Name: "moca-v9.9.8-linux-amd64.tar.gz"}}}
	if a, err := FindAsset(lag, "linux", "amd64"); err != nil || a.Name != "moca-v9.9.8-linux-amd64.tar.gz" {
		t.Errorf("fallback match = %q, %v", a.Name, err)
	}
}

func TestParseChecksums(t *testing.T) {
	sums := ParseChecksums([]byte("abc123  moca-1.tar.gz\nDEF456 *moca-2.zip\n\nnot a checksum line\n"))
	want := map[string]string{"moca-1.tar.gz": "abc123", "moca-2.zip": "def456"}
	if len(sums) != len(want) {
		t.Fatalf("parsed %v, want %v", sums, want)
	}
	for k, v := range want {
		if sums[k] != v {
			t.Errorf("%s = %q, want %q", k, sums[k], v)
		}
	}
}

func tarGz(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, content := range files {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(content))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func zipFile(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestExtractBinary(t *testing.T) {
	want := "#!/bin/sh\necho moca\n"
	tgz := tarGz(t, map[string]string{"moca": want, "LICENSE": "MIT"})
	got, err := ExtractBinary(tgz, "moca-v9.9.9-linux-amd64.tar.gz")
	if err != nil || string(got) != want {
		t.Fatalf("tar.gz extract = %q, %v", got, err)
	}
	// A directory prefix in the archive is tolerated.
	prefixed := tarGz(t, map[string]string{"dist/pkg/moca": want})
	if got, err := ExtractBinary(prefixed, "moca-v9.9.9-linux-amd64.tar.gz"); err != nil || string(got) != want {
		t.Fatalf("prefixed tar.gz extract = %q, %v", got, err)
	}
	z := zipFile(t, map[string]string{"moca.exe": want, "LICENSE": "MIT"})
	if got, err := ExtractBinary(z, "moca-v9.9.9-windows-amd64.zip"); err != nil || string(got) != want {
		t.Fatalf("zip extract = %q, %v", got, err)
	}
	if _, err := ExtractBinary(tgz, "moca-v9.9.9-windows-amd64.zip"); err == nil {
		t.Fatal("tar.gz mislabelled as zip must fail")
	}
	if _, err := ExtractBinary(tarGz(t, map[string]string{"README": "x"}), "moca-v9.9.9-linux-amd64.tar.gz"); err == nil {
		t.Fatal("archive without the binary must fail")
	}
	if _, err := ExtractBinary(tgz, "moca-v9.9.9-linux-amd64.rar"); err == nil {
		t.Fatal("unsupported format must fail")
	}
}

func TestLatest(t *testing.T) {
	var gotAuth, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/adeotek/moca/releases" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		gotAuth, gotQuery = r.Header.Get("Authorization"), r.URL.RawQuery
		fmt.Fprint(w, `[{"tag_name":"v10.0.0","draft":true},{"tag_name":"v9.9.9","prerelease":true,"assets":[{"name":"moca-v9.9.9-linux-amd64.tar.gz","browser_download_url":"http://x/pkg","size":12}]}]`)
	}))
	defer srv.Close()
	cl := Client{APIBase: srv.URL, Token: "t0ken"}
	rel, err := cl.Latest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if rel.Tag != "v9.9.9" || len(rel.Assets) != 1 || rel.Assets[0].Name != "moca-v9.9.9-linux-amd64.tar.gz" {
		t.Fatalf("release = %+v", rel)
	}
	if gotAuth != "Bearer t0ken" {
		t.Errorf("Authorization = %q", gotAuth)
	}
	if gotQuery != "per_page=10" {
		t.Errorf("query = %q", gotQuery)
	}
}

func TestLatestErrors(t *testing.T) {
	empty := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[]`)
	}))
	defer empty.Close()
	if _, err := (Client{APIBase: empty.URL}).Latest(context.Background()); err == nil || !strings.Contains(err.Error(), "no releases published") {
		t.Errorf("empty repo: err = %v", err)
	}

	limited := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("x-ratelimit-remaining", "0")
		w.WriteHeader(http.StatusForbidden)
	}))
	defer limited.Close()
	if _, err := (Client{APIBase: limited.URL}).Latest(context.Background()); err == nil || !strings.Contains(err.Error(), "rate limit") {
		t.Errorf("rate limited: err = %v", err)
	}

	broken := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer broken.Close()
	if _, err := (Client{APIBase: broken.URL}).Latest(context.Background()); err == nil || !strings.Contains(err.Error(), "500") {
		t.Errorf("server error: err = %v", err)
	}
}

// installServer serves one package + its checksums file.
func installServer(t *testing.T, pkg []byte, sums string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/pkg":
			w.Write(pkg)
		case "/sums":
			fmt.Fprint(w, sums)
		default:
			http.NotFound(w, r)
		}
	}))
}

func TestInstallReplacesBinary(t *testing.T) {
	newBin := []byte("#!/bin/sh\necho new\n")
	pkg := tarGz(t, map[string]string{"moca": string(newBin), "LICENSE": "MIT"})
	sum := sha256.Sum256(pkg)
	sums := hex.EncodeToString(sum[:]) + "  moca-v9.9.9-linux-amd64.tar.gz\n"
	srv := installServer(t, pkg, sums)
	defer srv.Close()

	dir := t.TempDir()
	exe := filepath.Join(dir, "moca")
	if err := os.WriteFile(exe, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	rel := Release{Tag: "v9.9.9", Assets: []Asset{
		{Name: "moca-v9.9.9-linux-amd64.tar.gz", URL: srv.URL + "/pkg"},
		{Name: "checksums.txt", URL: srv.URL + "/sums"},
	}}
	verified, err := (Client{APIBase: srv.URL}).Install(context.Background(), rel, rel.Assets[0], exe)
	if err != nil {
		t.Fatal(err)
	}
	if !verified {
		t.Fatal("checksum should have been verified")
	}
	got, err := os.ReadFile(exe)
	if err != nil || !bytes.Equal(got, newBin) {
		t.Fatalf("replaced binary = %q, %v", got, err)
	}
	if fi, err := os.Stat(exe); err != nil || fi.Mode().Perm()&0o111 == 0 {
		t.Fatalf("mode = %v, %v", fi.Mode(), err)
	}
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(ents) != 1 {
		t.Fatalf("leftover files after install: %v", ents)
	}
}

func TestInstallChecksumMismatch(t *testing.T) {
	pkg := tarGz(t, map[string]string{"moca": "new"})
	srv := installServer(t, pkg, strings.Repeat("ab", 32)+"  moca-v9.9.9-linux-amd64.tar.gz\n")
	defer srv.Close()
	dir := t.TempDir()
	exe := filepath.Join(dir, "moca")
	if err := os.WriteFile(exe, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	rel := Release{Tag: "v9.9.9", Assets: []Asset{
		{Name: "moca-v9.9.9-linux-amd64.tar.gz", URL: srv.URL + "/pkg"},
		{Name: "checksums.txt", URL: srv.URL + "/sums"},
	}}
	if _, err := (Client{}).Install(context.Background(), rel, rel.Assets[0], exe); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("mismatch: err = %v", err)
	}
	if got, _ := os.ReadFile(exe); string(got) != "old" {
		t.Fatalf("binary was replaced despite a bad checksum: %q", got)
	}
}

func TestInstallWithoutChecksums(t *testing.T) {
	pkg := tarGz(t, map[string]string{"moca": "new"})
	srv := installServer(t, pkg, "")
	defer srv.Close()
	dir := t.TempDir()
	exe := filepath.Join(dir, "moca")
	if err := os.WriteFile(exe, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	rel := Release{Tag: "v9.9.9", Assets: []Asset{{Name: "moca-v9.9.9-linux-amd64.tar.gz", URL: srv.URL + "/pkg"}}}
	verified, err := (Client{}).Install(context.Background(), rel, rel.Assets[0], exe)
	if err != nil {
		t.Fatal(err)
	}
	if verified {
		t.Fatal("nothing to verify, yet verified=true")
	}
	if got, _ := os.ReadFile(exe); string(got) != "new" {
		t.Fatalf("binary = %q", got)
	}
}

func TestInstallChecksumEntryMissing(t *testing.T) {
	pkg := tarGz(t, map[string]string{"moca": "new"})
	srv := installServer(t, pkg, "ab  other-file.tar.gz\n")
	defer srv.Close()
	dir := t.TempDir()
	exe := filepath.Join(dir, "moca")
	if err := os.WriteFile(exe, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	rel := Release{Tag: "v9.9.9", Assets: []Asset{
		{Name: "moca-v9.9.9-linux-amd64.tar.gz", URL: srv.URL + "/pkg"},
		{Name: "checksums.txt", URL: srv.URL + "/sums"},
	}}
	if _, err := (Client{}).Install(context.Background(), rel, rel.Assets[0], exe); err == nil || !strings.Contains(err.Error(), "no entry") {
		t.Fatalf("missing entry: err = %v", err)
	}
}

func TestIsTemporary(t *testing.T) {
	if !IsTemporary(filepath.Join(t.TempDir(), "moca")) {
		t.Fatal("a binary under the temp dir must be reported temporary")
	}
	if IsTemporary("/usr/local/bin/moca") {
		t.Fatal("/usr/local/bin is not temporary")
	}
}
