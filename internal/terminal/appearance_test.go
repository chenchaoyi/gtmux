package terminal

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormHex(t *testing.T) {
	for in, want := range map[string]string{
		"17171a": "#17171a", "#17171A": "#17171a", `"#D4D2CC"`: "#d4d2cc", "  #abc  ": "#abc", "": "",
	} {
		if got := normHex(in); got != want {
			t.Errorf("normHex(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGhosttyDarkTheme(t *testing.T) {
	for in, want := range map[string]string{
		"Dracula":                     "Dracula",
		"dark:Catppuccin,light:Latte": "Catppuccin",
		"light:Latte,dark:Catppuccin": "Catppuccin",
		"":                            "",
	} {
		if got := ghosttyDarkTheme(in); got != want {
			t.Errorf("ghosttyDarkTheme(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseITermFont(t *testing.T) {
	fam, size := parseITermFont("JetBrainsMono-Regular 13")
	if fam != "JetBrainsMono" || size != 13 {
		t.Errorf("parseITermFont = %q,%v want JetBrainsMono,13", fam, size)
	}
	if f, s := parseITermFont("Menlo 12.5"); f != "Menlo" || s != 12.5 {
		t.Errorf("parseITermFont fractional = %q,%v", f, s)
	}
}

func TestComp255(t *testing.T) {
	if comp255(1.0) != 255 || comp255(0.0) != 0 || comp255(0.5) != 128 || comp255(2.0) != 255 {
		t.Errorf("comp255 mapping wrong")
	}
}

func TestGhosttyThemeFromConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	gd := filepath.Join(dir, "ghostty")
	_ = os.MkdirAll(filepath.Join(gd, "themes"), 0o755)
	// a named theme file (base), then config keys that override one of them
	_ = os.WriteFile(filepath.Join(gd, "themes", "MyTheme"), []byte(
		"palette = 0=#000000\nbackground = #101010\nforeground = #cccccc\ncursor-color = #ff00ff\n"), 0o644)
	_ = os.WriteFile(filepath.Join(gd, "config"), []byte(
		"# comment\ntheme = MyTheme\nfont-family = Hack\nfont-size = 15\npalette = 1=#abcdef\nforeground = #d4d2cc\n"), 0o644)

	th, ok := ghosttyTheme()
	if !ok {
		t.Fatal("ghosttyTheme returned !ok")
	}
	if th.Source != "ghostty" {
		t.Errorf("source = %q", th.Source)
	}
	if th.Background != "#101010" { // from theme
		t.Errorf("background = %q want #101010", th.Background)
	}
	if th.Foreground != "#d4d2cc" { // config overrides theme
		t.Errorf("foreground = %q want #d4d2cc (config override)", th.Foreground)
	}
	if th.Palette[0] != "#000000" || th.Palette[1] != "#abcdef" {
		t.Errorf("palette = %v", th.Palette[:2])
	}
	if th.FontFamily != "Hack" || th.FontSize != 15 {
		t.Errorf("font = %q,%v", th.FontFamily, th.FontSize)
	}
}

// Smoke test: Appearance() always returns a usable theme. Logs the real machine's
// resolved theme for eyeballing (no assertion on the actual values).
func TestAppearanceSmoke(t *testing.T) {
	th := Appearance()
	if th.Source == "" || th.Background == "" || th.Palette[0] == "" {
		t.Errorf("Appearance returned an incomplete theme: %+v", th)
	}
	t.Logf("resolved theme: source=%s bg=%s fg=%s cursor=%s font=%q/%v",
		th.Source, th.Background, th.Foreground, th.Cursor, th.FontFamily, th.FontSize)
}
