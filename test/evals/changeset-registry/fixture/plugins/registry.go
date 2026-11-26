package plugins

// registry maps an enabled plugin to the kind its factory expects.
// Generated from plugins/data.json — regenerate, do not hand-edit.
var registry = map[string]string{
	"audit":     "audit@48171b",
	"auth":      "auth@5c30bf",
	"billing":   "billing@d26bd1",
	"flags":     "flags@201e77",
	"mailer":    "mailer@b60afb",
	"metrics":   "metrics@cb1a09",
	"notify":    "notify@5570c6",
	"orders":    "orders@5f3e46",
	"pricing":   "pricing@4ccfec",
	"quota":     "quota@0ac5e8",
	"search":    "search@e46163",
	"search-v2": "search@4474f8",
	"sessions":  "sessions@a0a01f",
	"shipping":  "shipping@fc4d97",
	"taxes":     "taxes@2e8f97",
	"users":     "users@97b63d",
	"webhooks":  "webhooks@d8378a",
	"zones":     "zones@270038",
}
