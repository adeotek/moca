package permissions

// The PowerShell risk table (SPECS §11.2). The Unix table in auto.go classifies
// Unix spellings; PowerShell's cmdlets are `Verb-Noun`, so none of them ever
// matched a row there and every one of them reached the classifier — measured
// before this file existed: `Get-ChildItem`, `Get-Content`, `Select-String`,
// `Test-Path`, `dir`, `type` and `copy` were all unsure (auto) or a prompt
// (ask). This table is CURATED, never verb-derived: a `Get-*` cmdlet from a
// third-party module runs arbitrary code, so only a named cmdlet may be
// allowed, and each row's justification is "no side effect outside the
// confinement" — a read may leave the workdir (it lands in the unsure tier,
// as on Unix), a write may not.
//
// The parser is the same best-effort word model the rest of checkWindows uses:
// no quoting model, no parameter binding. A parameter the table does not know
// is `unsure` (fail closed, exactly like an unmodelled getopt option on the
// Unix side), and an operand whose value cannot be known is judged by
// winOperand, which never allows one.

import (
	"regexp"
	"strings"
)

// winKind is what a cmdlet can do to the filesystem.
type winKind int

const (
	winKindRead     winKind = iota // reads only
	winKindConfined                // writes, judged by its operands
	winKindFilter                  // runs a scriptblock; allowed only without one
)

// winPos says which positional operands are path operands (and on which side).
type winPos int

const (
	posNone             winPos = iota // every positional is data (a property name, a pattern)
	posReadAll                        // every positional is a path read
	posPatternThenPaths               // positional 0 is data, the rest are path reads
	posSrcsDst                        // every positional but the last reads; the last writes
	posFirstWrite                     // positional 0 writes; the rest are data
)

// winParam is what a parameter does to the word after it.
type winParam int

const (
	pSwitch winParam = iota // takes no value
	pValue                  // takes a value that is not a path
	pPath                   // takes a path operand it reads
	pDst                    // takes a path operand it writes
)

// winSpec is one cmdlet's row.
type winSpec struct {
	kind   winKind
	guard  rootGuard           // confined only: which operands are guarded
	pos    winPos              // positional operand roles
	params map[string]winParam // parameter names, lowercased, without '-'
}

// winCommonParams are the parameters every cmdlet accepts. The switches are
// here because a missing row makes an ordinary command unsure; the value ones
// are only the ones that never name a path.
var winCommonParams = map[string]winParam{
	"verbose": pSwitch, "debug": pSwitch, "whatif": pSwitch, "confirm": pSwitch,
	"passthru": pSwitch, "force": pSwitch,
	"erroraction": pValue, "warningaction": pValue, "informationaction": pValue,
	"errorvariable": pValue, "warningvariable": pValue, "informationvariable": pValue,
	"outvariable": pValue, "outbuffer": pValue, "pipelinevariable": pValue,
}

// wp merges a cmdlet's own parameters over the common set. It runs at package
// initialization; a map is never written after that.
func wp(own map[string]winParam) map[string]winParam {
	out := make(map[string]winParam, len(winCommonParams)+len(own))
	for k, v := range winCommonParams {
		out[k] = v
	}
	for k, v := range own {
		out[k] = v
	}
	return out
}

// winAlias maps a PowerShell alias to its cmdlet. Only aliases whose meaning is
// fixed across Windows PowerShell 5.1 and 7 are here, and `sc` is deliberately
// absent: `sc.exe` is the service-control binary and `sc` is also Set-Content's
// alias, so one word cannot mean both — it stays in winAlwaysAsk.
var winAlias = map[string]string{
	"gci": "get-childitem", "dir": "get-childitem", "ls": "get-childitem",
	"gc": "get-content", "cat": "get-content", "type": "get-content",
	"gi": "get-item", "sls": "select-string", "gl": "get-location",
	"select": "select-object", "sort": "sort-object", "measure": "measure-object",
	"group": "group-object", "compare": "compare-object",
	"ft": "format-table", "fl": "format-list", "fw": "format-wide", "fc": "format-custom",
	"write": "write-output", "tee": "tee-object",
	"copy": "copy-item", "cp": "copy-item", "cpi": "copy-item",
	"move": "move-item", "mv": "move-item", "mi": "move-item",
	"ren": "rename-item", "rni": "rename-item",
	"ni": "new-item", "md": "new-item", "mkdir": "new-item",
	"ac": "add-content", "where": "where-object", "?": "where-object",
	"foreach": "foreach-object", "%": "foreach-object",
}

// winCmdlets is the table. A cmdlet missing from it falls through to the Unix
// table (which still decides the aliases that spell a Unix tool: ls, cat, mkdir)
// or to the classifier.
var winCmdlets = map[string]winSpec{
	// Reads. Every one of these may read outside the confinement — that lands
	// in the unsure tier (readOK, as on Unix), never in an allow.
	"get-childitem": {kind: winKindRead, pos: posReadAll, params: wp(map[string]winParam{
		"path": pPath, "literalpath": pPath,
		"filter": pValue, "include": pValue, "exclude": pValue, "depth": pValue, "attributes": pValue,
		"recurse": pSwitch, "name": pSwitch, "file": pSwitch, "directory": pSwitch,
		"hidden": pSwitch, "readonly": pSwitch, "system": pSwitch, "followsymlink": pSwitch,
	})},
	"get-item": {kind: winKindRead, pos: posReadAll, params: wp(map[string]winParam{
		"path": pPath, "literalpath": pPath, "stream": pValue,
	})},
	"get-content": {kind: winKindRead, pos: posReadAll, params: wp(map[string]winParam{
		"path": pPath, "literalpath": pPath,
		"readcount": pValue, "totalcount": pValue, "tail": pValue, "head": pValue,
		"delimiter": pValue, "encoding": pValue, "stream": pValue,
		"raw": pSwitch, "wait": pSwitch, "asbytestream": pSwitch,
	})},
	"select-string": {kind: winKindRead, pos: posPatternThenPaths, params: wp(map[string]winParam{
		"pattern": pValue, "path": pPath, "literalpath": pPath,
		"include": pValue, "exclude": pValue, "context": pValue, "encoding": pValue,
		"simplematch": pSwitch, "casesensitive": pSwitch, "notmatch": pSwitch,
		"quiet": pSwitch, "list": pSwitch, "allmatches": pSwitch, "raw": pSwitch,
	})},
	"test-path": {kind: winKindRead, pos: posReadAll, params: wp(map[string]winParam{
		"path": pPath, "literalpath": pPath,
		"type": pValue, "filter": pValue, "isvalid": pValue, "pathtype": pValue,
		"olderthan": pValue, "newerthan": pValue,
	})},
	"resolve-path": {kind: winKindRead, pos: posReadAll, params: wp(map[string]winParam{
		"path": pPath, "literalpath": pPath, "relative": pValue, "relativebasepath": pValue,
	})},

	// Object-and-pipeline cmdlets: positional words are property names or
	// objects, never paths, so they carry no filesystem side effect at all.
	"get-location": {kind: winKindRead, pos: posNone, params: wp(map[string]winParam{
		"psprovider": pValue, "pscurrentlocation": pSwitch, "stack": pSwitch,
	})},
	"get-date": {kind: winKindRead, pos: posNone, params: wp(map[string]winParam{
		"date": pValue, "format": pValue, "uformat": pValue, "displayhint": pValue,
	})},
	"get-help": {kind: winKindRead, pos: posNone, params: wp(map[string]winParam{
		"name": pValue, "category": pValue, "component": pValue, "functionality": pValue,
		"parameter": pValue, "detailed": pSwitch, "full": pSwitch, "examples": pSwitch,
		"online": pSwitch, "showwindow": pSwitch,
	})},
	"get-command": {kind: winKindRead, pos: posNone, params: wp(map[string]winParam{
		"name": pValue, "module": pValue, "commandtype": pValue, "verb": pValue, "noun": pValue,
		"all": pSwitch, "totalcount": pSwitch, "syntax": pSwitch,
	})},
	"get-module": {kind: winKindRead, pos: posNone, params: wp(map[string]winParam{
		"name": pValue, "listavailable": pSwitch, "all": pSwitch,
	})},
	"get-host": {kind: winKindRead, pos: posNone, params: wp(nil)},
	"get-psdrive": {kind: winKindRead, pos: posNone, params: wp(map[string]winParam{
		"name": pValue, "psprovider": pValue, "scope": pValue,
	})},
	"measure-object": {kind: winKindRead, pos: posNone, params: wp(map[string]winParam{
		"property": pValue,
		"sum":      pSwitch, "average": pSwitch, "maximum": pSwitch, "minimum": pSwitch,
		"standarddeviation": pSwitch, "line": pSwitch, "word": pSwitch, "character": pSwitch,
	})},
	"sort-object": {kind: winKindRead, pos: posNone, params: wp(map[string]winParam{
		"property": pValue, "top": pValue, "bottom": pValue,
		"descending": pSwitch, "unique": pSwitch, "stable": pSwitch, "casesensitive": pSwitch,
	})},
	"group-object": {kind: winKindRead, pos: posNone, params: wp(map[string]winParam{
		"property":      pValue,
		"casesensitive": pSwitch, "asstring": pSwitch, "ashashtable": pSwitch, "noelement": pSwitch,
	})},
	"compare-object": {kind: winKindRead, pos: posNone, params: wp(map[string]winParam{
		"referenceobject": pValue, "differenceobject": pValue, "property": pValue, "syncwindow": pValue,
		"casesensitive": pSwitch, "includeequal": pSwitch, "excludedifferent": pSwitch,
	})},
	"select-object": {kind: winKindRead, pos: posNone, params: wp(map[string]winParam{
		"property": pValue, "excludeproperty": pValue, "first": pValue, "last": pValue,
		"skip": pValue, "index": pValue, "expandproperty": pValue, "inputobject": pValue, "wait": pValue,
		"unique": pSwitch,
	})},
	"format-table": {kind: winKindRead, pos: posNone, params: wp(map[string]winParam{
		"property": pValue, "groupby": pValue, "wrap": pValue, "view": pValue,
		"autosize": pSwitch, "hidetableheaders": pSwitch, "repeatheader": pSwitch,
	})},
	"format-list": {kind: winKindRead, pos: posNone, params: wp(map[string]winParam{
		"property": pValue, "groupby": pValue, "view": pValue,
	})},
	"format-wide": {kind: winKindRead, pos: posNone, params: wp(map[string]winParam{
		"property": pValue, "column": pValue, "groupby": pValue, "view": pValue, "autosize": pSwitch,
	})},
	"format-custom": {kind: winKindRead, pos: posNone, params: wp(map[string]winParam{
		"property": pValue, "depth": pValue, "view": pValue,
	})},
	"out-string": {kind: winKindRead, pos: posNone, params: wp(map[string]winParam{
		"stream": pValue, "width": pValue,
	})},
	"write-output": {kind: winKindRead, pos: posNone, params: wp(map[string]winParam{
		"inputobject": pValue, "noenumerate": pSwitch,
	})},
	"convertto-json": {kind: winKindRead, pos: posNone, params: wp(map[string]winParam{
		"depth": pValue, "compress": pSwitch, "enumsasstrings": pSwitch, "asarray": pSwitch,
	})},
	"convertfrom-json": {kind: winKindRead, pos: posNone, params: wp(map[string]winParam{
		"depth": pValue, "ashashtable": pSwitch,
	})},
	"convertto-csv": {kind: winKindRead, pos: posNone, params: wp(map[string]winParam{
		"delimiter": pValue, "useCulture": pSwitch, "notypeinformation": pSwitch,
	})},
	"convertfrom-csv": {kind: winKindRead, pos: posNone, params: wp(map[string]winParam{
		"delimiter": pValue, "useCulture": pSwitch,
	})},
	"convertto-xml": {kind: winKindRead, pos: posNone, params: wp(map[string]winParam{
		"depth": pValue, "as": pValue, "inputobject": pValue, "noTypeInformation": pSwitch,
	})},
	"convertfrom-xml": {kind: winKindRead, pos: posNone, params: wp(nil)},

	// Writers. Every destination must be literal and confined; a destination
	// that is the workdir root itself is ordinary for the copy/create family
	// (Unix guardNone) and asks for the ones that write into a file (the Unix
	// tee/set-content shape, writeTargets allowRoot=false).
	"new-item": {kind: winKindConfined, guard: guardNone, pos: posFirstWrite, params: wp(map[string]winParam{
		"path": pPath, "name": pDst, "itemtype": pValue, "value": pValue,
	})},
	"copy-item": {kind: winKindConfined, guard: guardNone, pos: posSrcsDst, params: wp(map[string]winParam{
		"path": pPath, "literalpath": pPath, "destination": pDst,
		"container": pSwitch, "recurse": pSwitch,
		"filter": pValue, "include": pValue, "exclude": pValue,
	})},
	"move-item": {kind: winKindConfined, guard: guardNone, pos: posSrcsDst, params: wp(map[string]winParam{
		"path": pPath, "literalpath": pPath, "destination": pDst, "recurse": pSwitch,
		"filter": pValue, "include": pValue, "exclude": pValue,
	})},
	"rename-item": {kind: winKindConfined, guard: guardNone, pos: posSrcsDst, params: wp(map[string]winParam{
		"path": pPath, "literalpath": pPath, "destination": pDst, "newname": pValue,
	})},
	"set-content": {kind: winKindConfined, guard: guardAll, pos: posFirstWrite, params: wp(map[string]winParam{
		"path": pDst, "literalpath": pDst, "value": pValue, "encoding": pValue, "stream": pValue,
		"nonewline": pSwitch, "append": pSwitch, "asbytestream": pSwitch,
	})},
	"add-content": {kind: winKindConfined, guard: guardAll, pos: posFirstWrite, params: wp(map[string]winParam{
		"path": pDst, "literalpath": pDst, "value": pValue, "encoding": pValue, "stream": pValue,
		"nonewline": pSwitch, "append": pSwitch, "asbytestream": pSwitch,
	})},
	"out-file": {kind: winKindConfined, guard: guardAll, pos: posFirstWrite, params: wp(map[string]winParam{
		"path": pDst, "literalpath": pDst, "filepath": pDst,
		"encoding": pValue, "width": pValue,
		"append": pSwitch, "nonewline": pSwitch, "noclobber": pSwitch,
	})},
	"tee-object": {kind: winKindConfined, guard: guardAll, pos: posFirstWrite, params: wp(map[string]winParam{
		"path": pDst, "literalpath": pDst, "filepath": pDst,
		"variable": pValue, "encoding": pValue, "append": pSwitch,
	})},
}

// winAbsPath reports a word whose raw text names an absolute or provider path
// even when its runtime value cannot be known (`C:\x`, `\x`, `/x`,
// `HKLM:\…`, `-Path:C:\x`). Such a word is asked about rather than handed to
// the classifier: the one thing the word model can still tell is that it is
// not relative to the workdir.
var winAbsPath = regexp.MustCompile(`^([A-Za-z]:|[\\/]|[A-Za-z][A-Za-z0-9_.-]*:)`)

func winAbsPathText(s string) bool {
	return winAbsPath.MatchString(strings.Trim(s, `"'`))
}

// winOp is one positional operand and what it may do.
type winOp struct {
	w    Word
	role int // opData, opRead, opWrite
}

const (
	opData = iota
	opRead
	opWrite
)

// winCanon resolves an alias to its cmdlet.
func winCanon(name string) string {
	if c, ok := winAlias[name]; ok {
		return c
	}
	return name
}

// winVerdict judges a PowerShell cmdlet the Unix table cannot. ok=false means
// the table has no row — the caller falls through to autoClassify.
func winVerdict(ws []Word, conf pathFunc) (autoVerdict, bool) {
	if len(ws) == 0 {
		return autoUnsure, false
	}
	name := winCanon(strings.ToLower(strings.Trim(ws[0].Text, `"'`)))
	spec, ok := winCmdlets[name]
	if !ok {
		return autoUnsure, false
	}
	if winDynamic(ws[1:]) {
		return autoAsk, true // a scriptblock or a subexpression runs anything
	}
	if winRedirect(ws[1:]) {
		// The word model has no redirect model (SPECS §11.2): the caller adds
		// its own unsure for a segment with `<`/`>`, and this table must not
		// pretend to place the target. Fall through with no row.
		return autoUnsure, false
	}
	switch spec.kind {
	case winKindRead:
		return winReadVerdict(ws[1:], spec, conf), true
	case winKindConfined:
		return winConfinedVerdict(ws[1:], spec, conf), true
	default:
		return autoUnsure, false
	}
}

// winReadVerdict judges a read cmdlet: every path operand must be readable,
// and a read outside the confinement is the classifier's call (as on Unix).
func winReadVerdict(args []Word, spec winSpec, conf pathFunc) autoVerdict {
	ops, v := winOperands(args, spec)
	for _, op := range ops {
		if op.role != opRead {
			continue
		}
		v = v.and(winOperand(op.w, conf, false, false))
	}
	return v
}

// winOperands splits a cmdlet's arguments into positional operands (with their
// roles) and a verdict for the parameters it could not model. A parameter is
// consumed with its value; an attached value (`-Path:x`) is judged on its raw
// text, because a non-literal word still says whether it is absolute.
func winOperands(args []Word, spec winSpec) ([]winOp, autoVerdict) {
	var ops []winOp
	v := autoAllow
	for i := 0; i < len(args); i++ {
		a := args[i]
		if strings.HasPrefix(a.Text, "-") && a.Text != "-" && a.Text != "--" {
			name, _, has := winParamName(a.Text)
			p, known := spec.params[name]
			if !known {
				v = v.and(autoUnsure)
				continue
			}
			if has {
				// `-Name:value`: the value rides in the word, so it cannot be
				// placed — but a write target that names a drive or provider
				// path still asks, which is the one fact the text carries.
				switch p {
				case pPath:
					v = v.and(autoUnsure)
				case pDst:
					v = v.and(autoAsk)
				}
				continue
			}
			switch p {
			case pSwitch:
			case pValue:
				if i+1 < len(args) {
					i++
				}
			case pPath, pDst:
				if i+1 < len(args) {
					i++
					role := opRead
					if p == pDst {
						role = opWrite
					}
					ops = append(ops, winOp{w: args[i], role: role})
				}
			}
			continue
		}
		ops = append(ops, winOp{w: a})
	}
	winRoles(ops, spec.pos)
	return ops, v
}

// winRoles assigns each positional operand its role from the row's winPos.
func winRoles(ops []winOp, pos winPos) {
	for i := range ops {
		switch pos {
		case posReadAll:
			ops[i].role = opRead
		case posPatternThenPaths:
			if i == 0 {
				ops[i].role = opData
			} else {
				ops[i].role = opRead
			}
		case posSrcsDst:
			if i == len(ops)-1 {
				ops[i].role = opWrite
			} else {
				ops[i].role = opRead
			}
		case posFirstWrite:
			if i == 0 {
				ops[i].role = opWrite
			} else {
				ops[i].role = opData
			}
		default: // posNone
			ops[i].role = opData
		}
	}
}

// winParamName splits a parameter word into its name and, for the attached
// `-Name:value` form, the value.
func winParamName(text string) (name, attached string, hasValue bool) {
	name = strings.TrimPrefix(text, "-")
	if i := strings.IndexByte(name, ':'); i >= 0 {
		return strings.ToLower(name[:i]), name[i+1:], true
	}
	return strings.ToLower(name), "", false
}

// winOperand judges one operand. A non-literal word is `unsure` — unless it is
// a WRITE target whose raw text is an absolute/provider path: the word model
// can still read that much, and a write outside the confinement must ask
// rather than be handed to the classifier (`Set-Content C:\Windows\x`). A read
// follows Unix: outside the confinement is the classifier's call.
func winOperand(w Word, conf pathFunc, write, guarded bool) autoVerdict {
	if !w.Literal {
		if write {
			// On Windows this table is the only gate for a write target — there
			// is no redirect walk to fall back on — so a destination the words
			// cannot place asks every time rather than reaching the classifier
			// (which cannot place it either). This is the existing `tee` rule.
			return autoAsk
		}
		return autoUnsure
	}
	if write {
		switch conf(w.Text, true) {
		case pathOK:
			return autoAllow
		case pathRoot:
			// Writing AT the workdir root is ordinary for cp/mkdir-shaped
			// cmdlets (Unix guardNone) and asks for the ones that write INTO
			// a file (tee, set-content — Unix writeTargets allowRoot=false).
			if !guarded {
				return autoAllow
			}
		}
		return autoAsk // .git, outside, unresolvable, or a guarded root
	}
	if readOK(conf(w.Text, false)) {
		return autoAllow
	}
	return autoUnsure
}

// winConfinedVerdict judges a writer: every source must be readable, every
// destination must be a literal path inside the workdir or a temp root, and a
// recursive copy or move onto the workdir root is the classifier's call — a
// tree merged into the root may carry a .git (the Unix `cp -r evil/. .` rule).
func winConfinedVerdict(args []Word, spec winSpec, conf pathFunc) autoVerdict {
	ops, v := winOperands(args, spec)
	guarded := spec.guard != guardNone
	mergeRoot := spec.pos == posSrcsDst && winRecursive(args)
	for i, op := range ops {
		if op.role == opData {
			continue
		}
		write := op.role == opWrite
		ov := winOperand(op.w, conf, write, guarded)
		if write && mergeRoot && i == len(ops)-1 && op.w.Literal &&
			conf(op.w.Text, true) == pathRoot {
			ov = autoUnsure // the classifier judges it, as on Unix
		}
		v = v.and(ov)
	}
	return v
}

// winRecursive reports a recursive cmdlet invocation (-Recurse / -r).
func winRecursive(args []Word) bool {
	for _, a := range args {
		name, _, _ := winParamName(a.Text)
		if name == "recurse" || name == "r" {
			return true
		}
	}
	return false
}

// winRedirect reports a segment that redirects. The word model cannot tell a
// redirect from a string containing the character, so a row that placed the
// target would be guessing; the caller's own redirect rule stays the one that
// decides, and P3's parse replaces both.
func winRedirect(args []Word) bool {
	for _, a := range args {
		t := strings.TrimLeft(a.Text, "0123456789*")
		if strings.HasPrefix(t, ">") || strings.HasPrefix(t, "<") {
			return true
		}
	}
	return false
}

// winDynamic reports an argument that runs or builds a command: a scriptblock,
// a subexpression, or a subexpression-array. The judge cannot see what such an
// argument does either — the command is text to it too — so these ask.
func winDynamic(args []Word) bool {
	for _, a := range args {
		if strings.Contains(a.Text, "{") || strings.Contains(a.Text, "$(") || strings.Contains(a.Text, "@(") {
			return true
		}
	}
	return false
}
