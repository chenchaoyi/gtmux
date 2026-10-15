package terminal

import (
	"github.com/chenchaoyi/gtmux/internal/hostplatform"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/ghostty"
)

// The Ghostty driver must satisfy the Terminal interface (compile-time check).
var _ Terminal = ghostty.Driver{}
var _ Terminal = cmux{}

func TestCmuxDriverRegistered(t *testing.T) {
	if !hostplatform.Current().DesktopFocus {
		if Active().Name() != "tmux" || ForSession("session").Name() != "tmux" {
			t.Fatal("headless host selected desktop driver")
		}
		return
	}
	t.Setenv("GTMUX_TERMINAL", "cmux")
	if !HasDriver("cmux") || Active().Name() != "cmux" || ForSession("session").Name() != "cmux" {
		t.Fatal("cmux should resolve to its own driver")
	}
}

// An unknown host retains Ghostty as the compatibility fallback.
func TestActiveIsGhostty(t *testing.T) {
	t.Setenv("GTMUX_TERMINAL", "unknown-terminal")
	want := "Ghostty"
	if !hostplatform.Current().DesktopFocus {
		want = "tmux"
	}
	if got := Active().Name(); got != want {
		t.Errorf("Active().Name() = %q, want Ghostty", got)
	}
}
