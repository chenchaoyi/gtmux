package hqnudge

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/state"
)

func TestHWA001AttributableParkCollisionKeepsIndependentDelivery(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	const legacy = "0000000000000000000-0.p1.k-workspace-reconciliation.txt"
	queued := filepath.Join(queueDir(), legacy)
	claim := queued + sendingSuffix
	parked := queued + nativeUncertainSuffix
	stage := func(payload, id string, park bool) nativeIntent {
		t.Helper()
		if err := state.WritePrivateFile(queued, []byte(payload)); err != nil {
			t.Fatal(err)
		}
		if !takeQueued(queued, claim) {
			t.Fatal("synthetic legacy claim failed")
		}
		intent, err := prepareNativeIntent("%isolated", id, payload, []string{claim})
		if err != nil {
			t.Fatal(err)
		}
		intent.Outcome = "uncertain"
		if err := writeNativeIntent(intent); err != nil {
			t.Fatal(err)
		}
		if park {
			if err := settleNativeIntent(intent); err != nil {
				t.Fatal(err)
			}
		}
		return intent
	}

	old := stage("first uncertain generation", "#123abc", true)
	newer := stage("second uncertain generation", "#234bcd", false)
	type fileEvidence struct {
		info os.FileInfo
		data []byte
	}
	evidence := make(map[string]fileEvidence)
	for _, path := range []string{claim, parked, nativeIntentPath(old), nativeIntentPath(newer), nativePinPath(old.Entries[0]), nativePinPath(newer.Entries[0])} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		evidence[path] = fileEvidence{info, data}
	}

	f := &fake{}
	x := f.io()
	var submitted []string
	x.nativeSubmit = func(_, payload, _ string) (dispatch.Result, bool) {
		submitted = append(submitted, payload)
		return dispatch.Result{State: dispatch.StateQueued}, true
	}
	enqueue(x, "independent third generation")
	drain(x, "%isolated")
	drain(x, "%isolated")
	if len(submitted) != 1 || !strings.Contains(submitted[0], "independent third generation") ||
		strings.Contains(submitted[0], "uncertain generation") || f.pastes != 0 {
		t.Fatalf("only independent entry may submit, once: %q, pastes=%d", submitted, f.pastes)
	}
	for path, before := range evidence {
		after, err := os.Stat(path)
		if err != nil || !os.SameFile(before.info, after) {
			t.Fatalf("evidence inode changed: %s: %v", filepath.Base(path), err)
		}
		data, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(data, before.data) {
			t.Fatalf("evidence contents changed: %s: %v", filepath.Base(path), err)
		}
	}

	if err := state.WritePrivateFile(filepath.Join(nativeIntentDir(), "unidentified.json"), []byte("{")); err != nil {
		t.Fatal(err)
	}
	enqueue(x, "must hold for unidentified record")
	drain(x, "%isolated")
	if len(submitted) != 1 || queuedCount(t) != 1 || f.pastes != 0 {
		t.Fatal("an unidentified recovery record did not close admission")
	}
}

func TestHWA002KeyedMergePreservesOnlyCurrentPendingIncarnation(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	const legacy = "0000000000000000000-0.p1.k-workspace-reconciliation.txt"
	path := filepath.Join(queueDir(), legacy)
	if err := state.WritePrivateFile(path, []byte("legacy original")); err != nil {
		t.Fatal(err)
	}

	f := &fake{}
	x := f.io()
	x.nowNano = func() int64 { return 42 }
	key := "workspace-reconciliation"
	enqueueKeyed(x, key, "legacy updated", 0)
	var attempts int
	var newer string
	x.nativeSubmit = func(_, payload, _ string) (dispatch.Result, bool) {
		attempts++
		if attempts == 1 {
			if !strings.Contains(payload, "legacy updated") || strings.Contains(payload, "legacy original") {
				t.Fatalf("legacy pending merge lost: %q", payload)
			}
			enqueueKeyed(x, key, "new generation", 0)
			newer = pendingKey(keyField(key))
			if newer == "" || newer == legacy || dueOf(newer) != 0 {
				t.Fatalf("queued generation reused legacy name: %q", newer)
			}
			return dispatch.Result{State: dispatch.StateRefusedWaiting, Submission: dispatch.SubmissionNotSubmitted, Evidence: "wake-loaded-idle-required"}, true
		}
		if !strings.Contains(payload, "new generation merged") {
			t.Fatalf("stale generation was reintroduced: %q", payload)
		}
		return dispatch.Result{State: dispatch.StateQueued}, true
	}
	drain(x, "%isolated")
	if queuedCount(t) != 1 || claimCount(t) != 0 {
		t.Fatal("refused older generation did not yield to the newer pending entry")
	}
	enqueueKeyed(x, key, "new generation merged", 0)
	if pendingKey(keyField(key)) != newer {
		t.Fatal("pending update changed the generation identity")
	}
	drain(x, "%isolated")
	enqueueKeyed(x, key, "third generation", 0)
	if third := pendingKey(keyField(key)); third == "" || third == newer || third == legacy {
		t.Fatalf("new entry reused a retired identity: %q", third)
	}
	if attempts != 2 || f.pastes != 0 {
		t.Fatal("unexpected native attempts or legacy input", attempts, f.pastes)
	}
}
