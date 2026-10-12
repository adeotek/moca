package permissions

import (
	"slices"
	"strings"
	"testing"
)

func newTestShell(t *testing.T) *Shell {
	root, _, _ := setup(t)
	j, _ := NewJail(root, nil, nil)
	return NewShell(RulesFromAllow([]string{"go", "git", "make", "grep", "cat", "tee", "ls"}), j, "linux")
}

type verdict struct {
	need, every, unsure []string
	deny                string // substring of error, "" = no error
}

func assertVerdict(t *testing.T, cmd string, want verdict, need, every, unsure []string, err error) {
	t.Helper()
	if want.deny != "" {
		if err == nil || !strings.Contains(err.Error(), want.deny) {
			t.Errorf("%q: want deny containing %q, got err=%v need=%v every=%v", cmd, want.deny, err, need, every)
		}
		return
	}
	if err != nil {
		t.Errorf("%q: unexpected deny %v", cmd, err)
		return
	}
	if !slices.Equal(need, want.need) || !slices.Equal(every, want.every) || !slices.Equal(unsure, want.unsure) {
		t.Errorf("%q: need=%v every=%v unsure=%v, want %+v", cmd, need, every, unsure, want)
	}
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
		need, every, unsure, err := s.Check(cmd)
		assertVerdict(t, cmd, want, need, every, unsure, err)
	}
}

func TestAllowAndRmNeverAllowlistable(t *testing.T) {
	s := newTestShell(t)
	s.Allow("python")
	if need, _, _, _ := s.Check("python x"); len(need) != 0 {
		t.Fatal("Allow must stick")
	}
	s.Allow("rm")
	if _, every, _, _ := s.Check("rm x"); len(every) != 1 {
		t.Fatal("rm stays ask-every-time even if allowed")
	}
	s.Allow("sudo")
	if _, _, _, err := s.Check("sudo x"); err == nil {
		t.Fatal("hard-deny has no override")
	}
}

func TestWindowsBestEffort(t *testing.T) {
	root, _, _ := setup(t)
	j, _ := NewJail(root, nil, nil)
	s := NewShell(RulesFromAllow([]string{"go", "git"}), j, "windows")
	if need, every, _, err := s.Check("go test ./...; Remove-Item x"); err != nil || len(need) != 0 || !slices.Equal(every, []string{"remove-item"}) {
		t.Fatal(need, every, err)
	}
	if _, _, _, err := s.Check("Format-Volume C"); err == nil {
		t.Fatal("Format-Volume hard-denied")
	}
	if need, _, _, _ := s.Check("git status | python.exe x"); !slices.Equal(need, []string{"python"}) {
		t.Fatal(need)
	}

	// rtk is unwrapped best-effort on Windows too (review F1/M4): the
	// runner's first positional word (or the tool sub word) is classified.
	s2 := NewShell(RulesFromAllow([]string{"rtk", "go", "git", "docker"}), j, "windows")
	if need, every, _, err := s2.Check("rtk proxy rm -rf x"); err != nil || len(need) != 0 || !slices.Equal(every, []string{"rm"}) {
		t.Fatal("rtk proxy rm:", need, every, err)
	}
	if need, _, _, _ := s2.Check("rtk proxy python.exe x"); !slices.Equal(need, []string{"python"}) {
		t.Fatal("rtk proxy python.exe:", need)
	}
	if need, every, _, err := s2.Check("rtk test -- go test ./..."); err != nil || len(need) != 0 || len(every) != 0 {
		t.Fatal("rtk test -- go:", need, every, err)
	}
	if _, _, _, err := s2.Check("rtk test -- sudo make"); err == nil {
		t.Fatal("rtk test -- sudo must hard-deny on Windows")
	}
	if need, every, _, err := s2.Check("rtk read big.log"); err != nil || len(need) != 0 || len(every) != 0 {
		t.Fatal("rtk read:", need, every, err)
	}
	if need, every, _, err := s2.Check("rtk docker ps"); err != nil || len(need) != 0 || len(every) != 0 {
		t.Fatal("rtk docker:", need, every, err)
	}
	// Native proxies and config-mutating subcommands are classified as the
	// wrapped command on Windows too (review pass 3).
	if need, _, _, _ := s2.Check("rtk find . -exec x ;"); !slices.Equal(need, []string{"find"}) {
		t.Fatal("rtk find:", need)
	}
	if need, _, _, _ := s2.Check("rtk init -g"); !slices.Equal(need, []string{"init"}) {
		t.Fatal("rtk init:", need)
	}
}

// Brace expansion happens before the command runs, so `{sudo,ls}` is not a
// command named "{sudo,ls}": classifying it as one skipped hard-deny and
// ask-every-time. And on case-insensitive filesystems `SUDO`/`RM` are the real
// binaries, so the safety classes match case-insensitively.
func TestShellBraceExpansionAndCaseVariants(t *testing.T) {
	s := newTestShell(t)
	cases := map[string]verdict{
		"{sudo,ls}":            {deny: "non-literal"},
		"{rm,x} y":             {deny: "non-literal"},
		"{1..3}":               {deny: "non-literal"},
		"echo > {a,b}":         {deny: "non-literal"},
		"env {rm,x} y":         {deny: "non-literal"},
		`{"sudo",ls}`:          {deny: "non-literal"}, // quotes split the brace group across parts
		`{r'm',x} y`:           {deny: "non-literal"},
		`r{"m",} y`:            {deny: "non-literal"},
		`/bin/r["m"] x`:        {deny: "non-literal"},
		`{rm","x}`:             {need: []string{"{rm,x}"}}, // a quoted comma does not expand
		`'{rm,x}' y`:           {need: []string{"{rm,x}"}}, // fully quoted: literal
		"SUDO ls":              {deny: "hard-deny"},
		"Sudo ls":              {deny: "hard-deny"},
		"MKFS.ext4 /dev/x":     {deny: "hard-deny"},
		"EVAL x":               {deny: "refused"},
		"RM -rf x":             {every: []string{"RM"}},
		"go test ./... ; Rm x": {every: []string{"Rm"}},
		"echo {a,b}":           {}, // arguments are not command positions
		"go test -run '{a,b}'": {},
	}
	for cmd, want := range cases {
		need, every, unsure, err := s.Check(cmd)
		assertVerdict(t, cmd, want, need, every, unsure, err)
	}
}

func newRuleShell(t *testing.T, proj, user RuleSet) *Shell {
	root, _, _ := setup(t)
	j, _ := NewJail(root, nil, nil)
	return NewShell(NewRules(proj, user), j, "linux")
}

func TestShellRules(t *testing.T) {
	proj, _ := ParseRuleSet("project", []string{"make *"}, nil, []string{"git push *"})
	user, _ := ParseRuleSet("user", []string{"git", "rtk", "rm build/*"}, []string{"git commit *"}, nil)
	s := newRuleShell(t, proj, user)
	cases := map[string]verdict{
		"git log":              {},
		"git push origin main": {deny: `"git push *" (project`},
		"git commit -m x":      {every: []string{"git"}},
		"make test":            {},
		"rm build/x.o":         {}, // explicit allow overrides the built-in rm ask
		"rm src/x.go":          {every: []string{"rm"}},
		"rtk git log":          {},
		"rtk proxy python x":   {need: []string{"python"}}, // rtk * never launders the wrapped command
		"sudo git log":         {deny: "hard-deny"},        // rules never beat hard-deny
		"./git log":            {need: []string{"./git"}},  // strict allow heads: full head is the identity
		"/bin/rm src/x.go":     {every: []string{"rm"}},
	}
	for cmd, want := range cases {
		need, every, unsure, err := s.Check(cmd)
		assertVerdict(t, cmd, want, need, every, unsure, err)
	}
}

func TestRulesSeeThroughWrappersAndCase(t *testing.T) {
	user, _ := ParseRuleSet("user", []string{"git"}, nil, []string{"git push *", "rm -rf *"})
	s := newRuleShell(t, RuleSet{Name: "project"}, user)
	for _, cmd := range []string{"GIT push x", "/usr/bin/git push", "timeout 5 git push", "env A=1 git push",
		"rtk proxy rm -rf x", "RM -rf ~", "git $X origin"} {
		if _, _, _, err := s.Check(cmd); err == nil || !strings.Contains(err.Error(), "denied by the permissions rule") {
			t.Errorf("%q: err = %v, want a rule denial", cmd, err)
		}
	}
}

func TestShellWindowsRules(t *testing.T) {
	root, _, _ := setup(t)
	j, _ := NewJail(root, nil, nil)
	user, _ := ParseRuleSet("user", []string{"git"}, nil, []string{"git push *"})
	s := NewShell(NewRules(user), j, "windows")
	if _, _, _, err := s.Check("git push origin"); err == nil || !strings.Contains(err.Error(), "denied by the permissions rule") {
		t.Errorf("windows deny: %v", err)
	}
	if need, _, _, err := s.Check(`C:\tools\py.exe x`); err != nil || !slices.Equal(need, []string{`c:\tools\py`}) {
		t.Errorf("windows path head: need=%v err=%v", need, err)
	}
}

func TestAllowLearnsExactHead(t *testing.T) {
	s := newTestShell(t)
	s.Allow("./tool")
	if need, _, _, _ := s.Check("./tool x"); len(need) != 0 {
		t.Errorf("learned ./tool: need=%v", need)
	}
	if need, _, _, _ := s.Check("/tmp/tool x"); !slices.Equal(need, []string{"/tmp/tool"}) {
		t.Errorf("other path: need=%v", need)
	}
}

func TestLegacyAllowCannotLiftAsk(t *testing.T) {
	root, _, _ := setup(t)
	j, _ := NewJail(root, nil, nil)
	s := NewShell(RulesFromAllow([]string{"rm", "*", "rm *", "sudo", "del", "rtk *"}), j, "linux")
	cases := map[string]verdict{
		"rm x":        {every: []string{"rm"}},
		"python x":    {need: []string{"python"}},
		"sudo ls":     {deny: "hard-deny"},
		"rtk git log": {need: []string{"rtk", "git"}},
		"del x":       {need: []string{"del"}},
	}
	for cmd, want := range cases {
		need, every, unsure, err := s.Check(cmd)
		assertVerdict(t, cmd, want, need, every, unsure, err)
	}
	w := NewShell(RulesFromAllow([]string{"del"}), j, "windows")
	if _, every, _, _ := w.Check("del x"); !slices.Equal(every, []string{"del"}) {
		t.Errorf("windows del: every=%v", every)
	}
	g := NewShell(RulesFromAllow([]string{"git"}), j, "linux")
	if need, every, _, err := g.Check("git log"); err != nil || len(need)+len(every) != 0 {
		t.Errorf("git log: %v %v %v", need, every, err)
	}
	if got := LegacyAllowNames([]string{"git", "rm", "a b", "x*", "", "del"}); !slices.Equal(got, []string{"git"}) {
		t.Errorf("LegacyAllowNames = %v", got)
	}
}

func TestAllowWinAskOnlyOnWindows(t *testing.T) {
	root, _, _ := setup(t)
	j, _ := NewJail(root, nil, nil)
	l := NewShell(NewRules(), j, "linux")
	l.Allow("rmdir")
	if need, _, _, _ := l.Check("rmdir x"); len(need) != 0 {
		t.Errorf("linux rmdir should be learnable: %v", need)
	}
	w := NewShell(NewRules(), j, "windows")
	w.Allow("rmdir")
	if _, every, _, _ := w.Check("rmdir x"); !slices.Equal(every, []string{"rmdir"}) {
		t.Errorf("windows rmdir must keep asking: %v", every)
	}
}

func TestShellWindowsRuleWords(t *testing.T) {
	root, _, _ := setup(t)
	j, _ := NewJail(root, nil, nil)
	user, _ := ParseRuleSet("user", []string{"git"}, nil, []string{"git push *"})
	s := NewShell(NewRules(user), j, "windows")
	for _, cmd := range []string{`git "push" origin`, `git $p origin`, "git `push` origin", `git 'push' origin`, "git push origin"} {
		if _, _, _, err := s.Check(cmd); err == nil || !strings.Contains(err.Error(), "denied by the permissions rule") {
			t.Errorf("%q: err = %v, want a rule denial", cmd, err)
		}
	}
	if need, every, _, err := s.Check("git status"); err != nil || len(need)+len(every) != 0 {
		t.Errorf("git status: %v %v %v", need, every, err)
	}
}
