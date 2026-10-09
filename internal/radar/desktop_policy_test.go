package radar

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/native"
	"github.com/chenchaoyi/gtmux/internal/sessionpolicy"
	"github.com/chenchaoyi/gtmux/internal/state"
)

func TestDesktopRadarDefaultConsentAndHQDirectoryIdentity(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	home := t.TempDir()
	t.Setenv("CODEX_HOME", home)
	root := filepath.Join(home, "sessions", "2026", "10", "09")
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	now := time.Now().Unix()
	for _, tc := range []struct{ id, originator string }{{"desk", "Codex Desktop"}, {"term", "codex-tui"}} {
		data := `{"type":"session_meta","payload":{"id":"` + tc.id + `","originator":"` + tc.originator + `"}}` + "\n"
		if err := os.WriteFile(filepath.Join(root, "rollout-2026-10-09T00-00-00-"+tc.id+".jsonl"), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		if err := native.Save(native.Record{SessionID: tc.id, Agent: "codex", State: "working", UpdatedAt: now, PID: os.Getpid(), Cwd: filepath.Join(state.Home(), ".config", "gtmux", "hq")}); err != nil {
			t.Fatal(err)
		}
	}
	check := func(follow bool) {
		t.Helper()
		panes := nativePanes(nil, nil, now)
		if len(panes) != 2 {
			t.Fatalf("rows=%+v", panes)
		}
		for _, p := range panes {
			if p.sessionID == "desk" {
				if p.StatusOnlyDesktop() == follow || p.role != "" || p.adoptable || p.Follow == nil || p.Follow.HQ != follow || p.DesktopNotifications() {
					t.Fatalf("desktop policy/identity=%+v", p)
				}
				b, err := agentsJSONBytes([]Pane{p})
				if err != nil {
					t.Fatal(err)
				}
				var rows []map[string]any
				if err := json.Unmarshal(b, &rows); err != nil {
					t.Fatal(err)
				}
				v := rows[0]
				if v["follow"] == nil || v["client"] != "chatgpt_desktop" {
					t.Fatalf("contract=%s", b)
				}
			} else if p.StatusOnlyDesktop() || p.Follow != nil {
				t.Fatalf("terminal changed: %+v", p)
			}
		}
	}
	check(false)
	panes := nativePanes(nil, nil, now)
	var desk Pane
	for _, p := range panes {
		if p.sessionID == "desk" {
			desk = p
		}
	}
	if got := digestFor([]Pane{desk}, nil); len(got) != 0 {
		t.Fatalf("unselected content entered digest: %+v", got)
	}
	if _, err := sessionpolicy.Save("desk", sessionpolicy.Settings{HQ: true}); err != nil {
		t.Fatal(err)
	}
	check(true)
	for _, p := range nativePanes(nil, nil, now) {
		if p.sessionID == "desk" {
			desk = p
		}
	}
	if got := digestFor([]Pane{desk}, nil); len(got) != 1 || got[0].Client != "chatgpt_desktop" || got[0].SessionID != "desk" || got[0].Follow.Knowledge {
		t.Fatalf("follow digest identity/permissions=%+v", got)
	}
}
