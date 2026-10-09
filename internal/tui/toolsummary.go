package tui

import (
	"encoding/json"
	"strings"

	"github.com/adeotek/moca/internal/llm"
)

// toolArgSummary is a one-line description of what a tool call operates on —
// the command, path or pattern — for the activity row and for replayed items
// (a stored call keeps its input, not the live summary the tool produced).
func toolArgSummary(call llm.ToolCall) string {
	var a struct {
		Command string `json:"command"`
		Path    string `json:"path"`
		Pattern string `json:"pattern"`
		Server  string `json:"server"`
		Tool    string `json:"tool"`
		Action  string `json:"action"`
		Query   string `json:"query"`
	}
	if json.Unmarshal(call.Input, &a) != nil {
		return ""
	}
	var s string
	switch call.Name {
	case "shell":
		s = firstLineOf(a.Command)
	case "read", "write", "edit":
		s = a.Path
	case "ls":
		s = a.Path
		if s == "" {
			s = "."
		}
	case "search":
		s = a.Pattern
		if a.Path != "" {
			s += " in " + a.Path
		}
	case "mcp":
		switch {
		case a.Server != "" && a.Tool != "":
			s = a.Server + "/" + a.Tool
		case a.Query != "":
			s = a.Action + " " + a.Query
		default:
			s = a.Action
		}
	}
	return strings.TrimSpace(Sanitize(s))
}
