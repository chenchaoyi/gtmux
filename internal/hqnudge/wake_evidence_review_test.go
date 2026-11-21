package hqnudge

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

func TestHWA014AcceptedProofBudgetDoesNotFreezeAdmission(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := time.Unix(1700000000, 0)
	if err := state.MkdirAll(acceptedWakeDir(), 0700); err != nil {
		t.Fatal(err)
	}
	// These are valid accepted records whose old transcripts are no longer available.
	for i := range maxQueueEntries {
		turn := fmt.Sprintf("historical-%d", i)
		record := acceptedWake{Pane: "%hq", Batch: "#123abc", Payload: "earlier accepted wake", Since: now.Add(-time.Hour).UnixNano(),
			Receipt: dispatch.WakeReceipt{Home: "another-retired-home", Session: "retired-session", Turn: turn, Request: turn}}
		body, err := json.Marshal(record)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(acceptedWakeDir(), fmt.Sprintf("%064x.json", i+1)), body, 0600); err != nil {
			t.Fatal(err)
		}
	}

	x := (&fake{}).io()
	x.nowNano = func() int64 { return now.UnixNano() }
	var submissions []string
	x.nativeSubmit = func(_, payload, _ string) (dispatch.Result, bool) {
		submissions = append(submissions, payload)
		turn := fmt.Sprintf("fresh-%d", len(submissions))
		return dispatch.Result{State: dispatch.StateQueued, WakeReceipt: &dispatch.WakeReceipt{
			Home: "current-home", Session: "current-session", Turn: turn, Request: turn,
		}}, true
	}
	enqueue(x, "first fresh receiver acknowledgement")
	drain(x, "%hq")
	if len(submissions) != 1 {
		t.Fatalf("expected first fresh submission, got %q", submissions)
	}
	enqueue(x, "independent after proof saturation")
	for range 3 {
		now = now.Add(time.Minute)
		ObserveWakeEvidence(now)
		drain(x, "%hq")
	}
	if len(submissions) != 2 || !strings.Contains(submissions[1], "independent after proof saturation") {
		t.Fatalf("accepted-but-unverified history froze unrelated native admission: submissions=%q queued=%d", submissions, queuedCount(t))
	}
	raw, err := os.ReadFile(acceptedArchivePath())
	if err != nil {
		t.Fatal("archive unavailable", err)
	}
	var overflow, expired bool
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		var entry acceptedArchiveRecord
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatal(err)
		}
		overflow = overflow || (entry.Reason == "proof-window-full" && entry.Accepted.Receipt.Request == "fresh-1" &&
			strings.Contains(entry.Accepted.Payload, "first fresh receiver acknowledgement"))
		expired = expired || (entry.Reason == "proof-window-expired" && strings.HasPrefix(entry.Accepted.Receipt.Request, "historical-"))
	}
	if !overflow || !expired {
		t.Fatalf("lost provenance for overflow or expired acknowledgements: overflow=%v expired=%v", overflow, expired)
	}
	if _, err := readIssue(issueProofArchived); err != nil {
		t.Fatal("preserved evidence was not independently announced", err)
	}
	if len(auditRecords(t, events.AuditEventWakeDelivered)) != 0 {
		t.Fatal("archival was relabelled as an independent delivery")
	}
}

func TestHWA015AcceptedProofCrashRetryKeepsOneDeliveryAudit(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := time.Unix(1700000000, 0)
	a := acceptedWake{Pane: "%hq", Batch: "#123abc", Payload: "verified wake · #123abc", Since: now.UnixNano(),
		Receipt: dispatch.WakeReceipt{Home: "current-home", Session: "current-session", Turn: "turn", Request: "request"}}
	body, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(acceptedWakeDir(), strings.Repeat("1", 64)+".json")
	witness := transcript.WakeWitness{Started: now.Add(time.Second), Delivered: now.Add(2 * time.Second)}
	for range 2 {
		// Leaving the exact retained receipt models exit after journal sync, before unlink.
		if err := state.WritePrivateFile(path, body); err != nil {
			t.Fatal(err)
		}
		if !confirmAcceptedWake(path, a, witness, now.Add(3*time.Second)) {
			t.Fatal("fixture could not confirm the accepted wake")
		}
	}
	rows := auditRecords(t, events.AuditEventWakeDelivered)
	if len(rows) != 1 || rows[0].OpID != strings.Repeat("1", 64) {
		t.Fatalf("one observed delivery must retain its exact op ID: %#v", rows)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("a confirmed receipt did not settle", err)
	}
}
