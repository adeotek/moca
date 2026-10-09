// Package update implements `moca update`: fetch the latest GitHub release
// and replace the running binary with the published package for this
// platform. Stdlib only and no internal imports — updating the binary must
// work even when the local configuration is broken.
package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"cmp"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	// DefaultAPIBase is the GitHub REST endpoint; overridable for tests and
	// mirrors.
	DefaultAPIBase = "https://api.github.com"
	// DefaultRepo is the repository releases are fetched from.
	DefaultRepo = "adeotek/moca"
	// checksumAsset is the release asset carrying sha256 sums, one
	// "«hex»  «name»" line per package.
	checksumAsset = "checksums.txt"
	// maxPackage caps a downloaded asset (and the extracted binary).
	maxPackage = 64 << 20
)

// Asset is one file attached to a GitHub release.
type Asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
	Size int64  `json:"size"`
}

// Release is the subset of the GitHub release payload the updater needs.
type Release struct {
	Tag        string  `json:"tag_name"`
	Name       string  `json:"name"`
	HTMLURL    string  `json:"html_url"`
	Prerelease bool    `json:"prerelease"`
	Assets     []Asset `json:"assets"`
}

// Version is a parsed git-describe version stamp:
// vMAJOR.MINOR.PATCH[-PRE][-N-gSHA][-dirty].
type Version struct {
	Core  [3]int
	Pre   []string // semver prerelease identifiers ("alpha", "2")
	Dev   int      // commits past the tag (git describe's -N-g… suffix)
	Dirty bool     // built with local changes (-dirty)
	Raw   string
}

// devRe matches git describe's "-N-gSHA" suffix (N commits past the tag).
var devRe = regexp.MustCompile(`^(.*)-(\d+)-g[0-9a-fA-F]+$`)

// ParseVersion parses a git-describe version stamp. A bare commit hash
// (git describe --always with no tags in reach) is an error — there is
// nothing to compare it with.
func ParseVersion(s string) (Version, error) {
	v := Version{Raw: s}
	t := strings.TrimSuffix(s, "-dirty")
	if t != s {
		v.Dirty = true
	}
	if m := devRe.FindStringSubmatch(t); m != nil {
		n, err := strconv.Atoi(m[2])
		if err != nil {
			return Version{}, fmt.Errorf("cannot parse version %q", s)
		}
		v.Dev, t = n, m[1]
	}
	t = strings.TrimPrefix(t, "v")
	if i := strings.IndexByte(t, '-'); i >= 0 {
		pre := t[i+1:]
		if pre == "" {
			return Version{}, fmt.Errorf("cannot parse version %q", s)
		}
		v.Pre = strings.Split(pre, ".")
		t = t[:i]
	}
	parts := strings.Split(t, ".")
	if len(parts) == 0 || len(parts) > 3 {
		return Version{}, fmt.Errorf("cannot parse version %q", s)
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return Version{}, fmt.Errorf("cannot parse version %q", s)
		}
		v.Core[i] = n
	}
	return v, nil
}

// Compare orders a and b by semver rules on core + prerelease. The
// git-describe dev/dirty flags do not affect the order — they are reported
// separately so the caller can phrase the outcome.
func Compare(a, b Version) int {
	for i := range a.Core {
		if c := cmp.Compare(a.Core[i], b.Core[i]); c != 0 {
			return c
		}
	}
	return comparePre(a.Pre, b.Pre)
}

// comparePre orders prerelease identifier lists per semver (§11): a release
// (no prerelease) is greater than any prerelease; identifiers compare
// numerically when both are numeric, alphanumerically otherwise, and a
// numeric identifier is always lower than an alphanumeric one.
func comparePre(a, b []string) int {
	switch {
	case len(a) == 0 && len(b) == 0:
		return 0
	case len(a) == 0:
		return 1
	case len(b) == 0:
		return -1
	}
	for i := 0; i < len(a) && i < len(b); i++ {
		an, aerr := strconv.Atoi(a[i])
		bn, berr := strconv.Atoi(b[i])
		switch {
		case aerr == nil && berr == nil:
			if c := cmp.Compare(an, bn); c != 0 {
				return c
			}
		case aerr == nil:
			return -1
		case berr == nil:
			return 1
		default:
			if c := strings.Compare(a[i], b[i]); c != 0 {
				return c
			}
		}
	}
	return cmp.Compare(len(a), len(b))
}

// Client talks to the GitHub releases API.
type Client struct {
	HTTP    *http.Client // optional; a 60s-timeout client is the default
	APIBase string       // optional; DefaultAPIBase
	Repo    string       // optional; DefaultRepo
	Token   string       // optional (GITHUB_TOKEN): raises the API rate limit
}

func (c Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: 60 * time.Second}
}

func (c Client) repo() string {
	if c.Repo != "" {
		return c.Repo
	}
	return DefaultRepo
}

func (c Client) endpoint(path string) string {
	base := c.APIBase
	if base == "" {
		base = DefaultAPIBase
	}
	return strings.TrimSuffix(base, "/") + path
}

// get performs one GET with the GitHub API headers and maps the error
// responses an update run can realistically meet to readable messages.
func (c Client) get(ctx context.Context, u string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "moca-update")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusForbidden && resp.Header.Get("x-ratelimit-remaining") == "0" {
			return nil, fmt.Errorf("GitHub API rate limit exceeded — set GITHUB_TOKEN or retry later")
		}
		return nil, fmt.Errorf("GET %s: %s", u, resp.Status)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxPackage+1))
	if err != nil {
		return nil, err
	}
	if len(b) > maxPackage {
		return nil, fmt.Errorf("%s: response exceeds %d MiB", u, maxPackage>>20)
	}
	return b, nil
}

// Latest returns the newest release, prereleases included: moca ships
// alphas, so a "stable only" query would find nothing for long stretches.
func (c Client) Latest(ctx context.Context) (Release, error) {
	b, err := c.get(ctx, c.endpoint("/repos/"+c.repo()+"/releases?per_page=1"))
	if err != nil {
		return Release{}, err
	}
	var rels []Release
	if err := json.Unmarshal(b, &rels); err != nil {
		return Release{}, fmt.Errorf("unexpected releases response: %w", err)
	}
	if len(rels) == 0 {
		return Release{}, fmt.Errorf("%s has no releases published yet", c.repo())
	}
	return rels[0], nil
}

// FindAsset picks the platform package from a release: an asset named
// moca-<tag>-<goos>-<goarch>.tar.gz (a .zip on Windows). The tag-in-name
// match is preferred; a looser suffix match covers a tag/asset skew.
func FindAsset(rel Release, goos, goarch string) (Asset, error) {
	ext := ".tar.gz"
	if goos == "windows" {
		ext = ".zip"
	}
	suffix := "-" + goos + "-" + goarch + ext
	want := "moca-" + rel.Tag + suffix
	for _, a := range rel.Assets {
		if a.Name == want {
			return a, nil
		}
	}
	for _, a := range rel.Assets {
		if strings.HasPrefix(a.Name, "moca-") && strings.HasSuffix(a.Name, suffix) {
			return a, nil
		}
	}
	return Asset{}, fmt.Errorf("release %s has no package for %s/%s", rel.Tag, goos, goarch)
}

// FindChecksums returns the release's checksums.txt asset when published.
func FindChecksums(rel Release) (Asset, bool) {
	for _, a := range rel.Assets {
		if a.Name == checksumAsset {
			return a, true
		}
	}
	return Asset{}, false
}

// ParseChecksums parses sha256sum output ("«hex»  «name»"; a leading * in
// binary mode is dropped).
func ParseChecksums(b []byte) map[string]string {
	sums := map[string]string{}
	for _, line := range strings.Split(string(b), "\n") {
		f := strings.Fields(line)
		if len(f) != 2 {
			continue
		}
		sums[strings.TrimPrefix(f[1], "*")] = strings.ToLower(f[0])
	}
	return sums
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// ExtractBinary returns the moca executable from a release package. The
// format follows the published naming contract: a .zip (Windows, moca.exe
// inside) or a .tar.gz (moca inside). Entries are matched by basename, so
// the archive may carry a directory prefix.
func ExtractBinary(archive []byte, name string) ([]byte, error) {
	switch {
	case strings.HasSuffix(name, ".zip"):
		zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		for _, f := range zr.File {
			if filepath.Base(f.Name) != "moca.exe" {
				continue
			}
			rc, err := f.Open()
			if err != nil {
				return nil, fmt.Errorf("%s: %w", name, err)
			}
			defer rc.Close()
			return io.ReadAll(io.LimitReader(rc, maxPackage))
		}
		return nil, fmt.Errorf("%s: no moca.exe inside", name)
	case strings.HasSuffix(name, ".tar.gz"), strings.HasSuffix(name, ".tgz"):
		gz, err := gzip.NewReader(bytes.NewReader(archive))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		defer gz.Close()
		tr := tar.NewReader(gz)
		for {
			hdr, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return nil, fmt.Errorf("%s: %w", name, err)
			}
			if filepath.Base(hdr.Name) != "moca" {
				continue
			}
			return io.ReadAll(io.LimitReader(tr, maxPackage))
		}
		return nil, fmt.Errorf("%s: no moca binary inside", name)
	}
	return nil, fmt.Errorf("%s: unsupported package format", name)
}

// IsTemporary reports whether exe lives in the OS temp dir — a `go run`
// build; replacing it would update a file that is deleted when it exits.
func IsTemporary(exe string) bool {
	dir, tmp := filepath.Dir(exe), os.TempDir()
	if r, err := filepath.EvalSymlinks(tmp); err == nil {
		tmp = r
	}
	if r, err := filepath.EvalSymlinks(dir); err == nil {
		dir = r
	}
	dir, tmp = filepath.Clean(dir), filepath.Clean(tmp)
	return dir == tmp || strings.HasPrefix(dir, tmp+string(filepath.Separator))
}

// Install downloads the release's platform package, verifies it against the
// release's checksums.txt (when present — verified reports whether one was
// checked), extracts the binary and swaps it in place of exe. The current
// process keeps running the old image; the new binary takes effect on the
// next launch.
func (c Client) Install(ctx context.Context, rel Release, asset Asset, exe string) (verified bool, err error) {
	pkg, err := c.get(ctx, asset.URL)
	if err != nil {
		return false, fmt.Errorf("download %s: %w", asset.Name, err)
	}
	if sumsAsset, ok := FindChecksums(rel); ok {
		sums, err := c.get(ctx, sumsAsset.URL)
		if err != nil {
			return false, fmt.Errorf("fetch %s: %w", sumsAsset.Name, err)
		}
		want, ok := ParseChecksums(sums)[asset.Name]
		if !ok {
			return false, fmt.Errorf("%s has no entry for %s", sumsAsset.Name, asset.Name)
		}
		if got := sha256Hex(pkg); got != want {
			return false, fmt.Errorf("checksum mismatch for %s: got %s, want %s", asset.Name, got, want)
		}
		verified = true
	}
	bin, err := ExtractBinary(pkg, asset.Name)
	if err != nil {
		return verified, err
	}
	tmp, err := os.CreateTemp(filepath.Dir(exe), ".moca-update-*")
	if err != nil {
		return verified, fmt.Errorf("cannot write next to %s: %w", exe, err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op after a successful replace
	if _, err := tmp.Write(bin); err != nil {
		tmp.Close()
		return verified, err
	}
	if err := tmp.Close(); err != nil {
		return verified, err
	}
	if err := os.Chmod(tmpName, 0o755); err != nil {
		return verified, err
	}
	if err := replace(exe, tmpName); err != nil {
		return verified, err
	}
	return verified, nil
}
