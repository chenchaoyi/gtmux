package hqnudge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/driver"
	"github.com/chenchaoyi/gtmux/internal/state"
)

func writeReviewWake(t *testing.T, path, payload string) {
	t.Helper()
	if err := state.WritePrivateFile(path, []byte(payload)); err != nil {
		t.Fatal(err)
	}
}

func seedReviewIntent(t *testing.T, id string, claims ...string) nativeIntent {
	t.Helper()
	intent, err := prepareNativeIntent("%isolated", id, "synthetic batch · "+id, claims)
	if err != nil {
		t.Fatal(err)
	}
	intent.Outcome = "uncertain"
	if err := writeNativeIntent(intent); err != nil {
		t.Fatal(err)
	}
	return intent
}

func TestHWA009LateReceiptRespectsProtectedClaim(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	base := filepath.Join(queueDir(), "0000000000000000000-0.p1.k-workspace-reconciliation.txt")
	claim := base + sendingSuffix
	writeReviewWake(t, claim, "older parked generation")
	old := seedReviewIntent(t, "#012abc", claim)
	if err := settleNativeIntent(old); err != nil {
		t.Fatal(err)
	}
	writeReviewWake(t, claim, "second generation with unknown native outcome")
	recent := seedReviewIntent(t, "#123bcd", claim)
	writeReviewWake(t, base, "third generation pending in legacy slot")
	before, err := os.Stat(claim)
	if err != nil {
		t.Fatal(err)
	}

	protectedPaths := []string{base + nativeUncertainSuffix, claim, nativePinPath(old.Entries[0]),
		nativePinPath(recent.Entries[0]), nativeIntentPath(old), nativeIntentPath(recent)}
	originals := make(map[string]os.FileInfo)
	for _, path := range protectedPaths {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		originals[path] = info
	}
	f := &fake{}
	x := f.io()
	const previousID = "#234bcd"
	recordAttempt([]string{claim}, "earlier legacy submission · "+previousID, previousID, x.nowNano()/int64(time.Second)-1)
	x.receipt = func(_, id string, _ int64) driver.Verdict {
		if id == previousID {
			return driver.Confirmed
		}
		return driver.NoEvidence
	}
	var submitted []string
	x.nativeSubmit = func(_, payload, _ string) (dispatch.Result, bool) {
		submitted = append(submitted, payload)
		return dispatch.Result{State: dispatch.StateQueued}, true
	}
	enqueue(x, "independent generation")
	drain(x, "%isolated")

	after, statErr := os.Stat(claim)
	queued, queuedErr := os.ReadFile(base)
	if statErr != nil || !os.SameFile(before, after) || queuedErr != nil || string(queued) != "third generation pending in legacy slot" {
		t.Fatalf("protected paths changed during old receipt settlement: claim error=%v queued error=%v independent submit=%q", statErr, queuedErr, submitted)
	}
	for path, original := range originals {
		info, err := os.Stat(path)
		if err != nil || !os.SameFile(original, info) {
			t.Fatalf("native evidence changed at %s: %v", filepath.Base(path), err)
		}
	}
	if _, err := os.Stat(filepath.Join(attemptsDir(), "234bcd.json")); err != nil {
		t.Fatalf("late attempt record did not survive: %v", err)
	}
	if len(submitted) != 1 || !strings.Contains(submitted[0], "independent generation") {
		t.Fatalf("wrong admission after local collision: %q", submitted)
	}
}

func TestHWA010MixedRecoveryFaultHoldsAdmission(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	first := filepath.Join(queueDir(), "0000000000000000000-0.p1.k-first.txt")
	second := filepath.Join(queueDir(), "0000000000000000000-0.p1.k-second.txt")
	secondParked := second + nativeUncertainSuffix
	writeReviewWake(t, first+sendingSuffix, "first held claim")
	writeReviewWake(t, first+nativeUncertainSuffix, "earlier parked generation")
	writeReviewWake(t, second+sendingSuffix, "second held claim")
	if err := os.Mkdir(secondParked, 0700); err != nil {
		t.Fatal(err)
	}
	if err := parkNativeClaim(second + sendingSuffix); err == nil || strings.Contains(err.Error(), "native wake claim conflict") {
		t.Fatalf("expected a nonlocal store error in second slot, got %v", err)
	}
	intent := seedReviewIntent(t, "#012abc", first+sendingSuffix, second+sendingSuffix)

	f := &fake{}
	x := f.io()
	var submitted []string
	x.nativeSubmit = func(_, payload, _ string) (dispatch.Result, bool) {
		submitted = append(submitted, payload)
		return dispatch.Result{State: dispatch.StateQueued}, true
	}
	enqueue(x, "must wait for explicit recovery decision")
	drain(x, "%isolated")
	if len(submitted) != 0 || queuedCount(t) != 1 {
		t.Fatalf("mixed recovery failure did not hold admission: %q", submitted)
	}
	for _, path := range []string{first + sendingSuffix, second + sendingSuffix, nativeIntentPath(intent),
		nativePinPath(intent.Entries[0]), nativePinPath(intent.Entries[1])} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("native evidence lost at %s: %v", filepath.Base(path), err)
		}
	}
	if err := os.Remove(secondParked); err != nil {
		t.Fatal(err)
	}
	drain(x, "%isolated")
	if len(submitted) != 1 || !strings.Contains(submitted[0], "must wait") {
		t.Fatalf("after the hard fault was cleared, independent work did not resume: %q", submitted)
	}
}
