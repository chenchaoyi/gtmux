package terminal

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/state"
)

// Theme is the host terminal's resolved appearance, served to the phone/browser
// so the pane mirror renders like the user's real terminal. Colors are #rrggbb.
// (The radar status-language colors are semantic and are NEVER themed from this.)
type Theme struct {
	Source     string     `json:"source"` // "ghostty" | "cmux" | "iterm2" | "default"
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
	case "cmux":
		if t, ok := ghosttyTheme(); ok {
			t.Source = "cmux"
			return t
		}
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

// ghosttyConfigPaths are the files Ghostty loads, in its order: config.ghostty (the
// name since 1.2.3) then the legacy config, in the XDG directory and then the macOS one.
// Every one that exists is loaded and a later file overrides an earlier one
// (ghostty.org/docs/config). This read only the first file it found, and never
// config.ghostty, so a config under the current name read as no config at all, and a
// macOS file never overrode an XDG one (%12, 2026-10-06).
func ghosttyConfigPaths() []string {
	home := state.Home()
	cfg := os.Getenv("XDG_CONFIG_HOME")
	if cfg == "" {
		cfg = filepath.Join(home, ".config")
	}
	mac := filepath.Join(home, "Library", "Application Support", "com.mitchellh.ghostty")
	return []string{
		filepath.Join(cfg, "ghostty", "config.ghostty"),
		filepath.Join(cfg, "ghostty", "config"),
		filepath.Join(mac, "config.ghostty"),
		filepath.Join(mac, "config"),
	}
}

// ghosttyUserPairs reads every config file Ghostty loads, in its order, with each
// `config-file` include expanded at the END of the file that names it (relative to that
// file; a leading ? makes it optional, and a quoted value is a literal name), as Ghostty
// does. A file may be included again once it has finished loading; only one already on
// the current include chain is a cycle. found is false when no config file exists.
func ghosttyUserPairs() (pairs [][2]string, found bool) {
	onChain := map[string]bool{}
	var load func(path string, depth int) bool
	load = func(path string, depth int) bool {
		if depth > 8 || onChain[path] { // an include cycle, or a chain no real config has
			return false
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return false
		}
		onChain[path] = true
		defer delete(onChain, path)
		var includes []string
		for _, kv := range parseGhosttyPairs(string(b)) {
			if kv[0] == "config-file" {
				includes = append(includes, kv[1])
				continue
			}
			pairs = append(pairs, kv)
		}
		for _, inc := range includes {
			inc = includePath(inc)
			if inc == "" {
				continue
			}
			if !filepath.IsAbs(inc) {
				inc = filepath.Join(filepath.Dir(path), inc)
			}
			load(inc, depth+1)
		}
		return true
	}
	for _, p := range ghosttyConfigPaths() {
		if load(p, 0) {
			found = true
		}
	}
	return pairs, found
}

// ghosttyThemeDirs are where a named `theme = NAME` file may live.
func ghosttyThemeDirs() []string {
	home := state.Home()
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

// includePath is a config-file value as a path: an unquoted leading ? only marks the
// include optional (a missing file is skipped either way here), while a quoted value is
// taken literally, so "?name" names a file that starts with "?" (ghostty.org/docs/config).
func includePath(v string) string {
	v = strings.TrimSpace(v)
	if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
		return v[1 : len(v)-1]
	}
	return strings.TrimPrefix(v, "?")
}

func ghosttyTheme() (Theme, bool) {
	pairs, found := ghosttyUserPairs()
	if !found {
		return Theme{}, false
	}

	t := defaultTheme()
	t.Source = "ghostty"
	// Named theme loads FIRST (as a base), then explicit user keys override it.
	if name := ghosttyDarkTheme(lastValue(pairs, "theme")); name != "" {
		if base, ok := readGhosttyThemeFile(name); ok {
			applyGhosttyLayer(&t, base)
		}
	}
	applyGhosttyLayer(&t, pairs)
	return t, true
}

// applyGhosttyLayer applies one layer (the theme, or the user's config files) over t.
// font-family is a list: within a layer the first entry is the font and the rest are
// its fallbacks, and an empty value clears the list. Across layers the user's font
// replaces the theme's. It used to be first-wins across BOTH, so a theme that names a
// font beat the user's own font-family (%12, 2026-10-06).
func applyGhosttyLayer(t *Theme, pairs [][2]string) {
	fontSet := false
	for _, kv := range pairs {
		if kv[0] != "font-family" {
			applyGhosttyPair(t, kv[0], kv[1])
			continue
		}
		v := strings.Trim(strings.TrimSpace(kv[1]), `"'`)
		if v == "" { // clears the list: no font until a later entry names one
			t.FontFamily = ""
			fontSet = false
			continue
		}
		if !fontSet {
			t.FontFamily = v
			fontSet = true
		}
	}
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

// normHex lowercases a hex color, ensures a leading '#', and widens the short form:
// Theme promises #rrggbb, and "#abc" went out as is. Ghostty accepts both "17171a" and
// "#17171a".
func normHex(s string) string {
	s = strings.TrimSpace(strings.Trim(s, `"'`))
	if s == "" {
		return s
	}
	s = strings.ToLower(strings.TrimPrefix(s, "#"))
	if len(s) == 3 {
		s = string([]byte{s[0], s[0], s[1], s[1], s[2], s[2]})
	}
	return "#" + s
}
