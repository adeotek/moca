package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/adeotek/moca/internal/config"
	"github.com/adeotek/moca/internal/llm"
)

const (
	exitOK          = 0
	exitRuntime     = 1
	exitUsage       = 2
	exitMaxSteps    = 3
	exitInterrupted = 130
)

// errHelp signals -h/--help: usage goes to stdout, exit 0.
var errHelp = errors.New("help requested")

func printUsage(w io.Writer) {
	fmt.Fprint(w, `moca — minimal, token-efficient coding agent

usage:
  moca                         TUI in the current directory
  moca -p "<prompt>"           one-shot; prompt also accepted on stdin (`+"`"+`-p -`+"`"+`)
    --model <provider/model>   override config `+"`"+`model`+"`"+` for this session
    --effort <level>           override effort for this session (off|minimal|low|medium|high|xhigh|max)
    --approve | --no-approve   project trust for this run (-p default: --no-approve)
    --mode <m>                 permission mode for this run: default | auto | yolo
    --yolo | --no-yolo         shorthand for --mode yolo / leave yolo (overrides config)
    --plan                     plan mode: analyze the request and write an implementation plan to docs/plans/ (no other writes; not with yolo)
  moca --do <plan.md> [-p "<notes>"]  execute an implementation plan step by step, ticking its - [ ] boxes
    --resume <id8|last>        resume a session
    --continue                 latest session in this workdir
    --config <path>            config file (dev/test)
  moca login <provider>        sign in (subscription OAuth where permitted) or store an API key
    --api-key                  prompt for an API key instead of the OAuth flow
    --no-browser               headless OAuth: print the URL, read the pasted code from stdin
  moca logout <provider>       clear a stored login or API key
  moca update                  replace the binary with the latest GitHub release package
    --check                    only report whether a newer release exists
  moca mcp import              import MCP servers from Claude Code / OpenCode / Pi configs
  moca mcp index               prebuild the persisted MCP discovery index
  moca --version
  logs: ~/.local/state/moca/logs/<date>-<pid>.log — config log.level (info|debug|off); MOCA_LOG overrides for one run
`)
}

type Options struct {
	Prompt     string
	OneShot    bool
	Model      string
	Effort     string
	Approve    *bool
	Mode       *string
	Yolo       *bool
	Plan       bool
	Do         string // plan file to execute (implies a one-shot run)
	Resume     string
	Continue   bool
	Version    bool
	Sub        []string
	ConfigPath string
}

type usageError struct{ error }

// ModeOf resolves the run's permission mode: an explicit flag wins
// (--mode/--yolo/--no-yolo, mutually exclusive), else a trusted project
// file's defaultMode, else the user config Mode() (§11). --no-yolo maps a
// resolved yolo to default and leaves auto alone; so does --plan (yolo and
// plan are mutually exclusive — parseArgs rejects --plan with an explicit
// --yolo/--mode yolo, a configured yolo yields to the flag).
func (o Options) ModeOf(cfg config.Config) (string, error) {
	var flag string
	switch {
	case o.Mode != nil:
		flag = *o.Mode
	case o.Yolo != nil:
		flag = "yolo"
		if !*o.Yolo {
			// --no-yolo: suppress yolo, other modes stay as configured.
			flag = ""
		}
	}
	base := cfg.Mode()
	if cfg.Project != nil && cfg.Project.DefaultMode != "" {
		base = cfg.Project.DefaultMode
	}
	if flag != "" {
		base = flag
	}
	if base == "yolo" && (o.Plan || (o.Yolo != nil && !*o.Yolo)) {
		base = "default"
	}
	return base, nil
}

func parseArgs(args []string, stdin io.Reader) (Options, error) {
	var o Options
	fs := flag.NewFlagSet("moca", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&o.Prompt, "p", "", "one-shot prompt (- = stdin)")
	fs.StringVar(&o.Model, "model", "", "provider/model")
	fs.StringVar(&o.Effort, "effort", "", "effort level")
	approve := fs.Bool("approve", false, "trust project resources for this run")
	noApprove := fs.Bool("no-approve", false, "skip project resources for this run")
	mode := fs.String("mode", "", "permission mode for this run: default | auto | yolo")
	yolo := fs.Bool("yolo", false, "turn all permission checks off for this run")
	noYolo := fs.Bool("no-yolo", false, "keep permission checks on (overrides config yolo)")
	fs.BoolVar(&o.Plan, "plan", false, "plan mode: write an implementation plan to docs/plans/, change nothing else")
	fs.StringVar(&o.Do, "do", "", "execute the implementation plan in this file (one-shot; -p adds instructions)")
	fs.StringVar(&o.Resume, "resume", "", "resume session id8 | last")
	fs.BoolVar(&o.Continue, "continue", false, "resume latest session in this workdir")
	fs.BoolVar(&o.Version, "version", false, "print version")
	fs.StringVar(&o.ConfigPath, "config", "", "config file (dev/test)")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return o, errHelp
		}
		return o, usageError{err}
	}
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "p" {
			o.OneShot = true
		}
	})
	if *approve && *noApprove {
		return o, usageError{errors.New("--approve and --no-approve are mutually exclusive")}
	}
	if *approve || *noApprove {
		v := *approve
		o.Approve = &v
	}
	if *yolo && *noYolo {
		return o, usageError{errors.New("--yolo and --no-yolo are mutually exclusive")}
	}
	n := 0
	if *mode != "" {
		n++
	}
	if *yolo {
		n++
	}
	if *noYolo {
		n++
	}
	if n > 1 {
		return o, usageError{errors.New("--mode, --yolo and --no-yolo are mutually exclusive")}
	}
	if *mode != "" {
		if !slices.Contains(config.PermModes, *mode) {
			return o, usageError{fmt.Errorf("--mode must be one of %s, got %q", strings.Join(config.PermModes, ", "), *mode)}
		}
		o.Mode = mode
	}
	if *yolo || *noYolo {
		v := *yolo
		o.Yolo = &v
	}
	if o.Plan && (*yolo || *mode == "yolo") {
		return o, usageError{errors.New("--plan and yolo mode are mutually exclusive (plan mode keeps writes inside docs/plans/)")}
	}
	if o.Effort != "" {
		if _, err := llm.ParseEffort(o.Effort); err != nil {
			return o, usageError{err}
		}
	}
	if o.OneShot && o.Prompt == "-" {
		if stdin == nil {
			return o, usageError{errors.New("-p - needs stdin")}
		}
		b, err := io.ReadAll(stdin)
		if err != nil {
			return o, err
		}
		o.Prompt = strings.TrimRight(string(b), "\n")
	}
	if o.Do != "" {
		if o.Plan {
			return o, usageError{errors.New("--do and --plan are mutually exclusive (plan mode only writes docs/plans/)")}
		}
		o.OneShot = true // -p, when given, adds instructions
	}
	if o.OneShot && o.Do == "" && strings.TrimSpace(o.Prompt) == "" {
		return o, usageError{errors.New("-p needs a non-empty prompt")}
	}
	if o.Resume != "" && o.Continue {
		return o, usageError{errors.New("--resume and --continue are mutually exclusive")}
	}
	o.Sub = fs.Args()
	if len(o.Sub) > 0 && !(o.Sub[0] == "login" || o.Sub[0] == "logout" || o.Sub[0] == "mcp" || o.Sub[0] == "update") {
		return o, usageError{fmt.Errorf("unknown command %q", o.Sub[0])}
	}
	return o, nil
}
