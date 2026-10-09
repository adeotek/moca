package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/update"
)

// updateAPIBaseEnv overrides the GitHub API endpoint (tests, mirrors).
const updateAPIBaseEnv = "MOCA_UPDATE_API"

// runUpdate implements `moca update [--check]`. It runs before the config is
// loaded: updating the binary must not depend on a parseable config — a
// broken config may be exactly what the update is meant to fix.
func runUpdate(ctx context.Context, o Options, stdout, stderr io.Writer) int {
	check := false
	for _, a := range o.Sub[1:] {
		switch a {
		case "--check":
			check = true
		default:
			fmt.Fprintln(stderr, "moca: usage: moca update [--check]")
			return exitUsage
		}
	}
	cur, err := update.ParseVersion(config.Version)
	if err != nil {
		fmt.Fprintf(stderr, "moca: %v\nmoca: install a package from https://github.com/%s/releases to use `moca update`\n", err, update.DefaultRepo)
		return exitRuntime
	}
	cl := update.Client{
		APIBase: envOr(updateAPIBaseEnv, update.DefaultAPIBase),
		Repo:    update.DefaultRepo,
		Token:   githubToken(),
	}
	fmt.Fprintf(stdout, "checking %s for the latest release…\n", cl.Repo)
	rel, err := cl.Latest(ctx)
	if err != nil {
		fmt.Fprintln(stderr, "moca:", err)
		return exitRuntime
	}
	latest, err := update.ParseVersion(rel.Tag)
	if err != nil {
		fmt.Fprintf(stderr, "moca: latest release %s: %v\n", rel.Tag, err)
		return exitRuntime
	}
	switch cmp := update.Compare(latest, cur); {
	case cmp < 0:
		fmt.Fprintf(stdout, "moca %s is newer than the latest release (%s) — nothing to do.\n", config.Version, rel.Tag)
		return exitOK
	case cmp == 0:
		switch {
		case cur.Dev > 0:
			fmt.Fprintf(stdout, "moca %s is a development build of %s (%d commits past the tag) — the latest release is not newer.\n", config.Version, rel.Tag, cur.Dev)
		case cur.Dirty:
			fmt.Fprintf(stdout, "moca %s has local changes — the latest release %s is not newer.\n", config.Version, rel.Tag)
		default:
			fmt.Fprintf(stdout, "moca %s is up to date (latest release %s).\n", config.Version, rel.Tag)
		}
		return exitOK
	}
	asset, err := update.FindAsset(rel, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		fmt.Fprintln(stderr, "moca:", err)
		return exitRuntime
	}
	if check {
		fmt.Fprintf(stdout, "update available: %s → %s (%s, %.1f MiB)\nrun `moca update` to install it.\n", config.Version, rel.Tag, asset.Name, float64(asset.Size)/(1<<20))
		return exitOK
	}
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(stderr, "moca: cannot locate the running binary: %v\n", err)
		return exitRuntime
	}
	// Replace the file a symlink points to, not the link (os.Executable may
	// return the link's path, e.g. on macOS).
	if r, err := filepath.EvalSymlinks(exe); err == nil {
		exe = r
	}
	if update.IsTemporary(exe) {
		fmt.Fprintf(stderr, "moca: %s is a temporary build — install a package from https://github.com/%s/releases\n", exe, cl.Repo)
		return exitRuntime
	}
	fmt.Fprintf(stdout, "updating %s → %s (%s)…\n", config.Version, rel.Tag, asset.Name)
	verified, err := cl.Install(ctx, rel, asset, exe)
	if err != nil {
		fmt.Fprintf(stderr, "moca: update failed: %v\n", err)
		return exitRuntime
	}
	if !verified {
		fmt.Fprintf(stderr, "moca: warning: release %s has no checksums.txt — package integrity was not verified\n", rel.Tag)
	}
	fmt.Fprintf(stdout, "updated %s to %s — restart moca to use the new version.\n", exe, rel.Tag)
	return exitOK
}

// githubToken returns a rate-limit-raising token from the environment, if
// any (never required: releases are public).
func githubToken() string {
	for _, k := range []string{"GITHUB_TOKEN", "GH_TOKEN"} {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return ""
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
