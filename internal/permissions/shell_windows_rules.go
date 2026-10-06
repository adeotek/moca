package permissions

import (
	"fmt"
	"regexp"
	"strings"
)

var winSplit = regexp.MustCompile(`&&|\|\||[;|]`)

var winAsk = map[string]bool{"remove-item": true, "rm": true, "ri": true, "del": true, "erase": true, "rd": true, "rmdir": true}

// checkWindows is best-effort (no PowerShell parser in v1, §7): the first
// token of every ;/|/&&/|| segment is checked, plus — for `rtk` — the
// wrapped command word on the same best-effort basis: no quoting model and
// no option tables, so a later `--shell <cmd-string>` shape is only
// approximated, never analysed (review F1/M4). Documented as weaker than
// Unix.
func (s *Shell) checkWindows(command string) ([]string, []string, error) {
	var v verdictAcc
	classify := func(name string) error {
		switch {
		case name == "format-volume" || hardDeny[name]:
			return fmt.Errorf("%s is never allowed (hard-deny)", name)
		case winAsk[name]:
			v.add(&v.every, name)
		case builtinsOK[name]:
		case !s.allowed(name):
			v.add(&v.need, name)
		}
		return nil
	}
	for _, seg := range winSplit.Split(command, -1) {
		fields := strings.Fields(seg)
		if len(fields) == 0 {
			continue
		}
		name := strings.TrimSuffix(strings.ToLower(fields[0]), ".exe")
		if err := classify(name); err != nil {
			return nil, nil, err
		}
		if name != "rtk" {
			continue
		}
		// rtk unwrap, best-effort: find the subcommand word (skipping rtk's
		// value-less flags), then mirror the Unix rule — for a runner the
		// first positional word is the wrapped command name; for the tool
		// class the sub word itself is; for rtkSelf nothing more is run.
		j := 1
		for j < len(fields) && strings.HasPrefix(fields[j], "-") {
			j++
		}
		if j >= len(fields) {
			continue
		}
		sub := winName(fields[j])
		switch {
		case rtkSelf[sub]:
		case rtkRun[sub]:
			j++ // runner flags first, then the `--` separator
			for j < len(fields) && strings.HasPrefix(fields[j], "-") && fields[j] != "--" {
				j++
			}
			if j < len(fields) && fields[j] == "--" {
				j++
				if j < len(fields) && strings.HasPrefix(fields[j], "-") {
					return nil, nil, fmt.Errorf("cannot analyse `rtk %s -- %s`: the word after `--` is option-shaped; run the command without the option", sub, fields[j])
				}
			}
			if j < len(fields) {
				if err := classify(winName(fields[j])); err != nil {
					return nil, nil, err
				}
			}
		default:
			if err := classify(sub); err != nil {
				return nil, nil, err
			}
		}
	}
	return v.need, v.every, nil
}

// winName normalizes a best-effort token: lowercase, surrounding quotes
// trimmed, `.exe` stripped.
func winName(tok string) string {
	return strings.TrimSuffix(strings.Trim(strings.ToLower(tok), `'"`), ".exe")
}
