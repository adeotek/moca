package tools

import (
	"context"
	"encoding/json"

	"github.com/adeotek/moca/internal/llm"
)

// MCPSpec is the frozen ~200-token schema of the lazy MCP proxy (§10.5).
// The phase-5 implementation in internal/mcp returns exactly this spec.
func MCPSpec() llm.ToolSpec {
	return llm.ToolSpec{Name: "mcp", Description: "Use tools from configured MCP servers (listed in the system " +
		"prompt). action=search finds tools by keyword (query, optional server); action=describe returns one " +
		"tool's input schema (server, tool); action=call runs it (server, tool, args). Describe before the first call of a tool.",
		Schema: json.RawMessage(`{"type":"object","properties":{` +
			`"action":{"type":"string","enum":["search","describe","call"]},` +
			`"server":{"type":"string"},"tool":{"type":"string"},` +
			`"args":{"type":"object","description":"Tool arguments for call"},` +
			`"query":{"type":"string","description":"Keywords for search"}},` +
			`"required":["action"],"additionalProperties":false}`)}
}

type mcpStub struct{}

func (mcpStub) Spec() llm.ToolSpec { return MCPSpec() }

func (mcpStub) Run(context.Context, *Env, json.RawMessage) Result {
	return errorf("no MCP servers configured")
}
