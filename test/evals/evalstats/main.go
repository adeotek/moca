// Command evalstats turns moca session transcripts into eval metrics.
//
//	evalstats -session s.jsonl [-scenario name -run n -pass -exit code]   # one JSON line
//	evalstats -summary results.jsonl                                      # markdown table
//
// run.sh appends one line per scenario run to a results file; -summary
// aggregates it (pass rate, tokens, steps, cache efficiency — mean ± sd) for
// BASELINE.md.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/adeotek/moca/internal/compact"
	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/session"
)

// CacheBreak is one request whose cache read fell below half of what was
// reusable: the prefix the previous request sent was not served from cache.
// After names the entry types appended since the previous request, which is
// the cheapest clue to the cause (elision and compaction rewrite history;
// tool_use/tool_result only append).
type CacheBreak struct {
	Step     int    `json:"step"`
	Reusable int    `json:"reusable"`
	Read     int    `json:"read"`
	After    string `json:"after"`
}

// Metrics is one scenario run.
type Metrics struct {
	Scenario string `json:"scenario,omitempty"`
	Label    string `json:"label,omitempty"` // e.g. "baseline" / "after-B"
	Run      int    `json:"run,omitempty"`
	Pass     bool   `json:"pass"`
	Exit     int    `json:"exit"`
	// Steps counts assistant turns (each one is a model request).
	Steps int            `json:"steps"`
	Calls map[string]int `json:"calls"`
	// ToolErrors counts error results; RepeatedFailures the error results
	// identical (tool, input, output) to an earlier error in the session.
	ToolErrors       int     `json:"toolErrors"`
	RepeatedFailures int     `json:"repeatedFailures"`
	Input            int     `json:"input"`
	CacheRead        int     `json:"cacheRead"`
	CacheWrite       int     `json:"cacheWrite"`
	Output           int     `json:"output"`
	Cost             float64 `json:"cost"`
	Compactions      int     `json:"compactions"`
	Seconds          float64 `json:"seconds"`

	// Cache attribution (task A1.2). FirstPrompt/FirstCacheRead are the first
	// request's prompt tokens and cache read (a cold start every session pays).
	// CacheEff is Σ cache read over Σ reusable prefix, for requests 2..n.
	FirstPrompt    int          `json:"firstPrompt,omitempty"`
	FirstCacheRead int          `json:"firstCacheRead,omitempty"`
	CacheEff       float64      `json:"cacheEff,omitempty"`
	CacheBreaks    []CacheBreak `json:"cacheBreaks,omitempty"`
	// Residency is, per tool, Σ tokens(result) × the number of later requests
	// whose live window still held that result (elision stubbing included).
	Residency map[string]int `json:"residency,omitempty"`
	// RedundantReadTokens counts read results whose window was fully covered
	// by an earlier, still-live read of the same path with identical lines and
	// no write/edit or compaction in between.
	RedundantReadTokens int            `json:"redundantReadTokens,omitempty"`
	Nudges              map[string]int `json:"nudges,omitempty"`
	EditRefusals        int            `json:"editRefusals,omitempty"`
	Hints               int            `json:"hints,omitempty"`
	SystemPromptTokens  int            `json:"systemPromptTokens,omitempty"`
}

// Tokens is every billed token of the run (cache reads included: they are
// cheaper, not free).
func (m Metrics) Tokens() int { return m.Input + m.CacheRead + m.CacheWrite + m.Output }

type usageRec struct {
	prompt, cacheRead int
	after             string // entry types appended since the previous request
}

type liveResult struct {
	tool   string
	tokens int
	idx    int
}

type readArgs struct {
	path          string
	offset, limit int
}

type readWindow struct {
	from, to int
	lines    map[int]string
	idx      int
	id       string
}

type readPending struct{ path string }

var (
	readFooterRe = regexp.MustCompile(`\[lines (\d+)-(\d+) of (\d+)`)
	readLineRe   = regexp.MustCompile(`^(\d+)\|(.*)$`)
)

// parseReadWindow pulls the window bounds and the numbered lines out of a
// read result ("N|content" lines + the "[lines A-B of TOTAL]" footer).
func parseReadWindow(content string) (from, to int, lines map[int]string, ok bool) {
	lines = map[int]string{}
	for _, ln := range strings.Split(content, "\n") {
		if m := readLineRe.FindStringSubmatch(ln); m != nil {
			n, err := strconv.Atoi(m[1])
			if err != nil {
				continue
			}
			lines[n] = m[2]
		}
	}
	m := readFooterRe.FindStringSubmatch(content)
	if m == nil || len(lines) == 0 {
		return 0, 0, nil, false
	}
	from, err1 := strconv.Atoi(m[1])
	to, err2 := strconv.Atoi(m[2])
	if err1 != nil || err2 != nil || to < from {
		return 0, 0, nil, false
	}
	return from, to, lines, true
}

// Collect computes the metrics of one transcript.
func Collect(entries []session.Entry) Metrics {
	m := Metrics{Calls: map[string]int{}, Residency: map[string]int{}, Nudges: map[string]int{}}
	callNames := map[string]string{}
	pending := map[string]readArgs{}
	elided := map[string]bool{}
	index := map[string]int{}
	windows := map[string][]*readWindow{}
	editAt := map[string]int{}
	live := map[string]*liveResult{}
	seenFail := map[string]bool{}

	var usages []usageRec
	var gap []string
	kept, lastCompact := -1, -1

	for i, e := range entries {
		if e.ID != "" {
			index[e.ID] = i
		}
		if e.Usage != nil {
			m.Input += e.Usage.Input
			m.CacheRead += e.Usage.CacheRead
			m.CacheWrite += e.Usage.CacheWrite
			m.Output += e.Usage.Output
		}
		m.Cost += e.Cost

		// Request boundary: the assistant message answers the request that
		// was sent after every entry so far.
		assistantUsage := e.Type == session.TypeMessage && e.Message != nil &&
			e.Message.Role == llm.RoleAssistant && e.Usage != nil
		if assistantUsage {
			usages = append(usages, usageRec{
				prompt:    e.Usage.Input + e.Usage.CacheRead + e.Usage.CacheWrite,
				cacheRead: e.Usage.CacheRead,
				after:     strings.Join(gap, ","),
			})
			gap = nil
			for _, r := range live {
				m.Residency[r.tool] += r.tokens
			}
		} else if e.Type != session.TypeSession {
			gap = append(gap, e.Type)
		}

		switch e.Type {
		case session.TypeSession:
			if e.Session != nil {
				m.SystemPromptTokens = compact.Tokens(len(e.Session.SystemPrompt))
			}
		case session.TypeMessage:
			if e.Message != nil && e.Message.Role == llm.RoleAssistant {
				m.Steps++
			}
			if e.Nudge != "" {
				m.Nudges[e.Nudge]++
			}
		case session.TypeToolUse:
			if e.ToolUse == nil {
				continue
			}
			c := e.ToolUse.Call
			callNames[c.ID] = c.Name
			m.Calls[c.Name]++
			var in struct {
				Path   string `json:"path"`
				Offset int    `json:"offset"`
				Limit  int    `json:"limit"`
			}
			_ = json.Unmarshal(c.Input, &in)
			switch c.Name {
			case "read":
				pending[c.ID] = readArgs{path: in.Path, offset: in.Offset, limit: in.Limit}
			case "write", "edit":
				if in.Path != "" {
					editAt[in.Path] = i
				}
			}
		case session.TypeToolResult:
			if e.ToolResult == nil {
				continue
			}
			tr := e.ToolResult
			if tr.IsError {
				m.ToolErrors++
				key := callNames[tr.CallID] + "\x00" + string(toolInputOf(entries, tr.CallID)) + "\x00" + tr.Content
				if seenFail[key] {
					m.RepeatedFailures++
				}
				seenFail[key] = true
			}
			tool := callNames[tr.CallID]
			if tool == "" {
				tool = "?"
			}
			tokens := compact.Tokens(len(tr.Content))
			if elided[e.ID] {
				tokens = compact.Tokens(len(session.ElidedStub))
			}
			live[e.ID] = &liveResult{tool: tool, tokens: tokens, idx: i}
			if strings.HasPrefix(tr.Content, "refused: tests are failing") {
				m.EditRefusals++
			}
			if strings.Contains(tr.Content, "[hint: tests failed") || strings.Contains(tr.Content, "[hint: locate the cause") {
				m.Hints++
			}
			if tool == "read" && !tr.IsError && e.ID != "" {
				if ra, ok := pending[tr.CallID]; ok {
					if from, to, lines, ok := parseReadWindow(tr.Content); ok {
						path := ra.path
						if path == "" {
							path = "?"
						}
						if covered(windows[path], from, to, lines, elided, kept, lastCompact, editAt[path]) {
							m.RedundantReadTokens += tokens
						}
						windows[path] = append(windows[path], &readWindow{from: from, to: to, lines: lines, idx: i, id: e.ID})
					}
				}
			}
		case session.TypeElision:
			if e.Elision != nil {
				for _, id := range e.Elision.IDs {
					elided[id] = true
					if r, ok := live[id]; ok {
						r.tokens = compact.Tokens(len(session.ElidedStub))
					}
				}
			}
		case session.TypeCompaction:
			m.Compactions++
			lastCompact = i
			if e.Compaction != nil && e.Compaction.FirstKeptEntryID != "" {
				if k, ok := index[e.Compaction.FirstKeptEntryID]; ok {
					kept = k
					for id, r := range live {
						if r.idx < k {
							delete(live, id)
						}
					}
				}
			}
		}
	}
	if n := len(entries); n > 1 {
		m.Seconds = math.Round(entries[n-1].Time.Sub(entries[0].Time).Seconds()*10) / 10
	}
	if len(usages) > 0 {
		m.FirstPrompt, m.FirstCacheRead = usages[0].prompt, usages[0].cacheRead
		var num, den int
		for k := 1; k < len(usages); k++ {
			prev, cur := usages[k-1], usages[k]
			reusable := min(prev.prompt, cur.prompt)
			num += cur.cacheRead
			den += reusable
			if reusable > 0 && cur.cacheRead < reusable/2 {
				m.CacheBreaks = append(m.CacheBreaks, CacheBreak{
					Step: k + 1, Reusable: reusable, Read: cur.cacheRead, After: cur.after})
			}
		}
		if den > 0 {
			m.CacheEff = float64(num) / float64(den)
		}
	}
	return m
}

// toolInputOf returns the recorded input of a call (for the repeated-failure
// key); an unknown call yields nil.
func toolInputOf(entries []session.Entry, callID string) json.RawMessage {
	for _, e := range entries {
		if e.Type == session.TypeToolUse && e.ToolUse != nil && e.ToolUse.Call.ID == callID {
			return e.ToolUse.Call.Input
		}
	}
	return nil
}

// covered reports whether an earlier read window still in the live window
// fully contains [from,to] with identical lines, with no write/edit of the
// path and no compaction since that read.
func covered(prev []*readWindow, from, to int, lines map[int]string,
	elided map[string]bool, kept, lastCompact, editAt int) bool {
	for _, w := range prev {
		if elided[w.id] || w.idx < kept || w.idx < lastCompact {
			continue
		}
		if editAt > w.idx {
			continue // the file changed after that read
		}
		if w.from > from || to > w.to {
			continue // no full coverage
		}
		ok := true
		for n := from; n <= to; n++ {
			if w.lines[n] != lines[n] {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

func main() {
	sess := flag.String("session", "", "session jsonl to measure")
	summary := flag.String("summary", "", "results jsonl to aggregate")
	scenario := flag.String("scenario", "", "scenario name")
	label := flag.String("label", "", "run label")
	run := flag.Int("run", 0, "run number")
	pass := flag.Bool("pass", false, "the scenario checker passed")
	exit := flag.Int("exit", 0, "moca exit code")
	flag.Parse()
	switch {
	case *summary != "":
		f, err := os.Open(*summary)
		if err != nil {
			fatal(err)
		}
		defer f.Close()
		rows, err := readResults(f)
		if err != nil {
			fatal(err)
		}
		fmt.Print(Summarize(rows))
	case *sess != "":
		entries, err := session.ReadFile(*sess)
		if err != nil {
			fatal(err)
		}
		m := Collect(entries)
		m.Scenario, m.Label, m.Run, m.Pass, m.Exit = *scenario, *label, *run, *pass, *exit
		b, _ := json.Marshal(m)
		fmt.Println(string(b))
	default:
		flag.Usage()
		os.Exit(2)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "evalstats:", err)
	os.Exit(1)
}

func readResults(r io.Reader) ([]Metrics, error) {
	var out []Metrics
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var m Metrics
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			return nil, fmt.Errorf("bad results line: %w", err)
		}
		out = append(out, m)
	}
	return out, sc.Err()
}

// Summarize renders one row per (label, scenario) plus a total row per
// label: pass rate, tokens and steps as mean ± sd, cache efficiency, first-hit
// (median first-request cache read), cache breaks per run, redundant read
// tokens, cost, and tokens per solved task (all tokens of the group / passes).
func Summarize(rows []Metrics) string {
	type key struct{ label, scenario string }
	groups := map[key][]Metrics{}
	var keys []key
	add := func(k key, m Metrics) {
		if _, ok := groups[k]; !ok {
			keys = append(keys, k)
		}
		groups[k] = append(groups[k], m)
	}
	for _, m := range rows {
		add(key{m.Label, m.Scenario}, m)
		add(key{m.Label, "∑ all"}, m)
	}
	sort.SliceStable(keys, func(i, j int) bool {
		if keys[i].label != keys[j].label {
			return keys[i].label < keys[j].label
		}
		return keys[i].scenario < keys[j].scenario
	})
	var sb strings.Builder
	sb.WriteString("| label | scenario | runs | pass | tokens (mean ± sd) | steps (mean ± sd) | cache eff | first-hit | breaks/run | redundant read tok | cost $ | tokens/solved | repeated fails |\n")
	sb.WriteString("|---|---|---|---|---|---|---|---|---|---|---|---|---|\n")
	for _, k := range keys {
		g := groups[k]
		var toks, steps, firstHit []float64
		passes, rep, breaks, redundant := 0, 0, 0, 0
		cost, total, eff := 0.0, 0, 0.0
		for _, m := range g {
			toks = append(toks, float64(m.Tokens()))
			steps = append(steps, float64(m.Steps))
			firstHit = append(firstHit, float64(m.FirstCacheRead))
			if m.Pass {
				passes++
			}
			rep += m.RepeatedFailures
			breaks += len(m.CacheBreaks)
			redundant += m.RedundantReadTokens
			cost += m.Cost
			total += m.Tokens()
			eff += m.CacheEff
		}
		perSolved := "—"
		if passes > 0 {
			perSolved = fmt.Sprintf("%.0f", float64(total)/float64(passes))
		}
		tm, ts := meanSD(toks)
		sm, ss := meanSD(steps)
		n := float64(len(g))
		fmt.Fprintf(&sb, "| %s | %s | %d | %d/%d | %.0f ± %.0f | %.1f ± %.1f | %.2f | %.0f | %.1f | %.0f | %.4f | %s | %d |\n",
			k.label, k.scenario, len(g), passes, len(g), tm, ts, sm, ss,
			eff/n, median(firstHit), float64(breaks)/n, float64(redundant)/n, cost, perSolved, rep)
	}
	return sb.String()
}

func median(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	cp := append([]float64(nil), xs...)
	sort.Float64s(cp)
	n := len(cp)
	if n%2 == 1 {
		return cp[n/2]
	}
	return (cp[n/2-1] + cp[n/2]) / 2
}

func meanSD(xs []float64) (float64, float64) {
	if len(xs) == 0 {
		return 0, 0
	}
	var sum float64
	for _, x := range xs {
		sum += x
	}
	mean := sum / float64(len(xs))
	var v float64
	for _, x := range xs {
		v += (x - mean) * (x - mean)
	}
	return mean, math.Sqrt(v / float64(len(xs)))
}
