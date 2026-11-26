package plugins

// registry maps an enabled plugin to the kind its factory expects.
// Generated from plugins/data.json — regenerate, do not hand-edit.
var registry = map[string]string{
	"auth":      "grpc",
	"billing":   "grpc",
	"catalog":   "http2",
	"checkout":  "grpc",
	"config":    "file",
	"events":    "nats",
	"files":     "s3",
	"flags":     "redis",
	"inventory": "sql",
	"ledger":    "sql",
	"metrics":   "kafka",
	"orders":    "grpc",
	"pricing":   "http",
	"queue":     "mqtt",
	"search":    "grpc",
	"sessions":  "redis",
	"shipping":  "http",
	"users":     "sql",
}
