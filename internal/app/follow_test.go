package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/server"
	"github.com/chenchaoyi/gtmux/internal/sessionpolicy"
)

func TestFollowCLIUsesOneConversationAndRecordsItsReceipt(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	home := t.TempDir()
	t.Setenv("CODEX_HOME", home)
	root := filepath.Join(home, "sessions", "2026", "10", "09")
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	b := []byte(`{"type":"session_meta","payload":{"id":"cli-desk","originator":"Codex Desktop"}}` + "\n")
	if err := os.WriteFile(filepath.Join(root, "rollout-2026-10-09T00-00-00-cli-desk.jsonl"), b, 0600); err != nil {
		t.Fatal(err)
	}
	if cmdFollow([]string{"cli-desk", "--hq", "on", "--json"}) != 0 {
		t.Fatal("save failed")
	}
	v := sessionpolicy.Get("cli-desk")
	if !v.HQ || v.Notify || v.Knowledge || v.Revision != 1 || sessionpolicy.Get("unrelated").HQ {
		t.Fatalf("permissions=%+v", v)
	}
	if cmdFollow([]string{"cli-desk", "--hq", "off", "--revision", "0"}) != 1 || !sessionpolicy.Get("cli-desk").HQ {
		t.Fatal("stale CLI overwrote policy")
	}
	records, _ := events.ReadSince(0)
	found := false
	for _, r := range records {
		if r.Event == "gtmux:audit:session-follow" && r.AgentSession == "cli-desk" && r.Client == "chatgpt_desktop" && r.Outcome == "saved" {
			found = true
		}
	}
	if !found {
		t.Fatal("missing permission receipt")
	}
	if cmdFollow([]string{"cli-desk", "--hq", "off"}) != 0 {
		t.Fatal("stop failed")
	}
	if got := sessionpolicy.Get("cli-desk"); got.HQ || got.Notify || got.Knowledge {
		t.Fatalf("stop=%+v", got)
	}
	if _, err := serveSessionFollow("unknown", nil); err != sessionpolicy.ErrUnverified {
		t.Fatal("unknown identity accepted")
	}
}

func TestDesktopPushDoesNotOfferUnusablePaneActions(t *testing.T) {
	for _, lang := range []string{"en", "zh"} {
		t.Setenv("GTMUX_LANG", lang)
		a := server.Alert{Client: "chatgpt_desktop", SessionID: "id", Agent: "Codex", Task: "Crash investigation", Kind: "waiting"}
		title, body, n := pushCopy(a, func(string) (string, bool) { t.Fatal("desktop notification read pane"); return "", false })
		if title != a.Task || body == "" || n != 0 {
			t.Fatalf("copy=%q/%q/%d", title, body, n)
		}
	}
}
