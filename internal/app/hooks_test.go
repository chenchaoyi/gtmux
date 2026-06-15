package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readSettings(t *testing.T, path string) map[string]any {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal %s: %v", path, err)
	}
	return m
}

// countHooks returns how many gtmux vs. foreign hook commands an event holds.
func countHooks(t *testing.T, path, event string) (gtmux, others int) {
	t.Helper()
	m := readSettings(t, path)
	hooks, _ := m["hooks"].(map[string]any)
	for _, raw := range asArray(hooks[event]) {
		entry, _ := raw.(map[string]any)
		for _, h := range asArray(entry["hooks"]) {
			hm, _ := h.(map[string]any)
			if isGtmuxHookCommand(asString(hm["command"])) {
				gtmux++
			} else {
				others++
			}
		}
	}
	return
}

func TestUpdateSettingsInstallIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	bin := "/opt/gtmux/gtmux"

	// Install twice — must converge to exactly one gtmux entry per event.
	for i := 0; i < 2; i++ {
		if err := updateSettings(path, bin, true); err != nil {
			t.Fatalf("install %d: %v", i, err)
		}
	}
	for _, ev := range hookEvents {
		if g, _ := countHooks(t, path, ev); g != 1 {
			t.Errorf("event %s: %d gtmux hooks, want exactly 1", ev, g)
		}
	}
	b, _ := os.ReadFile(path)
	if !strings.Contains(string(b), bin+" hook") {
		t.Errorf("settings should use the absolute command %q", bin+" hook")
	}
	if !strings.Contains(string(b), `"async": true`) {
		t.Error("hook entries should be async")
	}
}

func TestUpdateSettingsPreservesForeignHooksAndUninstalls(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	seed := map[string]any{
		"model": "opus", // unrelated top-level key — must survive
		"hooks": map[string]any{
			"Stop": []any{map[string]any{
				"matcher": "",
				"hooks":   []any{map[string]any{"type": "command", "command": "/usr/bin/say done"}},
			}},
		},
	}
	b, _ := json.MarshalIndent(seed, "", "  ")
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}

	bin := "/opt/gtmux/gtmux"
	if err := updateSettings(path, bin, true); err != nil {
		t.Fatal(err)
	}

	// Foreign Stop hook preserved alongside the new gtmux one.
	if g, o := countHooks(t, path, "Stop"); g != 1 || o != 1 {
		t.Errorf("Stop after install: gtmux=%d others=%d, want 1/1", g, o)
	}
	if m := readSettings(t, path); m["model"] != "opus" {
		t.Errorf("unrelated key 'model' = %v, want preserved 'opus'", m["model"])
	}
	if _, err := os.Stat(path + ".gtmux.bak"); err != nil {
		t.Error("install should back up the existing settings.json")
	}

	// Uninstall: gtmux gone, foreign kept, gtmux-only events removed entirely.
	if err := updateSettings(path, "", false); err != nil {
		t.Fatal(err)
	}
	if g, o := countHooks(t, path, "Stop"); g != 0 || o != 1 {
		t.Errorf("Stop after uninstall: gtmux=%d others=%d, want 0/1", g, o)
	}
	hooks, _ := readSettings(t, path)["hooks"].(map[string]any)
	if _, ok := hooks["Notification"]; ok {
		t.Error("Notification (gtmux-only) should be removed after uninstall")
	}
}

func TestIsGtmuxHookCommand(t *testing.T) {
	for _, s := range []string{"gtmux hook", "/usr/local/bin/gtmux hook", "/Users/x/go/bin/gtmux hook"} {
		if !isGtmuxHookCommand(s) {
			t.Errorf("isGtmuxHookCommand(%q) = false, want true", s)
		}
	}
	for _, s := range []string{"/usr/bin/say done", "gtmuxx hook", "/opt/notgtmux hook", "gtmux agents", ""} {
		if isGtmuxHookCommand(s) {
			t.Errorf("isGtmuxHookCommand(%q) = true, want false", s)
		}
	}
}
