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
func (s *Shell) checkWindows(command string) ([]string, []string, []string, error) {
	var v verdictAcc
	classify := func(fields []string) error {
		ws := make([]Word, len(fields))
		for i, f := range fields {
			ws[i] = Word{Text: f, Literal: i == 0 || !strings.ContainsAny(f, "$`\"'(){}@%")}
		}
		ws[0].Text = winName(fields[0])
		return s.classify(ws, &v, cmdState{})
	}
	for _, seg := range winSplit.Split(command, -1) {
		fields := strings.Fields(seg)
		if len(fields) == 0 {
			continue
		}
		name := strings.TrimSuffix(strings.ToLower(fields[0]), ".exe")
		if err := classify(fields); err != nil {
			return nil, nil, nil, err
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
					return nil, nil, nil, fmt.Errorf("cannot analyse `rtk %s -- %s`: the word after `--` is option-shaped; run the command without the option", sub, fields[j])
				}
			}
			if j < len(fields) {
				if err := classify(fields[j:]); err != nil {
					return nil, nil, nil, err
				}
			}
		default:
			if err := classify(fields[j:]); err != nil {
				return nil, nil, nil, err
			}
		}
	}
	return v.need, v.every, v.unsure, nil
}

// winName normalizes a best-effort token: lowercase, surrounding quotes
// trimmed, `.exe` stripped.
func winName(tok string) string {
	return strings.TrimSuffix(strings.Trim(strings.ToLower(tok), `'"`), ".exe")
}
