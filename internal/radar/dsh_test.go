package radar

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/agentclient"
	"github.com/chenchaoyi/gtmux/internal/conversation"
	"github.com/chenchaoyi/gtmux/internal/dsh"
	"github.com/chenchaoyi/gtmux/internal/native"
	"github.com/chenchaoyi/gtmux/internal/sessionpolicy"
)

func TestDSHFollowedHistoryUsesExternalContractWithoutClaimingControl(t *testing.T) {
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
	if _, err := sessionpolicy.SaveFor("dsh", "web", sessionpolicy.Settings{HQ: true}); err != nil {
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
	if p.PaneID != "" || p.adoptable || p.role != "" || p.StatusOnlyDesktop() || p.DesktopNotifications() || p.Follow == nil || !p.Follow.HQ {
		t.Fatal("followed history changed control or notification consent")
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
	if rows := nativePanes(nil, nil, time.Now().Unix()); len(rows) != 0 {
		t.Fatal("released session reappeared from history alone")
	}
	if _, err := conversation.Resolve(a.SessionID, a.HomeID); err != nil {
		t.Fatal("hiding history lost exact read access", err)
	}
	if _, err := sessionpolicy.SaveFor("dsh", a.SessionID, sessionpolicy.Settings{HQ: true}); err != nil {
		t.Fatal(err)
	}
	terminal[a.Pane] = a
	if rows := nativePanes(nil, nil, time.Now().Unix()); len(rows) != 0 {
		t.Fatal("explicit follow duplicated a managed session")
	}
	delete(terminal, a.Pane)
	if rows := nativePanes(nil, nil, time.Now().Unix()); len(rows) != 1 || rows[0].adoptable {
		t.Fatal("explicitly followed history lost read-only behavior")
	}
}

func TestDSHExternalHistoryDoesNotReappearWithoutActivityOrFollow(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CODEX_HOME", t.TempDir())
	t.Setenv("DSH_HOME", t.TempDir())
	now := time.Now().Unix()
	for _, phase := range []string{"complete", "working", "waiting"} {
		t.Run(phase, func(t *testing.T) {
			id := "history-" + phase
			dir := filepath.Join(dsh.Home(), "sessions", "project", id)
			if err := os.MkdirAll(dir, 0700); err != nil {
				t.Fatal(err)
			}
			body := `{"type":"session","version":4,"id":"` + id + `"}` + "\n" + `{"type":"user/message","seq":1,"time":2000,"data":{"role":"user","source":{"kind":"user"},"content":[{"type":"text","text":"synthetic"}]}}` + "\n"
			switch phase {
			case "complete":
				body += `{"type":"turn/end","time":3000,"data":{"reason":{"kind":"completed"}}}` + "\n"
			case "working":
				body += `{"type":"turn/start","time":3000,"data":{}}` + "\n"
			case "waiting":
				body += `{"type":"user-questions/request","time":3000,"data":{}}` + "\n"
			}
			path := filepath.Join(dir, "session.v4.jsonl")
			if err := os.WriteFile(path, []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
			check := func(visible bool) Pane {
				t.Helper()
				rows := nativePanes(nil, nil, now)
				for _, row := range rows {
					if row.sessionID == id {
						if !visible {
							t.Fatal("history reappeared", row)
						}
						return row
					}
				}
				if visible {
					t.Fatal("active or followed session missing", rows)
				}
				return Pane{}
			}
			check(false)
			// A fresh process observation surfaces the exact session. A dead/reused
			// process after reboot must not be bootstrapped again by the saved file.
			rec := native.Record{Agent: "dsh", SessionID: id, HomeID: dsh.HomeID(), State: "working", UpdatedAt: now, PID: os.Getpid()}
			if err := native.Save(rec); err != nil {
				t.Fatal(err)
			}
			check(true)
			rec.Comm = "not-the-process-that-recorded-this-session"
			if err := native.Save(rec); err != nil {
				t.Fatal(err)
			}
			check(false)
			// New PID-less hooks can report work/wait, but completed observations are
			// history. Filtering does not remove the observation or conversation.
			rec.PID, rec.Comm, rec.State = 0, "", "waiting"
			if err := native.Save(rec); err != nil {
				t.Fatal(err)
			}
			check(true)
			rec.State = "idle"
			if err := native.Save(rec); err != nil {
				t.Fatal(err)
			}
			check(false)
			if _, ok := native.Load(id, dsh.HomeID()); !ok {
				t.Fatal("presentation removed observation")
			}
			if _, err := os.Stat(path); err != nil {
				t.Fatal("presentation removed history", err)
			}
			native.Remove(id, dsh.HomeID())
			grant, err := sessionpolicy.SaveFor("dsh", id, sessionpolicy.Settings{HQ: true})
			if err != nil {
				t.Fatal(err)
			}
			row := check(true)
			if row.Follow == nil || *row.Follow != grant || row.currentClient != "unconfirmed" || row.adoptable || row.PaneID != "" {
				t.Fatal("history changed consent/control", row)
			}
			if row.Status == "working" || row.Status == "waiting" {
				t.Fatal("unfinished history claimed a live phase", row)
			}
			// Revocation removes the history-only row and preserves the source file.
			grant.HQ = false
			if _, err := sessionpolicy.SaveFor("dsh", id, grant); err != nil {
				t.Fatal(err)
			}
			check(false)
			if _, err := conversation.Resolve(id, dsh.HomeID()); err != nil {
				t.Fatal("history no longer readable", err)
			}
		})
	}
}
