package plugins

// registry maps an enabled plugin to the kind its factory expects.
// Generated from plugins/data.json — regenerate, do not hand-edit.
var registry = map[string]string{
	"audit":     "audit@df1aa9",
	"auth":      "auth@bda1c2",
	"billing":   "billing@c28c31",
	"flags":     "flags@689ecd",
	"mailer":    "mailer@184463",
	"metrics":   "metrics@65f5cf",
	"notify":    "notify@3bd89e",
	"orders":    "orders@bb893b",
	"pricing":   "pricing@d4ef40",
	"quota":     "quota@468e74",
	"search":    "search@413b6a",
	"search-v2": "search@d06f60",
	"sessions":  "sessions@bd29c5",
	"shipping":  "shipping@682af7",
	"taxes":     "taxes@aaa3c2",
	"users":     "users@f16fb1",
	"webhooks":  "webhooks@bca497",
	"zones":     "zones@33b224",
}
