package mcp

// Pin for T-8: a misspelled field in the mcp tool's input was silently dropped.
// `{"action":"call","server":"s","tool":"t","arguments":{...}}` decoded to
// Args=nil and the server was called with {} — a write tool ran with no
// arguments instead of telling the model it guessed the schema wrong.

import (
	"context"
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/tools"
)

func TestProxyToolRejectsAMisspelledField(t *testing.T) {
	p := &ProxyTool{}
	in := []byte(`{"action":"call","server":"s","tool":"t","arguments":{"x":1}}`)
	res := p.Run(context.Background(), &tools.Env{}, in)
	if !res.IsError {
		t.Fatalf("a misspelled field must be an error, got %q", res.Content)
	}
	if !strings.Contains(res.Content, "unknown field") {
		t.Fatalf("the error must name the problem, got %q", res.Content)
	}

	// Control: the right spelling gets past the decoder — an empty manager
	// then fails on the unknown server, which is a different message.
	m := NewManager(nil, 0, nil, Options{})
	ok := NewTool(m, nil).Run(context.Background(), &tools.Env{}, []byte(`{"action":"call","server":"s","tool":"t","args":{"x":1}}`))
	if strings.Contains(ok.Content, "unknown field") {
		t.Fatalf("the correct spelling was rejected: %q", ok.Content)
	}
}
