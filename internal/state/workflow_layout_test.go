package state

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGSW001WorkflowStorageRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := ChangeStorageLayout(true, layoutWritersStopped); err != nil {
		t.Fatal(err)
	}
	samples := map[string]string{
		"hq-nudges/parked.native-uncertain":  "unknown delivery",
		"hq-nudge-native-batches/batch.json": "original intent",
		"hq-wake-accepted/proof.json":        "receiver proof",
		workflowArchive:                      "archived proof",
		"hqwake/input.lock":                  "",
		"lease-notify-queue.lock":            "",
	}
	original := make(map[string]os.FileInfo)
	for path, body := range samples {
		original[path] = layoutFixture(t, path, body)
	}
	pin := filepath.Join(Dir(), "hq-nudge-native-claims")
	if err := MkdirAll(pin, PrivateDir); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(filepath.Join(Dir(), "hq-nudges/parked.native-uncertain"), filepath.Join(pin, "claim.pin")); err != nil {
		t.Fatal(err)
	}
	terminal := layoutFixture(t, "terminal-input/pending.json", "original input")
	terminalLock := layoutFixture(t, "lease-terminal-input.lock", "")
	bootstrap, err := os.Stat(filepath.Join(Dir(), "lease-storage-layout.lock"))
	if err != nil {
		t.Fatal(err)
	}
	for _, grouped := range []bool{true, false} {
		if err := ChangeWorkflowLayout(grouped, layoutWritersStopped); err != nil {
			t.Fatal(err)
		}
		status, err := ReadStorageLayout()
		if err != nil || status.WorkflowActive() != grouped || status.Mode != layoutGrouped {
			t.Fatalf("layout after grouping=%v: %+v (%v)", grouped, status, err)
		}
		if err := CheckStorageLayout(); err != nil {
			t.Fatal(err)
		}
		if grouped {
			if _, err := readLayoutTransaction(); err == nil {
				t.Fatal("legacy layout maintenance would accept the version fence")
			}
		}
		wantLayoutContents(t, filepath.Join(Dir(), "terminal-input/pending.json"), "original input", terminal)
		wantLayoutContents(t, WorkflowPath("lease-terminal-input.lock"), "", terminalLock)
		wantLayoutContents(t, filepath.Join(Dir(), "lease-storage-layout.lock"), "", bootstrap)
		wantLayoutContents(t, WorkflowPath("hq-nudge-native-claims", "claim.pin"), "unknown delivery", original["hq-nudges/parked.native-uncertain"])
		for path, body := range samples {
			base, child := path, ""
			if path != workflowArchive {
				base, child, _ = strings.Cut(path, "/")
			}
			actual := WorkflowPath(base, child)
			if grouped == (actual == filepath.Join(Dir(), path)) {
				t.Fatalf("unexpected selected path after grouping=%v: %s", grouped, actual)
			}
			wantLayoutContents(t, actual, body, original[path])
		}
		lease, err := TryLease("notify-queue")
		if err != nil {
			t.Fatal(err)
		}
		_ = lease.Close()
	}
}

func TestGSW002WorkflowResumeAndConflict(t *testing.T) {
	for _, scenario := range []string{"resume", "conflict", "held lease"} {
		t.Run(scenario, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			if err := ChangeStorageLayout(true, layoutWritersStopped); err != nil {
				t.Fatal(err)
			}
			original := layoutFixture(t, "hq-nudges/item", "original")
			unrelated := layoutFixture(t, "uploads/independent", "independent")
			if scenario == "held lease" {
				held, err := TryLease("notify-queue")
				if err != nil {
					t.Fatal(err)
				}
				defer held.Close()
				if err := ChangeWorkflowLayout(true, layoutWritersStopped); !errors.Is(err, ErrLeaseHeld) {
					t.Fatalf("held lease: %v", err)
				}
				if journal, err := readWorkflowJournal(); journal != nil || err != nil {
					t.Fatalf("held lease published a plan: %+v %v", journal, err)
				}
				return
			}
			entries, err := planWorkflowMoves(true)
			if err != nil || len(entries) != 1 {
				t.Fatalf("plan: %+v %v", entries, err)
			}
			journal := &workflowJournal{Version: workflowVersion, Phase: workflowGrouping, Entries: entries}
			if err := publishWorkflowJournal(journal, nil); err != nil {
				t.Fatal(err)
			}
			entry := entries[0]
			from, to := workflowMovePaths(entry.Name, journal.Phase)
			if err := advanceLayoutEntry(entry, from, to); err != nil {
				t.Fatal(err)
			}
			if err := CheckStorageLayout(); err == nil {
				t.Fatal("pending migration admitted commands")
			}
			if scenario == "conflict" {
				conflict := layoutFixture(t, "hq-nudges/item", "different original")
				if err := ChangeWorkflowLayout(true, layoutWritersStopped); err == nil ||
					!strings.Contains(err.Error(), from) || !strings.Contains(err.Error(), to) {
					t.Fatalf("conflict diagnosis: %v", err)
				}
				wantLayoutContents(t, filepath.Join(from, "item"), "different original", conflict)
				if err := CheckStorageLayout(); err == nil {
					t.Fatal("conflict retired the startup fence")
				}
			} else if err := ChangeWorkflowLayout(true, layoutWritersStopped); err != nil {
				t.Fatal(err)
			} else if err := CheckStorageLayout(); err != nil {
				t.Fatal(err)
			}
			wantLayoutContents(t, filepath.Join(to, "item"), "original", original)
			wantLayoutContents(t, filepath.Join(Dir(), "uploads/independent"), "independent", unrelated)
		})
	}
}
