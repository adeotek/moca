package permissions

import (
	"testing"
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
