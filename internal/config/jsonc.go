package config

import "errors"

// Standardize turns JSONC into JSON that encoding/json accepts. Comments are
// recognized only outside string literals (escapes honoured) and replaced by
// spaces, keeping newlines, so the output has the same length and line
// structure as the input and decode-error offsets still point at the right
// line:col. A comma followed only by whitespace/comments before } or ] is
// blanked (trailing comma).
func Standardize(src []byte) ([]byte, error) {
	out := append([]byte(nil), src...)
	const (
		code = iota
		str
		lineComment
		blockComment
	)
	state := code
	pendingComma := -1
	for i := 0; i < len(out); i++ {
		c := out[i]
		switch state {
		case str:
			if c == '\\' {
				i++
			} else if c == '"' {
				state = code
			}
		case lineComment:
			if c == '\n' {
				state = code
			} else {
				out[i] = ' '
			}
		case blockComment:
			if c == '*' && i+1 < len(out) && out[i+1] == '/' {
				out[i], out[i+1] = ' ', ' '
				i++
				state = code
			} else if c != '\n' && c != '\r' {
				out[i] = ' '
			}
		case code:
			switch {
			case c == '/' && i+1 < len(out) && out[i+1] == '/':
				out[i], out[i+1] = ' ', ' '
				i++
				state = lineComment
			case c == '/' && i+1 < len(out) && out[i+1] == '*':
				out[i], out[i+1] = ' ', ' '
				i++
				state = blockComment
			case c == '"':
				pendingComma = -1
				state = str
			case c == ',':
				pendingComma = i
			case c == '}' || c == ']':
				if pendingComma >= 0 {
					out[pendingComma] = ' '
				}
				pendingComma = -1
			case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			default:
				pendingComma = -1
			}
		}
	}
	if state == blockComment {
		return nil, errors.New("unterminated /* comment")
	}
	return out, nil
}
