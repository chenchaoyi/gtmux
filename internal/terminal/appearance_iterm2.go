package terminal

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"howett.net/plist"
)

// iterm2Theme reads iTerm2's preferences plist and resolves the DEFAULT profile's
// appearance (Ansi 0–15 + background/foreground/cursor colors, Normal Font). Pure
// Go (howett.net/plist), so the CLI stays cgo-free.
func iterm2Theme() (Theme, bool) {
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, "Library", "Preferences", "com.googlecode.iterm2.plist")
	b, err := os.ReadFile(path)
	if err != nil {
		return Theme{}, false
	}
	var prefs struct {
		DefaultGuid string                   `plist:"Default Bookmark Guid"`
		Bookmarks   []map[string]interface{} `plist:"New Bookmarks"`
	}
	if _, err := plist.Unmarshal(b, &prefs); err != nil || len(prefs.Bookmarks) == 0 {
		return Theme{}, false
	}

	prof := prefs.Bookmarks[0]
	if prefs.DefaultGuid != "" {
		for _, p := range prefs.Bookmarks {
			if g, _ := p["Guid"].(string); g == prefs.DefaultGuid {
				prof = p
				break
			}
		}
	}

	t := defaultTheme()
	t.Source = "iterm2"
	if c := itermHex(prof, "Background Color"); c != "" {
		t.Background = c
	}
	if c := itermHex(prof, "Foreground Color"); c != "" {
		t.Foreground = c
	}
	if c := itermHex(prof, "Cursor Color"); c != "" {
		t.Cursor = c
	}
	for i := 0; i < 16; i++ {
		if c := itermHex(prof, fmt.Sprintf("Ansi %d Color", i)); c != "" {
			t.Palette[i] = c
		}
	}
	if fam, size := parseITermFont(prof["Normal Font"]); fam != "" {
		t.FontFamily = fam
		t.FontSize = size
	}
	return t, true
}

func itermHex(prof map[string]interface{}, key string) string {
	c, ok := prof[key].(map[string]interface{})
	if !ok {
		return ""
	}
	return fmt.Sprintf("#%02x%02x%02x",
		comp255(c["Red Component"]), comp255(c["Green Component"]), comp255(c["Blue Component"]))
}

func comp255(v interface{}) int {
	f, _ := v.(float64)
	n := int(f*255 + 0.5)
	if n < 0 {
		n = 0
	}
	if n > 255 {
		n = 255
	}
	return n
}

// parseITermFont turns "JetBrainsMono-Regular 13" into ("JetBrainsMono", 13). The
// style suffix is stripped; the client maps the family to a bundled font.
func parseITermFont(v interface{}) (string, float64) {
	s, _ := v.(string)
	s = strings.TrimSpace(s)
	if s == "" {
		return "", 0
	}
	name, size := s, 0.0
	if i := strings.LastIndex(s, " "); i > 0 {
		if f, err := strconv.ParseFloat(s[i+1:], 64); err == nil {
			name, size = strings.TrimSpace(s[:i]), f
		}
	}
	for _, suf := range []string{"-Regular", "-Bold", "-Italic", "-Medium", "-Light", "-Retina"} {
		name = strings.TrimSuffix(name, suf)
	}
	return name, size
}
