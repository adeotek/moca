package tools

import (
	"strings"
	"testing"
)

func mustApply(t *testing.T, in, old, new string, all bool) string {
	t.Helper()
	out, _, err := applyEdit([]byte(in), old, new, all)
	if err != nil {
		t.Fatalf("applyEdit: %v", err)
	}
	return string(out)
}

func applyErr(t *testing.T, in, old, new string, all bool) string {
	t.Helper()
	_, _, err := applyEdit([]byte(in), old, new, all)
	if err == nil {
		t.Fatal("want error")
	}
	return err.Error()
}

func TestEditLadder(t *testing.T) {
	t.Run("exact unique", func(t *testing.T) {
		if got := mustApply(t, "a\nb\nc\n", "b\n", "B\n", false); got != "a\nB\nc\n" {
			t.Fatal(got)
		}
	})
	t.Run("exact ambiguous lists lines", func(t *testing.T) {
		e := applyErr(t, "x\ny\nx\n", "x", "z", false)
		if !strings.Contains(e, "2 matches") || !strings.Contains(e, "lines 1, 3") || !strings.Contains(e, "replace_all") {
			t.Fatal(e)
		}
	})
	t.Run("replace_all n hits", func(t *testing.T) {
		out, hunks, err := applyEdit([]byte("x\ny\nx\nx\n"), "x", "z", true)
		if err != nil || string(out) != "z\ny\nz\nz\n" || len(hunks) != 3 {
			t.Fatal(string(out), len(hunks), err)
		}
	})
	t.Run("replace_all never uses fallback", func(t *testing.T) {
		applyErr(t, "\tx := 1\n", "    x := 1", "y", true)
	})
	t.Run("fallback deeper indent", func(t *testing.T) {
		in := "func f() {\n\t\tif a {\n\t\t\tb()\n\t\t}\n}\n"
		got := mustApply(t, in, "if a {\n\tb()\n}", "if a {\n\tc()\n\td()\n}", false)
		if got != "func f() {\n\t\tif a {\n\t\t\tc()\n\t\t\td()\n\t\t}\n}\n" {
			t.Fatalf("%q", got)
		}
	})
	t.Run("fallback shallower indent", func(t *testing.T) {
		in := "class A:\n    def f(self):\n        return 1\n"
		got := mustApply(t, in, "        def f(self):\n            return 1", "        def f(self):\n            return 2", false)
		if got != "class A:\n    def f(self):\n        return 2\n" {
			t.Fatalf("%q", got)
		}
	})
	t.Run("fallback ambiguous", func(t *testing.T) {
		e := applyErr(t, "  a\n  b\n    a\n    b\n", "a\nb", "c", false)
		if !strings.Contains(e, "whitespace-insensitive") {
			t.Fatal(e)
		}
	})
	t.Run("no match", func(t *testing.T) {
		e := applyErr(t, "a\n", "zzz", "y", false)
		if !strings.Contains(e, "not found") || !strings.Contains(e, "re-read") {
			t.Fatal(e)
		}
	})
	t.Run("no match with line-number prefixes", func(t *testing.T) {
		e := applyErr(t, "a\nb\n", "1|a\n2|b", "c", false)
		if !strings.Contains(e, "N|") {
			t.Fatalf("must explain read prefixes: %s", e)
		}
	})
	t.Run("no match with line-number prefixes and replace_all", func(t *testing.T) {
		e := applyErr(t, "a\nb\n", "1|a\n2|b", "c", true)
		if !strings.Contains(e, "N|") {
			t.Fatalf("replace_all must explain read prefixes too: %s", e)
		}
	})
	t.Run("no-op", func(t *testing.T) {
		if e := applyErr(t, "a\n", "a", "a", false); !strings.Contains(e, "identical") {
			t.Fatal(e)
		}
	})
	t.Run("empty old_string", func(t *testing.T) {
		if e := applyErr(t, "a\n", "", "b", false); !strings.Contains(e, "write") {
			t.Fatal(e)
		}
	})
	t.Run("CRLF stays CRLF", func(t *testing.T) {
		got := mustApply(t, "a\r\nb\r\nc\r\n", "b\nc", "B\nC", false)
		if got != "a\r\nB\r\nC\r\n" {
			t.Fatalf("%q", got)
		}
	})
	t.Run("BOM preserved and unmatchable", func(t *testing.T) {
		got := mustApply(t, "\ufeffa\nb\n", "a", "A", false)
		if got != "\ufeffA\nb\n" {
			t.Fatalf("%q", got)
		}
		applyErr(t, "\ufeffa\n", "\ufeffa", "b", false)
	})
	t.Run("unicode", func(t *testing.T) {
		if got := mustApply(t, "naïve → 日本\n", "→ 日本", "← 中文", false); got != "naïve ← 中文\n" {
			t.Fatal(got)
		}
	})
	t.Run("50-line span", func(t *testing.T) {
		var in, old, nw strings.Builder
		in.WriteString("head\n")
		for i := range 50 {
			line := strings.Repeat("x", i%7) + "\n"
			in.WriteString(line)
			old.WriteString(line)
			nw.WriteString("y" + line)
		}
		in.WriteString("tail\n")
		got := mustApply(t, in.String(), old.String(), nw.String(), false)
		if !strings.HasPrefix(got, "head\ny") || !strings.HasSuffix(got, "tail\n") {
			t.Fatal(got[:20])
		}
	})
	t.Run("missing trailing newline preserved", func(t *testing.T) {
		if got := mustApply(t, "a\nb", "b", "c", false); got != "a\nc" {
			t.Fatalf("%q", got)
		}
	})
	t.Run("non-UTF-8 refused", func(t *testing.T) {
		if e := applyErr(t, "a\xff\n", "a", "b", false); !strings.Contains(e, "UTF-8") {
			t.Fatal(e)
		}
	})
}

func TestFormatDiff(t *testing.T) {
	orig := "1\n2\n3\n4\n5\n6\n7\n8\n"
	out, hunks, _ := applyEdit([]byte(orig), "5\n", "five\n", false)
	d := formatDiff("f", splitLines(orig), splitLines(string(out)), hunks)
	want := "--- f\n+++ f\n@@ -2,7 +2,7 @@\n 2\n 3\n 4\n-5\n+five\n 6\n 7\n 8\n"
	if d != want {
		t.Fatalf("got\n%s\nwant\n%s", d, want)
	}
}
