package hq

import (
	"path/filepath"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/notify"
	"github.com/chenchaoyi/gtmux/internal/state"
)

func TestHND010WorkspaceDecisionStaysQuietAcrossCooldownAndRefresh(t *testing.T) {
	sent := captureAttention(t)
	const now = 1000
	rows := []WorkspaceAttention{
		{MatterID: "decision", EvidenceID: "question-one", NeedsUser: true},
		{MatterID: "blocker", EvidenceID: "verified-held-attempt"},
	}
	WatchWorkspaceAttention(now, rows)
	if len(*sent) != 2 || (*sent)[0].AttentionID != "workspace/blocker" || (*sent)[1].AttentionID != "workspace/decision" {
		t.Fatalf("expected an initial notice for each matter: %#v", *sent)
	}
	for _, scan := range []int64{now + wakeNoticeInterval - 1, now + wakeNoticeInterval, now + wakeNoticeInterval + 1} {
		WatchWorkspaceAttention(scan, rows)
	}
	if len(*sent) != 3 || (*sent)[2].AttentionID != "workspace/blocker" {
		t.Fatalf("only the continuing blocker should repeat after cooldown: %#v", *sent)
	}
	requireAttentionSnapshot(t, "workspace", `["workspace/blocker","workspace/decision"]`)
	const changedAt = now + wakeNoticeInterval + 2
	rows[0].EvidenceID = "question-two"
	WatchWorkspaceAttention(changedAt, rows)
	if len(*sent) != 4 || (*sent)[3].AttentionID != "workspace/decision" {
		t.Fatalf("new question should use the same matter identity: %#v", *sent)
	}
	WatchWorkspaceAttention(changedAt+1, rows[1:])
	requireAttentionSnapshot(t, "workspace", `["workspace/blocker"]`)
	if len(*sent) != 4 {
		t.Fatal("resolution should withdraw the identity without another notice")
	}
}

func TestHND010WorkspaceDecisionReadsLegacyTimestampAfterRestart(t *testing.T) {
	sent := captureAttention(t)
	path := filepath.Join(state.Dir(), "hqwake", "notices-workspace.json")
	if err := state.WritePrivateFile(path, []byte(`{"decision@decision@question-one":1000}`)); err != nil {
		t.Fatal(err)
	}
	rows := []WorkspaceAttention{{MatterID: "decision", EvidenceID: "question-one", NeedsUser: true}}
	for _, scan := range []int64{1000 + wakeNoticeInterval, 1000 + 2*wakeNoticeInterval, 999} {
		WatchWorkspaceAttention(scan, rows)
	}
	if len(*sent) != 0 {
		t.Fatalf("a persisted question repeated after restart or clock correction: %#v", *sent)
	}
	requireAttentionSnapshot(t, "workspace", `["workspace/decision"]`)
}

func TestHND011WorkspaceDecisionRetriesKnownQueueRefusal(t *testing.T) {
	sent := captureAttention(t)
	admitted := false
	wakeDesktopNotice = func(o notify.Options) bool {
		*sent = append(*sent, o)
		return admitted
	}
	const now = 1000
	rows := []WorkspaceAttention{{MatterID: "decision", EvidenceID: "question-one", NeedsUser: true}}
	WatchWorkspaceAttention(now, rows)
	admitted = true
	WatchWorkspaceAttention(now+wakeNoticeInterval-1, rows)
	if len(*sent) != 1 {
		t.Fatalf("a refused request retried before cooldown: %#v", *sent)
	}
	WatchWorkspaceAttention(now+wakeNoticeInterval, rows)
	WatchWorkspaceAttention(now+2*wakeNoticeInterval, rows)
	if len(*sent) != 2 || (*sent)[0].AttentionID != "workspace/decision" || (*sent)[1].AttentionID != "workspace/decision" {
		t.Fatalf("only a confirmed refusal should retry the same identity: %#v", *sent)
	}
	requireAttentionSnapshot(t, "workspace", `["workspace/decision"]`)
	journal, gap := events.ReadSince(0)
	if gap || len(journal) != 2 || journal[0].Event != "gtmux:wake-degraded" || journal[1].Event != "gtmux:wake-degraded" {
		t.Fatalf("expected independent journal entries for both attempts: %#v", journal)
	}
}
