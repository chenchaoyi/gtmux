package radar

import (
	"errors"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/agentclient"
	"github.com/chenchaoyi/gtmux/internal/dsh"
)

func TestTerminalTitleUsesExactSavedSessionAndPaneFallback(t *testing.T) {
	a, _ := terminalFixture(t)
	old := terminalHistory
	defer func() { terminalHistory = old }()
	saved := dsh.Session{Header: dsh.Header{ID: a.SessionID, Cwd: a.Cwd}, HomeID: a.HomeID, Title: "Saved\n conversation"}
	terminalHistory = func(string) (dsh.Session, error) { return saved, nil }
	if got := terminalTask(a, "✦ 🐋 Terminal title", ""); got != "Saved conversation" {
		t.Fatal(got)
	}
	for _, mismatch := range []string{"id", "home", "cwd", "child", "missing-title", "missing-log"} {
		terminalHistory = func(string) (dsh.Session, error) {
			s := saved
			switch mismatch {
			case "id":
				s.ID = "other"
			case "home":
				s.HomeID = "other"
			case "cwd":
				s.Cwd = "/other"
			case "child":
				s.Origin = "subagent"
			case "missing-title":
				s.Title = ""
			case "missing-log":
				return s, errors.New("unavailable")
			}
			return s, nil
		}
		if got := terminalTask(a, "✦ 🐋 Terminal title", ""); got != "Terminal title" {
			t.Fatalf("%s: %q", mismatch, got)
		}
	}
	a.HomeID = "foreign"
	if got := terminalTask(a, "✦ 🐋 Other home", "existing"); got != "existing" {
		t.Fatal("foreign home borrowed title", got)
	}
}

func TestGatherVerifiedTerminalRetainsTitleWithoutChangingPhase(t *testing.T) {
	a, now := terminalFixture(t)
	oldAttachments, oldHistory := terminalAttachments, terminalHistory
	defer func() { terminalAttachments, terminalHistory = oldAttachments, oldHistory }()
	terminalAttachments = func() map[string]agentclient.Attachment { return map[string]agentclient.Attachment{a.Pane: a} }
	terminalHistory = func(string) (dsh.Session, error) { return dsh.Session{}, errors.New("no saved title yet") }
	for _, title := range []string{"✦ 🐋 Terminal title", "◐ 🐋 Terminal title", "🐋️ Terminal title"} {
		lines := []string{paneLine(a.Pane, "generic-session", "0", "0", title, "node", now.Unix(), a.PID, a.Cwd)}
		withFixture(t, lines, func() {
			rows := GatherAgents()
			if len(rows) != 1 || rows[0].Task != "Terminal title" || rows[0].Status != "running" || rows[0].Agent != "DeepSeek Harness" {
				t.Fatalf("lost title or invented state: %+v", rows)
			}
		})
	}
	for _, title := range []string{"✦ 🐋 DeepSeek", "dsh", "🐋️", ""} {
		if got := terminalTask(a, title, ""); got != "" {
			t.Fatalf("placeholder %q: %q", title, got)
		}
	}
}
