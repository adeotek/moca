package permissions

import (
	"slices"
	"strings"
	"testing"
)

func newTestShell(t *testing.T) *Shell {
	root, _, _ := setup(t)
	j, _ := NewJail(root, nil)
	return NewShell([]string{"go", "git", "make", "grep", "cat", "tee", "ls"}, j, "linux")
}

type verdict struct {
	need, every []string
	deny        string // substring of error, "" = no error
}

func TestShellAnalysisLadder(t *testing.T) {
	s := newTestShell(t)
	cases := map[string]verdict{
		"go test ./...":                  {},
		"python x.py":                    {need: []string{"python"}},
		"git log | python -c 1":          {need: []string{"python"}},
		"go build && python x":           {need: []string{"python"}},
		"go vet; python x":               {need: []string{"python"}},
		"echo $(rm x)":                   {every: []string{"rm"}},
		"echo `rm x`":                    {every: []string{"rm"}},
		"( sudo ls )":                    {deny: "sudo"},
		"FOO=1 go test":                  {},
		"timeout 5 rm x":                 {every: []string{"rm"}},
		"env A=1 B=2 nice -n 5 python x": {need: []string{"python"}},
		"cat a > ../outside/x":           {deny: "outside"},
		"cat a > /dev/null 2>&1":         {},
		"cat a >> out.txt":               {},
		"cat a | tee ../outside/log":     {deny: "outside"},
		"eval ls":                        {deny: "eval"},
		"$CMD args":                      {deny: "non-literal"},
		"cat a > $OUT":                   {deny: "non-literal"},
		"if then fi (":                   {deny: "parse"},
		"cd sub && make":                 {},
		"mkfs.ext4 /dev/sda":             {deny: "mkfs"},
		"source env.sh":                  {deny: "source"},
		". env.sh":                       {deny: "."},
		"exec go":                        {deny: "exec"},
		"cat <(rm x)":                    {every: []string{"rm"}},
		"{ go test; python y; }":         {need: []string{"python"}},
		"go test & python bg":            {need: []string{"python"}},
		"go test\npython y":              {need: []string{"python"}},
		`"go" test`:                      {},
		"command -v python":              {},
		"rm -rf build":                   {every: []string{"rm"}},
		"/usr/bin/sudo ls":               {deny: "sudo"},
		// wrapper option-argument parsing (review: env -u ate the command)
		"env -u ls sudo id":      {deny: "sudo"},
		"env -u FOO sudo id":     {deny: "sudo"},
		"env -u FOO rm -rf x":    {every: []string{"rm"}},
		"env -C /tmp rm -rf x":   {every: []string{"rm"}},
		"env --unset=FOO rm x":   {every: []string{"rm"}},
		"env -S 'rm -rf x'":      {deny: "cannot analyse"},
		"command -p sudo id":     {deny: "sudo"},
		"command -p rm x":        {every: []string{"rm"}},
		"timeout -s KILL 5 rm x": {every: []string{"rm"}},
		"nice -10 rm x":          {every: []string{"rm"}},
		"time -p go test":        {},
		// escaped / globbed command names (review: \sudo, sud?)
		`\sudo id`:      {deny: "non-literal"},
		`s\udo id`:      {deny: "non-literal"},
		`$'s\x75do' id`: {deny: "non-literal"},
		"sud? id":       {deny: "non-literal"},
		"su* id":        {deny: "non-literal"},
		"s[ud]o id":     {deny: "cannot parse"}, // array-index syntax → parse-level refusal
		// redirect hardening (review: cd-relative and <> targets)
		"cd / && echo hi > etc/moca-test": {deny: "relative redirect"},
		"cd .. && echo hi > x":            {deny: "relative redirect"},
		"echo hi 1<> /etc/x":              {deny: "outside"},
		"echo hi <> out.txt":              {},
		"cd sub && echo hi > /dev/null":   {},
		"cd sub && cat a | tee out.log":   {deny: "relative redirect"},
	}
	for cmd, want := range cases {
		need, every, err := s.Check(cmd)
		if want.deny != "" {
			if err == nil || !strings.Contains(err.Error(), want.deny) {
				t.Errorf("%q: want deny containing %q, got err=%v need=%v every=%v", cmd, want.deny, err, need, every)
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
}

func TestAllowAndRmNeverAllowlistable(t *testing.T) {
	s := newTestShell(t)
	s.Allow("python")
	if need, _, _ := s.Check("python x"); len(need) != 0 {
		t.Fatal("Allow must stick")
	}
	s.Allow("rm")
	if _, every, _ := s.Check("rm x"); len(every) != 1 {
		t.Fatal("rm stays ask-every-time even if allowed")
	}
	s.Allow("sudo")
	if _, _, err := s.Check("sudo x"); err == nil {
		t.Fatal("hard-deny has no override")
	}
}

func TestWindowsBestEffort(t *testing.T) {
	root, _, _ := setup(t)
	j, _ := NewJail(root, nil)
	s := NewShell([]string{"go", "git"}, j, "windows")
	if need, every, err := s.Check("go test ./...; Remove-Item x"); err != nil || len(need) != 0 || !slices.Equal(every, []string{"remove-item"}) {
		t.Fatal(need, every, err)
	}
	if _, _, err := s.Check("Format-Volume C"); err == nil {
		t.Fatal("Format-Volume hard-denied")
	}
	if need, _, _ := s.Check("git status | python.exe x"); !slices.Equal(need, []string{"python"}) {
		t.Fatal(need)
	}
}
