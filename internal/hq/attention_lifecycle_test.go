package hq

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/notify"
	"github.com/chenchaoyi/gtmux/internal/state"
)

func captureAttention(t *testing.T) *[]notify.Options {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	prior := wakeDesktopNotice
	t.Cleanup(func() { wakeDesktopNotice = prior })
	var requests []notify.Options
	wakeDesktopNotice = func(o notify.Options) bool {
		requests = append(requests, o)
		return true
	}
	return &requests
}

func requireAttentionSnapshot(t *testing.T, scope, want string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(state.NotifyDir(), "hq-"+scope+".attention"))
	if err != nil || string(data) != want {
		t.Fatalf("%s snapshot = %s %v, want %s", scope, data, err, want)
	}
}

func TestHND001WorkspaceAttentionChangesCopyWithoutChangingIdentity(t *testing.T) {
	sent := captureAttention(t)
	const now = 1000
	rows := []WorkspaceAttention{
		{MatterID: "matter-a", EvidenceID: "attempt-held"},
		{MatterID: "matter-b", EvidenceID: "independent"},
	}
	WatchWorkspaceAttention(now, rows)
	if len(*sent) != 2 || (*sent)[0].AttentionID != "workspace/matter-a" ||
		(*sent)[1].AttentionID != "workspace/matter-b" {
		t.Fatalf("independent matters shared an identity: %#v", *sent)
	}
	requireAttentionSnapshot(t, "workspace", `["workspace/matter-a","workspace/matter-b"]`)
	rows[0].EvidenceID, rows[0].NeedsUser = "decision", true
	WatchWorkspaceAttention(now+10, rows)
	if len(*sent) != 3 || (*sent)[2].AttentionID != "workspace/matter-a" ||
		!strings.Contains((*sent)[2].Message, "Answer the pending question") {
		t.Fatalf("new actionable status did not update the same notice: %#v", *sent)
	}
	WatchWorkspaceAttention(now+30, rows[1:])
	requireAttentionSnapshot(t, "workspace", `["workspace/matter-b"]`)
	if len(*sent) != 3 {
		t.Fatal("withdrawing one matter created another banner")
	}
}

func TestHND002RelayAttentionReplacesStatusAndKeepsARealBlocker(t *testing.T) {
	sent := captureAttention(t)
	const now = 1200
	rows := []RelayAttention{{ID: "report-a", Since: 1000}, {ID: "report-b", Since: 1001}}
	WatchRelayAttention(now, rows)
	if len(*sent) != 2 || (*sent)[0].AttentionID != "relay/report-a" || (*sent)[1].AttentionID != "relay/report-b" {
		t.Fatalf("two reports must be surfaced separately: %#v", *sent)
	}
	rows[0].ReservedAt = now + 3
	WatchRelayAttention(now+3, rows)
	rows[0].Unverified = true
	WatchRelayAttention(now+8, rows)
	if len(*sent) != 4 || (*sent)[2].AttentionID != "relay/report-a" || (*sent)[3].AttentionID != "relay/report-a" ||
		!strings.Contains((*sent)[2].Message, "has not claimed") || !strings.Contains((*sent)[3].Message, "could not verify") {
		t.Fatalf("relay state did not update in place: %#v", *sent)
	}
	WatchRelayAttention(now+10, rows[1:])
	requireAttentionSnapshot(t, "relay", `["relay/report-b"]`)
	WatchRelayAttention(now+wakeNoticeInterval+1, rows[1:])
	if len(*sent) != 5 || (*sent)[4].AttentionID != "relay/report-b" {
		t.Fatalf("remaining blocker was lost or replayed the resolved notice: %#v", *sent)
	}
	WatchRelayAttention(now+wakeNoticeInterval+2, nil)
	requireAttentionSnapshot(t, "relay", "[]")
}
