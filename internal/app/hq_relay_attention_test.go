package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/hqnudge"
	"github.com/chenchaoyi/gtmux/internal/hqworkspace"
	"github.com/chenchaoyi/gtmux/internal/relay"
	"github.com/chenchaoyi/gtmux/internal/state"
)

func TestHRA003ProspectiveRelayAttentionUsesExactSourceAndRealClaim(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	s := hqworkspace.New(t.TempDir())
	const start = int64(1000)
	prepareAttentionFixture(t, s, start)
	oldEvents, oldRelays, oldGet := workspaceReadEvents, workspaceRelayRecords, workspaceGetAttentionRelay
	oldObserve := workspaceObserveEvent
	t.Cleanup(func() {
		workspaceReadEvents, workspaceRelayRecords, workspaceGetAttentionRelay = oldEvents, oldRelays, oldGet
		workspaceObserveEvent = oldObserve
	})
	makeReport := func(id, thread string, at int64) relay.Request {
		return relay.Request{ID: id, Kind: "report", For: "hq", Agent: "codex", HomeID: "fixture-home", ThreadID: thread,
			TaskID: "trusted-task", Pane: "%84", PanePID: "90", Session: "review", State: "pending", CreatedAt: at,
			Body: "secret body; unrelated contents cannot change scheduling"}
	}
	old := makeReport("older-report", "old-root", start-1)
	ordinary := makeReport("ordinary-report", "ordinary-root", start+1)
	ordinary.TaskID = ""
	other := makeReport("other-home-stop", "other-root", start+2)
	paired := makeReport("formal-review", "paired-root", start+3)
	reports := []relay.Request{old, ordinary, other, paired}
	workspaceRelayRecords = func() []relay.Request { return reports }
	workspaceGetAttentionRelay = func(id string) (relay.Request, error) {
		for _, r := range reports {
			if id == r.ID {
				return r, nil
			}
		}
		return relay.Request{}, relay.ErrMissing
	}
	workspaceReadEvents = func(int64) ([]events.Record, bool) { return nil, false }
	workspaceObserveEvent = func(string, string, string, string, int64, ...string) bool { return true }
	if err := ingestWorkspaceObservations(s, start+10); err != nil {
		t.Fatal(err)
	}
	if err := s.Ingest("fixture-stop", 0, 2, []hqworkspace.Observation{
		{ID: "event-41", Ref: "events:41", Kind: "triage-unassigned:Stop",
			Conversation: hqworkspace.Conversation{Agent: "codex", HomeID: "different-home", SessionID: other.ThreadID}, OccurredAt: start + 5},
		{ID: "event-42", Ref: "events:42", Kind: "triage-unassigned:Stop",
			Conversation: hqworkspace.Conversation{Agent: "codex", HomeID: paired.HomeID, SessionID: paired.ThreadID}, OccurredAt: start + 5},
	}, start+10); err != nil {
		t.Fatal(err)
	}
	d, err := s.Read()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := hqworkspace.RelayAttentionPending(d, old.ID, start+10); ok {
		t.Fatal("historical backlog was enrolled")
	}
	if _, ok := hqworkspace.RelayAttentionPending(d, ordinary.ID, start+10); ok {
		t.Fatal("ordinary report was promoted without managed task proof")
	}
	pair, ok := hqworkspace.RelayAttentionPending(d, paired.ID, start+10)
	if !ok || pair.Observation.MatterID != "" || pair.State != "pending" {
		t.Fatal("unassigned management evidence did not qualify", pair)
	}
	serviceWorkspaceRelayAttention(s, d, start+10)
	path, line := onlyProtectedRelay(t)
	if !strings.Contains(line, "relay:"+paired.ID) || strings.Contains(line, paired.Body) || strings.Contains(line, other.ID) {
		t.Fatalf("wake did not prioritize exact source evidence: %q", line)
	}
	if id, ok := workspaceAttentionLineID(line); !ok || id != paired.ID {
		t.Fatalf("production revalidator could not identify the reserved line: %q", line)
	}
	if keep, out := revalidateWorkspaceRelayAttention(s, start+11, paired.ID, line); !keep || out != line {
		t.Fatal("unclaimed report did not revalidate")
	}

	// Treat the reserved file as no longer available for ordinary retry. The
	// native queue test models the retained unknown outcome; here the actual
	// durable source reservation must prevent another scheduler publication.
	if err := os.Rename(path, path+".native-uncertain"); err != nil {
		t.Fatal(err)
	}
	d, _ = s.Read()
	serviceWorkspaceRelayAttention(s, d, start+121)
	if names, _ := filepath.Glob(filepath.Join(state.Dir(), "hq-nudges", "*.hqrelay")); len(names) != 0 {
		t.Fatal("an unpaired report bypassed its grace period")
	}
	serviceWorkspaceRelayAttention(s, d, start+122)
	_, nextLine := onlyProtectedRelay(t)
	if !strings.Contains(nextLine, "relay:"+other.ID) || strings.Contains(nextLine, paired.ID) {
		t.Fatalf("unknown report was reoffered or blocked independent work: %q", nextLine)
	}
	d, _ = s.Read()
	if q := d.Reconciliations["relay-"+paired.ID]; q.RelayAttentionAt == 0 || q.State != "pending" || q.ReceiptID != "" {
		t.Fatal("opportunity became a claim, receipt or lost its identity", q)
	}
	if _, err := s.ClaimObservation("relay-"+other.ID, "codex:fixture:HQ", start+123); err != nil {
		t.Fatal(err)
	}
	if keep, _ := revalidateWorkspaceRelayAttention(s, start+124, other.ID, nextLine); keep {
		t.Fatal("already claimed report could still start a turn")
	}
	reports[3].State = "closed"
	if keep, _ := revalidateWorkspaceRelayAttention(s, start+124, paired.ID, line); keep {
		t.Fatal("closed relay still eligible while its workspace record is pending")
	}
}

func TestHRA004BlockedExactRelayAlertsIndependentlyAndClearsOnClaim(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	s := hqworkspace.New(t.TempDir())
	const at = int64(1000)
	prepareAttentionFixture(t, s, at-10)
	r := relay.Request{ID: "blocked-review", Kind: "report", For: "hq", Agent: "codex", HomeID: "fixture-home", ThreadID: "source",
		TaskID: "trusted-task", Pane: "%84", PanePID: "90", Session: "review", State: "pending", CreatedAt: at, Body: "private body"}
	o := hqworkspace.Observation{RelayAttention: true, ID: "relay-" + r.ID, Kind: "relay-report", Ref: "relay:" + r.ID, OccurredAt: at,
		Conversation: hqworkspace.Conversation{Agent: r.Agent, HomeID: r.HomeID, SessionID: r.ThreadID}}
	if err := s.Ingest("fixture-relay", 0, 1, []hqworkspace.Observation{o}, at); err != nil {
		t.Fatal(err)
	}
	if err := state.WritePrivateFile(filepath.Join(state.Dir(), "hqwake", "notices-channel.json"), []byte(`{"legacy-down":1000}`)); err != nil {
		t.Fatal(err)
	}
	oldGet, oldReserve := workspaceGetAttentionRelay, workspaceReserveRelayAttention
	t.Cleanup(func() { workspaceGetAttentionRelay, workspaceReserveRelayAttention = oldGet, oldReserve })
	workspaceGetAttentionRelay = func(string) (relay.Request, error) { return r, nil }
	workspaceReserveRelayAttention = func(string, func() (bool, error)) (bool, error) { return false, nil }
	d, err := s.Read()
	if err != nil {
		t.Fatal(err)
	}
	serviceWorkspaceRelayAttention(s, d, at+119)
	serviceWorkspaceRelayAttention(s, d, at+120)
	serviceWorkspaceRelayAttention(s, d, at+160)
	count := 0
	rows, _ := events.ReadSince(0)
	for _, row := range rows {
		if strings.Contains(row.Summary, "gtmux relay show "+r.ID) {
			count++
			if !strings.Contains(row.Summary, "has not reserved") || strings.Contains(row.Summary, r.Body) {
				t.Fatalf("notice did not distinguish the absent opportunity or leaked the body: %s", row.Summary)
			}
		}
	}
	if count != 1 {
		t.Fatalf("want one independent exact notice, got %d", count)
	}
	path := filepath.Join(state.Dir(), "hqwake", "notices-relay-continuation.json")
	workspaceGetAttentionRelay = func(string) (relay.Request, error) { return relay.Request{}, errors.New("temporarily unreadable") }
	serviceWorkspaceRelayAttention(s, d, at+170)
	if body, err := os.ReadFile(path); err != nil || !strings.Contains(string(body), r.ID) {
		t.Fatal("unknown evidence cleared the alert", err)
	}
	if _, err := s.ClaimObservation(o.ID, "codex:fixture:HQ", at+171); err != nil {
		t.Fatal(err)
	}
	d, _ = s.Read()
	serviceWorkspaceRelayAttention(s, d, at+172)
	if body, err := os.ReadFile(path); err != nil || string(body) != "{}" {
		t.Fatalf("verified claim did not clear the exact alert: %s %v", body, err)
	}
}

func prepareAttentionFixture(t *testing.T, s *hqworkspace.Store, at int64) {
	t.Helper()
	if err := s.ActivateObservations(10, at-10); err != nil {
		t.Fatal(err)
	}
	if err := s.Ingest(workspaceRelayAttentionCursor, 0, at, nil, at); err != nil {
		t.Fatal(err)
	}
}

func onlyProtectedRelay(t *testing.T) (string, string) {
	t.Helper()
	names, err := filepath.Glob(filepath.Join(state.Dir(), "hq-nudges", "*.hqrelay"))
	if err != nil || len(names) != 1 {
		t.Fatalf("want one protected report, got %v %v", names, err)
	}
	if !hqnudge.Pending() {
		t.Fatal("the existing fast-tick gate cannot see the reserve")
	}
	body, err := os.ReadFile(names[0])
	if err != nil {
		t.Fatal(err)
	}
	return names[0], string(body)
}
