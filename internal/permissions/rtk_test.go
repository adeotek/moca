package permissions

import (
	"slices"
	"testing"
)

// rtk must never launder a non-allowlisted command: `rtk <cmd>` is unwrapped
// like env/time, the wrapped name passes the same analysis, and rtk itself
// must be allowlisted too. Subcommand classes come from the real CLI (rtk
// 1.1.0, re-checked 2026-10-27) — see rtk.go.
func TestRtkUnwrap(t *testing.T) {
	root, _, _ := setup(t)
	j, _ := NewJail(root, nil, nil)
	s := NewShell(RulesFromAllow([]string{"rtk", "git", "go", "make", "docker", "tee"}), j, "linux")
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

		// rtk 1.1.0's new surface stays fail-closed: proxy subcommands are
		// classified as the tool they wrap (so the tool must be allowlisted),
		// and the authoring subcommands as themselves — none is rtkSelf, so
		// `pipeline add`/`plugins build` ask instead of writing silently.
		"rtk glab mr list":           {need: []string{"glab"}},
		"rtk aws s3 ls":              {need: []string{"aws"}},
		"rtk psql -l":                {need: []string{"psql"}},
		"rtk pnpm install":           {need: []string{"pnpm"}},
		"rtk dotnet test":            {need: []string{"dotnet"}},
		"rtk oc get pods":            {need: []string{"oc"}},
		"rtk wget https://x/y":       {need: []string{"wget"}},
		"rtk mvn test":               {need: []string{"mvn"}},
		"rtk gradlew build":          {need: []string{"gradlew"}},
		"rtk pipeline add --x y":     {need: []string{"pipeline"}},
		"rtk plugins build":          {need: []string{"plugins"}},
		"rtk tasks list":             {need: []string{"tasks"}},
		"rtk methods list":           {need: []string{"methods"}},
		"rtk mvn test | rtk git log": {need: []string{"mvn"}},

		// Pinned probe result (rtk 0.51.0, 2026-10-06; re-probed on 1.1.0,
		// 2026-10-27: `rtk test echo A --shell` prints `A --shell`): after the
		// command word, `--shell` is literal argv of that command — the harness
		// spy never ran; only the option-before-command form executes a string
		// and is refused above.
		"rtk test echo hi --shell sh 'echo x'": {},
	}
	for cmd, want := range cases {
		need, every, unsure, err := s.Check(cmd)
		assertVerdict(t, cmd, want, need, every, unsure, err)
	}
	// Native-binary proxies forward their flags to the real tool, so they must
	// pass the allowlist exactly as when run directly (review pass 3): with a
	// narrow allowlist `rtk find -exec` must not run programs for free. rtk's
	// own read-only machinery stays plain `rtk`; the config-mutating
	// subcommands ask once.
	narrow := NewShell(RulesFromAllow([]string{"rtk", "git"}), j, "linux")
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
		need, every, _, err := narrow.Check(cmd)
		if err != nil || len(every) != 0 || !slices.Equal(need, want) {
			t.Errorf("narrow %q: need=%v every=%v err=%v; want need=%v", cmd, need, every, err, want)
		}
	}
	noRtk := NewShell(RulesFromAllow([]string{"git"}), j, "linux")
	if need, _, _, _ := noRtk.Check("rtk git status"); !slices.Equal(need, []string{"rtk"}) {
		t.Fatal("rtk itself must be allowlisted", need)
	}
}
