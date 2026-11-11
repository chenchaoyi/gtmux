package dispatchbridge

import (
	"strings"
	"testing"
)

const currentFolderGate = "Folder access\n/synthetic/worktree\nTrust this folder? Codex can read, edit, and run files here.\n› 1. Trust and continue\n2. Back to Agent Command Center\nenter continue · esc back"

func TestWTR004VisibleTrustNamesBlockerBeforePostStartTransport(t *testing.T) {
	for _, cmd := range []string{SelfCommand, "bash", "codex"} {
		calls := 0
		got := readyBlockerSnapshot(cmd, currentFolderGate, "codex", func() bool { calls++; return false })
		if !strings.Contains(got, "startup") || calls != 0 {
			t.Fatal("native readiness masked prior consent", cmd, got, calls)
		}
	}
}
func TestWTR004NormalTransportStillRequiredAndHistoricalTrustIgnored(t *testing.T) {
	calls := 0
	got := readyBlockerSnapshot("codex", currentFolderGate+"\n› Ask Codex to do anything", "codex", func() bool { calls++; return false })
	if !strings.Contains(got, "native terminal attachment") || calls != 1 {
		t.Fatal(got, calls)
	}
}
