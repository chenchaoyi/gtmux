package app

import (
	"strings"
	"testing"
)

// The help text is bilingual and complete: both the en and zh usage blocks are
// non-empty, differ, and document every top-level command.
func TestUsageTextBilingualComplete(t *testing.T) {
	if strings.TrimSpace(usageEN) == "" || strings.TrimSpace(usageZH) == "" {
		t.Fatalf("usage text must be non-empty in both langs")
	}
	if usageEN == usageZH {
		t.Fatalf("en and zh usage blocks must differ")
	}

	for _, cmd := range []string{
		"overview", "agents", "restore", "focus", "new",
		"serve", "tunnel", "doctor", "update",
		"install-hooks", "uninstall-hooks", "uninstall-app", "hook",
	} {
		if !strings.Contains(usageEN, cmd) {
			t.Errorf("usageEN missing command %q", cmd)
		}
		if !strings.Contains(usageZH, cmd) {
			t.Errorf("usageZH missing command %q", cmd)
		}
	}
}
