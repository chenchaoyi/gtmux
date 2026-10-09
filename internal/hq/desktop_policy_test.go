package hq

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/sessionpolicy"
	"github.com/chenchaoyi/gtmux/internal/state"
)

func TestDesktopDebtAndHQPullUseTheSameConsentWindow(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	home := t.TempDir()
	t.Setenv("CODEX_HOME", home)
	dir := filepath.Join(home, "sessions", "2026", "10", "09")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "rollout-2026-10-09T00-00-00-hq-desktop.jsonl"), []byte(`{"type":"session_meta","payload":{"id":"hq-desktop","originator":"codex_work_desktop"}}`+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(state.HQHome(), 0700); err != nil {
		t.Fatal(err)
	}
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Chdir(state.HQHome()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(old); err != nil {
			t.Error(err)
		}
	})
	t.Setenv("TMUX_PANE", "%hq-test")
	now := time.Now().Unix()
	events.Append(events.Record{Ts: now - 50, Event: "UserPromptSubmit", Agent: "Codex", AgentSession: "hq-desktop", Client: "chatgpt_desktop", Summary: "not followed"})
	events.Append(events.Record{Ts: now, Event: "Stop", Agent: "Claude Code", AgentSession: "other", Summary: "other agent"})
	tally := unreadScan(0, "%hq-test")
	delta, _ := events.ReadSince(0)
	shown, _ := pullView(delta, true)
	if tally.N != 1 || len(shown) != 1 || shown[0].Agent != "Claude Code" {
		t.Fatalf("default debt=%+v pull=%+v", tally, shown)
	}
	p, err := sessionpolicy.Save("hq-desktop", sessionpolicy.Settings{HQ: true})
	if err != nil {
		t.Fatal(err)
	}
	events.Append(events.Record{Ts: p.FollowSince + 2, Event: "Stop", Agent: "Codex", AgentSession: "hq-desktop", Client: "chatgpt_desktop", Summary: "enrolled"})
	tally = unreadScan(0, "%hq-test")
	delta, _ = events.ReadSince(0)
	shown, _ = pullView(delta, true)
	if tally.N != 2 || len(shown) != 2 {
		t.Fatalf("enrolled debt=%+v pull=%+v", tally, shown)
	}
	p.HQ = false
	if _, err = sessionpolicy.Save("hq-desktop", p); err != nil {
		t.Fatal(err)
	}
	if got := unreadScan(0, "%hq-test"); got.N != 1 {
		t.Fatal("stop left desktop debt")
	}
	// Diagnostics are not rewritten or deleted by enrollment/revocation.
	if raw, hidden := pullView(delta, false); len(raw) != 3 || hidden != 0 {
		t.Fatal("raw history changed")
	}
}
