package state

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestGSS016RejectsJournalReplacementBeforeMove(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	identity := layoutFixture(t, "events.seq", "19")
	journal := filepath.Join(Dir(), layoutTransition)
	backup := filepath.Join(Dir(), "original-migration-journal")
	calls := 0
	err := ChangeStorageLayout(true, func() error {
		calls++
		if calls != 2 {
			return nil
		}
		if err := os.Rename(journal, backup); err != nil {
			return err
		}
		data, err := os.ReadFile(backup)
		if err != nil {
			return err
		}
		return WritePrivateFile(journal, data) // same plan and contents, distinct inode
	})
	if err == nil {
		t.Fatal("transition accepted a replacement for the journal it just published")
	}
	current, err := os.Stat(journal)
	if err != nil {
		t.Fatal(err)
	}
	original, err := os.Stat(backup)
	if err != nil || os.SameFile(original, current) {
		t.Fatalf("journal identity not retained: %v", err)
	}
	wantLayoutContents(t, filepath.Join(Dir(), "events.seq"), "19", identity)
	if _, err := os.Lstat(groupedRoot()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("journal replacement still caused a move: %v", err)
	}
}

func TestGSS016RetirementPreservesReplacedJournal(t *testing.T) {
	for _, kind := range []string{"different file", "symlink", "edited in place"} {
		t.Run(kind, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			identity := layoutFixture(t, "events.seq", "19")
			tx, journal := pauseLayoutForCommitTest(t, true)
			for _, group := range layoutGroups() {
				if err := MkdirAll(filepath.Join(groupedRoot(), group), PrivateDir); err != nil {
					t.Fatal(err)
				}
			}
			flat, grouped := layoutLocations("events.seq", layoutFlat)
			if err := os.Rename(flat, grouped); err != nil {
				t.Fatal(err)
			}
			if err := createLayoutMarker(); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(Dir(), layoutTransition)
			preserved := filepath.Join(Dir(), "retained-journal")
			contents := "independent file"
			var err error
			if kind == "edited in place" {
				contents = string(journal) + "\n"
				err = os.WriteFile(path, []byte(contents), 0600)
			} else {
				if err := os.Rename(path, preserved); err != nil {
					t.Fatal(err)
				}
				if kind == "different file" {
					err = os.WriteFile(path, []byte(contents), 0600)
				} else {
					err = os.Symlink(preserved, path)
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			before, err := os.Lstat(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := completeLayoutChange(tx); err == nil {
				t.Fatal("retirement deleted replacement journal")
			}
			after, err := os.Lstat(path)
			if err != nil || !os.SameFile(before, after) {
				t.Fatalf("conflicting journal changed: %v", err)
			}
			if kind != "symlink" {
				wantLayoutContents(t, path, contents, before)
			}
			if kind != "edited in place" {
				wantLayoutContents(t, preserved, string(journal), nil)
			}
			wantLayoutContents(t, grouped, "19", identity)
		})
	}
}

func TestGSS017RollbackPreservesReplacementEmptyGroup(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	identity := layoutFixture(t, "events.seq", "19")
	if err := ChangeStorageLayout(true, layoutWritersStopped); err != nil {
		t.Fatal(err)
	}
	_, journal := pauseLayoutForCommitTest(t, false)
	path := filepath.Join(groupedRoot(), "backups")
	original := filepath.Join(Dir(), "original-empty-group")
	if err := os.Rename(path, original); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, PrivateDir); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := ChangeStorageLayout(false, layoutWritersStopped); err == nil {
		t.Fatal("rollback accepted a substitute empty group")
	}
	after, err := os.Stat(path)
	if err != nil || !os.SameFile(after, before) {
		t.Fatalf("replacement group was not preserved: %v", err)
	}
	old, err := os.Stat(original)
	if err != nil || os.SameFile(old, after) {
		t.Fatalf("original group was not preserved: %v", err)
	}
	wantLayoutContents(t, StorePath("events.seq"), "19", identity)
	wantLayoutContents(t, filepath.Join(Dir(), layoutTransition), string(journal), nil)
}
