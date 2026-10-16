package tui

import (
	"reflect"
	"strings"
	"testing"

	"github.com/adeotek/moca/internal/config"
)

// TestThemeFieldsMirrorConfig pins the tui.Colors ↔ config.ThemeColors field
// lists together: a role added on one side without the other would either
// decode a config key that never applies or leave a color unconfigurable.
func TestThemeFieldsMirrorConfig(t *testing.T) {
	names := func(v any) map[string]bool {
		rv := reflect.ValueOf(v)
		rt := rv.Type()
		out := make(map[string]bool, rt.NumField())
		for i := range rt.NumField() {
			f := rt.Field(i)
			if f.Type.Kind() != reflect.String {
				t.Fatalf("%s field %s must be a string", rt.Name(), f.Name)
			}
			out[f.Name] = true
		}
		return out
	}
	tc, tw := names(config.ThemeColors{}), names(Colors{})
	for n := range tc {
		if !tw[n] {
			t.Errorf("config.ThemeColors.%s has no tui.Colors counterpart", n)
		}
	}
	for n := range tw {
		if !tc[n] {
			t.Errorf("tui.Colors.%s is not configurable (no config.ThemeColors field)", n)
		}
	}
}

// TestThemeOverridesPrecedence: per variant, the variant's block wins over
// `both`; empty fields keep the current value; the other variant is
// untouched — and cmd's layer order (user first, a trusted project second,
// §4) makes the project win per field.
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
	proj := config.ThemeConfig{
		Dark: config.ThemeColors{UserBg: "#040506"},
		Both: config.ThemeColors{Accent: "3"},
	}
	th = th.WithOverrides(proj)
	if th.Dark.UserBg != "#040506" {
		t.Fatalf("project must win over the user: %q", th.Dark.UserBg)
	}
	if th.Dark.Accent != "3" || th.Light.Accent != "3" {
		t.Fatalf("project both: %q / %q", th.Dark.Accent, th.Light.Accent)
	}
	if th.Dark.Muted != "8" {
		t.Fatalf("user both must survive where the project is silent: %q", th.Dark.Muted)
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
