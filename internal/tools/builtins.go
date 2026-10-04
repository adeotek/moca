package tools

// Builtins returns the seven frozen tools in schema order (§4). Tasks 3–10
// add each tool here.
func Builtins() []Tool { return []Tool{readTool{}, lsTool{}, searchTool{}} }
