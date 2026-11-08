package app

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/hqworkspace"
	"github.com/chenchaoyi/gtmux/internal/relay"
)

func TestPhase1WorkspaceIngestionRecoveryAndReadIndependence(t *testing.T) {
	home := t.TempDir()
	s := hqworkspace.New(home)
	if _, e := s.Create("goal-one", "Synthetic", 100); e != nil {
		t.Fatal(e)
	}
	c := hqworkspace.Conversation{Agent: "codex", HomeID: "synthetic-home", SessionID: "thread-one"}
	if e := s.Associate("goal-one", hqworkspace.Association{ID: "association-one", Conversation: c, Role: "execution", From: 100}, 0, 100); e != nil {
		t.Fatal(e)
	}
	oldWake := workspaceReconcileWake
	wakes := []string{}
	workspaceReconcileWake = func(line string) { wakes = append(wakes, line) }
	t.Cleanup(func() { workspaceReconcileWake = oldWake })
	read, seq, relays, observe := workspaceReadEvents, workspaceEventSequence, workspaceRelayRecords, workspaceObserveEvent
	t.Cleanup(func() {
		workspaceReadEvents, workspaceEventSequence, workspaceRelayRecords, workspaceObserveEvent = read, seq, relays, observe
	})
	latest := int64(10)
	gap := false
	rows := []events.Record{}
	workspaceEventSequence = func() int64 { return latest }
	workspaceReadEvents = func(cursor int64) ([]events.Record, bool) {
		out := []events.Record{}
		for _, r := range rows {
			if r.Seq > cursor {
				out = append(out, r)
			}
		}
		return out, gap
	}
	workspaceRelayRecords = func() []relay.Request {
		return []relay.Request{{ID: "relay-one", Kind: "report", Agent: c.Agent, HomeID: c.HomeID, ThreadID: c.SessionID, Body: "Result", CreatedAt: 102, State: "closed"}}
	}
	workspaceObserveEvent = func(_ string, _ string, id string, _ string, _ int64, _ ...string) bool { return id != "denied-thread" }
	if e := ingestWorkspaceObservations(s, 101); e != nil {
		t.Fatal(e)
	}
	latest = 13
	rows = []events.Record{{Seq: 11, Event: "Stop", Agent: "Codex", HomeID: c.HomeID, AgentSession: c.SessionID, Summary: "Turn ended", Ts: 102}, {Seq: 12, Event: "UserPromptSubmit", Origin: events.OriginInstruction, Agent: "Codex", HomeID: c.HomeID, AgentSession: c.SessionID, Summary: "New request", Ts: 103}, {Seq: 13, Event: "Stop", Agent: "Codex", HomeID: c.HomeID, AgentSession: "denied-thread", Ts: 103}}
	if e := ingestWorkspaceObservations(s, 104); e != nil {
		t.Fatal(e)
	}
	d, e := s.Read()
	if e != nil || len(d.Reconciliations) != 3 || d.Matters["goal-one"].State != "open" || len(d.Messages) != 0 {
		t.Fatal(d, e)
	}
	next, e := notifyWorkspaceReconciliation(s, 104, 0)
	if e != nil || len(wakes) != 1 || !strings.Contains(wakes[0], "workspace reconcile") {
		t.Fatal(wakes, e)
	}
	if _, e = notifyWorkspaceReconciliation(s, 105, next); e != nil || len(wakes) != 1 {
		t.Fatal("wake did not debounce", e)
	}
	if _, e = notifyWorkspaceReconciliation(hqworkspace.New(home), 106, 0); e != nil || len(wakes) != 2 {
		t.Fatal("restart forgot debt wake", e)
	}
	// Reading event records or workspace projections never discharges business debt.
	_, _ = workspaceReadEvents(13)
	_ = workspaceStoredSnapshot(d, true, 105)
	raw, e := workspaceReconciliationJSON(s, 105)
	if e != nil || !strings.Contains(string(raw), "event-11") {
		t.Fatal(string(raw), e)
	}
	after, _ := s.Read()
	if !reflect.DeepEqual(d, after) {
		t.Fatal("read consumed reconciliation")
	}
	s = hqworkspace.New(home)
	if e := ingestWorkspaceObservations(s, 106); e != nil {
		t.Fatal(e)
	}
	after, _ = s.Read()
	if !reflect.DeepEqual(d, after) {
		t.Fatal("restart duplicated observations")
	}
	latest = 15
	rows = nil
	gap = true
	if e := ingestWorkspaceObservations(s, 107); e != nil {
		t.Fatal(e)
	}
	after, _ = s.Read()
	if after.ObservationCursors["events"] != 13 || len(after.Reconciliations) != 4 {
		t.Fatal("source gap lost", after)
	}
	t.Log("isolated records:", filepath.Join(home, "workspace", "records.json"))
}

func TestPhase1WorkspaceActionAuthorizationAndCompatibility(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("TMUX_PANE", "")
	s := hqworkspace.New(t.TempDir())
	if _, e := s.Create("goal-one", "Synthetic", 100); e != nil {
		t.Fatal(e)
	}
	if _, e := applyWorkspaceAction(s, workspaceAction{Op: "claim", ObservationID: "not-found"}, 101); e == nil {
		t.Fatal("non-HQ claimed observation")
	}
	d, _ := s.Read()
	if e := s.Update("goal-one", "", "completed", "Verified", d.Matters["goal-one"].Revision, 102); e != nil {
		t.Fatal(e)
	}
	if _, e := s.PublishVersion("report-one", "goal-one", "", "result", "Old report", "open", 1, 103); e != nil {
		t.Fatal(e)
	}
	d, _ = s.Read()
	snapshot := workspaceStoredSnapshot(d, true, 104)
	if snapshot.Schema != 1 || snapshot.Matters[0].State != "completed" {
		t.Fatal("legacy read projection changed", snapshot)
	}
	a, e := readWorkspaceAction(strings.NewReader(`{"op":"assess","id":"receipt-one","observation_id":"message-report-one","generation":1,"matter":"goal-one","revision":2,"decision":"noted","body":"Old evidence","dry_run":true}`))
	if e != nil || !a.DryRun {
		t.Fatal(a, e)
	}
	if _, e = applyWorkspaceAction(s, a, 105); e == nil {
		t.Fatal("non-HQ assessed observation")
	}
	raw, e := workspaceReconciliationJSON(s, 105)
	if e != nil {
		t.Fatal(e)
	}
	var value struct {
		Pending []hqworkspace.Reconciliation `json:"pending"`
	}
	if e = json.Unmarshal(raw, &value); e != nil || len(value.Pending) != 1 {
		t.Fatal(string(raw), e)
	}
	if value.Pending[0].Observation.MatterRevision != 1 {
		t.Fatal("report lost source revision")
	}
}

func TestPhase1HQCanClaimDryRunAndCommit(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	s := hqworkspace.New(t.TempDir())
	m, e := s.Create("goal-one", "Synthetic", 100)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.PublishVersion("report-one", "goal-one", "", "result", "Verified output", "completed", m.Revision, 101); e != nil {
		t.Fatal(e)
	}
	oldHQ, oldOwner := workspaceIsHQCaller, workspaceAssessmentOwner
	t.Cleanup(func() { workspaceIsHQCaller, workspaceAssessmentOwner = oldHQ, oldOwner })
	workspaceIsHQCaller = func() bool { return true }
	workspaceAssessmentOwner = func() string { return "synthetic-hq-thread" }
	value, e := applyWorkspaceAction(s, workspaceAction{Op: "claim", ObservationID: "message-report-one"}, 102)
	if e != nil {
		t.Fatal(e)
	}
	claim := value.(hqworkspace.Reconciliation)
	a := workspaceAction{Op: "assess", ID: "receipt-one", ObservationID: claim.Observation.ID, Generation: claim.Generation, Matter: m.ID, Revision: m.Revision, Decision: "transition", State: "completed", Body: "Verified output; no remaining work", DryRun: true}
	before, _ := s.Read()
	if _, e = applyWorkspaceAction(s, a, 103); e != nil {
		t.Fatal(e)
	}
	after, _ := s.Read()
	if !reflect.DeepEqual(before, after) {
		t.Fatal("CLI dry run changed Store")
	}
	a.DryRun = false
	if _, e = applyWorkspaceAction(s, a, 103); e != nil {
		t.Fatal(e)
	}
	after, _ = s.Read()
	if after.Matters[m.ID].State != "completed" || after.Reconciliations[claim.Observation.ID].ReceiptID != a.ID {
		t.Fatal("CLI missed projection or receipt")
	}
	// The existing desktop Continue following action supplies revision, no body.
	if _, e = applyWorkspaceAction(s, workspaceAction{Op: "update", Matter: m.ID, State: "open", Revision: after.Matters[m.ID].Revision}, 104); e != nil {
		t.Fatal("explicit legacy user reopen failed", e)
	}
}
