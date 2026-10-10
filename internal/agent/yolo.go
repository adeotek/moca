package agent

import (
	"github.com/adeotek/moca/internal/permissions"
	"github.com/adeotek/moca/internal/session"
	"github.com/adeotek/moca/internal/tools"
)

type YoloChanged struct{ On bool }

func (YoloChanged) isEvent() {}

func (a *Agent) Yolo() bool { return a.yolo }

func (a *Agent) applyYolo(on bool) {
	env := a.opts.Env
	if on {
		env.Paths, env.Commands, env.Ask = permissions.NewUnjailed(env.Root), permissions.AllowAll{}, tools.AutoAllow
	} else {
		env.Paths, env.Commands, env.Ask = a.strict.paths, a.strict.cmds, a.strict.ask
	}
	a.yolo = on
}

// SetYolo toggles yolo mode between runs (never during Run).
func (a *Agent) SetYolo(on bool) {
	a.applyYolo(on)
	a.append(session.Entry{Type: session.TypePermissionMode, PermissionMode: &session.PermissionMode{Yolo: on, Plan: a.plan}})
	a.emit(YoloChanged{On: on})
}
