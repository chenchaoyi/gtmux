package hq

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/notify"
	"github.com/chenchaoyi/gtmux/internal/state"
)

func TestHND014WorkspaceDecisionWaitsForDurableReservation(t *testing.T) {
	sent := captureAttention(t)
	const now = 1000
	const key = "decision@decision@question-one"
	rows := []WorkspaceAttention{{MatterID: "decision", EvidenceID: "question-one", NeedsUser: true}}
	writer := writeWakeNoticeLedger
	t.Cleanup(func() { writeWakeNoticeLedger = writer })
	writeWakeNoticeLedger = func(path string, data []byte) error {
		if err := writer(path, data); err != nil {
			t.Fatal(err)
		}
		return fs.ErrPermission
	}
	WatchWorkspaceAttention(now, rows)
	if len(*sent) != 0 {
		t.Fatalf("failed reservation called the queue: %#v", *sent)
	}
	requireAttentionSnapshot(t, "workspace", `["workspace/decision"]`)
	writeWakeNoticeLedger = writer
	wakeDesktopNotice = func(o notify.Options) bool {
		path := filepath.Join(state.Dir(), "hqwake", "notices-workspace.json")
		ledger, err := readWakeNotices(path)
		if err != nil || ledger[workspaceNoticePendingPrefix+key] != now+1 {
			t.Fatalf("the durable reservation should precede the queue call: %v, %v", ledger, err)
		}
		*sent = append(*sent, o)
		return true
	}
	WatchWorkspaceAttention(now+1, rows)
	WatchWorkspaceAttention(now+wakeNoticeInterval+1, rows)
	if len(*sent) != 1 || (*sent)[0].AttentionID != "workspace/decision" {
		t.Fatalf("storage recovery should admit the unsubmitted question once: %#v", *sent)
	}
}

func TestHND014WorkspaceDecisionRepairsKnownRefusal(t *testing.T) {
	sent := captureAttention(t)
	const now = 1000
	rows := []WorkspaceAttention{{MatterID: "decision", EvidenceID: "question-one", NeedsUser: true}}
	writer := writeWakeNoticeLedger
	t.Cleanup(func() { writeWakeNoticeLedger = writer })
	var writes int
	writeWakeNoticeLedger = func(path string, data []byte) error {
		writes++
		if writes == 2 {
			return fs.ErrPermission
		}
		return writer(path, data)
	}
	wakeDesktopNotice = func(o notify.Options) bool {
		*sent = append(*sent, o)
		return false
	}
	WatchWorkspaceAttention(now, rows)
	if len(*sent) != 1 || writes != 2 {
		t.Fatalf("expected a reservation and one refusal write: %d writes, %#v", writes, *sent)
	}
	path := filepath.Join(state.Dir(), "hqwake", "notices-workspace.json")
	ledger, err := readWakeNotices(path)
	const key = "decision@decision@question-one"
	if err != nil || ledger[workspaceNoticePendingPrefix+key] != now {
		t.Fatalf("the reservation should survive a failed result write: %v, %v", ledger, err)
	}
	writeWakeNoticeLedger = writer
	wakeDesktopNotice = func(o notify.Options) bool {
		*sent = append(*sent, o)
		return true
	}
	WatchWorkspaceAttention(now+wakeNoticeInterval-1, rows)
	ledger, err = readWakeNotices(path)
	if err != nil || len(*sent) != 1 || ledger[key] != -now || ledger[workspaceNoticePendingPrefix+key] != 0 {
		t.Fatalf("storage recovery should retain the original refusal time without publishing: %v, %v, %#v", ledger, err, *sent)
	}
	WatchWorkspaceAttention(now+wakeNoticeInterval, rows)
	WatchWorkspaceAttention(now+2*wakeNoticeInterval, rows)
	if len(*sent) != 2 || (*sent)[1].AttentionID != "workspace/decision" {
		t.Fatalf("the confirmed refusal should retry once at its original cooldown: %#v", *sent)
	}
	requireAttentionSnapshot(t, "workspace", `["workspace/decision"]`)
	journal, gap := events.ReadSince(0)
	if gap || len(journal) != 2 || journal[0].Event != "gtmux:wake-degraded" || journal[1].Event != "gtmux:wake-degraded" {
		t.Fatalf("the actual attempts should retain their audits: %#v", journal)
	}
}

func TestHND014WorkspaceDecisionRepairsKnownAdmission(t *testing.T) {
	sent := captureAttention(t)
	const now = 1000
	const key = "decision@decision@question-one"
	rows := []WorkspaceAttention{{MatterID: "decision", EvidenceID: "question-one", NeedsUser: true}}
	writer := writeWakeNoticeLedger
	t.Cleanup(func() { writeWakeNoticeLedger = writer })
	writeWakeNoticeLedger = func(path string, data []byte) error {
		if len(*sent) != 0 {
			return fs.ErrPermission
		}
		return writer(path, data)
	}
	WatchWorkspaceAttention(now, rows)
	path := filepath.Join(state.Dir(), "hqwake", "notices-workspace.json")
	ledger, err := readWakeNotices(path)
	if err != nil || len(*sent) != 1 || ledger[workspaceNoticePendingPrefix+key] != now {
		t.Fatalf("expected a durable reservation and one admitted request: %v, %v, %#v", ledger, err, *sent)
	}
	writeWakeNoticeLedger = writer
	WatchWorkspaceAttention(now+wakeNoticeInterval, rows)
	ledger, err = readWakeNotices(path)
	if err != nil || len(*sent) != 1 || ledger[key] != now || ledger[workspaceNoticePendingPrefix+key] != 0 {
		t.Fatalf("the known admission should be recorded without another submission: %v, %v, %#v", ledger, err, *sent)
	}
}

func TestHND014WorkspaceDecisionHoldsOnlyUnknownRequestAfterRestart(t *testing.T) {
	sent := captureAttention(t)
	accept := wakeDesktopNotice
	const now = 1000
	rows := []WorkspaceAttention{
		{MatterID: "first", EvidenceID: "question-one", NeedsUser: true},
		{MatterID: "second", EvidenceID: "question-two", NeedsUser: true},
	}
	interrupted := errors.New("simulated interruption at queue call")
	wakeDesktopNotice = func(o notify.Options) bool {
		accept(o)
		panic(interrupted)
	}
	func() {
		defer func() {
			if caught := recover(); caught != interrupted {
				t.Fatalf("expected interruption at the first queue call, got %v", caught)
			}
		}()
		WatchWorkspaceAttention(now, rows)
	}()
	workspaceNoticeDelivery.path = "" // A fresh process knows only the ledger.
	wakeDesktopNotice = accept
	WatchWorkspaceAttention(now+wakeNoticeInterval+1, rows)
	WatchWorkspaceAttention(now+2*wakeNoticeInterval+1, rows)
	if len(*sent) != 2 || (*sent)[0].AttentionID != "workspace/first" || (*sent)[1].AttentionID != "workspace/second" {
		t.Fatalf("only the known unsubmitted second question should proceed: %#v", *sent)
	}
	path := filepath.Join(state.Dir(), "hqwake", "notices-workspace.json")
	ledger, err := readWakeNotices(path)
	if err != nil || ledger[workspaceNoticePendingPrefix+"first@decision@question-one"] != now {
		t.Fatalf("lost the unknown queue result: %v, %v", ledger, err)
	}
	requireAttentionSnapshot(t, "workspace", `["workspace/first","workspace/second"]`)
	logs, err := filepath.Glob(filepath.Join(state.Dir(), "logs", "*.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	diagnostics := 0
	for _, log := range logs {
		data, err := os.ReadFile(log)
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(string(data), "\n") {
			if strings.Contains(line, `"event":"act.wake.notice.recovery"`) && strings.Contains(line, `"reason":"first@decision@question-one"`) {
				diagnostics++
			}
		}
	}
	if diagnostics != 1 {
		t.Fatalf("expected one diagnostic for the exact unknown request, got %d", diagnostics)
	}
}

func TestHND013WorkspaceBlockerRetainsSlotAfterReservationFailure(t *testing.T) {
	sent := captureAttention(t)
	const now = 1000
	rows := []WorkspaceAttention{{MatterID: "blocker", EvidenceID: "held-attempt"}}
	WatchWorkspaceAttention(now, rows)
	for n := range 8 {
		rows = append(rows, WorkspaceAttention{MatterID: fmt.Sprintf("question-%d", n), EvidenceID: "pending", NeedsUser: true})
	}
	writer := writeWakeNoticeLedger
	t.Cleanup(func() { writeWakeNoticeLedger = writer })
	writeWakeNoticeLedger = func(string, []byte) error { return fs.ErrPermission }
	WatchWorkspaceAttention(now+wakeNoticeInterval, rows)
	WatchWorkspaceAttention(now+wakeNoticeInterval+1, rows)
	writeWakeNoticeLedger = writer
	path := filepath.Join(state.Dir(), "hqwake", "notices-workspace.json")
	if err := state.WritePrivateFile(path, []byte("{")); err != nil {
		t.Fatal(err)
	}
	WatchWorkspaceAttention(now+wakeNoticeInterval+2, rows)
	if len(*sent) != 2 || (*sent)[1].AttentionID != "workspace/blocker" {
		t.Fatalf("eight failed questions consumed the due blocker's slot or cooldown: %#v", *sent)
	}
}

func TestHND013WakeNoticeWriterKeepsLedgerWithinReadBudget(t *testing.T) {
	captureAttention(t)
	path := filepath.Join(state.Dir(), "hqwake", "notices-workspace.json")
	if err := saveWakeNotices(path, map[string]int64{"decision": 1000}); err != nil {
		t.Fatal(err)
	}
	if err := saveWakeNotices(path, map[string]int64{strings.Repeat("x", wakeNoticeLedgerLimit): 1100}); !errors.Is(err, errWakeNoticeLedgerTooLarge) {
		t.Fatalf("expected the writer's size guard, got %v", err)
	}
	ledger, err := readWakeNotices(path)
	if err != nil || len(ledger) != 1 || ledger["decision"] != 1000 {
		t.Fatalf("oversized write disturbed the prior ledger: %v, %v", ledger, err)
	}
}
