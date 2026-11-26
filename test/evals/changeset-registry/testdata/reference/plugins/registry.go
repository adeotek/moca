package plugins

// registry maps an enabled plugin to the kind its factory expects.
// Generated from plugins/data.json — regenerate, do not hand-edit.
var registry = map[string]string{
	"audit":     "audit@92ef00",
	"auth":      "auth@0b8fda",
	"billing":   "billing@4cc881",
	"flags":     "flags@57306d",
	"mailer":    "mailer@84be4f",
	"metrics":   "metrics@e97662",
	"notify":    "notify@9c789d",
	"orders":    "orders@d29e64",
	"pricing":   "pricing@9e40bd",
	"quota":     "quota@c13dab",
	"search":    "search@258495",
	"search-v2": "search@599b7c",
	"sessions":  "sessions@f9923a",
	"shipping":  "shipping@de5b1a",
	"taxes":     "taxes@924566",
	"users":     "users@e509c1",
	"webhooks":  "webhooks@28e74c",
	"zones":     "zones@487e2d",
}
