package compact

// FindCut chooses the first kept entry. es is the live window (entries after
// the previous compaction's first kept entry, inclusive) and the cut may only
// land on a user or assistant message: tool calls and their results move
// together. Returns the cut index, the turnStart index (>= 0 only when the
// cut splits a turn — es[cut] is an assistant message whose turn's opening
// user message sits at turnStart, possibly 0), and ok=false when nothing can
// be summarized (no valid cut point, or the whole window already fits within
// keepRecent: everything is recent, nothing is older).
func FindCut(es []Entry, keepRecent int) (cut, turnStart int, ok bool) {
	suffix := make([]int, len(es)+1)
	for i := len(es) - 1; i >= 0; i-- {
		suffix[i] = suffix[i+1] + EntryTokens(es[i])
	}
	if suffix[0] <= keepRecent {
		return 0, -1, false
	}
	cut = -1
	for i := 1; i < len(es); i++ {
		if es[i].Kind != KindUser && es[i].Kind != KindAssistant {
			continue
		}
		if suffix[i] <= keepRecent {
			cut = i
			break
		}
		cut = i // remember the latest valid cut in case none fits
	}
	if cut < 0 {
		return 0, -1, false
	}
	turnStart = -1
	if es[cut].Kind == KindAssistant {
		for j := cut - 1; j >= 0; j-- {
			if es[j].Kind == KindUser {
				turnStart = j
				break
			}
		}
	}
	return cut, turnStart, true
}
