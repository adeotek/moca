package tui

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/config"
)

// TestDefaultThemeComplete: every role has a built-in default in both
// variants, and the defaults pass the config's own color validation — a role
// added to config.ThemeColors without a default would render as no color.
func TestDefaultThemeComplete(t *testing.T) {
	d := DefaultTheme()
	for name, c := range map[string]Colors{"dark": d.Dark, "light": d.Light} {
		rv, rt := reflect.ValueOf(c), reflect.TypeOf(c)
		for i := range rt.NumField() {
			if rv.Field(i).String() == "" {
				t.Errorf("%s.%s has no built-in default", name, rt.Field(i).Name)
			}
		}
	}
	b, err := json.Marshal(map[string]any{"tui": map[string]any{"theme": map[string]any{"dark": d.Dark, "light": d.Light}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := config.Parse(b); err != nil {
		t.Fatalf("built-in defaults fail config validation: %v", err)
	}
}

// TestThemeOverridesPrecedence: per variant, the variant's block wins over
// `both`; empty fields keep the current value; the other variant is
// untouched; a second overlay wins per field where it sets a key.
func TestThemeOverridesPrecedence(t *testing.T) {
	user := config.ThemeConfig{
		Dark: config.ThemeColors{UserBg: "#010203", Accent: "9"},
		Both: config.ThemeColors{Muted: "8"},
	}
	th := DefaultTheme().WithOverrides(user)
	if th.Dark.UserBg != "#010203" {
		t.Fatalf("variant override lost: %q", th.Dark.UserBg)
	}
	if th.Light.UserBg != DefaultTheme().Light.UserBg {
		t.Fatalf("dark-only override leaked into light: %q", th.Light.UserBg)
	}
	if th.Dark.Muted != "8" || th.Light.Muted != "8" {
		t.Fatalf("both must fill both variants: %q / %q", th.Dark.Muted, th.Light.Muted)
	}
	if th.Light.Accent != DefaultTheme().Light.Accent {
		t.Fatalf("light accent must keep its default: %q", th.Light.Accent)
	}
	second := config.ThemeConfig{
		Dark: config.ThemeColors{UserBg: "#040506"},
		Both: config.ThemeColors{Accent: "3"},
	}
	th = th.WithOverrides(second)
	if th.Dark.UserBg != "#040506" {
		t.Fatalf("second overlay must win: %q", th.Dark.UserBg)
	}
	if th.Dark.Accent != "3" || th.Light.Accent != "3" {
		t.Fatalf("second overlay both: %q / %q", th.Dark.Accent, th.Light.Accent)
	}
	if th.Dark.Muted != "8" {
		t.Fatalf("first overlay both must survive where the second is silent: %q", th.Dark.Muted)
	}
}

// A custom theme reaches every render path: the message and response bands,
// the code band and the status bar all read the model's theme.
func TestCustomThemeRenders(t *testing.T) {
	th := DefaultTheme().WithOverrides(config.ThemeConfig{
		Both: config.ThemeColors{UserBg: "#010203", RespBg: "#040506", CodeBg: "#070809", StatusCwd: "#0a0b0c"},
	})
	m := newTestModel()
	m.th = th
	user := printed(m.printlnUser("hi"))
	ack(m)
	if !strings.Contains(user, "48;2;1;2;3") {
		t.Fatalf("custom user band missing: %q", user)
	}
	resp := printed(m.printlnResponse("yo"))
	ack(m)
	if !strings.Contains(resp, "48;2;4;5;6") {
		t.Fatalf("custom response band missing: %q", resp)
	}
	var st mdState
	if out := renderMarkdown(th, &st, "```\nx\n```", 40, true); !strings.Contains(out, "48;2;7;8;9") {
		t.Fatalf("custom code band missing: %q", out)
	}
	if line := th.styleLine([]segment{{text: "~/p", role: roleCwd}}, true, pressureOK); !strings.Contains(line, "38;2;10;11;12") {
		t.Fatalf("custom status color missing: %q", line)
	}
}

// fgSGR converts the theme's color forms to the raw SGR the markdown code
// span toggles with: hex → truecolor, ANSI numbers → their palette forms.
func TestFGSSR(t *testing.T) {
	cases := map[string]string{
		"#f2c57c": "\x1b[38;2;242;197;124m",
		"#000000": "\x1b[38;2;0;0;0m",
		"1":       "\x1b[31m",
		"9":       "\x1b[91m",
		"208":     "\x1b[38;5;208m",
		"bogus":   sgrFgOff,
	}
	for in, want := range cases {
		if got := fgSGR(in); got != want {
			t.Errorf("fgSGR(%q) = %q, want %q", in, got, want)
		}
	}
}
