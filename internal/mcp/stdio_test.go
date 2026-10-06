package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/adeotek/moca/internal/config"
)

// TestHelperProcess is the fake stdio MCP server, run as a subprocess.
// Behaviour switches: MOCA_FAKE_MODE = "" | "noisy" | "die" | "paged" |
// "errtext" | "crashcall" (exit during tools/call) | "hangcall" / "hanginit"
// (never answer) | "loopcursor" (tools/list never ends).
func TestHelperProcess(t *testing.T) {
	if os.Getenv("MOCA_FAKE_MCP") != "1" {
		return
	}
	mode := os.Getenv("MOCA_FAKE_MODE")
	if mode == "die" {
		fmt.Fprintln(os.Stderr, "fatal: missing API token")
		os.Exit(3)
	}
	if f := os.Getenv("MOCA_FAKE_STARTS"); f != "" { // count spawns
		fh, _ := os.OpenFile(f, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		fh.WriteString("x")
		fh.Close()
	}
	in := bufio.NewScanner(os.Stdin)
	out := json.NewEncoder(os.Stdout)
	if mode == "noisy" {
		fmt.Println("server starting up...")
	}
	for in.Scan() {
		var req struct {
			ID     *int64          `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		json.Unmarshal(in.Bytes(), &req)
		if req.ID == nil {
			continue
		}
		var result any
		switch req.Method {
		case "initialize":
			if mode == "hanginit" {
				continue
			}
			result = map[string]any{"protocolVersion": "2025-06-18", "capabilities": map[string]any{"tools": map[string]any{}},
				"serverInfo": map[string]any{"name": "fake", "version": "1"}}
			if mode == "noisy" {
				out.Encode(map[string]any{"jsonrpc": "2.0", "id": 999, "method": "ping"}) // server→client request
			}
		case "tools/list":
			var p struct {
				Cursor string `json:"cursor"`
			}
			json.Unmarshal(req.Params, &p)
			ro := true
			page1 := []map[string]any{
				{"name": "read_doc", "description": "Read library documentation", "inputSchema": map[string]any{"type": "object", "properties": map[string]any{"lib": map[string]any{"type": "string"}}}, "annotations": map[string]any{"readOnlyHint": ro}},
				{"name": "create_issue", "description": "Create an issue", "inputSchema": map[string]any{"type": "object"}},
			}
			if mode == "loopcursor" {
				result = map[string]any{"tools": page1[:1], "nextCursor": "same"}
			} else if mode == "paged" && p.Cursor == "" {
				result = map[string]any{"tools": page1[:1], "nextCursor": "p2"}
			} else if mode == "paged" {
				result = map[string]any{"tools": page1[1:]}
			} else {
				result = map[string]any{"tools": page1}
			}
		case "tools/call":
			if mode == "crashcall" {
				os.Exit(5)
			}
			if mode == "hangcall" {
				continue
			}
			if mode == "errtext" {
				out.Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "error": map[string]any{"code": -32000, "message": "job exited"}})
				continue
			}
			var p struct {
				Name      string         `json:"name"`
				Arguments map[string]any `json:"arguments"`
			}
			json.Unmarshal(req.Params, &p)
			result = map[string]any{"content": []map[string]any{
				{"type": "text", "text": fmt.Sprintf("%s(%v)", p.Name, p.Arguments)},
				{"type": "image", "data": "AAAA", "mimeType": "image/png"}}}
		default:
			out.Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "error": map[string]any{"code": -32601, "message": "nope"}})
			continue
		}
		if mode == "noisy" {
			fmt.Println("log: handled", req.Method)
		}
		out.Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
	}
}

func fakeServer(mode string) config.MCPServer {
	return config.MCPServer{Command: os.Args[0], Args: []string{"-test.run=TestHelperProcess"},
		Env: map[string]string{"MOCA_FAKE_MCP": "1", "MOCA_FAKE_MODE": mode}}
}

func dial(t *testing.T, s config.MCPServer) *client {
	t.Helper()
	tr, err := startStdio(context.Background(), "fake", s, os.Environ())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { tr.Close() })
	if err := initialize(context.Background(), tr); err != nil {
		t.Fatal(err)
	}
	return &client{t: tr, name: "fake"}
}

func TestStdioListAndCall(t *testing.T) {
	c := dial(t, fakeServer("noisy"))
	tools, err := c.listTools(context.Background())
	if err != nil || len(tools) != 2 || *tools[0].Annotations.ReadOnlyHint != true {
		t.Fatal(tools, err)
	}
	r, err := c.callTool(context.Background(), "read_doc", json.RawMessage(`{"lib":"go"}`))
	if err != nil || r.Content[0].Text != "read_doc(map[lib:go])" || r.Content[1].Type != "image" {
		t.Fatal(r, err)
	}
}

func TestStdioPaged(t *testing.T) {
	tools, err := dial(t, fakeServer("paged")).listTools(context.Background())
	if err != nil || len(tools) != 2 {
		t.Fatal(tools, err)
	}
}

func TestStdioEarlyExit(t *testing.T) {
	tr, err := startStdio(context.Background(), "fake", fakeServer("die"), os.Environ())
	if err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err = initialize(ctx, tr)
	}
	if err == nil || !strings.Contains(err.Error(), "missing API token") {
		t.Fatalf("exit surfaces stderr, no hang: %v", err)
	}
}

func TestFilterEnv(t *testing.T) {
	got := FilterEnv([]string{"PATH=/bin", "HOME=/h", "OPENAI_API_KEY=x", "XDG_CONFIG_HOME=/c", "AWS_SECRET=y", "TERM=xterm"})
	if !slices.Equal(got, []string{"PATH=/bin", "HOME=/h", "XDG_CONFIG_HOME=/c", "TERM=xterm"}) {
		t.Fatal(got)
	}
	t.Setenv("MOCA_T_TOK", "sekret")
	env, err := serverEnv([]string{"PATH=/bin"}, map[string]string{"TOKEN": "env:MOCA_T_TOK", "MODE": "x"})
	if err != nil || !slices.Contains(env, "TOKEN=sekret") || !slices.Contains(env, "MODE=x") {
		t.Fatal(env, err)
	}
}

// An explicit env entry replaces the inherited variable of the same name:
// duplicate names resolve platform-dependently (glibc keeps the first), so an
// appended override behind the inherited entry would be silently ignored.
func TestServerEnvExplicitEntryReplacesInherited(t *testing.T) {
	env, err := serverEnv([]string{"PATH=/base:/usr/bin", "HOME=/h"}, map[string]string{"PATH": "/custom/bin"})
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, kv := range env {
		if strings.HasPrefix(kv, "PATH=") {
			n++
			if kv != "PATH=/custom/bin" {
				t.Fatalf("PATH = %q, want the explicit value", kv)
			}
		}
	}
	if n != 1 {
		t.Fatalf("%d PATH entries in %v", n, env)
	}
	if !slices.Contains(env, "HOME=/h") {
		t.Fatalf("unrelated inherited entries must survive: %v", env)
	}
}

// The write-error branch must surface the recorded exit error with its stderr
// tail (wrapping errTransportDead), never a bare EPIPE or a ctx error — the
// manager's restart-once predicate keys on the sentinel (pass-1 M2).
func TestStdioWriteEPIPE(t *testing.T) {
	tr, err := startStdio(context.Background(), "fake", fakeServer("die"), os.Environ())
	if err != nil {
		t.Fatal(err)
	}
	st := tr.(*stdioTransport)
	st.stdin.Close() // the pipe is gone: write() must fail
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = tr.Call(ctx, "initialize", nil)
	if err == nil || !strings.Contains(err.Error(), "missing API token") {
		t.Fatalf("write failure must surface the exit error: %v", err)
	}
	if !errors.Is(err, errTransportDead) {
		t.Fatalf("the exit error must wrap errTransportDead: %v", err)
	}
}
