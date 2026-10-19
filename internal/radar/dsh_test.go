package radar

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/agentclient"
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

func TestVerifiedTerminalIdentityAndExternalDedup(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DSH_HOME", t.TempDir())
	t.Setenv("CODEX_HOME", t.TempDir())
	a := agentclient.Attachment{Agent: "dsh", Client: "dst", Pane: "%44", HomeID: dsh.HomeID(), SessionID: "synthetic"}
	terminal := map[string]agentclient.Attachment{a.Pane: a}
	fields := []string{a.Pane, "session", "0", "0", "unbranded", "node", "/project", "0", "123"}
	identity := identifyPane(fields, nil, nil, LoadProfiles(), terminal)
	if !identity.isAgent || identity.agent != "DeepSeek Harness" {
		t.Fatal("community client lost harness identity", identity)
	}
	if identifyPane(fields, nil, nil, LoadProfiles(), map[string]agentclient.Attachment{}).isAgent {
		t.Fatal("unverified client became an agent")
	}
	old := terminalAttachments
	terminalAttachments = func() map[string]agentclient.Attachment { return terminal }
	defer func() { terminalAttachments = old }()
	dir := filepath.Join(dsh.Home(), "sessions", "project", "synthetic")
	_ = os.MkdirAll(dir, 0700)
	text := `{"type":"session","version":4,"id":"synthetic"}` + "\n" + `{"type":"user/message","seq":1,"time":2000,"data":{"role":"user","source":{"kind":"user"},"content":[{"type":"text","text":"synthetic"}]}}` + "\n"
	_ = os.WriteFile(filepath.Join(dir, "session.v4.jsonl"), []byte(text), 0600)
	if rows := nativePanes(nil, nil, time.Now().Unix()); len(rows) != 0 {
		t.Fatal("managed session duplicated as external")
	}
	terminal = map[string]agentclient.Attachment{}
	if rows := nativePanes(nil, nil, time.Now().Unix()); len(rows) != 1 || rows[0].adoptable {
		t.Fatal("released session did not retain external read-only behavior")
	}
}
