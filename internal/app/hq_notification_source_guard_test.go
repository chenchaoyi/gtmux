package app

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/hqworkspace"
	"github.com/chenchaoyi/gtmux/internal/relay"
	"github.com/chenchaoyi/gtmux/internal/state"
)

func assertNotificationSnapshot(t *testing.T, scope, want string) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(state.NotifyDir(), "hq-"+scope+".attention"))
	if err != nil || string(b) != want {
		t.Fatalf("%s attention=%s, %v; want %s", scope, b, err, want)
	}
}

func TestHND006WorkspaceSourceOutageKeepsLiveAttentionAndPreventsReset(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	home := t.TempDir()
	s := hqworkspace.New(home)
	const at = 1000
	boot, err := workspaceForReconciliation(s, at)
	if err != nil || boot != s {
		t.Fatalf("a new workspace must be allowed to bootstrap: %v", err)
	}
	m, err := boot.Create("work", "fixture work", at)
	if err != nil {
		t.Fatal(err)
	}
	question := hqworkspace.UserInput{ID: "question", Kind: "decision", Prompt: "Continue?", Why: "Operator choice", Impact: "Proceed"}
	if err := s.RequestInput(m.ID, question, m.Revision, at+1); err != nil {
		t.Fatal(err)
	}
	verified, err := workspaceForReconciliation(s, at+2)
	if err != nil {
		t.Fatal(err)
	}
	assertNotificationSnapshot(t, "workspace", `["workspace/work"]`)

	path := filepath.Join(home, "workspace", "records.json")
	saved := path + ".temporarily-unavailable"
	if err := os.Rename(path, saved); err != nil {
		t.Fatal(err)
	}
	if pending, err := workspaceForReconciliation(s, at+3); pending != nil || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a missing source was taken for new work: %v %v", pending, err)
	}
	if _, err := verified.Create("accidental", "must not initialize", at+4); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("previously opened reconciliation recreated missing records: %v", err)
	}
	assertNotificationSnapshot(t, "workspace", `["workspace/work"]`)

	if err := os.Rename(saved, path); err != nil {
		t.Fatal(err)
	}
	if _, err := workspaceForReconciliation(s, at+5); err != nil {
		t.Fatal(err)
	}
	assertNotificationSnapshot(t, "workspace", `["workspace/work"]`)
	d, err := s.Read()
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ClearInput(m.ID, question.ID, "clear-receipt", "Verified completion of the fixture question.", d.Matters[m.ID].Revision, at+6); err != nil {
		t.Fatal(err)
	}
	if _, err := workspaceForReconciliation(s, at+7); err != nil {
		t.Fatal(err)
	}
	assertNotificationSnapshot(t, "workspace", "[]")
}

func TestHND006RelayParentOutageIsDistinctFromAnExactMissingReport(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	store := hqworkspace.New(t.TempDir())
	intake := []hqworkspace.Observation{}
	var at int64
	for _, id := range []string{"report-a", "report-b"} {
		r, _, err := relay.Put(relay.Request{ID: id, Kind: "report", For: "hq", Agent: "codex",
			HomeID: "fixture-home", ThreadID: "fixture-thread", Pane: "%7", PanePID: "81", TaskID: "fixture-task",
			Body: "fixture report", Session: "fixture-session"})
		if err != nil {
			t.Fatal(err)
		}
		at = max(at, r.CreatedAt)
		intake = append(intake, hqworkspace.Observation{ID: "relay-" + id, Ref: "relay:" + id, Kind: "relay-report",
			OccurredAt: r.CreatedAt, RelayAttention: true,
			Conversation: hqworkspace.Conversation{Agent: r.Agent, HomeID: r.HomeID, SessionID: r.ThreadID}})
	}
	prepareAttentionFixture(t, store, at-10)
	if err := store.Ingest("fixture-relay", 0, 2, intake, at); err != nil {
		t.Fatal(err)
	}
	oldReserve := workspaceReserveRelayAttention
	t.Cleanup(func() { workspaceReserveRelayAttention = oldReserve })
	workspaceReserveRelayAttention = func(string, func() (bool, error)) (bool, error) { return false, nil }
	d, err := store.Read()
	if err != nil {
		t.Fatal(err)
	}
	const both = `["relay/report-a","relay/report-b"]`
	serviceWorkspaceRelayAttention(store, d, at+130)
	assertNotificationSnapshot(t, "relay", both)

	parent := filepath.Join(state.Dir(), "relay")
	offline := parent + ".offline"
	if err := os.Rename(parent, offline); err != nil {
		t.Fatal(err)
	}
	serviceWorkspaceRelayAttention(store, d, at+131)
	assertNotificationSnapshot(t, "relay", both)

	if err := os.Rename(offline, parent); err != nil {
		t.Fatal(err)
	}
	serviceWorkspaceRelayAttention(store, d, at+132)
	assertNotificationSnapshot(t, "relay", both)
	if err := os.Rename(filepath.Join(parent, "report-a.json"), filepath.Join(parent, "report-a.fixture")); err != nil {
		t.Fatal(err)
	}
	serviceWorkspaceRelayAttention(store, d, at+133)
	assertNotificationSnapshot(t, "relay", `["relay/report-b"]`)
}
