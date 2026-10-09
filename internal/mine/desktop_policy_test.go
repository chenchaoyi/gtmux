package mine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/sessionpolicy"
)

func TestDesktopMiningRequiresIndependentFutureConsent(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	home := t.TempDir()
	t.Setenv("CODEX_HOME", home)
	root := filepath.Join(home, "sessions", "2026", "10", "09")
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "rollout-2026-10-09T00-00-00-mine-desktop.jsonl")
	old := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	appendDesktopLog(t, path, cx(0, old, "session_meta", rec{"id": "mine-desktop", "originator": "codex_work_desktop", "cwd": "/w/repo"}), cx(1, old, "response_item", cxMsg("assistant", "Made a change.")), cx(2, old, "response_item", cxMsg("user", "不对，你没部署，那是 dry run")))
	dir := t.TempDir()
	options := Options{Roots: []Root{{Agent: "codex", Dir: root, Glob: "*.jsonl"}}}
	run := func() Report {
		t.Helper()
		rep, err := Run(dir, options)
		if err != nil {
			t.Fatal(err)
		}
		return rep
	}
	if rep := run(); rep.Files != 0 || len(rep.Candidates) != 0 {
		t.Fatalf("unconsented content was read: %+v", rep)
	}
	v, err := sessionpolicy.Save("mine-desktop", sessionpolicy.Settings{HQ: true})
	if err != nil {
		t.Fatal(err)
	}
	if rep := run(); rep.Files != 0 {
		t.Fatal("HQ follow silently granted learning")
	}
	v.Knowledge = true
	v, err = sessionpolicy.Save("mine-desktop", v)
	if err != nil {
		t.Fatal(err)
	}
	if rep := run(); len(rep.Candidates) != 0 {
		t.Fatal("old conversation retroactively mined")
	}
	after := time.Unix(v.KnowledgeSince+2, 0).UTC().Format(time.RFC3339)
	appendDesktopLog(t, path, cx(3, after, "response_item", cxMsg("assistant", "Made another change.")), cx(4, after, "response_item", cxMsg("user", "不对，你没部署，那是 dry run")))
	if rep := run(); len(rep.Candidates) != 1 || rep.Candidates[0].Session != "mine-desktop" {
		t.Fatalf("future authorized lead missing or misattributed: %+v", rep)
	}
	v.HQ = false
	if _, err = sessionpolicy.Save("mine-desktop", v); err != nil {
		t.Fatal(err)
	}
	appendDesktopLog(t, path, cx(5, after, "response_item", cxMsg("assistant", "Changed again.")), cx(6, after, "response_item", cxMsg("user", "不对，你没部署，那是 dry run")))
	if rep := run(); rep.Files != 0 || len(rep.Candidates) != 0 {
		t.Fatal("revoked conversation still mined")
	}
}

func appendDesktopLog(t *testing.T, path string, records ...rec) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range records {
		if err := json.NewEncoder(f).Encode(r); err != nil {
			f.Close()
			t.Fatal(err)
		}
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestDesktopMessagesWithoutOrdinalKeepDistinctProvenance(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	home := t.TempDir()
	t.Setenv("CODEX_HOME", home)
	root := filepath.Join(home, "sessions", "2026", "10", "09")
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "rollout-2026-10-09T00-00-00-no-ordinal.jsonl")
	appendDesktopLog(t, path, rec{"type": "session_meta", "payload": rec{"id": "no-ordinal", "originator": "Codex Desktop"}})
	p, err := sessionpolicy.Save("no-ordinal", sessionpolicy.Settings{HQ: true, Knowledge: true})
	if err != nil {
		t.Fatal(err)
	}
	ts := time.Unix(p.KnowledgeSince+2, 0).UTC().Format(time.RFC3339)
	message := func(role, text string) rec {
		return rec{"timestamp": ts, "type": "response_item", "payload": cxMsg(role, text)}
	}
	appendDesktopLog(t, path, message("assistant", "A change."), message("user", "不对，你没部署，那是 dry run"), message("assistant", "Another change."), message("user", "不对，你没部署，那是 dry run"))
	ledger := t.TempDir()
	o := Options{Roots: []Root{{Agent: "codex", Dir: root, Glob: "*.jsonl"}}}
	rep, err := Run(ledger, o)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Candidates) != 2 || rep.Candidates[0].ID == rep.Candidates[1].ID {
		t.Fatalf("modern messages collided: %+v", rep)
	}
	appendDesktopLog(t, path, message("assistant", "A third change."), message("user", "不对，你没部署，那是 dry run"))
	next, err := Run(ledger, o)
	if err != nil {
		t.Fatal(err)
	}
	if len(next.Candidates) != 1 || next.Candidates[0].ID == rep.Candidates[0].ID || next.Candidates[0].Session != "no-ordinal" {
		t.Fatalf("incremental identity=%+v", next)
	}
}
