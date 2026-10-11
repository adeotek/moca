package permissions

import "strings"

// Auto risk table: curl and wget (ruling 1: a plain download is a network
// read; sending data, unconfined outputs and other schemes are not). Every
// parser fails toward unsure or ask; none can turn an unknown form into an
// allow.

var (
	curlWriteLong = map[string]bool{"--output": true, "--dump-header": true, "--cookie-jar": true, "--trace": true,
		"--trace-ascii": true, "--libcurl": true, "--stderr": true, "--etag-save": true, "--hsts": true, "--alt-svc": true}
	curlReadLong = map[string]bool{"--cookie": true, "--cacert": true, "--cert": true, "--key": true, "--capath": true,
		"--netrc-file": true, "--proxy-cacert": true, "--proxy-cert": true, "--proxy-key": true,
		"--etag-compare": true, "--crlfile": true}
	curlValueLong = map[string]bool{"--header": true, "--user-agent": true, "--user": true, "--referer": true,
		"--proxy": true, "--max-time": true, "--connect-timeout": true, "--retry": true, "--retry-delay": true,
		"--retry-max-time": true, "--resolve": true, "--connect-to": true, "--interface": true, "--limit-rate": true,
		"--max-filesize": true, "--range": true, "--oauth2-bearer": true, "--proxy-user": true,
		"--continue-at": true, "--time-cond": true, "--max-redirs": true, "--speed-limit": true, "--speed-time": true,
		"--dns-servers": true, "--doh-url": true, "--noproxy": true, "--preproxy": true, "--proxy-header": true}
	curlAskPrefixes = []string{"--data", "--json", "--form", "--upload-file", "--config", "--unix-socket",
		"--abstract-unix-socket", "--variable", "--expand-", "--mail-", "--next", "--request"}
	curlBoolLong = map[string]bool{"--fail": true, "--fail-with-body": true, "--silent": true, "--show-error": true,
		"--location": true, "--compressed": true, "--insecure": true, "--verbose": true, "--include": true,
		"--head": true, "--globoff": true, "--http1.0": true, "--http1.1": true, "--http2": true, "--http3": true,
		"--no-progress-meter": true, "--progress-bar": true, "--create-dirs": true, "--retry-all-errors": true,
		"--retry-connrefused": true, "--ipv4": true, "--ipv6": true, "--raw": true, "--no-buffer": true,
		"--path-as-is": true, "--remote-header-name": true, "--list-only": true, "--digest": true, "--basic": true,
		"--anyauth": true, "--negotiate": true, "--ntlm": true, "--location-trusted": true, "--tlsv1.2": true,
		"--tlsv1.3": true, "--ssl-reqd": true, "--no-keepalive": true, "--tcp-nodelay": true, "--netrc": true,
		"--netrc-optional": true, "--styled-output": true, "--no-styled-output": true, "--parallel": true,
		"--fail-early": true, "--http2-prior-knowledge": true, "--compressed-ssh": true, "--no-sessionid": true}
)

// curlLongAsk reports whether a long word names (or abbreviates) a
// sending, config-reading or socket option.
func curlLongAsk(t string) bool {
	for _, p := range curlAskPrefixes {
		if strings.HasPrefix(t, p) || (len(t) >= 4 && strings.HasPrefix(p, t)) {
			return true
		}
	}
	return false
}

// netURL allows http(s) URLs and bare host[:port] forms; any other scheme
// (file:, ftp:, dict:, gopher:, …) is unsure.
func netURL(w Word) autoVerdict {
	if !w.Literal {
		return autoUnsure
	}
	t := strings.ToLower(w.Text)
	if i := strings.Index(t, "://"); i >= 0 {
		if s := t[:i]; s == "http" || s == "https" {
			return autoAllow
		}
		return autoUnsure
	}
	if before, after, ok := strings.Cut(t, ":"); ok && !strings.Contains(before, "/") &&
		(after == "" || after[0] < '0' || after[0] > '9') {
		return autoUnsure // scheme:… without //, e.g. file:/etc/passwd
	}
	return autoAllow
}

func getVerb(w Word) bool {
	u := strings.ToUpper(w.Text)
	return w.Literal && (u == "GET" || u == "HEAD")
}

// fileRead read-checks a file-valued option (an @file value is the file).
func fileRead(w Word, conf pathFunc, cookie bool) autoVerdict {
	if !w.Literal {
		return autoUnsure
	}
	t := w.Text
	if cookie && strings.Contains(t, "=") {
		return autoAllow // a cookie string, not a file
	}
	t = strings.TrimPrefix(t, "@")
	if t == "-" || readOK(conf(t, false)) {
		return autoAllow
	}
	return autoUnsure
}

// atRead read-checks a value only when it names a file with @.
func atRead(w Word, conf pathFunc) autoVerdict {
	if w.Literal && strings.HasPrefix(w.Text, "@") {
		return fileRead(w, conf, false)
	}
	return autoAllow
}

// writeOut judges a curl -w/--write-out format: %output{FILE} (or
// %output{>>FILE}) writes a file, so it — or a value we cannot see — asks.
// @file reads the format from a file whose content (possibly %output{…},
// written earlier in the same chain) cannot be seen: unsure.
func writeOut(w Word) autoVerdict {
	switch {
	case !w.Literal || strings.Contains(strings.ToLower(w.Text), "%output{"):
		return autoAsk
	case strings.HasPrefix(w.Text, "@"):
		return autoUnsure
	}
	return autoAllow
}

type cursor struct {
	ws []Word
	i  int
}

func (c *cursor) next() Word {
	if c.i+1 < len(c.ws) {
		c.i++
		return c.ws[c.i]
	}
	return Word{} // missing value: non-literal → never allowed
}

func curlVerdict(ws []Word, conf pathFunc) autoVerdict {
	v := autoAllow
	remoteName, outDir := false, false
	c := &cursor{ws: ws}
	for c.i = 1; c.i < len(ws); c.i++ {
		w := ws[c.i]
		t := w.Text
		switch {
		case !w.Literal:
			v = v.and(autoUnsure)
		case t == "--":
			for _, u := range ws[c.i+1:] {
				v = v.and(netURL(u))
			}
			c.i = len(ws)
		case t == "--request":
			if !getVerb(c.next()) {
				return autoAsk
			}
		case strings.HasPrefix(t, "--write-out"):
			_, val, eq := strings.Cut(t, "=")
			if !eq {
				v = v.and(writeOut(c.next()))
			} else {
				v = v.and(writeOut(Word{Text: val, Literal: true}))
			}
		case strings.HasPrefix(t, "--"):
			if curlLongAsk(t) {
				return autoAsk
			}
			switch {
			case strings.Contains(t, "="):
				v = v.and(autoUnsure) // curl has no --opt=value form
			case curlWriteLong[t]:
				v = v.and(writeTargets([]Word{c.next()}, conf, false))
			case t == "--output-dir":
				outDir = true
				v = v.and(writeTargets([]Word{c.next()}, conf, true))
			case t == "--remote-name" || t == "--remote-name-all":
				remoteName = true
			case t == "--url":
				v = v.and(netURL(c.next()))
			case curlReadLong[t]:
				v = v.and(fileRead(c.next(), conf, t == "--cookie"))
			case curlValueLong[t]:
				v = v.and(atRead(c.next(), conf))
			case !curlBoolLong[t]:
				v = v.and(autoUnsure) // unknown, or an abbreviation we cannot place
			}
		case strings.HasPrefix(t, "-") && len(t) > 1:
			for k := 1; k < len(t); k++ {
				ch := t[k]
				val := func() Word {
					if k+1 < len(t) {
						return Word{Text: t[k+1:], Literal: true}
					}
					return c.next()
				}
				switch {
				case strings.IndexByte("dFTK", ch) >= 0:
					return autoAsk
				case ch == 'X':
					if !getVerb(val()) {
						return autoAsk
					}
				case ch == 'o' || ch == 'D' || ch == 'c':
					v = v.and(writeTargets([]Word{val()}, conf, false))
				case ch == 'O':
					remoteName = true
					continue
				case ch == 'b' || ch == 'E':
					v = v.and(fileRead(val(), conf, ch == 'b'))
				case ch == 'w':
					v = v.and(writeOut(val()))
				case strings.IndexByte("HueAxrmCyYztUQP", ch) >= 0:
					v = v.and(atRead(val(), conf))
				default:
					continue // a boolean flag
				}
				break // the value consumed the rest of the cluster
			}
		default:
			v = v.and(netURL(w))
		}
	}
	if remoteName && !outDir && conf(".", true) == pathOut {
		return autoAsk // writes into a cwd that a cd made unknown
	}
	return v
}

var (
	wgetAskLong = map[string]bool{"--post-data": true, "--post-file": true, "--body-data": true, "--body-file": true,
		"--execute": true, "--config": true, "--use-askpass": true}
	// wgetSafeBool / wgetSafeValue: the only other long options a plain
	// download may use. They cannot write a file (beyond the checked -O and
	// -P), load code or read credentials. Anything else is unsure.
	wgetSafeBool  = map[string]bool{"--quiet": true, "--no-verbose": true, "--server-response": true, "--spider": true, "--continue": true}
	wgetSafeValue = map[string]bool{"--timeout": true, "--tries": true, "--user-agent": true, "--header": true}
)

// wgetLong expands an abbreviated long option (getopt_long) to the known
// name it may select, preferring the ask set.
func wgetLong(name string) string {
	for _, set := range []map[string]bool{wgetAskLong, {"--method": true}, {"--output-document": true},
		{"--directory-prefix": true}, wgetSafeBool, wgetSafeValue} {
		for full := range set {
			if longIs(name, full) {
				return full
			}
		}
	}
	return name
}

// wgetVerdict: a plain download (http/https, the safe options, -O/-P inside
// the confinement) is allowed; sending data, -e/--execute and --config ask;
// every other option is unsure.
func wgetVerdict(ws []Word, conf pathFunc) autoVerdict {
	v := autoAllow
	doc, prefix := false, false
	c := &cursor{ws: ws}
	for c.i = 1; c.i < len(ws); c.i++ {
		w := ws[c.i]
		t := w.Text
		switch {
		case !w.Literal:
			v = v.and(autoUnsure)
		case t == "--":
			for _, u := range ws[c.i+1:] {
				v = v.and(netURL(u))
			}
			c.i = len(ws)
		case strings.HasPrefix(t, "--"):
			name, val, eq := strings.Cut(t, "=")
			name = wgetLong(name)
			value := func() Word {
				if eq {
					return Word{Text: val, Literal: true}
				}
				return c.next()
			}
			switch {
			case wgetAskLong[name]:
				return autoAsk
			case name == "--method":
				if !getVerb(value()) {
					return autoAsk
				}
			case name == "--output-document":
				doc = true
				v = v.and(writeTargets([]Word{value()}, conf, false))
			case name == "--directory-prefix":
				prefix = true
				v = v.and(writeTargets([]Word{value()}, conf, true))
			case wgetSafeValue[name]:
				value()
			case wgetSafeBool[name] && !eq:
			default:
				// Unknown: unsure. Keep scanning so a later ask form still
				// asks (a value word read as an option only over-matches).
				v = v.and(autoUnsure)
			}
		case t == "-nv" || t == "-nc":
		case strings.HasPrefix(t, "-") && len(t) > 1:
			for k := 1; k < len(t); k++ {
				ch := t[k]
				val := func() Word {
					if k+1 < len(t) {
						return Word{Text: t[k+1:], Literal: true}
					}
					return c.next()
				}
				switch {
				case ch == 'e':
					return autoAsk
				case ch == 'O':
					doc = true
					v = v.and(writeTargets([]Word{val()}, conf, false))
				case ch == 'P':
					prefix = true
					v = v.and(writeTargets([]Word{val()}, conf, true))
				case ch == 'T' || ch == 't' || ch == 'U':
					val()
				case ch == 'q' || ch == 'S' || ch == 'c' || ch == 'v':
					continue
				default:
					v = v.and(autoUnsure) // -o/-a logs, -i, -r, -x, … and unknown letters
				}
				break
			}
		default:
			v = v.and(netURL(w))
		}
	}
	if !doc && !prefix && conf(".", true) == pathOut {
		return autoAsk // downloads into a cwd that a cd made unknown
	}
	return v
}
