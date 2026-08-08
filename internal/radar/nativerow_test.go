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

// A native row must carry the same icon hint its tmux twin gets. The mobile avatar fetches
// /api/icon only when the row's `icon` is non-empty, so an empty hint is not a cosmetic
// nicety — it is the difference between the agent's real mark and a neutral monogram.
//
// Codex is the case that broke (seen on a phone, 2026-08-09: two "Cx" boxes under
// ELSEWHERE while Claude rows two lines up showed their mark): it ships no desktop app, so
// its profile carries no icon PATH and it depends on IconFor's `builtin:<key>` fallback to
// the committed PNG. nativePanes read `p.Icon` raw and skipped that fallback entirely.
func TestNativePanesCarryIcon(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := time.Now().Unix()
	if err := native.Save(native.Record{
		SessionID: "codex-native-1", Agent: "codex", State: "working", UpdatedAt: now - 30,
	}); err != nil {
		t.Fatal(err)
	}
	panes := nativePanes(nil, LoadProfiles(), now)
	if len(panes) != 1 {
		t.Fatalf("nativePanes = %d rows, want 1", len(panes))
	}
	if panes[0].icon == "" {
		t.Fatal("native row has no icon hint — the phone will render the neutral monogram")
	}
	// The same agent through the tmux path resolves the identical hint; the two surfaces
	// must not disagree about who an agent is.
	if want := IconFor(panes[0].Agent, LoadProfiles()); panes[0].icon != want {
		t.Errorf("native icon = %q, want %q (what the tmux row gets)", panes[0].icon, want)
	}
}
