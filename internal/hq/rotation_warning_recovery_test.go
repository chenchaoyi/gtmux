package hq

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/hostplatform"
	"github.com/chenchaoyi/gtmux/internal/state"
)

// A warning attempt is persisted on every host. Its desktop queue is only a
// supported transport on hosts that advertise that capability; do not skip the
// request/timing/input assertions on headless CI.
func expectedWarningFiles(n int) int {
	if hostplatform.Current().DesktopNotifications {
		return n
	}
	return 0
}

func warningFiles(t *testing.T) int {
	t.Helper()
	fs, err := os.ReadDir(state.NotifyDir())
	if os.IsNotExist(err) {
		return 0
	}
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, f := range fs {
		if !strings.HasSuffix(f.Name(), ".json") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(state.NotifyDir(), f.Name()))
		if err != nil {
			t.Fatal(err)
		}
		var v map[string]any
		if json.Unmarshal(b, &v) != nil || v["kind"] != "attention" {
			t.Fatal("warning did not use independent attention consumer", string(b))
		}
		count++
	}
	return count
}
func TestHRW016WarningThenRecoveryKeepsRequestAndEveryGuard(t *testing.T) {
	for _, mode := range []string{"ready", "draft", "active", "waiting", "unfinished", "copy-mode", "unreadable", "changed-session", "changed-agent"} {
		t.Run(mode, func(t *testing.T) {
			now := time.Now()
			r := testRotationRequest(t, now)
			p := &spyPane{screen: box("")}
			boundary := testBoundary("task_complete", now.Add(-time.Minute))
			advanceHQRotation(r, "%6", "codex", "old-session", p.io(), boundary, now.Add(time.Second), "proof-expired")
			first, ok, err := readRotationRequest()
			if err != nil || !ok {
				t.Fatal("first unknown dropped request", err)
			}
			advanceHQRotation(r, "%6", "codex", "old-session", p.io(), boundary, now.Add(61*time.Second), "proof-expired")
			warned, ok, err := readRotationRequest()
			if err != nil || !ok || warned.CreatedAt != r.CreatedAt || warned.IdentityUnavailableAt != first.IdentityUnavailableAt || warned.SentAt != 0 || warned.IdentityWarnedAt != now.Add(61*time.Second).Unix() {
				t.Fatal("60-second warning retired or extended original pending request", warned, ok, err)
			}
			if warningFiles(t) != expectedWarningFiles(1) || len(p.pasted) != 0 || p.enters != 0 {
				t.Fatal("warning missing, repeated or authorized input")
			}
			for _, e := range testRotationEvents(t) {
				if e.Event == events.AuditEventRotateFailed {
					t.Fatal("warning was recorded as terminal failure")
				}
			}
			// Reconstruct from disk and change the cause. This is still the same request,
			// not another warning and not another thirty-minute grant.
			advanceHQRotation(warned, "%6", "codex", "old-session", p.io(), boundary, now.Add(62*time.Second), "bridge-proof-unavailable")
			if warningFiles(t) != expectedWarningFiles(1) {
				t.Fatal("cause change/reload repeated warning")
			}
			at := now.Add(63 * time.Second)
			agent, sid := "codex", "old-session"
			switch mode {
			case "draft":
				p.screen = box("user draft")
			case "active":
				if err := state.WriteMarker(state.ActivePath("%6"), "old-session"); err != nil {
					t.Fatal(err)
				}
			case "waiting":
				if err := state.WriteMarker(state.WaitingPath("%6"), "old-session"); err != nil {
					t.Fatal(err)
				}
			case "unfinished":
				boundary = testBoundary("task_started", now)
			case "copy-mode":
				p.inMode = true
			case "unreadable":
				p.screen = "no composer"
			case "changed-session":
				sid = "real-new-session"
			case "changed-agent":
				agent = "claude"
			}
			advanceHQRotation(r, "%6", agent, sid, p.io(), boundary, at)
			if mode == "ready" {
				if len(p.pasted) != 1 || p.pasted[0] != "/new" || p.enters != 1 {
					t.Fatal("exact recovered safe identity could not continue original request", p)
				}
			} else if len(p.pasted) != 0 || p.enters != 0 {
				t.Fatal("warning/recovery bypassed original guard", mode, p)
			}
			if mode == "changed-session" || mode == "changed-agent" {
				if _, exists, _ := readRotationRequest(); exists {
					t.Fatal("real identity change was treated as merely unavailable")
				}
				return
			}
			p.screen = box("")
			p.inMode = false
			state.Remove(state.ActivePath("%6"))
			state.Remove(state.WaitingPath("%6"))
			advanceHQRotation(r, "%6", "codex", "old-session", p.io(), testBoundary("task_complete", now.Add(-time.Minute)), at.Add(time.Second))
			if len(p.pasted) != 1 || p.enters != 1 {
				t.Fatal("guard recovery repeated reset or permanently lost original request", p)
			}
		})
	}
}
func TestHRW016RepeatedUnknownNeverExtendsExpiryOrRepeatsWarning(t *testing.T) {
	for _, mode := range []string{"still-unknown", "recovered-at-expiry", "expiry-before-warning"} {
		t.Run(mode, func(t *testing.T) {
			now := time.Now()
			r := testRotationRequest(t, now)
			p := &spyPane{screen: box("")}
			boundary := testBoundary("task_complete", now.Add(-time.Minute))
			firstAt := now.Add(time.Second)
			if mode == "expiry-before-warning" {
				firstAt = now.Add(30*time.Minute - 30*time.Second)
			}
			advanceHQRotation(r, "%6", "codex", "", p.io(), boundary, firstAt)
			if mode != "expiry-before-warning" {
				advanceHQRotation(r, "%6", "codex", "", p.io(), boundary, now.Add(61*time.Second))
				if _, exists, _ := readRotationRequest(); !exists {
					t.Fatal("attention retired pending request")
				}
				for _, s := range []int{62, 100, 1700} {
					cur, exists, err := readRotationRequest()
					if err != nil || !exists {
						t.Fatal("pending missing", err)
					}
					advanceHQRotation(cur, "%6", "codex", "old-session", p.io(), boundary, now.Add(time.Duration(s)*time.Second), "arbitrary private details\n/home/private")
					cur, exists, err = readRotationRequest()
					if err != nil || !exists || cur.CreatedAt != r.CreatedAt || cur.IdentityUnavailableAt != firstAt.Unix() || cur.IdentityCause != "unknown" || cur.IdentityWarnedAt != now.Add(61*time.Second).Unix() {
						t.Fatal("diagnostic changed original deadline/identity", cur, err)
					}
				}
				if warningFiles(t) != expectedWarningFiles(1) {
					t.Fatal("repeat/reload/cause change republished warning")
				}
			}
			sid := ""
			if mode == "recovered-at-expiry" {
				sid = "old-session"
			}
			advanceHQRotation(r, "%6", "codex", sid, p.io(), boundary, now.Add(30*time.Minute))
			if _, exists, _ := readRotationRequest(); exists || len(p.pasted) != 0 || p.enters != 0 {
				t.Fatal("original expiry extended or authorized reset")
			}
			failed := 0
			for _, e := range testRotationEvents(t) {
				if e.Event == events.AuditEventRotateFailed {
					failed++
				}
			}
			if failed != 1 {
				t.Fatal("expiry not exactly one terminal failure", failed)
			}
			n := warningFiles(t)
			advanceHQRotation(r, "%6", "codex", "old-session", p.io(), boundary, now.Add(31*time.Minute))
			if warningFiles(t) != n || len(p.pasted) != 0 {
				t.Fatal("stale/late recovery revived expired request")
			}
		})
	}
}
func TestHRW016FailedPublicationIsNotAReasonToReplayOrBlockRecovery(t *testing.T) {
	now := time.Now()
	r := testRotationRequest(t, now)
	p := &spyPane{screen: box("")}
	boundary := testBoundary("task_complete", now.Add(-time.Minute))
	if err := os.MkdirAll(filepath.Dir(state.NotifyDir()), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(state.NotifyDir(), []byte("blocked publication"), 0600); err != nil {
		t.Fatal(err)
	}
	advanceHQRotation(r, "%6", "codex", "", p.io(), boundary, now.Add(time.Second))
	advanceHQRotation(r, "%6", "codex", "", p.io(), boundary, now.Add(61*time.Second))
	cur, exists, err := readRotationRequest()
	if err != nil || !exists || cur.CreatedAt != r.CreatedAt || cur.IdentityWarnedAt != now.Add(61*time.Second).Unix() {
		t.Fatal("notification failure dropped pending or warning attempt", err)
	}
	if err := os.Remove(state.NotifyDir()); err != nil {
		t.Fatal(err)
	}
	advanceHQRotation(cur, "%6", "codex", "", p.io(), boundary, now.Add(62*time.Second))
	if warningFiles(t) != 0 || len(p.pasted) != 0 {
		t.Fatal("unknown notification outcome was republished or authorized input")
	}
	advanceHQRotation(cur, "%6", "codex", "old-session", p.io(), boundary, now.Add(63*time.Second))
	if len(p.pasted) != 1 || p.enters != 1 {
		t.Fatal("failed warning prevented guarded recovery")
	}
	// A genuinely new request has its own warning ownership, not a reused old flag.
	sent, _, _ := readRotationRequest()
	failQueuedRotation(sent, "fixture terminal close", now.Add(64*time.Second).Unix())
	next := r
	next.CreatedAt = now.Add(65 * time.Second).Unix()
	if _, ok, reason := queueHQRotation(next); !ok {
		t.Fatal(reason)
	}
	advanceHQRotation(next, "%6", "codex", "", p.io(), boundary, now.Add(66*time.Second))
	advanceHQRotation(next, "%6", "codex", "", p.io(), boundary, now.Add(126*time.Second))
	cur, exists, err = readRotationRequest()
	if err != nil || !exists || cur.CreatedAt != next.CreatedAt || cur.IdentityWarnedAt != now.Add(126*time.Second).Unix() || warningFiles(t) != expectedWarningFiles(2) {
		t.Fatal("new request warning inherited old ownership")
	}
}

func TestHRW016UnknownSourcesAndReasonChangesKeepOriginalOwnership(t *testing.T) {
	for _, mode := range []string{"agent-missing", "session-missing", "active-conflict"} {
		t.Run(mode, func(t *testing.T) {
			now := time.Now()
			r := testRotationRequest(t, now)
			p := &spyPane{screen: box("")}
			boundary := testBoundary("task_complete", now.Add(-time.Minute))
			agent, sid := "codex", "old-session"
			if mode == "agent-missing" {
				agent = ""
				sid = ""
			}
			if mode == "session-missing" {
				sid = ""
			}
			if mode == "active-conflict" {
				if err := state.WriteMarker(state.ActivePath("%6"), "other-active"); err != nil {
					t.Fatal(err)
				}
			}
			advanceHQRotation(r, "%6", agent, sid, p.io(), boundary, now.Add(time.Second))
			first, exists, err := readRotationRequest()
			if err != nil || !exists {
				t.Fatal("unavailable evidence was falsely settled as changed identity")
			}
			advanceHQRotation(r, "%6", agent, sid, p.io(), boundary, now.Add(30*time.Second))
			advanceHQRotation(r, "%6", agent, sid, p.io(), boundary, now.Add(61*time.Second))
			current, exists, err := readRotationRequest()
			if err != nil || !exists || current.IdentityUnavailableAt != first.IdentityUnavailableAt || current.CreatedAt != r.CreatedAt || current.IdentityWarnedAt != now.Add(61*time.Second).Unix() || warningFiles(t) != expectedWarningFiles(1) || len(p.pasted) != 0 || p.enters != 0 {
				t.Fatal("unknown sources changed timing, failed or granted input", current)
			}
			advanceHQRotation(r, "%6", "codex", "old-session", p.io(), boundary, now.Add(62*time.Second), "proof-expired")
			advanceHQRotation(r, "%6", "codex", "old-session", p.io(), boundary, now.Add(63*time.Second), "arbitrary private text")
			current, exists, err = readRotationRequest()
			if err != nil || !exists || current.IdentityCause != "unknown" || current.IdentityUnavailableAt != first.IdentityUnavailableAt || warningFiles(t) != expectedWarningFiles(1) || len(p.pasted) != 0 {
				t.Fatal("reason change extended time, repeated warning or granted input", current)
			}
			state.Remove(state.ActivePath("%6"))
			advanceHQRotation(r, "%6", "codex", "old-session", p.io(), boundary, now.Add(64*time.Second))
			if len(p.pasted) != 1 || p.enters != 1 {
				t.Fatal("exact recovery lost the original safe request")
			}
		})
	}
}
