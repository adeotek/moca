package secret

import "testing"

func TestTokenish(t *testing.T) {
	yes := []string{
		"sk-abc123xyz", "ghp_abcdefgh", "github_pat_abcdefgh", "glpat-abcdefgh",
		"xoxb-abcdefgh", "AKIAabcdefgh", "Bearer abcdefgh", "Basic abcdefgh",
		"run with «redacted:ghp_abcdefgh» now",
		"Bearer $GITHUB_TOKEN", // pinned false positive: a scheme value matches
	}
	for _, s := range yes {
		if !Tokenish(s) {
			t.Errorf("Tokenish(%q) = false, want true", s)
		}
	}
	no := []string{"sk-1", "monkey", "-race", "https://example.com/x"}
	for _, s := range no {
		if Tokenish(s) {
			t.Errorf("Tokenish(%q) = true, want false", s)
		}
	}
}

func TestValue(t *testing.T) {
	yes := []string{"sk-abc123xyz", "Bearer abcdefgh", "sk-1"}
	for _, s := range yes {
		if !Value(s) {
			t.Errorf("Value(%q) = false, want true", s)
		}
	}
	no := []string{"prefix sk-abc123xyz", "-race"}
	for _, s := range no {
		if Value(s) {
			t.Errorf("Value(%q) = true, want false", s)
		}
	}
}

func TestKey(t *testing.T) {
	yes := []string{"API_KEY", "authorization", "monkey"} // "monkey" is pinned broadness
	for _, s := range yes {
		if !Key(s) {
			t.Errorf("Key(%q) = false, want true", s)
		}
	}
	no := []string{"model", "path"}
	for _, s := range no {
		if Key(s) {
			t.Errorf("Key(%q) = true, want false", s)
		}
	}
}

func TestUserinfo(t *testing.T) {
	if !Userinfo("postgres://user:pass@host") {
		t.Error("Userinfo missed URL credentials")
	}
	if Userinfo("https://example.com/x") {
		t.Error("Userinfo matched a plain URL")
	}
}

func TestEmbedded(t *testing.T) {
	yes := []string{"postgres://user:pass@host", "password=hunter2", "ghp_abcdefgh"}
	for _, s := range yes {
		if !Embedded(s) {
			t.Errorf("Embedded(%q) = false, want true", s)
		}
	}
	no := []string{"https://example.com/x", "-race", "sk-1"}
	for _, s := range no {
		if Embedded(s) {
			t.Errorf("Embedded(%q) = true, want false", s)
		}
	}
}

func TestMatch(t *testing.T) {
	rows := []struct {
		in, class string
	}{
		{"postgres://user:pass@host", "URL credentials"},
		{"ghp_abcdefgh", "credential token"},
		{"password=hunter2", "password"},
		{"-----BEGIN RSA PRIVATE KEY-----", "private key"},
		{"eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U", "JWT"},
		{"sk_live_abcdefgh1234", "Stripe key"},
		{"rk_live_abcdefgh1234", "Stripe key"},
		{"AIzaSyA1234567890abcdefghijklmnopqrstu", "Google API key"},
		{"npm_abcdefghijklmnopqrstuvwxyz", "npm token"},
		{"pypi-abcdefghijklmnopqrstuvwxyz", "PyPI token"},
		{"xoxe-1234567890-abcdef", "Slack token"},
		{"https://hooks.slack.com/services/T000/B000/XXXX", "Slack webhook"},
	}
	for _, r := range rows {
		class, ok := Match(r.in)
		if !ok {
			t.Errorf("Match(%q) = no match, want %q", r.in, r.class)
			continue
		}
		if class != r.class {
			t.Errorf("Match(%q) class = %q, want %q", r.in, class, r.class)
		}
	}
	if class, ok := Match("-race"); ok {
		t.Errorf("Match(-race) = %q, want no match", class)
	}
	// A token prefix inside a longer word is prose, not a credential.
	for _, s := range []string{"run the task-scheduler suite", "the risk-assessment job", "make desk-integration"} {
		if class, ok := Match(s); ok {
			t.Errorf("Match(%q) = %q, want no match (a prefix inside a word)", s, class)
		}
	}
	if class, ok := Match("token: sk-abc123xyz42"); !ok || class != "credential token" {
		t.Errorf("Match(sk-…) at a word start = %q/%v, want credential token", class, ok)
	}
}

func TestMatchIgnoresEnvReferences(t *testing.T) {
	for _, s := range []string{"password=$DB_PASSWORD", "Bearer ${TOKEN}", "env:API_KEY", "%API_KEY%", "$env:API_KEY", "Bearer `env:API_KEY`"} {
		if class, ok := Match(s); ok {
			t.Errorf("Match(%q) = %q, want no match (variable reference)", s, class)
		}
	}
	if class, ok := Match("password=hunter2"); !ok || class != "password" {
		t.Errorf("Match(password=hunter2) = %q/%v, want password/true", class, ok)
	}
	// The user part cannot hold a ':' — the first one after :// splits it, so
	// a $env:VAR password is still a reference.
	for _, s := range []string{"DATABASE_URL=postgres://app:${PGPASSWORD}@localhost/db", "DSN=postgres://app:$env:PGPASS@db/x"} {
		if class, ok := Match(s); ok {
			t.Errorf("Match(%q): URL userinfo naming a variable = %q, want no match", s, class)
		}
	}
	if class, ok := Match("postgres://app:hunter2@localhost/db"); !ok || class != "URL credentials" {
		t.Errorf("URL userinfo with a literal password = %q/%v, want URL credentials", class, ok)
	}
}

// A Bearer/Basic scheme followed by an ordinary word is prose; a base64 or
// random-looking value is still a credential.
func TestMatchSchemeProse(t *testing.T) {
	for _, s := range []string{
		"- **ci**: Basic validation runs in make check (×1, 2026-10-19)",
		"send Bearer authentication-free requests to the mock",
		"Basic Configuration lives in config.yaml",
	} {
		if class, ok := Match(s); ok {
			t.Errorf("Match(%q) = %q, want no match (prose)", s, class)
		}
	}
	// The scheme match swallows a prefix token after it (matches do not
	// overlap): "Bearer sk-…" must still be refused, digits or not.
	for _, s := range []string{"Authorization: Basic dXNlcjpwYXNz", "Bearer abc123def456", "Bearer eyJhbGciOi.x",
		"Authorization: Bearer sk-abcdefghijklmnop", "Bearer glpat-abcdefghijkl", "Bearer xoxb-abcdefghijkl"} {
		if class, ok := Match(s); !ok || class != "credential token" {
			t.Errorf("Match(%q) = %q/%v, want credential token", s, class, ok)
		}
	}
}
