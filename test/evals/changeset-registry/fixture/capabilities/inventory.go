package capabilities

// Inventory is generated from plugins/data.json — edit the data, not this file.
var Inventory = Spec{
	Name:     "inventory",
	Kind:     "sql",
	Endpoint: "postgres://inventory.internal:5432/acme",
	Timeout:  12000,
}
