package acp

import (
	"encoding/json"
	"strings"
	"testing"
)

// optionOf finds a config option by id (the list carries mode, model, …).
func optionOf(t *testing.T, opts []ConfigOption, id string) ConfigOption {
	t.Helper()
	for _, o := range opts {
		if o.ID == id {
			return o
		}
	}
	t.Fatalf("no %q option in %+v", id, opts)
	return ConfigOption{}
}

func newSessionFull(t *testing.T, c *testClient, cwd string) NewSessionResponse {
	t.Helper()
	res, err := c.call("session/new", NewSessionRequest{Cwd: cwd, MCPServers: []json.RawMessage{}})
	if err != nil {
		t.Fatal(err.Message)
	}
	var r NewSessionResponse
	json.Unmarshal(res, &r)
	return r
}

func TestNewSessionAdvertisesModesAndModels(t *testing.T) {
	c := startClient(t, testOptions(t, newScript(t)))
	r := newSessionFull(t, c, t.TempDir())
	conform(t, "NewSessionResponse", r)
	if r.Modes == nil || r.Modes.CurrentModeID != "auto" || len(r.Modes.AvailableModes) != 4 {
		t.Fatalf("modes %+v", r.Modes)
	}
	if len(r.ConfigOptions) != 2 {
		t.Fatalf("config options %+v", r.ConfigOptions)
	}
	// The modes API and the "mode" config option carry the same state.
	if m := optionOf(t, r.ConfigOptions, "mode"); m.Category != "mode" || m.CurrentValue != "auto" ||
		len(m.Options) != 4 || m.Options[0].Value != "ask" || m.Options[3].Value != "plan" {
		t.Fatalf("mode option %+v", m)
	}
	model := optionOf(t, r.ConfigOptions, "model")
	if model.CurrentValue != "fake/m" || len(model.Options) != 2 {
		t.Fatalf("model option %+v", model)
	}
}

func TestModeFlagSetsInitialMode(t *testing.T) {
	o := testOptions(t, newScript(t))
	o.Mode = "plan"
	c := startClient(t, o)
	if r := newSessionFull(t, c, t.TempDir()); r.Modes.CurrentModeID != "plan" {
		t.Fatalf("mode %q", r.Modes.CurrentModeID)
	}
}

func TestSetMode(t *testing.T) {
	c := startClient(t, testOptions(t, newScript(t)))
	sid := newSessionFull(t, c, t.TempDir()).SessionID
	for _, m := range []string{"plan", "auto", "yolo", "ask", "default"} {
		if _, err := c.call("session/set_mode", SetModeRequest{SessionID: sid, ModeID: m}); err != nil {
			t.Fatalf("%s: %v", m, err.Message)
		}
		// "default" is v0.7.1's name for ask: accepted, reported canonically.
		want := "ask"
		if m != "default" {
			want = m
		}
		ups := c.updatesOf(updCurrentMode)
		if len(ups) == 0 || ups[len(ups)-1]["currentModeId"] != want {
			t.Fatalf("after %s: %v", m, ups)
		}
	}
	if _, err := c.call("session/set_mode", SetModeRequest{SessionID: sid, ModeID: "turbo"}); err == nil || err.Code != codeInvalidParams {
		t.Fatalf("bogus mode: %+v", err)
	}
}

func TestSetModel(t *testing.T) {
	c := startClient(t, testOptions(t, newScript(t)))
	sid := newSessionFull(t, c, t.TempDir()).SessionID
	res, err := c.call("session/set_config_option", map[string]any{"sessionId": sid, "configId": "model", "value": "fake/m2"})
	if err != nil {
		t.Fatal(err.Message)
	}
	var r SetConfigOptionResponse
	json.Unmarshal(res, &r)
	if model := optionOf(t, r.ConfigOptions, "model"); model.CurrentValue != "fake/m2" {
		t.Fatalf("%+v", r)
	}
	for _, bad := range []map[string]any{
		{"sessionId": sid, "configId": "model", "value": "fake/nope"},
		{"sessionId": sid, "configId": "effort", "value": "high"},
	} {
		if _, err := c.call("session/set_config_option", bad); err == nil || err.Code != codeInvalidParams {
			t.Errorf("%v: %+v", bad, err)
		}
	}
}

func TestSetModeSendsOneUpdate(t *testing.T) {
	c := startClient(t, testOptions(t, newScript(t)))
	sid := newSessionFull(t, c, t.TempDir()).SessionID
	c.call("session/set_mode", SetModeRequest{SessionID: sid, ModeID: "plan"})
	before := len(c.updatesOf(updCurrentMode))
	c.call("session/set_mode", SetModeRequest{SessionID: sid, ModeID: "auto"})
	ups := c.updatesOf(updCurrentMode)[before:]
	if len(ups) != 1 || ups[0]["currentModeId"] != "auto" {
		t.Fatalf("plan→auto must be one update to auto, got %v", ups)
	}
	before = len(c.updatesOf(updCurrentMode))
	c.call("session/set_mode", SetModeRequest{SessionID: sid, ModeID: "auto"})
	if n := len(c.updatesOf(updCurrentMode)) - before; n != 0 {
		t.Fatalf("an unchanged mode sent %d updates", n)
	}
}

func TestSetModeRefusedWhileRunning(t *testing.T) {
	s, work := rmScript(t)
	c := startClient(t, askOptions(t, s))
	sid := c.newSession(work)
	got := make(chan *RPCError, 1)
	c.onPermission = func(RequestPermissionRequest) RequestPermissionResponse {
		_, err := c.call("session/set_mode", SetModeRequest{SessionID: sid, ModeID: "yolo"})
		got <- err
		return selected(optReject)
	}
	c.prompt(sid, "delete x")
	if err := <-got; err == nil || err.Code != codeInvalidRequest {
		t.Fatalf("mid-run set_mode: %+v", err)
	}
}

// The mode is offered under the config-options API too (the spec's transition
// rule), and choosing it there must reach the modes API as well.
func TestSetModeViaConfigOption(t *testing.T) {
	c := startClient(t, testOptions(t, newScript(t)))
	sid := newSessionFull(t, c, t.TempDir()).SessionID
	res, err := c.call("session/set_config_option", map[string]any{"sessionId": sid, "configId": "mode", "value": "plan"})
	if err != nil {
		t.Fatal(err.Message)
	}
	var r SetConfigOptionResponse
	json.Unmarshal(res, &r)
	if m := optionOf(t, r.ConfigOptions, "mode"); m.CurrentValue != "plan" {
		t.Fatalf("mode option %+v", m)
	}
	ups := c.updatesOf(updCurrentMode)
	if len(ups) != 1 || ups[0]["currentModeId"] != "plan" {
		t.Fatalf("the modes API must carry the same state: %v", ups)
	}
	conform(t, "SessionNotification", SessionNotification{SessionID: sid, Update: ups[0]})
	if _, err := c.call("session/set_config_option", map[string]any{"sessionId": sid, "configId": "mode", "value": "turbo"}); err == nil || err.Code != codeInvalidParams {
		t.Fatalf("bogus mode: %+v", err)
	}
}

// session/set_mode (the modes API) must report the config option: a client
// that uses config options reads the mode from there, not from `modes`.
func TestSetModeReportsTheConfigOption(t *testing.T) {
	c := startClient(t, testOptions(t, newScript(t)))
	sid := newSessionFull(t, c, t.TempDir()).SessionID
	c.call("session/set_mode", SetModeRequest{SessionID: sid, ModeID: "yolo"})
	ups := c.updatesOf(updConfigOptions)
	if len(ups) != 1 {
		t.Fatalf("want one config_option_update, got %v", ups)
	}
	if b := mustJSON(ups[0]); !strings.Contains(b, `"id":"mode"`) || !strings.Contains(b, `"currentValue":"yolo"`) {
		t.Fatalf("update: %s", b)
	}
	conform(t, "SessionNotification", SessionNotification{SessionID: sid, Update: ups[0]})
	// An unchanged mode reports nothing.
	before := len(c.updatesOf(updConfigOptions))
	c.call("session/set_mode", SetModeRequest{SessionID: sid, ModeID: "yolo"})
	if n := len(c.updatesOf(updConfigOptions)) - before; n != 0 {
		t.Fatalf("a repeated set_mode sent %d updates", n)
	}
}
