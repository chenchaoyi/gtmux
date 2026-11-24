package state

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func pauseLayoutForCommitTest(t *testing.T, grouped bool) (*layoutTransaction, []byte) {
	t.Helper()
	paused := errors.New("stop after journaling")
	calls := 0
	if err := ChangeStorageLayout(grouped, func() error {
		calls++
		if calls == 2 {
			return paused
		}
		return nil
	}); !errors.Is(err, paused) {
		t.Fatalf("prepare transition: %v", err)
	}
	tx, err := readLayoutTransaction()
	if err != nil || tx == nil {
		t.Fatalf("read transition: %v", err)
	}
	bytes, err := os.ReadFile(filepath.Join(Dir(), layoutTransition))
	if err != nil {
		t.Fatal(err)
	}
	return tx, bytes
}

func TestGSS013MarkerConflictPreservesOriginals(t *testing.T) {
	for _, kind := range []string{"regular", "symlink", "nonempty directory", "different empty directory"} {
		t.Run(kind, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			before := layoutFixture(t, "events.seq", "19")
			if err := ChangeStorageLayout(true, layoutWritersStopped); err != nil {
				t.Fatal(err)
			}
			_, journal := pauseLayoutForCommitTest(t, false)
			source := StorePath("events.seq")
			marker := filepath.Join(Dir(), layoutMarker)
			old := filepath.Join(Dir(), "unrelated-original")
			if err := os.Rename(marker, old); err != nil {
				t.Fatal(err)
			}
			var err error
			switch kind {
			case "regular":
				err = os.WriteFile(marker, []byte("unrelated data"), 0600)
			case "symlink":
				err = os.Symlink(old, marker)
			case "nonempty directory":
				if err = os.Mkdir(marker, 0700); err == nil {
					err = os.WriteFile(filepath.Join(marker, "record"), []byte("unrelated data"), 0600)
				}
			case "different empty directory":
				err = os.Mkdir(marker, 0700)
			}
			if err != nil {
				t.Fatal(err)
			}
			replacement, err := os.Lstat(marker)
			if err != nil {
				t.Fatal(err)
			}
			if err := ChangeStorageLayout(false, layoutWritersStopped); err == nil {
				t.Fatal("rollback accepted a conflicting layout marker")
			}
			after, err := os.Lstat(marker)
			if err != nil || !os.SameFile(replacement, after) {
				t.Fatalf("rollback replaced the marker: %v", err)
			}
			if kind == "regular" {
				wantLayoutContents(t, marker, "unrelated data", replacement)
			}
			if kind == "nonempty directory" {
				wantLayoutContents(t, filepath.Join(marker, "record"), "unrelated data", nil)
			}
			wantLayoutContents(t, source, "19", before)
			wantLayoutContents(t, filepath.Join(Dir(), layoutTransition), string(journal), nil)
			if _, err := os.Lstat(filepath.Join(Dir(), "events.seq")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("rollback moved an authoritative store despite conflict: %v", err)
			}
		})
	}
	t.Run("occupied grouped marker", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())
		identity := layoutFixture(t, "events.seq", "19")
		_, journal := pauseLayoutForCommitTest(t, true)
		other := layoutFixture(t, layoutMarker, "unknown marker contents")
		if err := ChangeStorageLayout(true, layoutWritersStopped); err == nil {
			t.Fatal("grouping overwrote a conflicting marker")
		}
		wantLayoutContents(t, filepath.Join(Dir(), layoutMarker), "unknown marker contents", other)
		wantLayoutContents(t, filepath.Join(Dir(), "events.seq"), "19", identity)
		wantLayoutContents(t, filepath.Join(Dir(), layoutTransition), string(journal), nil)
	})
}

func TestGSS014CommitNeedsCompleteTargetLayout(t *testing.T) {
	for _, kind := range []string{"source returned", "missing target group", "unarchived staging", "reappeared grouped root"} {
		t.Run(kind, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			layoutFixture(t, "events.seq", "19")
			targetGrouped := kind == "source returned" || kind == "missing target group"
			if !targetGrouped {
				if err := ChangeStorageLayout(true, layoutWritersStopped); err != nil {
					t.Fatal(err)
				}
			}
			if kind == "unarchived staging" {
				layoutFixture(t, filepath.Join(layoutDir, "usage", privateTempPrefix+"ABCDEFGHIJKLMNOPQRSTUVWXYZ"), "scratch")
			}
			tx, journal := pauseLayoutForCommitTest(t, targetGrouped)
			flat := filepath.Join(Dir(), "events.seq")
			grouped := filepath.Join(groupedRoot(), "events", "events.seq")
			txPath := filepath.Join(Dir(), layoutTransition)
			if kind == "unarchived staging" {
				if err := os.Rename(grouped, flat); err != nil {
					t.Fatal(err)
				}
				if err := removeLayoutMarker(*tx.Marker); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := ChangeStorageLayout(targetGrouped, layoutWritersStopped); err != nil {
					t.Fatal(err)
				}
				if err := WritePrivateFile(txPath, journal); err != nil {
					t.Fatal(err)
				}
				current, err := readLayoutTransaction()
				if err != nil || current == nil {
					t.Fatalf("read re-created journal: %v", err)
				}
				tx = current
				switch kind {
				case "source returned":
					err = os.Rename(grouped, flat)
				case "missing target group":
					err = os.Remove(filepath.Join(groupedRoot(), "backups"))
				case "reappeared grouped root":
					err = os.Mkdir(groupedRoot(), 0700)
				}
				if err != nil {
					t.Fatal(err)
				}
			}
			if err := completeLayoutChange(tx); err == nil {
				t.Fatal("incomplete target caused the transition to commit")
			}
			wantLayoutContents(t, txPath, string(journal), nil)
		})
	}
}
