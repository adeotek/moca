package permissions

import (
	"slices"
	"testing"

	"github.com/adeotek/moca/internal/config"
)

// winAuto builds the auto-mode view a Windows session gets, for the table
// tests in this file.
func winAuto(t *testing.T) *Shell {
	t.Helper()
	root, _, _ := setup(t)
	j, err := NewJail(root, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	return NewShell(NewRules(), j, "windows").AutoView(j)
}

// TestAutoWindowsCmdlets is the PowerShell table's policy record: every allow
// row here is a cmdlet with no side effect outside the confinement, and every
// name/method a row relies on is pinned by a row. The measured "before" for
// this table is the review's F1: all of these were unsure (auto) or a prompt
// (ask) because the Unix table has no spelling for a Verb-Noun cmdlet.
func TestAutoWindowsCmdlets(t *testing.T) {
	s := winAuto(t)
	cases := map[string]verdict{
		// plain reads inside the workdir
		"Get-ChildItem":                              {},
		"Get-ChildItem -Recurse src":                 {},
		"Get-ChildItem -File -Force src":             {},
		"Get-Content go.mod":                         {},
		"Get-Content -Tail 20 go.mod":                {},
		"Select-String -Pattern foo -Path README.md": {},
		"Select-String -Pattern foo":                 {},
		"Test-Path go.mod":                           {},
		"Resolve-Path src":                           {},
		"Get-Item go.mod":                            {},
		"Get-Location":                               {},
		"Get-Date":                                   {},
		"Get-Command go":                             {},
		// aliases resolve to their cmdlet
		"dir":         {},
		"ls src":      {},
		"type go.mod": {},
		"gc go.mod":   {},
		"gi go.mod":   {},
		// pipeline filters and object cmdlets: positional words are property
		// names, never paths, so there is no filesystem operand to place
		"Get-ChildItem | Select-Object Name": {},
		"Get-ChildItem | Sort-Object Length": {},
		"Get-ChildItem | Measure-Object":     {},
		"Get-ChildItem | Format-Table":       {},
		"Get-ChildItem | Out-String":         {},
		"Get-ChildItem | ConvertTo-Json":     {},
		"Compare-Object $a $b":               {},
		// .git is readable: the guard is on writes
		"Get-ChildItem -Recurse .git": {},
		// a read outside the confinement stays the classifier's call, exactly
		// as `cat /etc/passwd` does on Unix
		`Get-Content C:\Windows\win.ini`: {unsure: []string{"get-content"}},
		"Get-ChildItem -Recurse C:\\":    {unsure: []string{"get-childitem"}},
		// a parameter the table does not model fails closed
		"Get-ChildItem -NotAParameter src": {unsure: []string{"get-childitem"}},
		"Get-Content -Path:src/x":          {unsure: []string{"get-content"}},
		// a provider path is outside even when its value cannot be known
		"Get-Content HKLM:\\Software": {unsure: []string{"get-content"}},
		// the Unix spellings still come from the Unix table, not this one
		"cat go.mod": {},
		"mkdir out":  {},
	}
	for cmd, want := range cases {
		need, every, unsure, err := s.Check(cmd)
		assertVerdict(t, cmd, want, need, every, unsure, err)
	}
}

// TestAutoWindowsCmdletsAreAskModeNeutral: the table is the auto-mode tier.
// In ask mode a cmdlet is still "needs approval" (the user allowlists what they
// use), so the same rows must NOT come back cleared — a regression here would
// silently widen default mode on Windows.
func TestAutoWindowsCmdletsAreAskModeNeutral(t *testing.T) {
	root, _, _ := setup(t)
	j, _ := NewJail(root, nil, nil)
	ask := NewShell(NewRules(), j, "windows")
	for _, cmd := range []string{"Get-ChildItem", "Get-Content go.mod", "dir", "Get-Location"} {
		need, every, unsure, err := ask.Check(cmd)
		if err != nil || len(need) == 0 || len(every)+len(unsure) != 0 {
			t.Errorf("%q in ask mode: need=%v every=%v unsure=%v err=%v, want one need name", cmd, need, every, unsure, err)
		}
	}
}

// TestAutoWindowsConfinedMutators: a PowerShell writer runs without a prompt
// only when every path it touches is literal and confined, exactly as the Unix
// confined mutators do. The write target is the row's role (posFirstWrite for
// the file writers, the last positional for the copy/rename family), never a
// word the table guessed at.
func TestAutoWindowsConfinedMutators(t *testing.T) {
	s := winAuto(t)
	cases := map[string]verdict{
		// confined destinations
		"New-Item -ItemType Directory out": {},
		"New-Item out/x.txt":               {},
		"Copy-Item a.txt b.txt":            {},
		"Move-Item src old":                {},
		"Rename-Item a.txt b.txt":          {},
		"Set-Content out.txt hi":           {},
		"Add-Content out.txt more":         {},
		"Out-File -FilePath out.txt":       {},
		"Tee-Object -FilePath out.txt":     {},
		"Get-ChildItem | Out-File out.txt": {},
		// outside, a drive path, a provider path, .git and the guarded root
		`Set-Content C:\Windows\x hi`:   {every: []string{"set-content"}},
		`Copy-Item a.txt C:\temp\b.txt`: {every: []string{"copy-item"}},
		`New-Item -Path .`:              {},
		"Move-Item x .git/y":            {every: []string{"move-item"}},
		"Set-Content . hi":              {every: []string{"set-content"}},
		`Out-File HKLM:\x`:              {every: []string{"out-file"}},
		// a tree merged into the workdir root may carry .git: the classifier
		// judges it (the Unix `cp -r evil/. .` rule)
		"Copy-Item -Recurse . .": {unsure: []string{"copy-item"}},
		// a destination the words cannot place asks every time: this table is
		// the only gate for a Windows write target (no redirect walk)
		"Set-Content $env:TEMP\\x hi": {every: []string{"set-content"}},
	}
	for cmd, want := range cases {
		need, every, unsure, err := s.Check(cmd)
		assertVerdict(t, cmd, want, need, every, unsure, err)
	}
}

// TestAutoWindowsAlwaysAsk: the code runners and machine-state cmdlets, and
// the `sc` collision that keeps the removal aliases and Set-Content apart from
// the service-control binary. Every row here is ask-every-time in auto mode;
// ask mode is unchanged (its own ladder consults askEvery/winAsk only).
func TestAutoWindowsAlwaysAsk(t *testing.T) {
	s := winAuto(t)
	for _, cmd := range []string{
		"Invoke-WebRequest https://example.com",
		"iwr https://example.com",
		"Invoke-RestMethod https://example.com",
		"Add-Type -TypeDefinition x",
		"Install-Module Pester",
		"Start-Job -ScriptBlock { x }",
		"New-Service -Name x -BinaryPathName y",
		"Set-Service x -Status Stopped",
		"Set-ItemProperty HKCU:\\x -Name y -Value z",
		"Get-Credential",
		"Start-Process notepad",
		"foreach ($f in $files) { Remove-Item $f }",
		"while ($true) { Get-ChildItem }",
		"sc query",
		"sc out.txt hi", // sc is sc.exe AND Set-Content's alias: never folded
		// Destructive content writers and machine state with no operand model.
		"Clear-Content x.txt",
		"clc x.txt",
		"Clear-Item x",
		"Set-Item x y",
		"si x y",
		"format c:",
		"diskpart",
		"Clear-Disk -Number 0",
		"Initialize-Disk -Number 0",
		"bcdedit /set testsigning on",
		"netsh advfirewall set allprofiles state off",
		"wmic logicaldisk get size",
		"icacls C:\\x /grant Everyone:F",
		"takeown /f C:\\Windows",
		"Unblock-File downloaded.zip",
		"Set-AuthenticodeSignature x.ps1 -Certificate $c",
	} {
		_, every, unsure, err := s.Check(cmd)
		if err != nil || len(every) == 0 || len(unsure) != 0 {
			t.Errorf("%q: every=%v unsure=%v err=%v, want an ask-every-time name", cmd, every, unsure, err)
		}
	}
}

// TestAutoWindowsFilters: a pipeline filter is allowed only when it carries no
// scriptblock. `Where-Object { … }` and `ForEach-Object { … }` run whatever the
// block holds, so they ask; the property-name forms do not.
func TestAutoWindowsFilters(t *testing.T) {
	s := winAuto(t)
	cases := map[string]verdict{
		"Get-ChildItem | Where-Object Name":                 {},
		"Get-ChildItem | Where-Object -Property Name -EQ x": {},

		"Get-ChildItem | ? Name":                           {},
		"Get-ChildItem | ForEach-Object Name":              {},
		"Get-ChildItem | % Name":                           {},
		"Get-ChildItem | Where-Object { $_.Length -gt 0 }": {every: []string{"where-object"}},
		"Get-ChildItem | ForEach-Object { Remove-Item x }": {every: []string{"foreach-object"}},
		// a filter's own output written outside is still a write
		"Get-ChildItem | Out-File C:\\x":                   {every: []string{"out-file"}},
		"Get-ChildItem | ConvertTo-Json | Out-File .git/x": {every: []string{"out-file"}},
	}
	for cmd, want := range cases {
		need, every, unsure, err := s.Check(cmd)
		assertVerdict(t, cmd, want, need, every, unsure, err)
	}
}

// TestAutoWindowsScriptblockAsks: a scriptblock, a subexpression or an
// array-subexpression runs code the table cannot see, and the classifier cannot
// see it either (it receives the command as text), so these ask every time
// rather than land in the unsure tier.
func TestAutoWindowsScriptblockAsks(t *testing.T) {
	s := winAuto(t)
	cases := map[string]verdict{
		"Get-Content $(Join-Path . x)":      {every: []string{"get-content"}},
		"Get-ChildItem -Path @('a', 'b')":   {every: []string{"get-childitem"}},
		"Get-ChildItem -Filter { $_.Name }": {every: []string{"get-childitem"}},
		// the call and dot-source operators run whatever the word holds
		"& $cmd":           {every: []string{"&"}},
		". .\\profile.ps1": {every: []string{"."}},
		// a subexpression in a *later* segment is judged on its own head
		"$x = 'Remove-Item'; & $x .git": {unsure: []string{"$x"}, every: []string{"&"}},
		// and an ordinary read with no block still passes
		"Get-ChildItem -Filter *.go": {},
	}
	for cmd, want := range cases {
		need, every, unsure, err := s.Check(cmd)
		assertVerdict(t, cmd, want, need, every, unsure, err)
	}
}

// TestAutoWindowsParameterRolesAreNotReassigned: a parameter already decided
// which word is the destination, so the positional sequence must not relabel
// it. Before the fix, `Copy-Item -Destination C:\Windows\System32 -Path a.txt`
// swapped the roles — the outside destination was judged as a READ (unsure, the
// classifier's call) while the in-workdir source became the write, and
// `Select-String -Path *.go -Pattern x` dropped the glob as data entirely and
// came back ALLOW without judging a single path.
func TestAutoWindowsParameterRolesAreNotReassigned(t *testing.T) {
	rows := map[string]verdict{
		`Copy-Item -Destination C:\Windows\System32 -Path a.txt`: {every: []string{"copy-item"}},
		`Copy-Item -Path a.txt -Destination C:\Windows\System32`: {every: []string{"copy-item"}},
		`Move-Item -Destination C:\x -Path a.txt`:                {every: []string{"move-item"}},
		// The glob is a read the table cannot place: unsure, never allow.
		`Select-String -Path *.go -Pattern x`: {unsure: []string{"select-string"}},
		// The confined shapes still clear with the parameter form.
		`Copy-Item -Path a -Destination b`: {every: nil},
		`Set-Content -Path out.txt hi`:     {every: nil},
		`New-Item -ItemType Directory foo`: {every: nil},
	}
	s := winAuto(t)
	for cmd, want := range rows {
		need, every, unsure, err := s.Check(cmd)
		assertVerdict(t, cmd, want, need, every, unsure, err)
	}
}

// TestAutoWindowsQuotedLiterals: a word that is one quoted string is data, not
// an unknown. Single quotes never evaluate in PowerShell at all; double quotes
// evaluate only `$` and backtick. Reading them as "unknowable" made every quoted
// argument a classifier call, and — worse — let a quoted path dodge the checks
// that run on literal words: `Set-Content '.moca/skills/x/SKILL.md'` came back
// as an ordinary prompt instead of the skills refusal, and `'..\x'` skipped the
// traversal rule.
func TestAutoWindowsQuotedLiterals(t *testing.T) {
	s := winAuto(t)
	cleared := []string{
		`Get-Content 'go.mod'`,
		`Get-ChildItem '*.go'`,
		`Set-Content 'out.json' '{"a":1}'`,
		`Set-Content "out.json" "hi"`,
		`Copy-Item 'a.txt' 'b.txt'`,
		`Get-Item './go.mod'`,
	}
	for _, c := range cleared {
		if need, every, unsure, err := s.Check(c); err != nil || len(need)+len(every)+len(unsure) != 0 {
			t.Errorf("%q: need=%v every=%v unsure=%v err=%v, want cleared", c, need, every, unsure, err)
		}
	}
	asked := []string{
		`Set-Content "$env:TEMP\out.json" hi`, // double quotes interpolate
		"Set-Content \"`out.json`\" hi",       // backtick escape
		`Get-ChildItem "$(Get-X)"`,            // subexpression runs code
		`Set-Content '..\evil.txt' hi`,        // quoted traversal is still a traversal
		`Set-Content 'a;b.txt' hi`,            // the split ignores quotes: fails closed
	}
	for _, c := range asked {
		if _, every, _, err := s.Check(c); err != nil || len(every) == 0 {
			t.Errorf("%q: every=%v err=%v, want an ask", c, every, err)
		}
	}
	// The skills guard is the point of the literal flag: quoting the path must not
	// downgrade the refusal to a prompt.
	for _, c := range []string{
		`Set-Content '.moca/skills/x/SKILL.md' hi`,
		`Out-File '.moca/skills/x/SKILL.md'`,
		`Add-Content ".moca/skills/x/SKILL.md" hi`,
		`Set-Content .moca/skills/x/SKILL.md hi`,
	} {
		if _, _, _, err := s.Check(c); err == nil {
			t.Errorf("%q: want the skills-write refusal, got none", c)
		}
	}
}

// TestWindowsDefaultAllowlistIsAllLearnable: the Windows built-in allowlist is
// consumed by LegacyAllowNames, which drops hard-deny, `rm` and the removal
// aliases. A member the filter drops is a promise the config cannot keep, so
// every entry must survive it — and every entry must actually clear its cmdlet.
func TestWindowsDefaultAllowlistIsAllLearnable(t *testing.T) {
	allow := config.DefaultShellAllowFor("windows")
	kept := LegacyAllowNames(slices.Clone(allow))
	if !slices.Equal(kept, allow) {
		t.Fatalf("LegacyAllowNames dropped entries: kept %v of %v", kept, allow)
	}
	root, _, _ := setup(t)
	j, _ := NewJail(root, nil, nil)
	s := NewShell(RulesFromAllow(allow), j, "windows").AutoView(j)
	for _, cmd := range []string{"Get-ChildItem", "Get-Content go.mod", "dir", "type go.mod", "Copy-Item a b", "Set-Content out.txt hi"} {
		if need, every, unsure, err := s.Check(cmd); err != nil || len(need)+len(every)+len(unsure) != 0 {
			t.Errorf("%q with the Windows default allowlist: need=%v every=%v unsure=%v err=%v, want cleared", cmd, need, every, unsure, err)
		}
	}
}
