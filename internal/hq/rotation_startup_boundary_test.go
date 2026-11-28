package hq

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/notify"
	"github.com/chenchaoyi/gtmux/internal/transcript"
)

func startupRotationRequest(t *testing.T) (rotationRequest, time.Time, time.Time) {
	t.Helper()
	now := time.Now().Truncate(time.Second)
	r := testRotationRequest(t, now)
	t.Setenv("CODEX_HOME", t.TempDir())
	r.SentAt, r.SubmittedAtNS = now.Unix(), now.Add(250*time.Nanosecond).UnixNano()
	r.Successor, r.SwitchedAt = "successor", now.Add(time.Second).Unix()
	// Use the normal generation check with a fixture from the verified pane.
	r.TargetStamp = "17:300:19"
	saveStamp, saveNotice := rotationTargetStamp, wakeDesktopNotice
	t.Cleanup(func() { rotationTargetStamp, wakeDesktopNotice = saveStamp, saveNotice })
	rotationTargetStamp = func(string) string { return r.TargetStamp }
	wakeDesktopNotice = func(notify.Options) bool { return true }
	if err := writeRotationFile(r, false); err != nil {
		t.Fatal(err)
	}
	return r, now.Add(2 * time.Second), time.Unix(rotationDeadline(r), 0)
}

func writeStartupRotationRollout(t *testing.T, dir, fileSID, metaSID string, start, complete time.Time) {
	t.Helper()
	var body strings.Builder
	fmt.Fprintf(&body, `{"type":"session_meta","payload":{"id":%q}}`+"\n", metaSID)
	if !start.IsZero() {
		fmt.Fprintf(&body, `{"timestamp":%q,"type":"event_msg","payload":{"type":"task_started","turn_id":"first"}}`+"\n", start.Format(time.RFC3339Nano))
		// Write complete records so the ordinary tail can be read, but it
		// can no longer reach the first start.
		filler := `{"type":"response_item","payload":{"type":"message","role":"assistant","text":"` + strings.Repeat("x", 4096) + `"}}` + "\n"
		body.WriteString(strings.Repeat(filler, 80))
	}
	if !complete.IsZero() {
		fmt.Fprintf(&body, `{"timestamp":%q,"type":"event_msg","payload":{"type":"task_complete","turn_id":"first"}}`+"\n", complete.Format(time.RFC3339Nano))
	}
	dir = filepath.Join(transcript.CodexHome(), dir)
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "rollout-time-"+fileSID+".jsonl"), []byte(body.String()), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestRotationConfirmsFromFirstStartOutsideTail(t *testing.T) {
	for _, name := range []string{"still-running", "completion-after-deadline"} {
		t.Run(name, func(t *testing.T) {
			r, start, deadline := startupRotationRequest(t)
			var complete time.Time
			wantTail := ""
			checkAt := deadline.Add(-time.Second)
			if name == "completion-after-deadline" {
				complete = deadline.Add(time.Second)
				wantTail = "task_complete"
				checkAt = deadline.Add(2 * time.Second)
			}
			writeStartupRotationRollout(t, "sessions/2026/11/28", r.Successor, r.Successor, start, complete)
			if kind, at := transcript.CodexLastTurnBoundary(r.Successor); kind != wantTail || !at.Equal(complete) {
				t.Fatalf("fixture tail = %q at %v, want %q at %v", kind, at, wantTail, complete)
			}
			pane := &spyPane{screen: box("")}
			for range 2 {
				advanceHQRotation(r, r.Pane, r.Agent, r.Successor, pane.io(), transcript.CodexLastTurnBoundary, checkAt)
			}
			if _, exists, err := readRotationRequest(); err != nil || exists {
				t.Fatalf("confirmed request still pending: exists=%v, err=%v", exists, err)
			}
			records := testRotationEvents(t)
			if len(records) != 2 || records[1].Event != events.AuditEventRotate ||
				records[1].AgentSession != r.Successor || records[1].PreviousAgentSession != r.Retiring {
				t.Fatalf("want exactly one confirmation for the bound successor: %+v", records)
			}
			if len(pane.pasted) != 0 || pane.enters != 0 {
				t.Fatalf("confirmation sent input: %+v", pane)
			}
		})
	}
}

func TestRotationRejectsUnprovenStartupBoundary(t *testing.T) {
	for _, name := range []string{
		"no-start", "before-submit", "after-deadline", "wrong-rollout-sid", "duplicate-rollout",
		"missing-history", "hook-only", "changed-live-sid", "missing-live-sid",
	} {
		t.Run(name, func(t *testing.T) {
			r, start, deadline := startupRotationRequest(t)
			liveSID, metaSID := r.Successor, r.Successor
			switch name {
			case "no-start", "missing-history", "hook-only":
				start = time.Time{}
			case "before-submit":
				start = time.Unix(0, r.SubmittedAtNS-1)
			case "after-deadline":
				start = deadline.Add(time.Nanosecond)
			case "wrong-rollout-sid":
				metaSID = "different"
			case "changed-live-sid":
				liveSID = "different"
			case "missing-live-sid":
				liveSID = ""
			}
			if name != "missing-history" {
				writeStartupRotationRollout(t, "sessions/2026/11/28", r.Successor, metaSID, start, time.Time{})
			}
			if name == "duplicate-rollout" {
				writeStartupRotationRollout(t, "archived_sessions", r.Successor, metaSID, start, time.Time{})
			}
			if name == "hook-only" {
				events.Append(events.Record{Event: "UserPromptSubmit", Ts: r.SwitchedAt, Pane: r.Pane,
					AgentKey: r.Agent, AgentSession: r.Successor})
			}
			pane := &spyPane{screen: box("")}
			advanceHQRotation(r, r.Pane, r.Agent, liveSID, pane.io(), transcript.CodexLastTurnBoundary, deadline.Add(-time.Second))
			if _, exists, err := readRotationRequest(); err != nil || exists != (name != "changed-live-sid") {
				t.Fatalf("unexpected pre-deadline request state: exists=%v, err=%v", exists, err)
			}
			for range 2 {
				advanceHQRotation(r, r.Pane, r.Agent, liveSID, pane.io(), transcript.CodexLastTurnBoundary, deadline)
			}
			if _, exists, err := readRotationRequest(); err != nil || exists {
				t.Fatalf("unconfirmed request did not settle: exists=%v, err=%v", exists, err)
			}
			records := testRotationEvents(t)
			wantRecords := 2
			if name == "hook-only" {
				wantRecords++
			}
			if len(records) != wantRecords || records[len(records)-1].Event != events.AuditEventRotateFailed {
				t.Fatalf("want one failure with no confirmation: %+v", records)
			}
			if len(pane.pasted) != 0 || pane.enters != 0 {
				t.Fatalf("failed confirmation sent input: %+v", pane)
			}
		})
	}
}
