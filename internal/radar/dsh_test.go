package radar

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/dsh"
)

func TestDSHSavedConversationUsesExternalContractWithoutClaimingControl(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CODEX_HOME", t.TempDir())
	t.Setenv("DSH_HOME", t.TempDir())
	dir := filepath.Join(dsh.Home(), "sessions", "project", "web")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	text := `{"type":"session","version":4,"id":"web","cwd":"/project"}` + "\n" + `{"type":"user/message","time":2000,"data":{"role":"user","source":{"kind":"user"}}}` + "\n" + `{"type":"session/title","time":2000,"data":{"title":"Saved DSH conversation"}}` + "\n" + `{"type":"turn/end","time":3000,"data":{"reason":{"kind":"blocked"}}}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "session.v4.jsonl"), []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	rows := nativePanes(nil, nil, time.Now().Unix())
	if len(rows) != 1 {
		t.Fatalf("want one root history, got %d", len(rows))
	}
	p := rows[0]
	if p.Agent != "DeepSeek Harness" || p.Task != "Saved DSH conversation" || p.source != "native" || p.currentClient != "unconfirmed" || !p.canReadConversation || p.homeID != dsh.HomeID() {
		t.Fatalf("external history metadata: %+v", p)
	}
	if p.Status != "idle" || p.EndReason != "error" || p.icon == "" {
		t.Fatal("blocked turn earned a success presentation or lost official icon", p.Status, p.EndReason, p.icon)
	}
	if p.PaneID != "" || p.adoptable || p.role != "" || !p.StatusOnlyDesktop() || p.DesktopNotifications() {
		t.Fatal("saved history claimed control or attention without consent")
	}
}
