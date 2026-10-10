package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
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
    --yolo | --no-yolo         all permission checks off/on for this run (overrides config yolo)
    --plan                     plan mode: analyze the request and write an implementation plan to docs/plans/ (no other writes)
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

// YoloOn: the flag wins; otherwise config `yolo` decides (§7.5).
func (o Options) YoloOn(cfg config.Config) bool {
	if o.Yolo != nil {
		return *o.Yolo
	}
	return cfg.Yolo
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
	if *yolo || *noYolo {
		v := *yolo
		o.Yolo = &v
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
