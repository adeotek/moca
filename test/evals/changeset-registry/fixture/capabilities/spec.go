package capabilities

// Spec is one plugin's static description; the fields mirror plugins/data.json.
type Spec struct {
	Name     string
	Kind     string
	Endpoint string
	Timeout  int
	Digest   string
}
