package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/provider"
)

// oauthLookup resolves a provider's OAuth parameters; a variable so the CLI
// can be driven against a fake authorization server in tests.
var oauthLookup = provider.OAuthProvider

// revokeTimeout bounds the best-effort remote revocation on logout (a var so
// tests can shrink it): a stalled revoke endpoint must not hang the command.
var revokeTimeout = 15 * time.Second

// loginHTTPTimeout bounds each login HTTP call — the token exchange and the
// JWKS fetch (a var so tests can shrink it). The browser-side wait is not
// part of it.
var loginHTTPTimeout = 30 * time.Second

// runLogin implements `moca login <provider> [--no-browser]`: the SIWC
// authorization-code flow (openai only — policy gate, §3), then the token
// store write, then an offer to flip the provider's config to auth "oauth".
func runLogin(ctx context.Context, o Options, cfg config.Config, cfgPath string, stdin io.Reader, stdout, stderr io.Writer) int {
	noBrowser := false
	var rest []string
	for _, a := range o.Sub[1:] {
		if a == "--no-browser" {
			noBrowser = true
			continue
		}
		rest = append(rest, a)
	}
	if len(rest) != 1 {
		fmt.Fprintln(stderr, "moca: usage: moca login <provider> [--no-browser]")
		return exitUsage
	}
	p := rest[0]
	oc, ok := oauthLookup(p)
	if !ok {
		if reason, unsupported := config.OAuthUnsupported[p]; unsupported {
			fmt.Fprintf(stderr, "moca: providers.%s cannot use OAuth: %s\n", p, reason)
		} else if _, known := cfg.Providers[p]; !known {
			fmt.Fprintf(stderr, "moca: unknown provider %q\n", p)
		} else {
			fmt.Fprintf(stderr, "moca: no OAuth for %s — set auth \"api_key\" for providers.%s\n", p, p)
		}
		return exitUsage
	}

	store := provider.NewStore(filepath.Join(config.DataDir(), "auth.json"))
	saved, hasSaved, err := store.Get(p)
	if err != nil {
		fmt.Fprintln(stderr, "moca:", err)
		return exitUsage
	}
	var savedPtr *provider.Token
	if hasSaved && saved.ClientID != "" {
		savedPtr = &saved // reauthorization with the saved registration
	}
	hostID, err := store.HostID()
	if err != nil {
		fmt.Fprintln(stderr, "moca:", err)
		return exitRuntime
	}
	oc.HostID = hostID
	// One line reader for the whole command: the paste prompt and the
	// "switch auth?" prompt share it, so an unanswered paste read can never
	// swallow the second answer.
	var lines *provider.LineReader
	if stdin != nil {
		lines = provider.NewLineReader(stdin)
	}
	lio := provider.LoginIO{Out: stdout, Lines: lines, Headless: noBrowser || provider.Headless()}
	if !noBrowser {
		lio.OpenURL = provider.OpenBrowser
	}
	tok, err := provider.Login(ctx, oc, savedPtr, lio, &http.Client{Timeout: loginHTTPTimeout})
	if err != nil {
		fmt.Fprintln(stderr, "moca:", err)
		return exitFor(ctx, err)
	}
	if err := store.Put(p, tok); err != nil {
		fmt.Fprintln(stderr, "moca:", err)
		return exitRuntime
	}
	if tok.Email != "" {
		fmt.Fprintf(stdout, "logged in to %s as %s\n", p, tok.Email)
	} else {
		fmt.Fprintf(stdout, "logged in to %s\n", p)
	}
	if cfg.Providers[p].Auth != "oauth" {
		fmt.Fprintf(stdout, "Set \"auth\": \"oauth\" for providers.%s in %s to use your subscription.\n", p, cfgPath)
		if yesNo(lines, stdout, "Switch it now? [y/N] ") {
			if _, err := config.SetString(cfgPath, []string{"providers", p}, "auth", "oauth"); err != nil {
				fmt.Fprintln(stderr, "moca:", err)
				return exitRuntime
			}
			fmt.Fprintf(stdout, "updated %s\n", cfgPath)
		}
	}
	return exitOK
}

// runLogout implements `moca logout <provider>`: best-effort remote
// revocation, then clear the local registration. Idempotent, and a corrupt
// store is reset rather than blocking (its error says to run this).
func runLogout(ctx context.Context, o Options, cfg config.Config, stdout, stderr io.Writer) int {
	args := o.Sub[1:]
	if len(args) != 1 {
		fmt.Fprintln(stderr, "moca: usage: moca logout <provider>")
		return exitUsage
	}
	p := args[0]
	if _, known := cfg.Providers[p]; !known {
		fmt.Fprintf(stderr, "moca: unknown provider %q\n", p)
		return exitUsage
	}
	store := provider.NewStore(filepath.Join(config.DataDir(), "auth.json"))
	tok, ok, err := store.Get(p)
	if err != nil {
		fmt.Fprintf(stderr, "moca: warning: %v\n", err)
	}
	if ok && tok.Refresh != "" {
		if oc, has := oauthLookup(p); has {
			rctx, cancel := context.WithTimeout(ctx, revokeTimeout)
			rerr := provider.Revoke(rctx, oc, tok, http.DefaultClient)
			cancel()
			if rerr != nil {
				fmt.Fprintf(stderr, "moca: warning: could not revoke the session remotely (%v); clearing locally\n", rerr)
			}
		}
	}
	if err := store.Delete(p); err != nil {
		fmt.Fprintln(stderr, "moca:", err)
		return exitRuntime
	}
	fmt.Fprintf(stdout, "logged out of %s\n", p)
	return exitOK
}

func yesNo(lines *provider.LineReader, stdout io.Writer, prompt string) bool {
	if lines == nil {
		return false
	}
	fmt.Fprint(stdout, prompt)
	line, err := lines.Next(nil)
	if err != nil && line == "" {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true
	}
	return false
}
