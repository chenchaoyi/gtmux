package transcript

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestCodexLastMessageTimeIgnoresMaintenanceRecords(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", "")
	const sid = "idle-history"
	complete := `{"timestamp":"2026-09-28T09:07:00Z","type":"event_msg","payload":{"type":"task_complete"}}`
	want := time.Date(2026, 9, 28, 9, 7, 0, 0, time.UTC).Unix()
	maintenance := []string{
		`{"timestamp":"2026-10-16T06:33:50Z","type":"event_msg","payload":{"type":"thread_settings_applied"}}`,
		`{"timestamp":"2026-10-16T06:33:51Z","type":"event_msg","payload":{"type":"token_count"}}`,
		`{"timestamp":"2026-10-16T06:33:52Z","type":"session_meta","payload":{"id":"idle-history"}}`,
		`{"timestamp":"2026-10-16T06:33:53Z","type":"turn_context","payload":{}}`,
		`{"timestamp":"2026-10-16T06:33:54Z","type":"response_item","payload":{"type":"message","role":"developer","content":[]}}`,
		`{"timestamp":"2026-10-16T06:33:55Z","type":"unknown","payload":{"type":"task_complete","timestamp":"2099-01-01T00:00:00Z"}}`,
		`{"timestamp":"2026-10-16T06:33:56Z","type":"event_msg","payload":{"type":"thread_settings_applied"`,
	}
	writeCodexLog(t, home, sid, append([]string{complete}, maintenance...))
	for _, read := range []func(string) int64{
		func(id string) int64 { return LastMessageTime("codex", id) },
		NewCodexPathScope().LastMessageTime,
	} {
		if got := read(sid); got != want {
			t.Fatalf("maintenance advanced an old completion: got %d, want %d", got, want)
		}
	}
	writeCodexLog(t, home, "no-activity", maintenance)
	if got := LastMessageTime("codex", "no-activity"); got != 0 {
		t.Fatalf("maintenance-only rollout claimed activity at %d", got)
	}
}

func TestCodexActivityTimeFollowsTypedRecords(t *testing.T) {
	const stamp = "2026-10-16T10:00:00Z"
	want := time.Date(2026, 10, 16, 10, 0, 0, 0, time.UTC).Unix()
	for _, payload := range []string{
		`"type":"event_msg","payload":{"type":"user_message","message":"question"}`,
		`"type":"event_msg","payload":{"type":"agent_message","message":"answer"}`,
		`"type":"event_msg","payload":{"type":"task_started"}`,
		`"type":"event_msg","payload":{"type":"task_complete"}`,
		`"type":"event_msg","payload":{"type":"turn_aborted"}`,
		`"type":"response_item","payload":{"type":"message","role":"user","content":[]}`,
		`"type":"response_item","payload":{"type":"message","role":"assistant","content":[]}`,
	} {
		t.Run(payload, func(t *testing.T) {
			// Deliberately reorder the timestamp and include a conflicting nested one.
			line := fmt.Sprintf(`{%s,"timestamp":%q,"extra":{"timestamp":"2099-01-01T00:00:00Z"}}`, payload, stamp)
			if got := messageTimeIn("Codex", line, true); got != want {
				t.Fatalf("typed record time = %d, want %d", got, want)
			}
		})
	}
	// The first-record timestamp still defines session age; this fix is about last activity.
	meta := `{"timestamp":"2026-09-01T00:00:00Z","type":"session_meta","payload":{}}`
	if got := messageTimeIn("codex", meta, false); got != time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC).Unix() {
		t.Fatalf("session age changed: %d", got)
	}
	if got := messageTimeIn("codex", strings.TrimSuffix(meta, "}"), true); got != 0 {
		t.Fatalf("partial metadata became activity: %d", got)
	}
}
