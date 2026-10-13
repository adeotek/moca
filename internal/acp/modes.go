package acp

import (
	"encoding/json"
	"slices"

	"github.com/adeotek/moca/internal/agent"
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

// configOptions offers the models whose credential is available (the
// current one always) as the "model" select.
func (s *session) configOptions() []ConfigOption {
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
	return []ConfigOption{{ID: "model", Name: "Model", Category: "model", Type: "select", CurrentValue: cur, Options: vals}}
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
	if req.ConfigID != "model" {
		return nil, invalidParams("unknown config option %q", req.ConfigID)
	}
	var value string
	if err := json.Unmarshal(req.Value, &value); err != nil {
		return nil, invalidParams("model value must be a string")
	}
	return s.idle(func() (any, error) {
		if err := s.agent.SetModel(value, ""); err != nil {
			return nil, invalidParams("%v", err)
		}
		return SetConfigOptionResponse{ConfigOptions: s.configOptions()}, nil
	})
}
