package acp

import (
	"encoding/json"
	"testing"
)

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
	if r.Modes == nil || r.Modes.CurrentModeID != "default" || len(r.Modes.AvailableModes) != 4 {
		t.Fatalf("modes %+v", r.Modes)
	}
	if len(r.ConfigOptions) != 1 || r.ConfigOptions[0].ID != "model" || r.ConfigOptions[0].CurrentValue != "fake/m" ||
		len(r.ConfigOptions[0].Options) != 2 {
		t.Fatalf("config options %+v", r.ConfigOptions)
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
	for _, m := range []string{"plan", "auto", "yolo", "default"} {
		if _, err := c.call("session/set_mode", SetModeRequest{SessionID: sid, ModeID: m}); err != nil {
			t.Fatalf("%s: %v", m, err.Message)
		}
		ups := c.updatesOf(updCurrentMode)
		if len(ups) == 0 || ups[len(ups)-1]["currentModeId"] != m {
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
	if r.ConfigOptions[0].CurrentValue != "fake/m2" {
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
	c := startClient(t, testOptions(t, s))
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
