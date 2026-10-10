package slug

import "testing"

func TestMake(t *testing.T) {
	cases := map[string]string{
		"Hello, World!":         "hello-world",
		"  spaced   out  ":      "spaced-out",
		"Go 1.27 Release Notes": "go-1-27-release-notes",
		"":                      "",
	}
	for in, want := range cases {
		if got := Make(in); got != want {
			t.Errorf("Make(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLimit(t *testing.T) {
	cases := []struct {
		in   string
		n    int
		want string
	}{
		{"hello-big-world", 20, "hello-big-world"},
		{"hello-big-world", 12, "hello-big"},
		{"hello-big-world", 9, "hello-big"},
		{"enormousword-x", 5, "enorm"},
	}
	for _, c := range cases {
		if got := Limit(c.in, c.n); got != c.want {
			t.Errorf("Limit(%q, %d) = %q, want %q", c.in, c.n, got, c.want)
		}
	}
}
