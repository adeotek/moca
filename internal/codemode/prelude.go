package codemode

import (
	_ "embed"
	"strconv"
	"strings"
)

//go:embed prelude.js
var preludeJS string

// renderPrelude substitutes the four values the guest needs. Both facts are
// rendered as literals: the guest must not be able to read or change them at
// runtime, and a boolean is the whole state (no strings to compare).
func renderPrelude(caps ExecCaps) string {
	src := preludeJS
	if !caps.NetworkPrelude {
		// The workspace-only variant has no web block AT ALL: the methods are
		// not defined and then refused, they are absent from the source. A
		// resumed network session keeps them and refuses at admission instead
		// ({true,false} in ExecCaps).
		src = cutWebBlock(src)
	}
	r := strings.NewReplacer(
		"__NET_SHAPE__", strconv.FormatBool(caps.NetworkPrelude),
		// Admission needs BOTH facts: a caller that passed the impossible
		// {NetworkPrelude:false, Network:true} pair still cannot grant web.
		"__NET_LIVE__", strconv.FormatBool(caps.Network && caps.NetworkPrelude),
		"__REQ_MAX__", strconv.Itoa(TheABI.Limits.RequestBytes),
		"__RES_MAX__", strconv.Itoa(TheABI.Limits.ResponseBytes),
	)
	return r.Replace(src)
}

// cutWebBlock removes everything between the two markers, comment lines
// included: the block's own closing brace is inside the cut range.
func cutWebBlock(src string) string {
	begin := strings.Index(src, "  // __WEB_BEGIN__")
	end := strings.Index(src, "  // __WEB_END__")
	if begin < 0 || end < begin {
		return src // a template without markers must not silently lose code
	}
	return src[:begin] + src[end+len("  // __WEB_END__"):]
}

// PreludeSource exposes the rendered prelude for the tests and the build
// report; the engine uses renderPrelude directly.
func PreludeSource(caps ExecCaps) string { return renderPrelude(caps) }
