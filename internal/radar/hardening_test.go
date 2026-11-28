package radar

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/resume"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

func TestCHH005_IdleSinceFastPathAndCachedAgentsSnapshot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// 1) Marked non-Codex pane returns finished marker mtime immediately without opening bound transcript.
	wantSince := int64(1_700_001_234)
	stampIdleTime(state.FinishedPath("%42"), wantSince)
	projDir := filepath.Join(home, ".claude", "projects", "demo")
	if err := os.MkdirAll(projDir, 0o700); err != nil {
		t.Fatal(err)
	}
	unreadableLog := filepath.Join(projDir, "sid-fast.jsonl")
	if err := os.WriteFile(unreadableLog, []byte(`{"timestamp":"2026-11-28T08:00:00Z"}`+"\n"), 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(unreadableLog, 0o600) })
	_ = resume.Save("work:0.0", resume.Record{Agent: "claude", SessionID: "sid-fast", Cwd: home})

	if got := idleSince("%42", "work:0.0", home, "Claude Code", 1_700_000_000, nil); got != wantSince {
		t.Fatalf("idleSince marked non-Codex = %d, want %d", got, wantSince)
	}

	// 2) CachedAgents reuses the recent snapshot within its TTL instead of rescanning.
	paths, _ := collectionFixture(t)
	withFixture(t, nil, func() {
		initial := RefreshAgentsSnapshot()
		if len(initial) != 2 {
			t.Fatalf("initial RefreshAgentsSnapshot rows = %d, want 2", len(initial))
		}
		before := transcript.CodexCandidateDiscoveryCount()

		f, err := os.OpenFile(paths["collect-one"], os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = fmt.Fprintf(f, "{\"type\":\"event_msg\",\"timestamp\":%q,\"payload\":{\"type\":\"task_started\"}}\n", time.Now().Format(time.RFC3339Nano))
		_ = f.Close()

		cached := CachedAgents()
		if len(cached) != 2 || cached[0].Status != "idle" {
			t.Fatalf("CachedAgents did not reuse fresh snapshot: %+v", cached)
		}
		if got := transcript.CodexCandidateDiscoveryCount() - before; got != 0 {
			t.Fatalf("CachedAgents triggered %d Codex discoveries within TTL, want 0", got)
		}
	})
}
