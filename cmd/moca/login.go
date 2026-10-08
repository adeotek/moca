package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
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

// runLogin implements `moca login <provider> [--api-key] [--no-browser]`.
// A provider with an OAuth flow (openai) runs the SIWC authorization-code
// flow unless --api-key is given; any provider can have an API key stored
// instead. Both credential kinds live in auth.json (~/.config/moca/). The
// OAuth path then offers to flip the provider's config to auth "oauth"; the
// key path notes when the configured auth mode would ignore the key.
func runLogin(ctx context.Context, o Options, cfg config.Config, cfgPath string, stdin io.Reader, stdout, stderr io.Writer) int {
	noBrowser, wantKey := false, false
	var rest []string
	for _, a := range o.Sub[1:] {
		switch a {
		case "--no-browser":
			noBrowser = true
		case "--api-key":
			wantKey = true
		default:
			rest = append(rest, a)
		}
	}
	if len(rest) != 1 {
		fmt.Fprintln(stderr, "moca: usage: moca login <provider> [--api-key] [--no-browser]")
		return exitUsage
	}
	p := rest[0]
	pc, known := cfg.Providers[p]
	if !known {
		fmt.Fprintf(stderr, "moca: unknown provider %q\n", p)
		return exitUsage
	}
	// One line reader for the whole command: the API-key prompt, the paste
	// prompt and the "switch auth?" prompt share it, so an unanswered read
	// can never swallow the next answer.
	var lines *provider.LineReader
	if stdin != nil {
		lines = provider.NewLineReader(stdin)
	}
	store := provider.NewDefaultStore()
	oc, hasOAuth := oauthLookup(p)
	if reason, unsupported := config.OAuthUnsupported[p]; unsupported && !hasOAuth && !wantKey {
		fmt.Fprintf(stdout, "note: %s cannot use subscription OAuth: %s\n", p, reason)
	}
	if wantKey || !hasOAuth {
		return loginAPIKey(p, pc.Auth, store, stdin, lines, stdout, stderr)
	}

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
	if pc.Auth != "oauth" {
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

// loginAPIKey prompts for one API key line and stores it. The prompt is
// shown only when stdin is a terminal (`echo -n "$KEY" | moca login <p>`
// stores silently); a nil or exhausted stdin is a usage error.
func loginAPIKey(p, authMode string, store *provider.Store, stdin io.Reader, lines *provider.LineReader, stdout, stderr io.Writer) int {
	if lines == nil {
		fmt.Fprintf(stderr, "moca: login %s needs stdin for the API key — run /login in the TUI, or set providers.%s.apiKey\n", p, p)
		return exitUsage
	}
	if f, ok := stdin.(*os.File); ok {
		if fi, err := f.Stat(); err == nil && fi.Mode()&os.ModeCharDevice != 0 {
			fmt.Fprintf(stdout, "API key for %s: ", p)
		}
	}
	line, _ := lines.Next(nil)
	key := strings.TrimSpace(line)
	if key == "" {
		fmt.Fprintln(stderr, "moca: no API key given")
		return exitUsage
	}
	if err := store.PutAPIKey(p, key); err != nil {
		fmt.Fprintln(stderr, "moca:", err)
		return exitRuntime
	}
	fmt.Fprintf(stdout, "stored API key for %s in %s\n", p, store.Path())
	if authMode == "oauth" {
		fmt.Fprintf(stdout, "note: providers.%s.auth is \"oauth\" — set it to \"api_key\" to use this key\n", p)
	}
	return exitOK
}

// runLogout implements `moca logout <provider>`: best-effort remote
// revocation of a stored OAuth session, then clears the provider's entry
// (token and API key). Idempotent, and a corrupt store is reset rather than
// blocking (its error says to run this).
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
	store := provider.NewDefaultStore()
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
