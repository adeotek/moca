package permissions

import (
	"fmt"
	"regexp"
	"strings"
)

var winSplit = regexp.MustCompile(`&&|\|\||[;|]`)

var winAsk = map[string]bool{"remove-item": true, "rm": true, "ri": true, "del": true, "erase": true, "rd": true, "rmdir": true}

// checkWindows is best-effort (no PowerShell parser in v1, §7): the first
// token of every ;/|/&&/|| segment is checked. Documented as weaker than Unix.
func (s *Shell) checkWindows(command string) ([]string, []string, error) {
	var v verdictAcc
	for _, seg := range winSplit.Split(command, -1) {
		fields := strings.Fields(seg)
		if len(fields) == 0 {
			continue
		}
		name := strings.TrimSuffix(strings.ToLower(fields[0]), ".exe")
		switch {
		case name == "format-volume" || hardDeny[name]:
			return nil, nil, fmt.Errorf("%s is never allowed (hard-deny)", name)
		case winAsk[name]:
			v.add(&v.every, name)
		case builtinsOK[name]:
		case !s.allowed(name):
			v.add(&v.need, name)
		}
	}
	return v.need, v.every, nil
}
