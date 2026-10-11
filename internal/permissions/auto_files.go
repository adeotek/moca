package permissions

import (
	"slices"
	"strings"
)

// Auto risk table: sed programs and archive extraction. Every parser fails
// toward unsure (cannot place a word) or ask (a risky form is present); none
// of them can turn an unknown form into an allow.

func sedVerdict(ws []Word, conf pathFunc) autoVerdict {
	inPlace, fromOpt := false, false
	var scripts []string
	var pos []Word // positionals in order; getopt permutes them after the options
	dd := false
	for i := 1; i < len(ws); i++ {
		w := ws[i]
		t := w.Text
		if !w.Literal {
			return autoUnsure // cannot tell a script from a file
		}
		switch {
		case dd || t == "-" || !strings.HasPrefix(t, "-"):
			pos = append(pos, w)
		case t == "--":
			dd = true
		case strings.HasPrefix(t, "--"):
			name, val, eq := strings.Cut(t, "=")
			switch name {
			case "--in-place":
				inPlace = true
				if strings.Contains(val, "/") {
					return autoAsk // a backup written elsewhere (GNU `*` with a directory)
				}
			case "--expression":
				fromOpt = true
				if !eq {
					if i+1 >= len(ws) || !ws[i+1].Literal {
						return autoUnsure
					}
					i++
					val = ws[i].Text
				}
				scripts = append(scripts, val)
			case "--line-length":
				if !eq {
					i++
				}
			case "--quiet", "--silent", "--regexp-extended", "--separate", "--null-data", "--unbuffered",
				"--posix", "--debug", "--sandbox", "--follow-symlinks", "--binary":
			default:
				return autoUnsure // --file and anything unknown
			}
		default:
			for k := 1; k < len(t); k++ {
				switch ch := t[k]; ch {
				case 'n', 'E', 'r', 's', 'z', 'u', 'b':
					continue
				case 'i':
					inPlace = true
					if strings.Contains(t[k+1:], "/") {
						return autoAsk
					}
				case 'e':
					fromOpt = true
					s := t[k+1:]
					if s == "" {
						if i+1 >= len(ws) || !ws[i+1].Literal {
							return autoUnsure
						}
						i++
						s = ws[i].Text
					}
					scripts = append(scripts, s)
				case 'l':
					if k+1 == len(t) {
						i++
					}
				default:
					return autoUnsure // -f (script file) and unknown flags
				}
				break
			}
		}
	}
	files := pos
	if !fromOpt { // without -e/--expression the first positional is the script
		if len(pos) == 0 {
			return autoUnsure
		}
		scripts, files = []string{pos[0].Text}, pos[1:]
	}
	v := autoAllow
	for _, s := range scripts {
		v = v.and(sedScript(s))
	}
	if v == autoAsk {
		return v
	}
	if inPlace {
		return v.and(mutatorArgs(files, conf, guardNone, false))
	}
	for _, f := range files {
		if f.Text != "-" && !readOK(conf(f.Text, false)) {
			return autoUnsure
		}
	}
	return v
}

// sedScript scans a sed program: e/w/W commands and the s///e and s///w
// flags execute or write (ask); r/R read a file and anything the scanner
// cannot follow (including a bracket expression in a regex, where GNU and
// BSD sed split delimiters differently) is unsure.
func sedScript(s string) autoVerdict {
	n := len(s)
	// skip returns the index after the closing delimiter d starting at i.
	skip := func(i int, d byte, regex bool) (int, bool) {
		for j := i; j < n; j++ {
			switch {
			case s[j] == '\\':
				j++
			case s[j] == d:
				return j + 1, true
			case regex && s[j] == '[':
				return 0, false
			case s[j] == '\n':
				return 0, false
			}
		}
		return 0, false
	}
	toEOL := func(i int) int {
		for i < n && s[i] != '\n' {
			if s[i] == '\\' {
				i++
			}
			i++
		}
		return i
	}
	i := 0
	for i < n {
		c := s[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == ';' || c == '{' || c == '}' || c == '!' ||
			(c >= '0' && c <= '9') || c == '$' || c == ',' || c == '~' || c == '+':
			i++
			continue
		case c == '/' || c == '\\':
			d := byte('/')
			if c == '\\' {
				if i+1 >= n {
					return autoUnsure
				}
				i++
				d = s[i]
			}
			j, ok := skip(i+1, d, true)
			if !ok {
				return autoUnsure
			}
			i = j
			for i < n && (s[i] == 'I' || s[i] == 'M') {
				i++
			}
			continue
		}
		i++
		switch c {
		case 's', 'y':
			if i >= n || s[i] == '\n' || s[i] == '\\' {
				return autoUnsure
			}
			d := s[i]
			j, ok := skip(i+1, d, c == 's')
			if !ok {
				return autoUnsure
			}
			if j, ok = skip(j, d, false); !ok {
				return autoUnsure
			}
			for ; j < n && s[j] != ';' && s[j] != '\n' && s[j] != '}'; j++ {
				switch f := s[j]; {
				case f == 'e' || f == 'w' || f == 'W':
					return autoAsk
				case c == 's' && (f == 'g' || f == 'p' || f == 'i' || f == 'I' || f == 'm' || f == 'M' ||
					(f >= '0' && f <= '9') || f == ' ' || f == '\t'):
				case f == ' ' || f == '\t':
				default:
					return autoUnsure
				}
			}
			i = j
		case 'e', 'w', 'W':
			return autoAsk
		case 'a', 'i', 'c', '#':
			i = toEOL(i)
		case ':', 'b', 't', 'T':
			for i < n && s[i] != ';' && s[i] != '\n' {
				i++
			}
		case 'q', 'Q', 'l', 'L':
			for i < n && (s[i] == ' ' || (s[i] >= '0' && s[i] <= '9')) {
				i++
			}
		case 'p', 'd', 'D', 'n', 'N', 'g', 'G', 'h', 'H', 'x', 'P', '=', 'z', 'F':
		default:
			return autoUnsure // r/R (read a file) and anything unknown
		}
	}
	return autoAllow
}

// Archives.

func isLetters(t string) bool {
	if t == "" {
		return false
	}
	for _, r := range t {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') {
			return false
		}
	}
	return true
}

var tarAskLong = []string{"--absolute-names", "--to-command", "--use-compress-program", "--checkpoint-action",
	"--info-script", "--new-volume-script", "--rsh-command", "--rmt-command"}

// tarVerdict: only extraction is a confined mutator; -P (absolute names),
// program options and remote archives (host:file) ask; create/list are
// unsure.
func tarVerdict(ws []Word, conf pathFunc) autoVerdict {
	extract := false
	from := 1
	if len(ws) > 1 && ws[1].Literal && isLetters(ws[1].Text) {
		cl := ws[1].Text // old-style bundled options: `tar xzf a.tgz`
		if strings.ContainsAny(cl, "PIF") {
			return autoAsk
		}
		extract = strings.Contains(cl, "x")
		from = 2
	}
	opts, args := splitArgs(ws, from)
	for _, o := range opts {
		t := o.Text
		if strings.HasPrefix(t, "--") {
			name, _, _ := strings.Cut(t, "=")
			switch {
			case slices.ContainsFunc(tarAskLong, func(full string) bool { return longIs(name, full) }):
				return autoAsk
			case longIs(name, "--transform") || longIs(name, "--xform"):
				return autoUnsure
			case name == "--extract" || name == "--get":
				extract = true
			}
			continue
		}
		if strings.ContainsAny(t[1:], "PIF") {
			return autoAsk
		}
		if strings.Contains(t, "x") {
			extract = true
		}
	}
	if !extract {
		return autoUnsure
	}
	for _, w := range append(opts, args...) {
		if w.Literal && strings.Contains(w.Text, ":") {
			return autoAsk // host:archive goes over rsh
		}
	}
	if conf(".", true) == pathOut {
		return autoAsk // extracts into a cwd that a cd made unknown
	}
	return archiveVerdict(mutatorVerdict(ws, from, conf, guardNone, false))
}

// archiveVerdict: an unconfined target asks; otherwise extraction is
// unsure, never allowed — archive members can plant .git, hooks or
// symlinks the table cannot see.
func archiveVerdict(v autoVerdict) autoVerdict {
	if v == autoAsk {
		return autoAsk
	}
	return autoUnsure
}

func unzipVerdict(ws []Word, conf pathFunc) autoVerdict {
	opts, _ := splitArgs(ws, 1)
	for _, o := range opts {
		if strings.Contains(o.Text, ":") {
			return autoAsk // -: keeps ../ components
		}
	}
	if conf(".", true) == pathOut {
		return autoAsk
	}
	return archiveVerdict(mutatorVerdict(ws, 1, conf, guardNone, false))
}
