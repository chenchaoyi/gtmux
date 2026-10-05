package terminal

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormHex(t *testing.T) {
	for in, want := range map[string]string{
		"17171a": "#17171a", "#17171A": "#17171a", `"#D4D2CC"`: "#d4d2cc", "  #abc  ": "#aabbcc", "ABC": "#aabbcc", "": "",
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
	// XDG_CONFIG_HOME alone does not isolate this: ghosttyConfigPaths() also offers
	// $HOME/Library/Application Support/com.mitchellh.ghostty/config, so on a Mac whose
	// owner keeps their Ghostty config there this test would read theirs.
	t.Setenv("HOME", t.TempDir())
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

func TestCmuxReusesGhosttyTheme(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("GTMUX_TERMINAL", "cmux")
	if err := os.MkdirAll(filepath.Join(dir, "ghostty"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ghostty", "config"), []byte("background = #123456\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := Appearance()
	if got.Source != "cmux" || got.Background != "#123456" {
		t.Fatalf("cmux appearance = %+v", got)
	}
}

// Smoke test: Appearance() always returns a usable theme — here with no terminal
// config anywhere, which is the fallback path. It used to run against the operator's
// real config and log what it found; that made the result differ per machine while
// asserting nothing about it. The configured path is pinned by the test above.
func TestAppearanceSmoke(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	th := Appearance()
	if th.Source == "" || th.Background == "" || th.Palette[0] == "" {
		t.Errorf("Appearance returned an incomplete theme: %+v", th)
	}
	t.Logf("resolved theme: source=%s bg=%s fg=%s cursor=%s font=%q/%v",
		th.Source, th.Background, th.Foreground, th.Cursor, th.FontFamily, th.FontSize)
}

// ghosttyHome is a private HOME + XDG_CONFIG_HOME, and writes files under them.
func ghosttyHome(t *testing.T) (xdg, mac string, write func(path, body string)) {
	t.Helper()
	home := t.TempDir()
	xdg = filepath.Join(t.TempDir(), "ghostty")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Dir(xdg))
	mac = filepath.Join(home, "Library", "Application Support", "com.mitchellh.ghostty")
	write = func(path, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return xdg, mac, write
}

// Ghostty's config is config.ghostty (since 1.2.3) or the legacy config, in the XDG
// directory and then the macOS one; every file that exists loads, later ones override,
// and config-file includes load at the end of their file (ghostty.org/docs/config).
// The reader took the first legacy-named file only (%12, 2026-10-06).
func TestGhosttyLoadsEveryConfigFileInOrder(t *testing.T) {
	t.Run("config.ghostty alone is read", func(t *testing.T) {
		xdg, _, write := ghosttyHome(t)
		write(filepath.Join(xdg, "config.ghostty"), "background = #123456\n")
		if th, ok := ghosttyTheme(); !ok || th.Background != "#123456" {
			t.Fatalf("XDG config.ghostty: ok=%v %+v", ok, th)
		}
	})
	t.Run("the macOS file alone is read", func(t *testing.T) {
		_, mac, write := ghosttyHome(t)
		write(filepath.Join(mac, "config.ghostty"), "background = #123456\n")
		if th, ok := ghosttyTheme(); !ok || th.Background != "#123456" {
			t.Fatalf("macOS config.ghostty: ok=%v %+v", ok, th)
		}
	})
	t.Run("later files override earlier ones", func(t *testing.T) {
		xdg, mac, write := ghosttyHome(t)
		write(filepath.Join(xdg, "config.ghostty"), "background = #111111\nforeground = #aaaaaa\n")
		write(filepath.Join(xdg, "config"), "foreground = #bbbbbb\n")
		write(filepath.Join(mac, "config"), "background = #222222\n")
		th, _ := ghosttyTheme()
		if th.Background != "#222222" || th.Foreground != "#bbbbbb" {
			t.Fatalf("got background %s foreground %s, want #222222 #bbbbbb", th.Background, th.Foreground)
		}
	})
	t.Run("an include loads at the end of its file, relative to it", func(t *testing.T) {
		xdg, _, write := ghosttyHome(t)
		write(filepath.Join(xdg, "config.ghostty"), "config-file = colors.conf\nbackground = #333333\nconfig-file = ?missing.conf\n")
		write(filepath.Join(xdg, "colors.conf"), "background = #444444\ncursor-color = #555555\nconfig-file = config.ghostty\n")
		th, ok := ghosttyTheme()
		if !ok || th.Background != "#444444" || th.Cursor != "#555555" {
			t.Fatalf("include: ok=%v background %s cursor %s, want #444444 #555555", ok, th.Background, th.Cursor)
		}
	})
	t.Run("no config at all", func(t *testing.T) {
		ghosttyHome(t)
		if _, ok := ghosttyTheme(); ok {
			t.Fatal("ok with no config file")
		}
	})
}

// A theme may name a font, and the user's font-family overrides it; within a layer the
// first entry is the font and the rest are fallbacks. It was first-wins across both, so
// the theme's font beat the user's (%12, 2026-10-06).
func TestTheUsersFontOverridesTheThemes(t *testing.T) {
	xdg, _, write := ghosttyHome(t)
	write(filepath.Join(xdg, "themes", "Fonted"), "font-family = Theme Font\nbackground = #101010\n")
	write(filepath.Join(xdg, "config.ghostty"), "theme = Fonted\nfont-family = User Font\nfont-family = Fallback Font\n")
	if th, _ := ghosttyTheme(); th.FontFamily != "User Font" || th.Background != "#101010" {
		t.Fatalf("font %q background %s, want User Font #101010", th.FontFamily, th.Background)
	}
	write(filepath.Join(xdg, "config.ghostty"), "theme = Fonted\n")
	if th, _ := ghosttyTheme(); th.FontFamily != "Theme Font" {
		t.Fatalf("no user font: %q, want the theme's", th.FontFamily)
	}
	write(filepath.Join(xdg, "config.ghostty"), "font-family = First\nfont-family = \"\"\nfont-family = After Reset\n")
	if th, _ := ghosttyTheme(); th.FontFamily != "After Reset" {
		t.Fatalf("after an empty reset: %q", th.FontFamily)
	}
}

// %12's edge cases on #1413 (3a4c403a): an empty font-family must clear the font, also
// at the end of a layer and over a theme's font; a file that has finished loading may be
// included again (only the current chain is a cycle); a quoted "?name" is a literal file.
func TestGhosttyConfigEdges(t *testing.T) {
	t.Run("an empty font-family at the end clears the font", func(t *testing.T) {
		xdg, _, write := ghosttyHome(t)
		write(filepath.Join(xdg, "themes", "Fonted"), "font-family = Theme Font\n")
		write(filepath.Join(xdg, "config.ghostty"), "theme = Fonted\nfont-family = \"\"\n")
		if th, _ := ghosttyTheme(); th.FontFamily != "" {
			t.Fatalf("over a theme font: %q, want none", th.FontFamily)
		}
		write(filepath.Join(xdg, "config.ghostty"), "font-family = User Font\nfont-family = \"\"\n")
		if th, _ := ghosttyTheme(); th.FontFamily != "" {
			t.Fatalf("after the user's own font: %q, want none", th.FontFamily)
		}
	})
	t.Run("a finished include can be included again", func(t *testing.T) {
		xdg, mac, write := ghosttyHome(t)
		shared := filepath.Join(t.TempDir(), "shared.conf")
		write(shared, "background = #111111\n")
		write(filepath.Join(xdg, "config"), "config-file = "+shared+"\n")
		write(filepath.Join(mac, "config"), "background = #222222\nconfig-file = "+shared+"\n")
		if th, _ := ghosttyTheme(); th.Background != "#111111" {
			t.Fatalf("background %s, want #111111 (shared loaded at the end of the macOS file)", th.Background)
		}
	})
	t.Run("a quoted ?name is a literal file name", func(t *testing.T) {
		xdg, _, write := ghosttyHome(t)
		write(filepath.Join(xdg, "?literal"), "background = #222222\n")
		write(filepath.Join(xdg, "config.ghostty"), "background = #111111\nconfig-file = \"?literal\"\n")
		if th, _ := ghosttyTheme(); th.Background != "#222222" {
			t.Fatalf("background %s, want #222222 from the file named ?literal", th.Background)
		}
		write(filepath.Join(xdg, "config.ghostty"), "background = #111111\nconfig-file = ?absent\n")
		if th, ok := ghosttyTheme(); !ok || th.Background != "#111111" {
			t.Fatalf("optional missing include: ok=%v %s", ok, th.Background)
		}
	})
}
