package hqnudge

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func parkedAcrossRestartFixture(t *testing.T, id, payload string) nativeIntent {
	t.Helper()
	enqueue((&fake{}).io(), payload)
	claimed, _, _ := claimBatch(queuedNames(func(string) bool { return true }))
	if len(claimed) != 1 {
		t.Fatalf("expected a single synthetic claim, got %v", claimed)
	}
	r, err := prepareNativeIntent("%isolated", id, payload, claimed)
	if err != nil {
		t.Fatal(err)
	}
	r.Outcome = "uncertain"
	if err := settleNativeIntent(r); err != nil {
		t.Fatal(err)
	}
	before := nativeIntentPath(r)
	_, inode, ok := strings.Cut(r.Entries[0].FileID, ":")
	if !ok {
		t.Fatal("missing inode in the saved identity")
	}
	// Simulate the old assigned device number without changing the live files.
	r.Entries[0].FileID = "18446744073709551615:" + inode
	if err := writeNativeIntent(r); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(before); err != nil {
		t.Fatal(err)
	}
	return r
}

func wakeRestartFileProofs(t *testing.T, paths ...string) map[string]nativeEntry {
	t.Helper()
	proofs := map[string]nativeEntry{}
	for _, path := range paths {
		proof, err := nativeFingerprint(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		proofs[path] = proof
	}
	return proofs
}

func restartFixturePaths(r nativeIntent) []string {
	e := r.Entries[0]
	claim := filepath.Join(queueDir(), e.Name)
	return []string{nativeIntentPath(r), nativePinPath(e), claimBase(claim) + nativeUncertainSuffix, claim}
}

func TestWNR001DeviceChangeKeepsParkedUnknownIntact(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	r := parkedAcrossRestartFixture(t, "#a00001", "parked before reboot")
	paths := restartFixturePaths(r)
	before := wakeRestartFileProofs(t, paths...)
	if len(before) != 3 {
		t.Fatalf("the parked fixture had %d files, want three", len(before))
	}
	for range 2 {
		protected, err := recoverNativeIntentsWithProtection()
		if err != nil || len(protected) > 0 {
			t.Fatalf("an exact parked generation was classified as a conflict: %v, %v", protected, err)
		}
	}
	if after := wakeRestartFileProofs(t, paths...); !reflect.DeepEqual(before, after) || Pending() {
		t.Fatal("read-only recognition changed or requeued the original")
	}
}

func TestWNR002DeviceChangeStillProtectsAmbiguousClaims(t *testing.T) {
	for _, mode := range []string{"live claim", "replacement inode", "changed bytes", "missing parked copy"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			original := parkedAcrossRestartFixture(t, "#a00001", "interrupted submission")
			independent := parkedAcrossRestartFixture(t, "#a00002", "independently parked")
			paths := restartFixturePaths(original)
			parked, source := paths[2], paths[3]
			var err error
			switch mode {
			case "live claim":
				err = os.Link(parked, source)
			case "replacement inode":
				if err := os.Remove(parked); err != nil {
					t.Fatal(err)
				}
				err = os.WriteFile(parked, []byte(original.Payload), 0600)
			case "changed bytes":
				err = os.WriteFile(parked, []byte("changed"), 0600)
			case "missing parked copy":
				err = os.Remove(parked)
			}
			if err != nil {
				t.Fatal(err)
			}
			paths = append(paths, restartFixturePaths(independent)...)
			before := wakeRestartFileProofs(t, paths...)
			protected, err := recoverNativeIntentsWithProtection()
			if !errors.Is(err, errNativeClaimConflict) || !reflect.DeepEqual(protected, map[string]bool{original.Entries[0].Name: true}) {
				t.Fatalf("ambiguous evidence did not protect only its slot: %v, %v", protected, err)
			}
			if after := wakeRestartFileProofs(t, paths...); !reflect.DeepEqual(before, after) {
				t.Fatal("recovery changed ambiguous or independently parked evidence")
			}
		})
	}
}
