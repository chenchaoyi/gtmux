package terminal

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Theme is the host terminal's resolved appearance, served to the phone/browser
// so the pane mirror renders like the user's real terminal. Colors are #rrggbb.
// (The radar status-language colors are semantic and are NEVER themed from this.)
type Theme struct {
	Source     string     `json:"source"` // "ghostty" | "iterm2" | "default"
	Background string     `json:"background"`
	Foreground string     `json:"foreground"`
	Cursor     string     `json:"cursor"`
	Palette    [16]string `json:"palette"`
	FontFamily string     `json:"fontFamily"`
	FontSize   float64    `json:"fontSize"`
}

// Appearance resolves the active host terminal's appearance, falling back to a
// neutral dark default for an unknown terminal or an unreadable config. It reuses
// the same detection as Active() (DetectedName), so adding a terminal later is one
// reader here — no change to the control Terminal interface.
func Appearance() Theme {
	switch DetectedName() { // lowercase registry keys (detect.go), not display names
	case "ghostty":
		if t, ok := ghosttyTheme(); ok {
			return t
		}
	case "iterm2":
		if t, ok := iterm2Theme(); ok {
			return t
		}
	}
	return defaultTheme()
}

// defaultTheme is a clean dark fallback (Tango-ish palette).
func defaultTheme() Theme {
	return Theme{
		Source:     "default",
		Background: "#1a1a1a",
		Foreground: "#d6d6da",
		Cursor:     "#d6d6da",
		FontFamily: "",
		FontSize:   0,
		Palette: [16]string{
			"#1a1a1a", "#cc0000", "#4e9a06", "#c4a000", "#3465a4", "#75507b", "#06989a", "#d3d7cf",
			"#555753", "#ef2929", "#8ae234", "#fce94f", "#729fcf", "#ad7fa8", "#34e2e2", "#eeeeec",
		},
	}
}

// --- Ghostty ----------------------------------------------------------------

func ghosttyConfigPaths() []string {
	home, _ := os.UserHomeDir()
	cfg := os.Getenv("XDG_CONFIG_HOME")
	if cfg == "" {
		cfg = filepath.Join(home, ".config")
	}
	return []string{
		filepath.Join(cfg, "ghostty", "config"),
		filepath.Join(home, "Library", "Application Support", "com.mitchellh.ghostty", "config"),
	}
}

// ghosttyThemeDirs are where a named `theme = NAME` file may live.
func ghosttyThemeDirs() []string {
	home, _ := os.UserHomeDir()
	cfg := os.Getenv("XDG_CONFIG_HOME")
	if cfg == "" {
		cfg = filepath.Join(home, ".config")
	}
	return []string{
		filepath.Join(cfg, "ghostty", "themes"),
		"/Applications/Ghostty.app/Contents/Resources/ghostty/themes",
		"/opt/homebrew/share/ghostty/themes",
		"/usr/local/share/ghostty/themes",
	}
}

func ghosttyTheme() (Theme, bool) {
	var text string
	for _, p := range ghosttyConfigPaths() {
		if b, err := os.ReadFile(p); err == nil {
			text = string(b)
			break
		}
	}
	if text == "" {
		return Theme{}, false
	}
	pairs := parseGhosttyPairs(text)

	t := defaultTheme()
	t.Source = "ghostty"
	// Named theme loads FIRST (as a base), then explicit user keys override it.
	if name := ghosttyDarkTheme(lastValue(pairs, "theme")); name != "" {
		if base, ok := readGhosttyThemeFile(name); ok {
			for _, kv := range base {
				applyGhosttyPair(&t, kv[0], kv[1])
			}
		}
	}
	for _, kv := range pairs {
		applyGhosttyPair(&t, kv[0], kv[1])
	}
	return t, true
}

func parseGhosttyPairs(text string) [][2]string {
	var out [][2]string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		out = append(out, [2]string{strings.TrimSpace(k), strings.TrimSpace(v)})
	}
	return out
}

func applyGhosttyPair(t *Theme, k, v string) {
	switch k {
	case "background":
		t.Background = normHex(v)
	case "foreground":
		t.Foreground = normHex(v)
	case "cursor-color":
		t.Cursor = normHex(v)
	case "font-family":
		if t.FontFamily == "" { // first wins; later entries are the fallback chain
			t.FontFamily = strings.Trim(v, `"'`)
		}
	case "font-size":
		if f, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
			t.FontSize = f
		}
	case "palette":
		idx, hex, ok := strings.Cut(v, "=")
		if !ok {
			return
		}
		if n, err := strconv.Atoi(strings.TrimSpace(idx)); err == nil && n >= 0 && n < 16 {
			t.Palette[n] = normHex(hex)
		}
	}
}

func readGhosttyThemeFile(name string) ([][2]string, bool) {
	name = strings.Trim(name, `"'`)
	for _, dir := range ghosttyThemeDirs() {
		if b, err := os.ReadFile(filepath.Join(dir, name)); err == nil {
			return parseGhosttyPairs(string(b)), true
		}
	}
	return nil, false
}

// ghosttyDarkTheme picks the dark side of a `dark:NAME,light:NAME` value (v1 has
// no light/dark split); a plain name passes through.
func ghosttyDarkTheme(v string) string {
	v = strings.TrimSpace(v)
	if v == "" || !strings.Contains(v, ":") {
		return v
	}
	for _, part := range strings.Split(v, ",") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "dark:") {
			return strings.TrimSpace(strings.TrimPrefix(part, "dark:"))
		}
	}
	return v
}

func lastValue(pairs [][2]string, key string) string {
	out := ""
	for _, kv := range pairs {
		if kv[0] == key {
			out = kv[1]
		}
	}
	return out
}

// normHex lowercases a hex color and ensures a leading '#'. Ghostty accepts both
// "17171a" and "#17171a".
func normHex(s string) string {
	s = strings.TrimSpace(strings.Trim(s, `"'`))
	if s == "" {
		return s
	}
	if !strings.HasPrefix(s, "#") {
		s = "#" + s
	}
	return strings.ToLower(s)
}
