package radar

import (
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/native"
)

// A native record's sensed terminal ("Warp", "Ghostty", …) must reach the radar
// row's `terminal` field — it's how the surfaces label an out-of-tmux agent's
// home ("Elsewhere · Warp"). Before this, nativePanes dropped it: every native
// row shipped with no terminal name at all (verified live with a Warp-hosted
// hook, 2026-08-08).
func TestNativePanesCarryTerminal(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := time.Now().Unix()
	if err := native.Save(native.Record{
		SessionID: "warp-native-1", Agent: "claude", State: "working",
		UpdatedAt: now - 30, Terminal: "Warp",
	}); err != nil {
		t.Fatal(err)
	}
	panes := nativePanes(nil, nil, now)
	if len(panes) != 1 {
		t.Fatalf("nativePanes = %d rows, want 1", len(panes))
	}
	if panes[0].terminal != "Warp" {
		t.Fatalf("native row terminal = %q, want Warp", panes[0].terminal)
	}
	if panes[0].source != "native" {
		t.Fatalf("native row source = %q, want native", panes[0].source)
	}
}
