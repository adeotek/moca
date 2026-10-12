package tools

// fakeCmds is the CommandChecker test double (untagged: plan_test.go uses
// it on every OS, the shell suites on Unix).
type fakeCmds struct {
	need, every []string
	unsure      []string
	err         error
	allowed     []string
}

func (f *fakeCmds) Check(string) ([]string, []string, []string, error) {
	return f.need, f.every, f.unsure, f.err
}
func (f *fakeCmds) Allow(n string) { f.allowed = append(f.allowed, n) }
