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
    --approve | --no-approve   project trust for this run (-p default: --no-approve)
    --effort <level>           override effort for this session (off|minimal|low|medium|high|xhigh|max)
    --config <path>            config file (dev/test)
    --continue                 latest session in this workdir
    --mode <m>                 permission mode for this run: default | auto | yolo | plan
                               (plan = the plan scope: write an implementation plan to docs/plans/, no other writes)
    --model <provider/model>   override config `+"`"+`model`+"`"+` for this session
    --resume <id8|last>        resume a session of this directory (last = its newest)
  moca execute <plan.md> [-p "<notes>"] execute an implementation plan step by step, ticking its - [ ] boxes
  moca login <provider>        sign in (subscription OAuth where permitted) or store an API key
    --api-key                  prompt for an API key instead of the OAuth flow
    --no-browser               headless OAuth: print the URL, read the pasted code from stdin
  moca logout <provider>       clear a stored login or API key
  moca update                  replace the binary with the latest GitHub release package
    --check                    only report whether a newer release exists
  moca prune                   remove built-in skill extractions left behind by previous versions
    --dry-run                  only list what would be removed
  moca mcp import              import MCP servers from Claude Code / OpenCode / Pi configs
  moca mcp index               prebuild the persisted MCP discovery index
  moca acp                     serve the Agent Client Protocol on stdio (Zed, VS Code, Paseo, …)
    --model / --effort / --mode   as above, for every session the editor opens
    --approve / --no-approve   trust / do not trust project resources (default: the
                               saved decision for the folder, else trusted)
  moca --version

logs: ~/.local/state/moca/logs/<date>-<pid>.log — config `+"`"+`log.level`+"`"+` (info|debug|off); MOCA_LOG overrides it for one run
`)
}

type Options struct {
	Prompt     string
	OneShot    bool
	Model      string
	Effort     string
	Approve    *bool
	Mode       *string
	Execute    string // plan file to execute (implies a one-shot run)
	Resume     string
	Continue   bool
	Version    bool
	Sub        []string
	ConfigPath string
}

type usageError struct{ error }

// cliModes are the --mode values — the permission modes plus plan (the plan
// scope, which rides on ask; the agent enforces the pairing). The set is
// config.PermModes plus the legacy `default`, which v0.7.1 and earlier printed
// as the ask mode's name: a script that passes it keeps working (§3, §4).
var cliModes = append(slices.Clone(config.PermModes), config.LegacyPermMode)

// ResolveMode resolves the run's permission mode and plan scope (§3): an
// explicit --mode wins — plan maps to the plan scope at the default mode —
// else a trusted project file's defaultMode (the project layer wins over the
// user's), else the config's resolved mode. A configured auto/yolo under
// --mode plan yields to plan (default), so it never counts as yolo for the
// trust decision. `moca execute` is explicitly plan-exiting: a configured plan
// layer is skipped and the next layer decides (config.ResolveExecuteMode).
func (o Options) ResolveMode(cfg config.Config) (string, bool) {
	flag := ""
	if o.Mode != nil {
		flag = *o.Mode
	}
	if o.Execute != "" {
		return config.ResolveExecuteMode(flag, cfg), false
	}
	return config.ResolveMode(flag, cfg)
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
	mode := fs.String("mode", "", "permission mode for this run: default | auto | yolo | plan")
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
	// `execute <plan.md>`: the plan is positional; flags may follow it.
	if a := fs.Args(); len(a) > 0 && a[0] == "execute" {
		if len(a) < 2 || strings.HasPrefix(a[1], "-") {
			return o, usageError{errors.New("usage: moca execute <plan.md> [-p \"<notes>\"]")}
		}
		o.Execute = a[1]
		if err := fs.Parse(a[2:]); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return o, errHelp
			}
			return o, usageError{err}
		}
		if len(fs.Args()) > 0 {
			return o, usageError{fmt.Errorf("unexpected argument %q", fs.Args()[0])}
		}
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
	if *mode != "" {
		if !slices.Contains(cliModes, *mode) {
			return o, usageError{fmt.Errorf("--mode must be one of %s, got %q", strings.Join(config.PermModes, ", "), *mode)}
		}
		o.Mode = mode
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
	if o.Execute != "" {
		if o.Mode != nil && *o.Mode == "plan" {
			return o, usageError{errors.New("execute and --mode plan are mutually exclusive (plan mode only writes docs/plans/)")}
		}
		o.OneShot = true // -p, when given, adds instructions
	}
	if o.OneShot && o.Execute == "" && strings.TrimSpace(o.Prompt) == "" {
		return o, usageError{errors.New("-p needs a non-empty prompt")}
	}
	if o.Resume != "" && o.Continue {
		return o, usageError{errors.New("--resume and --continue are mutually exclusive")}
	}
	o.Sub = fs.Args()
	if len(o.Sub) > 0 && !(o.Sub[0] == "login" || o.Sub[0] == "logout" || o.Sub[0] == "mcp" || o.Sub[0] == "update" || o.Sub[0] == "acp" || o.Sub[0] == "prune") {
		return o, usageError{fmt.Errorf("unknown command %q", o.Sub[0])}
	}
	return o, nil
}
