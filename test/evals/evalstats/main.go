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
	"path/filepath"
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

// Metrics is one scenario run. Rows are versioned by Schema (codemode plan
// v13, Task 1): a row without one is schema 1 (a single-prompt `-p` run,
// identified by Run). Schema 2 is one two-turn trial (E2): the second process
// resumed the first with --continue, so Trial identifies the repetition and
// Turns records the boundary. `-compare` refuses schema-2 rows (its paired
// statistics assume one prompt per row) and `-compare-carry` requires them.
type Metrics struct {
	Schema   int    `json:"schema,omitempty"`
	Scenario string `json:"scenario,omitempty"`
	Label    string `json:"label,omitempty"` // e.g. "baseline" / "after-B"
	Run      int    `json:"run,omitempty"`   // schema 1: the repetition number
	// Trial/Turns/Arm are schema 2's identity. Arm names the config arm
	// (`cm-on`/`cm-off`; `on`/`off` are reserved by the corpus skills arm).
	Trial int    `json:"trial,omitempty"`
	Turns int    `json:"turns,omitempty"`
	Arm   string `json:"arm,omitempty"`
	// Session is the transcript's id8: the join key between a row and the
	// carry observations written for that trial.
	Session string `json:"session,omitempty"`
	// Carry is the pre-change instrument's figure: moca's token estimate for
	// the tool-result content in the first outbound request of the resuming
	// process (see internal/agent/carry_hook.go). CarryMeasured separates an
	// observed zero (a second turn that carried no tool output) from a
	// missing observation — a missing one fails the trial and no comparison
	// may silently read it as free.
	Carry         int  `json:"carry,omitempty"`
	CarryMeasured bool `json:"carryMeasured,omitempty"`
	// Work counts logical work requests — one per turn() boundary across both
	// processes — and is the changeset campaign's primary endpoint. Attempts
	// counts physical adapter attempts, a diagnostic; the two are never summed.
	Work        int   `json:"work,omitempty"`
	WorkMissing bool  `json:"workMissing,omitempty"`
	Attempts    int   `json:"attempts,omitempty"`
	MS          int64 `json:"ms,omitempty"`
	CMAttempted int   `json:"cmAttempted,omitempty"`
	CMOk        int   `json:"cmOk,omitempty"`
	Pass        bool  `json:"pass"`
	Exit        int   `json:"exit"`
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
	// ThinkingDropped sums the thinking blocks the provider reported dropping
	// for the session (anthropic input_transformations, prefix_binding_mismatch):
	// non-zero means a preserved-thinking route saw history that no longer
	// matched the prefix its thinking was produced against.
	ThinkingDropped int `json:"thinkingDropped,omitempty"`
	// SkillWrites counts write/edit calls targeting .moca/skills/ that
	// LANDED; SkillWriteRefused counts the calls the guard refused. Counting
	// calls alone sold a refusal as a write (v0.7.1's "31 skill-file writes"
	// was 4 landed and 27 refused). (the
	// self-managed skills feature, §12); SkillListed reports that this
	// session's stored prompt lists a moca-* skill — the skills.2 row is the
	// one that matters (session 1 wrote it, session 2 reads it).
	SkillWrites       int  `json:"skillWrites,omitempty"`
	SkillWriteRefused int  `json:"skillWriteRefused,omitempty"`
	SkillListed       bool `json:"skillListed,omitempty"`

	// Size (v0.8.1, task 1): what the agent WROTE, measured from the fixture
	// commit to the final tree by run.sh's size_of. These are measurements —
	// no checker reads them, so a run cannot fail for writing more code — and
	// they are additive, so an older results row decodes to zero.
	// Added/Removed are line counts over tracked files; Files counts files
	// with at least one changed line; NewFiles counts files the fixture never
	// had. `.moca/` is excluded at the source: it holds project resources
	// (an allow-always config, a self-managed skill, a saved command), not code.
	Added    int `json:"added,omitempty"`
	Removed  int `json:"removed,omitempty"`
	Files    int `json:"files,omitempty"`
	NewFiles int `json:"newFiles,omitempty"`
}

// Churn is every line the run added or removed.
func (m Metrics) Churn() int { return m.Added + m.Removed }

// ReusePct is the share of touched files that already existed — how much of
// the change landed in code that was already there. A run that touched nothing
// reads 0, never NaN.
func (m Metrics) ReusePct() float64 {
	if m.Files <= 0 {
		return 0
	}
	return 100 * float64(m.Files-m.NewFiles) / float64(m.Files)
}

// skillListedRe matches a moca-* skill line in the system prompt's listing.
var skillListedRe = regexp.MustCompile(`(?m)^- moca-[a-z0-9-]+:`)

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
	callInputs := map[string]json.RawMessage{} // the repeated-failure key's input, by call id
	pending := map[string]readArgs{}
	// skillWrite marks call ids whose write/edit targets .moca/skills/: the
	// landing (or refusal) is decided by the result, not by the call.
	skillWrite := map[string]bool{}
	// cmCall marks tool calls the codemode tool made: attempted on the call,
	// completed only when its result is not an error (schema 2's adoption
	// counters, independent of whether the scenario passed).
	cmCall := map[string]bool{}
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
				m.SkillListed = skillListedRe.MatchString(e.Session.SystemPrompt)
			}
		case session.TypeMessage:
			if e.Message != nil && e.Message.Role == llm.RoleAssistant {
				m.Steps++
			}
			if e.Nudge != "" {
				m.Nudges[e.Nudge]++
			}
			m.ThinkingDropped += e.ThinkingDropped
		case session.TypeToolUse:
			if e.ToolUse == nil {
				continue
			}
			c := e.ToolUse.Call
			callNames[c.ID] = c.Name
			callInputs[c.ID] = c.Input
			m.Calls[c.Name]++
			var in struct {
				Path   string `json:"path"`
				Offset int    `json:"offset"`
				Limit  int    `json:"limit"`
			}
			_ = json.Unmarshal(c.Input, &in)
			if c.Name == "codemode" {
				cmCall[c.ID] = true
				m.CMAttempted++
			}
			switch c.Name {
			case "read":
				pending[c.ID] = readArgs{path: in.Path, offset: in.Offset, limit: in.Limit}
			case "write", "edit":
				if in.Path != "" {
					editAt[in.Path] = i
				}
				if strings.Contains(filepath.ToSlash(in.Path), ".moca/skills/") {
					skillWrite[c.ID] = true
				}
			}
		case session.TypeToolResult:
			if e.ToolResult == nil {
				continue
			}
			tr := e.ToolResult
			if cmCall[tr.CallID] {
				if !tr.IsError {
					m.CMOk++
				}
				delete(cmCall, tr.CallID)
			}
			if skillWrite[tr.CallID] {
				// A refused skill write is not a write: the guard said no.
				if tr.IsError {
					m.SkillWriteRefused++
				} else {
					m.SkillWrites++
				}
				delete(skillWrite, tr.CallID)
			}
			if tr.IsError {
				m.ToolErrors++
				key := callNames[tr.CallID] + "\x00" + string(callInputs[tr.CallID]) + "\x00" + tr.Content
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
	compare := flag.String("compare", "", "two results jsonl paths, base.jsonl,cand.jsonl — bootstrap comparison")
	compareCarry := flag.String("compare-carry", "", "two schema-2 results files, base.jsonl,cand.jsonl — paired carry comparison")
	iters := flag.Int("iters", 2000, "bootstrap iterations for -compare / -compare-carry")
	seed := flag.Int64("seed", 1, "bootstrap RNG seed for -compare / -compare-carry (deterministic output)")
	scenario := flag.String("scenario", "", "scenario name")
	label := flag.String("label", "", "run label")
	run := flag.Int("run", 0, "run number (schema 1)")
	schema := flag.Int("schema", 0, "row schema: 2 = one two-turn trial")
	trial := flag.Int("trial", 0, "trial number (schema 2)")
	turns := flag.Int("turns", 0, "turns the trial ran (schema 2: 2)")
	arm := flag.String("arm", "", "config arm, cm-on|cm-off (schema 2)")
	sessionID := flag.String("sessionId", "", "the transcript's id8 (schema 2)")
	carry := flag.Int("carry", 0, "carried tool-result tokens of the resuming turn (schema 2)")
	carryMissing := flag.Bool("carryMissing", false, "the carry observation was absent: write the row with carryMeasured=false")
	ms := flag.Int64("ms", 0, "trial wall-clock milliseconds (schema 2)")
	carryOf := flag.String("carryOf", "", "print the carry of file.jsonl,turn (exit 1 when unobserved)")
	work := flag.Int("work", 0, "logical work requests across the trial's two processes (schema 2)")
	workMissing := flag.Bool("workMissing", false, "the work observation was absent (recorded, never read as zero)")
	attempts := flag.Int("attempts", 0, "physical adapter attempts observed at the last work turn (diagnostic)")
	workOf := flag.String("workOf", "", "print the logical work requests across file.jsonl,file.jsonl")
	attemptsOf := flag.String("attemptsOf", "", "print the summed physical attempts across file.jsonl,file.jsonl")
	trialCheck := flag.String("trialCheck", "", "verify a two-turn transcript")
	p1 := flag.String("p1", "", "turn 1 prompt file (-trialCheck)")
	p2 := flag.String("p2", "", "turn 2 prompt file (-trialCheck)")
	prechange := flag.Bool("prechange", false, "re-capture the pre-change goldens (prompt, wire schema, direct-tool output)")
	prechangeCheck := flag.Bool("prechangeCheck", false, "verify the committed pre-change goldens")
	pass := flag.Bool("pass", false, "the scenario checker passed")
	exit := flag.Int("exit", 0, "moca exit code")
	added := flag.Int("added", 0, "lines the run added (run.sh size_of, .moca/ excluded)")
	removed := flag.Int("removed", 0, "lines the run removed")
	files := flag.Int("files", 0, "files with at least one changed line")
	newFiles := flag.Int("newFiles", 0, "files the fixture did not have")
	flag.Parse()
	set := map[string]bool{}
	flag.Visit(func(f *flag.Flag) { set[f.Name] = true })
	switch {
	case *prechange:
		if err := WritePrechange(); err != nil {
			fatal(err)
		}
	case *prechangeCheck:
		if err := CheckPrechange(); err != nil {
			fatal(err)
		}
		fmt.Println("pre-change goldens: unchanged")
	case *carryOf != "":
		parts := strings.SplitN(*carryOf, ",", 2)
		if len(parts) != 2 {
			fatal(fmt.Errorf("-carryOf wants file.jsonl,turn (got %q)", *carryOf))
		}
		turn, err := strconv.Atoi(parts[1])
		if err != nil {
			fatal(fmt.Errorf("-carryOf turn: %w", err))
		}
		obs, err := ReadCarryObservations(parts[0])
		if err != nil {
			fatal(err)
		}
		c, ok := CarryOfTurn(obs, turn)
		if !ok {
			fatal(fmt.Errorf("no carry observation for turn %d in %s", turn, parts[0]))
		}
		fmt.Println(c)
	case *workOf != "":
		n, err := WorkOf(strings.Split(*workOf, ","))
		if err != nil {
			fatal(err)
		}
		fmt.Println(n)
	case *attemptsOf != "":
		n, ok, err := AttemptsOf(strings.Split(*attemptsOf, ","))
		if err != nil {
			fatal(err)
		}
		if !ok {
			fatal(fmt.Errorf("no attempt observations in %s", *attemptsOf))
		}
		fmt.Println(n)
	case *trialCheck != "":
		a, err := os.ReadFile(*p1)
		if err != nil {
			fatal(err)
		}
		b, err := os.ReadFile(*p2)
		if err != nil {
			fatal(err)
		}
		if err := CheckTrialSession(*trialCheck, string(a), string(b)); err != nil {
			fatal(err)
		}
	case *compare != "":
		parts := strings.SplitN(*compare, ",", 2)
		if len(parts) != 2 {
			fatal(fmt.Errorf("-compare wants base.jsonl,cand.jsonl (got %q)", *compare))
		}
		base, cand := readResultsFile(parts[0]), readResultsFile(parts[1])
		if err := refuseSchema2(base, cand); err != nil {
			fatal(err)
		}
		fmt.Print(RenderComparison(Compare(base, cand, *iters, *seed)))
	case *compareCarry != "":
		parts := strings.SplitN(*compareCarry, ",", 2)
		if len(parts) != 2 {
			fatal(fmt.Errorf("-compare-carry wants base.jsonl,cand.jsonl (got %q)", *compareCarry))
		}
		c, err := CompareCarry(readResultsFile(parts[0]), readResultsFile(parts[1]), *iters, *seed)
		if err != nil {
			fatal(err)
		}
		fmt.Print(RenderCarryComparison(c))
	case *summary != "":
		fmt.Print(Summarize(readResultsFile(*summary)))
	case *sess != "":
		if err := checkRowContract(*schema, set, *carryMissing, *workMissing, *trial, *turns, *arm, *sessionID); err != nil {
			fatal(err)
		}
		entries, err := session.ReadFile(*sess)
		if err != nil {
			fatal(err)
		}
		m := Collect(entries)
		m.Scenario, m.Label, m.Run, m.Pass, m.Exit = *scenario, *label, *run, *pass, *exit
		m.Added, m.Removed, m.Files, m.NewFiles = *added, *removed, *files, *newFiles
		m.Schema, m.Trial, m.Turns, m.Arm, m.Session = *schema, *trial, *turns, *arm, *sessionID
		m.Carry, m.CarryMeasured, m.MS = *carry, set["carry"], *ms
		if *carryMissing {
			m.CarryMeasured = false
		}
		m.Work, m.WorkMissing, m.Attempts = *work, *workMissing, *attempts
		b, _ := json.Marshal(m)
		fmt.Println(string(b))
	default:
		flag.Usage()
		os.Exit(2)
	}
}

// refuseSchema2 keeps -compare's paired statistics off schema-2 rows: one
// row is one trial with two turns, so its token volume is not the single-prompt
// quantity -compare was built for. The message names the flag that does read
// them. A row without a schema is schema 1 and stays comparable.
func refuseSchema2(rows ...[]Metrics) error {
	for _, group := range rows {
		for _, m := range group {
			if m.Schema >= 2 {
				return fmt.Errorf("-compare is for schema-1 rows; %s has a schema-%d trial row (use -compare-carry)", m.Scenario, m.Schema)
			}
		}
	}
	return nil
}

// checkRowContract refuses an incomplete row instead of writing one: a
// schema-2 trial row without its arm, its trial number, its session id or a
// carry observation is how a harness bug would become a silent zero in the
// statistics (plan v13, Task 1 — "missing carry" and "mixed schemas" must
// fail for the intended reason).
func checkRowContract(schema int, set map[string]bool, carryMissing, workMissing bool, trial, turns int, arm, sessionID string) error {
	if schema == 0 {
		if set["trial"] || set["arm"] || set["carry"] {
			return fmt.Errorf("schema-2 flags given without -schema 2")
		}
		return nil
	}
	if schema != 2 {
		return fmt.Errorf("unknown row schema %d (want 2)", schema)
	}
	for _, req := range []struct {
		name string
		ok   bool
	}{
		{"trial", trial > 0},
		{"turns", turns > 0},
		{"arm", arm != ""},
		{"sessionId", sessionID != ""},
	} {
		if !req.ok {
			return fmt.Errorf("schema 2 row needs -%s", req.name)
		}
	}
	if !set["carry"] && !carryMissing {
		return fmt.Errorf("schema 2 row needs -carry (or -carryMissing to record the gap): an unmeasured trial must not read as zero")
	}
	// The changeset campaign's endpoint is the work count, so a schema-2 row
	// without it is incomplete for every comparison that uses one — but old
	// rows (carry-era labels) predate the field and must stay readable, which
	// is why the refusal is on the writer, not on the reader.
	if !set["work"] && !workMissing {
		return fmt.Errorf("schema 2 row needs -work (or -workMissing to record the gap): the work count is the changeset endpoint")
	}
	return nil
}

func readResultsFile(path string) []Metrics {
	f, err := os.Open(path)
	if err != nil {
		fatal(err)
	}
	defer f.Close()
	rows, err := readResults(f)
	if err != nil {
		fatal(err)
	}
	return rows
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "evalstats:", err)
	os.Exit(1)
}

func readResults(r io.Reader) ([]Metrics, error) {
	var out []Metrics
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	var lines []string
	for sc.Scan() {
		if line := strings.TrimSpace(sc.Text()); line != "" {
			lines = append(lines, line)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	for i, line := range lines {
		var m Metrics
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			if i == len(lines)-1 {
				// A harness killed mid-append leaves a torn final line. It is
				// not a corrupt file, and refusing every later summary would
				// make the whole label unreadable until someone edited it by
				// hand — report it and read the rest.
				fmt.Fprintf(os.Stderr, "evalstats: ignoring a torn final line (%d bytes) in the results file\n", len(line))
				break
			}
			return nil, fmt.Errorf("bad results line %d: %w", i+1, err)
		}
		out = append(out, m)
	}
	return out, nil
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
	sb.WriteString("| label | scenario | runs | pass | tokens (mean ± sd) | steps (mean ± sd) | lines (mean ± sd) | new files | cache eff | first-hit | breaks/run | redundant read tok | cost $ | tokens/solved | repeated fails | skill writes | skill listed |\n")
	sb.WriteString("|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|\n")
	for _, k := range keys {
		g := groups[k]
		var toks, steps, firstHit, churn []float64
		passes, rep, breaks, redundant, skillWrites, skillListed, newFiles := 0, 0, 0, 0, 0, 0, 0
		cost, total, eff := 0.0, 0, 0.0
		for _, m := range g {
			toks = append(toks, float64(m.Tokens()))
			steps = append(steps, float64(m.Steps))
			firstHit = append(firstHit, float64(m.FirstCacheRead))
			churn = append(churn, float64(m.Churn()))
			newFiles += m.NewFiles
			if m.Pass {
				passes++
			}
			rep += m.RepeatedFailures
			breaks += len(m.CacheBreaks)
			redundant += m.RedundantReadTokens
			cost += m.Cost
			total += m.Tokens()
			eff += m.CacheEff
			skillWrites += m.SkillWrites
			if m.SkillListed {
				skillListed++
			}
		}
		perSolved := "—"
		if passes > 0 {
			perSolved = fmt.Sprintf("%.0f", float64(total)/float64(passes))
		}
		tm, ts := meanSD(toks)
		sm, ss := meanSD(steps)
		cm, cs := meanSD(churn)
		n := float64(len(g))
		fmt.Fprintf(&sb, "| %s | %s | %d | %d/%d | %.0f ± %.0f | %.1f ± %.1f | %.0f ± %.0f | %.0f | %.2f | %.0f | %.1f | %.0f | %.4f | %s | %d | %d | %d/%d |\n",
			k.label, k.scenario, len(g), passes, len(g), tm, ts, sm, ss,
			cm, cs, float64(newFiles)/n,
			eff/n, median(firstHit), float64(breaks)/n, float64(redundant)/n, cost, perSolved, rep, skillWrites, skillListed, len(g))
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
