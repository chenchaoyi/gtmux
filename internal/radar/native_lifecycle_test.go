package radar

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/native"
	"github.com/chenchaoyi/gtmux/internal/sessionpolicy"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

func TestCompletedPIDLessCodexIsHistoryUntilFreshTurnOrExplicitFollow(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	home := t.TempDir()
	t.Setenv("CODEX_HOME", home)
	now := time.Now().Unix()
	root := filepath.Join(home, "sessions", "2026", "10", "19")
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "rollout-2026-10-19T00-00-00-history.jsonl")
	body := `{"type":"session_meta","payload":{"id":"history","originator":"codex-tui"}}` + "\n" + `{"timestamp":"` + time.Unix(now-5, 0).UTC().Format(time.RFC3339) + `","type":"event_msg","payload":{"type":"task_complete"}}` + "\n"
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	r := native.Record{Agent: "codex", SessionID: "history", HomeID: transcript.CodexHomeID(), State: "working", UpdatedAt: now - 10}
	save := func() {
		t.Helper()
		if err := native.Save(r); err != nil {
			t.Fatal(err)
		}
	}
	check := func(visible bool) {
		t.Helper()
		rows := nativePanes(nil, nil, now)
		if (len(rows) == 1) != visible {
			t.Fatalf("rows=%+v want visible=%v", rows, visible)
		}
	}
	save()
	check(false)
	if _, ok := native.Load(r.SessionID, r.HomeID); !ok {
		t.Fatal("presentation deleted observation")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("presentation deleted history", err)
	}
	// A later submit or waiting hook must surface again even if a saved completed
	// turn still exists, without resetting consent or acquiring input authority.
	r.UpdatedAt = now
	r.State = "working"
	save()
	check(true)
	r.State = "waiting"
	save()
	check(true)
	r.State = "idle"
	r.PID = os.Getpid()
	save()
	check(true)
	r.PID = 0
	save()
	check(false)
	if _, err := sessionpolicy.Save("history", sessionpolicy.Settings{HQ: true}); err != nil {
		t.Fatal(err)
	}
	check(true)
	rows := nativePanes(nil, nil, now)
	if rows[0].currentClient != "unconfirmed" || rows[0].adoptable || rows[0].Follow == nil || !rows[0].Follow.HQ {
		t.Fatal("changed control/consent", rows)
	}
}
