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
	j, _ := NewJail(root, nil, nil)
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
		"rtk ls src":                     {need: []string{"ls"}}, // native proxy: ls must be allowlisted
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
	// Native-binary proxies forward their flags to the real tool, so they must
	// pass the allowlist exactly as when run directly (review pass 3): with a
	// narrow allowlist `rtk find -exec` must not run programs for free. rtk's
	// own read-only machinery stays plain `rtk`; the config-mutating
	// subcommands ask once.
	narrow := NewShell([]string{"rtk", "git"}, j, "linux")
	narrowCases := map[string][]string{
		"rtk find . -exec python3 x ;":        {"find"},
		"rtk find . -delete":                  {"find"},
		"rtk rg --pre ./x pat .":              {"rg"},
		"rtk grep -r pat .":                   {"grep"},
		"rtk ls":                              {"ls"},
		"rtk tree -o out.txt":                 {"tree"},
		"rtk ast-grep run -p a -r b -U .":     {"ast-grep"},
		"rtk wc -l x":                         {"wc"},
		"rtk diff a b":                        {"diff"},
		"rtk -v find . -name x":               {"find"},
		"RTK Find . -name x":                  {"RTK", "Find"},
		"rtk init -g":                         {"init"},
		"rtk trust -y":                        {"trust"},
		"rtk config":                          {"config"},
		"rtk learn --write-rules":             {"learn"},
		"rtk telemetry disable":               {"telemetry"},
		"rtk read big.log":                    nil,
		"rtk gain":                            nil,
		"rtk json j.json":                     nil,
		"rtk smart f.go":                      nil,
		"rtk env":                             nil,
		"rtk git status | rtk find . -type f": {"find"},
	}
	for cmd, want := range narrowCases {
		need, every, err := narrow.Check(cmd)
		if err != nil || len(every) != 0 || !slices.Equal(need, want) {
			t.Errorf("narrow %q: need=%v every=%v err=%v; want need=%v", cmd, need, every, err, want)
		}
	}
	noRtk := NewShell([]string{"git"}, j, "linux")
	if need, _, _ := noRtk.Check("rtk git status"); !slices.Equal(need, []string{"rtk"}) {
		t.Fatal("rtk itself must be allowlisted", need)
	}
}
