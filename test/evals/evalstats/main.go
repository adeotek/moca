// Command evalstats turns moca session transcripts into eval metrics.
//
//	evalstats -session s.jsonl [-scenario name -run n -pass -exit code]   # one JSON line
//	evalstats -summary results.jsonl                                      # markdown table
//
// run.sh appends one line per scenario run to a results file; -summary
// aggregates it (pass rate, tokens, steps — mean ± sd) for BASELINE.md.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strings"

	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/session"
)

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
}

// Tokens is every billed token of the run (cache reads included: they are
// cheaper, not free).
func (m Metrics) Tokens() int { return m.Input + m.CacheRead + m.CacheWrite + m.Output }

// Collect computes the metrics of one transcript.
func Collect(entries []session.Entry) Metrics {
	m := Metrics{Calls: map[string]int{}}
	calls := map[string]llm.ToolCall{}
	seenFail := map[string]bool{}
	for _, e := range entries {
		if e.Usage != nil {
			m.Input += e.Usage.Input
			m.CacheRead += e.Usage.CacheRead
			m.CacheWrite += e.Usage.CacheWrite
			m.Output += e.Usage.Output
		}
		m.Cost += e.Cost
		switch e.Type {
		case session.TypeMessage:
			if e.Message != nil && e.Message.Role == llm.RoleAssistant {
				m.Steps++
			}
		case session.TypeToolUse:
			calls[e.ToolUse.Call.ID] = e.ToolUse.Call
			m.Calls[e.ToolUse.Call.Name]++
		case session.TypeToolResult:
			if !e.ToolResult.IsError {
				continue
			}
			m.ToolErrors++
			c := calls[e.ToolResult.CallID]
			key := c.Name + "\x00" + string(c.Input) + "\x00" + e.ToolResult.Content
			if seenFail[key] {
				m.RepeatedFailures++
			}
			seenFail[key] = true
		case session.TypeCompaction:
			m.Compactions++
		}
	}
	if n := len(entries); n > 1 {
		m.Seconds = math.Round(entries[n-1].Time.Sub(entries[0].Time).Seconds()*10) / 10
	}
	return m
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
// label: pass rate, tokens and steps as mean ± sd, cost, and tokens per
// solved task (all tokens of the group / passes).
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
	sb.WriteString("| label | scenario | runs | pass | tokens (mean ± sd) | steps (mean ± sd) | cost $ | tokens/solved | repeated fails |\n")
	sb.WriteString("|---|---|---|---|---|---|---|---|---|\n")
	for _, k := range keys {
		g := groups[k]
		var toks, steps []float64
		passes, rep := 0, 0
		cost, total := 0.0, 0
		for _, m := range g {
			toks = append(toks, float64(m.Tokens()))
			steps = append(steps, float64(m.Steps))
			if m.Pass {
				passes++
			}
			rep += m.RepeatedFailures
			cost += m.Cost
			total += m.Tokens()
		}
		perSolved := "—"
		if passes > 0 {
			perSolved = fmt.Sprintf("%.0f", float64(total)/float64(passes))
		}
		tm, ts := meanSD(toks)
		sm, ss := meanSD(steps)
		fmt.Fprintf(&sb, "| %s | %s | %d | %d/%d | %.0f ± %.0f | %.1f ± %.1f | %.4f | %s | %d |\n",
			k.label, k.scenario, len(g), passes, len(g), tm, ts, sm, ss, cost, perSolved, rep)
	}
	return sb.String()
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
