package state

import (
	"crypto/rand"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestGSS006RollbackRetainsInterruptedWrite(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	usage := layoutFixture(t, "usage-daily.json", "authoritative")
	if err := ChangeStorageLayout(true, layoutWritersStopped); err != nil {
		t.Fatal(err)
	}
	rel := filepath.Join("usage", privateTempPrefix+rand.Text())
	staged := layoutFixture(t, filepath.Join(layoutDir, rel), "partial replacement")
	status, err := ReadStorageLayout()
	if err != nil || status.StagedFiles != 1 {
		t.Fatalf("staging inspection: %+v %v", status, err)
	}
	calls := 0
	paused := errors.New("paused after preparation")
	if err := ChangeStorageLayout(false, func() error {
		calls++
		if calls == 2 {
			return paused
		}
		return nil
	}); !errors.Is(err, paused) {
		t.Fatalf("prepare rollback: %v", err)
	}
	txPath := filepath.Join(Dir(), layoutTransition)
	journal, err := os.ReadFile(txPath)
	if err != nil {
		t.Fatal(err)
	}
	status, err = ReadStorageLayout()
	if err != nil || status.RetainedPath == "" || status.Continuing != layoutFlat {
		t.Fatalf("pending rollback: %+v %v", status, err)
	}
	if err := ChangeStorageLayout(false, layoutWritersStopped); err != nil {
		t.Fatal(err)
	}
	retained := filepath.Join(Dir(), status.RetainedPath, rel)
	wantLayoutContents(t, retained, "partial replacement", staged)
	wantLayoutContents(t, StorePath("usage-daily.json"), "authoritative", usage)
	// Model loss of the last journal unlink after the archive was already durable.
	if err := WritePrivateFile(txPath, journal); err != nil {
		t.Fatal(err)
	}
	if err := ChangeStorageLayout(false, layoutWritersStopped); err != nil {
		t.Fatalf("resume after archival: %v", err)
	}
	if err := ChangeStorageLayout(true, layoutWritersStopped); err != nil {
		t.Fatalf("group after rollback: %v", err)
	}
	wantLayoutContents(t, retained, "partial replacement", staged)
	wantLayoutContents(t, StorePath("usage-daily.json"), "authoritative", usage)
}

func TestGSS006RollbackRefusesUnknownOrUnsafeStage(t *testing.T) {
	for _, test := range []string{"unrecognized", "symlink", "hardlink", "shared"} {
		t.Run(test, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			usage := layoutFixture(t, "usage-daily.json", "authoritative")
			if err := ChangeStorageLayout(true, layoutWritersStopped); err != nil {
				t.Fatal(err)
			}
			source := StorePath("usage-daily.json")
			path := filepath.Join(filepath.Dir(source), privateTempPrefix+rand.Text())
			var err error
			switch test {
			case "unrecognized":
				err = WritePrivateFile(filepath.Join(filepath.Dir(source), ".private-abandoned"), nil)
			case "symlink":
				err = os.Symlink(source, path)
			case "hardlink":
				err = os.Link(source, path)
			case "shared":
				err = os.WriteFile(path, nil, 0644)
				if err == nil {
					err = os.Chmod(path, 0644)
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := ChangeStorageLayout(false, layoutWritersStopped); err == nil {
				t.Fatal("unsafe entry allowed rollback")
			}
			wantLayoutContents(t, source, "authoritative", usage)
			if _, err := os.Lstat(filepath.Join(Dir(), layoutTransition)); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("failed preflight left a journal: %v", err)
			}
		})
	}
}
