package permissions

import (
	"slices"
	"strings"
)

// Auto risk table: git subcommands. Every parser fails toward unsure
// (cannot place a word) or ask (a risky form is present); none of them can
// turn an unknown form into an allow.

var (
	gitSafeGlobals = map[string]bool{"--no-pager": true, "-P": true, "--paginate": true, "-p": true,
		"--no-optional-locks": true, "--no-replace-objects": true, "--literal-pathspecs": true,
		"--glob-pathspecs": true, "--noglob-pathspecs": true, "--icase-pathspecs": true, "--no-advice": true}
	gitValueGlobals = map[string]bool{"-C": true, "-c": true, "--git-dir": true, "--work-tree": true,
		"--namespace": true, "--config-env": true, "--super-prefix": true}
	gitReadSubs = map[string]bool{"status": true, "log": true, "diff": true, "show": true, "blame": true,
		"ls-files": true, "rev-parse": true, "describe": true, "shortlog": true}
	gitConfigReads = []string{"--get", "--get-all", "--get-regexp", "--get-urlmatch", "-l", "--list", "get", "list"}
)

func gitVerdict(ws []Word, conf pathFunc) autoVerdict {
	safe := true // only the value-less, behaviour-neutral globals came first
	i := 1
	for ; i < len(ws); i++ {
		t := ws[i].Text
		if !ws[i].Literal {
			return autoUnsure
		}
		if !strings.HasPrefix(t, "-") {
			break
		}
		switch {
		case gitSafeGlobals[t]:
		case gitValueGlobals[t]:
			safe = false
			i++
		default:
			safe = false // -C, -c, --git-dir=…, --exec-path=…: a different repo or config
		}
	}
	if i >= len(ws) {
		return autoUnsure
	}
	sub, from := ws[i].Text, i+1
	has := func(names ...string) bool {
		for _, w := range ws[from:] {
			if slices.Contains(names, w.Text) {
				return true
			}
		}
		return false
	}
	var v autoVerdict
	switch {
	case sub == "push" || sub == "clean" || sub == "rebase" || strings.HasPrefix(sub, "filter-"):
		return autoAsk
	case sub == "reset":
		if has("--hard") {
			return autoAsk
		}
		return autoUnsure
	case sub == "checkout":
		if has("--", ".", "-f", "--force") {
			return autoAsk
		}
		return autoUnsure
	case sub == "restore":
		if !hasOpt(ws, from, "S", "--staged") || hasOpt(ws, from, "W", "--worktree") {
			return autoAsk // the working tree: uncommitted edits are lost
		}
		v = mutatorVerdict(ws, from, conf, guardNone, true)
	case sub == "config":
		if has(gitConfigReads...) {
			return autoUnsure
		}
		return autoAsk // a write: core.pager, aliases, hooksPath run code later
	case sub == "stash":
		v = gitStash(ws, from, conf)
	case sub == "branch":
		v = gitBranch(ws, from)
	case sub == "tag":
		v = gitTag(ws, from)
	case sub == "remote":
		v = gitRemote(ws, from)
	case gitReadSubs[sub]:
		v = writeTargets(optValues(ws, from, "", []string{"--output"}), conf, false).and(readCheck(ws, from, conf))
	case sub == "add":
		v = mutatorVerdict(ws, from, conf, guardNone, true)
	case sub == "commit":
		v = gitCommit(ws, from, conf)
	default:
		return autoUnsure
	}
	if v == autoAllow && (!safe || conf(".", false) == pathOut) {
		// Another repo or config (-C, -c, …), or a cwd a cd made unknown:
		// that directory's .git config may run code.
		v = autoUnsure
	}
	return v
}

func gitStash(ws []Word, from int, conf pathFunc) autoVerdict {
	if from >= len(ws) {
		return autoAllow // `git stash` = push: saves the working tree into the repo
	}
	switch t := ws[from].Text; {
	case t == "list" || t == "show":
		return readCheck(ws, from+1, conf)
	case t == "drop" || t == "clear":
		return autoAsk
	case t == "push" || t == "save" || t == "apply" || t == "pop":
		return mutatorVerdict(ws, from+1, conf, guardNone, true)
	case strings.HasPrefix(t, "-"):
		return mutatorVerdict(ws, from, conf, guardNone, true) // push with options
	}
	return autoUnsure
}

var (
	gitBranchAsk  = []string{"--delete", "--move", "--copy", "--force", "--unset-upstream", "--edit-description", "--set-upstream-to", "--track", "--no-track"}
	gitListImply  = []string{"--list", "--contains", "--no-contains", "--merged", "--no-merged", "--points-at"}
	gitBranchRead = []string{"--all", "--remotes", "--verbose", "--quiet", "--show-current", "--sort", "--format",
		"--column", "--no-column", "--color", "--no-color", "--abbrev", "--no-abbrev", "--ignore-case", "--omit-empty"}
	gitTagRead = []string{"--sort", "--format", "--column", "--no-column", "--color", "--no-color", "--ignore-case", "--omit-empty"}
)

// longIn reports whether a long option word selects one of names.
func longIn(t string, names []string) bool {
	name, _, _ := strings.Cut(t, "=")
	return slices.ContainsFunc(names, func(full string) bool { return longIs(name, full) })
}

// gitBranch: listing is read-only; delete/move/copy/force/upstream forms ask;
// creating a branch or any option outside the known read set is unsure.
func gitBranch(ws []Word, from int) autoVerdict {
	opts, args := splitArgs(ws, from)
	list := false
	for _, o := range opts {
		t := o.Text
		long := strings.HasPrefix(t, "--")
		switch {
		case long && longIn(t, gitBranchAsk), !long && strings.ContainsAny(t[1:], "dDmMcCfut"):
			return autoAsk
		case long && longIn(t, gitListImply), !long && strings.Contains(t, "l"):
			list = true
		case long && longIn(t, gitBranchRead):
		case !long && strings.Trim(t[1:], "arvqi") == "":
		default:
			return autoUnsure
		}
	}
	if len(args) > 0 && !list {
		return autoUnsure // creates a branch (or a value we cannot place)
	}
	return autoAllow
}

func gitTag(ws []Word, from int) autoVerdict {
	opts, args := splitArgs(ws, from)
	list, create := false, false
	for _, o := range opts {
		t := o.Text
		long := strings.HasPrefix(t, "--")
		switch {
		case long && longIn(t, []string{"--delete", "--force"}), !long && strings.ContainsAny(t[1:], "df"):
			return autoAsk
		case long && longIn(t, gitListImply), !long && strings.Contains(t, "l"):
			list = true
		case long && longIn(t, gitTagRead):
		case !long && strings.Trim(t[1:], "ni0123456789") == "":
		default:
			create = true // -a -s -u -m -F -e and anything unknown
		}
	}
	if create || (len(args) > 0 && !list) {
		return autoUnsure
	}
	return autoAllow
}

func gitRemote(ws []Word, from int) autoVerdict {
	_, args := splitArgs(ws, from)
	if len(args) == 0 {
		return autoAllow
	}
	switch args[0].Text {
	case "show", "get-url":
		return autoAllow
	case "add", "remove", "rm", "rename", "set-url", "set-head", "set-branches":
		return autoAsk // where pushes and fetches go
	}
	return autoUnsure
}

// gitCommit drops message-like option values (a commit message is not a
// path) and read-checks the file-valued ones, then judges the pathspecs.
func gitCommit(ws []Word, from int, conf pathFunc) autoVerdict {
	msgLong := map[string]bool{"--message": true, "--reuse-message": true, "--reedit-message": true,
		"--author": true, "--date": true, "--fixup": true, "--squash": true, "--cleanup": true, "--trailer": true}
	fileLong := map[string]bool{"--file": true, "--template": true}
	kept := []Word{ws[0]}
	v := autoAllow
	readVal := func(w Word) {
		if !w.Literal || !readOK(conf(w.Text, false)) {
			v = v.and(autoUnsure)
		}
	}
	for i := from; i < len(ws); i++ {
		w := ws[i]
		t := w.Text
		if !w.Literal || !strings.HasPrefix(t, "-") || t == "-" {
			kept = append(kept, w)
			continue
		}
		if t == "--" {
			kept = append(kept, ws[i:]...)
			break
		}
		if strings.HasPrefix(t, "--") {
			name, val, eq := strings.Cut(t, "=")
			switch {
			case msgLong[name] && eq:
			case msgLong[name]:
				i++
			case fileLong[name] && eq:
				readVal(Word{Text: val, Literal: true})
			case fileLong[name]:
				if i+1 < len(ws) {
					i++
					readVal(ws[i])
				}
			default:
				kept = append(kept, w)
			}
			continue
		}
		k := strings.IndexAny(t[1:], "mFCct")
		if k < 0 {
			kept = append(kept, w)
			continue
		}
		k++ // index in t
		file := t[k] == 'F' || t[k] == 't'
		switch {
		case k+1 < len(t) && file:
			readVal(Word{Text: t[k+1:], Literal: true})
		case k+1 < len(t):
		case i+1 < len(ws):
			i++
			if file {
				readVal(ws[i])
			}
		}
	}
	return v.and(mutatorVerdict(kept, 1, conf, guardNone, true))
}
