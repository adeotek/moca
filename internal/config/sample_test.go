package config

import (
	"reflect"
	"strings"
	"testing"
)

// The sample must stay a config: strict decode + Validate, exactly what a run
// does with the user's file. A schema change that breaks the sample fails here
// rather than in someone's terminal.
func TestSampleParses(t *testing.T) {
	if _, err := Parse([]byte(Sample())); err != nil {
		t.Fatalf("the sample must parse as a user config: %v", err)
	}
}

// Completeness: every key the schema knows appears in the sample, active or
// commented. Reflection over the struct tree (Config itself, not ProjectConfig,
// which is a separate file's shape) so a new option cannot ship undocumented.
func TestSampleCoversEveryKey(t *testing.T) {
	var names []string
	// ThemeColors is exempt by name: 28 color roles, each optional and each
	// documented where it lives (internal/tui/theme.go, and the interactive
	// builder at docs/themes/tui-theme-builder.html, which emits a ready block).
	// Enumerating them would triple the sample without teaching anything the
	// builder does not.
	skip := map[reflect.Type]bool{reflect.TypeOf(ThemeColors{}): true}
	collectKeys(reflect.TypeOf(Config{}), &names, skip)
	sample := Sample()
	for _, k := range names {
		if !strings.Contains(sample, `"`+k+`"`) {
			t.Errorf("sample.jsonc does not mention %q — every option belongs in the sample", k)
		}
	}
}

// collectKeys walks the JSON shape of a struct type and collects field names,
// descending into structs, pointers and map values.
func collectKeys(t reflect.Type, out *[]string, skip map[reflect.Type]bool) {
	for t.Kind() == reflect.Pointer || t.Kind() == reflect.Map {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct || skip[t] {
		return
	}
	skip[t] = true
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		tag := f.Tag.Get("json")
		if tag == "-" || tag == "" {
			continue
		}
		name := strings.Split(tag, ",")[0]
		if name == "" || name == "-" {
			continue
		}
		*out = append(*out, name)
		collectKeys(f.Type, out, skip)
	}
}
