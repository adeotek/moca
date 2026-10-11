package permissions

import "strings"

// A small getopt_long model for the commands whose option values decide
// where a write lands (cp/mv/ln destinations) or whether a form is allowed
// (git restore --staged, touch/mkdir value words). Scanning letters is not
// enough: in `mv -St . sub` the `t` is -S's value, and `.` is a source.
// The model follows GNU getopt_long: permutation (options after operands),
// `--` ends options, a short letter that takes a value consumes the rest of
// its cluster or the next word, long options accept unambiguous
// abbreviations. It fails closed: an unknown or ambiguous option, a missing
// value, or `=value` on an option without one sets bad.

type argKind int

const (
	noArg  argKind = iota
	reqArg         // --name=value or --name value
	optArg         // --name=value only
)

type getoptSpec struct {
	flags  string             // short letters without a value
	values string             // short letters with a required value
	long   map[string]argKind // long names, with the leading --
}

type optHit struct {
	name  string // "-t" or "--target-directory" (resolved)
	value Word
	has   bool // a value was given
}

type parsedArgs struct {
	opts     []optHit
	operands []Word
	bad      bool
}

func (p parsedArgs) has(names ...string) bool {
	for _, o := range p.opts {
		for _, n := range names {
			if o.name == n {
				return true
			}
		}
	}
	return false
}

func (p parsedArgs) values(names ...string) []Word {
	var out []Word
	for _, o := range p.opts {
		for _, n := range names {
			if o.name == n && o.has {
				out = append(out, o.value)
			}
		}
	}
	return out
}

// resolveLong maps a long option name to its spec entry: exact, or the one
// entry it abbreviates. Ambiguous and unknown names report false.
func resolveLong(name string, spec map[string]argKind) (string, argKind, bool) {
	if k, ok := spec[name]; ok {
		return name, k, true
	}
	match, kind, n := "", noArg, 0
	for full, k := range spec {
		if strings.HasPrefix(full, name) {
			match, kind, n = full, k, n+1
		}
	}
	return match, kind, n == 1
}

func getopt(ws []Word, from int, spec getoptSpec) parsedArgs {
	var p parsedArgs
	dd := false
	for i := from; i < len(ws); i++ {
		w := ws[i]
		t := w.Text
		switch {
		case !w.Literal:
			p.operands = append(p.operands, w)
			p.bad = true // could be an option at run time
		case dd || t == "-" || !strings.HasPrefix(t, "-"):
			p.operands = append(p.operands, w)
		case t == "--":
			dd = true
		case strings.HasPrefix(t, "--"):
			name, val, eq := strings.Cut(t, "=")
			full, kind, ok := resolveLong(name, spec.long)
			if !ok {
				p.bad = true
				continue
			}
			hit := optHit{name: full}
			switch {
			case kind == noArg && eq:
				p.bad = true
			case eq:
				hit.value, hit.has = Word{Text: val, Literal: true}, true
			case kind == reqArg:
				if i+1 >= len(ws) {
					p.bad = true
					continue
				}
				i++
				hit.value, hit.has = ws[i], true
			}
			p.opts = append(p.opts, hit)
		default:
			for k := 1; k < len(t); k++ {
				c := t[k]
				switch {
				case strings.IndexByte(spec.values, c) >= 0:
					hit := optHit{name: "-" + string(c), has: true}
					if k+1 < len(t) {
						hit.value = Word{Text: t[k+1:], Literal: true}
					} else if i+1 < len(ws) {
						i++
						hit.value = ws[i]
					} else {
						p.bad = true
						hit.has = false
					}
					p.opts = append(p.opts, hit)
					k = len(t) // the value consumed the rest of the cluster
				case strings.IndexByte(spec.flags, c) >= 0:
					p.opts = append(p.opts, optHit{name: "-" + string(c)})
				default:
					p.bad = true
				}
			}
		}
	}
	return p
}

// GNU coreutils option tables (cp/mv/ln/touch/mkdir) and git restore.
var (
	cpSpec = getoptSpec{flags: "abdfHilLnPpRrsuvxZT", values: "St", long: map[string]argKind{
		"--archive": noArg, "--attributes-only": noArg, "--backup": optArg, "--copy-contents": noArg,
		"--debug": noArg, "--dereference": noArg, "--force": noArg, "--interactive": noArg, "--link": noArg,
		"--no-clobber": noArg, "--no-dereference": noArg, "--no-preserve": reqArg, "--no-target-directory": noArg,
		"--one-file-system": noArg, "--parents": noArg, "--preserve": optArg, "--recursive": noArg,
		"--reflink": optArg, "--remove-destination": noArg, "--sparse": reqArg, "--strip-trailing-slashes": noArg,
		"--suffix": reqArg, "--symbolic-link": noArg, "--target-directory": reqArg, "--update": optArg,
		"--verbose": noArg, "--keep-directory-symlink": noArg, "--context": optArg}}
	mvSpec = getoptSpec{flags: "bfinuvZT", values: "St", long: map[string]argKind{
		"--backup": optArg, "--context": noArg, "--debug": noArg, "--exchange": noArg, "--force": noArg,
		"--interactive": noArg, "--no-clobber": noArg, "--no-copy": noArg, "--no-target-directory": noArg,
		"--strip-trailing-slashes": noArg, "--suffix": reqArg, "--target-directory": reqArg, "--update": optArg,
		"--verbose": noArg}}
	lnSpec = getoptSpec{flags: "bdFfiLnPrsvT", values: "St", long: map[string]argKind{
		"--backup": optArg, "--directory": noArg, "--force": noArg, "--interactive": noArg, "--logical": noArg,
		"--no-dereference": noArg, "--no-target-directory": noArg, "--physical": noArg, "--relative": noArg,
		"--suffix": reqArg, "--symbolic": noArg, "--target-directory": reqArg, "--verbose": noArg}}
	touchSpec = getoptSpec{flags: "acfhm", values: "drt", long: map[string]argKind{
		"--date": reqArg, "--no-create": noArg, "--no-dereference": noArg, "--reference": reqArg, "--time": reqArg}}
	mkdirSpec = getoptSpec{flags: "pvZ", values: "m", long: map[string]argKind{
		"--mode": reqArg, "--parents": noArg, "--verbose": noArg, "--context": optArg}}
	gitRestoreSpec = getoptSpec{flags: "SWpqm23", values: "s", long: map[string]argKind{
		"--source": reqArg, "--staged": noArg, "--worktree": noArg, "--patch": noArg, "--quiet": noArg,
		"--progress": noArg, "--no-progress": noArg, "--ours": noArg, "--theirs": noArg, "--merge": noArg,
		"--conflict": reqArg, "--ignore-unmerged": noArg, "--ignore-skip-worktree-bits": noArg,
		"--recurse-submodules": noArg, "--no-recurse-submodules": noArg, "--overlay": noArg,
		"--no-overlay": noArg, "--pathspec-from-file": reqArg, "--pathspec-file-nul": noArg}}

	// specMutators: the mutators parsed with a real option spec. dest marks
	// the commands whose -t/--target-directory is a destination.
	specMutators = map[string]struct {
		spec getoptSpec
		dest bool
	}{"cp": {cpSpec, true}, "mv": {mvSpec, true}, "ln": {lnSpec, true}, "touch": {touchSpec, false}, "mkdir": {mkdirSpec, false}}
)

// specMutatorVerdict judges cp/mv/ln/touch/mkdir from the parsed arguments:
// destinations (-t, or the last operand) are checked unguarded, sources
// under g; touch -r is a read; a backup suffix with a slash asks; an
// unknown, ambiguous or conflicting option is unsure. It also returns the
// paths that name the destination or the sources, for the I2d root check.
func specMutatorVerdict(ws []Word, lower string, conf pathFunc, g rootGuard) (autoVerdict, parsedArgs, []Word) {
	sm := specMutators[lower]
	p := getopt(ws, 1, sm.spec)
	v := autoAllow
	if p.bad {
		v = autoUnsure
	}
	for _, s := range p.values("-S", "--suffix") {
		if !s.Literal || strings.ContainsAny(s.Text, `/\`) {
			return autoAsk, p, nil // the backup lands outside the destination's directory
		}
	}
	for _, r := range p.values("-r", "--reference") {
		if lower == "touch" && (!r.Literal || !readOK(conf(r.Text, false))) {
			v = v.and(autoUnsure)
		}
	}
	var dests []Word
	if sm.dest {
		dests = p.values("-t", "--target-directory")
		if len(dests) > 1 || (len(dests) > 0 && p.has("-T", "--no-target-directory")) {
			v = v.and(autoUnsure) // GNU refuses these; do not guess which wins
		}
	}
	for _, d := range dests {
		if !d.Literal {
			v = v.and(autoUnsure)
			continue
		}
		v = v.and(mutatorPath(d.Text, conf, false, false))
	}
	if len(dests) > 0 && g == guardSources {
		g = guardAll // every operand is a source
	}
	v = v.and(mutatorArgs(p.operands, conf, g, false))
	return v, p, append(dests, p.operands...)
}
