package plugins

// registry maps an enabled plugin to the kind its factory expects.
// Generated from plugins/data.json — regenerate, do not hand-edit.
var registry = map[string]string{
	"auth":      "http",
	"billing":   "grpc",
	"catalog":   "http",
	"checkout":  "grpc",
	"config":    "file",
	"events":    "kafka",
	"files":     "s3",
	"flags":     "redis",
	"inventory": "sql",
	"ledger":    "sql",
	"metrics":   "kafka",
	"orders":    "grpc",
	"pricing":   "http",
	"queue":     "amqp",
	"search":    "http",
	"sessions":  "redis",
	"shipping":  "http",
	"users":     "sql",
}
