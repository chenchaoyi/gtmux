package state

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestGSS010RollbackPreservesLateConflict(t *testing.T) {
	for _, kind := range []string{"unknown file", "unknown empty directory", "fixed group is a file", "fixed group is a symlink", "fixed group is not empty"} {
		t.Run(kind, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			if err := ChangeStorageLayout(true, layoutWritersStopped); err != nil {
				t.Fatal(err)
			}
			paused := errors.New("stop after journaling")
			calls := 0
			if err := ChangeStorageLayout(false, func() error {
				calls++
				if calls == 2 {
					return paused
				}
				return nil
			}); !errors.Is(err, paused) {
				t.Fatalf("prepare rollback: %v", err)
			}
			tx, err := readLayoutTransaction()
			if err != nil || tx == nil {
				t.Fatalf("read rollback journal: %v", err)
			}
			if _, err := verifyLayoutTransition(tx); err != nil {
				t.Fatalf("verify before a late write: %v", err)
			}
			source := filepath.Join(groupedRoot(), "panes")
			switch kind {
			case "unknown file":
				source = filepath.Join(groupedRoot(), "other")
				err = os.WriteFile(source, []byte("unrelated record"), 0600)
			case "unknown empty directory":
				source = filepath.Join(groupedRoot(), "other")
				err = os.Mkdir(source, 0700)
			case "fixed group is a file":
				if err = os.Remove(source); err == nil {
					err = os.WriteFile(source, []byte("unrelated record"), 0600)
				}
			case "fixed group is a symlink":
				if err = os.Remove(source); err == nil {
					err = os.Symlink(filepath.Join(groupedRoot(), "clients"), source)
				}
			case "fixed group is not empty":
				source = filepath.Join(source, "late")
				err = os.WriteFile(source, []byte("unrelated record"), 0600)
			}
			if err != nil {
				t.Fatal(err)
			}
			before, err := os.Lstat(source)
			if err != nil {
				t.Fatal(err)
			}
			if err := removeEmptyLayoutDirs(tx.Scaffold); err == nil {
				t.Fatal("cleanup accepted a late conflicting object")
			}
			after, err := os.Lstat(source)
			if err != nil || !os.SameFile(before, after) {
				t.Fatalf("cleanup changed the conflicting object: %v", err)
			}
			if before.Mode().IsRegular() {
				wantLayoutContents(t, source, "unrelated record", before)
			}
			if tx, err := readLayoutTransaction(); err != nil || tx == nil {
				t.Fatalf("cleanup retired the pending journal: %v", err)
			}
		})
	}
}
