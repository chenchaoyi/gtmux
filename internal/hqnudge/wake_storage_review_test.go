package hqnudge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/state"
)

func TestHWA017UnboundLegacyRefusalDoesNotEnterNewTarget(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	f := &fake{}
	x := f.io()
	enqueueKeyed(x, "same-key", "old-target originally refused", 0)
	claims, _, _ := claimBatch(queuedNames(func(string) bool { return true }))
	if len(claims) != 1 {
		t.Fatal("legacy fixture did not take its claim")
	}
	intent, err := prepareNativeIntent("%old", "#123abc", "old-target originally refused · #123abc", claims)
	if err != nil {
		t.Fatal(err)
	}
	intent.Outcome = "refused" // installed v2 record without retry or target proof
	if err := writeNativeIntent(intent); err != nil {
		t.Fatal(err)
	}
	// Model the old runtime having linked the pending name before process exit.
	if err := os.Link(claims[0], claimBase(claims[0])); err != nil {
		t.Fatal(err)
	}
	evidence := []string{claims[0], claimBase(claims[0]), nativeIntentPath(intent), nativePinPath(intent.Entries[0])}
	original := make(map[string]os.FileInfo)
	for _, path := range evidence {
		old, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		original[path] = old
	}
	var submitted []string
	x.nativeSubmit = func(pane, payload, _ string) (dispatch.Result, bool) {
		if pane != "%new" {
			t.Fatal("unexpected target", pane)
		}
		submitted = append(submitted, payload)
		return dispatch.Result{State: dispatch.StateQueued}, true
	}
	enqueue(x, "independently scheduled for current HQ")
	drain(x, "%new")
	drain(x, "%new")
	if len(submitted) != 1 || !strings.Contains(submitted[0], "independently scheduled") ||
		strings.Contains(submitted[0], "old-target") || f.pastes != 0 || f.enters != 0 {
		t.Fatalf("old record authorized current input or blocked its independent wake: %q", submitted)
	}
	for _, path := range evidence {
		after, err := os.Stat(path)
		if err != nil || !os.SameFile(original[path], after) {
			t.Fatalf("original evidence changed: %s: %v", filepath.Base(path), err)
		}
	}
}

func TestHWA018QueueIOHealthSurvivesUntilVerifiedProgress(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	const base = int64(1700000000)
	now := base
	x := (&fake{}).io()
	x.nowNano = func() int64 { return now * int64(time.Second) }
	if err := state.WritePrivateFile(queueDir(), []byte("foreign queue leaf")); err != nil {
		t.Fatal(err)
	}
	notice := func() bool {
		for _, reason := range ReadHealth(now).Issues {
			if reason.Reason == issueQueueIO {
				return true
			}
		}
		return false
	}
	if !notice() {
		t.Fatal("read-only queue inspection hid directory failure")
	}
	enqueue(x, "unavailable ordinary admission")
	now++
	enqueueKeyed(x, "unavailable", "unavailable keyed admission", 0)
	if _, err := readIssue(issueQueueIO); err != nil || !notice() {
		t.Fatal("failed producers did not persist an independently visible cause", err)
	}
	if err := os.Rename(queueDir(), filepath.Join(state.Dir(), "queue-foreign-fixture")); err != nil {
		t.Fatal(err)
	}
	if err := state.MkdirAll(queueDir(), 0700); err != nil {
		t.Fatal(err)
	}
	now += 90
	if !notice() {
		t.Fatal("readable storage alone cleared the issue")
	}
	noteConfirmedDelivery(now + 1)
	now += 2
	if notice() {
		t.Fatal("clear check ignored readable storage and independently verified progress")
	}
}
