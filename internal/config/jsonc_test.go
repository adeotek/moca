// internal/config/jsonc_test.go
package config

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestStandardizeTable(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"plain json untouched", `{"a":[1,2],"b":"x"}`, `{"a":[1,2],"b":"x"}`},
		{"line comment", "{\"a\":1 // c\n}", "{\"a\":1     \n}"},
		{"block comment keeps newline", "{/*x\ny*/\"a\":1}", "{   \n   \"a\":1}"},
		{"url in string survives", `{"u":"https://x.io/a//b"}`, `{"u":"https://x.io/a//b"}`},
		{"escaped quote in string", `{"s":"a\"//b"}`, `{"s":"a\"//b"}`},
		{"trailing comma object", `{"a":1,}`, `{"a":1 }`},
		{"trailing comma array with comment", "[1, // x\n]", "[1      \n]"},
		{"comma before comment then value kept", "[1, /*c*/ 2]", "[1,       2]"},
		{"comment chars in string after comment", "{//x\n\"a\":\"/*\"}", "{   \n\"a\":\"/*\"}"},
	}
	for _, c := range cases {
		got, err := Standardize([]byte(c.in))
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if string(got) != c.want {
			t.Errorf("%s:\n got %q\nwant %q", c.name, got, c.want)
		}
		if len(got) != len(c.in) {
			t.Errorf("%s: length changed — error offsets would drift", c.name)
		}
	}
}

func TestStandardizeUnterminatedBlock(t *testing.T) {
	if _, err := Standardize([]byte(`{"a":1 /* oops`)); err == nil {
		t.Fatal("want error for unterminated block comment")
	}
}

func TestStandardizeNoOpOnPlainJSON(t *testing.T) {
	src := []byte("{\n  \"model\": \"a/b\",\n  \"x\": [1, 2, {\"y\": \"//\"}]\n}\n")
	got, _ := Standardize(src)
	if !bytes.Equal(got, src) {
		t.Fatal("pre-pass must be a no-op on plain JSON")
	}
	var v any
	if err := json.Unmarshal(got, &v); err != nil {
		t.Fatal(err)
	}
}
