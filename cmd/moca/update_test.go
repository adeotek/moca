package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/config"
)

// fakeReleases serves a single-release GitHub payload whose only asset
// matches this platform.
func fakeReleases(t *testing.T, tag string) *httptest.Server {
	t.Helper()
	ext := ".tar.gz"
	if runtime.GOOS == "windows" {
		ext = ".zip"
	}
	name := fmt.Sprintf("moca-%s-%s-%s%s", tag, runtime.GOOS, runtime.GOARCH, ext)
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/adeotek/moca/releases" {
			http.NotFound(w, r)
			return
		}
		fmt.Fprintf(w, `[{"tag_name":%q,"assets":[{"name":%q,"browser_download_url":"http://%s/asset","size":12345678}]}]`, tag, name, r.Host)
	}))
}

func setVersion(t *testing.T, v string) {
	t.Helper()
	cur := config.Version
	config.Version = v
	t.Cleanup(func() { config.Version = cur })
}

func TestRunUpdateCheck(t *testing.T) {
	srv := fakeReleases(t, "v99.9.9")
	defer srv.Close()
	t.Setenv("MOCA_UPDATE_API", srv.URL)
	setVersion(t, "v1.0.0")
	var out, errb bytes.Buffer
	if code := run(context.Background(), []string{"update", "--check"}, nil, &out, &errb); code != exitOK {
		t.Fatalf("exit %d, stderr: %s", code, errb.String())
	}
	if want := "update available: v1.0.0 → v99.9.9"; !strings.Contains(out.String(), want) {
		t.Fatalf("stdout missing %q:\n%s", want, out.String())
	}
}

func TestRunUpdateUpToDate(t *testing.T) {
	srv := fakeReleases(t, "v1.0.0")
	defer srv.Close()
	t.Setenv("MOCA_UPDATE_API", srv.URL)
	setVersion(t, "v1.0.0")
	var out, errb bytes.Buffer
	if code := run(context.Background(), []string{"update", "--check"}, nil, &out, &errb); code != exitOK {
		t.Fatalf("exit %d, stderr: %s", code, errb.String())
	}
	if !strings.Contains(out.String(), "up to date (latest release v1.0.0)") {
		t.Fatalf("stdout:\n%s", out.String())
	}
}

func TestRunUpdateLocalNewer(t *testing.T) {
	srv := fakeReleases(t, "v1.0.0")
	defer srv.Close()
	t.Setenv("MOCA_UPDATE_API", srv.URL)
	setVersion(t, "v1.1.0")
	var out, errb bytes.Buffer
	if code := run(context.Background(), []string{"update", "--check"}, nil, &out, &errb); code != exitOK {
		t.Fatalf("exit %d, stderr: %s", code, errb.String())
	}
	if !strings.Contains(out.String(), "is newer than the latest release") {
		t.Fatalf("stdout:\n%s", out.String())
	}
}

func TestRunUpdateNoVersionStamp(t *testing.T) {
	// The API must not even be contacted: the version can't be compared.
	hit := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("API called for an unparseable local version: %s", r.URL)
	}))
	defer hit.Close()
	t.Setenv("MOCA_UPDATE_API", hit.URL)
	setVersion(t, "0b9528d")
	var out, errb bytes.Buffer
	if code := run(context.Background(), []string{"update"}, nil, &out, &errb); code != exitRuntime {
		t.Fatalf("exit %d, stderr: %s", code, errb.String())
	}
	if !strings.Contains(errb.String(), "cannot parse version") {
		t.Fatalf("stderr:\n%s", errb.String())
	}
}

func TestRunUpdateUsage(t *testing.T) {
	setVersion(t, "v1.0.0")
	var out, errb bytes.Buffer
	if code := run(context.Background(), []string{"update", "--bogus"}, nil, &out, &errb); code != exitUsage {
		t.Fatalf("exit %d, stderr: %s", code, errb.String())
	}
}

func TestRunUpdateFetchFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	t.Setenv("MOCA_UPDATE_API", srv.URL)
	setVersion(t, "v1.0.0")
	var out, errb bytes.Buffer
	if code := run(context.Background(), []string{"update"}, nil, &out, &errb); code != exitRuntime {
		t.Fatalf("exit %d, stderr: %s", code, errb.String())
	}
	if !strings.Contains(errb.String(), "500") {
		t.Fatalf("stderr:\n%s", errb.String())
	}
}

func TestRunUpdateMissingPlatformPackage(t *testing.T) {
	// A release whose assets carry no package for this platform.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `[{"tag_name":"v99.9.9","assets":[{"name":"moca-v99.9.9-plan9-mips.tar.gz","browser_download_url":"http://x/a","size":1}]}]`)
	}))
	defer srv.Close()
	t.Setenv("MOCA_UPDATE_API", srv.URL)
	setVersion(t, "v1.0.0")
	var out, errb bytes.Buffer
	if code := run(context.Background(), []string{"update", "--check"}, nil, &out, &errb); code != exitRuntime {
		t.Fatalf("exit %d, stderr: %s", code, errb.String())
	}
	if !strings.Contains(errb.String(), "no package for") {
		t.Fatalf("stderr:\n%s", errb.String())
	}
}
