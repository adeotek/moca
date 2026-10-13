package acp

import (
	"encoding/json"
	"fmt"
	"slices"

	"github.com/adeotek/moca/internal/agent"
	"github.com/adeotek/moca/internal/llm"
	"github.com/adeotek/moca/internal/provider"
)

// sessionModes are moca's permission modes plus the plan scope (which rides
// on default), worded as the TUI's mode badge and notices.
var sessionModes = []SessionMode{
	{ID: "default", Name: "Default", Description: "Allowlisted actions run; everything else asks"},
	{ID: "auto", Name: "Auto", Description: "Safe actions run, risky ones ask, unsure ones get a classifier's opinion"},
	{ID: "yolo", Name: "Yolo", Description: "All permission checks are off"},
	{ID: "plan", Name: "Plan", Description: "Write an implementation plan to docs/plans/; no other writes"},
}

func (s *session) modeID() string {
	if s.agent.Plan() {
		return "plan"
	}
	return string(s.agent.Mode())
}

// modeState builds the modes block of a session/new or session/load
// response and records the id the client is told.
func (s *session) modeState() *SessionModeState {
	id := s.modeID()
	s.mu.Lock()
	s.lastMode = id
	s.mu.Unlock()
	return &SessionModeState{CurrentModeID: id, AvailableModes: sessionModes}
}

// reportMode sends current_mode_update when the mode id differs from the
// last one the client saw. SetMode from plan to auto emits PlanChanged
// (mode still default) and then ModeChanged, so set_mode mutes the events
// while it runs and reports once; agent-side changes report directly.
func (s *session) reportMode() {
	id := s.modeID()
	s.mu.Lock()
	if s.modeMuted || id == s.lastMode {
		s.mu.Unlock()
		return
	}
	s.lastMode = id
	s.mu.Unlock()
	s.send(CurrentModeUpdate{SessionUpdate: updCurrentMode, CurrentModeID: id})
}

func (s *session) muteMode(on bool) {
	s.mu.Lock()
	s.modeMuted = on
	s.mu.Unlock()
}

// buildConfigOptions offers the models whose credential is available (the
// current one always) as the "model" select, plus the effort select when
// the model has levels to pick.
func (s *session) buildConfigOptions() []ConfigOption {
	cur := s.agent.Model().Qualified()
	var vals []ConfigOptionValue
	for _, m := range s.agent.Models() {
		q := m.Qualified()
		if q == cur || s.agent.HasCredential(q) {
			vals = append(vals, ConfigOptionValue{Value: q, Name: q})
		}
	}
	if !slices.ContainsFunc(vals, func(v ConfigOptionValue) bool { return v.Value == cur }) {
		vals = append([]ConfigOptionValue{{Value: cur, Name: cur}}, vals...)
	}
	opts := []ConfigOption{{ID: "model", Name: "Model", Category: "model", Type: "select", CurrentValue: cur, Options: vals}}
	if e := effortOption(s.agent.Model(), s.agent.Effort()); e != nil {
		opts = append(opts, *e)
	}
	return opts
}

// configOptions builds the options for a response and records them as what
// the client has seen.
func (s *session) configOptions() []ConfigOption {
	opts := s.buildConfigOptions()
	b, _ := json.Marshal(opts)
	s.mu.Lock()
	s.lastConfig = string(b)
	s.mu.Unlock()
	return opts
}

// reportConfig sends config_option_update when the options changed since
// the client last saw them — e.g. a `moca login` in a terminal made another
// provider's models usable. Called after each prompt.
func (s *session) reportConfig() {
	opts := s.buildConfigOptions()
	b, _ := json.Marshal(opts)
	s.mu.Lock()
	changed := string(b) != s.lastConfig
	s.lastConfig = string(b)
	s.mu.Unlock()
	if changed {
		s.send(ConfigOptionUpdate{SessionUpdate: updConfigOptions, ConfigOptions: opts})
	}
}

// effortLevels are the efforts m takes as-is, in llm.Efforts order; a model
// without thinking takes only "off".
func effortLevels(m provider.Model) []llm.Effort {
	var out []llm.Effort
	for _, e := range llm.Efforts {
		if m.ClampEffort(e) == e {
			out = append(out, e)
		}
	}
	return out
}

// effortOption is the "effort" select, or nil when m has fewer than two
// levels (nothing to pick).
func effortOption(m provider.Model, cur llm.Effort) *ConfigOption {
	lv := effortLevels(m)
	if len(lv) < 2 {
		return nil
	}
	o := ConfigOption{ID: "effort", Name: "Effort", Category: "thought_level", Type: "select", CurrentValue: string(cur)}
	for _, e := range lv {
		o.Options = append(o.Options, ConfigOptionValue{Value: string(e), Name: string(e)})
	}
	return &o
}

func (srv *Server) setMode(params json.RawMessage) (any, error) {
	req, err := decode[SetModeRequest](params)
	if err != nil {
		return nil, err
	}
	s, err := srv.lookup(req.SessionID)
	if err != nil {
		return nil, err
	}
	if !slices.ContainsFunc(sessionModes, func(m SessionMode) bool { return m.ID == req.ModeID }) {
		return nil, invalidParams("unknown mode %q", req.ModeID)
	}
	return s.idle(func() (any, error) {
		s.muteMode(true)
		if req.ModeID == "plan" {
			s.agent.SetPlan(true)
		} else {
			if s.agent.Plan() {
				s.agent.SetPlan(false)
			}
			s.agent.SetMode(agent.PermMode(req.ModeID))
		}
		s.muteMode(false)
		s.reportMode()
		return struct{}{}, nil
	})
}

func (srv *Server) setConfigOption(params json.RawMessage) (any, error) {
	req, err := decode[SetConfigOptionRequest](params)
	if err != nil {
		return nil, err
	}
	s, err := srv.lookup(req.SessionID)
	if err != nil {
		return nil, err
	}
	var value string
	if err := json.Unmarshal(req.Value, &value); err != nil {
		return nil, invalidParams("%s value must be a string", req.ConfigID)
	}
	var apply func() error
	switch req.ConfigID {
	case "model":
		apply = func() error { return s.agent.SetModel(value, "") }
	case "effort":
		apply = func() error {
			if !slices.Contains(effortLevels(s.agent.Model()), llm.Effort(value)) {
				return fmt.Errorf("effort %q is not offered by %s", value, s.agent.Model().Qualified())
			}
			_, err := s.agent.SetEffort(llm.Effort(value))
			return err
		}
	default:
		return nil, invalidParams("unknown config option %q", req.ConfigID)
	}
	return s.idle(func() (any, error) {
		if err := apply(); err != nil {
			return nil, invalidParams("%v", err)
		}
		return SetConfigOptionResponse{ConfigOptions: s.configOptions()}, nil
	})
}
