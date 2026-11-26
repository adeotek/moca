package plugins

// registry maps an enabled plugin to the kind its factory expects.
// Generated from plugins/data.json — regenerate, do not hand-edit.
var registry = map[string]string{
	"audit":     "audit@755166",
	"auth":      "auth@31fd13",
	"billing":   "billing@cddff1",
	"flags":     "flags@94ca1a",
	"mailer":    "mailer@e4ce1b",
	"metrics":   "metrics@4cda38",
	"notify":    "notify@941fc0",
	"orders":    "orders@9f1b55",
	"pricing":   "pricing@90be21",
	"quota":     "quota@5aed92",
	"search":    "search@e3d954",
	"search-v2": "search@a306ac",
	"sessions":  "sessions@b83753",
	"shipping":  "shipping@34295c",
	"taxes":     "taxes@0fce0c",
	"users":     "users@279b3b",
	"webhooks":  "webhooks@dedba9",
	"zones":     "zones@d1d55e",
}
