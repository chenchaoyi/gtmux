package state

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func layoutFixture(t *testing.T, name, text string) os.FileInfo {
	t.Helper()
	path := filepath.Join(Dir(), name)
	if err := MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := WritePrivateFile(path, []byte(text)); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info
}

func wantLayoutContents(t *testing.T, path, text string, before os.FileInfo) {
	t.Helper()
	payload, err := os.ReadFile(path)
	if err != nil || string(payload) != text {
		t.Fatalf("read %s: %q (%v)", path, payload, err)
	}
	after, err := os.Stat(path)
	if err != nil || before != nil && !os.SameFile(before, after) {
		t.Fatalf("identity changed: %s (%v)", path, err)
	}
}

func layoutWritersStopped() error { return nil }

func TestGSS001GroupedLayoutRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	status, err := ReadStorageLayout()
	if err != nil || status.Mode != layoutFlat {
		t.Fatalf("empty layout: %+v (%v)", status, err)
	}
	if _, err := os.Lstat(Dir()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("inspection created a root: %v", err)
	}
	moves := []struct {
		store, child, group, text string
	}{
		{"active", "%2", "panes", "turn"},
		{"relay", "durable.json", "inbox", "unknown outcome"},
		{"events.seq", "", "events", "413"},
		{"hq-snapshots", "recovery.tar.gz", "backups", "backup"},
	}
	before := make(map[string]os.FileInfo)
	for _, sample := range moves {
		before[sample.store] = layoutFixture(t, filepath.Join(sample.store, sample.child), sample.text)
	}
	stable := layoutFixture(t, "uploads/source.png", "original path")
	unknown := layoutFixture(t, "unclassified.note", "independent record")
	if err := ChangeStorageLayout(true, layoutWritersStopped); err != nil {
		t.Fatal(err)
	}
	if err := CheckStorageLayout(); err != nil {
		t.Fatal(err)
	}
	for _, sample := range moves {
		want := filepath.Join(Dir(), layoutDir, sample.group, sample.store, sample.child)
		if got := StorePath(sample.store, sample.child); got != want {
			t.Fatalf("grouped path = %s, want %s", got, want)
		}
		wantLayoutContents(t, want, sample.text, before[sample.store])
		if _, err := os.Stat(filepath.Join(Dir(), sample.store)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("flat duplicate %s: %v", sample.store, err)
		}
	}
	shadow := layoutFixture(t, "events.seq", "unexpected legacy write")
	if err := CheckStorageLayout(); err == nil {
		t.Fatal("a legacy writer recreated a flat store without a conflict")
	}
	if err := ChangeStorageLayout(false, layoutWritersStopped); err == nil {
		t.Fatal("rollback should refuse an occupied legacy path")
	}
	flatSequence := filepath.Join(Dir(), "events.seq")
	wantLayoutContents(t, flatSequence, "unexpected legacy write", shadow)
	wantLayoutContents(t, StorePath("events.seq"), "413", before["events.seq"])
	if err := os.Remove(flatSequence); err != nil {
		t.Fatal(err)
	}
	// A receipt produced after activation follows the same directory on rollback.
	newReceipt := StorePath("relay", "after.json")
	if err := WritePrivateFile(newReceipt, []byte("accepted once")); err != nil {
		t.Fatal(err)
	}
	newIdentity, err := os.Stat(newReceipt)
	if err != nil {
		t.Fatal(err)
	}
	if err := ChangeStorageLayout(false, layoutWritersStopped); err != nil {
		t.Fatal(err)
	}
	if err := ChangeStorageLayout(false, layoutWritersStopped); err != nil {
		t.Fatalf("repeat flatten: %v", err)
	}
	for _, sample := range moves {
		wantLayoutContents(t, StorePath(sample.store, sample.child), sample.text, before[sample.store])
	}
	wantLayoutContents(t, StorePath("relay", "after.json"), "accepted once", newIdentity)
	wantLayoutContents(t, StorePath("uploads", "source.png"), "original path", stable)
	wantLayoutContents(t, StorePath("unclassified.note"), "independent record", unknown)
	if _, err := os.Lstat(filepath.Join(Dir(), layoutDir)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unused groups should be removed on rollback: %v", err)
	}
}

func TestGSS002InterruptedLayoutAndConflicts(t *testing.T) {
	t.Run("partial", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())
		input := layoutFixture(t, "active/%8", "original")
		relay := layoutFixture(t, "relay/receipt.json", "uncertain")
		probes := 0
		stop := errors.New("a writer appeared")
		if err := ChangeStorageLayout(true, func() error {
			probes++
			if probes == 2 {
				return stop
			}
			return nil
		}); !errors.Is(err, stop) {
			t.Fatalf("second writer check: %v", err)
		}
		if err := CheckStorageLayout(); err == nil {
			t.Fatal("normal commands should be gated")
		}
		status, err := ReadStorageLayout()
		if err != nil || status.Mode != layoutFlat || status.Continuing != layoutGrouped {
			t.Fatalf("partial status: %+v (%v)", status, err)
		}
		if err := ChangeStorageLayout(false, layoutWritersStopped); err == nil {
			t.Fatal("the opposite direction should be refused")
		}
		dest := filepath.Join(Dir(), layoutDir, "panes", "active")
		if err := MkdirAll(filepath.Dir(dest), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(StorePath("active"), dest); err != nil {
			t.Fatal(err)
		}
		other := layoutFixture(t, "layout-v2/inbox/relay/receipt.json", "unexpected")
		if err := ChangeStorageLayout(true, layoutWritersStopped); err == nil {
			t.Fatal("destination conflict should be preserved")
		}
		wantLayoutContents(t, filepath.Join(dest, "%8"), "original", input)
		wantLayoutContents(t, StorePath("relay", "receipt.json"), "uncertain", relay)
		wantLayoutContents(t, filepath.Join(Dir(), layoutDir, "inbox", "relay", "receipt.json"), "unexpected", other)
		if err := os.RemoveAll(filepath.Join(Dir(), layoutDir, "inbox", "relay")); err != nil {
			t.Fatal(err)
		}
		if err := ChangeStorageLayout(true, layoutWritersStopped); err != nil {
			t.Fatal(err)
		}
		wantLayoutContents(t, StorePath("active", "%8"), "original", input)
		wantLayoutContents(t, StorePath("relay", "receipt.json"), "uncertain", relay)
		if err := CheckStorageLayout(); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("symlink", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())
		original := layoutFixture(t, "keep/record", "independent")
		if err := os.Symlink(filepath.Join(Dir(), "keep"), StorePath("active")); err != nil {
			t.Fatal(err)
		}
		if err := ChangeStorageLayout(true, layoutWritersStopped); err == nil {
			t.Fatal("migrating a symlink should be refused")
		}
		wantLayoutContents(t, filepath.Join(Dir(), "keep", "record"), "independent", original)
		if err := CheckStorageLayout(); err != nil {
			t.Fatalf("preflight should not leave a transaction: %v", err)
		}
	})
}
