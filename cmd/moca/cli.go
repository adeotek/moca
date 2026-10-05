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

type Options struct {
	Prompt     string
	OneShot    bool
	Model      string
	Effort     string
	Approve    *bool
	Yolo       *bool
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
	fs.StringVar(&o.Resume, "resume", "", "resume session id8 | last")
	fs.BoolVar(&o.Continue, "continue", false, "resume latest session in this workdir")
	fs.BoolVar(&o.Version, "version", false, "print version")
	fs.StringVar(&o.ConfigPath, "config", "", "config file (dev/test)")
	if err := fs.Parse(args); err != nil {
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
	if o.OneShot && strings.TrimSpace(o.Prompt) == "" {
		return o, usageError{errors.New("-p needs a non-empty prompt")}
	}
	if o.Resume != "" && o.Continue {
		return o, usageError{errors.New("--resume and --continue are mutually exclusive")}
	}
	o.Sub = fs.Args()
	if len(o.Sub) > 0 && !(o.Sub[0] == "login" || o.Sub[0] == "logout" || o.Sub[0] == "mcp") {
		return o, usageError{fmt.Errorf("unknown command %q", o.Sub[0])}
	}
	return o, nil
}
