package tools

// Builtins returns the seven frozen tools in schema order (§4).
func Builtins() []Tool {
	return []Tool{readTool{}, writeTool{}, editTool{}, shellTool{}, searchTool{}, lsTool{}, mcpStub{}}
}
