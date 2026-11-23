package hq

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/notify"
	"github.com/chenchaoyi/gtmux/internal/state"
)

func TestHND012WorkspaceDecisionPreservesUnreadableLedger(t *testing.T) {
	sent := captureAttention(t)
	const now = 1000
	rows := []WorkspaceAttention{
		{MatterID: "decision", EvidenceID: "question-one", NeedsUser: true},
		{MatterID: "blocker", EvidenceID: "held-attempt"},
	}
	WatchWorkspaceAttention(now, rows)
	if len(*sent) != 2 {
		t.Fatalf("expected both initial notices: %#v", *sent)
	}
	path := filepath.Join(state.Dir(), "hqwake", "notices-workspace.json")
	good, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	const damaged = "{"
	if err := state.WritePrivateFile(path, []byte(damaged)); err != nil {
		t.Fatal(err)
	}
	WatchWorkspaceAttention(now+wakeNoticeInterval, rows)
	if len(*sent) != 3 || (*sent)[2].AttentionID != "workspace/blocker" {
		t.Fatalf("ledger failure should hold the question and still alert the blocker: %#v", *sent)
	}
	stillDamaged, err := os.ReadFile(path)
	if err != nil || string(stillDamaged) != damaged {
		t.Fatalf("an unreadable ledger was replaced: %q, %v", stillDamaged, err)
	}
	if err := state.WritePrivateFile(path, good); err != nil {
		t.Fatal(err)
	}
	WatchWorkspaceAttention(now+wakeNoticeInterval+1, rows[:1])
	if len(*sent) != 3 {
		t.Fatalf("the original durable record should still prevent replay: %#v", *sent)
	}
	requireAttentionSnapshot(t, "workspace", `["workspace/decision"]`)
}

func TestHND012WorkspaceDecisionWaitsForDurableReservation(t *testing.T) {
	sent := captureAttention(t)
	const now = 1000
	rows := []WorkspaceAttention{
		{MatterID: "decision", EvidenceID: "question-one", NeedsUser: true},
		{MatterID: "blocker", EvidenceID: "held-attempt"},
	}
	writer := writeWakeNoticeLedger
	t.Cleanup(func() { writeWakeNoticeLedger = writer })
	writeWakeNoticeLedger = func(string, []byte) error { return fs.ErrPermission }
	WatchWorkspaceAttention(now, rows)
	if len(*sent) != 1 || (*sent)[0].AttentionID != "workspace/blocker" {
		t.Fatalf("a failed reservation should hold only the question: %#v", *sent)
	}
	requireAttentionSnapshot(t, "workspace", `["workspace/blocker","workspace/decision"]`)
	writeWakeNoticeLedger = writer
	wakeDesktopNotice = func(o notify.Options) bool {
		path := filepath.Join(state.Dir(), "hqwake", "notices-workspace.json")
		ledger, err := readWakeNotices(path)
		if err != nil || ledger["decision@decision@question-one"] != now+1 {
			t.Fatalf("the reservation should precede the queue call: %v, %v", ledger, err)
		}
		*sent = append(*sent, o)
		return true
	}
	WatchWorkspaceAttention(now+1, rows[:1])
	WatchWorkspaceAttention(now+wakeNoticeInterval+1, rows[:1])
	if len(*sent) != 2 || (*sent)[1].AttentionID != "workspace/decision" {
		t.Fatalf("storage recovery should admit the question once: %#v", *sent)
	}
}

func TestHND012WorkspaceDecisionRetainsReservationIfRefusalWriteFails(t *testing.T) {
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
	if err != nil || ledger["decision@decision@question-one"] != now {
		t.Fatalf("the reservation should survive a failed result write: %v, %v", ledger, err)
	}
	writeWakeNoticeLedger = writer
	workspaceNoticeDelivery.path = "" // Only persisted state remains after restart.
	WatchWorkspaceAttention(now+2*wakeNoticeInterval, rows)
	if len(*sent) != 1 {
		t.Fatalf("a reservation with an unrecorded result should hold replay: %#v", *sent)
	}
	requireAttentionSnapshot(t, "workspace", `["workspace/decision"]`)
	journal, gap := events.ReadSince(0)
	if gap || len(journal) != 1 || journal[0].Event != "gtmux:wake-degraded" {
		t.Fatalf("the actual attempt should retain its audit: %#v", journal)
	}
}
