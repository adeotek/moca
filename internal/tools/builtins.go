package tools

// Builtins returns the eight frozen tools in schema order (§4, web added
// rev 19).
func Builtins() []Tool {
	return []Tool{readTool{}, writeTool{}, editTool{}, shellTool{}, searchTool{}, lsTool{}, webTool{}, mcpStub{}}
}
