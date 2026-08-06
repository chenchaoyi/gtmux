package app

import (
	"os"
	"path/filepath"
	"testing"
)

// codexHooksHasAsync must flag a gtmux entry marked async (which Codex 0.146.0 skips), and
// only a gtmux one — not a sync gtmux entry, not a foreign async hook.
func TestCodexHooksHasAsync(t *testing.T) {
	dir := t.TempDir()
	gtmux := "/x/gtmux hook --agent codex Stop" // matches isGtmuxHookCommand
	cases := []struct {
		name    string
		content string // "" => a path that does not exist
		want    bool
	}{
		{"async gtmux entry", `{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"` + gtmux + `","async":true}]}]}}`, true},
		{"sync gtmux entry (async absent)", `{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"` + gtmux + `","timeout":3}]}]}}`, false},
		{"async but foreign (not gtmux)", `{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"/x/other run","async":true}]}]}}`, false},
		{"missing file", "", false},
	}
	for _, c := range cases {
		p := filepath.Join(dir, "nope.json")
		if c.content != "" {
			p = filepath.Join(dir, c.name+".json")
			if err := os.WriteFile(p, []byte(c.content), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if got := codexHooksHasAsync(p); got != c.want {
			t.Errorf("%s: codexHooksHasAsync = %v, want %v", c.name, got, c.want)
		}
	}
}
