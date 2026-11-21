package hqnudge

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
)

func TestHWA006NativePositiveRefusalUsesDurableBoundedBackoff(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	const start = 1700000000000000000
	now := int64(start)
	var payloads, ids []string
	newDrainer := func() io {
		x := (&fake{}).io()
		x.nowNano = func() int64 { return now }
		x.nativeSubmit = func(_, payload, id string) (dispatch.Result, bool) {
			payloads = append(payloads, payload)
			ids = append(ids, id)
			return dispatch.Result{State: dispatch.StateRefusedWaiting, Submission: dispatch.SubmissionNotSubmitted, Evidence: "wake-loaded-idle-required"}, true
		}
		return x
	}
	x := newDrainer()
	enqueue(x, "a known unsent wake")
	for _, tick := range []struct {
		after time.Duration
		want  int
	}{{0, 1}, {time.Second, 1}, {5 * time.Second, 2}, {19 * time.Second, 2}, {20 * time.Second, 3}, {time.Minute, 3}} {
		now = start + int64(tick.after)
		drain(newDrainer(), "%hq")
		if len(ids) != tick.want {
			t.Fatalf("at %s called transport %d times, want %d: %v", tick.after, len(ids), tick.want, ids)
		}
	}
	for i := 1; i < len(ids); i++ {
		if payloads[i] != payloads[0] || !strings.HasPrefix(ids[i], ids[0]+"-r") || ids[i] == ids[i-1] {
			t.Fatalf("one logical batch and distinct transport attempts required: %q %q", payloads, ids)
		}
	}
	entries := readQueue()
	if len(entries) != 1 || !strings.HasSuffix(entries[0].Name(), wakeRefusedSuffix) {
		t.Fatal("exhaustion must retain original without pending replay", entries)
	}
	if _, err := readIssue(issueRetryExhausted); err != nil {
		t.Fatal("exhaustion not discoverable", err)
	}
	b, err := readNativeBytes(filepath.Join(queueDir(), entries[0].Name()))
	if err != nil || string(b) != "a known unsent wake" {
		t.Fatal("retained wrong input", string(b), err)
	}
}

func TestHWA006UnknownAndChangedTargetNeverResubmit(t *testing.T) {
	for _, reason := range []string{"unknown", "changed-target"} {
		t.Run(reason, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			now := time.Unix(1700000000, 0).UnixNano()
			x := (&fake{}).io()
			x.nowNano = func() int64 { return now }
			calls := 0
			x.nativeSubmit = func(_, _, _ string) (dispatch.Result, bool) {
				calls++
				res := dispatch.Result{State: dispatch.StateRefusedWaiting, Submission: dispatch.SubmissionNotSubmitted, Evidence: "local-proxy-unavailable"}
				if reason == "unknown" {
					res.Submission = dispatch.SubmissionUnknown
				}
				return res, true
			}
			enqueue(x, "never automatically reapply here")
			drain(x, "%hq")
			now += int64(time.Hour)
			x.identity = func(string) string { return strings.Repeat("a", 32) }
			drain(x, "%hq")
			if calls != 1 || queuedCount(t) != 0 {
				t.Fatalf("only original attempt allowed: calls=%d queued=%d", calls, queuedCount(t))
			}
			view := ReadHealth(now / int64(time.Second))
			if reason == "unknown" && view.Uncertain != 1 || reason == "changed-target" && view.Refused != 1 {
				t.Fatalf("outcome not retained under the correct state: %+v", view)
			}
		})
	}
}

// The review regression seeds the exact format written by older terminals.
func recordAttempt(claims []string, payload, id string, since int64) {
	_, _ = writeAttempt(legacyAttempt(claims, payload, id, since))
}

func TestHWA007GuardsPermitNoClientInput(t *testing.T) {
	for _, condition := range []string{"draft", "copy-mode", "busy", "unreadable", "unknown-target"} {
		t.Run(condition, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			f := &fake{draft: ""}
			f.draft = map[string]string{"draft": "unfinished user input"}[condition]
			f.inMode, f.unstructured = condition == "copy-mode", condition == "unreadable"
			x := f.io()
			x.busy = func(string) bool { return condition == "busy" }
			if condition == "unknown-target" {
				x.identity = func(string) string { return "" }
			}
			deliver(x, "%hq", "independent synthetic nudge")
			if f.pastes != 0 || f.enters != 0 || queuedCount(t) != 1 {
				t.Fatalf("unsafe legacy target did not preserve input: %s, paste=%d enter=%d", condition, f.pastes, f.enters)
			}
		})
	}
}

func TestHWA007CrashAfterLegacyEnterRetainsOriginal(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	f := &fake{ackBlind: true}
	x := f.io()
	send := x.enter
	x.enter = func(pane string) error {
		if err := send(pane); err != nil {
			t.Fatal(err)
		}
		panic("simulated process interrupted after successful Enter")
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("did not reach interrupted mutation")
			}
		}()
		deliver(x, "%hq", "before restart")
	}()
	for _, e := range readQueue() {
		if strings.HasSuffix(e.Name(), sendingSuffix) {
			stampAge(t, e.Name(), 2*time.Minute)
		}
	}
	deliver(f.io(), "%hq", "independent after restart")
	if len(f.sent) != 2 || legacyUncertainCount() != 2 || !strings.Contains(f.sent[1], "independent after restart") {
		t.Fatalf("restart replayed or lost original claims: %q", f.sent)
	}
	f.receipt = confirmOnly(strings.TrimSpace(idOf(t, f.sent[0])))
	drain(f.io(), "%hq")
	if f.pastes != 2 || legacyUncertainCount() != 1 {
		t.Fatalf("late receipt did not settle only its original batch: paste=%d held=%d", f.pastes, legacyUncertainCount())
	}
}
