package terminal

import (
	"testing"

	"github.com/chenchaoyi/gtmux/internal/ghostty"
)

// The Ghostty driver must satisfy the Terminal interface (compile-time check).
var _ Terminal = ghostty.Driver{}

// Active() resolves to Ghostty for now (host detection lands in a later slice);
// this is the guard that the refactor stayed behavior-preserving.
func TestActiveIsGhostty(t *testing.T) {
	if got := Active().Name(); got != "Ghostty" {
		t.Errorf("Active().Name() = %q, want Ghostty", got)
	}
}
