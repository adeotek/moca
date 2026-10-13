package acp

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
)

// node is a JSON-schema fragment: only the keywords conform reads.
type node struct {
	Ref        string           `json:"$ref"`
	Type       any              `json:"type"`
	Required   []string         `json:"required"`
	Properties map[string]*node `json:"properties"`
	Items      *node            `json:"items"`
	Const      any              `json:"const"`
	Enum       []any            `json:"enum"`
	OneOf      []*node          `json:"oneOf"`
	AnyOf      []*node          `json:"anyOf"`
	AllOf      []*node          `json:"allOf"`
}

var (
	defsOnce sync.Once
	defs     map[string]*node
)

func loadDefs(t *testing.T) map[string]*node {
	t.Helper()
	defsOnce.Do(func() {
		defs = map[string]*node{}
		for _, f := range []string{"testdata/schema.json", "testdata/schema.unstable.json"} {
			b, err := os.ReadFile(f)
			if err != nil {
				continue // the unstable schema is optional
			}
			var doc struct {
				Defs  map[string]*node `json:"$defs"`
				Defs2 map[string]*node `json:"definitions"`
			}
			if err := json.Unmarshal(b, &doc); err != nil {
				panic(fmt.Sprintf("%s: %v", f, err))
			}
			for k, v := range doc.Defs {
				if _, ok := defs[k]; !ok {
					defs[k] = v
				}
			}
			for k, v := range doc.Defs2 {
				if _, ok := defs[k]; !ok {
					defs[k] = v
				}
			}
		}
	})
	if len(defs) == 0 {
		t.Fatal("no ACP schema in testdata (Task 2 Step 1)")
	}
	return defs
}

// conform checks v's JSON against the pinned schema's def: types, required
// properties, const/enum values, $ref/allOf, and union branches (oneOf/anyOf:
// at least one must match). Deliberately partial — it pins what moca emits.
func conform(t *testing.T, def string, v any) {
	t.Helper()
	d := loadDefs(t)
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var x any
	json.Unmarshal(b, &x)
	if _, ok := d[def]; !ok {
		t.Fatalf("schema has no def %q", def)
	}
	if err := check(d, &node{Ref: "#/$defs/" + def}, x, def); err != nil {
		t.Errorf("%v\njson: %s", err, b)
	}
}

func resolve(d map[string]*node, n *node) (*node, error) {
	for n.Ref != "" {
		name := n.Ref[strings.LastIndex(n.Ref, "/")+1:]
		next, ok := d[name]
		if !ok {
			return nil, fmt.Errorf("unknown $ref %s", n.Ref)
		}
		n = next
	}
	return n, nil
}

func jsonType(x any) string {
	switch v := x.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case float64:
		if v == float64(int64(v)) {
			return "integer"
		}
		return "number"
	case string:
		return "string"
	case []any:
		return "array"
	default:
		return "object"
	}
}

func typeOK(n *node, x any) bool {
	var want []string
	switch t := n.Type.(type) {
	case string:
		want = []string{t}
	case []any:
		for _, s := range t {
			want = append(want, s.(string))
		}
	default:
		return true
	}
	got := jsonType(x)
	return slices.Contains(want, got) || (got == "integer" && slices.Contains(want, "number"))
}

func check(d map[string]*node, n *node, x any, path string) error {
	n, err := resolve(d, n)
	if err != nil {
		return fmt.Errorf("%s: %v", path, err)
	}
	if !typeOK(n, x) {
		return fmt.Errorf("%s: type %s, want %v", path, jsonType(x), n.Type)
	}
	for _, sub := range n.AllOf {
		if err := check(d, sub, x, path); err != nil {
			return err
		}
	}
	if n.Const != nil && !reflect.DeepEqual(n.Const, x) {
		return fmt.Errorf("%s: %v, want const %v", path, x, n.Const)
	}
	if len(n.Enum) > 0 && !slices.ContainsFunc(n.Enum, func(e any) bool { return reflect.DeepEqual(e, x) }) {
		return fmt.Errorf("%s: %v not in %v", path, x, n.Enum)
	}
	if alts := append(append([]*node{}, n.OneOf...), n.AnyOf...); len(alts) > 0 {
		var errs []string
		matched := false
		for _, a := range alts {
			if err := check(d, a, x, path); err == nil {
				matched = true
				break
			} else {
				errs = append(errs, err.Error())
			}
		}
		if !matched {
			return fmt.Errorf("%s: no alternative matches: %s", path, strings.Join(errs, " | "))
		}
	}
	if obj, ok := x.(map[string]any); ok {
		for _, r := range n.Required {
			if _, ok := obj[r]; !ok {
				return fmt.Errorf("%s: missing required %q", path, r)
			}
		}
		for k, p := range n.Properties {
			if v, ok := obj[k]; ok {
				if err := check(d, p, v, path+"."+k); err != nil {
					return err
				}
			}
		}
	}
	if arr, ok := x.([]any); ok && n.Items != nil {
		for i, v := range arr {
			if err := check(d, n.Items, v, fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
	}
	return nil
}

func TestConformRejectsBadShapes(t *testing.T) {
	d := loadDefs(t)
	bad := SessionNotification{SessionID: "abcd1234", Update: map[string]any{"sessionUpdate": "no_such_update"}}
	b, _ := json.Marshal(bad)
	var x any
	json.Unmarshal(b, &x)
	if check(d, &node{Ref: "#/$defs/SessionNotification"}, x, "n") == nil {
		t.Fatal("checker accepted an unknown sessionUpdate")
	}
}
