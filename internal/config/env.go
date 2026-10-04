package config

import (
	"fmt"
	"os"
	"slices"
	"strings"
)

const envPrefix = "env:"

// ResolveEnv resolves an "env:VAR" reference. Any other string is returned
// unchanged (MCP headers/env values may be literal; apiKey is validated to be
// a reference separately).
func ResolveEnv(ref string) (string, error) {
	name, ok := strings.CutPrefix(ref, envPrefix)
	if !ok {
		return ref, nil
	}
	v := os.Getenv(name)
	if v == "" {
		return "", &EnvError{Name: name, Ref: ref}
	}
	return v, nil
}

// EnvError reports an unset env: reference — a configuration error (exit 2).
type EnvError struct{ Name, Ref string }

func (e *EnvError) Error() string {
	return fmt.Sprintf("environment variable %s is not set (referenced as %q)", e.Name, e.Ref)
}

// EnvRefs lists every variable named by an env: reference anywhere in the
// config. The shell tool removes these from the model's environment (§4).
func EnvRefs(c Config) []string {
	var out []string
	add := func(s string) {
		if name, ok := strings.CutPrefix(s, envPrefix); ok && !slices.Contains(out, name) {
			out = append(out, name)
		}
	}
	for _, p := range c.Providers {
		add(p.APIKey)
	}
	for _, s := range c.MCP.Servers {
		for _, v := range s.Env {
			add(v)
		}
		for _, v := range s.Headers {
			add(v)
		}
	}
	slices.Sort(out)
	return out
}
