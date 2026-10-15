package acp

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// scriptServer serves openai-completions SSE responses from a script, one per request.
type scriptServer struct {
	mu     sync.Mutex
	turns  []string
	bodies []map[string]any
	srv    *httptest.Server
}

func textTurn(s string) string {
	b, _ := json.Marshal(s)
	return fmt.Sprintf("data: {\"choices\":[{\"delta\":{\"content\":%s},\"finish_reason\":\"stop\"}]}\n\ndata: {\"choices\":[],\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":5}}\n\ndata: [DONE]\n\n", b)
}

func toolTurn(calls ...[2]string) string {
	var sb strings.Builder
	for i, c := range calls {
		args, _ := json.Marshal(c[1])
		fmt.Fprintf(&sb, "data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":%d,\"id\":\"call_%d\",\"function\":{\"name\":%q,\"arguments\":%s}}]}}]}\n\n", i, i, c[0], args)
	}
	sb.WriteString("data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"tool_calls\"}]}\n\ndata: [DONE]\n\n")
	return sb.String()
}

func newScript(t *testing.T, turns ...string) *scriptServer {
	s := &scriptServer{turns: turns}
	s.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		b, _ := io.ReadAll(r.Body)
		var m map[string]any
		json.Unmarshal(b, &m)
		s.bodies = append(s.bodies, m)
		w.Header().Set("Content-Type", "text/event-stream")
		if len(s.turns) == 0 {
			io.WriteString(w, textTurn("(script exhausted)"))
			return
		}
		io.WriteString(w, s.turns[0])
		s.turns = s.turns[1:]
	}))
	t.Cleanup(s.srv.Close)
	return s
}

func mustJSON(v any) string { b, _ := json.Marshal(v); return string(b) }

// lastUserText returns the last user message text of the last request.
func lastUserText(t *testing.T, s *scriptServer) string {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.bodies) == 0 {
		t.Fatal("no request made")
	}
	msgs, _ := s.bodies[len(s.bodies)-1]["messages"].([]any)
	for i := len(msgs) - 1; i >= 0; i-- {
		m := msgs[i].(map[string]any)
		if m["role"] == "user" {
			return mustJSON(m["content"])
		}
	}
	return ""
}
