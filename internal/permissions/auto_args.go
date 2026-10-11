package permissions

import "strings"

// Auto risk table: argument splitting and the generic read / write /
// mutator checks shared by the per-tool parsers.

// splitArgs splits ws[from:] into option words and operands; after a
// literal "--" every word is an operand, and a lone "-" is an operand.
func splitArgs(ws []Word, from int) (opts, args []Word) {
	dd := false
	for i := from; i < len(ws); i++ {
		w := ws[i]
		switch {
		case dd || !w.Literal || w.Text == "-" || !strings.HasPrefix(w.Text, "-"):
			args = append(args, w)
		case w.Text == "--":
			dd = true
		default:
			opts = append(opts, w)
		}
	}
	return opts, args
}

// optPath extracts a path-like value attached to an option word
// (`--file=/x`, `-o/x`, `-C..`). Values that do not look like paths
// (`-rf`, `--depth=3`) report false.
func optPath(t string) (string, bool) {
	var v string
	switch {
	case t == "-" || t == "--" || !strings.HasPrefix(t, "-"):
		return "", false
	case strings.Contains(t, "="):
		v = t[strings.Index(t, "=")+1:]
	case !strings.HasPrefix(t, "--") && len(t) > 2:
		v = t[2:]
	default:
		return "", false
	}
	if strings.HasPrefix(v, "/") || strings.HasPrefix(v, "~") || strings.HasPrefix(v, ".") || strings.Contains(v, "/") {
		return v, true
	}
	return "", false
}

// hasOpt reports whether ws[from:] (before any "--") holds the short option
// letter (alone or in a cluster; "" = none) or a long option (bare or
// with =value).
func hasOpt(ws []Word, from int, short string, long ...string) bool {
	opts, _ := splitArgs(ws, from)
	for _, o := range opts {
		t := o.Text
		name, _, _ := strings.Cut(t, "=")
		for _, l := range long {
			if longIs(name, l) {
				return true
			}
		}
		if short != "" && !strings.HasPrefix(t, "--") && strings.ContainsAny(t[1:], short) {
			return true
		}
	}
	return false
}

// longIs reports whether the long option name (before any =) selects full:
// exactly, or as an abbreviation — getopt_long, argparse, optparse and git
// accept any unambiguous prefix (`sort --compress-prog=sh`). Matching every
// prefix over-matches ambiguous ones, which the tools reject anyway.
func longIs(name, full string) bool {
	return name == full || (len(name) >= 3 && strings.HasPrefix(name, "--") && strings.HasPrefix(full, name))
}

// shortFlag reports whether a single-dash cluster in ws[from:] holds c.
func shortFlag(ws []Word, from int, c byte) bool {
	return hasOpt(ws, from, string(c))
}

// optValues returns the values of options in ws[from:]: short letters
// (`-o v`, `-ov`, or ending a cluster `-uo v`) and long names (`--out v`,
// `--out=v`). Clusters are scanned letter by letter, so a value letter
// anywhere in a cluster is caught (over-matching only adds checks).
func optValues(ws []Word, from int, short string, long []string) []Word {
	var out []Word
	for i := from; i < len(ws); i++ {
		t := ws[i].Text
		if !ws[i].Literal || t == "--" {
			if t == "--" && ws[i].Literal {
				break
			}
			continue
		}
		if strings.HasPrefix(t, "--") {
			name, val, eq := strings.Cut(t, "=")
			for _, l := range long {
				switch {
				case !longIs(name, l):
				case eq:
					out = append(out, Word{Text: val, Literal: true})
				case i+1 < len(ws):
					out = append(out, ws[i+1])
				}
			}
			continue
		}
		if !strings.HasPrefix(t, "-") || short == "" {
			continue
		}
		for k := 1; k < len(t); k++ {
			if strings.IndexByte(short, t[k]) < 0 {
				continue
			}
			if k+1 < len(t) {
				out = append(out, Word{Text: t[k+1:], Literal: true})
			} else if i+1 < len(ws) {
				out = append(out, ws[i+1])
			}
			break
		}
	}
	return out
}

// readCheck allows a command whose literal operands and path-like option
// values all resolve inside the confinement (read side); anything outside
// is unsure (rule 17a), as is a non-literal word.
func readCheck(ws []Word, from int, conf pathFunc) autoVerdict {
	opts, args := splitArgs(ws, from)
	for _, o := range opts {
		if p, ok := optPath(o.Text); ok && !readOK(conf(p, false)) {
			return autoUnsure
		}
	}
	for _, a := range args {
		if !a.Literal || !readOK(conf(a.Text, false)) {
			return autoUnsure
		}
		if p, ok := optPath(a.Text); ok && !readOK(conf(p, false)) {
			return autoUnsure // an option-shaped word after --, passed on
		}
	}
	return autoAllow
}

// writeTargets asks unless every write target is literal and confined (a
// non-literal target is unsure). allowRoot admits the roots themselves
// (a directory to write into).
func writeTargets(ts []Word, conf pathFunc, allowRoot bool) autoVerdict {
	v := autoAllow
	for _, t := range ts {
		if !t.Literal {
			v = v.and(autoUnsure)
			continue
		}
		if t.Text == "-" {
			continue // stdout
		}
		switch conf(t.Text, true) {
		case pathOK:
		case pathRoot:
			if !allowRoot {
				return autoAsk
			}
		default:
			return autoAsk
		}
	}
	return v
}

// buildWithWrites is readCheck plus write-target options (a profile, an
// output binary, a temp dir pytest deletes) that must be confined.
func buildWithWrites(ws []Word, from int, conf pathFunc, short string, long []string) autoVerdict {
	return writeTargets(optValues(ws, from, short, long), conf, false).and(readCheck(ws, from, conf))
}

// mutatorVerdict: allow iff every operand and path-like option value is
// literal and confined. An operand outside, inside .git (unless gitOK), or
// a guarded root asks every time; a non-literal operand is unsure.
func mutatorVerdict(ws []Word, from int, conf pathFunc, g rootGuard, gitOK bool) autoVerdict {
	opts, args := splitArgs(ws, from)
	v := autoAllow
	for _, o := range opts {
		name, _, _ := strings.Cut(o.Text, "=")
		if g == guardSources && (longIs(name, "--target-directory") ||
			(!strings.HasPrefix(name, "--") && strings.Contains(name, "t"))) {
			g = guardAll // with -t every operand is a source
		}
		if p, ok := optPath(o.Text); ok {
			v = v.and(mutatorPath(p, conf, false, gitOK))
		}
	}
	return v.and(mutatorArgs(args, conf, g, gitOK))
}

// mutatorArgs judges mutator operands (see mutatorVerdict).
func mutatorArgs(args []Word, conf pathFunc, g rootGuard, gitOK bool) autoVerdict {
	v := autoAllow
	for k, a := range args {
		if !a.Literal {
			v = v.and(autoUnsure)
			continue
		}
		guarded := g == guardAll || (g == guardSources && k < len(args)-1)
		v = v.and(mutatorPath(a.Text, conf, guarded, gitOK))
	}
	return v
}

func mutatorPath(p string, conf pathFunc, guarded, gitOK bool) autoVerdict {
	switch conf(p, true) {
	case pathOK:
		return autoAllow
	case pathRoot:
		if !guarded {
			return autoAllow
		}
	case pathGit:
		if gitOK {
			return autoAllow
		}
	}
	return autoAsk
}
