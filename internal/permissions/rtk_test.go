package permissions

import (
	"slices"
	"strings"
	"testing"
)

// rtk must never launder a non-allowlisted command: `rtk <cmd>` is unwrapped
// like env/time, the wrapped name passes the same analysis, and rtk itself
// must be allowlisted too. Subcommand classes come from the real CLI (rtk
// 0.51.0) — see rtk.go.
func TestRtkUnwrap(t *testing.T) {
	root, _, _ := setup(t)
	j, _ := NewJail(root, nil)
	s := NewShell([]string{"rtk", "git", "go", "make", "docker", "tee"}, j, "linux")
	cases := map[string]verdict{
		"rtk":                            {},
		"rtk --version":                  {},
		"rtk git status":                 {},
		"rtk -v git status":              {},
		"rtk --ultra-compact git log":    {},
		"rtk docker ps":                  {},
		"env rtk git status":             {}, // rtk under a transparent wrapper
		"rtk proxy rtk git status":       {}, // nested rtk stays unwrapped
		"rtk read big.log":               {},
		"rtk ls src":                     {},
		"rtk gain":                       {},
		"rtk test":                       {}, // bare: rtk errors with "command is required"
		"rtk test -- go test ./...":      {},
		"rtk test go test ./...":         {}, // the direct form runs on the real CLI
		"rtk err echo hi":                {},
		"rtk summary make test":          {},
		"rtk run make build":             {},
		"rtk proxy tee out.txt":          {},
		"rtk python x.py":                {need: []string{"python"}},
		"rtk tsc --noEmit":               {need: []string{"tsc"}},
		"rtk proxy python x":             {need: []string{"python"}},
		"rtk proxy rm -rf build":         {every: []string{"rm"}},
		"rtk proxy timeout 5 rm x":       {every: []string{"rm"}},
		"rtk test -- sudo make":          {deny: "sudo"},
		"rtk err -- sudo make":           {deny: "sudo"},
		"rtk run -c 'echo hi'":           {deny: "cannot analyse"},
		"rtk run --command 'echo hi'":    {deny: "cannot analyse"},
		"rtk test --shell sh 'go run .'": {deny: "cannot analyse"},
		"rtk test --shell=sh 'go run .'": {deny: "cannot analyse"},
		"rtk $X":                         {deny: "non-literal"},
		"rtk proxy $X":                   {deny: "non-literal"},
		"rtk git log | rtk proxy curl":   {need: []string{"curl"}},
		"rtk proxy tee /etc/x":           {deny: "outside"},

		// Case variants must not skip the unwrap: `RTK proxy rm` classified
		// only `RTK` before, so one allow-always click laundered everything
		// behind it (review F2/M3).
		"RTK proxy rm -rf build": {need: []string{"RTK"}, every: []string{"rm"}},
		"Rtk proxy python x":     {need: []string{"Rtk", "python"}},
		"rtk PROXY python x":     {need: []string{"python"}},

		// An option-shaped word after `--` is refused, not classified as a
		// command name (review F4/M2).
		"rtk test -- -c 'echo hi'": {deny: "option-shaped"},
		"rtk run -- --shell x":     {deny: "option-shaped"},

		// Pinned probe result (rtk 0.51.0, 2026-10-06): after the command
		// word, `--shell` is literal argv of that command — the harness spy
		// never ran; only the option-before-command form executes a string
		// and is refused above.
		"rtk test echo hi --shell sh 'echo x'": {},
	}
	for cmd, want := range cases {
		need, every, err := s.Check(cmd)
		if want.deny != "" {
			if err == nil || !strings.Contains(err.Error(), want.deny) {
				t.Errorf("%q: want deny %q, got err=%v need=%v every=%v", cmd, want.deny, err, need, every)
			}
			continue
		}
		if err != nil {
			t.Errorf("%q: unexpected deny %v", cmd, err)
			continue
		}
		if !slices.Equal(need, want.need) || !slices.Equal(every, want.every) {
			t.Errorf("%q: need=%v every=%v, want need=%v every=%v", cmd, need, every, want.need, want.every)
		}
	}
	noRtk := NewShell([]string{"git"}, j, "linux")
	if need, _, _ := noRtk.Check("rtk git status"); !slices.Equal(need, []string{"rtk"}) {
		t.Fatal("rtk itself must be allowlisted", need)
	}
}
