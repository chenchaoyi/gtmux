package hqnudge

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/state"
)

func reservationFixture(t *testing.T, sent int64) string {
	t.Helper()
	p := filepath.Join(state.Dir(), "hqwake", "rotate-pending.json")
	if err := state.MkdirAll(filepath.Dir(p), 0700); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(map[string]any{"pane": "%isolated", "agent": "codex", "retiring": "old-session", "input": "/new", "createdAt": 1700000000, "sentAt": sent})
	if err := state.WriteFile(p, b, 0600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestHRW004PublicChannelsHoldBehindRotationAndPreservePriority(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	f := &fake{}
	old := prod
	prod = f.io()
	defer func() { prod = old }()
	calls := 0
	prod.nativeSubmit = func(string, string, string) (dispatch.Result, bool) {
		calls++
		return dispatch.Result{State: dispatch.StateRefusedWaiting}, true
	}
	p := reservationFixture(t, 0)
	before, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	events.Append(events.Record{Event: "Waiting", Pane: "%worker", Summary: "important blocker", Severity: events.SevImportant, Ts: 1700000000})
	Deliver("%isolated", "» gtmux·waiting important blocker")
	DeliverKeyedAt("%isolated", "routine-key", "» gtmux·tick old", 1)
	DeliverKeyedAt("%isolated", "routine-key", "» gtmux·tick newest", 1)
	Drain("%isolated")
	if calls != 0 || f.pastes != 0 || len(f.sent) != 0 {
		t.Fatalf("new wake crossed rotation reservation: native=%d paste=%d", calls, f.pastes)
	}
	if queuedCount(t) != 2 || !Pending() {
		t.Fatal("retained entries/key coalescing lost")
	}
	after, _ := os.ReadFile(p)
	if string(before) != string(after) {
		t.Fatal("wake altered maintenance request")
	}
	records, _ := events.ReadSince(0)
	important := 0
	for _, r := range records {
		if r.Severity == events.SevImportant && r.Summary == "important blocker" {
			important++
		}
		if r.Event == events.AuditEventWakeDelivered {
			t.Fatal("held input claimed delivered")
		}
	}
	if important != 1 {
		t.Fatal("source important event lost", records)
	}
	var payload string
	prod.nativeSubmit = func(_ string, s, _ string) (dispatch.Result, bool) {
		calls++
		payload = s
		return dispatch.Result{State: dispatch.StateRefusedWaiting}, true
	}
	reservationFixture(t, 1700000001)
	Drain("%isolated")
	if calls != 1 || !strings.Contains(payload, "important blocker") || !strings.Contains(payload, "newest") || strings.Contains(payload, "tick old") || strings.Index(payload, "important blocker") > strings.Index(payload, "newest") {
		t.Fatal("sent handoff did not release priority-preserving drain", calls, payload)
	}
	if queuedCount(t) != 2 {
		t.Fatal("busy native refusal lost saved entries")
	}
}

func TestHRW004RotationDoesNotSubmitOrRepairUserDraft(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	f := &fake{draft: "unsent user draft"}
	old := prod
	prod = f.io()
	defer func() { prod = old }()
	reservationFixture(t, 0)
	Deliver("%isolated", "» gtmux·waiting retained")
	Drain("%isolated")
	if f.draft != "unsent user draft" || f.pastes != 0 || len(f.sent) != 0 || queuedCount(t) != 1 {
		t.Fatal("rotation wake touched user draft", f)
	}
}

func TestHRW004ExactOwnStrandedRepairPrecedesReservation(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	f := &fake{swallow: true}
	x := f.io()
	deliver(x, "%isolated", "» gtmux·waiting own stranded")
	if f.pastes != 1 || len(f.sent) != 0 {
		t.Fatal("stranded fixture did not paste exactly once", f)
	}
	reservationFixture(t, 0)
	enqueue(x, "» gtmux·tick later retained")
	f.swallow = false
	drain(x, "%isolated")
	if f.pastes != 1 || len(f.sent) != 1 || queuedCount(t) != 1 || !strings.Contains(f.sent[0], "own stranded") || strings.Contains(f.sent[0], "later retained") {
		t.Fatal("maintenance blocked exact own repair or sent new batch", f)
	}
}

func TestHRW005AutomaticProducerSetsPolicyAndKeepsHQIdentity(t *testing.T) {
	oldIO, oldTarget := nativeWakeIO, nativeWakeTarget
	defer func() { nativeWakeIO, nativeWakeTarget = oldIO, oldTarget }()
	current := "%isolated"
	calls := 0
	nativeWakeTarget = func() string { return current }
	nativeWakeIO = func(pane string, agents ...string) dispatch.IO {
		if pane != "%isolated" || len(agents) != 1 || agents[0] != "codex" {
			t.Fatal("wrong native producer target", pane, agents)
		}
		return dispatch.IO{CaptureColor: func() string { return boxWith("") }, InMode: func() bool { return false }, Sleep: func() {},
			SubmitNative: func(opts dispatch.Opts, text string, hold func() string) dispatch.Result {
				calls++
				if !opts.IdleWakeInput || opts.DirectUserInput || opts.ActiveImageInput || opts.DeliveryID != "hq-wake-#abc123" || hold() == "" && current != "%isolated" {
					t.Fatal("automatic producer policy changed", opts)
				}
				return dispatch.Result{State: dispatch.StateQueued, Delivery: "wake-accepted"}
			},
			Paste: func(string) error { t.Fatal("native wake pasted"); return nil }, Enter: func() error { t.Fatal("native wake pressed Enter"); return nil },
		}
	}
	if result, managed := nativeWakeSubmit("%isolated", "synthetic auto wake", "#abc123"); !managed || result.State != dispatch.StateQueued || calls != 1 {
		t.Fatal(result, managed, calls)
	}
	current = "%foreign"
	if result, managed := nativeWakeSubmit("%isolated", "synthetic auto wake", "#abc123"); !managed || result.State != dispatch.StateRefusedWaiting || calls != 1 {
		t.Fatal("changed HQ received input", result, managed, calls)
	}
}

func TestHRW008BusyRetentionAndAcceptedReceiptDoNotConsumeImportantSource(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	events.Append(events.Record{Event: "Waiting", Pane: "%worker", Summary: "important original blocker", Severity: events.SevImportant, Ts: 1700000000})
	f := &fake{}
	x := f.io()
	calls := 0
	busy := true
	x.nativeSubmit = func(string, string, string) (dispatch.Result, bool) {
		calls++
		if busy {
			return dispatch.Result{State: dispatch.StateRefusedWaiting}, true
		}
		return dispatch.Result{State: dispatch.StateQueued, Delivery: "wake-accepted"}, true
	}
	deliver(x, "%isolated", "» gtmux·waiting important original blocker")
	if calls != 1 || queuedCount(t) != 1 {
		t.Fatal("busy wake was dropped", calls)
	}
	if got := auditRecords(t, events.AuditEventWakeDelivered); len(got) != 0 {
		t.Fatal("busy wake claimed delivered", got)
	}
	if got := auditRecords(t, events.AuditEventWakeAccepted); len(got) != 0 {
		t.Fatal("busy wake claimed accepted", got)
	}
	busy = false
	drain(x, "%isolated")
	if calls != 2 || Pending() || len(f.sent) != 0 {
		t.Fatal("idle acceptance did not close exact local batch", calls, f)
	}
	accepted := auditRecords(t, events.AuditEventWakeAccepted)
	if len(accepted) != 1 || events.IsSupervisorAct(accepted[0]) || accepted[0].Severity != events.SevRoutine {
		t.Fatal("transport acceptance misreported as work", accepted)
	}
	if got := auditRecords(t, events.AuditEventWakeDelivered); len(got) != 0 {
		t.Fatal("accepted was relabelled model-read/delivered", got)
	}
	all, _ := events.ReadSince(0)
	important := 0
	for _, r := range all {
		if r.Event == "Waiting" && r.Summary == "important original blocker" && r.Severity == events.SevImportant {
			important++
		}
	}
	if important != 1 {
		t.Fatal("acceptance consumed source blocker", all)
	}
	drain(x, "%isolated")
	if calls != 2 {
		t.Fatal("accepted batch replayed")
	}
}
