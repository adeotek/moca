package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/adeotek/moca/internal/session"
)

// CLI: replay -base bin/moca-base -cand bin/moca -label <label> [-scenarios a,b]
//
// For every kept transcript test/evals/results/<label>/<scenario>-<n>.jsonl it
// re-runs both binaries on the model's recorded turns and compares request
// bytes and prefix breaks: zero-variance measurement of harness-side changes.
func main() {
	base := flag.String("base", "", "base binary (required)")
	cand := flag.String("cand", "", "candidate binary (required)")
	label := flag.String("label", "", "results label under test/evals/results/<label>/ (required)")
	scenarios := flag.String("scenarios", "", "optional comma-separated scenario filter")
	evals := flag.String("evals", "test/evals", "eval corpus root")
	ctxWindow := flag.Int("contextWindow", 1000000, "model context window for the replayed runs (glm-5.3-flash: 1000000)")
	flag.Parse()
	if *base == "" || *cand == "" || *label == "" {
		flag.Usage()
		os.Exit(2)
	}
	abs := func(p string) string {
		a, err := filepath.Abs(p)
		if err != nil {
			fatal(err)
		}
		return a
	}
	baseBin, candBin := abs(*base), abs(*cand)
	for _, b := range []string{baseBin, candBin} {
		if st, err := os.Stat(b); err != nil || st.Mode()&0o111 == 0 {
			fatal(fmt.Errorf("not an executable: %s", b))
		}
	}
	files, err := filepath.Glob(filepath.Join(*evals, "results", *label, "*.jsonl"))
	if err != nil || len(files) == 0 {
		fatal(fmt.Errorf("no transcripts under %s", filepath.Join(*evals, "results", *label)))
	}
	sort.Strings(files)
	want := map[string]bool{}
	if *scenarios != "" {
		for _, s := range strings.Split(*scenarios, ",") {
			want[strings.TrimSpace(s)] = true
		}
	}
	scratch, err := os.MkdirTemp("", "moca-replay-*")
	if err != nil {
		fatal(err)
	}
	defer os.RemoveAll(scratch)

	type row struct {
		name                 string
		reqB, reqC           int
		bytesB, bytesC       int
		breaksB, breaksC     int
		divergedB, divergedC bool
	}
	var rows []row
	var sumB, sumC int
	for _, f := range files {
		scen := strings.TrimSuffix(filepath.Base(f), ".jsonl")
		if i := strings.LastIndex(scen, "-"); i > 0 {
			scen = scen[:i]
		}
		// run.sh names every transcript <scenario>.<session>-<run>.jsonl: the
		// scenario DIRECTORY (fixture, flags, sessions.txt) is the part before
		// the session suffix, and -scenarios accepts either spelling.
		dir := scen
		if i := strings.Index(dir, "."); i > 0 {
			dir = dir[:i]
		}
		if len(want) > 0 && !want[scen] && !want[dir] {
			continue
		}
		// A multi-session scenario cannot be replayed: the replay starts from
		// the scenario's fixture, where session 1's skills do not exist yet, so
		// session 2 diverges by construction.
		if _, err := os.Stat(filepath.Join(*evals, dir, "sessions.txt")); err == nil {
			fmt.Printf("skip %s: multi-session scenario (the fixture lacks the earlier sessions)\n", filepath.Base(f))
			continue
		}
		entries, err := session.ReadFile(f)
		if err != nil {
			fmt.Printf("skip %s: %v\n", filepath.Base(f), err)
			continue
		}
		if UsesWeb(entries) {
			fmt.Printf("skip %s: the session uses the web tool (not hermetic)\n", filepath.Base(f))
			continue
		}
		turns, prompt := Turns(entries), PromptOf(entries)
		if len(turns) == 0 || prompt == "" {
			fmt.Printf("skip %s: no turns or no prompt in the transcript\n", filepath.Base(f))
			continue
		}
		// Equal-length Dirs for the pair: both paths appear in the system prompt.
		run := func(bin, tag string) (Result, error) {
			return Run(context.Background(), Options{
				Moca: bin, Fixture: filepath.Join(*evals, dir, "fixture"), Prompt: prompt,
				Flags: scenarioFlags(filepath.Join(*evals, dir, "flags")),
				Dirs:  filepath.Join(scratch, tag), ContextWindow: *ctxWindow, Turns: turns,
			})
		}
		rb, errB := run(baseBin, "base")
		rc, errC := run(candBin, "cand")
		if errB != nil || errC != nil {
			fmt.Printf("skip %s: base err=%v cand err=%v\n", filepath.Base(f), errB, errC)
			continue
		}
		r := row{name: filepath.Base(f), reqB: rb.Requests, reqC: rc.Requests,
			bytesB: sum(rb.Bytes), bytesC: sum(rc.Bytes),
			breaksB: rb.PrefixBreaks, breaksC: rc.PrefixBreaks,
			divergedB: rb.Diverged, divergedC: rc.Diverged}
		if !r.divergedB && !r.divergedC {
			sumB += r.bytesB
			sumC += r.bytesC
		}
		rows = append(rows, r)
	}
	fmt.Printf("| transcript | requests b→c | Σ bytes b→c | Δ%% | prefix breaks b/c | diverged |\n")
	fmt.Printf("|---|---|---|---|---|---|\n")
	for _, r := range rows {
		delta := "—"
		if r.bytesB > 0 && !r.divergedB && !r.divergedC {
			delta = fmt.Sprintf("%+.1f%%", 100*float64(r.bytesC-r.bytesB)/float64(r.bytesB))
		}
		fmt.Printf("| %s | %d→%d | %d→%d | %s | %d/%d | %v/%v |\n",
			r.name, r.reqB, r.reqC, r.bytesB, r.bytesC, delta, r.breaksB, r.breaksC, r.divergedB, r.divergedC)
	}
	if sumB > 0 || sumC > 0 {
		fmt.Printf("| **Σ (non-diverged)** | | %d→%d | %+.1f%% | | |\n", sumB, sumC, 100*float64(sumC-sumB)/float64(sumB))
	}
	fmt.Println("\nReplay is counterfactual: the model's turns are fixed, so this measures formatting and")
	fmt.Println("token-accounting changes exactly and says nothing about how a real model would react —")
	fmt.Println("behavior changes still need the eval corpus (`evalstats -compare`). Diverged runs asked")
	fmt.Println("for more turns than recorded and are reported but excluded from Σ.")
}

func scenarioFlags(path string) []string {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var out []string
	for _, f := range strings.Fields(string(b)) {
		out = append(out, f)
	}
	return out
}

func sum(xs []int) int {
	n := 0
	for _, x := range xs {
		n += x
	}
	return n
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "replay:", err)
	os.Exit(1)
}
